package helpers

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Pre-compiled regex patterns for Zabbix config parsing
var (
	reSyschecksLine = regexp.MustCompile(`(?i)syschecks`)
)

// ZabbixInit configures Zabbix agent to integrate with syschecks
func ZabbixInit() {
	RootUserCheck()

	// Find Zabbix config file
	possiblePaths := []string{
		"/etc/zabbix/zabbix_agentd.conf",
		"/etc/zabbix_agentd.conf",
	}

	var configPath string
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			configPath = path
			break
		}
	}

	if configPath == "" {
		log.Fatal("Could not find Zabbix config file! Checked: /etc/zabbix/zabbix_agentd.conf and /etc/zabbix_agentd.conf")
	}

	// Read the config file
	file, err := os.Open(configPath)
	if err != nil {
		log.Fatalf("Error opening Zabbix config file: %v", err)
	}
	defer file.Close()

	// Process lines: remove existing syschecks integration and empty line duplicates
	var lines []string
	consecutiveBlankLines := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip lines containing syschecks (case insensitive)
		if reSyschecksLine.MatchString(line) {
			continue
		}

		// Track consecutive blank lines to avoid duplicates
		if len(line) == 0 {
			consecutiveBlankLines++
			if consecutiveBlankLines > 1 {
				continue
			}
		} else {
			consecutiveBlankLines = 0
		}

		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading Zabbix config file: %v", err)
	}

	// Add syschecks integration lines
	if len(lines) > 0 && len(lines[len(lines)-1]) > 0 {
		lines = append(lines, "") // Add blank line if last line isn't blank
	}
	lines = append(lines, "#_ SYSCHECKS INTEGRATION _#")
	lines = append(lines, "UserParameter=syschecks[*],syschecks $1")
	lines = append(lines, "") // Trailing newline

	// Write the modified config back
	outFile, err := os.Create(configPath)
	if err != nil {
		log.Fatalf("Error creating Zabbix config file: %v", err)
	}
	defer outFile.Close()

	writer := bufio.NewWriter(outFile)
	for _, line := range lines {
		if _, err := writer.WriteString(line + "\n"); err != nil {
			log.Fatalf("Error writing to Zabbix config file: %v", err)
		}
	}

	if err := writer.Flush(); err != nil {
		log.Fatalf("Error flushing Zabbix config file: %v", err)
	}

	fmt.Println("Zabbix configuration updated successfully")

	// Restart Zabbix agent with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "systemctl", "restart", "zabbix-agent")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Fatal("Zabbix agent restart timed out after 30 seconds")
		}
		log.Fatalf("Could not restart Zabbix agent: %s", strings.TrimSpace(string(out)))
	}

	fmt.Println("Zabbix agent restarted successfully")
}
