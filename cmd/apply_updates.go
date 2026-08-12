package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"syschecks/helpers"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// Pre-compiled regex for package parsing
var (
	reWhitespace = regexp.MustCompile(`\s+`)
	reVersionDnf = regexp.MustCompile(`-\d+.+`)
)

// applyUpdatesDefaultDelay spreads unattended update runs across roughly a quarter of an
// hour. Guests on one hypervisor are commonly created from the same image and so share an
// identical cron schedule; without a splay they all refresh metadata and unpack packages in
// the same minute, which shows up on the host as an I/O spike.
const applyUpdatesDefaultDelay = 15 * time.Minute

// Update scopes. A single enum replaces the previous pair of booleans, which could not
// express "exactly one of these" and meant something different on each command that had them.
const (
	updateScopeSecurity = "security"
	updateScopeSystem   = "system"
)

var updateScopeCompletions = []string{
	"security\tSecurity updates only (default)",
	"system\tAll available system updates",
}

var (
	applyUpdatesCmdSystemUpdates     bool
	applyUpdatesCmdIgnorePackageLock bool
	applyUpdatesIgnoreLocks          bool
	applyUpdatesScope                string
	applyUpdatesMaxDelay             time.Duration
	applyUpdatesNoDelay              bool
	applyUpdatesDryRun               bool

	applyUpdatesLong = `Apply system or security updates.

Unattended runs wait a random interval of up to --delay before starting, so that guests
sharing a virtualization host do not all update at once. Interactive runs never wait.`

	updatesApplyCmd = &cobra.Command{
		Use:     "apply",
		Short:   "Install available updates",
		Long:    applyUpdatesLong,
		Args:    cobra.NoArgs,
		PreRunE: applyUpdatesPreRun,
		Run:     func(cmd *cobra.Command, args []string) { runApplyUpdates() },
	}

	// The pre-restructure spelling. Kept because deployed cron files invoke it by name.
	applyUpdatesCmd = &cobra.Command{
		Use:     "apply-updates",
		Short:   "Apply updates (deprecated: use 'updates apply')",
		Long:    applyUpdatesLong,
		Args:    cobra.NoArgs,
		Hidden:  true,
		PreRunE: applyUpdatesPreRun,
		Run:     func(cmd *cobra.Command, args []string) { runApplyUpdates() },
	}
)

func applyUpdatesPreRun(cmd *cobra.Command, args []string) error {
	if applyUpdatesMaxDelay < 0 {
		return fmt.Errorf("--delay must not be negative")
	}
	if _, err := resolveUpdateScope(); err != nil {
		return err
	}
	return nil
}

// resolveUpdateScope folds the deprecated --system boolean into the --scope enum. Passing
// both is rejected rather than silently resolved, because the two could disagree.
func resolveUpdateScope() (string, error) {
	switch applyUpdatesScope {
	case "":
		if applyUpdatesCmdSystemUpdates {
			return updateScopeSystem, nil
		}
		return updateScopeSecurity, nil
	case updateScopeSecurity, updateScopeSystem:
		if applyUpdatesCmdSystemUpdates && applyUpdatesScope != updateScopeSystem {
			return "", fmt.Errorf("--system contradicts --scope %s; use --scope only", applyUpdatesScope)
		}
		return applyUpdatesScope, nil
	default:
		return "", fmt.Errorf("invalid --scope %q: must be %s or %s", applyUpdatesScope, updateScopeSecurity, updateScopeSystem)
	}
}

func runApplyUpdates() {
	scope, err := resolveUpdateScope()
	if err != nil {
		log.Fatal(err)
	}

	osType := detectOs()
	if scope == updateScopeSecurity && !securityOnlyUpdatesSupported(osType) {
		log.Fatal("Security-only updates are not supported on Alpine: apk does not expose Alpine secdb security advisories; use --scope system or query secdb separately")
	}

	// Delay before anything touches the network or disk, so the metadata refresh is
	// spread too, not just the package installs.
	waitBeforeApplyingUpdates(applyUpdatesMaxDelay, applyUpdatesNoDelay, runningInteractively())

	updates := systemUpdates(false)

	updateList := updates.SecurityUpdatesList
	if scope == updateScopeSystem {
		updateList = updates.SystemUpdatesList
	}

	if applyUpdatesDryRun {
		printApplyUpdatesPlan(scope, updateList)
		return
	}

	applyUpdates(updateList, osType)
	// Refresh cache after applying updates
	checkUpdates(true, false, false)
}

func securityOnlyUpdatesSupported(osType detectOsStruct) bool {
	return osType.packageManagerKind() != packageManagerAPK
}

