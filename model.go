package main

import (
	"context"
	"os"
	"sort"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	width  int
	height int
	ready  bool

	stage     stage
	prevStage stage // where help returns to
	mode      browseMode

	// search stage
	query   string
	results resultList

	// album mode only
	selectedArtist Artist
	albums         []Album
	albumCursor    int
	albumFilter    listFilter
	selectedAlbum  Album
	tracks         []Track
	trackCursor    int

	devices      []Device
	deviceCursor int

	pendingURI    string
	pendingOffset string

	playback      PlaybackState
	statusMessage string
	statusExpiry  time.Time
	loading       bool
	quitting      bool
	selectMode    bool
	keysMode      bool
	client        *SpotifyClient
}

// resultList holds whichever kind of search result the current mode produced.
type resultList struct {
	playlists []Playlist
	artists   []Artist
	filtered  int // count after filtering, for the scroll indicator
	cursor    int
	filter    listFilter
}

var cfg Config

func newModel(client *SpotifyClient, mode browseMode, selectMode, keysMode bool) model {
	m := model{
		client:     client,
		mode:       mode,
		selectMode: selectMode,
		keysMode:   keysMode,
	}
	if selectMode {
		m.stage = stageQuery
	}
	if keysMode {
		m.stage = stageHelp
	}
	return m
}

func (m model) Init() tea.Cmd {
	if m.selectMode || m.keysMode {
		return nil
	}
	return tea.Batch(
		fetchPlayback(m.client),
		tick(),
		vizTick(),
	)
}

// --- update ---

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case tickMsg:
		return m, tea.Batch(fetchPlayback(m.client), tick())

	case vizTickMsg:
		grad.Tick(m.loading, m.playback.Playing)
		return m, vizTick()

	case playbackUpdatedMsg:
		m.playback = msg.State
		_ = SavePlayback(msg.State)

	case popupClosedMsg:
		m.mode = msg.Mode
		if msg.HasState {
			m.playback = msg.State
			_ = SavePlayback(msg.State)
		}

	case playlistsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.setStatus("failed to load playlists: " + msg.Err.Error())
			m.stage = stageQuery
		} else {
			pl := msg.Playlists
			sort.Slice(pl, func(i, j int) bool {
				return pl[i].TrackCount > pl[j].TrackCount
			})
			m.results = resultList{playlists: pl, filtered: len(pl)}
		}

	case artistsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.setStatus("failed to search artists: " + msg.Err.Error())
			m.stage = stageQuery
		} else {
			m.results = resultList{artists: msg.Artists, filtered: len(msg.Artists)}
		}

	case albumsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.setStatus("failed to load albums: " + msg.Err.Error())
			m.stage = stageResults
		} else {
			m.albums = msg.Albums
			m.albumCursor = 0
			m.albumFilter.clear()
		}

	case tracksLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.setStatus("failed to load tracks: " + msg.Err.Error())
			m.stage = stageAlbums
		} else {
			m.tracks = msg.Tracks
			m.trackCursor = 0
		}

	case devicesLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.stage = m.previousStage()
			m.setStatus("failed to load devices: " + msg.Err.Error())
		} else {
			m.devices = msg.Devices
			m.deviceCursor = 0
			if len(m.devices) == 0 {
				m.stage = m.previousStage()
				m.setStatus("no devices available")
			}
		}

	case apiErrorMsg:
		m.setStatus(friendlyError(msg.Err))

	case statusMsg:
		m.setStatus(msg.Text)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m *model) setStatus(s string) {
	m.statusMessage = s
	m.statusExpiry = time.Now().Add(4 * time.Second)
}

func friendlyError(err error) string {
	if err == nil {
		return ""
	}
	return "error: " + err.Error()
}

func (m model) requireDevice() (model, bool) {
	if m.playback.DeviceID == "" {
		m.setStatus("no active Spotify device")
		return m, false
	}
	return m, true
}

func exePath() string {
	exe, err := os.Executable()
	if err != nil {
		return os.Args[0]
	}
	return exe
}

