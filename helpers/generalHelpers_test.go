package helpers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrettyOsNameFromValues(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		want   string
	}{
		{
			name: "pretty name",
			values: map[string]string{
				"PRETTY_NAME": "Ubuntu 26.04 LTS",
				"NAME":        "Ubuntu",
				"VERSION":     "26.04 LTS",
			},
			want: "Ubuntu 26.04 LTS",
		},
		{
			name: "name and version",
			values: map[string]string{
				"NAME":    "Debian GNU/Linux",
				"VERSION": "12 (bookworm)",
			},
			want: "Debian GNU/Linux 12 (bookworm)",
		},
		{
			name: "id and version id",
			values: map[string]string{
				"ID":         "almalinux",
				"VERSION_ID": "8.10",
			},
			want: "AlmaLinux 8.10",
		},
		{
			name: "lsb description",
			values: map[string]string{
				"DISTRIB_ID":          "LinuxMint",
				"DISTRIB_RELEASE":     "22.1",
				"DISTRIB_DESCRIPTION": "Linux Mint 22.1 Xia",
			},
			want: "Linux Mint 22.1 Xia",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "os-release")
			content := ""
			for key, value := range tt.values {
				content += key + `="` + value + `"` + "\n"
			}
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatalf("failed to write os-release fixture: %v", err)
			}

			if got := prettyOsNameFromFile(path); got != tt.want {
				t.Fatalf("prettyOsNameFromFile() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrettyOsNameFromMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	etcPath := filepath.Join(dir, "os-release")
	usrPath := filepath.Join(dir, "usr-lib-os-release")

	if err := os.WriteFile(etcPath, []byte("ID=ubuntu\nVERSION_ID=26.04\n"), 0644); err != nil {
		t.Fatalf("failed to write os-release fixture: %v", err)
	}
	if err := os.WriteFile(usrPath, []byte("PRETTY_NAME=\"Upstream Linux\"\n"), 0644); err != nil {
		t.Fatalf("failed to write fallback fixture: %v", err)
	}

	if got := prettyOsNameFromFiles([]string{etcPath, usrPath}); got != "Upstream Linux" {
		t.Fatalf("prettyOsNameFromFiles() = %q, want %q", got, "Upstream Linux")
	}
}
