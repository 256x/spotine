# spotine User Guide

## First Run

On first launch, spotine opens a browser window for Spotify OAuth authentication.
After logging in, the token is saved to `~/.config/spotine/token.json` and reused on subsequent runs.

A default config file is created at `~/.config/spotine/config.toml` if it doesn't exist.

**Coming from slp?** There is nothing to do. spotine copies slp's token on first run and falls
back to slp's credentials, so no new login is needed. The copy is one-way: after it, the two
keep separate tokens.

---

## Two Modes

The player line and every playback key are the same either way. What changes is what the
picker searches for.

| Mode | Flow |
|---|---|
| **playlist** | search or list playlists → device → play |
| **artist** | search an artist → albums → tracks → device → play |

Start in one:

```sh
spotine        # playlist mode, or whatever default_mode says
spotine -p     # playlist mode
spotine -a     # artist mode
```

Switch while running: press `tab` in the search box. The choice sticks, including across
popups opened from a tmux or zellij pane.

Set the default:

```toml
[ui]
default_mode = "album"   # or "playlist"
```

`-p` / `-a` override the config for a single run. Passing both is an error.

---

## Typical Workflow

**1. Start spotine in a dedicated pane**

In tmux, open a small pane at the bottom:

```
Ctrl-b : split-window -v -l 1 'spotine'
```

In zellij, open a new pane:

```
Ctrl-p n  (then run spotine)
```

**2. Open the picker**

Press `space`. A floating popup appears using your multiplexer's native window.

**3. Find something**

In playlist mode, press `enter` with no input for your own playlists, or type a word to
search Spotify. In artist mode, type an artist name. Press `tab` to swap modes.

**4. Select a device**

Use `j` / `k` to navigate and `enter` to select. `backspace` goes back one screen.

**5. Go back to work**

The popup closes. The player line updates. Done.

---

## Playlist Mode

The popup opens with a text cursor:

```
╭─ playlist mode ──────────────────────────────────╮
│ ❯ █                                              │
│ ──────────────────────────────────────────────── │
╰──────────────────────────────────────────────────╯
  enter:my playlists  tab:artist mode  bs:back  esc:close
```

**Loading your playlists:**
Press `enter` with empty input. Your playlists load, sorted by track count (most tracks first).

**Searching Spotify:**
Type a word (e.g. `lofi`) and press `enter`. Returns matching public playlists.

**Filtering loaded results:**
Press `/` to go back to the search box. Typing there searches Spotify again; to narrow what is
already loaded without a new request, use the filter inside the list.

**Playing:**
`enter` on a playlist opens the device picker, then plays from the first track.

---

## Artist Mode

Three screens, one per step:

```
artist mode  →  albums — <artist>  →  <album>  →  select device
```

**Searching:**
Type an artist name and press `enter`. Japanese, romanized, or partial names all work.

An empty box searches for whoever is playing right now — handy for "what else did they make?".
There is no list of artists you follow: that needs an OAuth scope spotine does not request.

**Albums:**
Listed **oldest first**. Reissues and remasters carry recent release dates, so newest-first
would bury an artist's actual work under decades of repackaging.

Long catalogues run past a hundred entries. Press `/` to filter the list by title — this is
local, no request is made.

**Every pressing, or one per record:**

By default every pressing is listed: the original, the remaster, the deluxe edition. Which one
you want is your call. To see one entry per record instead:

```toml
[albums]
collapse_reissues = true
```

With this on, only the original of each record is kept. The original wins rather than the
largest edition, so John Mellencamp's *Scarecrow* shows as the 1985 album and sits at 1985 in
the list, not as the 2022 deluxe mix sitting at 2022.

**Tracks:**

```
╭─ Scarecrow ──────────────────────────────── 1/12 ╮
│ ❯ play all (11 tracks)                           │
│   1. Rain On The Scarecrow                  3:47 │
│   2. Grandma's Theme                        0:53 │
│   3. Small Town                             3:41 │
╰──────────────────────────────────────────────────╯
```

The top row plays the whole record. Picking a track starts there and plays on through the rest
of the album — it does not stop after one song.

---

## Device Picker

The last screen before playback shows every available Spotify Connect device.
The active one is marked with `·` in the right column.

| Key | Action |
|---|---|
| `j` / `k` | navigate list |
| `enter` | select device and start playback |
| `backspace` / `esc` | back one screen |
| `q` / `ctrl+c` | quit |

`backspace` returns to the track list in artist mode, and to the playlist list in playlist mode.

---

## Going Back

`backspace` always steps back exactly one screen:

```
device → tracks → albums → artists → search → player     (artist mode)
device → playlists → search → player                     (playlist mode)
```

`esc` or `q` closes the whole popup at once.

---

## Progress Bar

The progress bar fills the available space between the track info and the right-side
indicators, using `=` for elapsed and `-` for remaining time.

