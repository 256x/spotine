# spotine

**A Spotify player that lives in one line of your terminal.**

---

## Concept

Most Spotify clients want your attention. `spotine` doesn't.

It is built for people who work in the terminal. You have tmux or zellij open, you're deep
in a task, and you want music — without switching windows, without a GUI, without breaking
flow. The entire player fits in a single line. It sits quietly at the bottom of a pane.
You forget it's there, and that's the point.

**Two ways to reach music**

Sometimes you want a mood, and a playlist made by a stranger runs indefinitely while you
work. Sometimes you want *a record* — front to back, in order, the way it was made.
`spotine` does both from one binary:

```
playlist mode   search or list playlists     → play
album mode      search an artist → albums → tracks → play
```

Press `tab` in the picker to switch between them. Everything else — the player line, the
keys, the device picker — is identical either way.

**Why oldest-first albums?**

Reissues and remasters carry recent release dates. Sorted newest-first, an artist's actual
work sits buried under decades of repackaging. Oldest-first puts the records where they
belong — in the order they were made.

Every pressing is listed by default, because which one you want is your call. If you would
rather see one entry per record, set `collapse_reissues` and only the original of each is
kept.

**Why tmux/zellij?**

The picker uses your multiplexer's native floating window. No alt-screen takeover, no
context switch. Press `space`, pick something, and the popup disappears. Your layout is
untouched.

---

## Requirements

- Go 1.24+
- Spotify Premium
- [Nerd Fonts](https://www.nerdfonts.com/) (recommended, fallback to plain text icons)
- tmux or zellij (recommended, not required)

---

## Install

```sh
go install github.com/256x/spotine@latest
```

---

## Setup

**Upgrading from [slp](https://github.com/256x/slp)?** There is nothing to set up. On first
run `spotine` copies slp's token and credentials, so you are already logged in. slp keeps
working; the two no longer share state after the copy.

Otherwise:

1. Go to [developer.spotify.com/dashboard](https://developer.spotify.com/dashboard) and create an app
2. Add `http://127.0.0.1:8888/callback` as a Redirect URI
3. Set your credentials — either as environment variables:

```sh
export SPOTIFY_CLIENT_ID=your_client_id
export SPOTIFY_CLIENT_SECRET=your_client_secret
# optional: override the default redirect URI
export SPOTIFY_REDIRECT_URI=http://127.0.0.1:8888/callback
```

Or in `~/.config/spotine/config.toml` (created automatically on first run):

```toml
[spotify]
client_id     = "your_client_id"
client_secret = "your_client_secret"
```

Environment variables take precedence over the config file if both are set.

On first launch, a browser window opens for OAuth. The token is saved locally and refreshed
automatically.

---

## tmux setup (recommended)

Add a dedicated 1-line pane at the bottom of your session:

```sh
# in your tmux config or session script
split-window -v -l 1 'spotine'
```

Or start it manually in any pane — the single-line UI works at any height.

## zellij setup (recommended)

```sh
zellij run -- spotine
```

Or add it to your zellij layout as a fixed-size pane.

---

## Keys

| Key | Action |
|---|---|
| `enter` | play / pause |
| `space` | open the picker |
| `h` / `←` | previous track |
| `l` / `→` | next track |
| `k` / `↑` | volume +5 |
| `j` / `↓` | volume -5 |
| `s` | toggle shuffle |
| `?` | key bindings |
| `q` / `esc` | quit (pauses playback) |

In the search box:

| Key | Action |
|---|---|
| `tab` | switch between playlists and artists |
| `enter` (with text) | search Spotify |
| `enter` (empty, playlist mode) | list your own playlists |
| `enter` (empty, album mode) | search for the artist currently playing |
| `backspace` | back / close |
| `esc` | close popup |

In a list:

| Key | Action |
|---|---|
| `j` / `k` | navigate |
| `enter` | playlist → play it; artist → its albums; album → its tracks |
| `/` | search again, or filter the list |
| `backspace` | back one screen |
| `esc` / `q` | close popup |

In the track list:

| Key | Action |
|---|---|
| `enter` (top row) | play the whole record |
| `enter` (on a track) | start there and play on to the end |

In the device picker:

| Key | Action |
|---|---|
| `enter` | select device and start playback |
| `backspace` / `esc` | back one screen |

---

## Configuration

Config file: `~/.config/spotine/config.toml`

```toml
[theme]
# default: "terminal" — reads your terminal's foreground color automatically
# other built-ins: dracula, nord, gruvbox, tokyo-night, catppuccin, rose-pine,
#                  iceberg, monokai, solarized-dark, solarized-light, mono
# name = "terminal"

[icons]
# plain text fallback if Nerd Fonts unavailable
# play = "▶"  pause = "⏸"  volume = "V"  shuffle = "S"

[spotify]
# client_id     = ""          # alternative to SPOTIFY_CLIENT_ID env var
# client_secret = ""          # alternative to SPOTIFY_CLIENT_SECRET env var
# redirect_uri  = "http://127.0.0.1:8888/callback"
# search_limit  = 20          # search results per query (max 50)

[albums]
# collapse_reissues = false   # true keeps only the original of each record

[ui]
default_mode  = "playlist"    # or "album"; -p / -a override it per run
tick_interval = 2             # polling interval in seconds
language      = ""            # names come back in this language; "" follows $LANG
```

### Names in your own language

Spotify stores localized names for many artists and returns them when asked. With a
Japanese locale, サカナクション is listed and displayed as サカナクション rather than
`sakanaction`, and 米津玄師 keeps its kanji.

This follows `$LANG` automatically. Set `language` to a tag like `"ja-JP"` or `"en"` to
pin it, or to `"none"` to always get whatever Spotify considers the canonical name.

---

## Flags

```
spotine -p          browse playlists (default)
spotine -a          browse artists and albums
spotine --version   print version
spotine --logout    remove stored token
spotine --debug     enable debug logging
spotine --export    print playlists as Markdown
spotine -o PATH     write the export to a file, or to a directory as one file per playlist
```

---

## Export

`--export` prints playlists as Markdown to stdout instead of starting the UI. An argument
filters playlists by name (case-insensitive substring match); without one, every playlist
is exported.

```
spotine --export                      # all playlists to stdout
spotine --export jazz                 # only playlists whose name contains "jazz"
spotine --export jazz -o ~/jazz.md    # write to a single file
spotine --export -o ~/playlists/      # existing directory: one <name>.md per playlist
```

Output looks like:

```markdown
# Jazz

- Tracks: 42
- Owner: 256x
- URI: spotify:playlist:xxxxxxxx
- Exported: 2026-08-11

1. So What — Miles Davis
2. Blue in Green — Miles Davis
```

---

## Development

```sh
go test ./...             # unit tests, no network
go test -tags live ./...  # also hits the real Spotify API (needs a token)
```

---

## History

`spotine` merges two earlier apps: [slp](https://github.com/256x/slp), which did playlists,
and an unreleased artist/album player. They had grown near-identical auth, config, and
rendering code, so they became one binary with two modes instead of two codebases drifting
apart. slp remains available but is no longer developed.

---

## License

MIT
