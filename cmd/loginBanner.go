package cmd

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syschecks/helpers"

	"github.com/Delta456/box-cli-maker/v2"
	"github.com/gookit/color"
	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// ANSI color codes
const (
	RED         = "\033[0;31m"
	LIGHT_RED   = "\033[38;5;203m"
	LIGHT_BLUE  = "\033[0;94m"
	LIGHT_GREEN = "\033[0;92m"
	LIGHT_CYAN  = "\033[0;93m"
	AMBER       = "\033[38;5;214m"
	NC          = "\033[0m"
)

var (
	noEmojies               bool
	bannerDiskUsedThreshold float64
	bannerShowAll           bool

	bannerCmd = &cobra.Command{
		Use:   "banner",
		Short: "Show system info banner",
		Long:  `Show system info banner. Intended to be used as a login banner.`,
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if bannerDiskUsedThreshold < 0 || bannerDiskUsedThreshold > 100 {
				return fmt.Errorf("--disk-used-threshold must be between 0 and 100")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			showLoginBanner(noEmojies, bannerShowAll)
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

func showLoginBanner(noEmojies bool, showAll bool) {
	config := box.Config{Px: 1, Py: 1, Type: "", AllowWrapping: true, WrappingLimit: bannerWrappingLimit()}
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
	versionStatus := displayVersion(GetShortVersion()) + " | self-update: "
	if selfUpdateEnabled(helpers.AUTOUPDATE_JOB) {
		versionStatus += LIGHT_GREEN + "ON" + NC
	} else {
		versionStatus += AMBER + "OFF" + NC
	}

	// System info section
	content.WriteString(LIGHT_BLUE + emoji("🔥", noEmojies) + "System info" + NC + "\n")
	content.WriteString(LIGHT_GREEN + emoji("💻", noEmojies) + "OS installed: " + NC + helpers.PrettyOsName() + "\n")
	content.WriteString(LIGHT_GREEN + emoji("📡", noEmojies) + "Hostname: " + NC + getHostName() + " || " + LIGHT_GREEN + "Machine IPs: " + NC + getIps() + "\n")
	content.WriteString(LIGHT_GREEN + emoji("🕓", noEmojies) + "System uptime: " + NC + getSystemUptime() + "\n")
	activeSessions := readActiveSessions()
	sessionCount := 0
	for _, sessions := range activeSessions {
		sessionCount += len(sessions)
	}
	if showAll || len(activeSessions) > 1 {
		lineColor := LIGHT_GREEN
		if len(activeSessions) > 1 {
			lineColor = AMBER
		}
		content.WriteString(lineColor + emoji("👥", noEmojies) + "Logged-in users: " + NC + fmt.Sprintf("%d (%d sessions)", len(activeSessions), sessionCount) + "\n")
	}
	content.WriteString(LIGHT_GREEN + emoji("🤖", noEmojies) + "CPU Info: " + NC + helpers.GetCpuInfoLinux() + "\n")

	ramInfo := helpers.GetRamInfoLinux()
	content.WriteString(LIGHT_GREEN + emoji("🧠", noEmojies) + "RAM Info (Used/Total): " + NC + ramInfo.Used + "/" + ramInfo.Total + "\n")

	if showAll {
		disks := diskSpacePartitions()
		if len(disks) > 0 {
			content.WriteString("\n" + LIGHT_BLUE + emoji("💾", noEmojies) + "Disk space" + NC + "\n")
			for _, disk := range disks {
				lineColor := LIGHT_GREEN
				if disk.freePercent < 100-bannerDiskUsedThreshold {
					lineColor = LIGHT_RED
				}
				content.WriteString(lineColor + "        " + disk.mountPoint + ": " + NC + fmt.Sprintf("%.1f%% free (%s available) [%s]", disk.freePercent, formatBytes(disk.availableBytes), disk.source) + "\n")
			}
		}
	} else if diskWarnings := lowDiskSpacePartitions(100 - bannerDiskUsedThreshold); len(diskWarnings) > 0 {
		content.WriteString("\n" + LIGHT_RED + emoji("💾", noEmojies) + fmt.Sprintf("Low disk space (over %.0f%% used)", bannerDiskUsedThreshold) + NC + "\n")
		for _, disk := range diskWarnings {
			content.WriteString(LIGHT_RED + "        " + disk.mountPoint + ": " + NC + fmt.Sprintf("%.1f%% free (%s available) [%s]", disk.freePercent, formatBytes(disk.availableBytes), disk.source) + "\n")
		}
	}
	content.WriteString("\n")

	kernComp := compareKernels()
	if showAll || kernComp.kernelNeedsReboot || kernComp.installedKernelCount > 6 {
		content.WriteString(LIGHT_BLUE + emoji("🔥", noEmojies) + "Kernel status" + NC + "\n")
		if kernComp.kernelNeedsReboot {
			content.WriteString(LIGHT_RED + emoji("🔴", noEmojies) + "Please reboot to apply the kernel update!" + NC + "\n")
			content.WriteString(LIGHT_RED + "        Currently active kernel:   " + NC + kernComp.runningKernel + "\n")
			content.WriteString(LIGHT_GREEN + "        Latest installed kernel:   " + NC + kernComp.latestInstalledKernel + "\n")
		} else if showAll {
			content.WriteString(LIGHT_GREEN + emoji("🌿", noEmojies) + "Running the latest installed kernel: " + NC + kernComp.runningKernel + "\n")
		}
		if kernComp.installedKernelCount > 6 {
			content.WriteString(AMBER + emoji("⚠️", noEmojies) + fmt.Sprintf("Installed kernels: %d. Cleanup recommended: `sudo syschecks kernel cleanup --keep 4`", kernComp.installedKernelCount) + NC + "\n")
		} else if showAll {
			content.WriteString(LIGHT_GREEN + emoji("🌿", noEmojies) + fmt.Sprintf("Installed kernels: %d", kernComp.installedKernelCount) + NC + "\n")
		}
		content.WriteString("\n")
	}

	// Update status is exception-only: healthy automation and zero counts stay quiet.
	var updateIssues strings.Builder
	switch currentAutomaticOSUpdateMode() {
	case automaticOSUpdatesSystem:
		if showAll {
			updateIssues.WriteString(LIGHT_GREEN + emoji("🔄", noEmojies) + "Automatic OS updates: Full system updates ON" + NC + "\n")
		}
	case automaticOSUpdatesSecurity:
		if showAll {
			updateIssues.WriteString(LIGHT_GREEN + emoji("🔄", noEmojies) + "Automatic OS updates: Security-only updates ON" + NC + "\n")
		}
	case automaticOSUpdatesConflict:
		updateIssues.WriteString(LIGHT_RED + emoji("🔄", noEmojies) + "Automatic OS updates: CONFLICT — full-system and security-only jobs are both ON" + NC + "\n")
	case automaticOSUpdatesOff:
		updateIssues.WriteString(LIGHT_RED + emoji("🔄", noEmojies) + "Automatic OS updates: OFF — no scheduled system or security updates" + NC + "\n")
	}
	sysUpdates := systemUpdates(true)

	if sysUpdates.NumberOfSystemUpdates > 0 {
		updateIssues.WriteString(LIGHT_CYAN + emoji("🔶", noEmojies) + "Number of system updates available: " + NC + strconv.Itoa(sysUpdates.NumberOfSystemUpdates) + "\n")
	} else if showAll {
		updateIssues.WriteString(LIGHT_GREEN + emoji("🌿", noEmojies) + "No new system updates available" + NC + "\n")
	}

	if sysUpdates.NumberOfSecurityUpdates > 0 {
		updateIssues.WriteString(LIGHT_RED + emoji("🛑", noEmojies) + "Number of security updates available: " + NC + strconv.Itoa(sysUpdates.NumberOfSecurityUpdates) + "\n")
	} else if showAll {
		updateIssues.WriteString(LIGHT_GREEN + emoji("🌿", noEmojies) + "No new security updates available" + NC + "\n")
	}

	if !sysUpdates.CacheUpToDate {
		updateIssues.WriteString(LIGHT_RED + emoji("🛑", noEmojies) + "Your update cache is out-of-date." + NC + " Refresh using: `sudo syschecks updates --cache-create`\n")
	} else if showAll {
		cacheStatus := "Update cache is current"
		if sysUpdates.CacheDateCreated != "" {
			cacheStatus += " (created " + sysUpdates.CacheDateCreated + ")"
		}
		updateIssues.WriteString(LIGHT_GREEN + emoji("🌿", noEmojies) + cacheStatus + NC + "\n")
	}

	if updateIssues.Len() > 0 {
		content.WriteString(LIGHT_BLUE + emoji("🔥", noEmojies) + "Update status" + NC + "\n")
		content.WriteString(updateIssues.String())
	}

	rendered := boxNew.String("", strings.TrimRight(content.String(), "\n"))
	rendered = addBannerHeader(rendered, userHello, versionStatus)
	fmt.Printf("\n%s\n", rendered)
}

func bannerWrappingLimit() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width < 60 {
		return 100
	}
	limit := width - 8
	if limit > 120 {
		return 120
	}
	return limit
}

func addBannerHeader(rendered string, leftTitle string, rightTitle string) string {
	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 {
		return rendered
	}

	insideWidth := runewidth.StringWidth(color.ClearCode(lines[0])) - 2
	left := " " + leftTitle + " "
	right := " " + rightTitle + " "
	gap := insideWidth - runewidth.StringWidth(color.ClearCode(left)) - runewidth.StringWidth(color.ClearCode(right))
	if gap < 1 {
		left = ""
		gap = insideWidth - runewidth.StringWidth(color.ClearCode(right))
	}
	if gap < 0 {
		return rendered
	}

	lines[0] = LIGHT_BLUE + "╭" + NC + left + strings.Repeat(LIGHT_BLUE+"─"+NC, gap) + right + LIGHT_BLUE + "╮" + NC
	return strings.Join(lines, "\n")
}

func selfUpdateEnabled(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	commandDefined := false
	commandScheduled := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "COMMAND=") && strings.Contains(line, "syschecks self-update") {
			commandDefined = true
			continue
		}
		if strings.Contains(line, "${COMMAND}") {
			commandScheduled = true
		}
		if strings.Contains(line, "syschecks self-update") {
			// Also support hand-written cron entries without a COMMAND variable.
			return true
		}
	}
	return commandDefined && commandScheduled
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
