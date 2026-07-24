package cmd

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// All fixtures below are real output captured from Debian 12 (apt 2.6), Rocky 9 (DNF4) and
// Fedora 44 (DNF5) containers with deliberately broken repositories.

func findIssue(issues []repoIssue, repo string) (repoIssue, bool) {
	for _, issue := range issues {
		if issue.Repo == repo {
			return issue, true
		}
	}
	return repoIssue{}, false
}

func TestParseAptRepoIssuesMissingGPGKey(t *testing.T) {
	// The Microsoft Defender scenario: repository reachable, signing key not installed.
	output := `Err:4 https://packages.microsoft.com/debian/12/prod bookworm InRelease
W: GPG error: https://packages.microsoft.com/debian/12/prod bookworm InRelease: The following signatures couldn't be verified because the public key is not available: NO_PUBKEY EB3E94ADBE1229CF
E: The repository 'https://packages.microsoft.com/debian/12/prod bookworm InRelease' is not signed.
`

	issues := parseAptRepoIssues(output)
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1: %#v", len(issues), issues)
	}
	issue, ok := findIssue(issues, "packages.microsoft.com bookworm")
	if !ok {
		t.Fatalf("repository not identified: %#v", issues)
	}
	// "missing key" outranks "not signed": both describe the same repository, but only the
	// first tells the operator what to do.
	if issue.Reason != "missing GPG key EB3E94ADBE1229CF" {
		t.Fatalf("reason = %q", issue.Reason)
	}
}

func TestParseAptRepoIssuesHostVanished(t *testing.T) {
	// apt exits 0 here, so the output is the only signal.
	output := `Ign:2 http://repo.gone.invalid/debian bookworm InRelease
Err:2 http://repo.gone.invalid/debian bookworm InRelease
  Could not resolve 'repo.gone.invalid'
W: Failed to fetch http://repo.gone.invalid/debian/dists/bookworm/InRelease  Could not resolve 'repo.gone.invalid'
W: Some index files failed to download. They have been ignored, or old ones used instead.
`

	issues := parseAptRepoIssues(output)
	// The Err: block and the "Failed to fetch" line describe one repository, not two.
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1: %#v", len(issues), issues)
	}
	issue, ok := findIssue(issues, "repo.gone.invalid bookworm")
	if !ok {
		t.Fatalf("repository not identified: %#v", issues)
	}
	if issue.Reason != "host does not resolve" {
		t.Fatalf("reason = %q", issue.Reason)
	}
}

func TestParseAptRepoIssuesRemovedSuite(t *testing.T) {
	output := `Err:5 http://deb.debian.org/debian nosuchsuite Release
  404  Not Found [IP: 151.101.194.132 80]
E: The repository 'http://deb.debian.org/debian nosuchsuite Release' does not have a Release file.
`

	issues := parseAptRepoIssues(output)
	issue, ok := findIssue(issues, "deb.debian.org nosuchsuite")
	if !ok {
		t.Fatalf("repository not identified: %#v", issues)
	}
	if !strings.Contains(issue.Reason, "404") {
		t.Fatalf("reason = %q, want an HTTP 404 diagnosis", issue.Reason)
	}
}

// With two broken repositories apt explains only one of them in its W:/E: summary; the
// other appears solely as an Err: block. Both must still be reported.
func TestParseAptRepoIssuesTwoBrokenReposOnlyOneExplained(t *testing.T) {
	output := `Err:5 https://packages.microsoft.com/debian/12/prod bookworm InRelease
Err:1 http://repo.gone.invalid/debian bookworm InRelease
  Could not resolve 'repo.gone.invalid'
W: GPG error: https://packages.microsoft.com/debian/12/prod bookworm InRelease: The following signatures couldn't be verified because the public key is not available: NO_PUBKEY EB3E94ADBE1229CF
E: The repository 'https://packages.microsoft.com/debian/12/prod bookworm InRelease' is not signed.
`

	issues := parseAptRepoIssues(output)
	if len(issues) != 2 {
		t.Fatalf("got %d issues, want 2: %#v", len(issues), issues)
	}

	ms, ok := findIssue(issues, "packages.microsoft.com bookworm")
	if !ok || ms.Reason != "missing GPG key EB3E94ADBE1229CF" {
		t.Fatalf("microsoft repo: %#v", issues)
	}
	gone, ok := findIssue(issues, "repo.gone.invalid bookworm")
	if !ok || gone.Reason != "host does not resolve" {
		t.Fatalf("vanished repo: %#v", issues)
	}
}

func TestParseAptRepoIssuesHealthyOutput(t *testing.T) {
	output := `Hit:1 http://deb.debian.org/debian bookworm InRelease
Hit:2 http://deb.debian.org/debian bookworm-updates InRelease
Reading package lists...
`
	if issues := parseAptRepoIssues(output); len(issues) != 0 {
		t.Fatalf("healthy output produced issues: %#v", issues)
	}
}

