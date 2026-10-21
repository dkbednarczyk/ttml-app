package formats

import (
	"strings"

	"ttml-app/core"
)

// ParseText turns plain lyrics into untimed lines, one per text line. Blank
// lines and section markers such as "[Verse 1]" or "[Chorus]" are skipped,
// and runs of spaces inside a line become one.
func ParseText(text string) []core.Line {
	var lines []core.Line
	for raw := range strings.Lines(text) {
		line := strings.Join(strings.Fields(raw), " ")
		if line == "" || isSectionMarker(line) {
			continue
		}
		lines = append(lines, core.Line{Text: line})
	}

	return lines
}

// isSectionMarker reports whether a line is a heading like "[Chorus]" that
// lyric sites put between parts of a song.
func isSectionMarker(line string) bool {
	return strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]")
}
