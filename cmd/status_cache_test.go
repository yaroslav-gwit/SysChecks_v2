package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func useTemporaryStatusCache(t *testing.T) string {
	t.Helper()
	oldPath := statusCacheFile
	oldUID := trustedStatusCacheUID
	statusCacheFile = filepath.Join(t.TempDir(), "syscheck_updates.json")
	trustedStatusCacheUID = uint32(os.Geteuid())
	t.Cleanup(func() {
		statusCacheFile = oldPath
		trustedStatusCacheUID = oldUID
	})
	return statusCacheFile
}

func TestStatusCacheRejectsWritableFile(t *testing.T) {
	path := useTemporaryStatusCache(t)
	if err := os.WriteFile(path, []byte(`{"system_updates_status":"ok"}`), 0666); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0666); err != nil {
		t.Fatal(err)
	}
	if _, err := readTrustedStatusCache(); err == nil {
		t.Fatal("group/world-writable status cache was trusted")
	}
}

func denyCronReads(t *testing.T) {
	t.Helper()
	oldOpen := openCronFile
	openCronFile = func(string) (*os.File, error) { return nil, os.ErrPermission }
	t.Cleanup(func() { openCronFile = oldOpen })
}

func TestUnreadableCronUsesCachedScheduleSnapshot(t *testing.T) {
	useTemporaryStatusCache(t)
	definitions := []cronJobDefinition{
		{name: "Security updates", path: "/protected/security"},
		{name: "Full system updates", path: "/protected/system"},
		{name: "Syschecks self-update", path: "/protected/self-update"},
	}
	wantStatuses := []cronJobStatus{
		{name: "Security updates", state: "disabled", schedule: "Daily 04:15 (default)", action: "enable security"},
		{name: "Full system updates", state: "enabled", schedule: "Daily 04:15", action: "disable updates", active: true},
		{name: "Syschecks self-update", state: "enabled", schedule: "Daily 03:30", action: "disable self-update", active: true},
	}
	if err := mergeStatusCache(func(document map[string]json.RawMessage) error {
		raw, err := json.Marshal(makeScheduleSnapshot(wantStatuses))
		document[scheduleCacheKey] = raw
		return err
	}); err != nil {
		t.Fatal(err)
	}
	denyCronReads(t)

	got := collectCronJobStatuses(definitions)
	if mode := automaticOSUpdateModeForStatuses(got); mode != automaticOSUpdatesSystem {
		t.Fatalf("automatic update mode = %q, want system", mode)
	}
	if enabled, known := selfUpdateStateForStatuses(got); !known || !enabled {
		t.Fatalf("self-update state = %v, known=%v; want true, true", enabled, known)
	}
	if source := scheduleStatusSource(got); source != "cache" {
		t.Fatalf("schedule source = %q, want cache", source)
	}
	for _, status := range got {
		if !status.cached || status.unknown {
			t.Fatalf("cached status was not used: %#v", status)
		}
	}
}

func TestUnreadableCronWithoutSnapshotIsUnknownNotOff(t *testing.T) {
	useTemporaryStatusCache(t)
	denyCronReads(t)
	statuses := collectCronJobStatuses([]cronJobDefinition{
		{name: "Security updates", path: "/protected/security"},
		{name: "Full system updates", path: "/protected/system"},
		{name: "Syschecks self-update", path: "/protected/self-update"},
	})

	if mode := automaticOSUpdateModeForStatuses(statuses); mode != automaticOSUpdatesUnknown {
		t.Fatalf("automatic update mode = %q, want unknown", mode)
	}
	if enabled, known := selfUpdateStateForStatuses(statuses); known || enabled {
		t.Fatalf("self-update state = %v, known=%v; want false, false", enabled, known)
	}
	if source := scheduleStatusSource(statuses); source != "unknown" {
		t.Fatalf("schedule source = %q, want unknown", source)
	}
}

func TestStatusCacheOverridesAggressiveUmask(t *testing.T) {
	path := useTemporaryStatusCache(t)
	oldUmask := syscall.Umask(0077)
	t.Cleanup(func() { syscall.Umask(oldUmask) })

	if err := mergeStatusCache(func(document map[string]json.RawMessage) error {
		document["test"] = json.RawMessage(`true`)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0644 {
		t.Fatalf("status cache mode = %04o, want 0644", got)
	}
}

func TestScheduleOnlyCacheDoesNotPretendUpdatesWereChecked(t *testing.T) {
	useTemporaryStatusCache(t)
	if err := mergeStatusCache(func(document map[string]json.RawMessage) error {
		raw, err := json.Marshal(makeScheduleSnapshot([]cronJobStatus{{name: "Update cache", state: "enabled", active: true}}))
		document[scheduleCacheKey] = raw
		return err
	}); err != nil {
		t.Fatal(err)
	}

	result := readCache(detectOsStruct{manager: packageManagerAPT})
	if result.CacheExists || result.CacheUpToDate || result.CacheDateCreated != "" {
		t.Fatalf("schedule-only document became an update cache: %#v", result)
	}
	if result.SystemUpdatesStatus != "unknown" || result.SecurityUpdatesStatus != "unknown" {
		t.Fatalf("missing updates were not unknown: %#v", result)
	}
}

func TestScheduleSnapshotWriteDoesNotRefreshUpdateAge(t *testing.T) {
	useTemporaryStatusCache(t)
	created := time.Now().Add(-24 * time.Hour).Format("2006-01-02 15:04:05")
	stale := systemUpdatesJsonStruct{
		SystemUpdatesList: []string{}, SecurityUpdatesList: []string{},
		SystemUpdatesStatus: "ok", SecurityUpdatesStatus: "ok",
		RepositoryIssues: []repoIssue{}, CacheExists: true, CacheUpToDate: true,
		CacheDateCreated: created,
	}
	if err := mergeStatusCache(func(document map[string]json.RawMessage) error {
		raw, err := json.Marshal(stale)
		if err != nil {
			return err
		}
		var updateFields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &updateFields); err != nil {
			return err
		}
		for key, value := range updateFields {
			document[key] = value
		}
		schedule, err := json.Marshal(makeScheduleSnapshot([]cronJobStatus{{name: "Update cache", state: "enabled", active: true}}))
		document[scheduleCacheKey] = schedule
		return err
	}); err != nil {
		t.Fatal(err)
	}

	result := readCache(detectOsStruct{manager: packageManagerAPT})
	if !result.CacheExists || result.CacheUpToDate {
		t.Fatalf("stale update result was freshened by schedule write: %#v", result)
	}
	if result.CacheDateCreated != created {
		t.Fatalf("update timestamp = %q, want %q", result.CacheDateCreated, created)
	}
}

func TestUnknownUpdateDataIsNeverHealthy(t *testing.T) {
	updates := systemUpdatesJsonStruct{SystemUpdatesStatus: "unknown", SecurityUpdatesStatus: "unknown"}
	if check := systemUpdateBannerCheck(updates); check.Healthy {
		t.Fatalf("unknown system updates reported healthy: %#v", check)
	}
	if check := securityUpdateBannerCheck(0, true, automaticOSUpdatesSystem, "unknown"); check.Healthy {
		t.Fatalf("unknown security updates reported healthy: %#v", check)
	}
	if detail := repositoryIssueDetail(nil, "unknown"); detail == "all repositories refreshed" {
		t.Fatalf("unknown repository state reported success: %q", detail)
	}
}
