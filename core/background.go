package core

import (
	"errors"
	"fmt"
)

// Errors from NewSetBackgroundTime, worded for showing next to the edited time.
var (
	ErrBackgroundEndNotAfterStart  = errors.New("the end must be after the background vocal's start")
	ErrBackgroundStartNotBeforeEnd = errors.New("the start must be before the background vocal's end")
)

// NewSetBackground returns the command that sets the background vocal of
// line i. Setting it to empty removes it, own times and all.
func NewSetBackground(p *Project, i int, text string) (Command, error) {
	if i < 0 || i >= len(p.Lines) {
		return nil, fmt.Errorf("%w: %d", errNoSuchLine, i)
	}
	c := &setBackground{line: i, old: p.Lines[i].Background, oldTiming: p.Lines[i].BackgroundTiming, text: text}
	if text != "" {
		c.timing = c.oldTiming
	}

	return c, nil
}

type setBackground struct {
	line              int
	old, text         string
	oldTiming, timing Timing
}

func (c *setBackground) Do(p *Project) {
	p.Lines[c.line].Background = c.text
	p.Lines[c.line].BackgroundTiming = c.timing
}

func (c *setBackground) Undo(p *Project) {
	p.Lines[c.line].Background = c.old
	p.Lines[c.line].BackgroundTiming = c.oldTiming
}

// NewSetBackgroundTime returns the command that sets one boundary of line
// i's background vocal to at, or with at nil goes back to the line's time.
// The background vocal may start and end anywhere in the song, as long as it
// ends after it starts; nothing is past songEnd, the length of the audio.
func NewSetBackgroundTime(p *Project, i int, edge Edge, at *Ms, songEnd Ms) (Command, error) {
	if i < 0 || i >= len(p.Lines) {
		return nil, fmt.Errorf("%w: %d", errNoSuchLine, i)
	}
	if at != nil && songEnd > 0 && *at > songEnd {
		return nil, fmt.Errorf("%w (%s)", ErrPastSongEnd, songEnd)
	}
	line := p.Lines[i]
	c := &setBackgroundTime{line: i, old: line.BackgroundTiming, timing: line.BackgroundTiming}
	if edge == End {
		c.timing.End = at
	} else {
		c.timing.Begin = at
	}

	line.BackgroundTiming = c.timing
	if t := line.BackgroundTimes(); t.Begin != nil && t.End != nil && *t.End <= *t.Begin {
		if edge == End {
			return nil, fmt.Errorf("%w (%s)", ErrBackgroundEndNotAfterStart, *t.Begin)
		}

		return nil, fmt.Errorf("%w (%s)", ErrBackgroundStartNotBeforeEnd, *t.End)
	}

	return c, nil
}

type setBackgroundTime struct {
	line        int
	old, timing Timing
}

func (c *setBackgroundTime) Do(p *Project)   { p.Lines[c.line].BackgroundTiming = c.timing }
func (c *setBackgroundTime) Undo(p *Project) { p.Lines[c.line].BackgroundTiming = c.old }