// visibleArtists / visiblePlaylists apply the local filter to the raw results.
func (m model) visiblePlaylists() []Playlist {
	return filterByName(m.results.playlists, m.results.filter.text, playlistName)
}

func (m model) visibleArtists() []Artist {
	return filterByName(m.results.artists, m.results.filter.text, artistName)
}

func (m model) visibleAlbums() []Album {
	return filterByName(m.albums, m.albumFilter.text, albumName)
}

// resultCount is how many rows the results stage is currently showing.
func (m model) resultCount() int {
	if m.mode == modeAlbum {
		return len(m.visibleArtists())
	}
	return len(m.visiblePlaylists())
}

// --- key handling ---

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.keysMode {
		return m, tea.Quit
	}
	switch m.stage {
	case stageHelp:
		m.stage = m.prevStage
		return m, nil
	case stageQuery:
		return m.handleQueryKey(msg)
	case stageResults:
		return m.handleResultsKey(msg)
	case stageAlbums:
		return m.handleAlbumsKey(msg)
	case stageTracks:
		return m.handleTracksKey(msg)
	case stageDevices:
		return m.handleDevicesKey(msg)
	default:
		return m.handlePlayerKey(msg)
	}
}

// openHelp shows the key list, remembering where to return to.
func (m model) openHelp() (model, tea.Cmd) {
	if os.Getenv("TMUX") != "" || os.Getenv("ZELLIJ") != "" {
		return m, openExternalHelp()
	}
	m.prevStage = m.stage
	m.stage = stageHelp
	return m, nil
}

// back steps one stage toward the player, quitting the popup subprocess when
// it runs off the top.
func (m model) back() (tea.Model, tea.Cmd) {
	prev := m.previousStage()
	if prev == stagePlayer && m.selectMode {
		return m, tea.Quit
	}
	m.stage = prev
	return m, nil
}

// closePopup dismisses the whole popup.
func (m model) closePopup() (tea.Model, tea.Cmd) {
	if m.selectMode {
		return m, tea.Quit
	}
	m.stage = stagePlayer
	return m, nil
}

func (m model) handlePlayerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "ctrl+c":
		m.quitting = true
		client := m.client
		deviceID := m.playback.DeviceID
		playing := m.playback.Playing
		return m, func() tea.Msg {
			if playing && deviceID != "" {
				_ = client.Pause(context.Background(), deviceID)
			}
			return tea.QuitMsg{}
		}

	case "enter":
		if m, ok := m.requireDevice(); !ok {
			return m, nil
		}
		var cmd tea.Cmd
		if m.playback.Playing {
			cmd = func() tea.Msg {
				if err := m.client.Pause(context.Background(), m.playback.DeviceID); err != nil {
					return apiErrorMsg{Err: err}
				}
				s := m.playback
				s.Playing = false
				return playbackUpdatedMsg{State: s}
			}
		} else {
			cmd = func() tea.Msg {
				if err := m.client.Resume(context.Background(), m.playback.DeviceID); err != nil {
					return apiErrorMsg{Err: err}
				}
				s := m.playback
				s.Playing = true
				return playbackUpdatedMsg{State: s}
			}
		}
		return m, cmd

	case "l", "right":
		if m, ok := m.requireDevice(); !ok {
			return m, nil
		}
		return m, skipCmd(m.client, m.playback.DeviceID, m.client.Next)

	case "h", "left":
		if m, ok := m.requireDevice(); !ok {
			return m, nil
		}
		return m, skipCmd(m.client, m.playback.DeviceID, m.client.Previous)

	case "k", "up":
		return m.adjustVolume(+5)

	case "j", "down":
		return m.adjustVolume(-5)

	case "?":
		return m.openHelp()

	case "s", "S":
		if m, ok := m.requireDevice(); !ok {
			return m, nil
		}
		newShuffle := !m.playback.Shuffle
		m.playback.Shuffle = newShuffle
		return m, func() tea.Msg {
			if err := m.client.SetShuffle(context.Background(), m.playback.DeviceID, newShuffle); err != nil {
				return apiErrorMsg{Err: err}
			}
			return nil
		}

	case " ":
		if os.Getenv("TMUX") != "" || os.Getenv("ZELLIJ") != "" {
			return m, openExternalSelect(m.client, m.mode)
		}
		return m.openQuery(), nil
	}

	return m, nil
}

