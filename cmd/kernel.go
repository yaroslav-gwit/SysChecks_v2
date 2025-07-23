package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"

	"github.com/facette/natsort"
	"github.com/spf13/cobra"
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
	if kernelJsonPretty {
		jsonMarshalIndent, err := json.MarshalIndent(kernelJsonOutput(), "", "   ")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(jsonMarshalIndent))
	} else {
		jsonMarshal, err := json.Marshal(kernelJsonOutput())
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(jsonMarshal))
	}
}

func getRunningKernel() string {
	app := "uname"
	arg0 := "-r"

	cmd := exec.Command(app, arg0)
	stdout, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	result := strings.TrimSpace(string(stdout))

	cleanupRepl1, _ := regexp.Compile(`\.el7.*`)
	cleanupRepl2, _ := regexp.Compile(`\.el8.*`)
	result = cleanupRepl1.ReplaceAllString(result, "")
	result = cleanupRepl2.ReplaceAllString(result, "")

	return result
}

type installedKernelsStruct struct {
	genericKernels []string
	oemKernels     []string
}

func getInstalledKernels() installedKernelsStruct {
	app := "ls"
	arg0 := "-1"
	arg1 := "/boot/"
	cmd := exec.Command(app, arg0, arg1)
	stdout, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	dirtyList := strings.Split(string(stdout), "\n")

	vmlinuzMatch, _ := regexp.Compile(`vmlinuz-.*`)
	oemMatch, _ := regexp.Compile(`.*-oem.*`)
	vmlinuzRepl, _ := regexp.Compile(`vmlinuz-`)
	cleanupRepl1, _ := regexp.Compile(`\.el7.*`)
	cleanupRepl2, _ := regexp.Compile(`\.el8.*`)
	cleanupIgnore1, _ := regexp.Compile(`.*0-rescue.*`)

	oemContinue1 := regexp.MustCompile(`System.map-.*`)
	oemContinue2 := regexp.MustCompile(`config-.*`)
	oemContinue3 := regexp.MustCompile(`initrd.img.*`)
	oemContinue4 := regexp.MustCompile(`retpoline-.*`)

	var genericResult []string
	var oemResult []string
	for _, v := range dirtyList {
		if cleanupIgnore1.MatchString(v) {
			continue
		} else if oemMatch.MatchString(v) {
			if oemContinue1.MatchString(v) || oemContinue2.MatchString(v) || oemContinue3.MatchString(v) || oemContinue4.MatchString(v) {
				continue
			}
			temp := vmlinuzRepl.ReplaceAllString(v, "")
			oemResult = append(oemResult, temp)
		} else if vmlinuzMatch.MatchString(v) {
			temp := vmlinuzRepl.ReplaceAllString(v, "")
			temp = cleanupRepl1.ReplaceAllString(temp, "")
			temp = cleanupRepl2.ReplaceAllString(temp, "")
			genericResult = append(genericResult, temp)
		}
	}
	natsort.Sort(genericResult)
	natsort.Sort(oemResult)
	finalResult := installedKernelsStruct{}
	finalResult.genericKernels = genericResult
	finalResult.oemKernels = oemResult
	return finalResult
}

type compareKernelsStruct struct {
	kernelNeedsReboot     bool
	runningKernel         string
	latestInstalledKernel string
	activeKernels         []string
}

func compareKernels() compareKernelsStruct {
	oemMatch, _ := regexp.Compile(`.*-oem.*`)

	runningKernel := getRunningKernel()
	allKernels := getInstalledKernels()
	genericKernels := allKernels.genericKernels
	oemKernels := allKernels.oemKernels

	var activeKernels []string
	if oemMatch.MatchString(runningKernel) {
		activeKernels = oemKernels
	} else {
		activeKernels = genericKernels
	}
	result := compareKernelsStruct{}
	result.activeKernels = activeKernels
	result.runningKernel = runningKernel
	result.latestInstalledKernel = activeKernels[len(activeKernels)-1]
	if runningKernel == result.latestInstalledKernel {
		result.kernelNeedsReboot = false
	} else {
		result.kernelNeedsReboot = true
	}
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
	result := kernelJsonOutputStruct{}
	result.KernelNeedsReboot = input.kernelNeedsReboot
	result.RunningKernel = input.runningKernel
	result.LatestInstalledKernel = input.latestInstalledKernel
	result.ListOfInstalledKernels = input.activeKernels
	return result
}

// kernelCleanupAction generates cleanup commands for old kernel packages
func kernelCleanupAction() {
	osType := detectOs()
	runningKernel := getRunningKernel()
	allKernels := getInstalledKernels()

	oemMatch, _ := regexp.Compile(`.*-oem.*`)

	var activeKernels []string
	if oemMatch.MatchString(runningKernel) {
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
	for i := range cleanupCommands {
		fmt.Printf("%s\n", cleanupCommands[i])
		fmt.Println()
	}
}

// getOldKernels returns a list of old kernels that can be removed, keeping the specified number of kernels
func getOldKernels(runningKernel string, installedKernels []string, numberToKeep int) []string {
	var oldKernels []string

	// Ensure we keep at least the running kernel
	if numberToKeep < 1 {
		numberToKeep = 1
	}

	// If we have fewer or equal kernels than we want to keep, return empty list
	if len(installedKernels) <= numberToKeep {
		return oldKernels
	}

	// Create a map to track which kernels to keep
	kernelsToKeep := make(map[string]bool)

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

	// Add kernels that are not in the "keep" list to the old kernels list
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
		// For Debian/Ubuntu systems
		packages := getDebianKernelPackages(oldKernels)
		if len(packages) > 0 {
			cmd := "sudo apt purge -y " + strings.Join(packages, " ")
			commands = append(commands, cmd)
		}
	} else if osType.dnf {
		// For RHEL/Fedora/AlmaLinux systems with dnf
		packages := getRHELKernelPackages(oldKernels)
		if len(packages) > 0 {
			cmd := "sudo dnf remove -y " + strings.Join(packages, " ")
			commands = append(commands, cmd)
		}
	} else if osType.yum {
		// For CentOS systems with yum
		packages := getRHELKernelPackages(oldKernels)
		if len(packages) > 0 {
			cmd := "sudo yum remove -y " + strings.Join(packages, " ")
			commands = append(commands, cmd)
		}
	}

	return commands
}

// getDebianKernelPackages maps kernel versions to Debian package names
func getDebianKernelPackages(kernelVersions []string) []string {
	var packages []string

	for _, version := range kernelVersions {
		// Check if it's an OEM kernel
		oemMatch, _ := regexp.Compile(`.*-oem.*`)
		if oemMatch.MatchString(version) {
			// OEM kernel packages
			packages = append(packages, "linux-image-"+version)
			packages = append(packages, "linux-headers-"+version)
			packages = append(packages, "linux-modules-"+version)
		} else {
			// Generic kernel packages
			packages = append(packages, "linux-image-"+version)
			packages = append(packages, "linux-headers-"+version)
			packages = append(packages, "linux-modules-"+version)
			packages = append(packages, "linux-modules-extra-"+version)
		}
	}

	return packages
}

// getRHELKernelPackages maps kernel versions to RHEL-based package names
func getRHELKernelPackages(kernelVersions []string) []string {
	var packages []string

	for _, version := range kernelVersions {
		// For RHEL-based systems, kernel packages are typically named kernel-<version>
		packages = append(packages, "kernel-"+version)
		packages = append(packages, "kernel-devel-"+version)
		packages = append(packages, "kernel-headers-"+version)
	}

	return packages
}
