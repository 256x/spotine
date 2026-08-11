package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// cachedState is what survives between runs: the playback line can be painted
// before the first API call returns, and the popup subprocess reports back
// which mode the user left it in.
type cachedState struct {
	Playback PlaybackState `json:"playback"`
	Mode     string        `json:"mode"`
}

func cacheDir() string {
	if base := os.Getenv("XDG_CACHE_HOME"); base != "" {
		return filepath.Join(base, "spotine")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cache/spotine"
	}
	return filepath.Join(home, ".cache", "spotine")
}

func stateCachePath() string {
	return filepath.Join(cacheDir(), "state.json")
}

func LoadCache() (cachedState, error) {
	var s cachedState
	data, err := os.ReadFile(stateCachePath())
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, err
	}
	return s, nil
}

func SaveCache(s cachedState) error {
	if err := os.MkdirAll(cacheDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stateCachePath(), data, 0o600)
}

// SavePlayback updates only the playback half, preserving the stored mode.
func SavePlayback(p PlaybackState) error {
	s, _ := LoadCache()
	s.Playback = p
	return SaveCache(s)
}

// SaveMode updates only the mode half, preserving the stored playback. The
// popup runs as its own process, so this is how a mode toggle inside it
// reaches the long-running player.
func SaveMode(m browseMode) error {
	s, _ := LoadCache()
	s.Mode = m.String()
	return SaveCache(s)
}
