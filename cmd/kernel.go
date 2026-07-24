package cmd

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syschecks/helpers"

	"github.com/facette/natsort"
	"github.com/spf13/cobra"
)

// Pre-compiled regex patterns for kernel operations (compiled once at package init)
var (
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
	kernelStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Report kernel version and whether a reboot is required",
		Long:  `Report the running kernel, the latest installed kernel, and whether a reboot is required.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			kernel()
		},
	}
)

var (
	kernelNumberToKeep   int
	kernelCleanupExecute bool
	kernelCleanupDryRun  bool
	kernelCleanupYes     bool

	kernelCleanupCmd = &cobra.Command{
		Use:     "cleanup",
		Short:   "Remove old kernel packages",
		Args:    cobra.NoArgs,
		Aliases: []string{"clean", "cl"},
		Long: `Remove old kernel packages, always retaining the running kernel and recent fallbacks.

This command REMOVES packages. Use --dry-run to preview what would be removed without
touching the system. Run interactively without --yes and it will list the packages and ask
for confirmation first; unattended runs (cron) proceed without prompting.`,
		Run: func(cmd *cobra.Command, args []string) {
			kernelCleanupAction()
		},
	}
)

func kernel() {
	emitJSON(kernelJsonOutput(), resolveOutput(outputJSON, false, kernelJsonPretty))
}

// getRunningKernel reads the running kernel version from /proc/version
// This is faster than spawning uname -r
func getRunningKernel() string {
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		log.Fatalf("Could not read kernel version: %v", err)
	}

	result := strings.TrimSpace(string(data))

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
		log.Printf("Warning: could not read /boot directory: %v", err)
		return installedKernelsStruct{}
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
	installedKernelCount  int
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

	installedKernelCount := len(allKernels.genericKernels) + len(allKernels.oemKernels)
	if installedKernelCount == 0 {
		// Containers and unified-kernel-image systems may not expose vmlinuz files
		// in /boot, but the running kernel still counts as one installed kernel.
		installedKernelCount = 1
	}
	result := compareKernelsStruct{
		runningKernel:        runningKernel,
		activeKernels:        activeKernels,
		installedKernelCount: installedKernelCount,
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
	InstalledKernelCount   int      `json:"installed_kernel_count"`
}

func kernelJsonOutput() kernelJsonOutputStruct {
	input := compareKernels()
	return kernelJsonOutputStruct{
		KernelNeedsReboot:      input.kernelNeedsReboot,
		RunningKernel:          input.runningKernel,
		LatestInstalledKernel:  input.latestInstalledKernel,
		ListOfInstalledKernels: input.activeKernels,
		InstalledKernelCount:   input.installedKernelCount,
	}
}

// kernelCleanupAction generates cleanup commands for old kernel packages
func kernelCleanupAction() {
	if kernelNumberToKeep < 2 {
		kernelNumberToKeep = 2
	}
	// Inverted in v1.3.0: cleanup now removes by default and --dry-run previews. The old
	// --execute is accepted and ignored so existing cron files keep working unchanged.
	if !kernelCleanupDryRun {
		helpers.RootUserCheck()
	}

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

	commandName, commandArgs := generateCleanupCommand(oldKernels, osType)
	if commandName == "" {
		fmt.Println("# No cleanup commands generated (unsupported OS or no packages found).")
		return
	}

	cleanupCommand := "sudo " + commandName + " " + strings.Join(commandArgs, " ")
	if kernelCleanupDryRun {
		fmt.Println("# Dry run — nothing was removed. The equivalent command is:")
		fmt.Printf("%s\n\n", cleanupCommand)
		return
	}

	// A human who typed `kernel cleanup` expecting the old preview behaviour gets a prompt
	// rather than a surprise. Cron has no TTY and proceeds.
	if !kernelCleanupYes && runningInteractively() && !confirmKernelCleanup(os.Stdin, len(oldKernels)) {
		fmt.Println("Aborted. Nothing was removed.")
		return
	}

	fmt.Printf("Executing: %s %s\n", commandName, strings.Join(commandArgs, " "))
	ctx, cancel := packageCommandTimeout()
	defer cancel()
	command := newCommand(ctx, commandName, commandArgs...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		log.Fatalf("Kernel cleanup failed: %v", err)
	}
	fmt.Printf("Kernel cleanup completed; retained at least %d kernels including the running kernel.\n", kernelNumberToKeep)
}

// confirmKernelCleanup requires an explicit yes. Every other answer — including a blank
// line, EOF, or an unreadable stdin — aborts, because this guards package removal.
func confirmKernelCleanup(in io.Reader, count int) bool {
	fmt.Printf("Remove %d old kernel package set(s)? [y/N]: ", count)
	answer, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && answer == "" {
		fmt.Println()
		return false
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

// getOldKernels returns a list of old kernels that can be removed, keeping the specified number of kernels
func getOldKernels(runningKernel string, installedKernels []string, numberToKeep int) []string {
	// Keep both the running kernel and a newer fallback when one exists.
	if numberToKeep < 2 {
		numberToKeep = 2
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
func generateCleanupCommand(oldKernels []string, osType detectOsStruct) (string, []string) {
	return getPackageManager(osType).KernelCleanupCommand(oldKernels)
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

// findRPMPackagesToRemove finds installed RPM packages that belong to old kernel versions.
// It queries the package database instead of assuming kernel package names.
func findRPMPackagesToRemove(versions []string) []string {
	candidates := make(map[string]bool)

	for _, version := range versions {
		for _, bootFile := range []string{
			"/boot/vmlinuz-" + version,
			"/boot/initramfs-" + version + ".img",
			"/boot/System.map-" + version,
			"/boot/config-" + version,
		} {
			cmd := exec.Command("rpm", "-qf", bootFile)
			out, err := cmd.Output()
			if err != nil {
				continue
			}
			for _, pkg := range strings.Fields(string(out)) {
				candidates[pkg] = true
			}
		}
	}

	cmd := exec.Command("rpm", "-qa")
	out, err := cmd.Output()
	if err != nil {
		log.Printf("Warning: Failed to list installed RPM packages: %v", err)
	} else {
		for _, pkg := range strings.Split(string(out), "\n") {
			pkg = strings.TrimSpace(pkg)
			if pkg == "" {
				continue
			}
			for _, version := range versions {
				if isRPMKernelPackageForVersion(pkg, version) {
					candidates[pkg] = true
					break
				}
			}
		}
	}

	packagesToRemove := make([]string, 0, len(candidates))
	for pkg := range candidates {
		packagesToRemove = append(packagesToRemove, pkg)
	}
	natsort.Sort(packagesToRemove)

	return packagesToRemove
}

func isRPMKernelPackageForVersion(pkgName string, version string) bool {
	if version == "" || !strings.Contains(pkgName, version) {
		return false
	}

	name := strings.SplitN(pkgName, "-", 2)[0]
	return name == "kernel" || strings.HasPrefix(pkgName, "kernel-")
}
