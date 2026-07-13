package cmd

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

const diskFreeWarningPercent = 10.0

type diskUsage struct {
	source         string
	mountPoint     string
	filesystem     string
	availableBytes uint64
	freePercent    float64
	readOnly       bool
}

type mountEntry struct {
	source     string
	mountPoint string
	filesystem string
	readOnly   bool
}

var ignoredFilesystemTypes = map[string]bool{
	"autofs": true, "bpf": true, "cgroup": true, "cgroup2": true,
	"configfs": true, "debugfs": true, "devpts": true, "devtmpfs": true,
	"efivarfs": true, "fusectl": true, "hugetlbfs": true, "mqueue": true,
	"proc": true, "pstore": true, "ramfs": true, "securityfs": true,
	"sysfs": true, "tmpfs": true, "tracefs": true,
}

func lowDiskSpacePartitions(threshold float64) []diskUsage {
	mounts, err := readMountInfo("/proc/self/mountinfo")
	if err != nil {
		return nil
	}
	return diskWarningsForMounts(mounts, threshold, syscall.Statfs)
}

func diskSpacePartitions() []diskUsage {
	mounts, err := readMountInfo("/proc/self/mountinfo")
	if err != nil {
		return nil
	}
	return diskUsagesForMounts(mounts, syscall.Statfs)
}

func diskWarningsForMounts(mounts []mountEntry, threshold float64, statfs func(string, *syscall.Statfs_t) error) []diskUsage {
	usages := diskUsagesForMounts(mounts, statfs)
	warnings := make([]diskUsage, 0)
	for _, usage := range usages {
		if usage.freePercent < threshold {
			warnings = append(warnings, usage)
		}
	}
	sort.Slice(warnings, func(i, j int) bool {
		if warnings[i].freePercent == warnings[j].freePercent {
			return warnings[i].mountPoint < warnings[j].mountPoint
		}
		return warnings[i].freePercent < warnings[j].freePercent
	})
	return warnings
}

func diskUsagesForMounts(mounts []mountEntry, statfs func(string, *syscall.Statfs_t) error) []diskUsage {
	// Bind mounts repeat the same backing source. Report its shortest mount path
	// once, while still treating distinct ZFS datasets as distinct filesystems.
	unique := make(map[string]mountEntry)
	for _, mount := range mounts {
		if mount.readOnly || ignoredFilesystemTypes[mount.filesystem] {
			continue
		}
		key := mount.filesystem + "\x00" + mount.source
		if previous, ok := unique[key]; !ok || len(mount.mountPoint) < len(previous.mountPoint) {
			unique[key] = mount
		}
	}

	usages := make([]diskUsage, 0)
	for _, mount := range unique {
		var stat syscall.Statfs_t
		if err := statfs(mount.mountPoint, &stat); err != nil || stat.Blocks == 0 {
			continue
		}

		availableBytes := stat.Bavail * uint64(stat.Bsize)
		freePercent := float64(stat.Bavail) * 100 / float64(stat.Blocks)
		usages = append(usages, diskUsage{
			source:         mount.source,
			mountPoint:     mount.mountPoint,
			filesystem:     mount.filesystem,
			availableBytes: availableBytes,
			freePercent:    freePercent,
		})
	}

	sort.Slice(usages, func(i, j int) bool {
		return usages[i].mountPoint < usages[j].mountPoint
	})
	return usages
}

func readMountInfo(path string) ([]mountEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var mounts []mountEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if mount, ok := parseMountInfoLine(scanner.Text()); ok {
			mounts = append(mounts, mount)
		}
	}
	return mounts, scanner.Err()
}

func parseMountInfoLine(line string) (mountEntry, bool) {
	fields := strings.Fields(line)
	separator := -1
	for i, field := range fields {
		if field == "-" {
			separator = i
			break
		}
	}
	if separator < 6 || len(fields) < separator+3 {
		return mountEntry{}, false
	}

	options := strings.Split(fields[5], ",")
	readOnly := false
	for _, option := range options {
		if option == "ro" {
			readOnly = true
			break
		}
	}

	return mountEntry{
		mountPoint: decodeMountInfoValue(fields[4]),
		filesystem: fields[separator+1],
		source:     decodeMountInfoValue(fields[separator+2]),
		readOnly:   readOnly,
	}, true
}

func decodeMountInfoValue(value string) string {
	replacer := strings.NewReplacer(
		`\040`, " ",
		`\011`, "\t",
		`\012`, "\n",
		`\134`, `\`,
	)
	return replacer.Replace(value)
}

func formatBytes(value uint64) string {
	const unit = uint64(1024)
	if value < unit {
		return strconv.FormatUint(value, 10) + " B"
	}

	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	amount := float64(value)
	unitIndex := -1
	for amount >= float64(unit) && unitIndex < len(units)-1 {
		amount /= float64(unit)
		unitIndex++
	}
	return fmt.Sprintf("%.1f %s", amount, units[unitIndex])
}
