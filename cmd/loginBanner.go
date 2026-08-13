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
			// The JSON view always reports every check, healthy ones included, so --all is
			// implied. Monitoring must be able to distinguish "passed" from "absent".
			if format := resolveOutput(outputText, false, false); format != outputText {
				emitJSON(collectBannerData(bannerDiskUsedThreshold), format)
				return
			}
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
	loginSessions := readLoginSessions()
	sessionCount := 0
	for _, sessions := range loginSessions {
		sessionCount += len(sessions)
	}
	if showAll || len(loginSessions) > 1 {
		lineColor := LIGHT_GREEN
		if len(loginSessions) > 1 {
			lineColor = AMBER
		}
		content.WriteString(lineColor + emoji("👥", noEmojies) + "Logged-in users: " + NC + fmt.Sprintf("%d (%d sessions)", len(loginSessions), sessionCount) + "\n")
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

	writeKernelStatus(&content, compareKernels(), noEmojies, showAll)

	// Update status is exception-only: healthy automation and zero counts stay quiet.
	var updateIssues strings.Builder
	updateMode := currentAutomaticOSUpdateMode()
	switch updateMode {
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

	if securityCount, supported := securityUpdateCount(sysUpdates); !supported {
		if shouldWarnUnsupportedSecurity(supported, updateMode) {
			updateIssues.WriteString(LIGHT_RED + emoji("🛑", noEmojies) + "Security update status: UNSUPPORTED — apk has no security-only channel" + NC + "\n")
		}
	} else if securityCount > 0 {
		updateIssues.WriteString(LIGHT_RED + emoji("🛑", noEmojies) + "Number of security updates available: " + NC + strconv.Itoa(securityCount) + "\n")
	} else if showAll {
		updateIssues.WriteString(LIGHT_GREEN + emoji("🌿", noEmojies) + "No new security updates available" + NC + "\n")
	}

	writeRepositoryIssues(&updateIssues, sysUpdates.RepositoryIssues, noEmojies, showAll)

	if !sysUpdates.CacheUpToDate {
		updateIssues.WriteString(LIGHT_RED + emoji("🛑", noEmojies) + "Your update cache is out-of-date" + NC + "\n")
		updateIssues.WriteString(LIGHT_RED + "        Run: " + NC + "sudo syschecks updates --cache-create\n")
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

// Unsupported security-only data is expected on apk and is not itself a banner fault.
// Surface it only when a security-only cron job is actually present (for example a legacy
// or manually-written job); attempts made through `schedule enable` already fail loudly.
func shouldWarnUnsupportedSecurity(supported bool, updateMode automaticOSUpdateMode) bool {
	return !supported && updateMode == automaticOSUpdatesSecurity
}

// kernelCleanupThreshold is the installed-kernel count above which cleanup is suggested.
const kernelCleanupThreshold = 6

// writeKernelStatus renders the kernel section. Advice is put on its own indented line
// rather than appended to the summary: as a single sentence the cleanup hint ran past the
// banner width on an 80-column terminal and box-cli-maker broke it mid-word.
func writeKernelStatus(out *strings.Builder, kernComp compareKernelsStruct, noEmojies bool, showAll bool) {
	needsCleanup := kernComp.installedKernelCount > kernelCleanupThreshold
	if !showAll && !kernComp.kernelNeedsReboot && !needsCleanup {
		return
	}

	out.WriteString(LIGHT_BLUE + emoji("🔥", noEmojies) + "Kernel status" + NC + "\n")
	if kernComp.kernelNeedsReboot {
		out.WriteString(LIGHT_RED + emoji("🔴", noEmojies) + "Please reboot to apply the kernel update!" + NC + "\n")
		out.WriteString(LIGHT_RED + "        Currently active kernel:   " + NC + kernComp.runningKernel + "\n")
		out.WriteString(LIGHT_GREEN + "        Latest installed kernel:   " + NC + kernComp.latestInstalledKernel + "\n")
	} else if showAll {
		out.WriteString(LIGHT_GREEN + emoji("🌿", noEmojies) + "Running the latest installed kernel: " + NC + kernComp.runningKernel + "\n")
	}

	if needsCleanup {
		out.WriteString(AMBER + emoji("🧹", noEmojies) + fmt.Sprintf("Installed kernels: %d — cleanup recommended", kernComp.installedKernelCount) + NC + "\n")
		out.WriteString(AMBER + "        Run: " + NC + "sudo syschecks kernel cleanup --keep 4\n")
	} else if showAll {
		out.WriteString(LIGHT_GREEN + emoji("🌿", noEmojies) + fmt.Sprintf("Installed kernels: %d", kernComp.installedKernelCount) + NC + "\n")
	}
	out.WriteString("\n")
}

// bannerRepoIssueLimit caps how many broken repositories are listed individually so a host
// with a widely-mirrored outage cannot push the rest of the banner off the screen.
const bannerRepoIssueLimit = 4

// writeRepositoryIssues renders failed repository refreshes. This is the only place an
// operator finds out that update counts are understated because a repository is broken.
func writeRepositoryIssues(out *strings.Builder, issues []repoIssue, noEmojies bool, showAll bool) {
	if len(issues) == 0 {
		if showAll {
			out.WriteString(LIGHT_GREEN + emoji("🌿", noEmojies) + "All repositories refreshed successfully" + NC + "\n")
		}
		return
	}

	out.WriteString(LIGHT_RED + emoji("📛", noEmojies) + fmt.Sprintf("Broken repositories: %d — update counts are incomplete", len(issues)) + NC + "\n")

	shown := issues
	if len(shown) > bannerRepoIssueLimit {
		shown = shown[:bannerRepoIssueLimit]
	}
	for _, issue := range shown {
		out.WriteString(LIGHT_RED + "        " + shortenCell(issue.Repo, 26) + ": " + NC + shortenCell(issue.Reason, 44) + "\n")
	}
	if remaining := len(issues) - len(shown); remaining > 0 {
		out.WriteString(LIGHT_RED + "        " + NC + fmt.Sprintf("... and %d more (see `syschecks updates --json-pretty`)", remaining) + "\n")
	}
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

// getIps returns the system's IP addresses formatted for display.
func getIps() string {
	ips := getIpList()
	if len(ips) == 0 {
		return "[ none ]"
	}
	return "[ " + strings.Join(ips, " ") + " ]"
}

// getIpList returns the system's IP addresses (excluding loopback and IPv6 link-local).
func getIpList() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
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

	return ips
}

// getSystemUptimeSeconds reads uptime directly from /proc/uptime (much faster than parsing
// /proc/stat). Returns 0 when unavailable.
func getSystemUptimeSeconds() float64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}

	// /proc/uptime format: "uptime_seconds idle_seconds"
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0
	}

	var uptimeSeconds float64
	if _, err := fmt.Sscanf(fields[0], "%f", &uptimeSeconds); err != nil {
		return 0
	}
	return uptimeSeconds
}

func getSystemUptime() string {
	uptimeSeconds := getSystemUptimeSeconds()
	if uptimeSeconds <= 0 {
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
