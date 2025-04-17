package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syschecks/helpers"
	"time"

	"github.com/Delta456/box-cli-maker/v2"
	"github.com/spf13/cobra"
)

const RED = "\033[0;31m"
const LIGHT_RED = "\033[38;5;203m"
const LIGHT_BLUE = "\033[0;94m"
const LIGHT_GREEN = "\033[0;92m"
const LIGHT_CYAN = "\033[0;93m"
const NC = "\033[0m"

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

func showLoginBanner(noEmojies bool) {
	config := box.Config{Px: 1, Py: 1, Type: "", TitlePos: "Top", AllowWrapping: true}
	boxNew := box.Box{TopLeft: LIGHT_BLUE + "╭" + NC, TopRight: LIGHT_BLUE + "╮" + NC, BottomLeft: LIGHT_BLUE + "╰" + NC, BottomRight: LIGHT_BLUE + "╯" + NC, Horizontal: LIGHT_BLUE + "─" + NC, Vertical: LIGHT_BLUE + "│" + NC, Config: config}
	userHello := ""
	if noEmojies {
		userHello = "Welcome back, " + getUserName() + "!"
	} else {
		userHello = "🚀 Welcome back, " + getUserName() + "!"
	}

	boxContent := ""
	if noEmojies {
		boxContent = LIGHT_BLUE + "System info" + NC
	} else {
		boxContent = LIGHT_BLUE + "🔥 System info 🔥" + NC
	}
	boxContent = boxContent + "\n"

	if noEmojies {
		boxContent = boxContent + LIGHT_GREEN + "   OS installed: " + NC + helpers.PrettyOsName()
	} else {
		boxContent = boxContent + LIGHT_GREEN + "💻 OS installed: " + NC + helpers.PrettyOsName()
	}
	boxContent = boxContent + "\n"

	if noEmojies {
		boxContent = boxContent + LIGHT_GREEN + "   Hostname: " + NC + getHostName() + " || " + LIGHT_GREEN + "Machine IPs: " + NC + getIps()
		boxContent = boxContent + "\n"
		boxContent = boxContent + LIGHT_GREEN + "   System uptime: " + NC + getSystemUptime()
	} else {
		boxContent = boxContent + LIGHT_GREEN + "📡 Hostname: " + NC + getHostName() + " || " + LIGHT_GREEN + "Machine IPs: " + NC + getIps()
		boxContent = boxContent + "\n"
		boxContent = boxContent + LIGHT_GREEN + "🕓 System uptime: " + NC + getSystemUptime()
	}
	boxContent = boxContent + "\n"

	cpuInfo := helpers.GetCpuInfoLinux()
	if noEmojies {
		boxContent = boxContent + LIGHT_GREEN + "   CPU Info: " + NC + cpuInfo
	} else {
		boxContent = boxContent + LIGHT_GREEN + "🤖 CPU Info: " + NC + cpuInfo
	}
	boxContent = boxContent + "\n"

	ramInfo := helpers.GetRamInfoLinux()
	if noEmojies {
		boxContent = boxContent + LIGHT_GREEN + "   RAM Info (Used/Total): " + NC + ramInfo.Used + "/" + ramInfo.Total
	} else {
		boxContent = boxContent + LIGHT_GREEN + "🧠 RAM Info (Used/Total): " + NC + ramInfo.Used + "/" + ramInfo.Total
	}

	boxContent = boxContent + "\n"
	boxContent = boxContent + "\n"

	if noEmojies {
		boxContent = boxContent + LIGHT_BLUE + "Kernel reboot status" + NC
	} else {
		boxContent = boxContent + LIGHT_BLUE + "🔥 Kernel reboot status 🔥" + NC
	}
	boxContent = boxContent + "\n"

	kernCompVar := compareKernels()
	if kernCompVar.kernelNeedsReboot {
		if noEmojies {
			boxContent = boxContent + LIGHT_RED + "   Please reboot to apply the kernel update!" + NC
		} else {
			boxContent = boxContent + LIGHT_RED + "🔴 Please reboot to apply the kernel update!" + NC
		}
		boxContent = boxContent + "\n"
		boxContent = boxContent + LIGHT_RED + "        Currently active kernel:   " + NC + kernCompVar.runningKernel
		boxContent = boxContent + "\n"
		boxContent = boxContent + LIGHT_GREEN + "        Latest installed kernel:   " + NC + kernCompVar.latestInstalledKernel
	} else {
		if noEmojies {
			boxContent = boxContent + LIGHT_GREEN + "   You are running the latest available kernel: " + NC + kernCompVar.latestInstalledKernel
		} else {
			boxContent = boxContent + LIGHT_GREEN + "🌿 You are running the latest available kernel: " + NC + kernCompVar.latestInstalledKernel
		}
	}

	boxContent = boxContent + "\n"
	boxContent = boxContent + "\n"

	if noEmojies {
		boxContent = boxContent + LIGHT_BLUE + "Update status" + NC
	} else {
		boxContent = boxContent + LIGHT_BLUE + "🔥 Update status 🔥" + NC
	}

	boxContent = boxContent + "\n"
	systemUpdatesVar := systemUpdates(true)
	if systemUpdatesVar.NumberOfSystemUpdates > 0 {
		if noEmojies {
			boxContent = boxContent + LIGHT_CYAN + "   Number of system updates available: " + NC + strconv.Itoa(systemUpdatesVar.NumberOfSystemUpdates)
		} else {
			boxContent = boxContent + LIGHT_CYAN + "🔶 Number of system updates available: " + NC + strconv.Itoa(systemUpdatesVar.NumberOfSystemUpdates)
		}
	} else {
		if noEmojies {
			boxContent = boxContent + LIGHT_GREEN + "   No new system updates available" + NC
		} else {
			boxContent = boxContent + LIGHT_GREEN + "🌿 No new system updates available" + NC
		}
	}

	boxContent = boxContent + "\n"
	if systemUpdatesVar.NumberOfSecurityUpdates > 0 {
		if noEmojies {
			boxContent = boxContent + LIGHT_RED + "   Number of security updates available: " + NC + strconv.Itoa(systemUpdatesVar.NumberOfSecurityUpdates)
		} else {
			boxContent = boxContent + LIGHT_RED + "🛑 Number of security updates available: " + NC + strconv.Itoa(systemUpdatesVar.NumberOfSecurityUpdates)
		}
	} else {
		if noEmojies {
			boxContent = boxContent + LIGHT_GREEN + "   No new security updates available" + NC
		} else {
			boxContent = boxContent + LIGHT_GREEN + "🌿 No new security updates available" + NC
		}
	}

	if !systemUpdatesVar.CacheUpToDate {
		boxContent = boxContent + "\n"
		boxContent = boxContent + "\n"
		if noEmojies {
			boxContent = boxContent + LIGHT_RED + "   Your update cache is out-of-date." + NC + " Refresh using: `sudo syschecks updates --cache-create`"
		} else {
			boxContent = boxContent + LIGHT_RED + "🛑 Your update cache is out-of-date." + NC + " Refresh using: `sudo syschecks updates --cache-create`"
		}
	}

	boxNew.Println(userHello, boxContent)
}