func printApplyUpdatesPlan(scope string, updateList []string) {
	if len(updateList) == 0 {
		fmt.Printf("No %s updates to apply.\n", scope)
		return
	}

	locks := loadPackageLock()
	if applyUpdatesIgnoreLocks || applyUpdatesCmdIgnorePackageLock {
		locks = nil
	}

	fmt.Printf("Would apply %d %s update(s):\n", len(updateList), scope)
	seen := make(map[string]bool)
	for _, entry := range updateList {
		pkg := extractPackageName(entry)
		if pkg == "" || seen[pkg] {
			continue
		}
		seen[pkg] = true
		if locks != nil && isPackageLocked(pkg, locks) {
			fmt.Printf("  %s (skipped: locked in package.lock.json)\n", pkg)
			continue
		}
		fmt.Printf("  %s\n", pkg)
	}
}

// runningInteractively reports whether a human is watching. Cron redirects stdin, so this
// cleanly separates the unattended runs that need spreading from manual ones that do not.
func runningInteractively() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) || term.IsTerminal(int(os.Stdout.Fd()))
}

// applyUpdatesDelay picks the random wait before an unattended update run. It is kept
// separate from the sleep so the selection logic stays testable.
func applyUpdatesDelay(maxDelay time.Duration, disabled bool, interactive bool) time.Duration {
	if disabled || interactive || maxDelay <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(maxDelay) + 1))
}

func waitBeforeApplyingUpdates(maxDelay time.Duration, disabled bool, interactive bool) {
	delay := applyUpdatesDelay(maxDelay, disabled, interactive)
	if delay <= 0 {
		return
	}
	log.Printf("Waiting %s before applying updates to spread load across hosts (--no-delay to skip)", delay.Round(time.Second))
	time.Sleep(delay)
}

// loadPackageLock loads the package lock file, returning empty slice if not found
func loadPackageLock() []string {
	const lockFile = "/opt/syschecks/package.lock.json"

	data, err := os.ReadFile(lockFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Note: Package lock file not found at %s, proceeding without locks", lockFile)
			return []string{}
		}
		log.Printf("Warning: Could not read package lock file: %v", err)
		return []string{}
	}

	var packageLock []string
	if err := json.Unmarshal(data, &packageLock); err != nil {
		log.Printf("Warning: Could not parse package lock file: %v", err)
		return []string{}
	}

	return packageLock
}

// extractPackageName extracts the package name from an update entry
func extractPackageName(entry string) string {
	// If the entry has whitespace, take the first part
	if reWhitespace.MatchString(entry) {
		parts := reWhitespace.Split(entry, -1)
		if len(parts) > 0 {
			return parts[0]
		}
	}
	// Otherwise strip version suffix (DNF style)
	return reVersionDnf.ReplaceAllString(entry, "")
}

// isPackageLocked checks if a package matches any lock pattern
func isPackageLocked(pkg string, lockPatterns []string) bool {
	for _, pattern := range lockPatterns {
		if strings.Contains(pkg, pattern) {
			return true
		}
	}
	return false
}

// applyUpdates applies the given list of updates using the appropriate package manager.
// It respects the package.lock.json file unless --ignore-lock-file is specified.
func applyUpdates(updateList []string, osType detectOsStruct) {
	helpers.RootUserCheck()

	if len(updateList) == 0 {
		log.Println("No updates to apply")
		return
	}

	packageLock := loadPackageLock()
	if applyUpdatesCmdIgnorePackageLock || applyUpdatesIgnoreLocks {
		packageLock = nil
	}

	// Deduplicate and extract package names
	seen := make(map[string]bool)
	var packages []string

	for _, entry := range updateList {
		pkg := extractPackageName(entry)
		if pkg == "" || seen[pkg] {
			continue
		}
		seen[pkg] = true
		packages = append(packages, pkg)
	}

	if osType.unsupported {
		log.Fatal("Unsupported OS")
	}
	pm := getPackageManager(osType)

	totalPkgs := len(packages)
	for i, pkg := range packages {
		pkgNum := i + 1

		// Check if package is locked
		if packageLock != nil && isPackageLocked(pkg, packageLock) {
			log.Printf("Package skipped (%d of %d): %s (locked in package.lock.json)\n", pkgNum, totalPkgs, pkg)
			continue
		}

		ctx, cancel := packageCommandTimeout()
		cmd := pm.ApplyCommand(ctx, pkg)

		out, err := cmd.CombinedOutput()
		cancel() // Clean up context immediately after command completes

		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("Package timed out (%d of %d): %s (exceeded 10min timeout)\n", pkgNum, totalPkgs, pkg)
			continue
		}

		if err != nil {
			// Trim output for logging
			outStr := strings.TrimSpace(string(out))
			if len(outStr) > 200 {
				outStr = outStr[:200] + "..."
			}
			log.Printf("Package error (%d of %d): %s (%s)\n", pkgNum, totalPkgs, pkg, outStr)
			continue
		}

		log.Printf("Package upgraded (%d of %d): %s\n", pkgNum, totalPkgs, pkg)
	}
}
