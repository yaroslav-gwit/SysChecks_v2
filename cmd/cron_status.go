package cmd

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"syschecks/helpers"
)

type cronJobDefinition struct {
	name            string
	path            string
	legacyPaths     []string
	defaultSchedule string
	enableAction    string
	disableAction   string
}

type cronJobStatus struct {
	name     string
	state    string
	schedule string
	action   string
	active   bool
	legacy   bool
	unknown  bool
	cached   bool
}

type automaticOSUpdateMode string

const (
	automaticOSUpdatesOff      automaticOSUpdateMode = "off"
	automaticOSUpdatesSecurity automaticOSUpdateMode = "security"
	automaticOSUpdatesSystem   automaticOSUpdateMode = "system"
	automaticOSUpdatesConflict automaticOSUpdateMode = "conflict"
	automaticOSUpdatesUnknown  automaticOSUpdateMode = "unknown"
)

var cronJobDefinitions = []cronJobDefinition{
	{
		name:            "Update cache",
		path:            helpers.CACHE_JOB,
		legacyPaths:     []string{helpers.LEGACY_CACHE_JOB},
		defaultSchedule: "At reboot + every 12h at :07",
		enableAction:    "schedule enable update-cache",
		disableAction:   "schedule disable update-cache",
	},
	{
		name:            "Security updates",
		path:            helpers.SECURITY_UPDATES_JOB,
		legacyPaths:     []string{helpers.LEGACY_SECURITY_UPDATES_JOB},
		defaultSchedule: "Daily 04:15",
		enableAction:    "schedule enable updates --scope security",
		disableAction:   "schedule disable updates",
	},
	{
		name:            "Full system updates",
		path:            helpers.SYSTEM_UPDATES_JOB,
		legacyPaths:     []string{helpers.LEGACY_SYSTEM_UPDATES_JOB, helpers.LEGACY_SYSTEM_HOLD_JOB},
		defaultSchedule: "Daily 04:15",
		enableAction:    "schedule enable updates --scope system",
		disableAction:   "schedule disable updates",
	},
	{
		name:            "Syschecks self-update",
		path:            helpers.AUTOUPDATE_JOB,
		defaultSchedule: "Daily 03:30",
		enableAction:    "schedule enable self-update",
		disableAction:   "schedule disable self-update",
	},
	{
		name:            "Kernel cleanup",
		path:            helpers.KERNEL_CLEANUP_JOB,
		defaultSchedule: "Sunday 03:45",
		enableAction:    "schedule enable kernel-cleanup",
		disableAction:   "schedule disable kernel-cleanup",
	},
}

// cronEnabledCell renders the at-a-glance on/off answer. The word carries the meaning on its
// own so the column still reads where emoji do not render (log capture, plain consoles); the
// symbol only makes a long list scannable without reading it.
func cronEnabledCell(status cronJobStatus) string {
	switch {
	case status.unknown:
		return "❔ unknown"
	case status.state == "CONFLICT":
		return "🛑 yes"
	case status.active && status.legacy:
		return "🟡 yes"
	case status.active:
		return "✅ yes"
	default:
		return "❌ no"
	}
}

func printCronStatus() {
	statuses := collectCronJobStatuses(cronJobDefinitions)
	rows := make([][]string, 0, len(statuses))
	for _, status := range statuses {
		state := status.state
		if status.cached {
			state += " (cached)"
		}
		rows = append(rows, []string{status.name, cronEnabledCell(status), state, status.schedule, status.action})
	}

	fmt.Println("SysChecks cron jobs (schedule shown in server local time)")
	printTable([]string{"Job", "Enabled", "State", "Schedule", "Manage with `syschecks ...`"}, rows)

	if automaticUpdateConflict(statuses) {
		fmt.Println("Warning: security-only and full-system update jobs are both active. Enable either mode to remove the conflicting job, or run `syschecks schedule disable updates`.")
	}
}

