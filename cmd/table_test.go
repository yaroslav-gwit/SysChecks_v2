package cmd

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

// Every rendered line must be the same display width, otherwise the right-hand border
// staggers. Emoji are two columns but three bytes, so a byte-based width calculation
// produces exactly this defect.
func TestFprintTableAlignsRowsContainingEmoji(t *testing.T) {
	var out strings.Builder
	fprintTable(&out,
		[]string{"Job", "Enabled", "State"},
		[][]string{
			{"Update cache", "✅ yes", "enabled"},
			{"Security updates", "🛑 yes", "CONFLICT"},
			{"Kernel cleanup", "❌ no", "disabled"},
			{"Legacy job", "🟡 yes", "legacy enabled"},
		},
	)

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	// top border + header + separator + 4 rows + bottom border
	if len(lines) != 8 {
		t.Fatalf("got %d lines, want 8: %q", len(lines), out.String())
	}

	want := runewidth.StringWidth(lines[0])
	for i, line := range lines {
		if got := runewidth.StringWidth(line); got != want {
			t.Errorf("line %d is %d columns, want %d: %q", i, got, want, line)
		}
	}
}

func TestCronEnabledCell(t *testing.T) {
	tests := []struct {
		name   string
		status cronJobStatus
		want   string
	}{
		{name: "disabled", status: cronJobStatus{state: "disabled"}, want: "❌ no"},
		{name: "inactive file", status: cronJobStatus{state: "inactive file"}, want: "❌ no"},
		{name: "enabled", status: cronJobStatus{state: "enabled", active: true}, want: "✅ yes"},
		{name: "legacy", status: cronJobStatus{state: "legacy enabled", active: true, legacy: true}, want: "🟡 yes"},
		{name: "conflict", status: cronJobStatus{state: "CONFLICT", active: true}, want: "🛑 yes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cronEnabledCell(tt.status); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// The word must stand alone: emoji do not render in every log capture or console.
func TestCronEnabledCellReadsWithoutEmoji(t *testing.T) {
	for _, status := range []cronJobStatus{
		{state: "disabled"},
		{state: "enabled", active: true},
		{state: "legacy enabled", active: true, legacy: true},
		{state: "CONFLICT", active: true},
	} {
		cell := cronEnabledCell(status)
		stripped := strings.TrimSpace(strings.Map(func(r rune) rune {
			if r > 0x2000 {
				return -1
			}
			return r
		}, cell))
		if stripped != "yes" && stripped != "no" {
			t.Fatalf("cell %q reduces to %q; want a bare yes/no", cell, stripped)
		}
	}
}
