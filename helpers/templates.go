package helpers

import (
	"fmt"
	"log"
	"os"
)

const (
	SECURITY_UPDATES_JOB        = "/etc/cron.d/syschecks_updates_security"
	SYSTEM_UPDATES_JOB          = "/etc/cron.d/syschecks_updates_system"
	CACHE_JOB                   = "/etc/cron.d/syschecks_cache"
	AUTOUPDATE_JOB              = "/etc/cron.d/syschecks_autoupdate"
	KERNEL_CLEANUP_JOB          = "/etc/cron.d/syschecks_kernel_cleanup"
	LEGACY_CACHE_JOB            = "/etc/cron.d/syschecks"
	LEGACY_SECURITY_UPDATES_JOB = "/etc/cron.d/automatic_security_updates"
	LEGACY_SYSTEM_UPDATES_JOB   = "/etc/cron.d/automatic_system_updates"
	LEGACY_SYSTEM_HOLD_JOB      = "/etc/cron.d/automatic_system_updates_hold"
	CRON_FILE_PERMS             = 0644
)

type cronJobFile struct {
	path        string
	description string
}

var legacyUpdateJobs = []cronJobFile{
	{path: LEGACY_SECURITY_UPDATES_JOB, description: "security-only updates"},
	{path: LEGACY_SYSTEM_UPDATES_JOB, description: "full system updates"},
	{path: LEGACY_SYSTEM_HOLD_JOB, description: "held-system updates"},
}

// CacheCreate creates a cron job to update the syschecks cache periodically
func CacheCreate() {
	RootUserCheck()
	removeLegacyCronJob(LEGACY_CACHE_JOB, "cache")

	cronTemplate := `# This cron job will create a cache file for the system and/or security updates every 12 hours (or on boot, with a random delay in both cases)
# Created by syschecks
#

SHELL=/bin/bash
PATH=/sbin:/bin:/usr/sbin:/usr/bin
MAILTO=root

@reboot       root  sleep ${RANDOM:0:2} && syschecks updates refresh
7 */12 * * *  root  sleep ${RANDOM:0:2} && syschecks updates refresh
`
	writeCronFile(CACHE_JOB, cronTemplate)

	fmt.Printf("Created cache cron job: %s\n", CACHE_JOB)
}

// CacheDisable removes the update-cache refresh job if present.
func CacheDisable() {
	RootUserCheck()
	removeCronJobWithReport(CACHE_JOB, "cache")
	removeLegacyCronJob(LEGACY_CACHE_JOB, "cache")
}

// SecurityUpdates creates a cron job for automatic security updates
func SecurityUpdates() {
	RootUserCheck()
	removeConflictingUpdateJob(SYSTEM_UPDATES_JOB, "full system updates")
	removeLegacyUpdateJobs()

	cronTemplate := `# This cron job will apply security updates every day at 4:15 AM (with a random delay)
# Created by syschecks
#

SHELL=/bin/bash
PATH=/sbin:/bin:/usr/sbin:/usr/bin
MAILTO=root

COMMAND="syschecks updates apply --scope security"
LOG_FILE="/var/log/syschecks_updates.log"
15 4 * * *  root  sleep ${RANDOM:0:2} && touch ${LOG_FILE} && ${COMMAND} 2>&1 | tee -a ${LOG_FILE}
`
	writeCronFile(SECURITY_UPDATES_JOB, cronTemplate)

	fmt.Printf("Created security updates cron job: %s\n", SECURITY_UPDATES_JOB)
}

// SystemUpdates creates a cron job for automatic system updates
func SystemUpdates() {
	RootUserCheck()
	removeConflictingUpdateJob(SECURITY_UPDATES_JOB, "security-only updates")
	removeLegacyUpdateJobs()

	cronTemplate := `# This cron job will apply system updates every day at 4:15 AM (with a random delay)
# Created by syschecks
#

SHELL=/bin/bash
PATH=/sbin:/bin:/usr/sbin:/usr/bin
MAILTO=root

COMMAND="syschecks updates apply --scope system"
LOG_FILE="/var/log/syschecks_updates.log"
15 4 * * *  root  sleep ${RANDOM:0:2} && touch ${LOG_FILE} && ${COMMAND} 2>&1 | tee -a ${LOG_FILE}
`
	writeCronFile(SYSTEM_UPDATES_JOB, cronTemplate)

	fmt.Printf("Created system updates cron job: %s\n", SYSTEM_UPDATES_JOB)
}

// AutoUpdateEnable creates a cron job that keeps syschecks updated to the latest release
func AutoUpdateEnable() {
	RootUserCheck()

	cronTemplate := `# This cron job updates syschecks to the latest GitHub release every day at 03:30 (with a random delay)
# Created by syschecks
#

SHELL=/bin/bash
PATH=/sbin:/bin:/usr/sbin:/usr/bin
MAILTO=root

COMMAND="syschecks self-update"
LOG_FILE="/var/log/syschecks_selfupdate.log"
30 3 * * *  root  sleep ${RANDOM:0:2} && touch ${LOG_FILE} && ${COMMAND} 2>&1 | tee -a ${LOG_FILE}
`
	writeCronFile(AUTOUPDATE_JOB, cronTemplate)

	fmt.Printf("Created auto-update cron job: %s\n", AUTOUPDATE_JOB)
}