func getHostName() string {
	cmd := exec.Command("hostname")
	std, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	return strings.TrimSpace(string(std))
}

func getUserName() string {
	cmd := exec.Command("whoami")
	std, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	return strings.TrimSpace(string(std))
}

func getIps() string {
	cmd := exec.Command("hostname", "-I")
	std, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}

	var ipList []string
	reSplit1 := regexp.MustCompile(`\s+`)
	for _, v := range strings.Split(string(std), "\n") {
		for _, vv := range reSplit1.Split(v, -1) {
			if len(vv) > 0 {
				ipList = append(ipList, vv)
			}
		}
	}

	result := "[ "
	for i, v := range ipList {
		if i == len(ipList)-1 {
			result = result + v
		} else if i > 2 {
			break
		} else {
			result = result + v + " "
		}
	}
	result = result + " ]"

	return result
}

func getSystemUptime() string {
	var systemUptime string

	stdout, err := exec.Command("cat", "/proc/stat").Output()
	if err != nil {
		fmt.Println("Func getSystemUptime/systemUptime: There has been an error:", err)
		os.Exit(1)
	} else {
		systemUptime = string(stdout)
	}

	reMatch1 := regexp.MustCompile(`.*btime.*`)
	for _, v := range strings.Split(systemUptime, "\n") {
		if reMatch1.MatchString(v) {
			systemUptime = strings.Split(v, " ")[1]
			break
		}
	}

	systemUptime = strings.Replace(systemUptime, ",", "", -1)
	systemUptime = strings.Replace(systemUptime, " ", "", -1)

	systemUptimeInt, _ := strconv.ParseInt(systemUptime, 10, 64)
	unixTime := time.Unix(systemUptimeInt, 0)

	timeSince := time.Since(unixTime).Seconds()
	secondsModulus := int(timeSince) % 60.0

	minutesSince := (timeSince - float64(secondsModulus)) / 60.0
	minutesModulus := int(minutesSince) % 60.0

	hoursSince := (minutesSince - float64(minutesModulus)) / 60
	hoursModulus := int(hoursSince) % 24

	daysSince := (int(hoursSince) - hoursModulus) / 24

	result := strconv.Itoa(daysSince) + "d "
	result = result + strconv.Itoa(hoursModulus) + "h "
	result = result + strconv.Itoa(minutesModulus) + "m "
	result = result + strconv.Itoa(secondsModulus) + "s"

	return result
}
