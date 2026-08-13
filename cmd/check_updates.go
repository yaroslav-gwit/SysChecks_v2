package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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

	// A full NEVRA: name-[epoch:]version-release.arch. Anchored and hyphen-counted so that
	// dates ("2026-07-11"), times and column headers can never match.
	reNEVRA = regexp.MustCompile(`^\S+-[^-\s]+-[^-\s]+\.[A-Za-z0-9_]+$`)

	reRHSAReplace = regexp.MustCompile(`^RHSA-\d+:\d+.*?\.`)

	// Skip patterns for DNF/YUM
	reSubscriptionMgmt = regexp.MustCompile(`Updating\s+Subscription\s+Management`)
	reSecurityPrefix   = regexp.MustCompile(`^Security:\s+`)

	// Informational/warning lines that are not packages and must never be parsed as one.
	reNoSecurityNeeded = regexp.MustCompile(`(?i)no security updates needed`)
	reRepoListedTwice  = regexp.MustCompile(`(?i)is listed more than once`)

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

	// apk version emits name-version with no separator other than the final hyphen before a
	// version ending in -rN. A greedy name capture preserves package names that end in digits.
	reApkPackageVersion = regexp.MustCompile(`^(.+)-([0-9][0-9A-Za-z._+~-]*-r[0-9]+)$`)
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

		// The file being written *is* the cache, so record that truthfully. Serialising the
		// placeholder false/false from systemUpdates() would make every consumer that reads
		// the JSON directly believe the cache is missing and stale.
		result.CacheExists = true
		result.CacheUpToDate = true
		result.CacheDateCreated = time.Now().Format("2006-01-02 15:04:05")

		if err := writeUpdateStatusCache(result); err != nil {
			log.Fatalf("Error writing status cache: %v", err)
		}
		return
	}

	emitJSON(systemUpdates(cacheUse), resolveOutput(outputJSON, false, jsonPretty))
}

