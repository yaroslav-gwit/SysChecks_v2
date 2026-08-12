package cmd

import (
	"os"
	"regexp"
	"strings"
	"syschecks/helpers"
	"testing"
)

func TestRewriteContentCronJobs(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "cache job",
			in:   "@reboot root sleep 5 && syschecks updates --cache-create",
			want: "@reboot root sleep 5 && syschecks updates refresh",
		},
		{
			name: "security updates job",
			in:   `COMMAND="syschecks apply-updates"`,
			want: `COMMAND="syschecks updates apply --scope security"`,
		},
		{
			name: "system updates job",
			in:   `COMMAND="syschecks apply-updates --system"`,
			want: `COMMAND="syschecks updates apply --scope system"`,
		},
		{
			name: "system updates job, short flag",
			in:   `COMMAND="syschecks apply-updates -s"`,
			want: `COMMAND="syschecks updates apply --scope system"`,
		},
		{
			name: "kernel cleanup keeps its --keep argument",
			in:   `COMMAND="syschecks kernel cleanup --execute --keep 4"`,
			want: `COMMAND="syschecks kernel cleanup --yes --keep 4"`,
		},
		{
			name: "retired sysinfo",
			in:   "UserParameter=syschecks.ips,syschecks sysinfo",
			want: "UserParameter=syschecks.ips,syschecks banner --output json",
		},
		{
			name: "userinfo",
			in:   "UserParameter=syschecks.users,syschecks userinfo --json",
			want: "UserParameter=syschecks.users,syschecks users list --json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changes := rewriteContent("test", tt.in)
			if got != tt.want {
				t.Fatalf("got  %q\nwant %q", got, tt.want)
			}
			if len(changes) == 0 {
				t.Fatal("no change was reported for a line that changed")
			}
		})
	}
}

// The wildcard UserParameter passes the subcommand through as $1, so it must not be
// rewritten — there is no old command name in it.
func TestRewriteContentLeavesWildcardUserParameterAlone(t *testing.T) {
	in := "UserParameter=syschecks[*],syschecks $1"
	got, changes := rewriteContent("test", in)
	if got != in {
		t.Fatalf("wildcard UserParameter was rewritten: %q", got)
	}
	if len(changes) != 0 {
		t.Fatalf("unexpected changes: %#v", changes)
	}
}

// Running migrate twice must be a no-op; installers call it on every upgrade.
func TestRewriteContentIsIdempotent(t *testing.T) {
	original := `COMMAND="syschecks apply-updates --system"
@reboot root syschecks updates --cache-create
COMMAND="syschecks kernel cleanup --execute --keep 4"`

	once, firstChanges := rewriteContent("test", original)
	if len(firstChanges) != 3 {
		t.Fatalf("got %d changes, want 3: %#v", len(firstChanges), firstChanges)
	}

	twice, secondChanges := rewriteContent("test", once)
	if len(secondChanges) != 0 {
		t.Fatalf("second pass changed something: %#v", secondChanges)
	}
	if twice != once {
		t.Fatalf("second pass altered content:\n%q\n%q", once, twice)
	}
}

// A file that is already on the new spellings must be left completely alone.
func TestRewriteContentLeavesCurrentTemplatesAlone(t *testing.T) {
	current := `@reboot root syschecks updates refresh
COMMAND="syschecks updates apply --scope security"
COMMAND="syschecks kernel cleanup --yes --keep 4"
COMMAND="syschecks self-update"`

	got, changes := rewriteContent("test", current)
	if len(changes) != 0 || got != current {
		t.Fatalf("current templates were rewritten: %#v", changes)
	}
}

func TestRewriteContentPreservesUnrelatedLines(t *testing.T) {
	in := "# Created by syschecks\nSHELL=/bin/bash\nPATH=/sbin:/bin\nMAILTO=root"
	got, changes := rewriteContent("test", in)
	if got != in || len(changes) != 0 {
		t.Fatalf("unrelated lines changed: %q", got)
	}
}

// A fresh install must not immediately report itself as needing migration. Rather than
// mirroring the templates (which would silently drift), this reads the real source of
// helpers/templates.go and checks every command line it writes.
func TestGeneratedTemplatesNeedNoMigration(t *testing.T) {
	source, err := os.ReadFile("../helpers/templates.go")
	if err != nil {
		t.Fatal(err)
	}

	lines := regexp.MustCompile(`(?m)^.*\bsyschecks [a-z].*$`).FindAllString(string(source), -1)
	checked := 0
	for _, line := range lines {
		// Skip Go code and comments; only the cron template bodies matter.
		if strings.Contains(line, "fmt.Printf") || strings.Contains(line, "//") {
			continue
		}
		checked++
		if _, changes := rewriteContent("templates.go", line); len(changes) != 0 {
			t.Errorf("generated template line would be migrated: %q -> %+v", strings.TrimSpace(line), changes)
		}
	}

	if checked < 4 {
		t.Fatalf("only scanned %d template lines; the scan is probably broken", checked)
	}
}

func TestMigrationTargetsIncludeCronAndZabbix(t *testing.T) {
	// Only asserts the shape of the glob set; the files themselves are host-dependent.
	for _, target := range migrationTargets() {
		if !strings.HasPrefix(target, "/etc/cron.d/") && !strings.HasPrefix(target, "/etc/zabbix/") && target != "/tmp/syscheck_updates.json" {
			t.Fatalf("unexpected migration target: %q", target)
		}
	}
}

func TestMigrationModeRepairsGeneratedStatusFiles(t *testing.T) {
	for _, path := range []string{"/etc/cron.d/syschecks_cache", "/tmp/syscheck_updates.json"} {
		mode, ok := migrationMode(path)
		if !ok || mode != helpers.CRON_FILE_PERMS {
			t.Fatalf("migrationMode(%q) = %04o, %v", path, mode, ok)
		}
	}
	if _, ok := migrationMode("/etc/zabbix/zabbix_agentd.conf"); ok {
		t.Fatal("Zabbix config mode must not be normalized")
	}
}
