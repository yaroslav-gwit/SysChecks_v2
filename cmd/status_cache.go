package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// statusCacheFile is shared by the update report and the schedule snapshot. Keeping both
// in one root-written, world-readable document lets hardened hosts keep /etc/cron.d private
// without making a regular-user login banner guess that every job is disabled.
var statusCacheFile = "/tmp/syscheck_updates.json"

// Writers are privileged commands, so a cache owned by anybody else is not authoritative.
// This matters in /tmp: before the first privileged write, an unprivileged user could create
// the predictable path and otherwise forge healthy update or schedule state.
var trustedStatusCacheUID uint32 = 0

const scheduleCacheKey = "schedule"

type cronJobSnapshot struct {
	Name     string `json:"name"`
	State    string `json:"state"`
	Schedule string `json:"schedule"`
	Action   string `json:"action"`
	Active   bool   `json:"active"`
	Legacy   bool   `json:"legacy"`
	Unknown  bool   `json:"unknown,omitempty"`
}

type scheduleSnapshot struct {
	CapturedAt string            `json:"captured_at"`
	Jobs       []cronJobSnapshot `json:"jobs"`
}

func makeScheduleSnapshot(statuses []cronJobStatus) scheduleSnapshot {
	jobs := make([]cronJobSnapshot, 0, len(statuses))
	for _, status := range statuses {
		jobs = append(jobs, cronJobSnapshot{
			Name: status.name, State: status.state, Schedule: status.schedule,
			Action: status.action, Active: status.active, Legacy: status.legacy,
			Unknown: status.unknown,
		})
	}
	return scheduleSnapshot{
		CapturedAt: time.Now().Format("2006-01-02 15:04:05"),
		Jobs:       jobs,
	}
}

func readScheduleSnapshot() (scheduleSnapshot, bool) {
	data, err := readTrustedStatusCache()
	if err != nil {
		return scheduleSnapshot{}, false
	}
	var document map[string]json.RawMessage
	if json.Unmarshal(data, &document) != nil {
		return scheduleSnapshot{}, false
	}
	raw, ok := document[scheduleCacheKey]
	if !ok {
		return scheduleSnapshot{}, false
	}
	var snapshot scheduleSnapshot
	if json.Unmarshal(raw, &snapshot) != nil || snapshot.CapturedAt == "" || len(snapshot.Jobs) == 0 {
		return scheduleSnapshot{}, false
	}
	return snapshot, true
}

func refreshScheduleStatusCache() error {
	// This function is called after privileged schedule mutations, from updates refresh, and
	// after migration. Under those callers the live cron directory should be readable. Never
	// replace a good snapshot with an unknown one if that assumption is violated.
	statuses := collectLiveCronJobStatuses(cronJobDefinitions)
	for _, status := range statuses {
		if status.unknown {
			return fmt.Errorf("cannot refresh schedule status cache: %s is unreadable", status.name)
		}
	}
	return mergeStatusCache(func(document map[string]json.RawMessage) error {
		raw, err := json.Marshal(makeScheduleSnapshot(statuses))
		if err != nil {
			return err
		}
		document[scheduleCacheKey] = raw
		return nil
	})
}

func writeUpdateStatusCache(result systemUpdatesJsonStruct) error {
	statuses := collectLiveCronJobStatuses(cronJobDefinitions)
	for _, status := range statuses {
		if status.unknown {
			return fmt.Errorf("cannot cache updates: schedule status for %s is unreadable", status.name)
		}
	}

	return mergeStatusCache(func(document map[string]json.RawMessage) error {
		updateJSON, err := json.Marshal(result)
		if err != nil {
			return err
		}
		var updateFields map[string]json.RawMessage
		if err := json.Unmarshal(updateJSON, &updateFields); err != nil {
			return err
		}
		for key, value := range updateFields {
			document[key] = value
		}
		scheduleJSON, err := json.Marshal(makeScheduleSnapshot(statuses))
		if err != nil {
			return err
		}
		document[scheduleCacheKey] = scheduleJSON
		return nil
	})
}

func mergeStatusCache(update func(map[string]json.RawMessage) error) error {
	document := make(map[string]json.RawMessage)
	if data, err := readTrustedStatusCache(); err == nil {
		// A damaged cache has no trustworthy fields worth preserving. The caller supplies a
		// complete replacement for the section it owns.
		var existing map[string]json.RawMessage
		if json.Unmarshal(data, &existing) == nil && existing != nil {
			document = existing
		}
	}
	if err := update(document); err != nil {
		return err
	}
	data, err := json.Marshal(document)
	if err != nil {
		return err
	}
	return atomicWriteStatusCache(data)
}

func readTrustedStatusCache() ([]byte, error) {
	file, err := os.Open(statusCacheFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("status cache is not a regular file")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != trustedStatusCacheUID {
		return nil, fmt.Errorf("status cache is not owned by the trusted user")
	}
	if info.Mode().Perm()&0022 != 0 {
		return nil, fmt.Errorf("status cache is writable by group or others")
	}
	return io.ReadAll(file)
}

func atomicWriteStatusCache(data []byte) error {
	dir := filepath.Dir(statusCacheFile)
	tmp, err := os.CreateTemp(dir, ".syschecks-status-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// Keep the temporary file private until its contents are complete, then make the final
	// root-owned snapshot readable immediately before the atomic rename.
	if err := os.Chmod(tmpPath, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, statusCacheFile); err != nil {
		return err
	}
	// Rename preserves the temporary file's explicit mode, but enforce it once more so this
	// invariant remains obvious and regression tests catch any future writer change.
	return os.Chmod(statusCacheFile, 0644)
}
