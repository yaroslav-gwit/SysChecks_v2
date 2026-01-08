package helpers

import (
	"fmt"
	"log"
	"os"
)

const (
	SECURITY_UPDATES_JOB = "/etc/cron.d/syschecks_updates_security"
	SYSTEM_UPDATES_JOB   = "/etc/cron.d/syschecks_updates_system"
	CACHE_JOB            = "/etc/cron.d/syschecks_cache"
	CRON_FILE_PERMS      = 0644
)

// CacheCreate creates a cron job to update the syschecks cache periodically
func CacheCreate() {
	RootUserCheck()

	cronTemplate := `# This cron job will create a cache file for the system and/or security updates every 12 hours (or on boot, with a random delay in both cases)
# Created by syschecks
#

SHELL=/bin/bash
PATH=/sbin:/bin:/usr/sbin:/usr/bin
MAILTO=root

@reboot       root  sleep ${RANDOM:0:2} && syschecks updates --cache-create
7 */12 * * *  root  sleep ${RANDOM:0:2} && syschecks updates --cache-create
`
	if err := os.WriteFile(CACHE_JOB, []byte(cronTemplate), CRON_FILE_PERMS); err != nil {
		log.Fatalf("Error writing cron file %s: %v", CACHE_JOB, err)
	}

	fmt.Printf("Created cache cron job: %s\n", CACHE_JOB)
}

// SecurityUpdates creates a cron job for automatic security updates
func SecurityUpdates() {
	RootUserCheck()
	removeOldJobs()

	cronTemplate := `# This cron job will apply security updates every day at 4:15 AM (with a random delay)
# Created by syschecks
#

SHELL=/bin/bash
PATH=/sbin:/bin:/usr/sbin:/usr/bin
MAILTO=root

COMMAND="syschecks apply-updates"
LOG_FILE="/var/log/syschecks_updates.log"
15 4 * * *  root  sleep ${RANDOM:0:2} && touch ${LOG_FILE} && ${COMMAND} 2>&1 | tee -a ${LOG_FILE}
`
	if err := os.WriteFile(SECURITY_UPDATES_JOB, []byte(cronTemplate), CRON_FILE_PERMS); err != nil {
		log.Fatalf("Error writing cron file %s: %v", SECURITY_UPDATES_JOB, err)
	}

	fmt.Printf("Created security updates cron job: %s\n", SECURITY_UPDATES_JOB)
}

// SystemUpdates creates a cron job for automatic system updates
func SystemUpdates() {
	RootUserCheck()
	removeOldJobs()

	cronTemplate := `# This cron job will apply system updates every day at 4:15 AM (with a random delay)
# Created by syschecks
#

SHELL=/bin/bash
PATH=/sbin:/bin:/usr/sbin:/usr/bin
MAILTO=root

COMMAND="syschecks apply-updates --system"
LOG_FILE="/var/log/syschecks_updates.log"
15 4 * * *  root  sleep ${RANDOM:0:2} && touch ${LOG_FILE} && ${COMMAND} 2>&1 | tee -a ${LOG_FILE}
`
	if err := os.WriteFile(SYSTEM_UPDATES_JOB, []byte(cronTemplate), CRON_FILE_PERMS); err != nil {
		log.Fatalf("Error writing cron file %s: %v", SYSTEM_UPDATES_JOB, err)
	}

	fmt.Printf("Created system updates cron job: %s\n", SYSTEM_UPDATES_JOB)
}

// removeOldJobs removes any existing update cron jobs to avoid conflicts
func removeOldJobs() {
	// Ignore errors - files may not exist
	_ = os.Remove(SECURITY_UPDATES_JOB)
	_ = os.Remove(SYSTEM_UPDATES_JOB)
}
