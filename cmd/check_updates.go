package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"syschecks/helpers"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

// Pre-compiled regex patterns for update checking (compiled once at package init)
var (
	// DNF/YUM patterns
	reMultiSpace       = regexp.MustCompile(`\s+`)
	reSecReplace       = regexp.MustCompile(`.*/Sec\.\s*`)
	reMetaDataContinue = regexp.MustCompile(`Last\s+metadata`)
	reKernelContinue   = regexp.MustCompile(`Security:\s+kernel-core`)
	reObsoleteBreak    = regexp.MustCompile(`Obsoleting\s+Packages`)
	reSrcMatch         = regexp.MustCompile(`\.src$`)
	reRHSAReplace      = regexp.MustCompile(`^RHSA-\d+:\d+.*?\.`)

	// Skip patterns for DNF/YUM
	reSubscriptionMgmt = regexp.MustCompile(`Updating\s+Subscription\s+Management`)
	reSecurityPrefix   = regexp.MustCompile(`^Security:\s+`)

	// YUM-specific continue patterns
	reLoadedPlugins  = regexp.MustCompile(`Loaded plugins:`)
	reUpdateInfoDone = regexp.MustCompile(`updateinfo list done`)
	reManagerComma   = regexp.MustCompile(`: manager,`)
	reNotRegistered  = regexp.MustCompile(`This system is not registered`)
	reVersionLock    = regexp.MustCompile(`:\s*versionlock`)
	reSubMgr         = regexp.MustCompile(`: subscription-manager`)
	reMgrVersionLock = regexp.MustCompile(`: manager, versionlock`)

	// APT patterns
	reMatchSysUpdate = regexp.MustCompile(`^Inst\s+`)
	reMatchSecUpdate = regexp.MustCompile(`(?i)security`)
	reAptInstLine    = regexp.MustCompile(`^Inst\s+(\S+)\s+(?:\[(.*?)\]\s+)?\((.*?)\)$`)
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
		result := systemUpdates(false)
		jsonOut, err := json.Marshal(result)
		if err != nil {
			log.Fatalf("Error marshaling updates: %v", err)
		}

		cacheFileLocation := "/tmp/syscheck_updates.json"
		if err := os.WriteFile(cacheFileLocation, jsonOut, 0644); err != nil {
			log.Fatalf("Error writing cache file: %v", err)
		}
		// Ensure file is readable by all users on hardened systems
		_ = os.Chmod(cacheFileLocation, 0644)
		return
	}

	result := systemUpdates(cacheUse)
	var jsonOut []byte
	var err error

	if jsonPretty {
		jsonOut, err = json.MarshalIndent(result, "", "   ")
	} else {
		jsonOut, err = json.Marshal(result)
	}

	if err != nil {
		log.Fatalf("Error marshaling updates: %v", err)
	}
	fmt.Println(string(jsonOut))
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

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithSuffix(" Running DNF related procedures"))
	s.Prefix = " "
	s.Start()
	defer s.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result := systemUpdatesStruct{
		systemUpdatesList:   []string{},
		securityUpdatesList: []string{},
	}

	// DNF cache refresh
	if _, exitCode, err := runCommandWithTimeoutCombined(ctx, "dnf", "makecache"); err != nil && exitCode != 100 {
		log.Printf("Warning: DNF cache update: %v", err)
	}

	// Query system updates. repoquery gives stable fields and avoids parsing aligned columns.
	sysOut, _, _ := runCommandWithTimeoutCombined(ctx, "dnf", "--cacheonly", "-q", "repoquery", "--upgrades", "--latest-limit=1", "--qf", "%{name}\t%{epoch}\t%{version}\t%{release}\t%{arch}\t%{repoid}")

	// Query security updates from advisory metadata where available.
	secOut, _, _ := runCommandWithTimeoutCombined(ctx, "dnf", "--cacheonly", "-q", "updateinfo", "list", "--updates", "--security")

	result.systemUpdatesList = parseDnfRepoqueryOutput(string(sysOut))
	if len(result.systemUpdatesList) == 0 {
		fallbackOut, _, _ := runCommandWithTimeoutCombined(ctx, "dnf", "--cacheonly", "check-update")
		result.systemUpdatesList = parseDnfOutput(string(fallbackOut), false)
	}

	result.securityUpdatesList = parseUpdateinfoSecurityOutput(string(secOut))
	if len(result.securityUpdatesList) == 0 {
		fallbackOut, _, _ := runCommandWithTimeoutCombined(ctx, "dnf", "--cacheonly", "check-update", "--security")
		result.securityUpdatesList = parseDnfOutput(string(fallbackOut), true)
	}

	result.numberOfSystemUpdates = len(result.systemUpdatesList)
	result.numberOfSecurityUpdates = len(result.securityUpdatesList)
	result.systemUpdatesAvailable = result.numberOfSystemUpdates > 0
	result.securityUpdatesAvailable = result.numberOfSecurityUpdates > 0

	return result
}

