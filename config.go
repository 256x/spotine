package main

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

//go:embed spotine.toml
var sampleConfig []byte

// legacyApp is the app spotine supersedes. Its config and token are read once,
// on first run, so upgrading does not force a fresh login.
const legacyApp = "slp"

// Spotify's own ceiling for search results per request.
const maxSearchLimit = 50

type ThemeConfig struct {
	Name       string `toml:"name"`
	Accent     string `toml:"accent"`
	SelectedFg string `toml:"selected_fg"`
	FilterFg   string `toml:"filter_fg"`
}

type IconsConfig struct {
	// App marks the line as this player's, so it is distinguishable from any
	// other one-line pane. Set it to "" to reclaim the column.
	App     string `toml:"app"`
	Play    string `toml:"play"`
	Pause   string `toml:"pause"`
	Volume  string `toml:"volume"`
	Shuffle string `toml:"shuffle"`
}

type SpotifyConfig struct {
	ClientID     string `toml:"client_id"`
	ClientSecret string `toml:"client_secret"`
	RedirectURI  string `toml:"redirect_uri"`
	SearchLimit  int    `toml:"search_limit"`
}

// searchLimit clamps the configured value to what the API will actually honour.
func (s SpotifyConfig) searchLimit() int {
	if s.SearchLimit <= 0 || s.SearchLimit > maxSearchLimit {
		return maxSearchLimit
	}
	return s.SearchLimit
}

type UIConfig struct {
	TickInterval int    `toml:"tick_interval"`
	DefaultMode  string `toml:"default_mode"`
}

type Config struct {
	Theme   ThemeConfig   `toml:"theme"`
	Icons   IconsConfig   `toml:"icons"`
	Spotify SpotifyConfig `toml:"spotify"`
	UI      UIConfig      `toml:"ui"`
}

type resolvedTheme struct {
	Accent           string
	SelectedFg       string
	FilterFg         string
	UseTerminalColor bool
}

var builtinThemes = map[string]resolvedTheme{
	"terminal": {
		Accent:           "#84a0c6", // placeholder, replaced at runtime with terminal fg
		SelectedFg:       "#c6c8d1",
		FilterFg:         "#89b8c2",
		UseTerminalColor: true,
	},
	"dracula": {
		Accent:     "#bd93f9",
		SelectedFg: "#f8f8f2",
		FilterFg:   "#ffb86c",
	},
	"iceberg": {
		Accent:     "#84a0c6",
		SelectedFg: "#c6c8d1",
		FilterFg:   "#89b8c2",
	},
	"monokai": {
		Accent:     "#a6e22e",
		SelectedFg: "#f8f8f2",
		FilterFg:   "#e6db74",
	},
	"solarized-dark": {
		Accent:     "#268bd2",
		SelectedFg: "#fdf6e3",
		FilterFg:   "#b58900",
	},
	"solarized-light": {
		Accent:     "#268bd2",
		SelectedFg: "#002b36",
		FilterFg:   "#b58900",
	},
	"nord": {
		Accent:     "#5e81ac",
		SelectedFg: "#eceff4",
		FilterFg:   "#ebcb8b",
	},
	"gruvbox": {
		Accent:     "#689d6a",
		SelectedFg: "#ebdbb2",
		FilterFg:   "#d79921",
	},
	"tokyo-night": {
		Accent:     "#7aa2f7",
		SelectedFg: "#c0caf5",
		FilterFg:   "#e0af68",
	},
	"catppuccin": {
		Accent:     "#cba6f7",
		SelectedFg: "#cdd6f4",
		FilterFg:   "#fab387",
	},
	"rose-pine": {
		Accent:     "#c4a7e7",
		SelectedFg: "#e0def4",
		FilterFg:   "#f6c177",
	},
	"mono": {
		Accent:     "15",
		SelectedFg: "0",
		FilterFg:   "7",
	},
}

func defaultConfig() Config {
	return Config{
		Theme: ThemeConfig{
			Name: "terminal",
		},
		Icons: IconsConfig{
			App:     "", // nf-fa-spotify
			Play:    "󰐊",
			Pause:   "󰏤",
			Volume:  "󰕾",
			Shuffle: "󰒝",
		},
		Spotify: SpotifyConfig{
			RedirectURI: "http://127.0.0.1:8888/callback",
			SearchLimit: 20,
		},
		UI: UIConfig{
			TickInterval: 2,
			DefaultMode:  "playlist",
		},
	}
}

func (t ThemeConfig) resolve() resolvedTheme {
	base := resolvedTheme{
		Accent:     "62",
		SelectedFg: "230",
		FilterFg:   "220",
	}
	if t.Name != "" {
		if b, ok := builtinThemes[t.Name]; ok {
			base = b
		}
	}
	if t.Accent != "" {
		base.Accent = t.Accent
	}
	if t.SelectedFg != "" {
		base.SelectedFg = t.SelectedFg
	}
	if t.FilterFg != "" {
		base.FilterFg = t.FilterFg
	}
	return base
}

func configDir() string {
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, "spotine")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "spotine")
}

func legacyConfigDir() string {
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, legacyApp)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", legacyApp)
}

func configPath() string {
	return filepath.Join(configDir(), "config.toml")
}

func LoadConfig() Config {
	cfg := defaultConfig()
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, sampleConfig, 0o644)
		return cfg
	}
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return defaultConfig()
	}
	return cfg
}

// legacyCredentials reads client_id/client_secret from slp's config. An adopted
// refresh token is bound to the credentials that issued it, so those are the
// ones that must be used to refresh it.
func legacyCredentials() (id, secret string) {
	data, err := os.ReadFile(filepath.Join(legacyConfigDir(), "config.toml"))
	if err != nil {
		return "", ""
	}
	var c Config
	if _, err := toml.Decode(string(data), &c); err != nil {
		return "", ""
	}
	return c.Spotify.ClientID, c.Spotify.ClientSecret
}