During playback, the track name, artist, and progress bar are rendered together as a single
animated color gradient.

---

## Names in Your Own Language

Spotify stores localized names for many artists and returns them when a request asks for them.
With a Japanese locale, サカナクション is listed and displayed as サカナクション rather than
`sakanaction`, and 米津玄師 keeps its kanji. This applies everywhere: search results, album and
track listings, and the currently-playing line.

This follows `$LANG` (then `LC_MESSAGES`, then `LC_ALL`) automatically. To pin it:

```toml
[ui]
language = "ja-JP"   # or "en", or "none" for Spotify's canonical names
```

---

## Themes

The default theme is `terminal`, which reads the foreground color from your terminal emulator
and uses it as the accent color. This means spotine automatically adapts to your terminal's
color scheme.

To use a different theme:

```toml
[theme]
name = "tokyo-night"
```

Available themes: `terminal` (default), `dracula`, `iceberg`, `monokai`, `solarized-dark`,
`solarized-light`, `nord`, `gruvbox`, `tokyo-night`, `catppuccin`, `rose-pine`, `mono`

Override individual colors (takes effect regardless of named theme):

```toml
[theme]
name = "nord"
accent = "#88c0d0"      # gradient base, borders, icons
selected_fg = "#eceff4" # selected item text
filter_fg = "#ebcb8b"   # filter prompt
```

Colors accept hex (`#rrggbb`) or terminal 256-color numbers.

---

## Spotify Settings

Configure credentials and search behavior in config:

```toml
[spotify]
client_id     = ""   # alternative to SPOTIFY_CLIENT_ID env var
client_secret = ""   # alternative to SPOTIFY_CLIENT_SECRET env var
redirect_uri  = "http://127.0.0.1:8888/callback"
search_limit  = 20   # results per search (max 50)
```

Environment variables (`SPOTIFY_CLIENT_ID`, `SPOTIFY_CLIENT_SECRET`, `SPOTIFY_REDIRECT_URI`)
take precedence over config file values.

A refresh token is bound to the credentials that issued it. If you upgraded from slp and set
your own `client_id` here, it must be the same one slp used, or the copied token cannot be
refreshed. Leaving the fields empty is the safe default.

---

## Icons

Defaults use Nerd Fonts glyphs. If Nerd Fonts is not available, set plain text fallbacks:

```toml
[icons]
play    = "▶"
pause   = "⏸"
volume  = "V"
shuffle = "S"
```

---

## Volume

Volume control requires a Spotify Premium device that supports it.
If the active device doesn't support volume, `j` / `k` will show a status message instead.
The volume indicator is hidden automatically on unsupported devices.

---

## Export

`--export` prints playlists as Markdown to stdout instead of starting the UI:

```sh
spotine --export                      # every playlist
spotine --export jazz                 # only those whose name contains "jazz"
spotine --export jazz -o ~/jazz.md    # write to one file
spotine --export -o ~/playlists/      # existing directory: one <name>.md per playlist
```

The name filter is a case-insensitive substring match against your own playlists.

---

## Without a Multiplexer

spotine works in any terminal. In non-tmux/zellij environments:

- `space` opens an inline popup centered on the screen
- `?` shows key bindings as an inline overlay

The single-line player still works in a full-height terminal window.

---

## Quitting

Press `q`, `esc`, or `ctrl+c` to quit. If a track is playing, spotine pauses it before exiting.

---

## Token Management

```sh
spotine --logout   # remove stored token, re-authenticate on next launch
```

Token is stored at `~/.config/spotine/token.json` and refreshed automatically before expiry.
This is spotine's own copy; `slp --logout` does not affect it.

---

## Troubleshooting

**"no active Spotify device"**
Open Spotify on any device (desktop, phone, web player) and start playing something.
The active device will be detected on the next poll (within 2 seconds).

**Popup doesn't appear in tmux**
Make sure your tmux version supports `display-popup` (tmux 3.2+).

**Icons show as squares or question marks**
Install a [Nerd Font](https://www.nerdfonts.com/) and configure your terminal to use it,
or set plain text icons in config.toml.

**OAuth callback fails**
Make sure `http://127.0.0.1:8888/callback` is listed as a Redirect URI in your Spotify
developer app settings.

**An album you expected is missing**
Three things narrow the list, in order of how much they remove. Compilations, singles and
guest appearances are not listed at all — only full albums. `collapse_reissues`, if you turned
it on, keeps one pressing per record. And Spotify files some acts under several artist
entries: *Miles Davis* and *Miles Davis Quintet* are separate artists with separate
catalogues, so check the other entry in the search results.

**An artist's name shows romanized**
Set `ui.language`, or check that `$LANG` names a real locale. Spotify only has localized
names for some artists; where it has none, the canonical name comes back unchanged.
