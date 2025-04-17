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
	// save_results bool

	kernelCmd = &cobra.Command{
		Use:   "kernel",
		Short: "Kernel reboot checks",
		Long:  `Kernel reboot checks. Returns JSON output (or pretty JSON) to display kernel related system checks.`,
		Run: func(cmd *cobra.Command, args []string) {
			kernel()
		},
		Aliases: []string{"kern"},
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
