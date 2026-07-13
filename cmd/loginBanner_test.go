package cmd

import (
	"os"
	"strings"
	"testing"

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
