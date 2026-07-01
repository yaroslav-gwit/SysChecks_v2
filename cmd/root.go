package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "syschecks",
		Short: "A set of system checks extending Zabbix functionality",
		Long: `A set of system checks, mainly extending Zabbix functionality.
Includes reboot checks to apply kernel updates, pretty SSH login banner, Zabbix config generator,
system and security updates checker, and some other cool things.`,
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

// Initialize the whole CLI app
func init() {
	rootCmd.AddCommand(kernelCmd)
	kernelCmd.Flags().BoolVar(&kernelJsonPretty, "json-pretty", false, "Use JSON pretty (human readable) output.")

	kernelCmd.AddCommand(kernelCleanupCmd)
	kernelCleanupCmd.Flags().IntVar(&kernelNumberToKeep, "keep", 4, "Number of kernels to keep (including the running kernel)")

	rootCmd.AddCommand(updatesCmd)
	updatesCmd.Flags().BoolVar(&updatesJsonPretty, "json-pretty", false, "Use JSON pretty (human readable) output.")
	updatesCmd.Flags().BoolVar(&updatesCacheCreate, "cache-create", false, "Create updates cache file in JSON format for future use")
	updatesCmd.Flags().BoolVar(&updatesCacheUse, "cache-use", true, "Use cache created in advance for instant results")

	rootCmd.AddCommand(bannerCmd)
	bannerCmd.Flags().BoolVarP(&noEmojies, "no-emojies", "n", false, "Disable emoji output")

	rootCmd.AddCommand(cronCmd)
	cronCmd.AddCommand(cronInitCmd)
	cronCmd.AddCommand(cronUpdatesCmd)
	cronUpdatesCmd.Flags().BoolVarP(&cronSecurityUpdates, "security", "", false, "Enable automatic security updates (using a cron job)")
	cronUpdatesCmd.Flags().BoolVarP(&cronSystemUpdates, "system", "", false, "Enable automatic system updates (using a cron job)")
	// cronUpdatesCmd.Flags().BoolVarP(&cronSystemUpdatesHold, "system-hold", "", false, "Enable automatic system updates, but hold back docker and Nvidia packages")
	cronCmd.AddCommand(cronAutoUpdateCmd)
	cronAutoUpdateCmd.Flags().BoolVar(&cronAutoUpdateDisable, "disable", false, "Remove the auto-update cron job")

	rootCmd.AddCommand(zabbixCmd)
	zabbixCmd.AddCommand(zabbixInitCmd)

	rootCmd.AddCommand(sysinfoCmd)

	rootCmd.AddCommand(userinfoCmd)
	userinfoCmd.Flags().BoolVar(&userinfoJSON, "json", false, "Output user information as JSON")
	userinfoCmd.Flags().BoolVar(&userinfoJSONPretty, "json-pretty", false, "Output user information as pretty JSON")
	userinfoCmd.Flags().BoolVar(&userinfoAllUsers, "all", false, "Include system and no-login users")

	// Apply system or security updates
	rootCmd.AddCommand(applyUpdatesCmd)
	applyUpdatesCmd.Flags().BoolVarP(&applyUpdatesCmdSystemUpdates, "system", "s", false, "Install system updates (instead of the default security updates)")
	applyUpdatesCmd.Flags().BoolVarP(&applyUpdatesCmdIgnorePackageLock, "ignore-lock-file", "i", false, "Ignore package lock file and proceed with updates")

	// Print version and exit
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().BoolVarP(&versionVerbose, "verbose", "v", false, "Show detailed version information")

	// Self-update from the latest GitHub release
	rootCmd.AddCommand(selfUpdateCmd)
	selfUpdateCmd.Flags().BoolVar(&selfUpdateCheckOnly, "check", false, "Only check whether an update is available; do not install")
	selfUpdateCmd.Flags().BoolVar(&selfUpdateForce, "force", false, "Reinstall the latest release even if the version already matches")
}
