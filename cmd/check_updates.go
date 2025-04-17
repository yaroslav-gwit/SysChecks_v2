package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syschecks/helpers"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

var (
	updatesJsonPretty  bool
	updatesCacheCreate bool
	updatesCacheUse    bool

	updatesCmd = &cobra.Command{
		Use:   "updates",
		Short: "System and security update checks",
		Long:  `System and security update checks.`,
		Run: func(cmd *cobra.Command, args []string) {
			checkUpdates(updatesCacheCreate, updatesCacheUse, updatesJsonPretty)
		},
	}
)

func checkUpdates(cacheCreate bool, cacheUse bool, jsonPretty bool) {
	if cacheCreate {
		helpers.RootUserCheck()
		jsonOut, err := json.Marshal(systemUpdates(false))
		if err != nil {
			log.Fatal(err)
		}
		cacheFileLocation := "/tmp/syscheck_updates.json"
		writeErr := os.WriteFile(cacheFileLocation, jsonOut, 0644)
		if writeErr != nil {
			log.Fatal(err)
		}
		// Hardened systems need the `chmod` command applied, so that regular users can read the cache file
		_ = exec.Command("chmod", "0644", cacheFileLocation).Run()
	} else if jsonPretty {
		jsonOutIndent, err := json.MarshalIndent(systemUpdates(updatesCacheUse), "", "   ")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(jsonOutIndent))
	} else if cacheUse {
		jsonOut, err := json.Marshal(systemUpdates(updatesCacheUse))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(jsonOut))
	}
}

type detectOsStruct struct {
	deb         bool
	dnf         bool
	yum         bool
	unsupported bool
}

func detectOs() detectOsStruct {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		log.Fatal(err)
	}

	osReleaseFile := string(data)
	reIdMatch, _ := regexp.Compile(`^ID=.*`)
	reIdStrip, _ := regexp.Compile(`^ID=`)
	reQuoteStrip, _ := regexp.Compile(`"`)
	var osType string
	osStruct := detectOsStruct{deb: false, yum: false, dnf: false, unsupported: false}

	for _, item := range strings.Split(osReleaseFile, "\n") {
		if reIdMatch.MatchString(item) {
			osType = reIdStrip.ReplaceAllString(item, "")
			osType = reQuoteStrip.ReplaceAllString(osType, "")
		}
	}

	if osType == "ubuntu" || osType == "pop" || osType == "debian" {
		osStruct.deb = true
	} else if osType == "centos" {
		osStruct.yum = true
	} else if osType == "almalinux" || osType == "ol" || osType == "rocky" || osType == "rhel" {
		osStruct.dnf = true
	} else {
		osStruct.unsupported = true
	}

	if osStruct.unsupported {
		log.Fatalf("Sorry, this OS (%s) is not yet supported!", osType)
	}

	return osStruct
}

type systemUpdatesStruct struct {
	numberOfSystemUpdates    int
	numberOfSecurityUpdates  int
	systemUpdatesAvailable   bool
	securityUpdatesAvailable bool
	systemUpdatesList        []string
	securityUpdatesList      []string
}

