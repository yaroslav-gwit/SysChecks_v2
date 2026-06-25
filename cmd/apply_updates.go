package cmd

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"regexp"
	"strings"
	"syschecks/helpers"

	"github.com/spf13/cobra"
)

// Pre-compiled regex for package parsing
var (
	reWhitespace = regexp.MustCompile(`\s+`)
	reVersionDnf = regexp.MustCompile(`-\d+.+`)
)

var (
	applyUpdatesCmdSystemUpdates     bool
	applyUpdatesCmdIgnorePackageLock bool

	applyUpdatesCmd = &cobra.Command{
		Use:   "apply-updates",
		Short: "Apply updates",
		Long:  `Apply system or security updates.`,
		Run: func(cmd *cobra.Command, args []string) {
			updates := systemUpdates(false)
			osType := detectOs()

			var updateList []string
			if applyUpdatesCmdSystemUpdates {
				updateList = updates.SystemUpdatesList
			} else {
				updateList = updates.SecurityUpdatesList
			}

			applyUpdates(updateList, osType)
			// Refresh cache after applying updates
			checkUpdates(true, false, false)
		},
	}
)

// loadPackageLock loads the package lock file, returning empty slice if not found
func loadPackageLock() []string {
	const lockFile = "/opt/syschecks/package.lock.json"

	data, err := os.ReadFile(lockFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Note: Package lock file not found at %s, proceeding without locks", lockFile)
			return []string{}
		}
		log.Printf("Warning: Could not read package lock file: %v", err)
		return []string{}
	}

	var packageLock []string
	if err := json.Unmarshal(data, &packageLock); err != nil {
		log.Printf("Warning: Could not parse package lock file: %v", err)
		return []string{}
	}

	return packageLock
}

// extractPackageName extracts the package name from an update entry
func extractPackageName(entry string) string {
	// If the entry has whitespace, take the first part
	if reWhitespace.MatchString(entry) {
		parts := reWhitespace.Split(entry, -1)
		if len(parts) > 0 {
			return parts[0]
		}
	}
	// Otherwise strip version suffix (DNF style)
	return reVersionDnf.ReplaceAllString(entry, "")
}

// isPackageLocked checks if a package matches any lock pattern
func isPackageLocked(pkg string, lockPatterns []string) bool {
	for _, pattern := range lockPatterns {
		if strings.Contains(pkg, pattern) {
			return true
		}
	}
	return false
}

// applyUpdates applies the given list of updates using the appropriate package manager.
// It respects the package.lock.json file unless --ignore-lock-file is specified.
func applyUpdates(updateList []string, osType detectOsStruct) {
	helpers.RootUserCheck()

	if len(updateList) == 0 {
		log.Println("No updates to apply")
		return
	}

	packageLock := loadPackageLock()
	if applyUpdatesCmdIgnorePackageLock {
		packageLock = nil
	}

	// Deduplicate and extract package names
	seen := make(map[string]bool)
	var packages []string

	for _, entry := range updateList {
		pkg := extractPackageName(entry)
		if pkg == "" || seen[pkg] {
			continue
		}
		seen[pkg] = true
		packages = append(packages, pkg)
	}

	if osType.unsupported {
		log.Fatal("Unsupported OS")
	}
	pm := getPackageManager(osType)

	totalPkgs := len(packages)
	for i, pkg := range packages {
		pkgNum := i + 1

		// Check if package is locked
		if packageLock != nil && isPackageLocked(pkg, packageLock) {
			log.Printf("Package skipped (%d of %d): %s (locked in package.lock.json)\n", pkgNum, totalPkgs, pkg)
			continue
		}

		ctx, cancel := packageCommandTimeout()
		cmd := pm.ApplyCommand(ctx, pkg)

		out, err := cmd.CombinedOutput()
		cancel() // Clean up context immediately after command completes

		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("Package timed out (%d of %d): %s (exceeded 10min timeout)\n", pkgNum, totalPkgs, pkg)
			continue
		}

		if err != nil {
			// Trim output for logging
			outStr := strings.TrimSpace(string(out))
			if len(outStr) > 200 {
				outStr = outStr[:200] + "..."
			}
			log.Printf("Package error (%d of %d): %s (%s)\n", pkgNum, totalPkgs, pkg, outStr)
			continue
		}

		log.Printf("Package upgraded (%d of %d): %s\n", pkgNum, totalPkgs, pkg)
	}
}
