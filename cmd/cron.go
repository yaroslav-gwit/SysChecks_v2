package cmd

import (
	"fmt"
	"syschecks/helpers"

	"github.com/spf13/cobra"
)

var (
	cronCmd = &cobra.Command{
		Use:   "cron",
		Short: "Manage scheduled syschecks jobs",
		Long:  `Show the cron job dashboard, or create, replace, and remove scheduled syschecks jobs.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			printCronStatus()
		},
	}

	cronStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show available cron jobs, current state, and schedules",
		Long:  `Show every syschecks cron job, whether it is enabled, and its configured execution schedule in server local time.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			printCronStatus()
		},
	}
)

var (
	cronKernelNumberToKeep   int
	cronKernelCleanupDisable bool

	cronKernelCleanupCmd = &cobra.Command{
		Use:   "kernels",
		Short: "Schedule safe cleanup of old kernel packages",
		Long:  `Create a weekly cron job that removes old kernel packages while retaining the running kernel and recent fallbacks. Use --disable to remove it.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if cronKernelCleanupDisable {
				helpers.KernelCleanupDisable()
				return
			}
			helpers.KernelCleanupEnable(cronKernelNumberToKeep)
		},
	}
)

var (
	cronCacheDisable bool

	cronInitCmd = &cobra.Command{
		Use:   "init",
		Short: "Manage the scheduled update-cache refresh",
		Long:  `Create the scheduled update-cache refresh job. Use --disable to remove it.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if cronCacheDisable {
				helpers.CacheDisable()
				return
			}
			helpers.CacheCreate()
		},
	}
)

var (
	cronSystemUpdates bool
	// cronSystemUpdatesHold bool
	cronSecurityUpdates bool
	cronUpdatesDisable  bool

	cronUpdatesCmd = &cobra.Command{
		Use:   "updates",
		Short: "Create a scheduled cron job to execute system or security updates",
		Long:  `Enable one automatic update mode, or use --disable to remove all automatic update jobs. Security-only and full-system modes are mutually exclusive.`,
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			selected := 0
			for _, enabled := range []bool{cronSystemUpdates, cronSecurityUpdates, cronUpdatesDisable} {
				if enabled {
					selected++
				}
			}
			if selected > 1 {
				return fmt.Errorf("choose exactly one of --security, --system, or --disable")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			if cronUpdatesDisable {
				helpers.UpdatesDisable()
			} else if cronSystemUpdates {
				helpers.SystemUpdates()
				// } else if cronSystemUpdatesHold {
				// 	helpers.SystemUpdatesHold()
			} else if cronSecurityUpdates {
				helpers.SecurityUpdates()
			} else {
				cmd.Help()
			}
		},
	}
)

var (
	cronAutoUpdateDisable bool

	cronAutoUpdateCmd = &cobra.Command{
		Use:   "autoupdate",
		Short: "Create a scheduled cron job that keeps syschecks updated to the latest release",
		Long:  `Create a scheduled cron job that updates syschecks to the latest GitHub release. Use --disable to remove it.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if cronAutoUpdateDisable {
				helpers.AutoUpdateDisable()
			} else {
				helpers.AutoUpdateEnable()
			}
		},
	}
)
