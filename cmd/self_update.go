package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"syschecks/helpers"

	"github.com/spf13/cobra"
)

// githubRepo is the owner/repo that publishes syschecks releases.
const githubRepo = "yaroslav-gwit/SysChecks_v2"

var (
	selfUpdateCheckOnly bool
	selfUpdateForce     bool

	selfUpdateCmd = &cobra.Command{
		Use:   "self-update",
		Short: "Update syschecks to the latest GitHub release",
		Long: `Check the latest GitHub release and, if a newer version is available,
download the matching binary and replace the running executable in place.

The update is a no-op when the installed version already matches the latest
release (use --force to reinstall anyway, or --check to only report).`,
		Run: func(cmd *cobra.Command, args []string) {
			selfUpdate(selfUpdateCheckOnly, selfUpdateForce)
		},
	}
)

type githubAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type githubRelease struct {
	TagName    string        `json:"tag_name"`
	Prerelease bool          `json:"prerelease"`
	Assets     []githubAsset `json:"assets"`
}

func selfUpdate(checkOnly, force bool) {
	rel, err := fetchLatestRelease(githubRepo)
	if err != nil {
		log.Fatalf("Error checking for updates: %v", err)
	}

	current := GetShortVersion()
	latest := strings.TrimPrefix(rel.TagName, "v")

	if compareVersions(current, latest) >= 0 && !force {
		fmt.Printf("syschecks is up to date (current: %s, latest: %s)\n", displayVersion(current), rel.TagName)
		return
	}

	if checkOnly {
		fmt.Printf("Update available: %s -> %s\n", displayVersion(current), rel.TagName)
		return
	}

	// Replacing the binary in /opt/syschecks needs write access there.
	helpers.RootUserCheck()

	assetName := fmt.Sprintf("syschecks-%s-%s", runtime.GOOS, runtime.GOARCH)
	asset := findAsset(rel, assetName)
	if asset == nil {
		log.Fatalf("Release %s has no asset %q for this platform", rel.TagName, assetName)
	}

	// Resolve the real file behind any symlinks (e.g. /bin/syschecks -> /opt/syschecks/syschecks).
	target, err := resolveSelfPath()
	if err != nil {
		log.Fatalf("Error locating current binary: %v", err)
	}

	expectedSum := fetchExpectedChecksum(rel, assetName)

	fmt.Printf("Downloading %s (%s)...\n", assetName, rel.TagName)
	// Download into the target's directory so the final rename is atomic (same filesystem).
	tmpPath, gotSum, err := downloadToDir(asset.URL, filepath.Dir(target))
	if err != nil {
		log.Fatalf("Error downloading update: %v", err)
	}
	defer os.Remove(tmpPath) // no-op once the rename succeeds; cleans up on any failure

	if expectedSum == "" {
		log.Printf("Warning: no checksum published for %s; skipping verification", assetName)
	} else if !strings.EqualFold(expectedSum, gotSum) {
		log.Fatalf("Checksum mismatch for %s: expected %s, got %s", assetName, expectedSum, gotSum)
	}

	if err := installBinary(tmpPath, target); err != nil {
		log.Fatalf("Error installing update: %v", err)
	}

	fmt.Printf("Updated syschecks %s -> %s\n", displayVersion(current), rel.TagName)
}

// fetchLatestRelease queries the GitHub API for the latest published release.
func fetchLatestRelease(repo string) (*githubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "syschecks-self-update")
	// Optional token raises the API rate limit and allows private repos.
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GitHub API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	if rel.TagName == "" {
		return nil, fmt.Errorf("no released version found for %s", repo)
	}
	return &rel, nil
}

func findAsset(rel *githubRelease, name string) *githubAsset {
	for i := range rel.Assets {
		if rel.Assets[i].Name == name {
			return &rel.Assets[i]
		}
	}
	return nil
}

// fetchExpectedChecksum returns the published SHA-256 for assetName, or "" if unavailable.
func fetchExpectedChecksum(rel *githubRelease, assetName string) string {
	var checksumURL string
	for _, a := range rel.Assets {
		if strings.HasPrefix(a.Name, "checksums") {
			checksumURL = a.URL
			break
		}
	}
	if checksumURL == "" {
		return ""
	}

	body, err := httpGet(checksumURL)
	if err != nil {
		return ""
	}
	defer body.Close()

	data, err := io.ReadAll(io.LimitReader(body, 1<<20))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		// Format: "<sha256>  <filename>" (filename may carry a leading '*' in binary mode).
		if len(fields) >= 2 && strings.TrimPrefix(fields[len(fields)-1], "*") == assetName {
			return fields[0]
		}
	}
	return ""
}

// httpGet performs a GET and returns the body for a 200 response.
func httpGet(url string) (io.ReadCloser, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "syschecks-self-update")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download failed: %s", resp.Status)
	}
	return resp.Body, nil
}

// downloadToDir streams url into a temp file inside dir and returns its path and SHA-256.
func downloadToDir(url, dir string) (string, string, error) {
	body, err := httpGet(url)
	if err != nil {
		return "", "", err
	}
	defer body.Close()

	tmp, err := os.CreateTemp(dir, ".syschecks-update-*")
	if err != nil {
		return "", "", err
	}
	tmpPath := tmp.Name()

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), body); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return "", "", err
	}
	return tmpPath, hex.EncodeToString(h.Sum(nil)), nil
}

// installBinary atomically replaces target with the file at tmpPath, preserving the
// original mode and ownership. On Linux a running executable can be renamed over: the
// live process keeps the old inode and the next invocation uses the new file.
func installBinary(tmpPath, target string) error {
	mode := os.FileMode(0755)
	if info, err := os.Stat(target); err == nil {
		mode = info.Mode().Perm()
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			_ = os.Chown(tmpPath, int(st.Uid), int(st.Gid))
		}
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return err
	}
	return os.Rename(tmpPath, target)
}

// resolveSelfPath returns the real path of the running binary, following symlinks.
func resolveSelfPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved, nil
	}
	return exe, nil
}

func displayVersion(v string) string {
	if v == "" || v == "dev" || v == "development" {
		return "development"
	}
	return "v" + strings.TrimPrefix(v, "v")
}

// compareVersions returns -1, 0 or 1 for a<b, a==b, a>b using a best-effort semantic
// comparison. An unparseable current version (e.g. a dev build) is treated as older so
// that updates proceed.
func compareVersions(a, b string) int {
	a = strings.TrimPrefix(strings.TrimSpace(a), "v")
	b = strings.TrimPrefix(strings.TrimSpace(b), "v")
	if a == b {
		return 0
	}

	pa, oka := parseSemver(a)
	pb, okb := parseSemver(b)
	switch {
	case !oka && !okb:
		return -1
	case !oka:
		return -1
	case !okb:
		return 1
	}

	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

// parseSemver extracts up to MAJOR.MINOR.PATCH, ignoring any pre-release/build suffix.
func parseSemver(v string) ([3]int, bool) {
	var out [3]int
	v = strings.SplitN(v, "-", 2)[0]
	v = strings.SplitN(v, "+", 2)[0]

	parts := strings.Split(v, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return out, false
	}
	for i := range parts {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
