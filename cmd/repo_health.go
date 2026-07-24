package cmd

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// Repository health detection.
//
// A broken repository is close to invisible from the outside: apt-get update exits 0 when a
// host stops resolving, and DNF5 exits 0 and still prints "Metadata cache created." even
// when a repository failed completely (its errors go to stderr behind a ">>>" prefix). So
// neither the exit code nor stdout can be trusted here — the refresh output has to be read.
// The practical consequence for an operator is that update counts silently go stale, which
// is exactly the failure mode this detection exists to surface.

// repoIssueRank orders competing diagnoses for the same repository so the banner shows the
// most actionable one. A repository that reports both "GPG error" and "is not signed" has a
// key problem, not a signing-policy problem.
type repoIssueRank int

const (
	rankUnknown repoIssueRank = iota
	rankFetch
	rankTLS
	rankUnreachable
	rankNotFound
	rankGPG
)

type repoIssue struct {
	Repo   string `json:"repo"`
	Reason string `json:"reason"`
	URL    string `json:"url,omitempty"`

	rank repoIssueRank
}

var (
	// APT
	reAptGPGError     = regexp.MustCompile(`^W: GPG error: (\S+) (\S+) \S+: (.+)$`)
	reAptNotSigned    = regexp.MustCompile(`^E: The repository '(\S+) (\S+) \S+' is not signed\.`)
	reAptNoRelease    = regexp.MustCompile(`^E: The repository '(\S+) (\S+) \S+' does not have a Release file\.`)
	reAptFailedFetch  = regexp.MustCompile(`^[WE]: Failed to fetch (\S+)\s+(.*)$`)
	reAptErrBlock     = regexp.MustCompile(`^Err:\d+\s+(\S+)\s+(\S+)`)
	reAptIndexFailure = regexp.MustCompile(`^W: Some index files failed to download`)

	// DNF4 names the repository directly; DNF5 only reports URLs.
	reDnfRepoErrorHeader = regexp.MustCompile(`^Errors during downloading metadata for repository '([^']+)':`)
	reDnfRepoErrorLine   = regexp.MustCompile(`^Error: Failed to download metadata for repo '([^']+)': (.+)$`)
	reDnfDetail          = regexp.MustCompile(`^\s*-\s+(.+)$`)
	reDnf5Error          = regexp.MustCompile(`^>>>\s*(.+)$`)

	// Any absolute URL, used to attribute a DNF5 error back to a configured repository.
	reURL = regexp.MustCompile(`https?://[^\s\[\]()]+`)

	// dnf repoinfo field lines.
	reRepoInfoID  = regexp.MustCompile(`^Repo ID\s*:\s*(\S+)`)
	reRepoInfoURL = regexp.MustCompile(`^\s*(?:Base URL|Mirrorlist|Metalink)\s*:\s*(\S+)`)
)

// classifyRepoFailure turns a package-manager error string into a short operator-facing
// reason plus a rank. Matching runs most-specific first.
func classifyRepoFailure(detail string) (string, repoIssueRank) {
	lower := strings.ToLower(detail)

	switch {
	case strings.Contains(lower, "no_pubkey"):
		if key := regexp.MustCompile(`NO_PUBKEY ([0-9A-Fa-f]+)`).FindStringSubmatch(detail); key != nil {
			return "missing GPG key " + key[1], rankGPG
		}
		return "missing GPG key", rankGPG
	case strings.Contains(lower, "public key is not available"),
		strings.Contains(lower, "gpg signature verification"),
		strings.Contains(lower, "key retrieval failed"),
		strings.Contains(lower, "gpg error"):
		return "missing or invalid GPG key", rankGPG
	case strings.Contains(lower, "is not signed"):
		return "repository is not signed", rankGPG
	case strings.Contains(lower, "status code: 404"),
		strings.Contains(lower, "404  not found"),
		strings.Contains(lower, "404 not found"),
		strings.Contains(lower, "does not have a release file"):
		return "not found (HTTP 404)", rankNotFound
	case strings.Contains(lower, "status code: 403"):
		return "access denied (HTTP 403)", rankNotFound
	case strings.Contains(lower, "could not resolve"), strings.Contains(lower, "couldn't resolve"),
		strings.Contains(lower, "name or service not known"):
		return "host does not resolve", rankUnreachable
	case strings.Contains(lower, "connection refused"), strings.Contains(lower, "failed to connect"),
		strings.Contains(lower, "timed out"), strings.Contains(lower, "timeout"),
		strings.Contains(lower, "usable url not found"), strings.Contains(lower, "all mirrors were tried"):
		return "unreachable", rankUnreachable
	case strings.Contains(lower, "certificate"), strings.Contains(lower, "ssl"), strings.Contains(lower, "handshake"):
		return "TLS certificate error", rankTLS
	case strings.Contains(lower, "release file") && strings.Contains(lower, "expired"):
		return "Release file expired", rankFetch
	default:
		return shortenCell(strings.TrimSpace(detail), 60), rankFetch
	}
}

