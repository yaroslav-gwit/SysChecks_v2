package cmd

import (
	"fmt"
	"net"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syschecks/helpers"

	"github.com/Delta456/box-cli-maker/v2"
	"github.com/spf13/cobra"
)

// ANSI color codes
const (
	RED         = "\033[0;31m"
	LIGHT_RED   = "\033[38;5;203m"
	LIGHT_BLUE  = "\033[0;94m"
	LIGHT_GREEN = "\033[0;92m"
	LIGHT_CYAN  = "\033[0;93m"
	NC          = "\033[0m"
)

var (
	noEmojies bool

	bannerCmd = &cobra.Command{
		Use:   "banner",
		Short: "Show system info banner",
		Long:  `Show system info banner. Intended to be used as a login banner.`,
		Run: func(cmd *cobra.Command, args []string) {
			showLoginBanner(noEmojies)
		},
	}
)

// emoji returns the emoji or empty string based on noEmojies flag
func emoji(e string, noEmojies bool) string {
	if noEmojies {
		return "  "
	}
	return e + " "
}

func showLoginBanner(noEmojies bool) {
	config := box.Config{Px: 1, Py: 1, Type: "", TitlePos: "Top", AllowWrapping: true}
	boxNew := box.Box{
		TopLeft:     LIGHT_BLUE + "╭" + NC,
		TopRight:    LIGHT_BLUE + "╮" + NC,
		BottomLeft:  LIGHT_BLUE + "╰" + NC,
		BottomRight: LIGHT_BLUE + "╯" + NC,
		Horizontal:  LIGHT_BLUE + "─" + NC,
		Vertical:    LIGHT_BLUE + "│" + NC,
		Config:      config,
	}

	// Build content efficiently with strings.Builder
	var content strings.Builder

	userHello := emoji("🚀", noEmojies) + "Welcome back, " + getUserName() + "!"

	// System info section
	content.WriteString(LIGHT_BLUE + emoji("🔥", noEmojies) + "System info" + NC + "\n")
	content.WriteString(LIGHT_GREEN + emoji("💻", noEmojies) + "OS installed: " + NC + helpers.PrettyOsName() + "\n")
	content.WriteString(LIGHT_GREEN + emoji("📡", noEmojies) + "Hostname: " + NC + getHostName() + " || " + LIGHT_GREEN + "Machine IPs: " + NC + getIps() + "\n")
	content.WriteString(LIGHT_GREEN + emoji("🕓", noEmojies) + "System uptime: " + NC + getSystemUptime() + "\n")
	content.WriteString(LIGHT_GREEN + emoji("🤖", noEmojies) + "CPU Info: " + NC + helpers.GetCpuInfoLinux() + "\n")

	ramInfo := helpers.GetRamInfoLinux()
	content.WriteString(LIGHT_GREEN + emoji("🧠", noEmojies) + "RAM Info (Used/Total): " + NC + ramInfo.Used + "/" + ramInfo.Total + "\n\n")

	// Kernel section
	content.WriteString(LIGHT_BLUE + emoji("🔥", noEmojies) + "Kernel reboot status" + NC + "\n")
	kernComp := compareKernels()
	if kernComp.kernelNeedsReboot {
		content.WriteString(LIGHT_RED + emoji("🔴", noEmojies) + "Please reboot to apply the kernel update!" + NC + "\n")
		content.WriteString(LIGHT_RED + "        Currently active kernel:   " + NC + kernComp.runningKernel + "\n")
		content.WriteString(LIGHT_GREEN + "        Latest installed kernel:   " + NC + kernComp.latestInstalledKernel)
	} else {
		content.WriteString(LIGHT_GREEN + emoji("🌿", noEmojies) + "You are running the latest available kernel: " + NC + kernComp.latestInstalledKernel)
	}
	content.WriteString("\n\n")

	// Update status section
	content.WriteString(LIGHT_BLUE + emoji("🔥", noEmojies) + "Update status" + NC + "\n")
	sysUpdates := systemUpdates(true)

	if sysUpdates.NumberOfSystemUpdates > 0 {
		content.WriteString(LIGHT_CYAN + emoji("🔶", noEmojies) + "Number of system updates available: " + NC + strconv.Itoa(sysUpdates.NumberOfSystemUpdates) + "\n")
	} else {
		content.WriteString(LIGHT_GREEN + emoji("🌿", noEmojies) + "No new system updates available" + NC + "\n")
	}

	if sysUpdates.NumberOfSecurityUpdates > 0 {
		content.WriteString(LIGHT_RED + emoji("🛑", noEmojies) + "Number of security updates available: " + NC + strconv.Itoa(sysUpdates.NumberOfSecurityUpdates))
	} else {
		content.WriteString(LIGHT_GREEN + emoji("🌿", noEmojies) + "No new security updates available" + NC)
	}

	if !sysUpdates.CacheUpToDate {
		content.WriteString("\n\n")
		content.WriteString(LIGHT_RED + emoji("🛑", noEmojies) + "Your update cache is out-of-date." + NC + " Refresh using: `sudo syschecks updates --cache-create`")
	}

	boxNew.Println(userHello, content.String())
}

// getHostName returns the system hostname using native Go
func getHostName() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// getUserName returns the current user's name using native Go
func getUserName() string {
	// Try to get the current user
	currentUser, err := user.Current()
	if err == nil {
		return currentUser.Username
	}

	// Fallback to environment variable
	if name := os.Getenv("USER"); name != "" {
		return name
	}
	if name := os.Getenv("LOGNAME"); name != "" {
		return name
	}

	return "user"
}

// getIps returns the system's IP addresses (excluding loopback and IPv6 link-local)
func getIps() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "[ unknown ]"
	}

	var ips []string
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok {
			ip := ipNet.IP

			// Skip loopback and link-local addresses
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				continue
			}

			// Only include IPv4 or global IPv6
			if ip4 := ip.To4(); ip4 != nil {
				ips = append(ips, ip4.String())
			} else if ip.IsGlobalUnicast() {
				ips = append(ips, ip.String())
			}
		}

		// Limit to 3 IPs for display
		if len(ips) >= 3 {
			break
		}
	}

	if len(ips) == 0 {
		return "[ none ]"
	}

	return "[ " + strings.Join(ips, " ") + " ]"
}

// getSystemUptime reads uptime directly from /proc/uptime (much faster than parsing /proc/stat)
func getSystemUptime() string {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return "unknown"
	}

	// /proc/uptime format: "uptime_seconds idle_seconds"
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return "unknown"
	}

	// Parse uptime in seconds (as float)
	uptimeStr := fields[0]
	var uptimeSeconds float64
	if _, err := fmt.Sscanf(uptimeStr, "%f", &uptimeSeconds); err != nil {
		return "unknown"
	}

	// Convert to duration components
	totalSeconds := int(uptimeSeconds)
	days := totalSeconds / 86400
	hours := (totalSeconds % 86400) / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
}
