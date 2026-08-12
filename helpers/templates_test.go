package helpers

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestRemoveCronJob(t *testing.T) {
	path := filepath.Join(t.TempDir(), "job")
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	removed, err := removeCronJob(path)
	if err != nil || !removed {
		t.Fatalf("removeCronJob() = %v, %v; want true, nil", removed, err)
	}
	removed, err = removeCronJob(path)
	if err != nil || removed {
		t.Fatalf("second removeCronJob() = %v, %v; want false, nil", removed, err)
	}
}

func TestWriteCronFileOverridesRestrictiveUmask(t *testing.T) {
	oldUmask := syscall.Umask(0077)
	t.Cleanup(func() { syscall.Umask(oldUmask) })

	path := filepath.Join(t.TempDir(), "job")
	writeCronFile(path, "15 4 * * * root true\n")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != CRON_FILE_PERMS {
		t.Fatalf("cron mode = %04o, want %04o", got, CRON_FILE_PERMS)
	}
}
