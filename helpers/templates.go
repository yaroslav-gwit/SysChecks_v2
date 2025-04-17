package helpers

import (
	"log"
	"os"
)

const (
	// SYSTEM_UPDATES_HOLD_JOB = "/etc/cron.d/automatic_system_updates_hold"
	// SECURITY_UPDATES_JOB = "/etc/cron.d/automatic_security_updates"
	// SYSTEM_UPDATES_JOB   = "/etc/cron.d/automatic_system_updates"
	SECURITY_UPDATES_JOB = "/etc/cron.d/syschecks_updates_security"
	SYSTEM_UPDATES_JOB   = "/etc/cron.d/syschecks_updates_system"
	CRON_FILE_PERMS      = 0644
)

func CacheCreate() {
	fileLocation := "/etc/cron.d/syschecks_cache"

	cronTemplate := `# This cron job will create a cache file for the system and/or security updates every 12 hours (or on boot, with a random delay in both cases)
# Created by syschecks
#

SHELL=/bin/bash
PATH=/sbin:/bin:/usr/sbin:/usr/bin
MAILTO=root

@reboot       root  sleep ${RANDOM:0:2} && syschecks updates --cache-create
7 */12 * * *  root  sleep ${RANDOM:0:2} && syschecks updates --cache-create
`
	err := os.WriteFile(fileLocation, []byte(cronTemplate), CRON_FILE_PERMS)
	if err != nil {
		log.Fatal("There was an error trying to write your cron file: " + fileLocation + ". Error: " + err.Error())
	}
}

func SecurityUpdates() {
	removeOldJobs()
	fileLocation := SECURITY_UPDATES_JOB

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

	err := os.WriteFile(fileLocation, []byte(cronTemplate), CRON_FILE_PERMS)
	if err != nil {
		log.Fatal("There was an error trying to write your cron file: " + fileLocation + ". Error: " + err.Error())
	}
}

func SystemUpdates() {
	removeOldJobs()
	fileLocation := SYSTEM_UPDATES_JOB

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

	err := os.WriteFile(fileLocation, []byte(cronTemplate), CRON_FILE_PERMS)
	if err != nil {
		log.Fatal("There was an error trying to write your cron file: " + fileLocation + ". Error: " + err.Error())
	}
}

func removeOldJobs() {
	// _ = os.Remove(SYSTEM_UPDATES_HOLD_JOB)
	_ = os.Remove(SECURITY_UPDATES_JOB)
	_ = os.Remove(SYSTEM_UPDATES_JOB)
}

// func SystemUpdatesHold() {
// 	removeOldJobs()
// 	fileLocation := SYSTEM_UPDATES_HOLD_JOB

// 	cronTemplate := `5 4 * * * root /opt/syschecks/automatic_system_updates_hold.sh
// `
// 	err := os.WriteFile(fileLocation, []byte(cronTemplate), CRON_FILE_PERMS)
// 	if err != nil {
// 		log.Fatal("There was an error trying to write your cron file: " + fileLocation + ". Error: " + err.Error())
// 	}
// }
