package helpers

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
)

func ZabbixInit() {
	filePath1 := "/etc/zabbix/zabbix_agentd.conf"
	filePath2 := "/etc/zabbix_agentd.conf"

	lineToCheck1 := regexp.MustCompile(`.*SYSCHECKS.*`)
	lineToCheck2 := regexp.MustCompile(`.*syschecks.*`)

	filePath := ""
	_, err1 := os.Stat(filePath1)
	_, err2 := os.Stat(filePath2)

	if os.IsNotExist(err1) && os.IsNotExist(err2) {
		log.Fatal("Could not find Zabbix config file!")
	} else if os.IsNotExist(err1) {
		filePath = filePath2
	} else if os.IsNotExist(err2) {
		filePath = filePath1
	}

	// Read the contents of the Zabbix config file into memory
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	// Get rid of old syscheck integration lines
	scanner := bufio.NewScanner(file)
	lines := []string{}
	blineIndex := 0
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 1 {
			blineIndex = blineIndex + 1
		}
		if blineIndex > 1 {
			blineIndex = 0
			continue
		}
		if !lineToCheck1.MatchString(line) && !lineToCheck2.MatchString(line) {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading file:", err)
		return
	}

	// Add lines required for the integration
	if len(lines[len(lines)-1]) > 0 {
		lines = append(lines, "") // Add break only if the previous line is not a break
	}
	lines = append(lines, "#_ SYSCHECKS INTEGRATION _#")
	lines = append(lines, "UserParameter=syschecks[*],syschecks $1")

	// Write the modified contents back to the file
	file, err = os.Create(filePath)
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, line := range lines {
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			panic(err)
		}
	}
	writer.Flush()

	// Restart Zabbix Agent service using `systemd`
	out, err := exec.Command("systemctl", "restart", "zabbix-agent").CombinedOutput()
	if err != nil {
		log.Fatal("Could not restart Zabbix agent process. Exact error: " + string(out))
	}
}
