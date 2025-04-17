package cmd

import (
	"syschecks/helpers"

	"github.com/spf13/cobra"
)

var (
	cronCmd = &cobra.Command{
		Use:   "cron",
		Short: "Create required cron jobs",
		Long:  `Create required cron jobs`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
)

var (
	cronInitCmd = &cobra.Command{
		Use:   "init",
		Short: "Create a scheduled cron job to check for system or security updates",
		Long:  `Create a scheduled cron job to check for system or security updates`,
		Run: func(cmd *cobra.Command, args []string) {
			helpers.CacheCreate()
		},
	}
)

var (
	cronSystemUpdates bool
	// cronSystemUpdatesHold bool
	cronSecurityUpdates bool

	cronUpdatesCmd = &cobra.Command{
		Use:   "updates",
		Short: "Create a scheduled cron job to execute system or security updates",
		Long:  `Create a scheduled cron job to execute system or security updates`,
		Run: func(cmd *cobra.Command, args []string) {
			if cronSystemUpdates {
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
