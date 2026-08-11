package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	colorful "github.com/lucasb-eyer/go-colorful"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/termenv"
)

// --- styles ---

var (
	stylePopupBorder lipgloss.Style
	styleSelected    lipgloss.Style
	styleTitle       lipgloss.Style
	styleFilter      lipgloss.Style
	styleAccent      lipgloss.Style
	styleDim         lipgloss.Style

	accentColor    colorful.Color
	hasAccentColor bool
)

func initStyles(t resolvedTheme) {
	stylePopupBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.Accent)).
		Padding(0, 1)
	styleSelected = lipgloss.NewStyle().
		Background(lipgloss.Color(t.Accent)).
		Foreground(lipgloss.Color(t.SelectedFg)).
		Bold(true)
	styleTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Accent)).
		Bold(true)
	styleFilter = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.FilterFg))
	styleAccent = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Accent))
	styleDim = lipgloss.NewStyle().
		Faint(true)

	hasAccentColor = false
	if c, err := colorful.Hex(t.Accent); err == nil {
		accentColor = c
		hasAccentColor = true
	} else {
		// Try as a termenv color (handles 256-color indices like "62")
		col := termenv.TrueColor.Color(t.Accent)
		if rgb := termenv.ConvertToRGB(col); rgb != (colorful.Color{}) {
			accentColor = rgb
			hasAccentColor = true
		}
	}
}

// --- view entry point ---

var helpItems = []string{
	"enter  play / pause",
	"h/←   previous track",
	"l/→   next track",
	"k/↑   volume +5",
	"j/↓   volume -5",
	"s     toggle shuffle",
	"spc   open the picker",
	"tab   playlists ↔ artists",
	"/     filter the list",
	"?     key bindings",
	"bs    go back / close",
	"q     quit",
	"",
	"any key to close",
}

func (m model) View() string {
	if !m.ready {
		return ""
	}
	switch m.stage {
	case stageHelp:
		popup := renderPopupBox("keys", helpItems, nil, -1, -1, "", false, m.width, m.height)
		if m.keysMode {
			return overlay("", popup, m.width, m.height)
		}
		return overlay(m.base(), popup, m.width, m.height)
	case stageQuery:
		return m.renderQuery()
	case stageResults:
		return m.renderResults()
	case stageAlbums:
		return m.renderAlbums()
	case stageTracks:
		return m.renderTracks()
	case stageDevices:
		return m.renderDevices()
	default:
		return m.renderPlayerLine()
	}
}

// --- player line ---

func (m model) renderPlayerLine() string {
	return buildPlayerLine(m.playback, m.width, m.currentStatus())
}

func (m model) currentStatus() string {
	if m.statusMessage != "" && time.Now().Before(m.statusExpiry) {
		return m.statusMessage
	}
	return ""
}

// clampLine cuts a rendered line to the pane width. The pieces below are sized
// independently, and in a very narrow pane the fixed ones — the icons, the
// volume and shuffle readout — can exceed the width on their own. Without this
// the line wraps and the single-line pane becomes two.
func clampLine(line string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(line) <= width {
		return line
	}
	return ansi.Truncate(line, width, "")
}

func buildPlayerLine(s PlaybackState, width int, status string) string {
	return clampLine(playerLine(s, width, status), width)
}