func dnfCheck() systemUpdatesStruct {
	helpers.RootUserCheck()

	startTheSpinner := spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithSuffix(" Running DNF related procedures"))
	startTheSpinner.Prefix = " "
	startTheSpinner.Start()

	result := systemUpdatesStruct{}
	var allUpdates []string
	var allUpdatesDirty []string
	var securityUpdates []string
	var securityUpdatesDirty []string

	// DNF cache refresh
	// Full command: dnf makecache
	// dnfCacheCommand := "dnf"
	// dnfCacheCommandArg0 := "makecache"
	cmd := exec.Command("dnf", "makecache")
	_, err := cmd.Output()
	if err != nil {
		if cmd.ProcessState.ExitCode() == 100 {
			// DNF exit code will be 100 when there are updates available and a list of the updates will be printed, 0 if not and 1 if an error occurs
			_ = 0
		} else {
			log.Fatal("DNF cache update error: ", err)
		}
	}

	// DNF System updates check
	// Full command: dnf --cacheonly check-update
	// dnfSystemCommand := "dnf"
	// dnfSystemCommandArg0 := "--cacheonly"
	// dnfSystemCommandArg1 := "check-update"
	cmd = exec.Command("dnf", "--cacheonly", "check-update")
	dnfSystemCommandStdout, err := cmd.Output()
	if err != nil {
		if cmd.ProcessState.ExitCode() == 100 {
			// DNF exit code will be 100 when there are updates available and a list of the updates will be printed, 0 if not and 1 if an error occurs
			_ = 0
		} else {
			log.Fatal("DNF system updates error: ", err)
		}
	}

	// DNF Security updates check
	// This is the previous command, but it's harder to parse the output
	// Full command: dnf --cacheonly updateinfo list updates security
	// cmd = exec.Command("dnf", "--cacheonly", "updateinfo", "list", "updates", "security")

	// Full command: dnf --cacheonly check-update --security
	cmd = exec.Command("dnf", "--cacheonly", "check-update", "--security")
	dnfSecurityCommandStdout, err := cmd.Output()
	if err != nil {
		if cmd.ProcessState.ExitCode() == 100 {
			// DNF exit code will be 100 when there are updates available and a list of the updates will be printed, 0 if not and 1 if an error occurs
			_ = 0
		} else {
			log.Fatal("DNF security updates error: ", err)
		}
	}

	// Split the output of the DNF system updates into slices
	for _, v := range strings.Split(string(dnfSystemCommandStdout), "\n") {
		v = strings.TrimSpace(v)
		if len(v) > 0 {
			allUpdatesDirty = append(allUpdatesDirty, v)
		}
	}
	// Split the output of the DNF security updates into slices
	for _, v := range strings.Split(string(dnfSecurityCommandStdout), "\n") {
		v = strings.TrimSpace(v)
		if len(v) > 0 {
			securityUpdatesDirty = append(securityUpdatesDirty, v)
		}
	}

	reMultiSpaceReplace, _ := regexp.Compile(`\s+`)
	reSecReplace, _ := regexp.Compile(`.*/Sec.\s+`)
	reMetaDataContinue, _ := regexp.Compile(`Last\s+metadata`)
	reKernelContinue, _ := regexp.Compile(`Security:\s+kernel-core`)
	reObsoleteBreak, _ := regexp.Compile(`Obsoleting\s+Packages`)
	reSrcMatch, _ := regexp.Compile(`.*\.src$`)

	// List of regex patterns
	skipPatterns := []string{
		`Updating\s+Subscription\s+Management`,
		`^Security:\s+`,
	}
	// Compile the regex patterns and store them in a slice
	var skipRegexes []*regexp.Regexp
	for _, pattern := range skipPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			log.Fatalf("Error compiling regex %q: %v\n", pattern, err)
		}
		skipRegexes = append(skipRegexes, re)
	}

	// List of regex patterns
	replacePatterns := []string{
		`^RHSA-\d+:\d+.*?\.`,
	}
	// Compile the regex patterns and store them in a slice
	var replaceRegexes []*regexp.Regexp
	for _, pattern := range replacePatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			log.Fatalf("Error compiling regex %q: %v\n", pattern, err)
		}
		replaceRegexes = append(replaceRegexes, re)
	}

OUTER:
	for _, v := range allUpdatesDirty {
		v = reMultiSpaceReplace.ReplaceAllString(v, " ")
		v = strings.ReplaceAll(v, " baseos ", "")
		v = strings.ReplaceAll(v, " appstream ", "")
		v = strings.ReplaceAll(v, " epel ", "")
		v = strings.ReplaceAll(v, " epel-source ", "")
		v = reMultiSpaceReplace.ReplaceAllString(v, " ")

		if reMetaDataContinue.MatchString(v) || reKernelContinue.MatchString(v) {
			continue
		} else if reObsoleteBreak.MatchString(v) {
			break
		}

		for _, re := range skipRegexes {
			if re.MatchString(v) {
				continue OUTER
			}
		}

		if len(v) > 0 {
			if reSrcMatch.MatchString(strings.TrimSpace(strings.Split(v, " ")[0])) {
				continue
			} else {
				allUpdates = append(allUpdates, strings.TrimSpace(v))
			}
		}
	}

