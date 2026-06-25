package cmd

import "testing"

func TestReadOsRelease(t *testing.T) {
	values, err := readOsRelease("testdata/os-release-debian")
	if err != nil {
		t.Fatalf("readOsRelease returned error: %v", err)
	}

	if values["ID"] != "debian" {
		t.Fatalf("expected ID debian, got %q", values["ID"])
	}
	if values["PRETTY_NAME"] != "Debian GNU/Linux 12 (bookworm)" {
		t.Fatalf("unexpected PRETTY_NAME: %q", values["PRETTY_NAME"])
	}
}

func TestSelectPackageManagerFromIDLike(t *testing.T) {
	originalCommandExists := commandExistsFunc
	t.Cleanup(func() {
		commandExistsFunc = originalCommandExists
	})
	commandExistsFunc = func(name string) bool {
		return name == "apt-get" || name == "dnf"
	}

	tests := []struct {
		name string
		os   detectOsStruct
		want packageManagerKind
	}{
		{
			name: "ubuntu derivative",
			os: detectOsStruct{
				osID:     "customubuntu",
				osIDLike: []string{"ubuntu", "debian"},
			},
			want: packageManagerAPT,
		},
		{
			name: "rhel derivative",
			os: detectOsStruct{
				osID:     "customrhel",
				osIDLike: []string{"rhel", "fedora"},
			},
			want: packageManagerDNF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectPackageManager(tt.os); got != tt.want {
				t.Fatalf("selectPackageManager() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsRPMKernelPackageForVersion(t *testing.T) {
	tests := []struct {
		name    string
		pkg     string
		version string
		want    bool
	}{
		{
			name:    "kernel core package",
			pkg:     "kernel-core-5.14.0-427.20.1.el9_4.x86_64",
			version: "5.14.0-427.20.1.el9_4.x86_64",
			want:    true,
		},
		{
			name:    "kernel modules package",
			pkg:     "kernel-modules-extra-6.8.9-100.fc38.x86_64",
			version: "6.8.9-100.fc38.x86_64",
			want:    true,
		},
		{
			name:    "non kernel package containing version",
			pkg:     "dracut-5.14.0-427.20.1.el9_4.x86_64",
			version: "5.14.0-427.20.1.el9_4.x86_64",
			want:    false,
		},
		{
			name:    "different kernel version",
			pkg:     "kernel-core-5.14.0-427.20.1.el9_4.x86_64",
			version: "5.14.0-427.18.1.el9_4.x86_64",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRPMKernelPackageForVersion(tt.pkg, tt.version); got != tt.want {
				t.Fatalf("isRPMKernelPackageForVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseDnfRepoqueryOutput(t *testing.T) {
	input := "openssl-libs\t1\t3.2.4\t1.fc40\tx86_64\tupdates\n" +
		"dnf\t0\t4.23.0\t1.fc40.1\tnoarch\tupdates\n"

	got := parseDnfRepoqueryOutput(input)
	want := []string{
		"openssl-libs.x86_64 1:3.2.4-1.fc40 updates",
		"dnf.noarch 4.23.0-1.fc40.1 updates",
	}

	if len(got) != len(want) {
		t.Fatalf("got %d updates, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseUpdateinfoSecurityOutput(t *testing.T) {
	input := "FEDORA-2025-becf280371 Important/Sec. openssl-libs-1:3.2.4-1.fc40.x86_64\n" +
		"ALSA-2026:22721 Important/Sec. expat-2.5.0-2.el8_10.x86_64\n" +
		"ALSA-2026:22730 Moderate/Sec. vim-minimal-2:8.0.1763-23.el8_10.x86_64\n" +
		"ALSA-2026:28553 Moderate/Sec. vim-minimal-2:8.0.1763-24.el8_10.x86_64\n"

	got := parseUpdateinfoSecurityOutput(input)
	want := []string{
		"openssl-libs-1:3.2.4-1.fc40.x86_64",
		"expat-2.5.0-2.el8_10.x86_64",
		"vim-minimal-2:8.0.1763-24.el8_10.x86_64",
	}

	if len(got) != len(want) {
		t.Fatalf("got %d updates, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFormatAptInstLine(t *testing.T) {
	line := "Inst libssl3 [3.0.2-0ubuntu1.23] (3.0.2-0ubuntu1.25 Ubuntu:22.04/jammy-updates, Ubuntu:22.04/jammy-security [amd64])"
	want := "libssl3 [3.0.2-0ubuntu1.23] (3.0.2-0ubuntu1.25 Ubuntu:22.04/jammy-updates, Ubuntu:22.04/jammy-security [amd64])"

	if got := formatAptInstLine(line); got != want {
		t.Fatalf("formatAptInstLine() = %q, want %q", got, want)
	}
}
