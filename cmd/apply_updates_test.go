package cmd

import (
	"testing"
	"time"
)

func TestApplyUpdatesDelayStaysWithinBounds(t *testing.T) {
	const maxDelay = 30 * time.Minute

	sawNonZero := false
	for i := 0; i < 500; i++ {
		delay := applyUpdatesDelay(maxDelay, false, false)
		if delay < 0 || delay > maxDelay {
			t.Fatalf("delay %s outside [0, %s]", delay, maxDelay)
		}
		if delay > 0 {
			sawNonZero = true
		}
	}
	if !sawNonZero {
		t.Fatal("delay was always zero; the run would not be spread at all")
	}
}

func TestApplyUpdatesDelaySuppressed(t *testing.T) {
	tests := []struct {
		name        string
		maxDelay    time.Duration
		disabled    bool
		interactive bool
	}{
		{name: "--no-delay", maxDelay: 30 * time.Minute, disabled: true},
		{name: "--delay 0", maxDelay: 0},
		{name: "negative delay", maxDelay: -5 * time.Minute},
		{name: "interactive run", maxDelay: 30 * time.Minute, interactive: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Repeat: a randomised result that is only usually zero would still block a
			// human waiting at a terminal.
			for i := 0; i < 100; i++ {
				if delay := applyUpdatesDelay(tt.maxDelay, tt.disabled, tt.interactive); delay != 0 {
					t.Fatalf("got delay %s, want 0", delay)
				}
			}
		})
	}
}
