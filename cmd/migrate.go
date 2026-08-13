package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syschecks/helpers"

	"github.com/spf13/cobra"
)

// Migration rewrites command strings that older versions wrote into files living on the
// host: cron job files under /etc/cron.d and the Zabbix agent's UserParameter line. It also
// repairs generated cron/cache modes that a restrictive root umask may have reduced to
// 0600 and records a readable schedule snapshot for hosts that intentionally prevent
// regular users from traversing /etc/cron.d.
//
// This is a command rather than a startup check on purpose. `banner` runs on every SSH
// login, and spending even 100-200 ms there to fix a one-time problem would tax every login
// forever.

var (
	migrateApply bool

	migrateCmd = &cobra.Command{
		Use:   "migrate",
		Short: "Migrate generated commands and repair cron/cache readability",
		Long: `Rewrite generated files that still reference pre-restructure command names.

Also repairs SysChecks cron/cache file modes and records schedule status outside /etc/cron.d,
so regular-user login banners work when that directory is intentionally root-only.

Reports what would change and exits non-zero when anything is outstanding, so a monitoring
check can ask "is this host fully migrated?". Pass --apply to actually rewrite. Installation
and upgrade scripts should call 'syschecks migrate --apply' as their final step.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrate(migrateApply)
		},
		// The exit code is the result, not a usage problem.
		SilenceUsage: true,
	}
)

// commandRewrite is one old→new substitution applied to file contents.
type commandRewrite struct {
	pattern     *regexp.Regexp
	replacement string
	description string
}

// Ordered most-specific first: `apply-updates --system` must be rewritten before the bare
// `apply-updates` rule can match its prefix.
var commandRewrites = []commandRewrite{
	{
		pattern:     regexp.MustCompile(`syschecks apply-updates\s+(?:--system|-s)\b`),
		replacement: "syschecks updates apply --scope system",
		description: "apply-updates --system → updates apply --scope system",
	},
	{
		pattern:     regexp.MustCompile(`syschecks apply-updates\b`),
		replacement: "syschecks updates apply --scope security",
		description: "apply-updates → updates apply --scope security",
	},
	{
		pattern:     regexp.MustCompile(`syschecks updates\s+--cache-create\b`),
		replacement: "syschecks updates refresh",
		description: "updates --cache-create → updates refresh",
	},
	{
		// --yes rather than a bare removal: it matches what a fresh install now writes, and
		// keeps the job non-interactive even if it ever runs somewhere with a TTY attached.
		pattern:     regexp.MustCompile(`syschecks kernel cleanup\s+--execute\b`),
		replacement: "syschecks kernel cleanup --yes",
		description: "kernel cleanup --execute → kernel cleanup --yes (removal is now the default)",
	},
	{
		pattern:     regexp.MustCompile(`syschecks cron status\b`),
		replacement: "syschecks schedule list",
		description: "cron status → schedule list",
	},
	{
		pattern:     regexp.MustCompile(`syschecks userinfo\b`),
		replacement: "syschecks users list",
		description: "userinfo → users list",
	},
	{
		pattern:     regexp.MustCompile(`syschecks sysinfo\b`),
		replacement: "syschecks banner --output json",
		description: "sysinfo → banner --output json",
	},
}

type migrationChange struct {
	path        string
	description string
	before      string
	after       string
}

// migrationTargets are the files migration may touch. Cron files are matched by glob because
// legacy installations left differently-named jobs behind.
func migrationTargets() []string {
	var targets []string
	for _, pattern := range []string{
		"/etc/cron.d/syschecks*",
		"/etc/cron.d/automatic_*",
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		targets = append(targets, matches...)
	}
	for _, path := range []string{
		"/etc/zabbix/zabbix_agentd.conf",
		"/etc/zabbix/zabbix_agent2.conf",
		"/tmp/syscheck_updates.json",
	} {
		if _, err := os.Stat(path); err == nil {
			targets = append(targets, path)
		}
	}
	return targets
}

// rewriteContent applies every rewrite and reports the individual line changes, so the
// report can show a real before/after rather than "this file needs attention".
func rewriteContent(path string, content string) (string, []migrationChange) {
	var changes []migrationChange
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		updated := line
		for _, rewrite := range commandRewrites {
			if !rewrite.pattern.MatchString(updated) {
				continue
			}
			next := rewrite.pattern.ReplaceAllString(updated, rewrite.replacement)
			if next == updated {
				continue
			}
			changes = append(changes, migrationChange{
				path:        path,
				description: rewrite.description,
				before:      strings.TrimSpace(updated),
				after:       strings.TrimSpace(next),
			})
			updated = next
		}
		lines[i] = updated
	}

	return strings.Join(lines, "\n"), changes
}

func runMigrate(apply bool) error {
	if apply {
		helpers.RootUserCheck()
	}

	var allChanges []migrationChange
	var failures []string

	for _, path := range migrationTargets() {
		var permissionChanges []migrationChange
		desiredMode, normalizeMode := migrationMode(path)
		if info, statErr := os.Stat(path); statErr == nil && normalizeMode && info.Mode().Perm() != desiredMode {
			permissionChanges = append(permissionChanges, migrationChange{
				path:        path,
				description: "make generated status readable by regular users",
				before:      info.Mode().Perm().String(),
				after:       desiredMode.String(),
			})
			allChanges = append(allChanges, permissionChanges...)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", path, err))
			continue
		}

		updated, changes := rewriteContent(path, string(data))
		if len(changes) == 0 && len(permissionChanges) == 0 {
			continue
		}
		allChanges = append(allChanges, changes...)

		if !apply {
			continue
		}

		mode := os.FileMode(helpers.CRON_FILE_PERMS)
		if info, statErr := os.Stat(path); statErr == nil {
			mode = info.Mode().Perm()
		}
		if len(changes) > 0 {
			if err := os.WriteFile(path, []byte(updated), mode); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", path, err))
				continue
			}
		}
		if normalizeMode {
			if err := os.Chmod(path, desiredMode); err != nil {
				failures = append(failures, fmt.Sprintf("%s: could not set permissions: %v", path, err))
			}
		}
	}
	if apply {
		if err := refreshScheduleStatusCache(); err != nil {
			failures = append(failures, fmt.Sprintf("schedule status cache: %v", err))
		}
	}

	printMigrationReport(allChanges, failures, apply)

	if len(failures) > 0 {
		return fmt.Errorf("%d file(s) could not be processed", len(failures))
	}
	if len(allChanges) > 0 && !apply {
		return fmt.Errorf("%d change(s) pending; re-run with --apply", len(allChanges))
	}
	return nil
}

func migrationMode(path string) (os.FileMode, bool) {
	if strings.HasPrefix(path, "/etc/cron.d/") || path == "/tmp/syscheck_updates.json" {
		return os.FileMode(helpers.CRON_FILE_PERMS), true
	}
	return 0, false
}

func printMigrationReport(changes []migrationChange, failures []string, apply bool) {
	if len(changes) == 0 && len(failures) == 0 {
		fmt.Println("Nothing to migrate: no file references an old command name.")
		return
	}

	verb := "Would rewrite"
	if apply {
		verb = "Rewrote"
	}

	currentPath := ""
	for _, change := range changes {
		if change.path != currentPath {
			currentPath = change.path
			fmt.Printf("\n%s %s\n", verb, currentPath)
		}
		fmt.Printf("  %s\n", change.description)
		fmt.Printf("    - %s\n", change.before)
		fmt.Printf("    + %s\n", change.after)
	}

	for _, failure := range failures {
		fmt.Printf("\nError: %s\n", failure)
	}

	if !apply && len(changes) > 0 {
		fmt.Printf("\n%d change(s) pending. Re-run with --apply to write them.\n", len(changes))
	}
}
