package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const version = "v0.1.0"

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	logoutFlag := flag.Bool("logout", false, "remove stored token and exit")
	debugFlag := flag.Bool("debug", false, "enable debug logging")
	selectFlag := flag.Bool("select", false, "popup selection mode (used internally with tmux display-popup)")
	keysFlag := flag.Bool("keys", false, "show key bindings (used internally with tmux display-popup)")
	albumFlag := flag.Bool("a", false, "browse artists and albums")
	playlistFlag := flag.Bool("p", false, "browse playlists (default)")
	exportFlag := flag.Bool("export", false, "print playlists as Markdown (optionally filtered by a name given as argument)")
	outFlag := flag.String("o", "", "write export to this file, or one file per playlist if it is a directory")
	flag.Parse()

	// Go's flag package stops at the first non-flag argument, so keep parsing
	// after each positional one to allow `spotine -export name -o out.md`.
	var positional []string
	for args := flag.Args(); len(args) > 0; args = flag.Args() {
		positional = append(positional, args[0])
		flag.CommandLine.Parse(args[1:])
	}

	if *versionFlag {
		fmt.Println("spotine", version)
		return
	}

	if *albumFlag && *playlistFlag {
		fmt.Fprintln(os.Stderr, "error: -a and -p are mutually exclusive")
		os.Exit(1)
	}

	if *logoutFlag {
		if err := DeleteToken(); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stderr, "logout error:", err)
			os.Exit(1)
		}
		fmt.Println("Logged out.")
		return
	}

	cfg = LoadConfig()
	theme := cfg.Theme.resolve()
	if theme.UseTerminalColor {
		if hex := queryTerminalFgHex(); hex != "" {
			theme.Accent = hex
		}
	}
	initStyles(theme)
	initGradient()

	if *keysFlag {
		m := newModel(nil, modePlaylist, false, true)
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	// Flags beat the config file, which beats the playlist default.
	mode := modePlaylist
	if m, ok := parseMode(cfg.UI.DefaultMode); ok {
		mode = m
	}
	switch {
	case *albumFlag:
		mode = modeAlbum
	case *playlistFlag:
		mode = modePlaylist
	}

	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	if clientID == "" {
		clientID = cfg.Spotify.ClientID
	}
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if clientSecret == "" {
		clientSecret = cfg.Spotify.ClientSecret
	}
	// An adopted token can only be refreshed with the credentials that issued
	// it, so fall back to the superseded app's config rather than re-prompting.
	if clientID == "" || clientSecret == "" {
		if id, secret := legacyCredentials(); id != "" && secret != "" {
			clientID, clientSecret = id, secret
		}
	}
	redirectURI := os.Getenv("SPOTIFY_REDIRECT_URI")
	if redirectURI == "" {
		redirectURI = cfg.Spotify.RedirectURI
	}

	if clientID == "" || clientSecret == "" {
		fmt.Fprintln(os.Stderr, "error: Spotify credentials not set.")
		fmt.Fprintln(os.Stderr, "  Set environment variables:  SPOTIFY_CLIENT_ID / SPOTIFY_CLIENT_SECRET")
		fmt.Fprintln(os.Stderr, "  Or add to config file:      ~/.config/spotine/config.toml")
		fmt.Fprintln(os.Stderr, "  See: https://github.com/256x/spotine#setup")
		os.Exit(1)
	}

	debugLog := func(format string, args ...any) {}
	if *debugFlag {
		logger := log.New(os.Stderr, "[debug] ", log.LstdFlags)
		debugLog = logger.Printf
	}

	if AdoptLegacyToken() {
		debugLog("adopted token from %s", legacyApp)
	}

	token, err := LoadToken()
	if err != nil {
		debugLog("no stored token, starting OAuth flow")
		token, err = Authenticate(clientID, clientSecret, redirectURI)
		if err != nil {
			fmt.Fprintln(os.Stderr, "authentication failed:", err)
			os.Exit(1)
		}
		if err := SaveToken(token); err != nil {
			fmt.Fprintln(os.Stderr, "warning: could not save token:", err)
		}
	}

	client := NewSpotifyClient(token, clientID, clientSecret, debugLog)

	if *exportFlag {
		if err := runExport(client, strings.Join(positional, " "), *outFlag); err != nil {
			fmt.Fprintln(os.Stderr, "export error:", err)
			os.Exit(1)
		}
		return
	}

	m := newModel(client, mode, *selectFlag, false)
	// The popup runs as its own process, so the cache is also how it learns
	// what is playing for the empty-search shortcut.
	if s, err := LoadCache(); err == nil {
		m.playback = s.Playback
	}

	opts := []tea.ProgramOption{tea.WithAltScreen()}
	p := tea.NewProgram(m, opts...)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