// parseDnfOutput parses DNF check-update output
func parseDnfOutput(output string, isSecurity bool) []string {
	updates := []string{}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		// Skip metadata and informational lines
		if reMetaDataContinue.MatchString(line) || reKernelContinue.MatchString(line) {
			continue
		}
		if reObsoleteBreak.MatchString(line) {
			break
		}
		if reSubscriptionMgmt.MatchString(line) || reSecurityPrefix.MatchString(line) {
			continue
		}

		// Clean up the line
		line = reMultiSpace.ReplaceAllString(line, " ")
		line = strings.ReplaceAll(line, " baseos ", "")
		line = strings.ReplaceAll(line, " appstream ", "")
		line = strings.ReplaceAll(line, " epel ", "")
		line = strings.ReplaceAll(line, " epel-source ", "")
		line = reMultiSpace.ReplaceAllString(line, " ")

		if isSecurity {
			line = reSecReplace.ReplaceAllString(line, "")
			line = reRHSAReplace.ReplaceAllString(line, "")
		}

		// Skip source packages
		parts := strings.Fields(line)
		if len(parts) > 0 && reSrcMatch.MatchString(parts[0]) {
			continue
		}

		line = strings.TrimSpace(line)
		if len(line) > 0 {
			updates = append(updates, line)
		}
	}

	return updates
}

// parseDnfRepoqueryOutput parses tab-separated dnf repoquery --upgrades output.
func parseDnfRepoqueryOutput(output string) []string {
	updates := []string{}
	seen := make(map[string]bool)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 6 {
			continue
		}

		name := strings.TrimSpace(fields[0])
		epoch := strings.TrimSpace(fields[1])
		version := strings.TrimSpace(fields[2])
		release := strings.TrimSpace(fields[3])
		arch := strings.TrimSpace(fields[4])
		repo := strings.TrimSpace(fields[5])
		if name == "" || version == "" {
			continue
		}

		evr := version
		if release != "" {
			evr += "-" + release
		}
		if epoch != "" && epoch != "0" && epoch != "(none)" {
			evr = epoch + ":" + evr
		}

		entry := fmt.Sprintf("%s.%s %s %s", name, arch, evr, repo)
		if !seen[entry] {
			updates = append(updates, entry)
			seen[entry] = true
		}
	}

	return updates
}

// parseUpdateinfoSecurityOutput parses dnf/yum updateinfo security advisory output.
func parseUpdateinfoSecurityOutput(output string) []string {
	updates := []string{}
	packageIndex := make(map[string]int)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(reMultiSpace.ReplaceAllString(line, " "))
		if line == "" || shouldSkipYumLine(line) {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		pkg := parts[len(parts)-1]
		if strings.HasSuffix(pkg, ".src") {
			continue
		}

		packageName := extractPackageName(pkg)
		if packageName == "" {
			continue
		}
		if existingIndex, ok := packageIndex[packageName]; ok {
			updates[existingIndex] = pkg
			continue
		}
		packageIndex[packageName] = len(updates)
		updates = append(updates, pkg)
	}

	return updates
}

func debCheck() systemUpdatesStruct {
	helpers.RootUserCheck()

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithSuffix(" Running APT related procedures"))
	s.Prefix = " "
	s.Start()
	defer s.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result := systemUpdatesStruct{
		systemUpdatesList:   []string{},
		securityUpdatesList: []string{},
	}

	// APT cache refresh
	if out, exitCode, err := runCommandWithTimeoutCombined(ctx, "apt-get", "-y", "update"); err != nil && exitCode != 100 {
		errorValue := strings.TrimSpace(string(out))
		log.Printf("Warning: APT cache update: %s (exit code: %d)", errorValue, exitCode)
	}

	// Check all updates using apt-get simulation mode. apt-get has a stable scripting interface; apt does not.
	sysOut, _, err := runCommandWithTimeoutCombined(ctx, "apt-get", "-s", "-o", "APT::Get::Show-Upgraded=true", "dist-upgrade")
	if err != nil {
		log.Printf("Warning: APT dist-upgrade simulation failed: %v", err)
		return result
	}

	// Parse APT output
	for _, line := range strings.Split(string(sysOut), "\n") {
		if !reMatchSysUpdate.MatchString(line) {
			continue
		}

		line = formatAptInstLine(line)

		if len(line) > 0 {
			result.systemUpdatesList = append(result.systemUpdatesList, line)
			if reMatchSecUpdate.MatchString(line) {
				result.securityUpdatesList = append(result.securityUpdatesList, line)
			}
		}
	}

	result.numberOfSystemUpdates = len(result.systemUpdatesList)
	result.numberOfSecurityUpdates = len(result.securityUpdatesList)
	result.systemUpdatesAvailable = result.numberOfSystemUpdates > 0
	result.securityUpdatesAvailable = result.numberOfSecurityUpdates > 0

	return result
}

