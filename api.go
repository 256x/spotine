package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// A prolific artist's catalogue runs to several hundred entries, and a user can
// own hundreds of playlists. Cap the walk rather than blocking the popup on a
// long chain of requests.
const maxPages = 20

// --- Playback state ---

type spotifyPlaybackResponse struct {
	IsPlaying  bool `json:"is_playing"`
	ProgressMS int  `json:"progress_ms"`
	Item       *struct {
		Name       string `json:"name"`
		DurationMS int    `json:"duration_ms"`
		Artists    []struct {
			Name string `json:"name"`
		} `json:"artists"`
	} `json:"item"`
	Device *struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		VolumePercent  *int   `json:"volume_percent"`
		SupportsVolume bool   `json:"supports_volume"`
	} `json:"device"`
	ShuffleState bool `json:"shuffle_state"`
}

func (c *SpotifyClient) GetCurrentPlayback(ctx context.Context) (PlaybackState, error) {
	resp, err := c.do(ctx, "GET", "/v1/me/player", nil)
	if err != nil {
		return PlaybackState{}, err
	}
	defer resp.Body.Close()

	// 204 = no active playback
	if resp.StatusCode == http.StatusNoContent {
		return PlaybackState{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return PlaybackState{}, fmt.Errorf("GET /v1/me/player: %d %s", resp.StatusCode, body)
	}

	var r spotifyPlaybackResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return PlaybackState{}, err
	}

	s := PlaybackState{
		Playing:    r.IsPlaying,
		ProgressMS: r.ProgressMS,
		Shuffle:    r.ShuffleState,
	}
	if r.Item != nil {
		s.Track = r.Item.Name
		s.DurationMS = r.Item.DurationMS
		if len(r.Item.Artists) > 0 {
			s.Artist = r.Item.Artists[0].Name
		}
	}
	if r.Device != nil {
		s.DeviceID = r.Device.ID
		s.DeviceName = r.Device.Name
		if r.Device.SupportsVolume {
			s.VolumePercent = r.Device.VolumePercent
		}
	}
	return s, nil
}

// --- Devices ---

func (c *SpotifyClient) GetDevices(ctx context.Context) ([]Device, error) {
	var r struct {
		Devices []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Type     string `json:"type"`
			IsActive bool   `json:"is_active"`
		} `json:"devices"`
	}
	if err := c.getJSON(ctx, "/v1/me/player/devices", &r); err != nil {
		return nil, err
	}
	out := make([]Device, len(r.Devices))
	for i, d := range r.Devices {
		out[i] = Device{ID: d.ID, Name: d.Name, Type: d.Type, IsActive: d.IsActive}
	}
	return out, nil
}

// --- Playlists ---

type playlistItem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	URI   string `json:"uri"`
	Owner struct {
		DisplayName string `json:"display_name"`
	} `json:"owner"`
	Tracks *struct {
		Total int `json:"total"`
	} `json:"tracks"`
}

func (p playlistItem) toPlaylist() Playlist {
	total := 0
	if p.Tracks != nil {
		total = p.Tracks.Total
	}
	return Playlist{
		ID:         p.ID,
		Name:       p.Name,
		URI:        p.URI,
		Owner:      p.Owner.DisplayName,
		TrackCount: total,
	}
}

func (c *SpotifyClient) SearchPlaylists(ctx context.Context, query string) ([]Playlist, error) {
	path := fmt.Sprintf("/v1/search?q=%s&type=playlist&limit=%d",
		url.QueryEscape(query), cfg.Spotify.searchLimit())
	var r struct {
		Playlists struct {
			Items []playlistItem `json:"items"`
		} `json:"playlists"`
	}
	if err := c.getJSON(ctx, path, &r); err != nil {
		return nil, err
	}
	var out []Playlist
	for _, item := range r.Playlists.Items {
		// Spotify pads search results with null entries.
		if item.ID == "" {
			continue
		}
		out = append(out, item.toPlaylist())
	}
	return out, nil
}

func (c *SpotifyClient) GetUserPlaylists(ctx context.Context) ([]Playlist, error) {
	var all []Playlist
	err := paginate(ctx, c, "/v1/me/playlists?limit=50", maxPages, func(items []playlistItem) {
		for _, item := range items {
			if item.ID == "" {
				continue
			}
			all = append(all, item.toPlaylist())
		}
	})
	return all, err
}

