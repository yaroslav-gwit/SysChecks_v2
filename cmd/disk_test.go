package cmd

import (
	"syscall"
	"testing"
)

func TestParseMountInfoLine(t *testing.T) {
	line := `36 25 0:31 / /var/lib rw,relatime shared:12 - zfs rpool/var\040lib rw,xattr`
	mount, ok := parseMountInfoLine(line)
	if !ok {
		t.Fatal("parseMountInfoLine() rejected valid input")
	}
	if mount.mountPoint != "/var/lib" || mount.source != "rpool/var lib" || mount.filesystem != "zfs" || mount.readOnly {
		t.Fatalf("unexpected mount: %#v", mount)
	}
}

func TestParseMountInfoLineReadOnly(t *testing.T) {
	line := `44 25 8:1 / /snap/core ro,nodev,relatime - squashfs /dev/loop0 ro`
	mount, ok := parseMountInfoLine(line)
	if !ok || !mount.readOnly {
		t.Fatalf("read-only mount not detected: %#v, ok=%v", mount, ok)
	}
}

func TestFormatBytes(t *testing.T) {
	if got := formatBytes(12 * 1024 * 1024 * 1024); got != "12.0 GiB" {
		t.Fatalf("formatBytes() = %q", got)
	}
}

func TestDiskWarningsForMounts(t *testing.T) {
	mounts := []mountEntry{
		{source: "/dev/sda1", mountPoint: "/", filesystem: "ext4"},
		{source: "/dev/sda1", mountPoint: "/bind", filesystem: "ext4"},
		{source: "tmpfs", mountPoint: "/run", filesystem: "tmpfs"},
		{source: "/dev/loop0", mountPoint: "/snap", filesystem: "squashfs", readOnly: true},
	}
	statfs := func(_ string, stat *syscall.Statfs_t) error {
		stat.Blocks = 1000
		stat.Bavail = 50
		stat.Bsize = 4096
		return nil
	}

	warnings := diskWarningsForMounts(mounts, 10, statfs)
	if len(warnings) != 1 {
		t.Fatalf("got %d warnings, want 1: %#v", len(warnings), warnings)
	}
	if warnings[0].mountPoint != "/" || warnings[0].freePercent != 5 {
		t.Fatalf("unexpected warning: %#v", warnings[0])
	}
}

func TestDiskUsagesForMountsIncludesHealthyFilesystems(t *testing.T) {
	mounts := []mountEntry{{source: "/dev/sda1", mountPoint: "/", filesystem: "ext4"}}
	statfs := func(_ string, stat *syscall.Statfs_t) error {
		stat.Blocks = 1000
		stat.Bavail = 800
		stat.Bsize = 4096
		return nil
	}
	usages := diskUsagesForMounts(mounts, statfs)
	if len(usages) != 1 || usages[0].freePercent != 80 {
		t.Fatalf("unexpected usages: %#v", usages)
	}
}
