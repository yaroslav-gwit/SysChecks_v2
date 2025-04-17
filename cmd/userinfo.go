package cmd

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

var (
	userinfoCmd = &cobra.Command{
		Use:   "userinfo",
		Short: "List system users",
		Long:  `List system users`,
		Run: func(cmd *cobra.Command, args []string) {
			usersMap := []map[string]string{}
			jsonData, err := json.Marshal(usersMap)
			if err != nil {
				log.Fatal("Error marshaling data to JSON:", err)
			}
			fmt.Println(string(jsonData))
		},
	}
)
