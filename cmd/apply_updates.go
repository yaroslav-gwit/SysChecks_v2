package cmd

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"syschecks/helpers"
	"time"

	"github.com/spf13/cobra"
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

			if applyUpdatesCmdSystemUpdates {
				applyUpdates(updates.SystemUpdatesList, osType)
			} else {
				applyUpdates(updates.SecurityUpdatesList, osType)
			}
			checkUpdates(true, false, false)
		},
	}
)

// This function doesn't return anything, it just applies the system updates
// using a default system package manager (or executes log.fatal on error).
//
// It also uses the package.lock.json file to skip the packages that are locked.
func applyUpdates(updateList []string, osType detectOsStruct) {
	helpers.RootUserCheck()

	packageLock := []string{}
	file, err := os.ReadFile("/opt/syschecks/package.lock.json")
	if err != nil {
		log.Fatal("Error reading package lock file: " + err.Error())
	}

	err = json.Unmarshal(file, &packageLock)
	if err != nil {
		log.Fatal("Error parsing package lock file: " + err.Error())
	}

	finalList := []string{}

	reSpace := regexp.MustCompile(`\s+`)
	reVersionDnf := regexp.MustCompile(`-\d+.+`)

	for _, v := range updateList {
		if reSpace.MatchString(v) {
			v = reSpace.Split(v, -1)[0]
		} else {
			v = reVersionDnf.ReplaceAllString(v, "")
		}

		if !slices.Contains(finalList, v) {
			finalList = append(finalList, v)
		}
	}

	if osType.unsupported {
		log.Fatal("Unsupported OS")
	}

PACKAGE_LIST:
	for i, v := range finalList {
		if !applyUpdatesCmdIgnorePackageLock {
			for _, vv := range packageLock {
				if strings.Contains(v, vv) {
					log.Printf("Package skipped (%d of %d): %s (locked in the package.lock.json)\n", i+1, len(finalList), v)
					continue PACKAGE_LIST
				}
			}
		}

		// Create a context with a 10-minute timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		var cmd *exec.Cmd
		if osType.yum {
			cmd = exec.CommandContext(ctx, "yum", "update", "-y", v)
		}
		if osType.dnf {
			cmd = exec.CommandContext(ctx, "dnf", "update", "-y", v)
		}
		if osType.deb {
			cmd = exec.CommandContext(ctx, "apt-get", "install", "-y", v)
		}

		out, err := cmd.CombinedOutput()
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("Package timed out (%d of %d): %s (command exceeded 10min execution cap)\n", i+1, len(finalList), v)
			continue
		}
		if err != nil {
			log.Printf("Package error (%d of %d): %s (%s; %s)\n", i+1, len(finalList), v, strings.TrimSpace(string(out)), err.Error())
			continue
		}
		log.Printf("Package upgraded (%d of %d): %s\n", i+1, len(finalList), v)
	}
}
