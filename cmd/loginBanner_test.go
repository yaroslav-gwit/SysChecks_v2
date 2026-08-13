package cmd

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Delta456/box-cli-maker/v2"
	"github.com/gookit/color"
	"github.com/mattn/go-runewidth"
)

func TestAddBannerHeaderPlacesStatusAtRight(t *testing.T) {
	rendered := "╭──────────────────────────────────────────────────────────╮\n│ body                                                     │\n╰──────────────────────────────────────────────────────────╯\n"
	got := addBannerHeader(rendered, "Welcome back, root!", "v1.2.0 | self-update: ON")
	lines := strings.Split(got, "\n")
	if !strings.Contains(color.ClearCode(lines[0]), "Welcome back, root!") || !strings.Contains(color.ClearCode(lines[0]), "v1.2.0 | self-update: ON ╮") {
		t.Fatalf("unexpected header: %q", color.ClearCode(lines[0]))
	}
	if runewidth.StringWidth(color.ClearCode(lines[0])) != runewidth.StringWidth(lines[1]) {
		t.Fatalf("header width differs from body: %d != %d", runewidth.StringWidth(color.ClearCode(lines[0])), runewidth.StringWidth(lines[1]))
	}
}

// Every emoji on the banner must be a single rune that go-runewidth measures as two
// columns. Sequences that end in a variation selector (U+FE0F) measure as one but render as
// two, so box-cli-maker under-pads that line and its right border lands a column too far
// right. "⚠️" caused exactly that on the kernel-cleanup line.
func TestBannerEmojisHaveConsistentWidth(t *testing.T) {
	source, err := os.ReadFile("loginBanner.go")
	if err != nil {
		t.Fatal(err)
	}

	matches := regexp.MustCompile(`emoji\("([^"]+)"`).FindAllStringSubmatch(string(source), -1)
	if len(matches) < 10 {
		t.Fatalf("found only %d emoji call sites; the scan is probably broken", len(matches))
	}

	for _, match := range matches {
		e := match[1]
		if runes := []rune(e); len(runes) != 1 {
			t.Errorf("emoji %q is %d runes; multi-rune sequences are mis-measured and shift the banner border", e, len(runes))
			continue
		}
		if width := runewidth.StringWidth(e); width != 2 {
			t.Errorf("emoji %q measures %d columns, want 2; it will shift the banner border", e, width)
		}
	}
}

// Banner lines must fit an 80-column terminal, which is the narrowest an operator realistically
// gets over SSH. Longer lines wrap mid-word inside the box.
func TestRepositoryIssueLinesFitNarrowTerminal(t *testing.T) {
	var out strings.Builder
	writeRepositoryIssues(&out, []repoIssue{
		{Repo: "packages.microsoft.com bookworm", Reason: "missing GPG key EB3E94ADBE1229CF"},
		{Repo: "dead-path", Reason: "not found (HTTP 404) — repository may have been removed"},
	}, "incomplete", false, false)

	for _, line := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
		if width := runewidth.StringWidth(color.ClearCode(line)); width > 76 {
			t.Errorf("line is %d columns, too wide for an 80-column banner: %q", width, color.ClearCode(line))
		}
	}
}

// The kernel-cleanup advice used to be appended to the summary line, producing an ~88
// column line that overflowed an 80-column banner and wrapped mid-word.
func TestKernelCleanupLinesFitNarrowTerminal(t *testing.T) {
	var out strings.Builder
	writeKernelStatus(&out, compareKernelsStruct{
		runningKernel:         "6.17.9-200.fc44.x86_64",
		latestInstalledKernel: "6.18.1-200.fc44.x86_64",
		installedKernelCount:  12,
		kernelNeedsReboot:     true,
	}, false, true)

	if !strings.Contains(out.String(), "cleanup recommended") {
		t.Fatalf("cleanup advice missing: %q", out.String())
	}
	for _, line := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
		if width := runewidth.StringWidth(color.ClearCode(line)); width > 76 {
			t.Errorf("line is %d columns, too wide for an 80-column banner: %q", width, color.ClearCode(line))
		}
	}
}

// A box whose lines all measure the same width is the condition box-cli-maker relies on to
// place the right border. The "⚠️" regression broke exactly this.
func TestKernelStatusRendersWithAlignedBorders(t *testing.T) {
	var content strings.Builder
	writeKernelStatus(&content, compareKernelsStruct{
		runningKernel:        "6.17.9-200.fc44.x86_64",
		installedKernelCount: 12,
	}, false, false)

	boxNew := box.Box{
		TopLeft: "╭", TopRight: "╮", BottomLeft: "╰", BottomRight: "╯",
		Horizontal: "─", Vertical: "│",
		Config: box.Config{Px: 1, Py: 1, Type: "", AllowWrapping: true, WrappingLimit: 100},
	}
	rendered := boxNew.String("", strings.TrimRight(content.String(), "\n"))

	var width int
	for i, line := range strings.Split(strings.TrimRight(rendered, "\n"), "\n") {
		got := runewidth.StringWidth(color.ClearCode(line))
		if i == 0 {
			width = got
			continue
		}
		if got != width {
			t.Fatalf("line %d is %d columns, want %d (border will be misaligned): %q", i, got, width, color.ClearCode(line))
		}
	}
}

func TestUnsupportedSecurityOnlyWarnsOnlyWhenScheduled(t *testing.T) {
	for _, mode := range []automaticOSUpdateMode{
		automaticOSUpdatesOff,
		automaticOSUpdatesSystem,
		automaticOSUpdatesConflict,
	} {
		if shouldWarnUnsupportedSecurity(false, mode) {
			t.Errorf("unsupported security warning shown for update mode %q", mode)
		}
	}
	if !shouldWarnUnsupportedSecurity(false, automaticOSUpdatesSecurity) {
		t.Fatal("security-only schedule did not surface unsupported state")
	}
	if shouldWarnUnsupportedSecurity(true, automaticOSUpdatesSecurity) {
		t.Fatal("supported package manager surfaced unsupported warning")
	}

	defaultCheck := securityUpdateBannerCheck(0, false, automaticOSUpdatesOff, "unsupported")
	if !defaultCheck.Healthy || !strings.Contains(defaultCheck.Detail, "not applicable") {
		t.Fatalf("default unsupported JSON check = %#v", defaultCheck)
	}
	securityOnlyCheck := securityUpdateBannerCheck(0, false, automaticOSUpdatesSecurity, "unsupported")
	if securityOnlyCheck.Healthy || !strings.Contains(securityOnlyCheck.Detail, "unsupported") {
		t.Fatalf("scheduled unsupported JSON check = %#v", securityOnlyCheck)
	}
}

func TestSelfUpdateEnabled(t *testing.T) {
	path := t.TempDir() + "/syschecks_autoupdate"
	data := "# Created by syschecks\n30 3 * * * root syschecks self-update\n"
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	if !selfUpdateEnabled(path) {
		t.Fatal("expected enabled self-update job")
	}
}

func TestSelfUpdateDisabledWhenScheduleIsCommented(t *testing.T) {
	path := t.TempDir() + "/syschecks_autoupdate"
	data := "COMMAND=\"syschecks self-update\"\n# 30 3 * * * root ${COMMAND}\n"
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	if selfUpdateEnabled(path) {
		t.Fatal("commented schedule must not be reported as enabled")
	}
}
