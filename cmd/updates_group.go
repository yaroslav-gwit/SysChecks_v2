package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// The `updates` resource group. Previously the same noun was spelled three different ways
// (`updates`, `apply-updates`, `cron updates`); it is now one group with one verb each.
//
// The bare `updates` command keeps its original behaviour — printing the update report —
// because Zabbix invokes it through `UserParameter=syschecks[*],syschecks $1` and cron files
// on already-deployed hosts still call it.

var (
	updatesCheckRefresh bool
	updatesCheckCached  bool

	updatesCheckCmd = &cobra.Command{
		Use:   "check",
		Short: "Report available system and security updates",
		Long: `Report available system and security updates.

Reads the cached result by default so the command stays fast enough for monitoring polls.
Use --refresh to query the package manager directly, which also refreshes repository health.`,
		Args: cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if updatesCheckRefresh && updatesCheckCached {
				return fmt.Errorf("choose either --refresh or --cached, not both")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			runUpdatesCheck()
		},
	}

	updatesRefreshCmd = &cobra.Command{
		Use:     "refresh",
		Short:   "Refresh the cached update report",
		Long:    `Query the package manager and write the result to the update cache used by the banner and by monitoring.`,
		Args:    cobra.NoArgs,
		Aliases: []string{"cache-create"},
		Run: func(cmd *cobra.Command, args []string) {
			checkUpdates(true, false, false)
		},
	}

	// updates cache refresh — the longer spelling, kept because both readings are natural.
	updatesCacheCmd = &cobra.Command{
		Use:   "cache",
		Short: "Manage the update cache",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
	}

	updatesCacheRefreshCmd = &cobra.Command{
		Use:   "refresh",
		Short: "Refresh the cached update report",
		Long:  `Identical to 'syschecks updates refresh'.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			checkUpdates(true, false, false)
		},
	}
)

// runUpdatesCheck resolves cached-vs-live and prints the report.
func runUpdatesCheck() {
	useCache := !updatesCheckRefresh
	// The deprecated --cache-use=false is still honoured when it was passed explicitly.
	if !updatesCacheUse {
		useCache = false
	}
	if updatesCheckCached {
		useCache = true
	}

	format := resolveOutput(outputJSON, false, updatesJsonPretty)
	emitJSON(systemUpdates(useCache), format)
}
