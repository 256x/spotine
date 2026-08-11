package main

import (
	"strings"
	"testing"
)

func TestProgressBarChars(t *testing.T) {
	tests := []struct {
		name           string
		progress, dur  int
		width          int
		wantLen        int
		wantFilled     int
		wantAllUnfille bool
	}{
		{name: "half", progress: 50, dur: 100, width: 10, wantLen: 10, wantFilled: 5},
		{name: "empty", progress: 0, dur: 100, width: 10, wantLen: 10, wantFilled: 0},
		{name: "full", progress: 100, dur: 100, width: 10, wantLen: 10, wantFilled: 10},
		{name: "zero width", progress: 50, dur: 100, width: 0, wantLen: 0, wantFilled: 0},
		{name: "negative width", progress: 50, dur: 100, width: -3, wantLen: 0, wantFilled: 0},

		// A duration of zero must not divide by zero.
		{name: "unknown duration", progress: 10, dur: 0, width: 8, wantLen: 8, wantFilled: 0, wantAllUnfille: true},

		// Spotify occasionally reports progress past the track length.
		{name: "overrun clamps", progress: 500, dur: 100, width: 10, wantLen: 10, wantFilled: 10},
		{name: "negative progress clamps", progress: -20, dur: 100, width: 10, wantLen: 10, wantFilled: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bar, filled := progressBarChars(tt.progress, tt.dur, tt.width)
			if len(bar) != tt.wantLen {
				t.Errorf("bar length = %d, want %d (%q)", len(bar), tt.wantLen, bar)
			}
			if filled != tt.wantFilled {
				t.Errorf("filled = %d, want %d", filled, tt.wantFilled)
			}
			if filled > len(bar) {
				t.Fatalf("filled %d exceeds bar length %d — slicing bar[:filled] would panic", filled, len(bar))
			}
			if tt.wantAllUnfille && strings.Contains(bar, "=") {
				t.Errorf("unknown duration should render no progress, got %q", bar)
			}
		})
	}
}

// buildPlayerLine slices the bar at the filled offset, so any width that makes
// those disagree is a panic in the running app.
func TestProgressBarCharsSliceSafe(t *testing.T) {
	for width := -2; width < 40; width++ {
		for _, p := range []int{-1, 0, 1, 999, 1_000_000} {
			for _, d := range []int{0, 1, 1000, 300_000} {
				bar, filled := progressBarChars(p, d, width)
				if filled < 0 || filled > len(bar) {
					t.Fatalf("width=%d progress=%d duration=%d: filled=%d, len=%d",
						width, p, d, filled, len(bar))
				}
				_ = bar[:filled]
			}
		}
	}
}

// The popup has to survive a one-line tmux pane and a very narrow terminal.
func TestRenderPopupBoxExtremeSizes(t *testing.T) {
	initStyles(ThemeConfig{Name: "mono"}.resolve())

	items := []string{"first", "second", "a very long entry that will not fit anywhere"}
	labels := []string{"1", "22", "333"}

	sizes := []struct{ w, h int }{
		{1, 1}, {2, 1}, {10, 1}, {20, 3}, {80, 24}, {200, 60}, {0, 0},
	}
	for _, s := range sizes {
		for _, cursor := range []int{-1, 0, 2} {
			out := renderPopupBox("title", items, labels, cursor, len(items), "filter", true, s.w, s.h)
			if out == "" {
				t.Errorf("empty render at %dx%d", s.w, s.h)
			}
		}
	}
}

func TestRenderPopupBoxNoItems(t *testing.T) {
	initStyles(ThemeConfig{Name: "mono"}.resolve())
	out := renderPopupBox("empty", nil, nil, -1, -1, "", false, 40, 10)
	if !strings.Contains(out, "empty") {
		t.Errorf("title missing from empty popup: %q", out)
	}
}

// Cursors past the end of the list must not index out of range.
func TestRenderPopupBoxCursorBeyondItems(t *testing.T) {
	initStyles(ThemeConfig{Name: "mono"}.resolve())
	items := []string{"only"}
	out := renderPopupBox("t", items, nil, 5, len(items), "", false, 40, 10)
	if out == "" {
		t.Error("expected a rendered box")
	}
}

func TestThemeResolve(t *testing.T) {
	// A named built-in comes through intact.
	got := ThemeConfig{Name: "dracula"}.resolve()
	if got.Accent != "#bd93f9" {
		t.Errorf("dracula accent = %q", got.Accent)
	}
	if got.UseTerminalColor {
		t.Error("dracula should not use the terminal colour")
	}

	// "terminal" defers its accent to the running terminal.
	if !(ThemeConfig{Name: "terminal"}.resolve()).UseTerminalColor {
		t.Error("terminal theme should set UseTerminalColor")
	}

	// Explicit fields override the named theme.
	got = ThemeConfig{Name: "dracula", Accent: "#ffffff"}.resolve()
	if got.Accent != "#ffffff" {
		t.Errorf("explicit accent should win, got %q", got.Accent)
	}
	if got.SelectedFg != "#f8f8f2" {
		t.Errorf("unset fields should stay from the named theme, got %q", got.SelectedFg)
	}

	// An unknown name falls back rather than producing empty colours.
	got = ThemeConfig{Name: "no-such-theme"}.resolve()
	if got.Accent == "" || got.SelectedFg == "" || got.FilterFg == "" {
		t.Errorf("unknown theme left empty colours: %+v", got)
	}
}