func collectCronJobStatuses(definitions []cronJobDefinition) []cronJobStatus {
	statuses := collectLiveCronJobStatuses(definitions)
	snapshot, hasSnapshot := readScheduleSnapshot()
	if hasSnapshot {
		cached := make(map[string]cronJobSnapshot, len(snapshot.Jobs))
		for _, job := range snapshot.Jobs {
			cached[job.Name] = job
		}
		for i := range statuses {
			if !statuses[i].unknown {
				continue
			}
			job, ok := cached[statuses[i].name]
			if !ok || job.Unknown {
				continue
			}
			statuses[i] = cronJobStatus{
				name: job.Name, state: job.State, schedule: job.Schedule, action: job.Action,
				active: job.Active, legacy: job.Legacy, cached: true,
			}
		}
	}
	markAutomaticUpdateConflict(statuses)
	return statuses
}

func collectLiveCronJobStatuses(definitions []cronJobDefinition) []cronJobStatus {
	statuses := make([]cronJobStatus, 0, len(definitions))
	for _, definition := range definitions {
		statuses = append(statuses, inspectCronJob(definition))
	}
	markAutomaticUpdateConflict(statuses)
	return statuses
}

func markAutomaticUpdateConflict(statuses []cronJobStatus) {
	if automaticUpdateConflict(statuses) {
		for i := range statuses {
			if statuses[i].name == "Security updates" || statuses[i].name == "Full system updates" {
				statuses[i].state = "CONFLICT"
			}
		}
	}
}

func inspectCronJob(definition cronJobDefinition) cronJobStatus {
	primaryState, primarySchedules := readCronSchedules(definition.path)
	legacySchedules := make([]string, 0)
	unknown := primaryState == cronFileUnreadable
	for _, path := range definition.legacyPaths {
		state, schedules := readCronSchedules(path)
		unknown = unknown || state == cronFileUnreadable
		for _, schedule := range schedules {
			legacySchedules = append(legacySchedules, schedule+" (legacy)")
		}
	}
	if unknown {
		return cronJobStatus{
			name: definition.name, state: "unknown", schedule: "unavailable",
			action: "run with sudo: syschecks schedule list", unknown: true,
		}
	}

	primaryExists := primaryState == cronFileReadable
	active := len(primarySchedules) > 0 || len(legacySchedules) > 0
	legacy := len(legacySchedules) > 0
	state := "disabled"
	switch {
	case len(primarySchedules) > 0 && legacy:
		state = "enabled + legacy"
	case len(primarySchedules) > 0:
		state = "enabled"
	case legacy:
		state = "legacy enabled"
	case primaryExists:
		state = "inactive file"
	}

	schedule := definition.defaultSchedule + " (default)"
	if active {
		formatted := make([]string, 0, len(primarySchedules)+len(legacySchedules))
		for _, value := range primarySchedules {
			formatted = append(formatted, formatCronSchedule(value))
		}
		for _, value := range legacySchedules {
			value = strings.TrimSuffix(value, " (legacy)")
			formatted = append(formatted, formatCronSchedule(value)+" (legacy)")
		}
		schedule = strings.Join(uniqueStrings(formatted), "; ")
	}

	action := definition.enableAction
	if active || primaryExists {
		action = definition.disableAction
	}

	return cronJobStatus{
		name:     definition.name,
		state:    state,
		schedule: schedule,
		action:   action,
		active:   active,
		legacy:   legacy,
	}
}

type cronFileReadState int

const (
	cronFileMissing cronFileReadState = iota
	cronFileReadable
	cronFileUnreadable
)

var openCronFile = os.Open