func formatAptInstLine(line string) string {
	matches := reAptInstLine.FindStringSubmatch(strings.TrimSpace(line))
	if len(matches) != 4 {
		line = reMatchSysUpdate.ReplaceAllString(line, "")
		line = strings.ReplaceAll(line, " []", "")
		return strings.TrimSpace(line)
	}

	name := matches[1]
	oldVersion := strings.TrimSpace(matches[2])
	candidate := strings.TrimSpace(matches[3])
	if oldVersion == "" {
		return fmt.Sprintf("%s (%s)", name, candidate)
	}
	return fmt.Sprintf("%s [%s] (%s)", name, oldVersion, candidate)
}

func yumCheck() systemUpdatesStruct {
	helpers.RootUserCheck()

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithSuffix(" Running YUM related procedures"))
	s.Prefix = " "
	s.Start()
	defer s.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result := systemUpdatesStruct{
		systemUpdatesList:   []string{},
		securityUpdatesList: []string{},
	}

	// YUM cache refresh
	if _, _, err := runCommandWithTimeoutCombined(ctx, "yum", "makecache", "fast"); err != nil {
		log.Printf("Warning: YUM cache update: %v", err)
	}

	// Check system updates
	sysOut, _, _ := runCommandWithTimeoutCombined(ctx, "yum", "--cacheonly", "check-update")

	// Check security updates from advisory metadata where the distribution provides it.
	secOut, _, _ := runCommandWithTimeoutCombined(ctx, "yum", "--cacheonly", "-q", "updateinfo", "list", "updates", "security")

	// Parse system updates
	result.systemUpdatesList = parseYumOutput(string(sysOut))
	result.securityUpdatesList = parseUpdateinfoSecurityOutput(string(secOut))

	result.numberOfSystemUpdates = len(result.systemUpdatesList)
	result.numberOfSecurityUpdates = len(result.securityUpdatesList)
	result.systemUpdatesAvailable = result.numberOfSystemUpdates > 0
	result.securityUpdatesAvailable = result.numberOfSecurityUpdates > 0

	return result
}

// parseYumOutput parses YUM check-update output
func parseYumOutput(output string) []string {
	updates := []string{}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		// Skip informational lines
		if shouldSkipYumLine(line) {
			continue
		}
		if reObsoleteBreak.MatchString(line) {
			break
		}

		// Clean up the line
		line = reMultiSpace.ReplaceAllString(line, " ")
		line = strings.ReplaceAll(line, " baseos ", "")
		line = strings.ReplaceAll(line, " appstream ", "")
		line = strings.ReplaceAll(line, " epel ", "")
		line = strings.ReplaceAll(line, " epel-source", "")

		line = strings.TrimSpace(line)
		if len(line) > 0 {
			updates = append(updates, line)
		}
	}

	return updates
}

// shouldSkipYumLine checks if a YUM output line should be skipped
func shouldSkipYumLine(line string) bool {
	return reLoadedPlugins.MatchString(line) ||
		reUpdateInfoDone.MatchString(line) ||
		reManagerComma.MatchString(line) ||
		reNotRegistered.MatchString(line) ||
		reVersionLock.MatchString(line) ||
		reMetaDataContinue.MatchString(line) ||
		reSubMgr.MatchString(line) ||
		reMgrVersionLock.MatchString(line)
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
	if useCache {
		return readCache()
	}

	osType := detectOs()
	input := getPackageManager(osType).CheckUpdates()

	return systemUpdatesJsonStruct{
		NumberOfSystemUpdates:    input.numberOfSystemUpdates,
		NumberOfSecurityUpdates:  input.numberOfSecurityUpdates,
		SystemUpdatesAvailable:   input.systemUpdatesAvailable,
		SecurityUpdatesAvailable: input.securityUpdatesAvailable,
		SystemUpdatesList:        input.systemUpdatesList,
		SecurityUpdatesList:      input.securityUpdatesList,
		CacheExists:              false,
		CacheUpToDate:            false,
	}
}

func readCache() systemUpdatesJsonStruct {
	const cacheFile = "/tmp/syscheck_updates.json"

	result := systemUpdatesJsonStruct{
		SystemUpdatesList:   []string{},
		SecurityUpdatesList: []string{},
	}

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		// Cache doesn't exist, return empty result with stale date
		result.CacheDateCreated = time.Now().Add(-48 * time.Hour).Format("2006-01-02 15:04:05")
		result.CacheExists = false
		result.CacheUpToDate = false
		return result
	}

	if err := json.Unmarshal(data, &result); err != nil {
		log.Printf("Warning: Could not parse cache file: %v", err)
		result.CacheExists = false
		result.CacheUpToDate = false
		return result
	}

	// Get file modification time
	fileInfo, err := os.Stat(cacheFile)
	if err != nil {
		log.Printf("Warning: Could not stat cache file: %v", err)
		return result
	}

	modTime := fileInfo.ModTime()
	result.CacheDateCreated = modTime.Format("2006-01-02 15:04:05")
	result.CacheExists = true
	result.CacheUpToDate = modTime.Add(12 * time.Hour).After(time.Now())

	return result
}
