package helpers

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Pre-compiled regex patterns for better performance (compiled once at package init)
var (
	rePrettyMatch = regexp.MustCompile(`^PRETTY_NAME=`)
	reQuotesStrip = regexp.MustCompile(`"`)

	// RAM info patterns
	reMemTotal     = regexp.MustCompile(`^MemTotal:\s+(\d+)`)
	reMemAvailable = regexp.MustCompile(`^MemAvailable:\s+(\d+)`)

	// CPU info patterns
	reModelName      = regexp.MustCompile(`^Model name:\s*(.+)`)
	reThreadsPerCore = regexp.MustCompile(`^Thread\(s\) per core:\s*(\d+)`)
	reCoresPerSocket = regexp.MustCompile(`^Core\(s\) per socket:\s*(\d+)`)
	reSockets        = regexp.MustCompile(`^Socket\(s\):\s*(\d+)`)
)

// PrettyOsName returns the human-readable OS name from /etc/os-release.
func PrettyOsName() string {
	if proxmoxName := proxmoxVersionName(); proxmoxName != "" {
		return proxmoxName
	}
	return prettyOsNameFromFiles([]string{
		"/etc/os-release",
		"/usr/lib/os-release",
		"/etc/lsb-release",
	})
}

func prettyOsNameFromFile(path string) string {
	return prettyOsNameFromFiles([]string{path})
}

func prettyOsNameFromFiles(paths []string) string {
	const fallback = "Unknown Linux distribution"

	var fallbackValues map[string]string
	for _, path := range paths {
		values := readReleaseValues(path)
		if len(values) == 0 {
			continue
		}
		if hasDescriptiveReleaseName(values) {
			if name := formatReleaseValues(values); name != "" {
				return name
			}
		}
		if fallbackValues == nil {
			fallbackValues = values
		}
	}

	if fallbackValues != nil {
		if name := formatReleaseValues(fallbackValues); name != "" {
			return name
		}
	}

	if name := legacyReleaseName(); name != "" {
		return name
	}

	return fallback
}

func readReleaseValues(path string) map[string]string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		values[key] = cleanReleaseValue(value)
	}

	return values
}

func hasDescriptiveReleaseName(values map[string]string) bool {
	return values["PRETTY_NAME"] != "" ||
		values["DISTRIB_DESCRIPTION"] != "" ||
		values["NAME"] != "" ||
		values["DISTRIB_ID"] != ""
}

func formatReleaseValues(values map[string]string) string {
	if prettyName := strings.TrimSpace(values["PRETTY_NAME"]); prettyName != "" {
		return prettyName
	}

	if description := strings.TrimSpace(values["DISTRIB_DESCRIPTION"]); description != "" {
		return description
	}

	if name := strings.TrimSpace(values["NAME"]); name != "" {
		if version := strings.TrimSpace(values["VERSION"]); version != "" {
			return name + " " + version
		}
		return name
	}

	if id := strings.TrimSpace(values["ID"]); id != "" {
		if versionID := strings.TrimSpace(values["VERSION_ID"]); versionID != "" {
			return releaseDisplayName(id) + " " + versionID
		}
		return releaseDisplayName(id)
	}

	if id := strings.TrimSpace(values["DISTRIB_ID"]); id != "" {
		if release := strings.TrimSpace(values["DISTRIB_RELEASE"]); release != "" {
			return releaseDisplayName(id) + " " + release
		}
		return releaseDisplayName(id)
	}

	return ""
}

func cleanReleaseValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	value = strings.ReplaceAll(value, `\"`, `"`)
	value = strings.ReplaceAll(value, `\\`, `\`)
	return value
}

func releaseDisplayName(id string) string {
	if name, ok := releaseNameAliases[strings.ToLower(id)]; ok {
		return name
	}
	return id
}

var releaseNameAliases = map[string]string{
	"almalinux": "AlmaLinux",
	"amzn":      "Amazon Linux",
	"arch":      "Arch Linux",
	"centos":    "CentOS",
	"debian":    "Debian GNU/Linux",
	"fedora":    "Fedora Linux",
	"kali":      "Kali GNU/Linux",
	"linuxmint": "Linux Mint",
	"neon":      "KDE neon",
	"ol":        "Oracle Linux",
	"openeuler": "openEuler",
	"pop":       "Pop!_OS",
	"raspbian":  "Raspberry Pi OS",
	"rhel":      "Red Hat Enterprise Linux",
	"rocky":     "Rocky Linux",
	"ubuntu":    "Ubuntu",
	"zorin":     "Zorin OS",
}

func legacyReleaseName() string {
	for _, path := range []string{
		"/etc/redhat-release",
		"/etc/centos-release",
		"/etc/almalinux-release",
		"/etc/rocky-release",
		"/etc/oracle-release",
		"/etc/debian_version",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		value := strings.TrimSpace(string(data))
		if value == "" {
			continue
		}
		if filepath.Base(path) == "debian_version" {
			return "Debian GNU/Linux " + value
		}
		return value
	}

	return ""
}

func proxmoxVersionName() string {
	path, err := exec.LookPath("pveversion")
	if err != nil {
		return ""
	}

	out, err := exec.Command(path).Output()
	if err != nil {
		return "Proxmox VE"
	}

	version := strings.TrimSpace(string(out))
	version = strings.TrimPrefix(version, "pve-manager/")
	if idx := strings.Index(version, "/"); idx >= 0 {
		version = version[:idx]
	}
	if version == "" {
		return "Proxmox VE"
	}
	return "Proxmox VE " + version
}

// RootUserCheck ensures the current process is running as root.
// Uses os.Getuid() which is faster than spawning a subprocess.
func RootUserCheck() {
	if os.Getuid() != 0 {
		log.Fatal("This subcommand can only be run as root!")
	}
}

// RamInfo holds memory usage statistics
type RamInfo struct {
	Free  string
	Used  string
	Total string
}

// GetRamInfoLinux parses /proc/meminfo to get RAM statistics.
// Uses bufio.Scanner for efficient line-by-line reading.
func GetRamInfoLinux() RamInfo {
	ramInfo := RamInfo{}

	file, err := os.Open("/proc/meminfo")
	if err != nil {
		log.Printf("Warning: Could not read /proc/meminfo: %v", err)
		return ramInfo
	}
	defer file.Close()

	var ramTotal, ramAvailable int
	found := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() && found < 2 {
		line := scanner.Text()

		if matches := reMemTotal.FindStringSubmatch(line); len(matches) > 1 {
			ramTotal, _ = strconv.Atoi(matches[1])
			found++
		} else if matches := reMemAvailable.FindStringSubmatch(line); len(matches) > 1 {
			ramAvailable, _ = strconv.Atoi(matches[1])
			found++
		}
	}

	// Convert from KB to GB
	ramInfo.Total = fmt.Sprintf("%.2fG", float64(ramTotal)/1024/1024)
	ramInfo.Free = fmt.Sprintf("%.2fG", float64(ramAvailable)/1024/1024)
	ramInfo.Used = fmt.Sprintf("%.2fG", float64(ramTotal-ramAvailable)/1024/1024)

	return ramInfo
}

// GetCpuInfoLinux parses /proc/cpuinfo and /sys/devices for CPU information.
// Falls back to lscpu if needed.
func GetCpuInfoLinux() string {
	// Try reading from /proc/cpuinfo first for model name
	modelName := getCPUModelName()
	cores, threads, sockets := getCPUTopology()

	// Clean up model name
	modelName = strings.ReplaceAll(modelName, "(R)", "")
	modelName = strings.ReplaceAll(modelName, "(TM)", "")
	modelName = strings.TrimSpace(modelName)

	// Remove trailing frequency if present (e.g., "@ 2.60GHz")
	if idx := strings.Index(modelName, " @"); idx > 0 {
		modelName = modelName[:idx]
	}

	return fmt.Sprintf("%s - Sockets: %d, Cores: %d, Threads: %d", modelName, sockets, cores, threads)
}

// getCPUModelName reads the CPU model name from /proc/cpuinfo
func getCPUModelName() string {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "Unknown CPU"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	return "Unknown CPU"
}

// getCPUTopology returns cores per socket, total threads, and socket count
func getCPUTopology() (cores, threads, sockets int) {
	// Default values
	cores, threads, sockets = 1, 1, 1

	// Try reading from /sys/devices/system/cpu/
	if data, err := os.ReadFile("/sys/devices/system/cpu/online"); err == nil {
		// Parse online CPU range (e.g., "0-7" means 8 CPUs)
		cpuRange := strings.TrimSpace(string(data))
		if parts := strings.Split(cpuRange, "-"); len(parts) == 2 {
			if end, err := strconv.Atoi(parts[1]); err == nil {
				threads = end + 1
			}
		} else if cpuNum, err := strconv.Atoi(cpuRange); err == nil {
			threads = cpuNum + 1
		}
	}

	// Try to get physical core count
	if data, err := os.ReadFile("/sys/devices/system/cpu/cpu0/topology/core_siblings_list"); err == nil {
		siblings := strings.TrimSpace(string(data))
		if parts := strings.Split(siblings, "-"); len(parts) == 2 {
			if end, err := strconv.Atoi(parts[1]); err == nil {
				// This gives us threads per socket
				threadsPerSocket := end + 1
				sockets = threads / threadsPerSocket
				if sockets < 1 {
					sockets = 1
				}
			}
		}
	}

	// Estimate cores (threads / 2 for hyperthreading, or equal if no HT)
	cores = threads / 2
	if cores < 1 {
		cores = threads
	}

	// If we couldn't get good values, fall back to parsing lscpu
	if threads <= 1 {
		return getCPUTopologyFromLscpu()
	}

	return cores, threads, sockets
}

// getCPUTopologyFromLscpu falls back to parsing lscpu output
func getCPUTopologyFromLscpu() (cores, threads, sockets int) {
	cores, threads, sockets = 1, 1, 1

	// Read from lscpu using a file descriptor to avoid shell
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return
	}
	defer file.Close()

	// Count physical and logical processors
	physicalIDs := make(map[string]bool)
	coreIDs := make(map[string]bool)
	processors := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "processor") {
			processors++
		} else if strings.HasPrefix(line, "physical id") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				physicalIDs[strings.TrimSpace(parts[1])] = true
			}
		} else if strings.HasPrefix(line, "core id") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				coreIDs[strings.TrimSpace(parts[1])] = true
			}
		}
	}

	threads = processors
	if len(physicalIDs) > 0 {
		sockets = len(physicalIDs)
	}
	if len(coreIDs) > 0 {
		cores = len(coreIDs)
	}

	return cores, threads, sockets
}
