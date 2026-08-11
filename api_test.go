package main

import (
	"encoding/json"
	"testing"
)

func TestNormalizeAlbumName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Kind of Blue", "kind of blue"},
		{"Walkin' (Remastered 2025)", "walkin'"},
		{"Miles: The New Miles Davis Quintet [Rudy Van Gelder Remaster]", "miles: the new miles davis quintet"},
		{"Miles Ahead (Mono Version)", "miles ahead"},
		{"Bitches Brew (Deluxe Edition)", "bitches brew"},
		{"Something (Remastered) (Deluxe Edition)", "something"},
		{"  Blue Haze  ", "blue haze"},

		// Parenthetical parts that are not edition markers must survive, or
		// genuinely different records would collapse into one.
		{"Live at the Plugged Nickel (Vol. 2)", "live at the plugged nickel (vol. 2)"},
		{"Miles '56", "miles '56"},
		{"(Untitled)", "(untitled)"},
	}
	for _, tt := range tests {
		if got := normalizeAlbumName(tt.in); got != tt.want {
			t.Errorf("normalizeAlbumName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDedupAlbumsCollapsesReissues(t *testing.T) {
	in := []Album{
		{Name: "Walkin'", ReleaseDate: "1957", TotalTracks: 5, AlbumType: "album"},
		{Name: "Walkin' (Remastered 2025)", ReleaseDate: "2025", TotalTracks: 5, AlbumType: "album"},
		{Name: "Kind of Blue", ReleaseDate: "1959", TotalTracks: 5, AlbumType: "album"},
	}
	got := dedupAlbums(in)
	if len(got) != 2 {
		t.Fatalf("expected 2 albums after dedup, got %d: %v", len(got), got)
	}
	// Equal track counts: the earlier release wins.
	if got[0].ReleaseDate != "1957" {
		t.Errorf("expected the 1957 original to win, got %+v", got[0])
	}
}

// A deluxe reissue has both a later date and more tracks. The original is the
// record; the reissue is a repackaging of it.
func TestDedupAlbumsPrefersTheOriginal(t *testing.T) {
	in := []Album{
		{Name: "Bitches Brew", ReleaseDate: "1970", TotalTracks: 6, AlbumType: "album"},
		{Name: "Bitches Brew (Deluxe Edition)", ReleaseDate: "1999", TotalTracks: 12, AlbumType: "album"},
	}
	got := dedupAlbums(in)
	if len(got) != 1 {
		t.Fatalf("expected 1 album, got %d", len(got))
	}
	if got[0].ReleaseDate != "1970" {
		t.Errorf("expected the 1970 original to win, got %+v", got[0])
	}
}

// Regression: John Mellencamp's Scarecrow lost to its 2022 deluxe, which both
// hid the 1985 record and sorted it 37 years out of place.
func TestDedupAlbumsScarecrow(t *testing.T) {
	in := []Album{
		{Name: "Scarecrow (Deluxe Edition / 2022 Mix)", ReleaseDate: "2022-11-04", TotalTracks: 24, AlbumType: "album"},
		{Name: "Scarecrow", ReleaseDate: "1985", TotalTracks: 13, AlbumType: "album"},
	}
	got := dedupAlbums(in)
	if len(got) != 1 {
		t.Fatalf("expected 1 album, got %d: %v", len(got), got)
	}
	if got[0].Name != "Scarecrow" || got[0].ReleaseDate != "1985" {
		t.Errorf("expected the 1985 original, got %+v", got[0])
	}
}

// An album with more tracks only wins when the dates are the same.
func TestDedupAlbumsTracksBreakDateTies(t *testing.T) {
	in := []Album{
		{Name: "Tutu", ReleaseDate: "1986", TotalTracks: 8, AlbumType: "album"},
		{Name: "Tutu (Expanded)", ReleaseDate: "1986", TotalTracks: 11, AlbumType: "album"},
	}
	got := dedupAlbums(in)
	if len(got) != 1 || got[0].TotalTracks != 11 {
		t.Errorf("expected the 11-track edition on a date tie, got %+v", got)
	}
}

// A missing release date must not win by comparing as the empty string.
func TestDedupAlbumsUnknownDateLoses(t *testing.T) {
	in := []Album{
		{Name: "Nighthawks", ReleaseDate: "", TotalTracks: 10, AlbumType: "album"},
		{Name: "Nighthawks (Remastered)", ReleaseDate: "1980", TotalTracks: 10, AlbumType: "album"},
	}
	got := dedupAlbums(in)
	if len(got) != 1 {
		t.Fatalf("expected 1 album, got %d", len(got))
	}
	if got[0].ReleaseDate != "1980" {
		t.Errorf("a known date should beat an unknown one, got %+v", got[0])
	}
}

// Spotify does not document a result order, so the same input in a different
// order must produce the same output.
func TestDedupAlbumsOrderIndependent(t *testing.T) {
	a := Album{Name: "Milestones", ReleaseDate: "1958", TotalTracks: 7, AlbumType: "album"}
	b := Album{Name: "Milestones (Remastered)", ReleaseDate: "2001", TotalTracks: 7, AlbumType: "album"}

	forward := dedupAlbums([]Album{a, b})
	reverse := dedupAlbums([]Album{b, a})

	if len(forward) != 1 || len(reverse) != 1 {
		t.Fatalf("expected 1 album each, got %d and %d", len(forward), len(reverse))
	}
	if forward[0].ReleaseDate != reverse[0].ReleaseDate {
		t.Errorf("order changed the winner: %q vs %q", forward[0].Name, reverse[0].Name)
	}
}

func TestDedupAlbumsSortsOldestFirst(t *testing.T) {
	in := []Album{
		{Name: "C", ReleaseDate: "2026-06-19", AlbumType: "album"},
		{Name: "A", ReleaseDate: "1959", AlbumType: "album"},
		{Name: "B", ReleaseDate: "1970-03-30", AlbumType: "album"},
	}
	got := dedupAlbums(in)
	want := []string{"A", "B", "C"}
	for i, w := range want {
		if got[i].Name != w {
			t.Errorf("position %d = %q, want %q (full order: %v)", i, got[i].Name, w, got)
		}
	}
}

// A single and an album sharing a title are different records.
func TestDedupAlbumsKeepsDistinctTypes(t *testing.T) {
	in := []Album{
		{Name: "So What", ReleaseDate: "1959", TotalTracks: 1, AlbumType: "single"},
		{Name: "So What", ReleaseDate: "1959", TotalTracks: 9, AlbumType: "album"},
	}
	if got := dedupAlbums(in); len(got) != 2 {
		t.Errorf("expected single and album to stay separate, got %d", len(got))
	}
}

func TestDedupAlbumsEmpty(t *testing.T) {
	if got := dedupAlbums(nil); len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}

// With collapsing off, every pressing survives and the order still runs
// oldest first.
func TestSortOldestFirstKeepsEveryPressing(t *testing.T) {
	in := []Album{
		{Name: "Scarecrow (Deluxe Edition / 2022 Mix)", ReleaseDate: "2022-11-04", TotalTracks: 24},
		{Name: "Scarecrow", ReleaseDate: "1985", TotalTracks: 13},
		{Name: "American Fool", ReleaseDate: "1982", TotalTracks: 10},
	}
	got := sortOldestFirst(in)
	if len(got) != 3 {
		t.Fatalf("expected all 3 pressings, got %d", len(got))
	}
	want := []string{"1982", "1985", "2022-11-04"}
	for i, w := range want {
		if got[i].ReleaseDate != w {
			t.Errorf("position %d = %q, want %q", i, got[i].ReleaseDate, w)
		}
	}
}

// An album with no release date sorts last rather than jumping to the top.
func TestSortOldestFirstUnknownDateLast(t *testing.T) {
	got := sortOldestFirst([]Album{
		{Name: "unknown", ReleaseDate: ""},
		{Name: "old", ReleaseDate: "1959"},
	})
	if got[0].Name != "old" {
		t.Errorf("expected the dated album first, got %v", got)
	}
}

func TestDefaultConfigListsEveryPressing(t *testing.T) {
	if defaultConfig().Albums.CollapseReissues {
		t.Error("collapsing reissues should be opt-in, not the default")
	}
}

// February 2026 renamed the playlist entry key from "track" to "item". Both
// spellings must decode, or a playlist reads as empty.
func TestPlaylistTrackItemAcceptsBothKeys(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{"new key", `{"item":{"name":"So What","uri":"spotify:track:1"}}`, "So What"},
		{"old key", `{"track":{"name":"So What","uri":"spotify:track:1"}}`, "So What"},
		{"both, new wins", `{"item":{"name":"new"},"track":{"name":"old"}}`, "new"},
		{"neither", `{}`, ""},
		{"removed track", `{"track":null}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var item playlistTrackItem
			if err := json.Unmarshal([]byte(tt.json), &item); err != nil {
				t.Fatal(err)
			}
			got := ""
			if e := item.entry(); e != nil {
				got = e.Name
			}
			if got != tt.want {
				t.Errorf("entry() name = %q, want %q", got, tt.want)
			}
		})
	}
}

// The same rename applies to the playlist object's own count.
func TestPlaylistItemTotalFromEitherKey(t *testing.T) {
	tests := []struct {
		name string
		json string
		want int
	}{
		{"new key", `{"id":"x","items":{"total":93}}`, 93},
		{"old key", `{"id":"x","tracks":{"total":42}}`, 42},
		{"both, new wins", `{"id":"x","items":{"total":93},"tracks":{"total":42}}`, 93},
		{"neither", `{"id":"x"}`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p playlistItem
			if err := json.Unmarshal([]byte(tt.json), &p); err != nil {
				t.Fatal(err)
			}
			if got := p.toPlaylist().TrackCount; got != tt.want {
				t.Errorf("TrackCount = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSearchLimitClamp(t *testing.T) {
	tests := []struct {
		in   int
		want int
	}{
		{0, maxSearchLimit},
		{-5, maxSearchLimit},
		{5, 5},
		{5, 5},
		{maxSearchLimit, maxSearchLimit},
		// Anything above the cap is rejected by the API outright, so clamp
		// rather than pass it through.
		{maxSearchLimit + 1, maxSearchLimit},
		{999, maxSearchLimit},
	}
	for _, tt := range tests {
		got := SpotifyConfig{SearchLimit: tt.in}.searchLimit()
		if got != tt.want {
			t.Errorf("searchLimit(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
