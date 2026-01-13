package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/facette/natsort"
	"github.com/spf13/cobra"
)

// Pre-compiled regex patterns for kernel operations (compiled once at package init)
var (
	// Kernel version cleanup patterns
	reElCleanup = regexp.MustCompile(`\.el[789].*`)

	// Kernel type detection
	reOemMatch = regexp.MustCompile(`-oem`)

	// Boot file patterns
	reVmlinuz      = regexp.MustCompile(`^vmlinuz-(.+)$`)
	reRescueKernel = regexp.MustCompile(`rescue`)
	reSystemMap    = regexp.MustCompile(`^System\.map-`)
	reConfig       = regexp.MustCompile(`^config-`)
	reInitrd       = regexp.MustCompile(`^initrd\.img`)
	reRetpoline    = regexp.MustCompile(`^retpoline-`)
)

var (
	kernelJsonPretty bool

	kernelCmd = &cobra.Command{
		Use:   "kernel",
		Short: "Kernel reboot checks",
		Long:  `Kernel reboot checks. Returns JSON output (or pretty JSON) to display kernel related system checks.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			kernel()
		},
		Aliases: []string{"kern"},
	}
)

var (
	kernelNumberToKeep int

	kernelCleanupCmd = &cobra.Command{
		Use:     "cleanup",
		Short:   "Cleanup old kernel packages",
		Long:    `Kernel cleanup command. Returns commands to clean up old kernel packages.`,
		Args:    cobra.NoArgs,
		Aliases: []string{"clean", "cl"},
		Run: func(cmd *cobra.Command, args []string) {
			kernelCleanupAction()
		},
	}
)

func kernel() {
	output := kernelJsonOutput()

	var jsonOut []byte
	var err error

	if kernelJsonPretty {
		jsonOut, err = json.MarshalIndent(output, "", "   ")
	} else {
		jsonOut, err = json.Marshal(output)
	}

	if err != nil {
		log.Fatalf("Error marshaling kernel output: %v", err)
	}
	fmt.Println(string(jsonOut))
}

// getRunningKernel reads the running kernel version from /proc/version
// This is faster than spawning uname -r
func getRunningKernel() string {
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		log.Fatalf("Could not read kernel version: %v", err)
	}

	result := strings.TrimSpace(string(data))

	// Clean up RHEL-style suffixes
	result = reElCleanup.ReplaceAllString(result, "")

	return result
}

type installedKernelsStruct struct {
	genericKernels []string
	oemKernels     []string
}

// getInstalledKernels uses native Go directory reading instead of spawning ls
func getInstalledKernels() installedKernelsStruct {
	entries, err := os.ReadDir("/boot")
	if err != nil {
		log.Fatalf("Could not read /boot directory: %v", err)
	}

	var genericKernels []string
	var oemKernels []string

	for _, entry := range entries {
		name := entry.Name()

		// Skip non-vmlinuz files
		matches := reVmlinuz.FindStringSubmatch(name)
		if len(matches) < 2 {
			continue
		}

		version := matches[1]

		// Skip rescue kernels
		if reRescueKernel.MatchString(version) {
			continue
		}

		// Skip auxiliary files
		if reSystemMap.MatchString(name) || reConfig.MatchString(name) ||
			reInitrd.MatchString(name) || reRetpoline.MatchString(name) {
			continue
		}

		// Clean up RHEL-style suffixes
		version = reElCleanup.ReplaceAllString(version, "")

		// Categorize as OEM or generic
		if reOemMatch.MatchString(version) {
			oemKernels = append(oemKernels, version)
		} else {
			genericKernels = append(genericKernels, version)
		}
	}

	// Natural sort for proper version ordering
	natsort.Sort(genericKernels)
	natsort.Sort(oemKernels)

	return installedKernelsStruct{
		genericKernels: genericKernels,
		oemKernels:     oemKernels,
	}
}

type compareKernelsStruct struct {
	kernelNeedsReboot     bool
	runningKernel         string
	latestInstalledKernel string
	activeKernels         []string
}

func compareKernels() compareKernelsStruct {
	runningKernel := getRunningKernel()
	allKernels := getInstalledKernels()

	// Select kernel list based on running kernel type
	var activeKernels []string
	if reOemMatch.MatchString(runningKernel) {
		activeKernels = allKernels.oemKernels
	} else {
		activeKernels = allKernels.genericKernels
	}

	result := compareKernelsStruct{
		runningKernel: runningKernel,
		activeKernels: activeKernels,
	}

	// Handle edge case: no kernels found
	if len(activeKernels) == 0 {
		result.latestInstalledKernel = runningKernel
		result.kernelNeedsReboot = false
		return result
	}

	result.latestInstalledKernel = activeKernels[len(activeKernels)-1]
	result.kernelNeedsReboot = runningKernel != result.latestInstalledKernel

	return result
}

type kernelJsonOutputStruct struct {
	KernelNeedsReboot      bool     `json:"reboot_required"`
	RunningKernel          string   `json:"running_kernel,omitempty"`
	LatestInstalledKernel  string   `json:"latest_installed_kernel,omitempty"`
	ListOfInstalledKernels []string `json:"list_of_installed_kernels,omitempty"`
}

func kernelJsonOutput() kernelJsonOutputStruct {
	input := compareKernels()
	return kernelJsonOutputStruct{
		KernelNeedsReboot:      input.kernelNeedsReboot,
		RunningKernel:          input.runningKernel,
		LatestInstalledKernel:  input.latestInstalledKernel,
		ListOfInstalledKernels: input.activeKernels,
	}
}

// kernelCleanupAction generates cleanup commands for old kernel packages
func kernelCleanupAction() {
	osType := detectOs()
	runningKernel := getRunningKernel()
	allKernels := getInstalledKernels()

	var activeKernels []string
	if reOemMatch.MatchString(runningKernel) {
		activeKernels = allKernels.oemKernels
	} else {
		activeKernels = allKernels.genericKernels
	}

	oldKernels := getOldKernels(runningKernel, activeKernels, kernelNumberToKeep)

	if len(oldKernels) == 0 {
		fmt.Printf("# No old kernels found to clean up. Keeping %d kernel(s) as requested.\n", kernelNumberToKeep)
		return
	}

	fmt.Printf("# Found %d old kernel(s) that can be cleaned up (keeping %d kernel(s)):\n", len(oldKernels), kernelNumberToKeep)
	for _, kernel := range oldKernels {
		fmt.Printf(" # %s\n", kernel)
	}
	fmt.Println()

	cleanupCommands := generateCleanupCommands(oldKernels, osType)
	if len(cleanupCommands) == 0 {
		fmt.Println("# No cleanup commands generated (unsupported OS or no packages found).")
		return
	}

	fmt.Println("# Run the following commands to clean up old kernels:")
	for _, cmd := range cleanupCommands {
		fmt.Printf("%s\n\n", cmd)
	}
}

// getOldKernels returns a list of old kernels that can be removed, keeping the specified number of kernels
func getOldKernels(runningKernel string, installedKernels []string, numberToKeep int) []string {
	// Ensure we keep at least the running kernel
	if numberToKeep < 1 {
		numberToKeep = 1
	}

	// If we have fewer or equal kernels than we want to keep, return empty list
	if len(installedKernels) <= numberToKeep {
		return nil
	}

	// Create a map to track which kernels to keep
	kernelsToKeep := make(map[string]bool, numberToKeep)

	// Always keep the running kernel
	kernelsToKeep[runningKernel] = true
	remainingToKeep := numberToKeep - 1

	// Keep the newest kernels (from the end of the sorted list)
	for i := len(installedKernels) - 1; i >= 0 && remainingToKeep > 0; i-- {
		kernel := installedKernels[i]
		if !kernelsToKeep[kernel] {
			kernelsToKeep[kernel] = true
			remainingToKeep--
		}
	}

	// Collect kernels that are not in the "keep" list
	oldKernels := make([]string, 0, len(installedKernels)-numberToKeep)
	for _, kernel := range installedKernels {
		if !kernelsToKeep[kernel] {
			oldKernels = append(oldKernels, kernel)
		}
	}

	return oldKernels
}

// generateCleanupCommands creates appropriate package manager commands for kernel cleanup
func generateCleanupCommands(oldKernels []string, osType detectOsStruct) []string {
	var commands []string

	if osType.deb {
		packages := findDebianPackagesToRemove(oldKernels)
		if len(packages) > 0 {
			commands = append(commands, "sudo apt purge -y "+strings.Join(packages, " "))
		}
	} else if osType.dnf {
		packages := getRHELKernelPackages(oldKernels)
		if len(packages) > 0 {
			commands = append(commands, "sudo dnf remove -y "+strings.Join(packages, " "))
		}
	} else if osType.yum {
		packages := getRHELKernelPackages(oldKernels)
		if len(packages) > 0 {
			commands = append(commands, "sudo yum remove -y "+strings.Join(packages, " "))
		}
	}

	return commands
}

// findDebianPackagesToRemove finds installed packages matching the old kernel versions
// supporting both standard Debian/Ubuntu and Proxmox package naming
func findDebianPackagesToRemove(versions []string) []string {
	// Get all installed packages
	cmd := exec.Command("dpkg-query", "-W", "-f=${Package}\n")
	out, err := cmd.Output()
	if err != nil {
		log.Printf("Warning: Failed to list installed packages: %v", err)
		return nil
	}

	installedSet := make(map[string]bool)
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if line != "" {
			installedSet[line] = true
		}
	}

	var packagesToRemove []string
	// Prefixes for kernel packages (Standard Debian/Ubuntu and Proxmox)
	prefixes := []string{
		"linux-image-",
		"linux-headers-",
		"linux-modules-",
		"linux-modules-extra-",
		"pve-kernel-",      // Old Proxmox
		"pve-headers-",     // Old Proxmox
		"proxmox-kernel-",  // New Proxmox
		"proxmox-headers-", // New Proxmox
	}

	for pkgName := range installedSet {
		for _, version := range versions {
			matched := false
			for _, prefix := range prefixes {
				// Check if package starts with a valid prefix AND contains the specific version string
				if strings.HasPrefix(pkgName, prefix) && strings.Contains(pkgName, version) {
					matched = true
					break
				}
			}
			if matched {
				packagesToRemove = append(packagesToRemove, pkgName)
				break
			}
		}
	}

	// Sort explicitly to output deterministic lists
	natsort.Sort(packagesToRemove)

	return packagesToRemove
}

// getRHELKernelPackages maps kernel versions to RHEL-based package names
func getRHELKernelPackages(kernelVersions []string) []string {
	packages := make([]string, 0, len(kernelVersions)*3)

	for _, version := range kernelVersions {
		packages = append(packages,
			"kernel-"+version,
			"kernel-devel-"+version,
			"kernel-headers-"+version,
		)
	}

	return packages
}