// AutoUpdateDisable removes the auto-update cron job if present
func AutoUpdateDisable() {
	RootUserCheck()
	removeCronJobWithReport(AUTOUPDATE_JOB, "auto-update")
}

// KernelCleanupEnable creates a weekly old-kernel cleanup job. Keeping at least
// two kernels ensures the running kernel and a recent fallback are protected.
func KernelCleanupEnable(numberToKeep int) {
	RootUserCheck()
	if numberToKeep < 2 {
		numberToKeep = 2
	}

	cronTemplate := fmt.Sprintf(`# This cron job safely removes old kernel packages every Sunday at 03:45 (with a random delay)
# Created by syschecks
#

SHELL=/bin/bash
PATH=/sbin:/bin:/usr/sbin:/usr/bin
MAILTO=root

COMMAND="syschecks kernel cleanup --yes --keep %d"
LOG_FILE="/var/log/syschecks_kernel_cleanup.log"
45 3 * * 0  root  sleep ${RANDOM:0:2} && touch ${LOG_FILE} && ${COMMAND} 2>&1 | tee -a ${LOG_FILE}
`, numberToKeep)
	writeCronFile(KERNEL_CLEANUP_JOB, cronTemplate)

	fmt.Printf("Created kernel cleanup cron job: %s\n", KERNEL_CLEANUP_JOB)
}

// writeCronFile applies the final mode explicitly after creation. os.WriteFile's mode is
// filtered through the caller's umask and does not update an existing file's permissions;
// either case could leave a root-created 0600 file invisible to regular-user banners.
func writeCronFile(path string, contents string) {
	if err := os.WriteFile(path, []byte(contents), CRON_FILE_PERMS); err != nil {
		log.Fatalf("Error writing cron file %s: %v", path, err)
	}
	if err := os.Chmod(path, CRON_FILE_PERMS); err != nil {
		log.Fatalf("Error setting cron file permissions on %s: %v", path, err)
	}
}

// KernelCleanupDisable removes the old-kernel cleanup job if present.
func KernelCleanupDisable() {
	RootUserCheck()
	removeCronJobWithReport(KERNEL_CLEANUP_JOB, "kernel cleanup")
}

// UpdatesDisable removes both mutually exclusive automatic-update modes.
func UpdatesDisable() {
	RootUserCheck()
	removedSecurity, err := removeCronJob(SECURITY_UPDATES_JOB)
	if err != nil {
		log.Fatalf("Error removing cron file %s: %v", SECURITY_UPDATES_JOB, err)
	}
	removedSystem, err := removeCronJob(SYSTEM_UPDATES_JOB)
	if err != nil {
		log.Fatalf("Error removing cron file %s: %v", SYSTEM_UPDATES_JOB, err)
	}

	if removedSecurity {
		fmt.Printf("Removed security-only updates cron job: %s\n", SECURITY_UPDATES_JOB)
	}
	if removedSystem {
		fmt.Printf("Removed full system updates cron job: %s\n", SYSTEM_UPDATES_JOB)
	}
	removedLegacy := removeLegacyUpdateJobs()
	if !removedSecurity && !removedSystem && !removedLegacy {
		fmt.Println("Automatic updates cron jobs are not enabled")
	}
}

func removeConflictingUpdateJob(path string, description string) {
	removed, err := removeCronJob(path)
	if err != nil {
		log.Fatalf("Error removing conflicting cron file %s: %v", path, err)
	}
	if removed {
		fmt.Printf("Warning: removed conflicting %s cron job: %s\n", description, path)
	}
}

func removeLegacyUpdateJobs() bool {
	removedAny := false
	for _, job := range legacyUpdateJobs {
		if removeLegacyCronJob(job.path, job.description) {
			removedAny = true
		}
	}
	return removedAny
}

func removeLegacyCronJob(path string, description string) bool {
	removed, err := removeCronJob(path)
	if err != nil {
		log.Fatalf("Error removing legacy cron file %s: %v", path, err)
	}
	if removed {
		fmt.Printf("Warning: removed duplicate or incompatible legacy %s cron job: %s\n", description, path)
	}
	return removed
}

func removeCronJobWithReport(path string, description string) {
	removed, err := removeCronJob(path)
	if err != nil {
		log.Fatalf("Error removing cron file %s: %v", path, err)
	}
	if !removed {
		fmt.Printf("%s cron job not present: %s\n", description, path)
		return
	}
	fmt.Printf("Removed %s cron job: %s\n", description, path)
}

func removeCronJob(path string) (bool, error) {
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
