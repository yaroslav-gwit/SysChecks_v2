package cmd

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

var (
	sysinfoCmd = &cobra.Command{
		Use:        "sysinfo",
		Short:      "Show system information",
		Hidden:     true,
		Deprecated: "use `syschecks banner --output json`, which reports this and every other banner check",
		Long: `Show system information.

Retired: everything this printed is part of ` + "`syschecks banner --output json`" + `, which
also reports CPU, RAM, disks, kernel state, update counts and repository health. The
ip_address_list field is preserved there under the same name.`,
		Run: func(cmd *cobra.Command, args []string) {
			ipsMap := map[string]string{
				"ip_address_list": getIps(),
			}
			jsonData, err := json.Marshal(ipsMap)
			if err != nil {
				log.Fatal("Error marshaling data to JSON:", err)
			}
			fmt.Println(string(jsonData))
		},
	}
)
