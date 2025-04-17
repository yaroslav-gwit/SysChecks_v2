package helpers

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func PrettyOsName() string {
	osSorry := "Sorry, could not pick up the OS name"
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return osSorry
	}

	osReleaseFile := string(data)
	rePrettyMatch, _ := regexp.Compile(`^PRETTY_NAME=.*`)
	rePrettyStrip, _ := regexp.Compile(`^PRETTY_NAME=`)
	reQuotesStrip, _ := regexp.Compile(`"`)
	var osPretty string
	for _, item := range strings.Split(osReleaseFile, "\n") {
		if rePrettyMatch.MatchString(item) {
			osPretty = rePrettyStrip.ReplaceAllString(item, "")
			osPretty = reQuotesStrip.ReplaceAllString(osPretty, "")
		}
	}

	if len(osPretty) > 0 {
		return osPretty
	} else {
		return osSorry
	}
}

func RootUserCheck() {
	command := "whoami"
	cmd := exec.Command(command)
	std, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	userName := strings.TrimSpace(string(std))
	if userName != "root" {
		log.Fatal("This subcommand can only be run as root!")
	}
}

type RamInfo struct {
	Free  string
	Used  string
	Total string
}

func GetRamInfoLinux() RamInfo {
	ramInfo := RamInfo{}
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		log.Fatal("Could not read /proc/meminfo: " + err.Error())
	}

	reMatch1 := regexp.MustCompile(`.*MemTotal.*`)
	reMatch2 := regexp.MustCompile(`.*MemAvailable.*`)

	reSub1 := regexp.MustCompile(`.*MemTotal.*:\s+`)
	reSub2 := regexp.MustCompile(`.*MemAvailable.*:\s+`)

	ramTotal := 0
	ramAvailable := 0
	for _, v := range strings.Split(string(data), "\n") {
		if reMatch1.MatchString(v) {
			v = reSub1.ReplaceAllString(v, "")
			v = strings.ReplaceAll(v, " kB", "")
			v = strings.TrimSpace(v)
			ramTotal, _ = strconv.Atoi(v)
			ramFloat := float64(ramTotal) / 1024 / 1024
			ramInfo.Total = fmt.Sprintf("%.2fG", ramFloat)
		}
	}

	for _, v := range strings.Split(string(data), "\n") {
		if reMatch2.MatchString(v) {
			v = reSub2.ReplaceAllString(v, "")
			v = strings.ReplaceAll(v, " kB", "")
			v = strings.TrimSpace(v)
			ramAvailable, _ = strconv.Atoi(v)
			ramFloat := float64(ramAvailable) / 1024 / 1024
			ramInfo.Free = fmt.Sprintf("%.2fG", ramFloat)
		}
	}

	ramInfo.Used = fmt.Sprintf("%.2fG", float64(ramTotal-ramAvailable)/1024/1024)
	return ramInfo
}

func GetCpuInfoLinux() string {
	std, err := exec.Command("lscpu").Output()
	if err != nil {
		log.Fatal("There was an error executing lscpu: " + err.Error())
	}

	reMatch1 := regexp.MustCompile(`^Model name:.*`)
	reMatch2 := regexp.MustCompile(`^Thread.s. per core:.*`)
	reMatch3 := regexp.MustCompile(`^Core.s. per socket:.*`)
	reMatch4 := regexp.MustCompile(`^Socket.s.:.*`)

	reSub1 := regexp.MustCompile(`^Model name:\s+`)
	reSub2 := regexp.MustCompile(`^Core.s. per socket:\s+`)
	reSub3 := regexp.MustCompile(`^Socket.s.:\s+`)
	reSub4 := regexp.MustCompile(`^Thread.s. per core:\s+`)

	modelName := ""
	cores := ""
	sockets := ""
	threads := ""

	for _, v := range strings.Split(string(std), "\n") {
		v = strings.TrimSpace(v)
		if reMatch1.MatchString(v) {
			modelName = reSub1.ReplaceAllString(v, "")
		} else if reMatch2.MatchString(v) {
			threads = reSub4.ReplaceAllString(v, "")
		} else if reMatch3.MatchString(v) {
			cores = reSub2.ReplaceAllString(v, "")
		} else if reMatch4.MatchString(v) {
			sockets = reSub3.ReplaceAllString(v, "")
		}
	}

	coresInt, _ := strconv.Atoi(cores)
	threadsInt, _ := strconv.Atoi(threads)
	threadsFinal := strconv.Itoa(threadsInt * coresInt)

	modelName = strings.ReplaceAll(modelName, " @", "")
	modelName = strings.ReplaceAll(modelName, "(R)", "")
	modelName = strings.ReplaceAll(modelName, "(TM)", "")

	// return modelName + " - " + "Socket(s): " + sockets + ", Core(s): " + cores + ", Thread(s): " + threadsFinal
	return modelName + " - " + "Sockets: " + sockets + ", Cores: " + cores + ", Threads: " + threadsFinal
}