OUTER_SEC:
	for _, v := range securityUpdatesDirty {
		v = reMultiSpaceReplace.ReplaceAllString(v, " ")
		v = reSecReplace.ReplaceAllString(v, "")
		if reMetaDataContinue.MatchString(v) {
			continue
		}

		for _, re := range skipRegexes {
			if re.MatchString(v) {
				continue OUTER_SEC
			}
		}

		if len(v) > 0 {
			for _, re := range replaceRegexes {
				v = re.ReplaceAllString(v, "")
			}
			securityUpdates = append(securityUpdates, v)
		}
	}

	if len(allUpdates) > 0 {
		result.numberOfSystemUpdates = len(allUpdates)
		result.systemUpdatesAvailable = true
		result.systemUpdatesList = allUpdates
	} else {
		result.numberOfSystemUpdates = 0
		result.systemUpdatesAvailable = false
		result.systemUpdatesList = []string{}
	}

	if len(securityUpdates) > 0 {
		result.numberOfSecurityUpdates = len(securityUpdates)
		result.securityUpdatesAvailable = true
		result.securityUpdatesList = securityUpdates
	} else {
		result.numberOfSecurityUpdates = 0
		result.securityUpdatesAvailable = false
		result.securityUpdatesList = []string{}
	}

	startTheSpinner.Stop()
	return result
}

func debCheck() systemUpdatesStruct {
	helpers.RootUserCheck()

	startTheSpinner := spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithSuffix(" Running APT related procedures"))
	startTheSpinner.Prefix = " "
	startTheSpinner.Start()

	result := systemUpdatesStruct{}
	var allUpdates []string
	var allUpdatesDirty []string
	var securityUpdates []string

	// APT cache refresh
	// Full command: apt-get -y update
	aptCmd := "apt-get"
	aptArg0 := "-y"
	aptArg1 := "update"
	aptUpdateOutput, err := exec.Command(aptCmd, aptArg0, aptArg1).CombinedOutput()
	if err != nil {
		errorValue := strings.TrimSpace(string(aptUpdateOutput))
		if string(errorValue[len(errorValue)-1]) != "." {
			errorValue = errorValue + "."
		}

		errorCodeSplit := strings.Split(err.Error(), " ")
		errorCode := errorCodeSplit[len(errorCodeSplit)-1]

		if errorCode == "100" {
			_ = 0
		} else {
			log.Fatal("APT cache update error: " + errorValue + " Exit code: " + errorCode + ".")
		}
	}

	// APT ALL updates check
	// Full command: apt-get dist-upgrade -s
	aptCmd = "apt"
	aptArg0 = "dist-upgrade"
	aptArg1 = "-s"
	aptSysStdout, err := exec.Command(aptCmd, aptArg0, aptArg1).CombinedOutput()
	if err != nil {
		errorValue := strings.TrimSpace(string(aptSysStdout))
		if string(errorValue[len(errorValue)-1]) != "." {
			errorValue = errorValue + "."
		}
		errorCodeSplit := strings.Split(err.Error(), " ")
		errorCode := errorCodeSplit[len(errorCodeSplit)-1]
		log.Fatal("APT get all updates list error: " + errorValue + " Exit code: " + errorCode + ".")
	}

	allUpdatesDirty = strings.Split(string(aptSysStdout), "\n")
	reMatchSysUpdate := regexp.MustCompile(`.*Inst.*`)
	reMatchSecUpdate := regexp.MustCompile(`.*security.*`)

	for _, v := range allUpdatesDirty {
		if reMatchSysUpdate.MatchString(v) {
			v = strings.ReplaceAll(v, "Inst ", "")
			v = strings.ReplaceAll(v, " []", "")
			v = strings.TrimSpace(v)
			allUpdates = append(allUpdates, v)
			if reMatchSecUpdate.MatchString(v) {
				securityUpdates = append(securityUpdates, v)
			}
		}
	}

	if len(allUpdates) > 0 {
		result.numberOfSystemUpdates = len(allUpdates)
		result.systemUpdatesAvailable = true
		result.systemUpdatesList = allUpdates
	} else {
		result.numberOfSystemUpdates = 0
		result.systemUpdatesAvailable = false
		result.systemUpdatesList = []string{}
	}

	if len(securityUpdates) > 0 {
		result.numberOfSecurityUpdates = len(securityUpdates)
		result.securityUpdatesAvailable = true
		result.securityUpdatesList = securityUpdates
	} else {
		result.numberOfSecurityUpdates = 0
		result.securityUpdatesAvailable = false
		result.securityUpdatesList = []string{}
	}

	startTheSpinner.Stop()
	return result
}

