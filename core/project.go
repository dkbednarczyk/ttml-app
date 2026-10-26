// Package core holds the lyrics project and the commands that edit it.
// The UI changes the project only through commands, so every edit can be undone.
package core

import (
	"fmt"
	"slices"
)

// Ms is a time in milliseconds from the start of the song.
type Ms int64

// Timing is a begin/end pair. A nil field is not set yet.
type Timing struct {
	Begin, End *Ms
}

// Line is one lyric line.
type Line struct {
	Text string
	Timing
	// Singer is who sings the line: an index into Project.Singers, or Everyone.
	Singer int
	// Background is a background vocal sung over the line, such as a scream
	// stacked under the lead; empty for none.
	Background string
	// BackgroundTiming is the background vocal's own start and end, for one
	// that comes in late or runs on; a nil field takes the line's time.
	BackgroundTiming Timing
}

// BackgroundTimes returns when the line's background vocal is sung: its own
// times where it has them, the line's otherwise.
func (l *Line) BackgroundTimes() Timing {
	t := l.BackgroundTiming
	if t.Begin == nil {
		t.Begin = l.Begin
	}
	if t.End == nil {
		t.End = l.End
	}

	return t
}

// Everyone is the Line.Singer of a line all the singers sing together.
const Everyone = -1

// Singer is one person who sings. Name may be empty.
type Singer struct {
	Name string
}

// MetaEntry is one piece of song metadata, in AMLL's key/value form: keys
// such as musicName, artists, album, isrc and platform IDs. A key may repeat,
// for example once per artist.
type MetaEntry struct {
	Key, Value string
}

// Project is the lyrics being edited.
type Project struct {
	Meta []MetaEntry
	// Singers are the people who sing, in order. Empty means one unnamed
	// singer, until a second is added.
	Singers []Singer
	Lines   []Line
}

// HasSingers reports whether the lyrics tell singers apart: more than one
// was added, or a line is sung by everyone.
func (p *Project) HasSingers() bool {
	return len(p.Singers) > 1 || slices.ContainsFunc(p.Lines, func(l Line) bool { return l.Singer == Everyone })
}

// SingerName returns the name to show for singer i (or Everyone): its name,
// or "Singer 2" and so on if it has none.
func (p *Project) SingerName(i int) string {
	switch {
	case i == Everyone:
		return "Everyone"
	case i < len(p.Singers) && p.Singers[i].Name != "":
		return p.Singers[i].Name
	default:
		return fmt.Sprintf("Singer %d", i+1)
	}
}

// singerAt returns who sings line i, or the first singer if there is no line
// i: new lines take the singer of the line before them.
func (p *Project) singerAt(i int) int {
	if i < 0 || i >= len(p.Lines) {
		return 0
	}

	return p.Lines[i].Singer
}

// MetaValues returns every value for key, in order.
func (p *Project) MetaValues(key string) []string {
	var values []string
	for _, e := range p.Meta {
		if e.Key == key {
			values = append(values, e.Value)
		}
	}

	return values
}

// FillMeta returns meta with the entries of defaults whose keys meta lacks
// appended, such as tags read from the audio file.
func FillMeta(meta, defaults []MetaEntry) []MetaEntry {
	has := map[string]bool{}
	for _, e := range meta {
		has[e.Key] = true
	}
	filled := slices.Clone(meta)
	for _, e := range defaults {
		if !has[e.Key] && e.Value != "" {
			filled = append(filled, e)
		}
	}

	return filled
}

// OpenLineAt returns the line a mark at time at would end, or -1: the last
// line starting at or before at, if it has no end yet. While tapping down
// the song that is the last line tapped.
func (p *Project) OpenLineAt(at Ms) int {
	if i := p.lineAt(at); i >= 0 && p.Lines[i].End == nil {
		return i
	}

	return -1
}

// lineAt returns the last line that starts at or before at, or -1.
func (p *Project) lineAt(at Ms) int {
	for i := len(p.Lines) - 1; i >= 0; i-- {
		if begin := p.Lines[i].Begin; begin != nil && *begin <= at {
			return i
		}
	}

	return -1
}

// NextLineAt returns the line a tap at time at starts, or -1 when the tap
// should add a new line at the end: the first line with no start whose place
// fits at, at or after the start of the timed line before it, before the
// start of the timed line after it, and before its own end if it has one.
// Pasted lyrics wait after the last line tapped; a line whose start was
// cleared waits between its neighbors, for when the song is back there.
func (p *Project) NextLineAt(at Ms) int {
	for i, line := range p.Lines {
		if line.Begin != nil || line.End != nil && at >= *line.End {
			continue
		}
		if prev := p.startBefore(i); prev >= 0 && at < *p.Lines[prev].Begin {
			continue
		}
		if next := p.startAfter(i); next >= 0 && at >= *p.Lines[next].Begin {
			continue
		}

		return i
	}

	return -1
}

// startBefore returns the nearest line before i that has a start, or -1.
// Untimed lines (inserted, or imported later) are skipped by every start rule.
func (p *Project) startBefore(i int) int {
	for j := i - 1; j >= 0; j-- {
		if p.Lines[j].Begin != nil {
			return j
		}
	}

	return -1
}

// startAfter returns the nearest line after i that has a start, or -1.
func (p *Project) startAfter(i int) int {
	for j := i + 1; j < len(p.Lines); j++ {
		if p.Lines[j].Begin != nil {
			return j
		}
	}

	return -1
}

// Command is one undoable edit.
type Command interface {
	Do(p *Project)
	Undo(p *Project)
}

// History owns the project and records the commands applied to it.
type History struct {
	project Project
	done    []Command
	undone  []Command
	saved   Command // last applied command at the last save; nil before any
}

// NewHistory starts a history at p, such as a project read from a file:
// nothing to undo, and nothing unsaved.
func NewHistory(p Project) History {
	return History{project: p}
}

// Project returns the project for reading. Change it only through Apply.
func (h *History) Project() *Project {
	return &h.project
}

// MarkSaved records the current state as saved.
func (h *History) MarkSaved() {
	h.saved = h.last()
}

// Dirty reports whether the project changed since the last save. Undoing
// back to the saved state makes it clean again.
func (h *History) Dirty() bool {
	return h.last() != h.saved
}

func (h *History) last() Command {
	if len(h.done) == 0 {
		return nil
	}

	return h.done[len(h.done)-1]
}

// Apply runs c and records it for undo. It clears the redo stack.
func (h *History) Apply(c Command) {
	c.Do(&h.project)
	h.done = append(h.done, c)
	h.undone = nil
}

// Undo reverts the last applied command. It reports whether there was one.
func (h *History) Undo() bool {
	if len(h.done) == 0 {
		return false
	}
	c := h.done[len(h.done)-1]
	h.done = h.done[:len(h.done)-1]
	c.Undo(&h.project)
	h.undone = append(h.undone, c)

	return true
}

// Redo re-applies the last undone command. It reports whether there was one.
func (h *History) Redo() bool {
	if len(h.undone) == 0 {
		return false
	}
	c := h.undone[len(h.undone)-1]
	h.undone = h.undone[:len(h.undone)-1]
	c.Do(&h.project)
	h.done = append(h.done, c)

	return true
}
