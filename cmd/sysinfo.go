package cmd

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

var (
	sysinfoCmd = &cobra.Command{
		Use:   "sysinfo",
		Short: "Show system information",
		Long:  `Show system information`,
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
