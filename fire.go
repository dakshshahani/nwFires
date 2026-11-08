package main

import (
	"math/rand"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// A small pseudo-3D fire animating in the bottom-right corner above the
// status line. A cellular-automaton heat field (the classic PSX-DOOM fire
// algorithm, tightened) is shaded through an ember→flame→heat palette and
// drawn at 2× vertical resolution with half-block glyphs.

const (
	fireCols   = 9  // sim columns
	fireRows   = 18 // sim rows (drawn at 2 per terminal line)
	fireHeight = 9  // terminal lines reserved
	fireWidth  = 9  // terminal columns reserved
)

// firePalette maps heat 0..9 to 256-color values: ember < flame < peak
// (matches the tuned render harness).
var fireFg = [10]int{0, 88, 124, 166, 202, 208, 214, 217, 220, 229}

type fireTickMsg struct{}

// fireInterval is fast enough that the flames feel alive, slow enough that
// the app's keystroke path never queues behind a repaint.
const fireInterval = 120 * time.Millisecond

func fireTick() tea.Cmd {
	return tea.Tick(fireInterval, func(time.Time) tea.Msg { return fireTickMsg{} })
}

// fireStep advances the heat field one tick. Bottom row burns at full heat,
// cells inherit the heat below with a 3-way-neighbor drift (the "3D" lean)
// and cool with jitter plus occasional gusts for flicker. Tuned off a render
// harness so the plume reads as a flame, not red haze.
func fireStep(heat *[fireRows][fireCols]int) {
	for y := fireRows - 2; y >= 0; y-- {
		for x := 0; x < fireCols; x++ {
			below := heat[y+1][x]
			// 3-way spread: heat drifts left/center/right by chance
			if x > 0 && rand.Intn(4) < 1 {
				if h := heat[y+1][x-1] - 1; h > below {
					below = h
				}
			}
			if x < fireCols-1 && rand.Intn(4) < 1 {
				if h := heat[y+1][x+1] - 1; h > below {
					below = h
				}
			}
			cool := 1
			if rand.Intn(100) < 45 {
				cool = 2
			}
			if rand.Intn(100) < 12 { // gust: puts flicker holes in the plume
				cool += 4
			}
			heat[y][x] = max(0, below-cool)
		}
	}
	for x := range fireCols {
		heat[fireRows-1][x] = 9 // full-burn fuse row
	}
}

// fireBlock renders one frame. Heat is shown in half-blocks: fg = upper sim
// row, bg = lower. Flames are placed as a unit, right-aligned with a margin.
func (m model) fireBlock(width int) string {
	heat := &m.fireHeat
	// width check: needs the block plus margin
	maxW := fireWidth
	pad := width - maxW - 2
	if pad < 1 {
		return ""
	}
	lead := strings.Repeat(" ", pad)
	var lines []string
	for r := 0; r < fireRows/2; r++ {
		var sb strings.Builder
		wrote := false
		runFg, runBg, runLen := 0, 0, 0
		flush := func() {
			if runLen == 0 {
				return
			}
			sb.WriteString("\x1b[")
			parts := []string{}
			if runFg > 0 {
				parts = append(parts, "38;5;"+itoa(fireFg[runFg]))
			}
			if runBg > 0 {
				parts = append(parts, "48;5;"+itoa(fireFg[runBg]))
			}
			sb.WriteString(strings.Join(parts, ";"))
			sb.WriteString("m")
			for i := 0; i < runLen; i++ {
				sb.WriteRune('▀')
			}
			runLen = 0
		}
		for c := 0; c < fireCols; c++ {
			t := heat[r*2][c]
			b := heat[r*2+1][c]
			if t == runFg && b == runBg {
				runLen++
				continue
			}
			flush()
			runFg, runBg = t, b
			runLen = 1
			if t > 0 || b > 0 {
				wrote = true
			}
		}
		flush()
		line := ""
		if wrote {
			line = "\x1b[m" // reset after painted run
			line = sb.String() + "\x1b[m"
		}
		lines = append(lines, lead+line)
	}
	return strings.Join(lines, "\n")
}

// fireLines is the fixed vertical cost of the corner block, matching the
// height the Window-aware layout reserves.
func fireLines() int { return fireHeight }

func itoa(n int) string {
	return strconv.Itoa(n)
}
