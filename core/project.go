// Package core holds the lyrics project and the commands that edit it.
// The UI changes the project only through commands, so every edit can be undone.
package core

import "slices"

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
}

// MetaEntry is one piece of song metadata, in AMLL's key/value form: keys
// such as musicName, artists, album, isrc and platform IDs. A key may repeat,
// for example once per artist.
type MetaEntry struct {
	Key, Value string
}

// Project is the lyrics being edited.
type Project struct {
	Meta  []MetaEntry
	Lines []Line
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

// OpenLine returns the index of the line being tapped, or -1: the last line
// with a start, if it has no end yet. Untimed lines after it don't count.
func (p *Project) OpenLine() int {
	if i := p.startBefore(len(p.Lines)); i >= 0 && p.Lines[i].End == nil {
		return i
	}

	return -1
}

// NextLine returns the line the next tap starts: the first line with no
// times after the last line with a start (pasted lyrics waiting to be
// tapped), or -1 when a tap should add a new line at the end.
func (p *Project) NextLine() int {
	for i := p.startBefore(len(p.Lines)) + 1; i < len(p.Lines); i++ {
		if p.Lines[i].Begin == nil && p.Lines[i].End == nil {
			return i
		}
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