func playerLine(s PlaybackState, width int, status string) string {
	if status != "" {
		return runewidth.Truncate(status, width, "…")
	}
	if s.Track == "" && s.DeviceID == "" {
		return styleAccent.Render(cfg.Icons.Pause) +
			styleDim.Render(" no active playback — [space] select")
	}

	playSymbol := styleAccent.Render(cfg.Icons.Play)
	if !s.Playing {
		playSymbol = styleAccent.Render(cfg.Icons.Pause)
	}

	shuffleChar := "-"
	if s.Shuffle {
		shuffleChar = "+"
	}
	right := styleDim.Render(fmt.Sprintf(" %s:%s", cfg.Icons.Shuffle, shuffleChar))
	if s.VolumePercent != nil {
		right = styleDim.Render(fmt.Sprintf(" %s:%d", cfg.Icons.Volume, *s.VolumePercent)) + right
	}

	prefix := playSymbol + " "
	prefixW := lipgloss.Width(prefix)
	rightW := lipgloss.Width(right)
	available := width - prefixW - rightW

	if available < 4 {
		return prefix + runewidth.Truncate(s.Track, available, "…") + right
	}

	text := s.Track + " @ " + s.Artist + " "
	textW := runewidth.StringWidth(text)

	const minBar = 4
	if textW > available-minBar {
		innerMax := available - minBar - 3
		if innerMax < 1 {
			innerMax = 1
		}
		text = runewidth.Truncate(s.Track+" @ "+s.Artist, innerMax, "…") + " "
		textW = runewidth.StringWidth(text)
	}

	barWidth := available - textW
	if barWidth < 0 {
		barWidth = 0
	}

	bar, filledW := progressBarChars(s.ProgressMS, s.DurationMS, barWidth)
	if s.Playing {
		return prefix + grad.Render(text+bar) + right
	}
	return prefix + text +
		styleAccent.Render(bar[:filledW]) +
		styleDim.Render(bar[filledW:]) +
		right
}

// progressBarChars returns the bar as a plain string and the number of filled characters.
func progressBarChars(progressMS, durationMS, width int) (string, int) {
	if width <= 0 {
		return "", 0
	}
	if durationMS <= 0 {
		return strings.Repeat("-", width), 0
	}
	ratio := float64(progressMS) / float64(durationMS)
	if ratio < 0 {
		ratio = 0
	} else if ratio > 1 {
		ratio = 1
	}
	filled := int(float64(width) * ratio)
	return strings.Repeat("=", filled) + strings.Repeat("-", width-filled), filled
}

// --- stage rendering ---

func (m model) base() string {
	if m.selectMode {
		return ""
	}
	return m.renderPlayerLine()
}

// modeLabel names what the current mode browses.
func (m model) modeLabel() string {
	if m.mode == modeAlbum {
		return "artists"
	}
	return "playlists"
}

func (m model) renderQuery() string {
	other := "artists"
	if m.mode == modeAlbum {
		other = "playlists"
	}
	enterLabel := "search"
	if m.mode == modePlaylist && m.query == "" {
		enterLabel = "my playlists"
	}
	hints := renderHints(
		[]string{"enter", enterLabel},
		[]string{"tab", other},
		[]string{"bs", "back"},
		[]string{"esc", "close"},
	)
	popup := renderPopupBox(m.modeLabel(), nil, nil, -1, -1, m.query, true, m.width, m.height)
	return overlay(hints, popup, m.width, m.height)
}

func (m model) renderResults() string {
	if m.loading {
		popup := renderPopupBox(m.modeLabel(), []string{"loading..."}, nil, -1, -1, m.query, false, m.width, m.height)
		return overlay(m.base(), popup, m.width, m.height)
	}

	var hints string
	if m.results.filter.active {
		hints = renderHints([]string{"enter", "done"}, []string{"bs", "back"}, []string{"esc", "clear"})
	} else {
		next := "play"
		if m.mode == modeAlbum {
			next = "albums"
		}
		hints = renderHints([]string{"↑↓", "move"}, []string{"enter", next},
			[]string{"/", "search"}, []string{"bs", "back"}, []string{"esc", "close"})
	}

	var items, rightLabels []string
	if m.mode == modeAlbum {
		for _, a := range m.visibleArtists() {
			items = append(items, a.Name)
			label := ""
			if len(a.Genres) > 0 {
				label = a.Genres[0]
			}
			rightLabels = append(rightLabels, label)
		}
	} else {
		for _, p := range m.visiblePlaylists() {
			items = append(items, p.Name)
			label := ""
			if p.TrackCount > 0 {
				label = fmt.Sprintf("%d", p.TrackCount)
			}
			rightLabels = append(rightLabels, label)
		}
	}

	total := len(items)
	if total == 0 {
		items = []string{"(no results)"}
		rightLabels = nil
	}

	popup := renderPopupBox(m.modeLabel(), items, rightLabels, m.results.cursor, total,
		m.results.filter.text, m.results.filter.active, m.width, m.height)
	return overlay(hints, popup, m.width, m.height)
}

