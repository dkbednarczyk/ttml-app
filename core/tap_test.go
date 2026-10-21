package core

import (
	"errors"
	"slices"
	"testing"
)

func TestTapsMarkLineBoundaries(t *testing.T) {
	var h History
	apply(t, &h, tapAt(&h, 1000))
	apply(t, &h, tapAt(&h, 2000))
	apply(t, &h, endAt(&h, 2500))
	// After an end, the next tap starts a new line instead of ending one.
	apply(t, &h, tapAt(&h, 4000))

	want := []struct{ begin, end Ms }{{1000, 2000}, {2000, 2500}, {4000, -1}}
	lines := h.Project().Lines
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(lines), len(want))
	}
	for i, w := range want {
		if got := lines[i].Begin; got == nil || *got != w.begin {
			t.Errorf("line %d begin = %v, want %d", i, got, w.begin)
		}
		if w.end < 0 {
			if lines[i].End != nil {
				t.Errorf("line %d end = %d, want unset", i, *lines[i].End)
			}
		} else if got := lines[i].End; got == nil || *got != w.end {
			t.Errorf("line %d end = %v, want %d", i, got, w.end)
		}
	}
	if got := h.Project().OpenLine(); got != 2 {
		t.Errorf("OpenLine() = %d, want 2", got)
	}
}

func TestTapRejectsTimesBeforePreviousBoundary(t *testing.T) {
	var h History
	apply(t, &h, tapAt(&h, 1000))

	if _, err := NewTap(h.Project(), 1000); !errors.Is(err, ErrBeforePrevious) {
		t.Errorf("NewTap at the open line's start: error = %v, want %v", err, ErrBeforePrevious)
	}
	if _, err := NewEndLine(h.Project(), 500); !errors.Is(err, ErrBeforePrevious) {
		t.Errorf("NewEndLine before the start: error = %v, want %v", err, ErrBeforePrevious)
	}

	apply(t, &h, endAt(&h, 2000))
	if _, err := NewTap(h.Project(), 500); !errors.Is(err, ErrBeforePrevious) {
		t.Errorf("NewTap before the last line's start: error = %v, want %v", err, ErrBeforePrevious)
	}
	// Starting before the last line ends is an overlap, which is allowed.
	if _, err := NewTap(h.Project(), 1500); err != nil {
		t.Errorf("NewTap overlapping the last line: error = %v, want nil", err)
	}
	if _, err := NewEndLine(h.Project(), 3000); !errors.Is(err, ErrNoOpenLine) {
		t.Errorf("NewEndLine with no open line: error = %v, want %v", err, ErrNoOpenLine)
	}
}

func TestUndoRedo(t *testing.T) {
	var h History
	apply(t, &h, tapAt(&h, 1000))
	apply(t, &h, tapAt(&h, 2000))
	text, err := NewSetText(h.Project(), 0, "hello")
	if err != nil {
		t.Fatalf("NewSetText() error: %v", err)
	}
	h.Apply(text)

	if !h.Undo() || h.Project().Lines[0].Text != "" {
		t.Fatalf("undoing the text edit left %q", h.Project().Lines[0].Text)
	}
	if !h.Undo() {
		t.Fatal("Undo() = false with a tap to undo")
	}
	lines := h.Project().Lines
	if len(lines) != 1 || lines[0].End != nil {
		t.Fatalf("after undoing the second tap: %d lines, first end %v; want 1 open line", len(lines), lines[0].End)
	}

	for range 2 {
		if !h.Redo() {
			t.Fatal("Redo() = false with commands to redo")
		}
	}
	if got := h.Project().Lines[0]; got.Text != "hello" || got.End == nil || *got.End != 2000 {
		t.Fatalf("after redo, line 0 = %q ending %v; want %q ending 2000", got.Text, got.End, "hello")
	}
	if h.Redo() {
		t.Fatal("Redo() = true with nothing left to redo")
	}
}

func TestDeleteLineKeepsOtherTimings(t *testing.T) {
	var h History
	apply(t, &h, tapAt(&h, 1000))
	apply(t, &h, tapAt(&h, 2000))
	apply(t, &h, tapAt(&h, 3000))
	apply(t, &h, endAt(&h, 4000))
	before := slices.Clone(h.Project().Lines)

	apply(t, &h, func() (Command, error) { return NewDeleteLine(h.Project(), 1) })
	got := h.Project().Lines
	if len(got) != 2 {
		t.Fatalf("got %d lines after deleting one of 3, want 2", len(got))
	}
	for i, want := range []Line{before[0], before[2]} {
		if *got[i].Begin != *want.Begin || *got[i].End != *want.End {
			t.Errorf("line %d = %d–%d, want %d–%d", i, *got[i].Begin, *got[i].End, *want.Begin, *want.End)
		}
	}

	h.Undo()
	if got := h.Project().Lines; len(got) != 3 || *got[1].Begin != 2000 || *got[1].End != 3000 {
		t.Fatalf("undo did not restore the deleted line in place: %d lines", len(got))
	}
	if _, err := NewDeleteLine(h.Project(), 3); err == nil {
		t.Fatal("NewDeleteLine accepted an index past the end")
	}
}

func TestDeletingOpenLineClosesNothing(t *testing.T) {
	var h History
	apply(t, &h, tapAt(&h, 1000))
	apply(t, &h, tapAt(&h, 2000))

	apply(t, &h, func() (Command, error) { return NewDeleteLine(h.Project(), 1) })
	if got := h.Project().OpenLine(); got != -1 {
		t.Fatalf("OpenLine() = %d after deleting the open line, want -1", got)
	}
	if end := h.Project().Lines[0].End; end == nil || *end != 2000 {
		t.Fatalf("first line end = %v after deleting the next line, want 2000", end)
	}
}

func TestNewHistory(t *testing.T) {
	h := NewHistory(Project{Lines: []Line{{Text: "imported"}}})
	if h.Dirty() || h.Undo() || len(h.Project().Lines) != 1 {
		t.Fatal("a new history from a project should be clean, with nothing to undo")
	}
}

func TestDirty(t *testing.T) {
	var h History
	if h.Dirty() {
		t.Fatal("new history is dirty")
	}
	apply(t, &h, tapAt(&h, 1000))
	if !h.Dirty() {
		t.Fatal("history is clean after a tap")
	}
	h.MarkSaved()
	if h.Dirty() {
		t.Fatal("history is dirty right after saving")
	}
	apply(t, &h, tapAt(&h, 2000))
	h.Undo()
	if h.Dirty() {
		t.Fatal("history is dirty after undoing back to the saved state")
	}
	h.Undo()
	if !h.Dirty() {
		t.Fatal("history is clean after undoing past the saved state")
	}
}

func tapAt(h *History, at Ms) func() (Command, error) {
	return func() (Command, error) { return NewTap(h.Project(), at) }
}

func endAt(h *History, at Ms) func() (Command, error) {
	return func() (Command, error) { return NewEndLine(h.Project(), at) }
}

func apply(t *testing.T, h *History, newCommand func() (Command, error)) {
	t.Helper()

	c, err := newCommand()
	if err != nil {
		t.Fatalf("building command: %v", err)
	}
	h.Apply(c)
}
