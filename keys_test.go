package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func runes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func key(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

func sampleTracks() []Track {
	return []Track{
		{Name: "So What", URI: "u:1"},
		{Name: "Blue in Green", URI: "u:2"},
		{Name: "All Blues", URI: "u:3"},
	}
}

// Typing in a list filters it, the way fzf does, instead of acting as a command.
func TestListTypingFilters(t *testing.T) {
	m := model{stage: stageAlbums, albums: []Album{{Name: "Kind of Blue"}, {Name: "Bitches Brew"}}}
	for _, k := range []tea.KeyMsg{runes("j"), runes("k"), runes("q"), runes("/")} {
		next, _ := m.handleKey(k)
		m = next.(model)
	}
	if m.albumFilter.text != "jkq/" {
		t.Errorf("filter text = %q, want typed keys", m.albumFilter.text)
	}
	if m.stage != stageAlbums {
		t.Errorf("typing left the stage: %v", m.stage)
	}
	next, _ := m.handleKey(key(tea.KeyBackspace))
	if got := next.(model).albumFilter.text; got != "jkq" {
		t.Errorf("backspace left %q", got)
	}
}

// esc steps back one screen at a time and closes only from the top.
func TestEscStepsBack(t *testing.T) {
	m := model{stage: stageTracks, mode: modeAlbum}
	want := []stage{stageAlbums, stageResults, stageQuery, stagePlayer}
	for _, w := range want {
		next, _ := m.handleKey(key(tea.KeyEsc))
		m = next.(model)
		if m.stage != w {
			t.Fatalf("esc went to %v, want %v", m.stage, w)
		}
	}
}

// Track rows: 0 sequential, 1 shuffle, then the tracks.
func TestTrackRowsPickShuffleAndOffset(t *testing.T) {
	tests := []struct {
		name        string
		cursor      int
		filter      string
		wantOffset  string
		wantShuffle bool
	}{
		{"sequential row", 0, "", "", false},
		{"shuffle row", 1, "", "", true},
		{"first track", 2, "", "u:1", false},
		{"last track", 4, "", "u:3", false},
		{"filtered match", 0, "blue", "u:2", false},
		{"second filtered match", 1, "blue", "u:3", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{stage: stageTracks, mode: modeAlbum, tracks: sampleTracks(), trackCursor: tt.cursor}
			m.trackFilter.text = tt.filter
			next, _ := m.handleKey(key(tea.KeyEnter))
			got := next.(model)
			if got.stage != stageDevices {
				t.Fatalf("stage = %v, want devices", got.stage)
			}
			if got.pendingOffset != tt.wantOffset {
				t.Errorf("offset = %q, want %q", got.pendingOffset, tt.wantOffset)
			}
			if got.pendingShuffle == nil || *got.pendingShuffle != tt.wantShuffle {
				t.Errorf("shuffle = %v, want %v", got.pendingShuffle, tt.wantShuffle)
			}
		})
	}
}

func TestTrackRowCount(t *testing.T) {
	m := model{tracks: sampleTracks()}
	if got := m.trackRowCount(); got != 5 {
		t.Errorf("unfiltered rows = %d, want 5", got)
	}
	m.trackFilter.text = "blue"
	if got := m.trackRowCount(); got != 2 {
		t.Errorf("filtered rows = %d, want 2", got)
	}
}

// Playback keys follow mpv: space pauses, r reselects, S shuffles.
func TestPlayerKeysWithoutDevice(t *testing.T) {
	for _, k := range []tea.KeyMsg{runes(" "), runes("S"), runes(">"), runes("<")} {
		m := model{stage: stagePlayer}
		next, _ := m.handleKey(k)
		if got := next.(model).currentStatus(); got != "no active Spotify device" {
			t.Errorf("%q: status = %q", k.String(), got)
		}
	}
	// enter and s no longer act on the player.
	for _, k := range []tea.KeyMsg{key(tea.KeyEnter), runes("s")} {
		m := model{stage: stagePlayer}
		next, cmd := m.handleKey(k)
		if cmd != nil || next.(model).currentStatus() != "" {
			t.Errorf("%q should be unbound", k.String())
		}
	}
	t.Setenv("TMUX", "")
	t.Setenv("ZELLIJ", "")
	m := model{stage: stagePlayer}
	next, _ := m.handleKey(runes("r"))
	if next.(model).stage != stageQuery {
		t.Errorf("r should open the picker")
	}
}