func (m model) renderAlbums() string {
	title := "albums"
	if m.selectedArtist.Name != "" {
		title += " — " + m.selectedArtist.Name
	}

	if m.loading {
		popup := renderPopupBox(title, []string{"loading..."}, nil, -1, -1, "", false, m.width, m.height)
		return overlay(m.base(), popup, m.width, m.height)
	}

	var hints string
	if m.albumFilter.active {
		hints = renderHints([]string{"enter", "done"}, []string{"bs", "back"}, []string{"esc", "clear"})
	} else {
		hints = renderHints([]string{"↑↓", "move"}, []string{"enter", "tracks"},
			[]string{"/", "filter"}, []string{"bs", "artists"}, []string{"esc", "close"})
	}

	albums := m.visibleAlbums()
	items := make([]string, len(albums))
	rightLabels := make([]string, len(albums))
	for i, a := range albums {
		items[i] = a.Name
		year := a.Year()
		switch {
		case year != "" && a.TotalTracks > 0:
			rightLabels[i] = fmt.Sprintf("%s · %d", year, a.TotalTracks)
		case year != "":
			rightLabels[i] = year
		case a.TotalTracks > 0:
			rightLabels[i] = fmt.Sprintf("%d", a.TotalTracks)
		}
	}
	if len(items) == 0 {
		items = []string{"(no albums)"}
		rightLabels = nil
	}

	popup := renderPopupBox(title, items, rightLabels, m.albumCursor, len(albums),
		m.albumFilter.text, m.albumFilter.active, m.width, m.height)
	return overlay(hints, popup, m.width, m.height)
}

func (m model) renderTracks() string {
	title := "tracks"
	if m.selectedAlbum.Name != "" {
		title = m.selectedAlbum.Name
	}

	if m.loading {
		popup := renderPopupBox(title, []string{"loading..."}, nil, -1, -1, "", false, m.width, m.height)
		return overlay(m.base(), popup, m.width, m.height)
	}

	hints := renderHints([]string{"↑↓", "move"}, []string{"enter", "play"},
		[]string{"bs", "albums"}, []string{"esc", "close"})

	// Row 0 is the whole record; the tracks follow it.
	items := make([]string, 0, len(m.tracks)+1)
	rightLabels := make([]string, 0, len(m.tracks)+1)

	all := "play all"
	if n := len(m.tracks); n > 0 {
		all = fmt.Sprintf("play all (%d tracks)", n)
	}
	items = append(items, all)
	rightLabels = append(rightLabels, "")

	for i, t := range m.tracks {
		n := t.TrackNumber
		if n <= 0 {
			n = i + 1
		}
		items = append(items, fmt.Sprintf("%d. %s", n, t.Name))
		rightLabels = append(rightLabels, t.Duration())
	}

	popup := renderPopupBox(title, items, rightLabels, m.trackCursor, len(items), "", false, m.width, m.height)
	return overlay(hints, popup, m.width, m.height)
}

func (m model) renderDevices() string {
	if m.loading {
		popup := renderPopupBox("select device", []string{"loading..."}, nil, -1, -1, "", false, m.width, m.height)
		return overlay(m.base(), popup, m.width, m.height)
	}
	items := make([]string, len(m.devices))
	rightLabels := make([]string, len(m.devices))
	for i, d := range m.devices {
		items[i] = d.Name
		rl := strings.ToLower(d.Type)
		if d.IsActive {
			rl += " ·"
		}
		rightLabels[i] = rl
	}
	if len(items) == 0 {
		items = []string{"(no devices)"}
		rightLabels = nil
	}
	popup := renderPopupBox("select device", items, rightLabels, m.deviceCursor, len(m.devices), "", false, m.width, m.height)
	return overlay(m.base(), popup, m.width, m.height)
}

