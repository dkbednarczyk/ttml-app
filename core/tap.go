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

// NewTap returns the command for a line tap at time at: it starts the line
// waiting there (see NextLineAt) and ends the timed line before it, if that
// has no end yet, at the same time. With no line waiting, the tap adds a new
// line at the end, as long as at is past the last start. Lines may overlap,
// but a line never starts before the previous one.
func NewTap(p *Project, at Ms) (Command, error) {
	next := p.NextLineAt(at)
	prev := p.startBefore(len(p.Lines))
	if next >= 0 {
		prev = p.startBefore(next)
	}
	open := -1
	if prev >= 0 {
		begin := *p.Lines[prev].Begin
		if at < begin {
			return nil, ErrBeforePrevious
		}
		if p.Lines[prev].End == nil {
			if at == begin { // ending it needs a positive length
				return nil, ErrBeforePrevious
			}
			open = prev
		}
	}

	return &tap{at: at, open: open, next: next, singer: p.singerAt(len(p.Lines) - 1)}, nil
}

type tap struct {
	at     Ms
	open   int // the line this tap ends, or -1
	next   int // the waiting line this tap starts, or -1 to add one
	singer int // who sings an added line: the last line's singer
}

func (c *tap) Do(p *Project) {
	if c.open >= 0 {
		p.Lines[c.open].End = new(c.at)
	}
	if c.next >= 0 {
		p.Lines[c.next].Begin = new(c.at)
	} else {
		p.Lines = append(p.Lines, Line{Timing: Timing{Begin: new(c.at)}, Singer: c.singer})
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

// NewEndLine returns the command that ends the line open at time at (see
// OpenLineAt) without starting another, for a gap such as an instrumental
// break.
func NewEndLine(p *Project, at Ms) (Command, error) {
	open := p.OpenLineAt(at)
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
// The inserted lines take the singer of the line before them.
func NewInsertLines(p *Project, i int, lines []Line) (Command, error) {
	if i < 0 || i > len(p.Lines) {
		return nil, fmt.Errorf("%w: %d", errNoSuchLine, i)
	}
	lines = slices.Clone(lines)
	for j := range lines {
		lines[j].Singer = p.singerAt(i - 1)
	}

	return &insertLines{index: i, lines: lines}, nil
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

		return &setTime{line: i, edge: End, old: line.End, at: new(at)}, nil
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

	return &setTime{line: i, edge: Begin, old: line.Begin, at: new(at)}, nil
}

// NewClearTime returns the command that unsets one boundary of line i, such
// as a tap that landed wrong, so that a tap can set it again. A tap that
// starts a line also ends the timed line before it, so where the two times
// are the same moment, both are cleared: the next tap there sets both again.
func NewClearTime(p *Project, i int, edge Edge) (Command, error) {
	if i < 0 || i >= len(p.Lines) {
		return nil, fmt.Errorf("%w: %d", errNoSuchLine, i)
	}
	c := commands{clearBoundary(p, i, edge)}
	at := p.Lines[i].Begin
	other, otherEdge := p.startBefore(i), End
	if edge == End {
		at, other, otherEdge = p.Lines[i].End, p.startAfter(i), Begin
	}
	if at != nil && other >= 0 {
		if c2 := clearBoundary(p, other, otherEdge); c2.old != nil && *c2.old == *at {
			c = append(c, c2)
		}
	}

	return &c, nil // a pointer: History tells commands apart by comparing them
}

func clearBoundary(p *Project, i int, edge Edge) *setTime {
	c := &setTime{line: i, edge: edge}
	c.old = *c.boundary(p)

	return c
}

// commands applies several commands as one, for one undo.
type commands []Command

func (c commands) Do(p *Project) {
	for _, cmd := range c {
		cmd.Do(p)
	}
}

func (c commands) Undo(p *Project) {
	for _, cmd := range slices.Backward(c) {
		cmd.Undo(p)
	}
}

type setTime struct {
	line    int
	edge    Edge
	old, at *Ms // nil for a boundary that is not set
}

func (c *setTime) boundary(p *Project) **Ms {
	if c.edge == End {
		return &p.Lines[c.line].End
	}

	return &p.Lines[c.line].Begin
}

func (c *setTime) Do(p *Project)   { *c.boundary(p) = c.at }
func (c *setTime) Undo(p *Project) { *c.boundary(p) = c.old }

// NewClearTimesFrom returns the command that clears the times of line i and
// every line after it, background vocals' own times too, so that S picks up
// again at line i: for a tap that went wrong, seek back and tap on from
// there. The line before keeps its times, except an end set by the tap that
// started the first cleared line: that end goes too, so the next S ends the
// line before as well.
func NewClearTimesFrom(p *Project, i int) (Command, error) {
	if i < 0 || i >= len(p.Lines) {
		return nil, fmt.Errorf("%w: %d", errNoSuchLine, i)
	}
	c := &clearTimesFrom{index: i, reopen: -1}
	for _, line := range p.Lines[i:] {
		c.old = append(c.old, [2]Timing{line.Timing, line.BackgroundTiming})
	}
	prev, first := p.startBefore(i), p.startAfter(i-1)
	if prev >= 0 && first >= 0 {
		if end := p.Lines[prev].End; end != nil && *end == *p.Lines[first].Begin {
			c.reopen, c.oldEnd = prev, end
		}
	}

	return c, nil
}

type clearTimesFrom struct {
	index  int
	old    [][2]Timing // each cleared line's own and background timing
	reopen int         // the line before whose end is cleared too, or -1
	oldEnd *Ms
}

func (c *clearTimesFrom) Do(p *Project) {
	for j := c.index; j < len(p.Lines); j++ {
		p.Lines[j].Timing, p.Lines[j].BackgroundTiming = Timing{}, Timing{}
	}
	if c.reopen >= 0 {
		p.Lines[c.reopen].End = nil
	}
}

func (c *clearTimesFrom) Undo(p *Project) {
	for j, old := range c.old {
		p.Lines[c.index+j].Timing, p.Lines[c.index+j].BackgroundTiming = old[0], old[1]
	}
	if c.reopen >= 0 {
		p.Lines[c.reopen].End = c.oldEnd
	}
}