func yumCheck() systemUpdatesStruct {
	helpers.RootUserCheck()

	startTheSpinner := spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithSuffix(" Running YUM related procedures"))
	startTheSpinner.Prefix = " "
	startTheSpinner.Start()

	result := systemUpdatesStruct{}
	var allUpdates []string
	var allUpdatesDirty []string
	var securityUpdates []string
	var securityUpdatesDirty []string

	// YUM cache refresh
	// Full command: yum makecache fast
	yumCmd := "yum"
	yumAgr0 := "makecache"
	yumAgr1 := "fast"
	cmd := exec.Command(yumCmd, yumAgr0, yumAgr1)
	_, err := cmd.Output()
	if err != nil {
		if cmd.ProcessState.ExitCode() == 100 {
			// YUM exit code will be 100 when there are updates available and a list of the updates will be printed, 0 if not and 1 if an error occurs
			_ = 0
		} else {
			log.Fatal("YUM cache update error: ", err)
		}
	}

	// YUM System updates check
	// Full command: yum --cacheonly check-update
	yumCmd = "yum"
	yumAgr0 = "--cacheonly"
	yumAgr1 = "check-update"
	cmd = exec.Command(yumCmd, yumAgr0, yumAgr1)
	yumSysStdout, err := cmd.Output()
	if err != nil {
		if cmd.ProcessState.ExitCode() == 100 {
			// YUM exit code will be 100 when there are updates available and a list of the updates will be printed, 0 if not and 1 if an error occurs
			_ = 0
		} else {
			log.Fatal("YUM system updates error: ", err)
		}
	}

	// YUM Security updates check
	// Full command: yum --cacheonly updateinfo list updates security
	yumCmd = "yum"
	yumAgr0 = "--cacheonly"
	yumAgr1 = "updateinfo"
	yumAgr2 := "list"
	yumAgr3 := "updates"
	yumAgr4 := "security"
	cmd = exec.Command(yumCmd, yumAgr0, yumAgr1, yumAgr2, yumAgr3, yumAgr4)
	yumSecStdout, err := cmd.Output()
	if err != nil {
		if cmd.ProcessState.ExitCode() == 100 {
			// YUM exit code will be 100 when there are updates available and a list of the updates will be printed, 0 if not and 1 if an error occurs
			_ = 0
		} else {
			log.Fatal("YUM security updates error: ", err)
		}
	}

	allUpdatesDirty = strings.Split(string(yumSysStdout), "\n")
	securityUpdatesDirty = strings.Split(string(yumSecStdout), "\n")

	reMultiSpaceReplace, _ := regexp.Compile(`\s{2,}`)
	reSecReplace, _ := regexp.Compile(`.*/Sec.\s`)

	reObsoleteBreak, _ := regexp.Compile(`.*Obsoleting Packages.*`)

	reContinue1 := regexp.MustCompile(`Loaded plugins: `)
	reContinue2 := regexp.MustCompile(`updateinfo list done`)
	reContinue3 := regexp.MustCompile(`: manager,`)
	reContinue4 := regexp.MustCompile(`This system is not registered`)
	reContinue5 := regexp.MustCompile(`\s:\sversionlock`)
	reContinue6 := regexp.MustCompile(`Last metadata expiration check`)
	reContinue7 := regexp.MustCompile(`.*: subscription-manager.*`)
	reContinue8 := regexp.MustCompile(`.*: manager, versionlock.*`)
	reContinue9, _ := regexp.Compile(`.*: versionlock.*`)

	for _, v := range allUpdatesDirty {
		v = reMultiSpaceReplace.ReplaceAllString(v, " ")
		v = strings.ReplaceAll(v, " baseos ", "")
		v = strings.ReplaceAll(v, " appstream ", "")
		v = strings.ReplaceAll(v, " epel ", "")
		v = strings.ReplaceAll(v, " epel-source", "")
		if reContinue1.MatchString(v) || reContinue2.MatchString(v) || reContinue3.MatchString(v) || reContinue4.MatchString(v) || reContinue5.MatchString(v) || reContinue6.MatchString(v) || reContinue7.MatchString(v) || reContinue8.MatchString(v) || reContinue9.MatchString(v) {
			continue
		} else if reObsoleteBreak.MatchString(v) {
			break
		}
		if len(v) > 0 {
			allUpdates = append(allUpdates, strings.TrimSpace(v))
		}
	}

	for _, v := range securityUpdatesDirty {
		v = reMultiSpaceReplace.ReplaceAllString(v, "")
		v = reSecReplace.ReplaceAllString(v, "")
		if reContinue1.MatchString(v) || reContinue2.MatchString(v) || reContinue3.MatchString(v) || reContinue4.MatchString(v) || reContinue5.MatchString(v) || reContinue6.MatchString(v) || reContinue7.MatchString(v) || reContinue8.MatchString(v) || reContinue9.MatchString(v) {
			continue
		}
		if len(v) > 0 {
			securityUpdates = append(securityUpdates, strings.TrimSpace(v))
		}
	}

	if len(allUpdates) > 0 {
		result.numberOfSystemUpdates = len(allUpdates)
		result.systemUpdatesAvailable = true
		result.systemUpdatesList = allUpdates
	} else {
		result.numberOfSystemUpdates = 0
		result.systemUpdatesAvailable = false
		result.systemUpdatesList = []string{}
	}

	if len(securityUpdates) > 0 {
		result.numberOfSecurityUpdates = len(securityUpdates)
		result.securityUpdatesAvailable = true
		result.securityUpdatesList = securityUpdates
	} else {
		result.numberOfSecurityUpdates = 0
		result.securityUpdatesAvailable = false
		result.securityUpdatesList = []string{}
	}

	startTheSpinner.Stop()
	return result
}