type systemUpdatesStruct struct {
	numberOfSystemUpdates    int
	numberOfSecurityUpdates  int
	systemUpdatesAvailable   bool
	securityUpdatesAvailable bool
	securityUpdatesSupported bool
	systemUpdatesList        []string
	securityUpdatesList      []string
	repositoryIssues         []repoIssue
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
		securityUpdatesSupported: true,
		systemUpdatesList:        []string{},
		securityUpdatesList:      []string{},
	}

	// DNF cache refresh. The output is parsed rather than the exit code: DNF5 reports a
	// failed repository only on stderr and still exits 0 with "Metadata cache created."
	refreshOut, exitCode, err := runCommandWithTimeoutCombined(ctx, "dnf", "makecache")
	if err != nil && exitCode != 100 {
		log.Printf("Warning: DNF cache update: %v", err)
	}
	result.repositoryIssues = parseDnfRepoIssues(string(refreshOut))
	if len(result.repositoryIssues) > 0 {
		// Only pay for repoinfo when something is already known to be broken; DNF5 errors
		// carry a URL but no repository ID.
		infoOut, _, _ := runCommandWithTimeoutStdout(ctx, "dnf", "-q", "repoinfo")
		result.repositoryIssues = attributeDnfRepoIssues(result.repositoryIssues, string(infoOut))
	}

	// Query system updates. repoquery gives stable fields and avoids parsing aligned columns.
	// stdout only: package-manager warnings on stderr must not pollute the parsed list.
	// The trailing newline is required: DNF5 emits the format string verbatim and adds no
	// record separator of its own, so without it every package runs into the next one.
	// DNF4 appends its own newline, which only yields harmless blank lines.
	sysOut, _, _ := runCommandWithTimeoutStdout(ctx, "dnf", "--cacheonly", "-q", "repoquery", "--upgrades", "--latest-limit=1", "--qf", "%{name}\t%{epoch}\t%{version}\t%{release}\t%{arch}\t%{repoid}\n")

	// Query security updates from advisory metadata where available.
	secOut, _, _ := runCommandWithTimeoutStdout(ctx, "dnf", "--cacheonly", "-q", "updateinfo", "list", "--updates", "--security")

	result.systemUpdatesList = parseDnfRepoqueryOutput(string(sysOut))
	if len(result.systemUpdatesList) == 0 {
		fallbackOut, _, _ := runCommandWithTimeoutStdout(ctx, "dnf", "--cacheonly", "check-update")
		result.systemUpdatesList = parseDnfOutput(string(fallbackOut), false)
	}

	result.securityUpdatesList = parseUpdateinfoSecurityOutput(string(secOut))
	if len(result.securityUpdatesList) == 0 {
		fallbackOut, _, _ := runCommandWithTimeoutStdout(ctx, "dnf", "--cacheonly", "check-update", "--security")
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
		if reNoSecurityNeeded.MatchString(line) || reRepoListedTwice.MatchString(line) {
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

		// Exactly six fields, not "at least six": a run-on line from a package manager that
		// ignored the record separator would otherwise be silently truncated into one bogus
		// entry, which is worse than returning nothing and letting check-update take over.
		fields := strings.Split(line, "\t")
		if len(fields) != 6 {
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

// findNEVRAField returns the last field shaped like a full NEVRA
// (name[-more]-[epoch:]version-release.arch), or "" if the line holds none. This keeps the
// security parser independent of how many columns a given dnf/yum release prints, and it
// discards header rows ("Name Type Severity Package Issued") for free.
func findNEVRAField(parts []string) string {
	for i := len(parts) - 1; i >= 0; i-- {
		if reNEVRA.MatchString(parts[i]) {
			return parts[i]
		}
	}
	return ""
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

		// Locate the package column by shape rather than by position. DNF4 puts the NEVRA
		// last ("ADVISORY Important/Sec. pkg-1.0-1.el9.x86_64"), DNF5 follows it with an
		// Issued date that itself splits into two fields ("... pkg-1.0-1.fc44.x86_64
		// 2026-07-11 01:06:30"), so "last field" silently collects timestamps there.
		pkg := findNEVRAField(parts)
		if pkg == "" || strings.HasSuffix(pkg, ".src") {
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
		securityUpdatesSupported: true,
		systemUpdatesList:        []string{},
		securityUpdatesList:      []string{},
	}

	// APT cache refresh. Parsed rather than trusted to the exit code: apt-get update exits 0
	// when a repository host stops resolving, and only fails outright on a missing Release.
	refreshOut, exitCode, err := runCommandWithTimeoutCombined(ctx, "apt-get", "-y", "update")
	if err != nil && exitCode != 100 {
		errorValue := strings.TrimSpace(string(refreshOut))
		log.Printf("Warning: APT cache update: %s (exit code: %d)", errorValue, exitCode)
	}
	result.repositoryIssues = parseAptRepoIssues(string(refreshOut))

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

func apkCheck() systemUpdatesStruct {
	helpers.RootUserCheck()

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithSuffix(" Running APK related procedures"))
	s.Prefix = " "
	s.Start()
	defer s.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// apk itself has no security-advisory channel. Keeping this list nil is deliberate:
	// JSON consumers receive null plus security_updates_status=unsupported, never a false 0.
	result := systemUpdatesStruct{
		securityUpdatesSupported: false,
		systemUpdatesList:        []string{},
	}

	refreshOut, _, refreshErr := runCommandWithTimeoutCombined(ctx, "apk", "update")
	if refreshErr != nil {
		detail := strings.TrimSpace(string(refreshOut))
		if detail == "" {
			detail = refreshErr.Error()
		}
		reason, rank := classifyRepoFailure(detail)
		result.repositoryIssues = []repoIssue{{
			Repo:   "apk repositories",
			Reason: reason,
			rank:   rank,
		}}
	}

	versionOut, _, err := runCommandWithTimeoutStdout(ctx, "apk", "version", "-l", "<")
	if err != nil {
		log.Fatalf("APK update query failed: %v", err)
	}
	result.systemUpdatesList = parseApkVersionOutput(string(versionOut))
	result.numberOfSystemUpdates = len(result.systemUpdatesList)
	result.systemUpdatesAvailable = result.numberOfSystemUpdates > 0

	return result
}

// parseApkVersionOutput parses `apk version -l '<'`, whose upgrade rows are shaped as
// "installed-package-version < candidate-version". Other comparison rows and warnings are
// ignored so they cannot inflate the update count.
func parseApkVersionOutput(output string) []string {
	updates := []string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		parts := strings.Fields(line)
		if len(parts) != 3 || parts[1] != "<" || parts[0] == "" || parts[2] == "" {
			continue
		}
		installed := reApkPackageVersion.FindStringSubmatch(parts[0])
		if len(installed) != 3 {
			continue
		}
		updates = append(updates, fmt.Sprintf("%s [%s] (%s)", installed[1], installed[2], parts[2]))
	}
	return updates
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
		securityUpdatesSupported: true,
		systemUpdatesList:        []string{},
		securityUpdatesList:      []string{},
	}

	// YUM cache refresh
	refreshOut, _, err := runCommandWithTimeoutCombined(ctx, "yum", "makecache", "fast")
	if err != nil {
		log.Printf("Warning: YUM cache update: %v", err)
	}
	result.repositoryIssues = parseDnfRepoIssues(string(refreshOut))

	// Check system updates (stdout only; stderr warnings must not be parsed as packages).
	sysOut, _, _ := runCommandWithTimeoutStdout(ctx, "yum", "--cacheonly", "check-update")

	// Check security updates from advisory metadata where the distribution provides it.
	secOut, _, _ := runCommandWithTimeoutStdout(ctx, "yum", "--cacheonly", "-q", "updateinfo", "list", "updates", "security")

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
		reMgrVersionLock.MatchString(line) ||
		reNoSecurityNeeded.MatchString(line) ||
		reRepoListedTwice.MatchString(line)
}

type systemUpdatesJsonStruct struct {
	NumberOfSystemUpdates    int      `json:"system_updates"`
	NumberOfSecurityUpdates  *int     `json:"security_updates"`
	SystemUpdatesAvailable   bool     `json:"system_updates_available"`
	SecurityUpdatesAvailable *bool    `json:"security_updates_available"`
	SystemUpdatesList        []string `json:"system_updates_list"`
	SecurityUpdatesList      []string `json:"security_updates_list"`
	SystemUpdatesStatus      string   `json:"system_updates_status"`
	SecurityUpdatesStatus    string   `json:"security_updates_status"`
	// RepositoryIssues records repositories that failed to refresh. When this is non-empty
	// the update counts above are incomplete, because the failed repository contributed
	// nothing to them.
	RepositoryIssues     []repoIssue `json:"repository_issues"`
	RepositoryIssueCount int         `json:"repository_issue_count"`
	CacheExists          bool        `json:"cache_exists"`
	CacheUpToDate        bool        `json:"cache_up_to_date"`
	CacheDateCreated     string      `json:"cache_created_on,omitempty"`
}

func systemUpdates(useCache bool) systemUpdatesJsonStruct {
	// Detection must happen before the cache shortcut. In v1.3.0 an unsupported OS could
	// read a missing cache and return a fully-patched-looking report with exit code 0.
	osType := detectOs()
	if useCache {
		return readCache(osType)
	}

	input := getPackageManager(osType).CheckUpdates()

	repositoryIssues := input.repositoryIssues
	if repositoryIssues == nil {
		repositoryIssues = []repoIssue{}
	}

	result := systemUpdatesJsonStruct{
		NumberOfSystemUpdates:  input.numberOfSystemUpdates,
		SystemUpdatesAvailable: input.systemUpdatesAvailable,
		SystemUpdatesList:      input.systemUpdatesList,
		SystemUpdatesStatus:    updateStatus(repositoryIssues),
		RepositoryIssues:       repositoryIssues,
		RepositoryIssueCount:   len(repositoryIssues),
		CacheExists:            false,
		CacheUpToDate:          false,
	}
	applySecurityUpdateSupport(&result, input.securityUpdatesSupported, input.numberOfSecurityUpdates, input.securityUpdatesAvailable, input.securityUpdatesList)
	return result
}

func readCache(osType detectOsStruct) systemUpdatesJsonStruct {
	result := systemUpdatesJsonStruct{
		SystemUpdatesList:   []string{},
		SecurityUpdatesList: []string{},
		RepositoryIssues:    []repoIssue{},
	}

	data, err := readTrustedStatusCache()
	if err != nil {
		// Missing and unreadable caches both mean that no update result is available. Do not
		// manufacture a timestamp or turn zero-value counters into a successful check.
		result.CacheExists = false
		result.CacheUpToDate = false
		result.SystemUpdatesStatus = "unknown"
		applyCachedSecuritySupport(&result, osType, "unknown")
		return result
	}

	if err := json.Unmarshal(data, &result); err != nil {
		log.Printf("Warning: Could not parse cache file: %v", err)
		result.CacheExists = false
		result.CacheUpToDate = false
		result.SystemUpdatesStatus = "unknown"
		applyCachedSecuritySupport(&result, osType, "unknown")
		return result
	}

	// A schedule-only status document is valid but does not contain an update report yet.
	if !result.CacheExists || result.CacheDateCreated == "" {
		result.CacheExists = false
		result.CacheUpToDate = false
		result.CacheDateCreated = ""
		result.SystemUpdatesStatus = "unknown"
		applyCachedSecuritySupport(&result, osType, "unknown")
		return result
	}

	// Schedule mutations also update this document, so its filesystem mtime is not the age
	// of the update report. Only the timestamp recorded by updates refresh is authoritative.
	written, err := time.Parse("2006-01-02 15:04:05", result.CacheDateCreated)
	result.CacheUpToDate = err == nil && written.Add(12*time.Hour).After(time.Now())
	normalizeCachedUpdateStatus(&result, osType)

	return result
}

func updateStatus(repositoryIssues []repoIssue) string {
	if len(repositoryIssues) > 0 {
		return "incomplete"
	}
	return "ok"
}

func applySecurityUpdateSupport(result *systemUpdatesJsonStruct, supported bool, count int, available bool, updates []string) {
	if !supported {
		result.NumberOfSecurityUpdates = nil
		result.SecurityUpdatesAvailable = nil
		result.SecurityUpdatesList = nil
		result.SecurityUpdatesStatus = "unsupported"
		return
	}
	result.NumberOfSecurityUpdates = intPointer(count)
	result.SecurityUpdatesAvailable = boolPointer(available)
	result.SecurityUpdatesList = updates
	result.SecurityUpdatesStatus = updateStatus(result.RepositoryIssues)
}

func applyCachedSecuritySupport(result *systemUpdatesJsonStruct, osType detectOsStruct, supportedStatus string) {
	if osType.packageManagerKind() == packageManagerAPK {
		applySecurityUpdateSupport(result, false, 0, false, nil)
		return
	}
	if result.NumberOfSecurityUpdates == nil {
		result.NumberOfSecurityUpdates = intPointer(0)
	}
	if result.SecurityUpdatesAvailable == nil {
		result.SecurityUpdatesAvailable = boolPointer(false)
	}
	if result.SecurityUpdatesList == nil {
		result.SecurityUpdatesList = []string{}
	}
	if result.SecurityUpdatesStatus == "" {
		result.SecurityUpdatesStatus = supportedStatus
	}
}

func normalizeCachedUpdateStatus(result *systemUpdatesJsonStruct, osType detectOsStruct) {
	status := updateStatus(result.RepositoryIssues)
	if result.SystemUpdatesStatus == "" {
		result.SystemUpdatesStatus = status
	}
	applyCachedSecuritySupport(result, osType, status)
}

func securityUpdateCount(result systemUpdatesJsonStruct) (int, bool) {
	if result.SecurityUpdatesStatus == "unsupported" || result.NumberOfSecurityUpdates == nil {
		return 0, false
	}
	return *result.NumberOfSecurityUpdates, true
}

func intPointer(value int) *int    { return &value }
func boolPointer(value bool) *bool { return &value }