// GetFirstTrackURI returns the URI of a playlist's first track. Playback of a
// playlist context is started with this as the offset; without it Spotify can
// resume mid-playlist rather than at the top.
func (c *SpotifyClient) GetFirstTrackURI(ctx context.Context, playlistID string) (string, error) {
	path := "/v1/playlists/" + url.PathEscape(playlistID) + "/tracks?limit=1&fields=items(track(uri))"
	var r struct {
		Items []struct {
			Track struct {
				URI string `json:"uri"`
			} `json:"track"`
		} `json:"items"`
	}
	if err := c.getJSON(ctx, path, &r); err != nil {
		return "", err
	}
	if len(r.Items) > 0 {
		return r.Items[0].Track.URI, nil
	}
	return "", nil
}

type playlistTrackItem struct {
	Track *struct {
		Name  string `json:"name"`
		URI   string `json:"uri"`
		Album struct {
			Name string `json:"name"`
		} `json:"album"`
		Artists []struct {
			Name string `json:"name"`
		} `json:"artists"`
		DurationMS int `json:"duration_ms"`
	} `json:"track"`
}

// GetPlaylistTracks returns every track in a playlist, following pagination.
func (c *SpotifyClient) GetPlaylistTracks(ctx context.Context, playlistID string) ([]Track, error) {
	const fields = "next,items(track(name,uri,duration_ms,album(name),artists(name)))"
	path := "/v1/playlists/" + url.PathEscape(playlistID) +
		"/tracks?limit=100&fields=" + url.QueryEscape(fields)

	var all []Track
	err := paginate(ctx, c, path, maxPages, func(items []playlistTrackItem) {
		for _, item := range items {
			// Removed or unavailable entries come back as a null track.
			if item.Track == nil {
				continue
			}
			t := Track{
				Name:       item.Track.Name,
				Album:      item.Track.Album.Name,
				DurationMS: item.Track.DurationMS,
				URI:        item.Track.URI,
			}
			for _, a := range item.Track.Artists {
				t.Artists = append(t.Artists, a.Name)
			}
			all = append(all, t)
		}
	})
	return all, err
}

// --- Artists ---

func (c *SpotifyClient) SearchArtists(ctx context.Context, query string) ([]Artist, error) {
	path := fmt.Sprintf("/v1/search?q=%s&type=artist&limit=%d",
		url.QueryEscape(query), cfg.Spotify.searchLimit())
	var r struct {
		Artists struct {
			Items []struct {
				ID         string   `json:"id"`
				Name       string   `json:"name"`
				URI        string   `json:"uri"`
				Genres     []string `json:"genres"`
				Popularity int      `json:"popularity"`
			} `json:"items"`
		} `json:"artists"`
	}
	if err := c.getJSON(ctx, path, &r); err != nil {
		return nil, err
	}
	var out []Artist
	for _, item := range r.Artists.Items {
		if item.ID == "" {
			continue
		}
		out = append(out, Artist{
			ID:         item.ID,
			Name:       item.Name,
			URI:        item.URI,
			Genres:     item.Genres,
			Popularity: item.Popularity,
		})
	}
	return out, nil
}

// --- Albums ---

type albumItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URI         string `json:"uri"`
	ReleaseDate string `json:"release_date"`
	TotalTracks int    `json:"total_tracks"`
	AlbumType   string `json:"album_type"`
}

// GetArtistAlbums returns an artist's full albums, oldest first, with reissues
// collapsed onto the original record.
func (c *SpotifyClient) GetArtistAlbums(ctx context.Context, artistID string) ([]Album, error) {
	path := fmt.Sprintf("/v1/artists/%s/albums?include_groups=album&limit=50", url.PathEscape(artistID))

	var all []Album
	err := paginate(ctx, c, path, maxPages, func(items []albumItem) {
		for _, item := range items {
			all = append(all, Album{
				ID:          item.ID,
				Name:        item.Name,
				URI:         item.URI,
				ReleaseDate: item.ReleaseDate,
				TotalTracks: item.TotalTracks,
				AlbumType:   item.AlbumType,
			})
		}
	})
	if err != nil {
		return nil, err
	}
	return dedupAlbums(all), nil
}

var editionMarkers = []string{
	"remaster", "mono", "stereo", "deluxe", "expanded",
	"edition", "version", "anniversary", "reissue", "bonus",
}

// normalizeAlbumName strips trailing edition markers — "(Remastered 2025)",
// "[Rudy Van Gelder Remaster]" — so reissues collapse onto the original. Only
// bracketed groups naming an edition are removed, leaving real title parts
// such as "(Vol. 2)" intact.
func normalizeAlbumName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	for {
		open := strings.LastIndexAny(s, "([")
		if open <= 0 || !strings.HasSuffix(s, ")") && !strings.HasSuffix(s, "]") {
			break
		}
		inner := s[open+1 : len(s)-1]
		matched := false
		for _, m := range editionMarkers {
			if strings.Contains(inner, m) {
				matched = true
				break
			}
		}
		if !matched {
			break
		}
		s = strings.TrimSpace(s[:open])
	}
	return s
}

