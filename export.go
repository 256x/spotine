package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// runExport writes playlists as Markdown. query filters playlists by name
// (empty means all). outPath is stdout when empty, a directory to write one
// file per playlist, or a file holding every matched playlist.
func runExport(client *SpotifyClient, query, outPath string) error {
	ctx := context.Background()

	all, err := client.GetUserPlaylists(ctx)
	if err != nil {
		return err
	}
	matched := filterByName(all, query, playlistName)
	if len(matched) == 0 {
		if query == "" {
			return fmt.Errorf("no playlists found")
		}
		return fmt.Errorf("no playlist matched %q", query)
	}

	isDir := false
	if outPath != "" {
		if info, err := os.Stat(outPath); err == nil && info.IsDir() {
			isDir = true
		}
	}

	used := make(map[string]bool)
	var buf strings.Builder
	for _, p := range matched {
		tracks, err := client.GetPlaylistTracks(ctx, p.ID)
		if err != nil {
			return fmt.Errorf("%s: %w", p.Name, err)
		}
		md := renderPlaylistMarkdown(p, tracks)

		if !isDir {
			buf.WriteString(md)
			continue
		}
		name := uniqueFileName(used, p.Name)
		dest := filepath.Join(outPath, name)
		if err := os.WriteFile(dest, []byte(md), 0o644); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "saved:", dest)
	}

	if isDir {
		return nil
	}
	if outPath == "" {
		_, err := os.Stdout.WriteString(buf.String())
		return err
	}
	if err := os.WriteFile(outPath, []byte(buf.String()), 0o644); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "saved:", outPath)
	return nil
}

func renderPlaylistMarkdown(p Playlist, tracks []Track) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", p.Name)
	fmt.Fprintf(&b, "- Tracks: %d\n", len(tracks))
	if p.Owner != "" {
		fmt.Fprintf(&b, "- Owner: %s\n", p.Owner)
	}
	if p.URI != "" {
		fmt.Fprintf(&b, "- URI: %s\n", p.URI)
	}
	fmt.Fprintf(&b, "- Exported: %s\n\n", time.Now().Format("2006-01-02"))

	for i, t := range tracks {
		line := fmt.Sprintf("%d. %s", i+1, t.Name)
		if artist := strings.Join(t.Artists, ", "); artist != "" {
			line += " — " + artist
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

// uniqueFileName turns a playlist name into a filesystem-safe .md file name,
// suffixing duplicates so one playlist never overwrites another.
func uniqueFileName(used map[string]bool, name string) string {
	base := strings.Map(func(r rune) rune {
		if r < 0x20 || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '-'
		}
		return r
	}, name)
	base = strings.TrimSpace(base)
	if base == "" || base == "." || base == ".." {
		base = "playlist"
	}

	candidate := base + ".md"
	for i := 2; used[candidate]; i++ {
		candidate = fmt.Sprintf("%s-%d.md", base, i)
	}
	used[candidate] = true
	return candidate
}
