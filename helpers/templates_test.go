package helpers

import (
	"os"
	"path/filepath"
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
