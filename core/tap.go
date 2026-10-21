package core

import (
	"errors"
	"fmt"
	"slices"
)

var (
	// ErrBeforePrevious means a tap would start a line before the previous
	// line starts, for example after seeking back.
	ErrBeforePrevious = errors.New("tap is before the previous line's start")
	// ErrNoOpenLine means there is no started line to end.
	ErrNoOpenLine = errors.New("no line is open")
	errNoSuchLine = errors.New("no such line")
)

// NewTap returns the command for a line tap at time at: it ends the open line,
// if there is one, and starts the next line at the same time. The next line
// is the first untimed one waiting after the last started line (pasted
// lyrics); with none waiting, the tap adds a new line at the end. Lines may
// overlap, but a line never starts before the previous one.
func NewTap(p *Project, at Ms) (Command, error) {
	open := p.OpenLine()
	if open >= 0 && at <= *p.Lines[open].Begin { // ending the open line needs a positive length
		return nil, ErrBeforePrevious
	}
	if prev := p.startBefore(len(p.Lines)); prev >= 0 && at < *p.Lines[prev].Begin {
		return nil, ErrBeforePrevious
	}

	return &tap{at: at, open: open, next: p.NextLine()}, nil
}

type tap struct {
	at   Ms
	open int // the line this tap ends, or -1
	next int // the waiting line this tap starts, or -1 to add one
}

func (c *tap) Do(p *Project) {
	if c.open >= 0 {
		p.Lines[c.open].End = new(c.at)
	}
	if c.next >= 0 {
		p.Lines[c.next].Begin = new(c.at)
	} else {
		p.Lines = append(p.Lines, Line{Timing: Timing{Begin: new(c.at)}})
	}
}

func (c *tap) Undo(p *Project) {
	if c.next >= 0 {
		p.Lines[c.next].Begin = nil
	} else {
		p.Lines = p.Lines[:len(p.Lines)-1]
	}
	if c.open >= 0 {
		p.Lines[c.open].End = nil
	}
}

// NewEndLine returns the command that ends the open line at time at without
// starting another, for a gap such as an instrumental break.
func NewEndLine(p *Project, at Ms) (Command, error) {
	open := p.OpenLine()
	if open < 0 {
		return nil, ErrNoOpenLine
	}
	if at <= *p.Lines[open].Begin {
		return nil, ErrBeforePrevious
	}

	return &endLine{line: open, at: at}, nil
}

type endLine struct {
	line int
	at   Ms
}

func (c *endLine) Do(p *Project)   { p.Lines[c.line].End = new(c.at) }
func (c *endLine) Undo(p *Project) { p.Lines[c.line].End = nil }

// NewSetText returns the command that sets the text of line i.
func NewSetText(p *Project, i int, text string) (Command, error) {
	if i < 0 || i >= len(p.Lines) {
		return nil, fmt.Errorf("%w: %d", errNoSuchLine, i)
	}

	return &setText{line: i, old: p.Lines[i].Text, text: text}, nil
}

type setText struct {
	line      int
	old, text string
}

func (c *setText) Do(p *Project)   { p.Lines[c.line].Text = c.text }
func (c *setText) Undo(p *Project) { p.Lines[c.line].Text = c.old }

// NewInsertLine returns the command that inserts an empty, untimed line at
// index i (len(p.Lines) appends). Other lines keep their times.
func NewInsertLine(p *Project, i int) (Command, error) {
	return NewInsertLines(p, i, []Line{{}})
}

// NewInsertLines returns the command that inserts lines at index i
// (len(p.Lines) appends), such as pasted lyrics; one undo removes them all.
func NewInsertLines(p *Project, i int, lines []Line) (Command, error) {
	if i < 0 || i > len(p.Lines) {
		return nil, fmt.Errorf("%w: %d", errNoSuchLine, i)
	}

	return &insertLines{index: i, lines: slices.Clone(lines)}, nil
}

type insertLines struct {
	index int
	lines []Line
}

func (c *insertLines) Do(p *Project) { p.Lines = slices.Insert(p.Lines, c.index, c.lines...) }
func (c *insertLines) Undo(p *Project) {
	p.Lines = slices.Delete(p.Lines, c.index, c.index+len(c.lines))
}

// NewDeleteLine returns the command that removes line i. Other lines keep
// their times; removing a line leaves a gap rather than closing it.
func NewDeleteLine(p *Project, i int) (Command, error) {
	if i < 0 || i >= len(p.Lines) {
		return nil, fmt.Errorf("%w: %d", errNoSuchLine, i)
	}

	return &deleteLine{index: i, line: p.Lines[i]}, nil
}

type deleteLine struct {
	index int
	line  Line
}

func (c *deleteLine) Do(p *Project)   { p.Lines = slices.Delete(p.Lines, c.index, c.index+1) }
func (c *deleteLine) Undo(p *Project) { p.Lines = slices.Insert(p.Lines, c.index, c.line) }

// Edge picks one boundary of a line.
type Edge int

const (
	// Begin is a line's start time.
	Begin Edge = iota
	// End is a line's end time.
	End
)

// Errors from NewSetTime, worded for showing next to the edited time.
var (
	ErrEndNotAfterStart  = errors.New("the end must be after the line's start")
	ErrStartNotBeforeEnd = errors.New("the start must be before the line's end")
	ErrStartOrder        = errors.New("lines must start in order")
	ErrPastSongEnd       = errors.New("the time is past the end of the song")
)

// NewSetTime returns the command that sets one boundary of line i to at, or
// an error saying why that time can't be used. Only that boundary changes.
// The rules: a line ends after it starts; each line starts at or after the
// previous line's start and at or before the next one's (ends may run past
// the next start, for held words and overlapping singers; untimed lines are
// skipped); nothing is past songEnd, the length of the audio.
func NewSetTime(p *Project, i int, edge Edge, at, songEnd Ms) (Command, error) {
	if i < 0 || i >= len(p.Lines) {
		return nil, fmt.Errorf("%w: %d", errNoSuchLine, i)
	}
	if songEnd > 0 && at > songEnd {
		return nil, fmt.Errorf("%w (%s)", ErrPastSongEnd, songEnd)
	}
	line := p.Lines[i]
	if edge == End {
		if line.Begin != nil && at <= *line.Begin {
			return nil, fmt.Errorf("%w (%s)", ErrEndNotAfterStart, *line.Begin)
		}

		return &setTime{line: i, edge: End, old: line.End, at: at}, nil
	}

	if line.End != nil && at >= *line.End {
		return nil, fmt.Errorf("%w (%s)", ErrStartNotBeforeEnd, *line.End)
	}
	if prev := p.startBefore(i); prev >= 0 && at < *p.Lines[prev].Begin {
		return nil, fmt.Errorf("%w: line %d starts at %s", ErrStartOrder, prev+1, *p.Lines[prev].Begin)
	}
	if next := p.startAfter(i); next >= 0 && at > *p.Lines[next].Begin {
		return nil, fmt.Errorf("%w: line %d starts at %s", ErrStartOrder, next+1, *p.Lines[next].Begin)
	}

	return &setTime{line: i, edge: Begin, old: line.Begin, at: at}, nil
}

type setTime struct {
	line int
	edge Edge
	old  *Ms // nil for an end that was not set
	at   Ms
}

func (c *setTime) boundary(p *Project) **Ms {
	if c.edge == End {
		return &p.Lines[c.line].End
	}

	return &p.Lines[c.line].Begin
}

func (c *setTime) Do(p *Project)   { *c.boundary(p) = new(c.at) }
func (c *setTime) Undo(p *Project) { *c.boundary(p) = c.old }
