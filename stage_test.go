package main

import "testing"

// The device picker's back target depends on the browse mode, and getting it
// wrong is silent: the popup simply skips a screen. That bug appeared twice in
// the predecessor apps, so it is pinned down here.
func TestPreviousStage(t *testing.T) {
	tests := []struct {
		name string
		from stage
		mode browseMode
		want stage
	}{
		{"devices back to tracks in album mode", stageDevices, modeAlbum, stageTracks},
		{"devices back to results in playlist mode", stageDevices, modePlaylist, stageResults},
		{"tracks back to albums", stageTracks, modeAlbum, stageAlbums},
		{"albums back to results", stageAlbums, modeAlbum, stageResults},
		{"results back to query in album mode", stageResults, modeAlbum, stageQuery},
		{"results back to query in playlist mode", stageResults, modePlaylist, stageQuery},
		{"query back to player", stageQuery, modePlaylist, stagePlayer},
		{"player stays put", stagePlayer, modePlaylist, stagePlayer},
		{"help returns to player", stageHelp, modePlaylist, stagePlayer},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{stage: tt.from, mode: tt.mode}
			if got := m.previousStage(); got != tt.want {
				t.Errorf("previousStage() from %v in %v mode = %v, want %v",
					tt.from, tt.mode, got, tt.want)
			}
		})
	}
}

// Walking back from the deepest stage must reach the player without looping or
// skipping a screen.
func TestPreviousStageTerminates(t *testing.T) {
	for _, mode := range []browseMode{modePlaylist, modeAlbum} {
		m := model{stage: stageDevices, mode: mode}
		seen := map[stage]bool{}
		for i := 0; ; i++ {
			if i > len(
				[]stage{stagePlayer, stageQuery, stageResults, stageAlbums, stageTracks, stageDevices, stageHelp}) {
				t.Fatalf("%v mode: back navigation did not terminate", mode)
			}
			if m.stage == stagePlayer {
				break
			}
			if seen[m.stage] {
				t.Fatalf("%v mode: revisited %v", mode, m.stage)
			}
			seen[m.stage] = true
			m.stage = m.previousStage()
		}
	}

	// Playlist mode must never route through the album-only screens.
	m := model{stage: stageDevices, mode: modePlaylist}
	for m.stage != stagePlayer {
		if m.stage == stageAlbums || m.stage == stageTracks {
			t.Fatalf("playlist mode reached album-only stage %v", m.stage)
		}
		m.stage = m.previousStage()
	}
}

func TestParseMode(t *testing.T) {
	tests := []struct {
		in     string
		want   browseMode
		wantOK bool
	}{
		{"album", modeAlbum, true},
		{"artist", modeAlbum, true},
		{"a", modeAlbum, true},
		{"ALBUM", modeAlbum, true},
		{"  album  ", modeAlbum, true},
		{"playlist", modePlaylist, true},
		{"p", modePlaylist, true},
		{"", modePlaylist, false},
		{"nonsense", modePlaylist, false},
	}
	for _, tt := range tests {
		got, ok := parseMode(tt.in)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("parseMode(%q) = (%v, %v), want (%v, %v)", tt.in, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestModeToggled(t *testing.T) {
	if modePlaylist.toggled() != modeAlbum {
		t.Error("playlist should toggle to album")
	}
	if modeAlbum.toggled() != modePlaylist {
		t.Error("album should toggle to playlist")
	}
	if modePlaylist.toggled().toggled() != modePlaylist {
		t.Error("toggling twice should return to the original mode")
	}
}

func TestFilterByName(t *testing.T) {
	albums := []Album{
		{Name: "Kind of Blue"},
		{Name: "Bitches Brew"},
		{Name: "BLUE HAZE"},
	}

	if got := filterByName(albums, "", albumName); len(got) != 3 {
		t.Errorf("empty query should return everything, got %d", len(got))
	}
	got := filterByName(albums, "blue", albumName)
	if len(got) != 2 {
		t.Fatalf("expected 2 matches for %q, got %d", "blue", len(got))
	}
	if got[0].Name != "Kind of Blue" || got[1].Name != "BLUE HAZE" {
		t.Errorf("case-insensitive match returned %v", got)
	}
	if got := filterByName(albums, "nothing", albumName); got != nil {
		t.Errorf("no match should return nil, got %v", got)
	}
}

func TestListFilterClear(t *testing.T) {
	f := listFilter{text: "blue"}
	f.clear()
	if f.text != "" {
		t.Errorf("clear() left %+v", f)
	}
}
