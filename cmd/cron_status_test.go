package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCronSchedules(t *testing.T) {
	path := filepath.Join(t.TempDir(), "job")
	contents := `# disabled job
SHELL=/bin/bash
COMMAND="syschecks updates"
@reboot root syschecks updates --cache-create
7 */12 * * * root syschecks updates --cache-create
# 15 4 * * * root disabled
`
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}

	exists, schedules := readCronSchedules(path)
	if !exists || len(schedules) != 2 {
		t.Fatalf("readCronSchedules() = %v, %#v", exists, schedules)
	}
}

func TestFormatCronSchedule(t *testing.T) {
	tests := map[string]string{
		"@reboot":      "At reboot",
		"7 */12 * * *": "Every 12h at :07",
		"15 4 * * *":   "Daily 04:15",
		"45 3 * * 0":   "Sunday 03:45",
	}
	for expression, expected := range tests {
		if got := formatCronSchedule(expression); got != expected {
			t.Fatalf("formatCronSchedule(%q) = %q, want %q", expression, got, expected)
		}
	}
}

func TestInspectCronJobInactiveFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "job")
	if err := os.WriteFile(path, []byte("# disabled\n"), 0644); err != nil {
		t.Fatal(err)
	}
	status := inspectCronJob(cronJobDefinition{name: "Test", path: path, defaultSchedule: "Daily 01:00", enableAction: "enable", disableAction: "disable"})
	if status.state != "inactive file" || status.active || status.action != "disable" {
		t.Fatalf("unexpected status: %#v", status)
	}
}

func TestCollectCronJobStatusesMarksUpdateConflict(t *testing.T) {
	dir := t.TempDir()
	securityPath := filepath.Join(dir, "security")
	systemPath := filepath.Join(dir, "system")
	for _, path := range []string{securityPath, systemPath} {
		if err := os.WriteFile(path, []byte("15 4 * * * root syschecks apply-updates\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	statuses := collectCronJobStatuses([]cronJobDefinition{
		{name: "Security updates", path: securityPath},
		{name: "Full system updates", path: systemPath},
	})
	if len(statuses) != 2 || statuses[0].state != "CONFLICT" || statuses[1].state != "CONFLICT" {
		t.Fatalf("unexpected conflict states: %#v", statuses)
	}
}

func TestAutomaticOSUpdateModeForStatuses(t *testing.T) {
	tests := []struct {
		name     string
		security bool
		system   bool
		want     automaticOSUpdateMode
	}{
		{name: "off", want: automaticOSUpdatesOff},
		{name: "security", security: true, want: automaticOSUpdatesSecurity},
		{name: "system", system: true, want: automaticOSUpdatesSystem},
		{name: "conflict", security: true, system: true, want: automaticOSUpdatesConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			statuses := []cronJobStatus{
				{name: "Security updates", active: test.security},
				{name: "Full system updates", active: test.system},
			}
			if got := automaticOSUpdateModeForStatuses(statuses); got != test.want {
				t.Fatalf("mode = %q, want %q", got, test.want)
			}
		})
	}
}
