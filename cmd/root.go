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
	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Show version and exit",
		Long:  `Show version and exit.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("0.2")
			os.Exit(0)
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

	rootCmd.AddCommand(zabbixCmd)
	zabbixCmd.AddCommand(zabbixInitCmd)

	rootCmd.AddCommand(sysinfoCmd)

	rootCmd.AddCommand(userinfoCmd)

	// Apply system or security updates
	rootCmd.AddCommand(applyUpdatesCmd)
	applyUpdatesCmd.Flags().BoolVarP(&applyUpdatesCmdSystemUpdates, "system", "s", false, "Install system updates (instead of the default security updates)")
	applyUpdatesCmd.Flags().BoolVarP(&applyUpdatesCmdIgnorePackageLock, "ignore-lock-file", "i", false, "Ignore package lock file and proceed with updates")

	// Print version and exit
	rootCmd.AddCommand(versionCmd)
}