// dedupAlbums collapses the regional and reissue duplicates Spotify returns for
// the same record. Spotify does not guarantee a result order, so the winner is
// picked by track count (then earliest release) rather than by position.
func dedupAlbums(albums []Album) []Album {
	type key struct{ name, albumType string }
	best := make(map[key]Album, len(albums))
	order := make([]key, 0, len(albums))

	for _, a := range albums {
		k := key{normalizeAlbumName(a.Name), a.AlbumType}
		prev, seen := best[k]
		if !seen {
			best[k] = a
			order = append(order, k)
			continue
		}
		if a.TotalTracks > prev.TotalTracks ||
			(a.TotalTracks == prev.TotalTracks && a.ReleaseDate < prev.ReleaseDate) {
			best[k] = a
		}
	}

	out := make([]Album, 0, len(order))
	for _, k := range order {
		out = append(out, best[k])
	}
	// Oldest first: reissues and compilations carry recent release dates, so
	// newest-first would bury an artist's original records under them.
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ReleaseDate < out[j].ReleaseDate
	})
	return out
}

type albumTrackItem struct {
	Name        string `json:"name"`
	URI         string `json:"uri"`
	TrackNumber int    `json:"track_number"`
	DurationMS  int    `json:"duration_ms"`
}

func (c *SpotifyClient) GetAlbumTracks(ctx context.Context, albumID string) ([]Track, error) {
	path := "/v1/albums/" + url.PathEscape(albumID) + "/tracks?limit=50"

	var all []Track
	err := paginate(ctx, c, path, maxPages, func(items []albumTrackItem) {
		for _, item := range items {
			all = append(all, Track{
				Name:        item.Name,
				URI:         item.URI,
				TrackNumber: item.TrackNumber,
				DurationMS:  item.DurationMS,
			})
		}
	})
	return all, err
}

// --- Playback controls ---

func (c *SpotifyClient) control(ctx context.Context, method, path string) error {
	resp, err := c.do(ctx, method, path, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return checkStatus(resp)
}

func (c *SpotifyClient) TransferPlayback(ctx context.Context, deviceID string) error {
	body := map[string]any{"device_ids": []string{deviceID}}
	resp, err := c.do(ctx, "PUT", "/v1/me/player", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return checkStatus(resp)
}

// PlayContext starts a playlist or album as the playback context. An empty
// offsetURI begins at the first track; otherwise playback jumps to that track
// and continues through the rest of the context.
func (c *SpotifyClient) PlayContext(ctx context.Context, contextURI, deviceID, offsetURI string) error {
	body := map[string]any{"context_uri": contextURI}
	if offsetURI != "" {
		body["offset"] = map[string]string{"uri": offsetURI}
	}
	resp, err := c.do(ctx, "PUT", "/v1/me/player/play"+deviceQuery(deviceID), body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return checkStatus(resp)
}

func (c *SpotifyClient) Pause(ctx context.Context, deviceID string) error {
	return c.control(ctx, "PUT", "/v1/me/player/pause"+deviceQuery(deviceID))
}

func (c *SpotifyClient) Resume(ctx context.Context, deviceID string) error {
	return c.control(ctx, "PUT", "/v1/me/player/play"+deviceQuery(deviceID))
}

func (c *SpotifyClient) Next(ctx context.Context, deviceID string) error {
	return c.control(ctx, "POST", "/v1/me/player/next"+deviceQuery(deviceID))
}

func (c *SpotifyClient) Previous(ctx context.Context, deviceID string) error {
	return c.control(ctx, "POST", "/v1/me/player/previous"+deviceQuery(deviceID))
}

func (c *SpotifyClient) SetVolume(ctx context.Context, deviceID string, volume int) error {
	q := "?volume_percent=" + strconv.Itoa(volume)
	if deviceID != "" {
		q += "&device_id=" + url.QueryEscape(deviceID)
	}
	resp, err := c.do(ctx, "PUT", "/v1/me/player/volume"+q, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return checkStatus(resp)
}

func (c *SpotifyClient) SetShuffle(ctx context.Context, deviceID string, state bool) error {
	q := "?state=" + strconv.FormatBool(state)
	if deviceID != "" {
		q += "&device_id=" + url.QueryEscape(deviceID)
	}
	resp, err := c.do(ctx, "PUT", "/v1/me/player/shuffle"+q, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return checkStatus(resp)
}
