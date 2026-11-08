package main

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Nuggets dances in the bottom-right corner above the status line.
// The art is modeled on the nugget mascot SVG: beige body, dark brown
// tail on the right, two little hands at the sides, dark eyes and a tooth.

var (
	nuggetBody = lipgloss.NewStyle().Foreground(lipgloss.Color("180")) // beige
	nuggetDark = lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // dark brown tail, dimmed to stay subtle
)

// nuggetFrames alternates poses: arms up / arms down, tail flick up / down.
var nuggetFrames = []string{
	// Pose 1 — hands up, tail flicked up
	strings.Join([]string{
		` .-------.         `,
		`/  ●   ●  \    ~,  `,
		`|    ᨆ    |~ ~~ \   `,
		` \   ▽   /      \   `,
		`  '-.,.-'    __/'   `,
		`    /   \~~~~~~~     `,
	}, "\n"),
	// Pose 2 — hands down, tail down
	strings.Join([]string{
		`  .-------.         `,
		` /  ●   ●  \        `,
		` |    ᨆ    |---,     `,
		`  \   ▽   / \  \~    `,
		`   '-.,.-'   \ \--~   `,
		`     /   \    ~~~~    `,
	}, "\n"),
}

// nuggetInterval keeps the dance slow: an ~8fps terminal repaint for a
// hand-sized block, far below where anyone would notice input delay.
const nuggetInterval = 500 * time.Millisecond

type nuggetTickMsg struct{}

func nuggetTick() tea.Cmd {
	return tea.Tick(nuggetInterval, func(time.Time) tea.Msg { return nuggetTickMsg{} })
}

// nuggetBlock returns the colored current frame, placed in the bottom-right
// corner with a small margin. The frame is kept as one unit (lines' relative
// offsets preserved), empty when there is no room.
func (m model) nuggetBlock(width int) string {
	if width < 16 {
		return ""
	}
	raw := strings.Split(nuggetFrames[m.nuggetFrame%len(nuggetFrames)], "\n")
	lines := make([]string, len(raw))
	w := 0
	for i, line := range raw {
		line = strings.TrimRight(line, " ")
		lines[i] = line
		if tw := ansi.StringWidth(line); tw > w {
			w = tw
		}
	}
	pad := width - w - 2
	if pad < 0 {
		return ""
	}
	lead := strings.Repeat(" ", pad)
	for i := range lines {
		lines[i] = lead + nuggetLine(lines[i])
	}
	return strings.Join(lines, "\n")
}

// nuggetLine colors segments: dark for the tail/mouth accents (everything
// after the body's right edge), beige for the rest.
func nuggetLine(line string) string {
	if i := strings.IndexAny(line, "~\\"); i > 0 {
		return nuggetBody.Render(line[:i]) + nuggetDark.Render(line[i:])
	}
	return nuggetBody.Render(line)
}
