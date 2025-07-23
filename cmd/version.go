package cmd

import (
	"fmt"
	"runtime"
	"strings"
)

// Version information - set at build time
var (
	Version   = "dev"     // Set via -ldflags at build time
	GitCommit = "unknown" // Set via -ldflags at build time
	BuildDate = "unknown" // Set via -ldflags at build time
	GoVersion = runtime.Version()
	Platform  = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
)

// GetVersion returns the version string
func GetVersion() string {
	if Version == "dev" || Version == "" {
		return "development"
	}
	return Version
}

// GetFullVersion returns detailed version information
func GetFullVersion() string {
	var parts []string

	// Version
	version := GetVersion()
	parts = append(parts, fmt.Sprintf("Version: %s", version))

	// Git commit (if available and not unknown)
	if GitCommit != "unknown" && GitCommit != "" {
		// Truncate commit hash to 8 characters
		commit := GitCommit
		if len(commit) > 8 {
			commit = commit[:8]
		}
		parts = append(parts, fmt.Sprintf("Commit: %s", commit))
	}

	// Build date (if available and not unknown)
	if BuildDate != "unknown" && BuildDate != "" {
		parts = append(parts, fmt.Sprintf("Built: %s", BuildDate))
	}

	// Go version and platform
	parts = append(parts, fmt.Sprintf("Go: %s", GoVersion))
	parts = append(parts, fmt.Sprintf("Platform: %s", Platform))

	return strings.Join(parts, "\n")
}

// GetShortVersion returns just the version number
func GetShortVersion() string {
	version := GetVersion()
	// Remove 'v' prefix if present
	if strings.HasPrefix(version, "v") {
		return version[1:]
	}
	return version
}
