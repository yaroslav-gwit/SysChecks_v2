package cmd

import (
	"syschecks/helpers"

	"github.com/spf13/cobra"
)

var (
	zabbixCmd = &cobra.Command{
		Use:   "zabbix",
		Short: "Integrate Zabbix and Syschecks",
		Long:  `Integrate Zabbix and Syschecks`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
)

var (
	zabbixInitCmd = &cobra.Command{
		Use:   "init",
		Short: "Initialize Zabbix support",
		Long:  `Initialize Zabbix support`,
		Run: func(cmd *cobra.Command, args []string) {
			helpers.ZabbixInit()
		},
	}
)
