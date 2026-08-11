//go:build live

// Tests that talk to the real Spotify API. They need a stored token and are
// excluded from a normal `go test ./...` run:
//
//	go test -tags live ./...
package main

import (
	"context"
	"testing"
)

func liveClient(t *testing.T) *SpotifyClient {
	t.Helper()
	cfg = LoadConfig()

	id, secret := cfg.Spotify.ClientID, cfg.Spotify.ClientSecret
	if id == "" || secret == "" {
		id, secret = legacyCredentials()
	}
	if id == "" || secret == "" {
		t.Skip("no Spotify credentials configured")
	}
	// Exercise the same first-run adoption the app performs, so a fresh
	// install with only the superseded app's token still runs these.
	if AdoptLegacyToken() {
		t.Logf("adopted token from %s", legacyApp)
	}
	token, err := LoadToken()
	if err != nil {
		t.Skip("no stored token:", err)
	}
	return NewSpotifyClient(token, id, secret, func(f string, a ...any) { t.Logf(f, a...) })
}

func TestLiveSearchArtists(t *testing.T) {
	c := liveClient(t)
	artists, err := c.SearchArtists(context.Background(), "miles davis")
	if err != nil {
		t.Fatal(err)
	}
	if len(artists) == 0 {
		t.Fatal("no artists returned")
	}
	t.Logf("%d artists, first: %s", len(artists), artists[0].Name)
	if artists[0].ID == "" || artists[0].URI == "" {
		t.Errorf("artist fields not populated: %+v", artists[0])
	}
}

func TestLiveArtistAlbumsAndTracks(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	artists, err := c.SearchArtists(ctx, "miles davis")
	if err != nil || len(artists) == 0 {
		t.Fatal("SearchArtists:", err)
	}

	albums, err := c.GetArtistAlbums(ctx, artists[0].ID)
	if err != nil {
		t.Fatal("GetArtistAlbums:", err)
	}
	if len(albums) == 0 {
		t.Fatal("no albums returned")
	}
	t.Logf("%d albums after dedup, oldest: %s (%s)", len(albums), albums[0].Name, albums[0].ReleaseDate)

	// The list is sorted oldest first.
	for i := 1; i < len(albums); i++ {
		if albums[i-1].ReleaseDate > albums[i].ReleaseDate {
			t.Fatalf("albums out of order at %d: %q then %q",
				i, albums[i-1].ReleaseDate, albums[i].ReleaseDate)
		}
	}

	tracks, err := c.GetAlbumTracks(ctx, albums[0].ID)
	if err != nil {
		t.Fatal("GetAlbumTracks:", err)
	}
	if len(tracks) == 0 {
		t.Fatal("no tracks returned")
	}
	for _, tr := range tracks {
		t.Logf("  %d. %s %s", tr.TrackNumber, tr.Name, tr.Duration())
	}
	if tracks[0].URI == "" || tracks[0].DurationMS == 0 {
		t.Errorf("track fields not populated: %+v", tracks[0])
	}
}

func TestLivePlaylists(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	mine, err := c.GetUserPlaylists(ctx)
	if err != nil {
		t.Fatal("GetUserPlaylists:", err)
	}
	t.Logf("%d playlists owned", len(mine))

	found, err := c.SearchPlaylists(ctx, "jazz")
	if err != nil {
		t.Fatal("SearchPlaylists:", err)
	}
	if len(found) == 0 {
		t.Fatal("no playlists found for 'jazz'")
	}
	t.Logf("%d playlists found, first: %s", len(found), found[0].Name)

	// Pagination and the offset lookup only matter if a playlist has tracks.
	if len(mine) > 0 {
		uri, err := c.GetFirstTrackURI(ctx, mine[0].ID)
		if err != nil {
			t.Fatal("GetFirstTrackURI:", err)
		}
		t.Logf("first track of %q: %s", mine[0].Name, uri)
	}
}

func TestLiveDevices(t *testing.T) {
	c := liveClient(t)
	devices, err := c.GetDevices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d devices", len(devices))
	for _, d := range devices {
		t.Logf("  %s (%s) active=%v", d.Name, d.Type, d.IsActive)
	}
}

// The search limit the config asks for is the number the API actually honours.
func TestLiveSearchLimitHonoured(t *testing.T) {
	c := liveClient(t)
	cfg.Spotify.SearchLimit = maxSearchLimit
	artists, err := c.SearchArtists(context.Background(), "jazz")
	if err != nil {
		t.Fatal(err)
	}
	if len(artists) < 11 {
		t.Errorf("asked for %d artists, got %d — the API cap may have changed",
			maxSearchLimit, len(artists))
	}
}