// repoLabelFromURL reduces a URL to something short enough for a banner line.
func repoLabelFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return shortenCell(raw, 40)
	}
	return parsed.Host
}

// parseAptRepoIssues extracts repository failures from `apt-get update` output.
//
// The "Err:" blocks are the primary source rather than the trailing W:/E: summary lines,
// because apt only explains the first failure in the summary: with two broken repositories
// it prints a full W:/E: diagnosis for one and nothing but an "Err:" block for the other.
// Entries are keyed by host so an Err block and its matching W:/E: line collapse together.
func parseAptRepoIssues(output string) []repoIssue {
	type aptEntry struct {
		label  string
		url    string
		reason string
		rank   repoIssueRank
	}

	entries := make(map[string]*aptEntry)
	var order []string
	sawGenericFailure := false

	upsert := func(rawURL, suite, detail string) {
		host := repoLabelFromURL(rawURL)
		if host == "" {
			return
		}

		entry, ok := entries[host]
		if !ok {
			entry = &aptEntry{label: host, url: rawURL}
			entries[host] = entry
			order = append(order, host)
		}
		// The suite is only present on Err/GPG lines, not on "Failed to fetch".
		if suite != "" && !strings.Contains(entry.label, " ") {
			entry.label = host + " " + suite
		}
		if detail == "" {
			return
		}
		if reason, rank := classifyRepoFailure(detail); rank > entry.rank || entry.reason == "" {
			entry.reason, entry.rank = reason, rank
		}
	}

	lines := strings.Split(output, "\n")
	for i, line := range lines {
		line = strings.TrimRight(line, "\r")

		// "Err:<n> <url> <suite> <file>", optionally followed by an indented reason.
		if m := reAptErrBlock.FindStringSubmatch(line); m != nil {
			detail := ""
			if i+1 < len(lines) {
				next := strings.TrimRight(lines[i+1], "\r")
				if strings.HasPrefix(next, " ") && strings.TrimSpace(next) != "" {
					detail = strings.TrimSpace(next)
				}
			}
			upsert(m[1], m[2], detail)
			continue
		}

		switch {
		case reAptGPGError.MatchString(line):
			m := reAptGPGError.FindStringSubmatch(line)
			upsert(m[1], m[2], m[3])
		case reAptNotSigned.MatchString(line):
			m := reAptNotSigned.FindStringSubmatch(line)
			upsert(m[1], m[2], "is not signed")
		case reAptNoRelease.MatchString(line):
			m := reAptNoRelease.FindStringSubmatch(line)
			upsert(m[1], m[2], "does not have a Release file")
		case reAptFailedFetch.MatchString(line):
			m := reAptFailedFetch.FindStringSubmatch(line)
			upsert(m[1], "", m[2])
		case reAptIndexFailure.MatchString(line):
			sawGenericFailure = true
		}
	}

	issues := make([]repoIssue, 0, len(order))
	for _, host := range order {
		entry := entries[host]
		reason := entry.reason
		if reason == "" {
			reason = "failed to refresh"
		}
		issues = append(issues, repoIssue{Repo: entry.label, Reason: reason, URL: entry.url, rank: entry.rank})
	}

	// apt prints the summary line alongside the specific ones; only fall back to it when
	// nothing more precise was found, so an unrecognised error is still reported.
	if len(issues) == 0 && sawGenericFailure {
		issues = append(issues, repoIssue{
			Repo:   "one or more repositories",
			Reason: "index files failed to download",
			rank:   rankFetch,
		})
	}

	return dedupeRepoIssues(issues)
}