// renderPopupBox renders a centered popup box. cursor=-1 means no selection, total=-1 suppresses scroll indicator.
func renderPopupBox(title string, items, rightLabels []string, cursor, total int, filter string, filterActive bool, termW, termH int) string {
	maxW := termW - 4
	if maxW > 60 {
		maxW = 60
	}
	if maxW < 20 {
		maxW = 20
	}

	maxRows := termH - 6
	if maxRows < 3 {
		maxRows = 3
	}
	if maxRows > 15 {
		maxRows = 15
	}

	start := 0
	if cursor >= maxRows {
		start = cursor - maxRows + 1
	}
	end := start + maxRows
	if end > len(items) {
		end = len(items)
	}

	innerW := maxW - 4 // border + padding

	var sb strings.Builder

	// Title + scroll indicator
	scrollStr := ""
	if total > 1 && cursor >= 0 {
		scrollStr = fmt.Sprintf("%d/%d", cursor+1, total)
	}
	titleMax := innerW
	if scrollStr != "" {
		titleMax = innerW - runewidth.StringWidth(scrollStr) - 1
	}
	titleRendered := styleTitle.Render(runewidth.Truncate(title, titleMax, ""))
	if scrollStr != "" {
		titleW := lipgloss.Width(titleRendered)
		pad := innerW - titleW - runewidth.StringWidth(scrollStr)
		if pad < 1 {
			pad = 1
		}
		sb.WriteString(titleRendered + strings.Repeat(" ", pad) + styleDim.Render(scrollStr))
	} else {
		sb.WriteString(titleRendered)
	}
	sb.WriteString("\n")

	// Filter line
	if filterActive || filter != "" {
		indicator := "❯ "
		if filterActive {
			indicator = styleFilter.Render("❯ ")
		}
		f := runewidth.Truncate(filter, innerW-2, "…")
		sb.WriteString(indicator + f)
		if filterActive {
			sb.WriteString("█")
		}
		sb.WriteString("\n")
	}

	// Separator
	sb.WriteString(styleDim.Render(strings.Repeat("─", innerW)))
	sb.WriteString("\n")

	// Compute max right-label width
	rightW := 0
	if rightLabels != nil {
		for _, r := range rightLabels {
			if w := runewidth.StringWidth(r); w > rightW {
				rightW = w
			}
		}
		if rightW > 0 {
			rightW++ // +1 for gap
		}
	}
	const prefixW = 2
	textW := innerW - prefixW - rightW

	// Item rows
	for i := start; i < end; i++ {
		rl := ""
		if rightLabels != nil && i < len(rightLabels) {
			rl = rightLabels[i]
		}
		text := items[i]
		if textW > 0 {
			text = runewidth.Truncate(text, textW, "…")
			text = runewidth.FillRight(text, textW)
		}
		if i == cursor {
			full := "❯ " + text
			if rightW > 0 {
				full += " " + runewidth.FillRight(rl, rightW-1)
			}
			sb.WriteString(styleSelected.Render(full))
		} else {
			sb.WriteString("  " + text)
			if rightW > 0 {
				sb.WriteString(" " + styleDim.Render(runewidth.FillRight(rl, rightW-1)))
			}
		}
		sb.WriteString("\n")
	}

	content := strings.TrimRight(sb.String(), "\n")
	return stylePopupBorder.Width(maxW).Render(content)
}

func renderHints(pairs ...[]string) string {
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = styleAccent.Render(p[0]) + styleDim.Render(":"+p[1])
	}
	return strings.Join(parts, styleDim.Render("  "))
}

// overlay centers the popup over the base line, filling the terminal height.
func overlay(base, popup string, termW, termH int) string {
	popupLines := strings.Split(popup, "\n")
	popupH := len(popupLines)

	popupW := 0
	for _, l := range popupLines {
		if w := lipgloss.Width(l); w > popupW {
			popupW = w
		}
	}

	startRow := (termH - popupH) / 2
	if startRow < 0 {
		startRow = 0
	}
	startCol := (termW - popupW) / 2
	if startCol < 0 {
		startCol = 0
	}

	blank := strings.Repeat(" ", termW)
	var sb strings.Builder

	for row := 0; row < termH-1; row++ {
		if row >= startRow && row < startRow+popupH {
			line := popupLines[row-startRow]
			lineW := lipgloss.Width(line)
			rightPad := termW - startCol - lineW
			if rightPad < 0 {
				rightPad = 0
			}
			sb.WriteString(strings.Repeat(" ", startCol))
			sb.WriteString(line)
			sb.WriteString(strings.Repeat(" ", rightPad))
		} else {
			sb.WriteString(blank)
		}
		if row < termH-2 {
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(base)
	return sb.String()
}
