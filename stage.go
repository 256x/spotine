package main

import "strings"

// browseMode selects what the popup searches for. The player line and every
// playback key behave identically in both.
type browseMode int

const (
	modePlaylist browseMode = iota
	modeAlbum
)

func (m browseMode) String() string {
	if m == modeAlbum {
		return "album"
	}
	return "playlist"
}

func parseMode(s string) (browseMode, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "album", "artist", "a":
		return modeAlbum, true
	case "playlist", "p":
		return modePlaylist, true
	}
	return modePlaylist, false
}

// toggled returns the other mode.
func (m browseMode) toggled() browseMode {
	if m == modeAlbum {
		return modePlaylist
	}
	return modeAlbum
}

// stage is the single source of truth for which screen is showing. It replaces
// the pile of independent booleans the earlier versions carried, where invalid
// combinations were representable and the back-navigation target had to be
// restated at every call site.
type stage int

const (
	stagePlayer  stage = iota // the one-line player, no popup
	stageQuery                // typing a search query
	stageResults              // playlists or artists, depending on mode
	stageAlbums               // album mode only
	stageTracks               // album mode only
	stageDevices
	stageHelp
)

// previousStage is where backspace goes. Keeping it in one place is the whole
// point: the device picker's back target depends on the mode, and spreading
// that decision across handlers is what produced the same bug twice.
func (m model) previousStage() stage {
	switch m.stage {
	case stageDevices:
		if m.mode == modeAlbum {
			return stageTracks
		}
		return stageResults
	case stageTracks:
		return stageAlbums
	case stageAlbums:
		return stageResults
	case stageResults:
		return stageQuery
	default:
		return stagePlayer
	}
}

// listFilter is the local, no-network filtering applied inside a list stage.
type listFilter struct {
	active bool
	text   string
}

func (f *listFilter) clear() {
	f.active = false
	f.text = ""
}

// filterByName keeps entries whose name contains query, case-insensitively.
func filterByName[T any](all []T, query string, name func(T) string) []T {
	if query == "" {
		return all
	}
	q := strings.ToLower(query)
	var out []T
	for _, v := range all {
		if strings.Contains(strings.ToLower(name(v)), q) {
			out = append(out, v)
		}
	}
	return out
}