func (m model) adjustVolume(delta int) (model, tea.Cmd) {
	if m.playback.DeviceID == "" {
		return m, func() tea.Msg { return statusMsg{Text: "no active Spotify device"} }
	}
	if m.playback.VolumePercent == nil {
		return m, func() tea.Msg { return statusMsg{Text: "volume not supported on this device"} }
	}
	newVol := *m.playback.VolumePercent + delta
	if newVol < 0 {
		newVol = 0
	}
	if newVol > 100 {
		newVol = 100
	}
	m.playback.VolumePercent = &newVol
	deviceID := m.playback.DeviceID
	client := m.client
	return m, func() tea.Msg {
		if err := client.SetVolume(context.Background(), deviceID, newVol); err != nil {
			return apiErrorMsg{Err: err}
		}
		return nil
	}
}

func (m model) openQuery() model {
	m.stage = stageQuery
	m.query = ""
	m.results = resultList{}
	m.albums = nil
	m.tracks = nil
	return m
}

// --- stage: query ---

func (m model) handleQueryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m.closePopup()

	case "tab":
		// Switching what we are browsing invalidates any results on hand.
		m.mode = m.mode.toggled()
		m.results = resultList{}
		_ = SaveMode(m.mode)
		return m, nil

	case "enter":
		query := m.query
		if query == "" && m.mode == modeAlbum {
			// Nothing typed: fall back to whoever is playing right now. There
			// is no scope-free way to list followed artists.
			if m.playback.Artist == "" {
				return m, func() tea.Msg { return statusMsg{Text: "type an artist name"} }
			}
			query = m.playback.Artist
			m.query = query
		}
		m.loading = true
		m.results = resultList{}
		m.stage = stageResults
		if m.mode == modeAlbum {
			return m, searchArtists(m.client, query)
		}
		if query == "" {
			// Playlist mode keeps slp's shortcut: an empty box lists your own.
			return m, fetchPlaylists(m.client)
		}
		return m, searchPlaylists(m.client, query)

	case "backspace":
		if len(m.query) > 0 {
			_, size := utf8.DecodeLastRuneInString(m.query)
			m.query = m.query[:len(m.query)-size]
			return m, nil
		}
		return m.closePopup()

	case "?":
		return m.openHelp()

	default:
		if len(msg.Runes) > 0 {
			m.query += string(msg.Runes)
		}
	}
	return m, nil
}

// --- stage: results (playlists or artists) ---

func (m model) handleResultsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.results.filter.active {
		return m.handleListFilterKey(msg, &m.results.filter, &m.results.cursor)
	}

	switch msg.String() {
	case "q", "esc":
		return m.closePopup()

	case "backspace":
		return m.back()

	case "j", "down":
		if m.results.cursor < m.resultCount()-1 {
			m.results.cursor++
		}

	case "k", "up":
		if m.results.cursor > 0 {
			m.results.cursor--
		}

	case "/":
		m.stage = stageQuery
		return m, nil

	case "?":
		return m.openHelp()

	case "enter":
		if m.loading {
			break
		}
		if m.mode == modeAlbum {
			artists := m.visibleArtists()
			if len(artists) == 0 {
				break
			}
			m.selectedArtist = artists[m.results.cursor]
			m.stage = stageAlbums
			m.loading = true
			m.albums = nil
			m.albumCursor = 0
			m.albumFilter.clear()
			return m, fetchAlbums(m.client, m.selectedArtist.ID)
		}
		playlists := m.visiblePlaylists()
		if len(playlists) == 0 {
			break
		}
		// A playlist plays as a whole; the offset is resolved at playback time
		// so it starts at the top rather than wherever Spotify left off.
		m.pendingURI = playlists[m.results.cursor].URI
		m.pendingOffset = ""
		return m.enterDeviceStage()
	}

	return m, nil
}