func readCronSchedules(path string) (cronFileReadState, []string) {
	file, err := openCronFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cronFileMissing, nil
		}
		return cronFileUnreadable, nil
	}
	defer file.Close()

	var schedules []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.Contains(strings.Fields(line)[0], "=") {
			continue
		}
		fields := strings.Fields(line)
		if strings.HasPrefix(fields[0], "@") {
			if len(fields) >= 3 {
				schedules = append(schedules, fields[0])
			}
			continue
		}
		if len(fields) >= 7 && validCronTimeFields(fields[:5]) {
			schedules = append(schedules, strings.Join(fields[:5], " "))
		}
	}
	if scanner.Err() != nil {
		return cronFileUnreadable, nil
	}
	return cronFileReadable, uniqueStrings(schedules)
}

func validCronTimeFields(fields []string) bool {
	if len(fields) != 5 {
		return false
	}
	for _, field := range fields {
		if field == "" || strings.Contains(field, "=") {
			return false
		}
	}
	return true
}

func formatCronSchedule(expression string) string {
	if expression == "@reboot" {
		return "At reboot"
	}
	fields := strings.Fields(expression)
	if len(fields) != 5 {
		return expression
	}

	minute, hour, dayOfMonth, month, dayOfWeek := fields[0], fields[1], fields[2], fields[3], fields[4]
	if dayOfMonth == "*" && month == "*" {
		if strings.HasPrefix(hour, "*/") && dayOfWeek == "*" {
			return fmt.Sprintf("Every %sh at :%s", strings.TrimPrefix(hour, "*/"), padCronNumber(minute))
		}
		if _, errHour := strconv.Atoi(hour); errHour == nil {
			if _, errMinute := strconv.Atoi(minute); errMinute == nil {
				timeValue := padCronNumber(hour) + ":" + padCronNumber(minute)
				if dayOfWeek == "*" {
					return "Daily " + timeValue
				}
				if dayName, ok := cronDayName(dayOfWeek); ok {
					return dayName + " " + timeValue
				}
			}
		}
	}
	return expression
}

func cronDayName(value string) (string, bool) {
	days := map[string]string{"0": "Sunday", "1": "Monday", "2": "Tuesday", "3": "Wednesday", "4": "Thursday", "5": "Friday", "6": "Saturday", "7": "Sunday"}
	day, ok := days[value]
	return day, ok
}

func padCronNumber(value string) string {
	if len(value) == 1 {
		return "0" + value
	}
	return value
}

func automaticUpdateConflict(statuses []cronJobStatus) bool {
	var securityActive, systemActive bool
	for _, status := range statuses {
		switch status.name {
		case "Security updates":
			securityActive = status.active
		case "Full system updates":
			systemActive = status.active
		}
	}
	return securityActive && systemActive
}

func currentAutomaticOSUpdateMode() automaticOSUpdateMode {
	return automaticOSUpdateModeForStatuses(collectCronJobStatuses(cronJobDefinitions))
}

func automaticOSUpdateModeForStatuses(statuses []cronJobStatus) automaticOSUpdateMode {
	var securityActive, systemActive bool
	for _, status := range statuses {
		switch status.name {
		case "Security updates":
			if status.unknown {
				return automaticOSUpdatesUnknown
			}
			securityActive = status.active
		case "Full system updates":
			if status.unknown {
				return automaticOSUpdatesUnknown
			}
			systemActive = status.active
		}
	}
	switch {
	case securityActive && systemActive:
		return automaticOSUpdatesConflict
	case systemActive:
		return automaticOSUpdatesSystem
	case securityActive:
		return automaticOSUpdatesSecurity
	default:
		return automaticOSUpdatesOff
	}
}

func selfUpdateStateForStatuses(statuses []cronJobStatus) (bool, bool) {
	for _, status := range statuses {
		if status.name != "Syschecks self-update" {
			continue
		}
		if status.unknown {
			return false, false
		}
		return status.active, true
	}
	return false, false
}

func scheduleStatusSource(statuses []cronJobStatus) string {
	hasCached := false
	for _, status := range statuses {
		if status.unknown {
			return "unknown"
		}
		hasCached = hasCached || status.cached
	}
	if hasCached {
		return "cache"
	}
	return "live"
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}
