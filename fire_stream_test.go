package main

import (
	"strings"
	"testing"
)

// Regression test for the white-artifact bug: a ▀ glyph must never render
// without an explicit 38;5; (fg) or 48;5; (bg) SGR, and unlit cells must be
// spaces — otherwise they show as white blocks/lines on the fire.
func fireLineHasBareGlyph(line string) bool {
	var attrs []string
	i := 0
	for i < len(line) {
		if line[i] == 0x1b {
			end := strings.IndexByte(line[i:], 'm')
			if end < 0 {
				return true
			}
			code := line[i+2 : i+end]
			attrs = nil
			if code != "" {
				attrs = strings.Split(code, ";")
			}
			i += end + 1
			continue
		}
		if line[i] == 0xE2 && i+2 < len(line) && line[i+1] == 0x96 && line[i+2] == 0x80 {
			hasColor := false
			for _, a := range attrs {
				if a == "38" || a == "48" {
					hasColor = true
				}
			}
			if !hasColor {
				return true
			}
			i += 3
			continue
		}
		i++
	}
	return false
}

func TestFireStreamNoDefaultGlyphs(t *testing.T) {
	m := newModel()
	for step := 0; step < 90; step++ {
		fireStep(&m.fireHeat)
		block := m.fireBlock(40)
		for i, line := range strings.Split(block, "\n") {
			if fireLineHasBareGlyph(line) {
				t.Fatalf("frame %d line %d renders a glyph without color:\n%q", step, i, line)
			}
		}
	}
}
