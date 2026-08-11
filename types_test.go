package main

import "testing"

func TestAlbumYear(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"1959", "1959"},
		{"1959-08-17", "1959"},
		{"1959-08", "1959"},
		{"", ""},
		{"59", ""},
		{"abc", ""},
	}
	for _, tt := range tests {
		if got := (Album{ReleaseDate: tt.in}).Year(); got != tt.want {
			t.Errorf("Album{%q}.Year() = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTrackDuration(t *testing.T) {
	tests := []struct {
		ms   int
		want string
	}{
		{0, ""},
		{-1, ""},
		{1000, "0:01"},
		{59000, "0:59"},
		{60000, "1:00"},
		{347000, "5:47"},
		{740000, "12:20"},
		{500, "0:00"}, // rounds down, still renders
	}
	for _, tt := range tests {
		if got := (Track{DurationMS: tt.ms}).Duration(); got != tt.want {
			t.Errorf("Track{%d}.Duration() = %q, want %q", tt.ms, got, tt.want)
		}
	}
}

func TestUniqueFileName(t *testing.T) {
	used := map[string]bool{}

	if got := uniqueFileName(used, "Jazz"); got != "Jazz.md" {
		t.Errorf("got %q", got)
	}
	// A second playlist of the same name must not overwrite the first.
	if got := uniqueFileName(used, "Jazz"); got != "Jazz-2.md" {
		t.Errorf("duplicate got %q, want Jazz-2.md", got)
	}
	if got := uniqueFileName(used, "Jazz"); got != "Jazz-3.md" {
		t.Errorf("third got %q, want Jazz-3.md", got)
	}

	// Path separators must never escape the output directory.
	got := uniqueFileName(used, "rock/roll")
	if got != "rock-roll.md" {
		t.Errorf("path separator got %q", got)
	}
	for _, bad := range []string{"/", `\`, ":", "*", "?", `"`, "<", ">", "|"} {
		if name := uniqueFileName(map[string]bool{}, "a"+bad+"b"); name != "a-b.md" {
			t.Errorf("%q not sanitised: %q", bad, name)
		}
	}

	// Names that would resolve to a directory entry get a safe fallback.
	for _, in := range []string{"", "  ", ".", ".."} {
		if name := uniqueFileName(map[string]bool{}, in); name != "playlist.md" {
			t.Errorf("uniqueFileName(%q) = %q, want playlist.md", in, name)
		}
	}
}
