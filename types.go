package main

import "fmt"

// PlaybackState holds current Spotify playback info.
type PlaybackState struct {
	Track         string `json:"track"`
	Artist        string `json:"artist"`
	ProgressMS    int    `json:"progress_ms"`
	DurationMS    int    `json:"duration_ms"`
	VolumePercent *int   `json:"volume_percent,omitempty"`
	Shuffle       bool   `json:"shuffle"`
	Playing       bool   `json:"playing"`
	DeviceID      string `json:"device_id"`
	DeviceName    string `json:"device_name"`
}

// Playlist holds info for a single playlist.
type Playlist struct {
	ID         string
	Name       string
	Owner      string
	TrackCount int
	URI        string
}

// Artist holds a single artist search result.
type Artist struct {
	ID         string
	Name       string
	URI        string
	Genres     []string
	Popularity int
}

// Album holds a single album from an artist's catalogue.
type Album struct {
	ID          string
	Name        string
	URI         string
	ReleaseDate string
	TotalTracks int
	AlbumType   string
}

// Year returns the album's release year, or "" if unparseable.
func (a Album) Year() string {
	if len(a.ReleaseDate) < 4 {
		return ""
	}
	return a.ReleaseDate[:4]
}

// Track holds a single track from an album or playlist.
type Track struct {
	Name        string
	Artists     []string
	Album       string
	URI         string
	TrackNumber int
	DurationMS  int
}

// Duration renders the track length as m:ss, or "" if unknown.
func (t Track) Duration() string {
	if t.DurationMS <= 0 {
		return ""
	}
	sec := t.DurationMS / 1000
	return fmt.Sprintf("%d:%02d", sec/60, sec%60)
}

// Device holds info for a Spotify Connect device.
type Device struct {
	ID       string
	Name     string
	Type     string
	IsActive bool
}

// Name accessors let filterByName work over any of the listable types.
// Go generics cannot constrain on a struct field, so the caller supplies one.
func playlistName(p Playlist) string { return p.Name }
func artistName(a Artist) string     { return a.Name }
func albumName(a Album) string       { return a.Name }
func trackName(t Track) string       { return t.Name }
