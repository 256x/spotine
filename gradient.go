package main

import (
	"math"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	colorful "github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/termenv"
)

var grad *Gradient

// queryTerminalFgHex queries the terminal's foreground color via OSC 10.
// Returns a "#rrggbb" hex string, or "" on failure.
func queryTerminalFgHex() string {
	output := termenv.NewOutput(os.Stdout)
	if rgb, ok := output.ForegroundColor().(termenv.RGBColor); ok {
		return string(rgb)
	}
	return ""
}

// initGradient must be called after initStyles (uses accentColor from theme).
func initGradient() {
	base := accentColor
	if !hasAccentColor {
		base, _ = colorful.Hex("#84a0c6")
	}
	grad = newGradient(base)
}

type Gradient struct {
	phase  float64
	colors []lipgloss.Color
	step   int
}

func newGradient(base colorful.Color) *Gradient {
	return &Gradient{
		step:   2,
		colors: buildPalette(base, 10),
	}
}

// Tick advances the gradient phase. Speed varies by playback state,
// with a small feedback oscillation to break periodicity.
func (g *Gradient) Tick(loading, playing bool) {
	var base float64
	switch {
	case loading:
		base = 1.4
	case playing:
		base = 0.6
	default:
		return
	}
	g.phase += base + math.Sin(g.phase*0.11)*0.35
}

// Render applies the gradient to text, grouping every g.step characters
// to the same color to reduce jaggedness.
func (g *Gradient) Render(text string) string {
	if len(g.colors) == 0 {
		return text
	}
	nc := len(g.colors)
	offset := int(g.phase)
	runes := []rune(text)
	var sb strings.Builder
	for i, r := range runes {
		idx := ((i / g.step) + offset) % nc
		sb.WriteString(lipgloss.NewStyle().Foreground(g.colors[idx]).Render(string(r)))
	}
	return sb.String()
}

// buildPalette creates a round-trip lightness palette from the base color.
func buildPalette(base colorful.Color, n int) []lipgloss.Color {
	h, s, l := base.Hsl()

	const amp = 0.28
	lo := l - amp
	hi := l + amp
	if lo < 0.1 {
		lo = 0.1
		hi = lo + 2*amp
	}
	if hi > 0.92 {
		hi = 0.92
		lo = hi - 2*amp
	}

	fwd := make([]lipgloss.Color, n)
	for i := range n {
		t := float64(i) / float64(n-1)
		fwd[i] = lipgloss.Color(colorful.Hsl(h, s, lo+(hi-lo)*t).Hex())
	}

	result := make([]lipgloss.Color, 0, 2*n-2)
	result = append(result, fwd...)
	for i := n - 2; i > 0; i-- {
		result = append(result, fwd[i])
	}
	return result
}
