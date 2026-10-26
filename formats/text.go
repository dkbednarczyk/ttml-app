package formats

import (
	"regexp"
	"strings"

	"ttml-app/core"
)

// ParseText turns plain lyrics into untimed lines, one per text line. Blank
// lines and section markers such as "[Verse 1]" or "[Chorus]" are skipped,
// and runs of spaces inside a line become one. Parentheses at the end of a
// line are a background vocal, the way lyric sites write them: "Let it go
// (let it burn)" becomes the line "Let it go" with the background vocal
// "(let it burn)", and a line all in parentheses is only a background vocal.
func ParseText(text string) []core.Line {
	var lines []core.Line
	for raw := range strings.Lines(text) {
		line := strings.Join(strings.Fields(raw), " ")
		if line == "" || isSectionMarker(line) {
			continue
		}
		text, background := splitBackground(line)
		lines = append(lines, core.Line{Text: text, Background: background})
	}

	return lines
}

// isSectionMarker reports whether a line is a heading like "[Chorus]" that
// lyric sites put between parts of a song.
func isSectionMarker(line string) bool {
	return strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]")
}

// repeatMarker is a note like "(x2)" that a line repeats, not a vocal.
var repeatMarker = regexp.MustCompile(`(?i)^\([x×]\s*\d+\)$`)

// splitBackground splits the parenthesized groups at the end of a line, such
// as "(let it burn)" or "(oh) (oh)", from the text before them. Parentheses
// earlier in the line ("I (don't) care") stay part of it, and so do
// unbalanced ones and repeat markers.
func splitBackground(line string) (text, background string) {
	text = line
	for strings.HasSuffix(text, ")") {
		open := groupStart(text)
		if open < 0 || repeatMarker.MatchString(text[open:]) {
			break
		}
		text = strings.TrimSpace(text[:open])
	}

	return text, strings.TrimSpace(line[len(text):])
}

// groupStart returns where the parenthesized group that ends text begins,
// or -1 if its parentheses don't balance.
func groupStart(text string) int {
	depth := 0
	for i := len(text) - 1; i >= 0; i-- {
		switch text[i] {
		case ')':
			depth++
		case '(':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}
