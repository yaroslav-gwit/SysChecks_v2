package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "syschecks",
		Short: "A set of system checks extending Zabbix functionality",
		Long: `A set of system checks, mainly extending Zabbix functionality.
Includes reboot checks to apply kernel updates, pretty SSH login banner, Zabbix config generator,
system and security updates checker, and some other cool things.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return validateOutputFlag()
		},
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
)

var (
	versionVerbose bool

	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  `Show version information including git commit, build date, and platform details.`,
		Run: func(cmd *cobra.Command, args []string) {
			if versionVerbose {
				fmt.Println(GetFullVersion())
			} else {
				fmt.Printf("syschecks %s\n", GetVersion())
			}
		},
	}
)

// Execute the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// registerScopeFlag wires the shared --scope enum plus its completion.
func registerScopeFlag(cmd *cobra.Command, target *string, usage string) {
	cmd.Flags().StringVar(target, "scope", "", usage)
	_ = cmd.RegisterFlagCompletionFunc("scope", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return updateScopeCompletions, cobra.ShellCompDirectiveNoFileComp
	})
}

// addApplyUpdatesFlags is shared by `updates apply` and the retained `apply-updates`, so both
// spellings accept exactly the same flags.
func addApplyUpdatesFlags(cmd *cobra.Command) {
	registerScopeFlag(cmd, &applyUpdatesScope, "Which updates to install: "+strings.Join([]string{updateScopeSecurity, updateScopeSystem}, " or ")+" (default "+updateScopeSecurity+")")
	cmd.Flags().BoolVar(&applyUpdatesIgnoreLocks, "ignore-locks", false, "Ignore package.lock.json and update locked packages too")
	cmd.Flags().BoolVar(&applyUpdatesDryRun, "dry-run", false, "List what would be installed without changing anything")
	cmd.Flags().DurationVar(&applyUpdatesMaxDelay, "delay", applyUpdatesDefaultDelay, "Maximum random wait before starting an unattended run, to spread load across guests (e.g. 30m, 0 to disable)")
	cmd.Flags().BoolVar(&applyUpdatesNoDelay, "no-delay", false, "Start immediately, ignoring --delay")

	// Pre-restructure spellings. Kept working, hidden from help.
	cmd.Flags().BoolVarP(&applyUpdatesCmdSystemUpdates, "system", "s", false, "Install system updates")
	_ = cmd.Flags().MarkDeprecated("system", "use --scope system instead")
	cmd.Flags().BoolVarP(&applyUpdatesCmdIgnorePackageLock, "ignore-lock-file", "i", false, "Ignore package lock file")
	_ = cmd.Flags().MarkDeprecated("ignore-lock-file", "use --ignore-locks instead")
}

func addUsersFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&userinfoAllUsers, "all", false, "Include system and no-login users")
}

// Initialize the whole CLI app
func init() {
	// One output flag for every command, with completion on its values.
	rootCmd.PersistentFlags().StringVarP(&outputFlag, "output", "o", "", "Output format: "+strings.Join(validOutputFormats(), ", "))
	registerOutputCompletion(rootCmd)

	// ---- banner (stays top level: /etc/profile.d on every deployed host invokes it) ----
	rootCmd.AddCommand(bannerCmd)
	bannerCmd.Flags().BoolVarP(&noEmojies, "no-emojis", "n", false, "Disable emoji output")
	bannerCmd.Flags().BoolVar(&noEmojies, "no-emojies", false, "Disable emoji output")
	_ = bannerCmd.Flags().MarkDeprecated("no-emojies", "use --no-emojis instead")
	bannerCmd.Flags().BoolVar(&bannerShowAll, "all", false, "Show all banner details, including healthy and normally suppressed checks")
	bannerCmd.Flags().Float64Var(&bannerDiskUsedThreshold, "disk-used-threshold", 100-diskFreeWarningPercent, "Warn when a writable filesystem exceeds this used-space percentage")

	// ---- updates ----
	rootCmd.AddCommand(updatesCmd)
	updatesCmd.Flags().BoolVar(&updatesJsonPretty, "json-pretty", false, "Use JSON pretty (human readable) output.")
	_ = updatesCmd.Flags().MarkDeprecated("json-pretty", "use --output json-pretty instead")
	updatesCmd.Flags().BoolVar(&updatesCacheCreate, "cache-create", false, "Create updates cache file")
	_ = updatesCmd.Flags().MarkDeprecated("cache-create", "use 'syschecks updates refresh' instead")
	updatesCmd.Flags().BoolVar(&updatesCacheUse, "cache-use", true, "Use cache created in advance for instant results")
	_ = updatesCmd.Flags().MarkDeprecated("cache-use", "use 'syschecks updates check --refresh' for a live check")

	updatesCmd.AddCommand(updatesCheckCmd)
	updatesCheckCmd.Flags().BoolVar(&updatesCheckRefresh, "refresh", false, "Query the package manager instead of reading the cache")
	updatesCheckCmd.Flags().BoolVar(&updatesCheckCached, "cached", false, "Read the cached report (default)")

	updatesCmd.AddCommand(updatesApplyCmd)
	addApplyUpdatesFlags(updatesApplyCmd)

	updatesCmd.AddCommand(updatesRefreshCmd)
	updatesCmd.AddCommand(updatesCacheCmd)
	updatesCacheCmd.AddCommand(updatesCacheRefreshCmd)

	// Retained top-level spelling; deployed cron files still invoke it.
	rootCmd.AddCommand(applyUpdatesCmd)
	addApplyUpdatesFlags(applyUpdatesCmd)

	// ---- kernel ----
	rootCmd.AddCommand(kernelCmd)
	kernelCmd.Flags().BoolVar(&kernelJsonPretty, "json-pretty", false, "Use JSON pretty (human readable) output.")
	_ = kernelCmd.Flags().MarkDeprecated("json-pretty", "use --output json-pretty instead")
	kernelCmd.AddCommand(kernelStatusCmd)

	kernelCmd.AddCommand(kernelCleanupCmd)
	kernelCleanupCmd.Flags().IntVar(&kernelNumberToKeep, "keep", 4, "Number of kernels to keep (including the running kernel)")
	kernelCleanupCmd.Flags().BoolVar(&kernelCleanupDryRun, "dry-run", false, "Show what would be removed without removing anything")
	kernelCleanupCmd.Flags().BoolVarP(&kernelCleanupYes, "yes", "y", false, "Do not prompt for confirmation")
	kernelCleanupCmd.Flags().BoolVar(&kernelCleanupExecute, "execute", false, "No effect; removal is now the default")
	_ = kernelCleanupCmd.Flags().MarkDeprecated("execute", "removal is now the default; use --dry-run to preview")

	// ---- users ----
	rootCmd.AddCommand(userinfoCmd)
	addUsersFlags(userinfoCmd)
	userinfoCmd.Flags().BoolVar(&userinfoJSON, "json", false, "Output user information as JSON")
	_ = userinfoCmd.Flags().MarkDeprecated("json", "use --output json instead")
	userinfoCmd.Flags().BoolVar(&userinfoJSONPretty, "json-pretty", false, "Output user information as pretty JSON")
	_ = userinfoCmd.Flags().MarkDeprecated("json-pretty", "use --output json-pretty instead")
	userinfoCmd.AddCommand(usersListCmd)
	addUsersFlags(usersListCmd)

	// ---- schedule (was: cron) ----
	rootCmd.AddCommand(scheduleCmd)
	scheduleCmd.AddCommand(scheduleListCmd)
	scheduleCmd.AddCommand(scheduleEnableCmd)
	registerScopeFlag(scheduleEnableCmd, &scheduleScope, "For the 'updates' job: which updates to install automatically")
	scheduleEnableCmd.Flags().IntVar(&scheduleKeep, "keep", scheduleDefaultKeep, "For the 'kernel-cleanup' job: number of kernels to retain (minimum 2)")
	scheduleCmd.AddCommand(scheduleDisableCmd)

	// Pre-restructure subcommands, kept working but hidden.
	scheduleCmd.AddCommand(cronInitCmd)
	cronInitCmd.Flags().BoolVar(&cronCacheDisable, "disable", false, "Remove the update-cache refresh cron job")
	scheduleCmd.AddCommand(cronUpdatesCmd)
	cronUpdatesCmd.Flags().BoolVar(&cronSecurityUpdates, "security", false, "Enable automatic security updates")
	cronUpdatesCmd.Flags().BoolVar(&cronSystemUpdates, "system", false, "Enable automatic system updates")
	cronUpdatesCmd.Flags().BoolVar(&cronUpdatesDisable, "disable", false, "Remove all automatic updates cron jobs")
	scheduleCmd.AddCommand(cronAutoUpdateCmd)
	cronAutoUpdateCmd.Flags().BoolVar(&cronAutoUpdateDisable, "disable", false, "Remove the auto-update cron job")
	scheduleCmd.AddCommand(cronKernelCleanupCmd)
	cronKernelCleanupCmd.Flags().IntVar(&cronKernelNumberToKeep, "keep", 4, "Number of kernels to retain (minimum 2)")
	cronKernelCleanupCmd.Flags().BoolVar(&cronKernelCleanupDisable, "disable", false, "Remove the kernel cleanup cron job")

	// ---- zabbix ----
	rootCmd.AddCommand(zabbixCmd)
	zabbixCmd.AddCommand(zabbixInitCmd)

	// ---- retired, retained for compatibility ----
	rootCmd.AddCommand(sysinfoCmd)

	// ---- migration ----
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.Flags().BoolVar(&migrateApply, "apply", false, "Write the changes instead of only reporting them")

	// ---- version ----
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().BoolVarP(&versionVerbose, "verbose", "v", false, "Show detailed version information")

	// ---- self-update ----
	rootCmd.AddCommand(selfUpdateCmd)
	selfUpdateCmd.Flags().BoolVar(&selfUpdateCheckOnly, "check", false, "Only check whether an update is available; do not install")
	selfUpdateCmd.Flags().BoolVar(&selfUpdateForce, "force", false, "Reinstall the latest release even if the version already matches")
}