// --- stage: albums ---

func (m model) handleAlbumsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.albumFilter.active {
		return m.handleListFilterKey(msg, &m.albumFilter, &m.albumCursor)
	}

	switch msg.String() {
	case "q", "esc":
		return m.closePopup()

	case "backspace":
		return m.back()

	case "j", "down":
		if m.albumCursor < len(m.visibleAlbums())-1 {
			m.albumCursor++
		}

	case "k", "up":
		if m.albumCursor > 0 {
			m.albumCursor--
		}

	case "/":
		m.albumFilter.active = true

	case "?":
		return m.openHelp()

	case "enter":
		if m.loading {
			break
		}
		albums := m.visibleAlbums()
		if len(albums) == 0 {
			break
		}
		m.selectedAlbum = albums[m.albumCursor]
		m.pendingURI = m.selectedAlbum.URI
		m.stage = stageTracks
		m.loading = true
		m.tracks = nil
		m.trackCursor = 0
		return m, fetchTracks(m.client, m.selectedAlbum.ID)
	}

	return m, nil
}

// --- stage: tracks ---

// The track list is offset by one: row 0 plays the whole record, and row i+1
// starts at track i and plays on through the rest of it.
func (m model) handleTracksKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		return m.closePopup()

	case "backspace":
		return m.back()

	case "j", "down":
		if m.trackCursor < len(m.tracks) {
			m.trackCursor++
		}

	case "k", "up":
		if m.trackCursor > 0 {
			m.trackCursor--
		}

	case "?":
		return m.openHelp()

	case "enter":
		if m.loading {
			break
		}
		m.pendingOffset = ""
		if m.trackCursor > 0 {
			if m.trackCursor > len(m.tracks) {
				break
			}
			m.pendingOffset = m.tracks[m.trackCursor-1].URI
		}
		return m.enterDeviceStage()
	}

	return m, nil
}

// --- stage: devices ---

func (m model) enterDeviceStage() (tea.Model, tea.Cmd) {
	m.stage = stageDevices
	m.loading = true
	m.devices = nil
	m.deviceCursor = 0
	return m, fetchDevices(m.client)
}

func (m model) handleDevicesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace":
		return m.back()

	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "j", "down":
		if m.deviceCursor < len(m.devices)-1 {
			m.deviceCursor++
		}

	case "k", "up":
		if m.deviceCursor > 0 {
			m.deviceCursor--
		}

	case "enter":
		if len(m.devices) == 0 {
			break
		}
		selected := m.devices[m.deviceCursor]
		m.stage = stagePlayer
		if m.selectMode {
			client := m.client
			uri, offset, deviceID := m.pendingURI, m.pendingOffset, selected.ID
			return m, func() tea.Msg {
				_ = startPlayback(context.Background(), client, uri, deviceID, offset)
				return tea.QuitMsg{}
			}
		}
		m.setStatus("playing on: " + selected.Name)
		return m, playCmd(m.client, m.pendingURI, selected.ID, m.pendingOffset)
	}
	return m, nil
}

// --- shared list filtering ---

// handleListFilterKey drives the "/" filter shared by the result and album
// lists. Filtering is local, so every keystroke just narrows what is on hand.
func (m model) handleListFilterKey(msg tea.KeyMsg, f *listFilter, cursor *int) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		f.clear()
		*cursor = 0
	case "enter":
		f.active = false
	case "backspace":
		if len(f.text) > 0 {
			_, size := utf8.DecodeLastRuneInString(f.text)
			f.text = f.text[:len(f.text)-size]
			*cursor = 0
		} else {
			f.active = false
		}
	default:
		if len(msg.Runes) > 0 {
			f.text += string(msg.Runes)
			*cursor = 0
		}
	}
	return m, nil
}