type systemUpdatesJsonStruct struct {
	NumberOfSystemUpdates    int      `json:"system_updates"`
	NumberOfSecurityUpdates  int      `json:"security_updates"`
	SystemUpdatesAvailable   bool     `json:"system_updates_available"`
	SecurityUpdatesAvailable bool     `json:"security_updates_available"`
	SystemUpdatesList        []string `json:"system_updates_list"`
	SecurityUpdatesList      []string `json:"security_updates_list"`
	CacheExists              bool     `json:"cache_exists"`
	CacheUpToDate            bool     `json:"cache_up_to_date"`
	CacheDateCreated         string   `json:"cache_created_on,omitempty"`
}

func systemUpdates(useCache bool) systemUpdatesJsonStruct {
	systemUpdatesInput := systemUpdatesStruct{}
	systemUpdatesOutput := systemUpdatesJsonStruct{}
	osType := detectOs()
	if useCache {
		return readCache()
	} else if osType.dnf {
		systemUpdatesInput = dnfCheck()
	} else if osType.deb {
		systemUpdatesInput = debCheck()
	} else if osType.yum {
		systemUpdatesInput = yumCheck()
	} else {
		log.Fatal("Sorry, but your OS is not yet supported!")
	}

	systemUpdatesOutput.CacheExists = false
	systemUpdatesOutput.CacheUpToDate = false

	systemUpdatesOutput.NumberOfSystemUpdates = systemUpdatesInput.numberOfSystemUpdates
	systemUpdatesOutput.NumberOfSecurityUpdates = systemUpdatesInput.numberOfSecurityUpdates

	systemUpdatesOutput.SystemUpdatesAvailable = systemUpdatesInput.systemUpdatesAvailable
	systemUpdatesOutput.SecurityUpdatesAvailable = systemUpdatesInput.securityUpdatesAvailable

	systemUpdatesOutput.SystemUpdatesList = systemUpdatesInput.systemUpdatesList
	systemUpdatesOutput.SecurityUpdatesList = systemUpdatesInput.securityUpdatesList

	return systemUpdatesOutput
}

func readCache() systemUpdatesJsonStruct {
	jsonOutput := systemUpdatesJsonStruct{}
	jsonFile := "/tmp/syscheck_updates.json"
	data, err := os.ReadFile(jsonFile)
	if err != nil {
		jsonOutput.CacheDateCreated = time.Now().Add(-(time.Hour * 48)).Format("2006-01-02 15:04:05")
		jsonOutput.CacheExists = false
		jsonOutput.CacheUpToDate = false
		return jsonOutput
	}

	jsonError := json.Unmarshal(data, &jsonOutput)
	if jsonError != nil {
		log.Fatal(jsonError)
	}

	file, err := os.Stat(jsonFile)
	if err != nil {
		log.Fatal(err)
	}

	jsonOutput.CacheDateCreated = file.ModTime().Format("2006-01-02 15:04:05")
	jsonOutput.CacheExists = true

	if file.ModTime().Add(time.Hour * 12).After(time.Now()) {
		jsonOutput.CacheUpToDate = true
	} else {
		jsonOutput.CacheUpToDate = false
	}

	return jsonOutput
}