func TestParseDnf4RepoIssues(t *testing.T) {
	output := `Vanished Repository                             0.0  B/s |   0  B     00:00
Errors during downloading metadata for repository 'gone-repo':
  - Curl error (6): Couldn't resolve host name for http://repo.gone.invalid/el9/repodata/repomd.xml [Could not resolve host: repo.gone.invalid]
Error: Failed to download metadata for repo 'gone-repo': Cannot download repomd.xml: Cannot download repodata/repomd.xml: All mirrors were tried
`

	issues := parseDnfRepoIssues(output)
	issue, ok := findIssue(issues, "gone-repo")
	if !ok {
		t.Fatalf("repository not identified: %#v", issues)
	}
	if issue.Reason != "host does not resolve" {
		t.Fatalf("reason = %q", issue.Reason)
	}
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want the repository collapsed into 1: %#v", len(issues), issues)
	}
}

func TestParseDnf4RepoIssues404(t *testing.T) {
	output := `Errors during downloading metadata for repository 'dead-path':
  - Status code: 404 for https://dl.rockylinux.org/pub/rocky/9/NoSuchRepo/x86_64/os/repodata/repomd.xml (IP: 199.232.194.132)
Error: Failed to download metadata for repo 'dead-path': Cannot download repomd.xml: All mirrors were tried
`

	issues := parseDnfRepoIssues(output)
	issue, ok := findIssue(issues, "dead-path")
	if !ok {
		t.Fatalf("repository not identified: %#v", issues)
	}
	if !strings.Contains(issue.Reason, "404") {
		t.Fatalf("reason = %q, want an HTTP 404 diagnosis", issue.Reason)
	}
}

// DNF5 exits 0 and prints "Metadata cache created." on stdout even when a repository failed
// outright; the error only appears on stderr and never names the repository.
func TestParseDnf5RepoIssuesAttributedViaRepoinfo(t *testing.T) {
	output := `Updating and loading repositories:
 Dead Path Repo                         100% |   1.0 KiB/s | 784.0   B |  00m01s
>>> Status code: 404 for https://dl.fedoraproject.org/pub/fedora/linux/releases/44/NoSuchRepo/x86_64/os/repodata/repomd.xml (IP: 38.145.32.23) - https://dl.fedoraproject.org/pub/fedora/linux/releases/44/NoSuchRepo/x86_64/os/repodata/repomd.xml
>>> Usable URL not found
Repositories loaded.
Metadata cache created.
`

	issues := parseDnfRepoIssues(output)
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1: %#v", len(issues), issues)
	}
	// Without repoinfo the best available label is the host.
	if issues[0].Repo != "dl.fedoraproject.org" {
		t.Fatalf("pre-attribution repo = %q", issues[0].Repo)
	}

	repoInfo := `Repo ID              : dead-path
Name                 : Dead Path Repo
Status               : enabled
URLs                 :
  Base URL           : https://dl.fedoraproject.org/pub/fedora/linux/releases/44/NoSuchRepo/x86_64/os/

Repo ID              : updates
Name                 : Fedora 44 - x86_64 - Updates
`

	attributed := attributeDnfRepoIssues(issues, repoInfo)
	if len(attributed) != 1 || attributed[0].Repo != "dead-path" {
		t.Fatalf("attribution failed: %#v", attributed)
	}
	if !strings.Contains(attributed[0].Reason, "404") {
		t.Fatalf("reason = %q", attributed[0].Reason)
	}
}

func TestParseDnf5RepoIssuesUnresolvableHost(t *testing.T) {
	// The same error repeats once per mirror attempt and must collapse to one entry.
	line := `>>> Curl error (6): Could not resolve hostname for http://repo.gone.invalid/f44/repodata/repomd.xml [Could not resolve host: repo.gone.invalid] - http://repo.gone.invalid/f44/repodata/repomd.xml`
	output := strings.Repeat(line+"\n", 4) + ">>> Usable URL not found\n"

	issues := parseDnfRepoIssues(output)
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want the repeated mirror errors collapsed into 1: %#v", len(issues), issues)
	}
	if issues[0].Reason != "host does not resolve" {
		t.Fatalf("reason = %q", issues[0].Reason)
	}
}

func TestParseDnfRepoIssuesHealthyOutput(t *testing.T) {
	output := `Updating and loading repositories:
 Fedora 44 - x86_64 - Updates           100% |   2.3 MiB/s |  10.5 MiB |  00m05s
Repositories loaded.
Metadata cache created.
`
	if issues := parseDnfRepoIssues(output); len(issues) != 0 {
		t.Fatalf("healthy output produced issues: %#v", issues)
	}
}

func TestShortenCellDoesNotSplitRunes(t *testing.T) {
	// Reasons contain em dashes; byte slicing would emit a replacement character.
	got := shortenCell("unreachable — all mirrors were tried", 14)
	if !utf8.ValidString(got) {
		t.Fatalf("shortenCell produced invalid UTF-8: %q", got)
	}
	if runes := []rune(got); len(runes) != 14 {
		t.Fatalf("got %d runes, want 14: %q", len(runes), got)
	}
}

func TestAttributeDnfRepoIssuesLeavesUnmatchedAlone(t *testing.T) {
	issues := []repoIssue{{Repo: "elsewhere.example", Reason: "unreachable", URL: "https://elsewhere.example/repodata/repomd.xml"}}
	repoInfo := "Repo ID              : updates\n  Base URL           : https://mirror.example/fedora/\n"

	got := attributeDnfRepoIssues(issues, repoInfo)
	if len(got) != 1 || got[0].Repo != "elsewhere.example" {
		t.Fatalf("unmatched issue was rewritten: %#v", got)
	}
}