// parseDnfRepoIssues extracts repository failures from `dnf makecache` output. It handles
// both the DNF4 layout (which names the repository) and the DNF5 ">>>" layout (which does
// not, leaving the URL as the only identifier until attributeDnfRepoIssues runs).
func parseDnfRepoIssues(output string) []repoIssue {
	var issues []repoIssue
	currentRepo := ""

	add := func(repo, detail, repoURL string) {
		reason, rank := classifyRepoFailure(detail)
		label := repo
		if label == "" {
			label = repoLabelFromURL(repoURL)
		}
		if label == "" {
			label = "unknown repository"
		}
		issues = append(issues, repoIssue{Repo: label, Reason: reason, URL: repoURL, rank: rank})
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")

		if m := reDnfRepoErrorHeader.FindStringSubmatch(line); m != nil {
			currentRepo = m[1]
			continue
		}
		if m := reDnfRepoErrorLine.FindStringSubmatch(line); m != nil {
			add(m[1], m[2], reURL.FindString(m[2]))
			currentRepo = ""
			continue
		}
		if m := reDnfDetail.FindStringSubmatch(line); m != nil && currentRepo != "" {
			add(currentRepo, m[1], reURL.FindString(m[1]))
			continue
		}
		if m := reDnf5Error.FindStringSubmatch(line); m != nil {
			detail := m[1]
			// "Usable URL not found" is a trailing summary with no URL of its own; the
			// specific cause was already reported on the lines above it.
			if strings.Contains(strings.ToLower(detail), "usable url not found") {
				continue
			}
			add("", detail, reURL.FindString(detail))
			continue
		}

		if strings.TrimSpace(line) == "" {
			currentRepo = ""
		}
	}

	return dedupeRepoIssues(issues)
}

// attributeDnfRepoIssues replaces URL-derived labels with the configured repository ID by
// matching against `dnf repoinfo` output. DNF5 never names the failing repository itself.
func attributeDnfRepoIssues(issues []repoIssue, repoInfoOutput string) []repoIssue {
	if len(issues) == 0 || repoInfoOutput == "" {
		return issues
	}

	type repoEntry struct {
		id   string
		urls []string
	}

	var entries []repoEntry
	current := repoEntry{}
	flush := func() {
		if current.id != "" {
			entries = append(entries, current)
		}
		current = repoEntry{}
	}

	for _, line := range strings.Split(repoInfoOutput, "\n") {
		if m := reRepoInfoID.FindStringSubmatch(line); m != nil {
			flush()
			current.id = m[1]
			continue
		}
		if m := reRepoInfoURL.FindStringSubmatch(line); m != nil && current.id != "" {
			current.urls = append(current.urls, m[1])
		}
	}
	flush()

	for i := range issues {
		if issues[i].URL == "" {
			continue
		}
		for _, entry := range entries {
			for _, candidate := range entry.urls {
				if urlsMatch(issues[i].URL, candidate) {
					issues[i].Repo = entry.id
				}
			}
		}
	}

	return dedupeRepoIssues(issues)
}

// urlsMatch reports whether a failing URL belongs to a configured repository URL. The
// failing URL is the configured one with "repodata/repomd.xml" appended, so a prefix
// comparison on the directory part is enough.
func urlsMatch(failing string, configured string) bool {
	configured = strings.TrimSuffix(configured, "/")
	if configured == "" {
		return false
	}
	return strings.HasPrefix(failing, configured+"/") || failing == configured
}

// dedupeRepoIssues keeps one entry per repository, preferring the most actionable diagnosis.
func dedupeRepoIssues(issues []repoIssue) []repoIssue {
	if len(issues) == 0 {
		return nil
	}

	best := make(map[string]repoIssue, len(issues))
	order := make([]string, 0, len(issues))
	for _, issue := range issues {
		existing, ok := best[issue.Repo]
		if !ok {
			best[issue.Repo] = issue
			order = append(order, issue.Repo)
			continue
		}
		if issue.rank > existing.rank {
			best[issue.Repo] = issue
		}
	}

	result := make([]repoIssue, 0, len(order))
	for _, key := range order {
		result = append(result, best[key])
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].rank > result[j].rank })
	return result
}
