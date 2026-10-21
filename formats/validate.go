// Package formats reads and writes lyrics files and checks a project before export.
package formats

import (
	"fmt"
	"strings"

	"ttml-app/core"
)

// Problem is one reason a project can't be exported.
type Problem struct {
	Line   int // 1-based; 0 for the project as a whole
	Reason string
}

func (p Problem) String() string {
	if p.Line == 0 {
		return p.Reason
	}

	return fmt.Sprintf("Line %d: %s", p.Line, p.Reason)
}

// InvalidError is returned when a project with problems is written.
type InvalidError struct {
	Problems []Problem
}

func (e *InvalidError) Error() string {
	reasons := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		reasons[i] = p.String()
	}

	return "can't export: " + strings.Join(reasons, "; ")
}

// Validate lists what keeps p from being exported, in line order. songEnd is
// the length of the audio; 0 skips that check. Overlapping lines are fine:
// an end may run past the next line's start (held words, several singers).
// So is a line with no text: it marks an instrumental break.
func Validate(p *core.Project, songEnd core.Ms) []Problem {
	if len(p.Lines) == 0 {
		return []Problem{{Reason: "There are no lines to export."}}
	}

	var problems []Problem
	add := func(i int, format string, args ...any) {
		problems = append(problems, Problem{Line: i + 1, Reason: fmt.Sprintf(format, args...)})
	}
	prev := -1 // the last line with a start
	for i, line := range p.Lines {
		switch {
		case line.Begin == nil && line.End == nil:
			add(i, "no start or end time")
		case line.Begin == nil:
			add(i, "no start time")
		case line.End == nil:
			add(i, "no end time")
		case *line.End <= *line.Begin:
			add(i, "ends at or before its start")
		}
		if line.Begin != nil {
			if prev >= 0 && *line.Begin < *p.Lines[prev].Begin {
				add(i, "starts before line %d", prev+1)
			}
			prev = i
		}
		if songEnd > 0 && line.End != nil && *line.End > songEnd {
			add(i, "ends after the song (%s)", songEnd)
		}
	}

	return problems
}

const (
	// shortLine is shorter than anyone sings a line: almost always a double tap.
	shortLine core.Ms = 150
	// shortBreak is a break too short to be worth showing as one; more likely
	// a line whose lyric was never typed.
	shortBreak core.Ms = 2000
)

// Warnings lists what exports fine but looks like a mistake, in line order.
// Unlike Validate's problems, these never block a save.
func Warnings(p *core.Project) []Problem {
	var warnings []Problem
	for i, line := range p.Lines {
		if line.Begin == nil || line.End == nil || *line.End <= *line.Begin {
			continue // Validate reports these
		}
		length := *line.End - *line.Begin
		switch {
		case length < shortLine:
			warnings = append(warnings, Problem{Line: i + 1, Reason: fmt.Sprintf("only %d ms long; a double tap?", length)})
		case strings.TrimSpace(line.Text) == "" && length < shortBreak:
			warnings = append(warnings, Problem{Line: i + 1, Reason: "no text, but too short for a break; a missing lyric?"})
		}
	}

	return warnings
}
