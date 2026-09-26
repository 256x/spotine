package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// --- message types ---

type tickMsg time.Time
type vizTickMsg struct{}
type playbackUpdatedMsg struct{ State PlaybackState }

// popupClosedMsg reports back from the multiplexer popup subprocess: which
// mode it was left in, and the playback state as of its exit.
type popupClosedMsg struct {
	Mode     browseMode
	State    PlaybackState
	HasState bool
}
type apiErrorMsg struct{ Err error }
type statusMsg struct{ Text string }

type playlistsLoadedMsg struct {
	Playlists []Playlist
	Err       error
}

type artistsLoadedMsg struct {
	Artists []Artist
	Err     error
}

type albumsLoadedMsg struct {
	Albums []Album
	Err    error
}

type tracksLoadedMsg struct {
	Tracks []Track
	Err    error
}

type devicesLoadedMsg struct {
	Devices []Device
	Err     error
}

// --- timers ---

func tick() tea.Cmd {
	d := time.Duration(cfg.UI.TickInterval) * time.Second
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func vizTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return vizTickMsg{}
	})
}

// --- fetches ---

func fetchPlayback(client *SpotifyClient) tea.Cmd {
	return func() tea.Msg {
		state, err := client.GetCurrentPlayback(context.Background())
		if err != nil {
			return apiErrorMsg{Err: err}
		}
		return playbackUpdatedMsg{State: state}
	}
}

func fetchPlaylists(client *SpotifyClient) tea.Cmd {
	return func() tea.Msg {
		playlists, err := client.GetUserPlaylists(context.Background())
		return playlistsLoadedMsg{Playlists: playlists, Err: err}
	}
}

func searchPlaylists(client *SpotifyClient, query string) tea.Cmd {
	return func() tea.Msg {
		playlists, err := client.SearchPlaylists(context.Background(), query)
		return playlistsLoadedMsg{Playlists: playlists, Err: err}
	}
}

func searchArtists(client *SpotifyClient, query string) tea.Cmd {
	return func() tea.Msg {
		artists, err := client.SearchArtists(context.Background(), query)
		return artistsLoadedMsg{Artists: artists, Err: err}
	}
}

func fetchAlbums(client *SpotifyClient, artistID string) tea.Cmd {
	return func() tea.Msg {
		albums, err := client.GetArtistAlbums(context.Background(), artistID)
		return albumsLoadedMsg{Albums: albums, Err: err}
	}
}

func fetchTracks(client *SpotifyClient, albumID string) tea.Cmd {
	return func() tea.Msg {
		tracks, err := client.GetAlbumTracks(context.Background(), albumID)
		return tracksLoadedMsg{Tracks: tracks, Err: err}
	}
}

func fetchDevices(client *SpotifyClient) tea.Cmd {
	return func() tea.Msg {
		devices, err := client.GetDevices(context.Background())
		return devicesLoadedMsg{Devices: devices, Err: err}
	}
}

// --- playback ---

func skipCmd(client *SpotifyClient, deviceID string, fn func(context.Context, string) error) tea.Cmd {
	return func() tea.Msg {
		if err := fn(context.Background(), deviceID); err != nil {
			return apiErrorMsg{Err: err}
		}
		time.Sleep(300 * time.Millisecond)
		s, _ := client.GetCurrentPlayback(context.Background())
		return playbackUpdatedMsg{State: s}
	}
}

// startPlayback moves playback to the chosen device and starts the context.
// A playlist with no explicit offset is pinned to its first track, otherwise
// Spotify may resume it wherever it was last left.
// A non-nil shuffle is applied before the context starts; nil leaves it alone.
func startPlayback(ctx context.Context, client *SpotifyClient, uri, deviceID, offsetURI string, shuffle *bool) error {
	if deviceID != "" {
		_ = client.TransferPlayback(ctx, deviceID)
		time.Sleep(500 * time.Millisecond)
	}
	if shuffle != nil {
		_ = client.SetShuffle(ctx, deviceID, *shuffle)
	}
	if offsetURI == "" {
		if parts := strings.Split(uri, ":"); len(parts) == 3 && parts[1] == "playlist" {
			offsetURI, _ = client.GetFirstTrackURI(ctx, parts[2])
		}
	}
	return client.PlayContext(ctx, uri, deviceID, offsetURI)
}

func playCmd(client *SpotifyClient, uri, deviceID, offsetURI string, shuffle *bool) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := startPlayback(ctx, client, uri, deviceID, offsetURI, shuffle); err != nil {
			return apiErrorMsg{Err: err}
		}
		time.Sleep(300 * time.Millisecond)
		state, err := client.GetCurrentPlayback(ctx)
		if err != nil {
			return statusMsg{Text: "playback started"}
		}
		return playbackUpdatedMsg{State: state}
	}
}

// --- multiplexer popups ---

func openExternalHelp() tea.Cmd {
	return func() tea.Msg {
		exe := exePath()
		var cmd *exec.Cmd
		switch {
		case os.Getenv("TMUX") != "":
			cmd = exec.Command("tmux", "display-popup", "-E", "-w", "50", "-h", "18", exe, "--keys")
		case os.Getenv("ZELLIJ") != "":
			cmd = exec.Command("zellij", "run", "--floating", "--close-on-exit", "--", exe, "--keys")
		}
		if cmd != nil {
			_ = cmd.Run()
		}
		return nil
	}
}

// openExternalSelect runs the picker in the multiplexer's floating window. The
// popup is a separate process, so the current mode is handed to it as a flag
// and read back from the cache afterwards in case the user toggled it there.
func openExternalSelect(client *SpotifyClient, mode browseMode) tea.Cmd {
	return func() tea.Msg {
		exe := exePath()
		modeFlag := "-p"
		if mode == modeAlbum {
			modeFlag = "-a"
		}
		var cmd *exec.Cmd
		switch {
		case os.Getenv("TMUX") != "":
			cmd = exec.Command("tmux", "display-popup", "-E", "-w", "66", "-h", "22", exe, "--select", modeFlag)
		case os.Getenv("ZELLIJ") != "":
			cmd = exec.Command("zellij", "run", "--floating", "--close-on-exit",
				"--width", "66", "--height", "22", "--", exe, "--select", modeFlag)
		}
		if cmd != nil {
			_ = cmd.Run()
		}

		closed := popupClosedMsg{Mode: mode}
		if s, err := LoadCache(); err == nil {
			if restored, ok := parseMode(s.Mode); ok {
				closed.Mode = restored
			}
		}
		if state, err := client.GetCurrentPlayback(context.Background()); err == nil {
			closed.State = state
			closed.HasState = true
		}
		return closed
	}
}
