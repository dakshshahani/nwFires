package main

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// A dancing ASCII nugget in the bottom-right corner above the status line,
// built from the nugget mascot SVG (beige body, dark brown tail, side hands).
// nuggetFrames.go is generated art with colors baked in; this file handles
// placement and the dance timing.

// nuggetInterval keeps the dance slow so terminal repaints stay quiet.
const nuggetInterval = 400 * time.Millisecond

type nuggetTickMsg struct{}

func nuggetTick() tea.Cmd {
	return tea.Tick(nuggetInterval, func(time.Time) tea.Msg { return nuggetTickMsg{} })
}

// nuggetLines is the fixed vertical cost of one dance frame.
func nuggetLines() int {
	return len(nuggetFrames[0])
}

// nuggetBlock returns the colored current frame, placed in the bottom-right
// corner with a small margin. Empty when there is no room. Frames are placed
// as a unit so their internal alignment never shifts between poses.
func (m model) nuggetBlock(width int) string {
	f := nuggetFrames[m.nuggetFrame%len(nuggetFrames)]
	maxW := 0
	for _, line := range f {
		if w := ansi.StringWidth(line); w > maxW {
			maxW = w
		}
	}
	pad := width - maxW - 2
	if pad < 1 || len(f) < 2 {
		return ""
	}
	lead := strings.Repeat(" ", pad)
	out := make([]string, len(f))
	for i, line := range f {
		out[i] = lead + line
	}
	return strings.Join(out, "\n")
}

// Foil for unused lint if frames.go ever regenerated empty.
var _ = lipgloss.NewStyle
