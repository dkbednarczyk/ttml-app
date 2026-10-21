package core

import (
	"errors"
	"testing"
)

func TestInsertLine(t *testing.T) {
	var h History
	apply(t, &h, tapAt(&h, 1000))
	apply(t, &h, tapAt(&h, 2000))
	apply(t, &h, endAt(&h, 3000))

	apply(t, &h, func() (Command, error) { return NewInsertLine(h.Project(), 1) })
	lines := h.Project().Lines
	if len(lines) != 3 || lines[1].Begin != nil || lines[1].End != nil || lines[1].Text != "" {
		t.Fatalf("after inserting at 1: %d lines, line 2 = %+v; want an empty untimed line 2 of 3", len(lines), lines[1])
	}
	if *lines[0].End != 2000 || *lines[2].Begin != 2000 {
		t.Fatal("inserting a line moved the neighbors' times")
	}

	h.Undo()
	if len(h.Project().Lines) != 2 {
		t.Fatalf("undo left %d lines, want 2", len(h.Project().Lines))
	}
	for _, i := range []int{-1, 3} {
		if _, err := NewInsertLine(h.Project(), i); err == nil {
			t.Errorf("NewInsertLine(%d) accepted an index outside 0..2", i)
		}
	}
}

func TestUntimedLinesAreSkippedByStartRules(t *testing.T) {
	// Line 1 at 1–2 s, an untimed line 2, line 3 at 3–4 s.
	var h History
	apply(t, &h, tapAt(&h, 1000))
	apply(t, &h, endAt(&h, 2000))
	apply(t, &h, tapAt(&h, 3000))
	apply(t, &h, endAt(&h, 4000))
	apply(t, &h, func() (Command, error) { return NewInsertLine(h.Project(), 1) })

	tests := []struct {
		name string
		edge Edge
		at   Ms
		want error
	}{
		{"start between the timed neighbors", Begin, 2000, nil},
		{"start before the previous timed start", Begin, 999, ErrStartOrder},
		{"start after the next timed start", Begin, 3001, ErrStartOrder},
		{"end with no start yet", End, 500, nil},
	}
	for _, tt := range tests {
		if _, err := NewSetTime(h.Project(), 1, tt.edge, tt.at, 0); !errors.Is(err, tt.want) {
			t.Errorf("%s: error = %v, want %v", tt.name, err, tt.want)
		}
	}

	// Line 3 checks its start against line 1, past the untimed line.
	if _, err := NewSetTime(h.Project(), 2, Begin, 999, 0); !errors.Is(err, ErrStartOrder) {
		t.Errorf("line 3 start before line 1's: error = %v, want %v", err, ErrStartOrder)
	}

	// With an end but no start, a start must come before that end.
	c, err := NewSetTime(h.Project(), 1, End, 2500, 0)
	if err != nil {
		t.Fatalf("NewSetTime(end) error: %v", err)
	}
	h.Apply(c)
	if _, err := NewSetTime(h.Project(), 1, Begin, 2600, 0); !errors.Is(err, ErrStartNotBeforeEnd) {
		t.Errorf("start after the typed end: error = %v, want %v", err, ErrStartNotBeforeEnd)
	}
}

func TestTapAfterUntimedLines(t *testing.T) {
	var h History
	apply(t, &h, tapAt(&h, 1000))
	// An untimed line below the open line doesn't stop S from ending it.
	apply(t, &h, func() (Command, error) { return NewInsertLine(h.Project(), 1) })
	if got := h.Project().OpenLine(); got != 0 {
		t.Fatalf("OpenLine() = %d with an untimed line after the open one, want 0", got)
	}

	// The untimed line is waiting to be tapped: the tap starts it.
	apply(t, &h, tapAt(&h, 2000))
	lines := h.Project().Lines
	if len(lines) != 2 || *lines[0].End != 2000 || lines[1].Begin == nil || *lines[1].Begin != 2000 {
		t.Fatalf("tap did not end line 1 and start the waiting line: %+v", lines)
	}
	h.Undo()
	if h.Project().Lines[0].End != nil || h.Project().Lines[1].Begin != nil || len(h.Project().Lines) != 2 {
		t.Fatal("undoing the tap did not reopen line 1 and clear the waiting line")
	}

	if _, err := NewTap(h.Project(), 500); !errors.Is(err, ErrBeforePrevious) {
		t.Fatalf("tap before line 1's start: error = %v, want %v", err, ErrBeforePrevious)
	}
}

func TestTapThroughPastedLines(t *testing.T) {
	var h History
	pasted := []Line{{Text: "one"}, {Text: "two"}, {Text: "three"}}
	apply(t, &h, func() (Command, error) { return NewInsertLines(h.Project(), 0, pasted) })
	if got := h.Project().NextLine(); got != 0 {
		t.Fatalf("NextLine() = %d with only pasted lines, want 0", got)
	}

	apply(t, &h, tapAt(&h, 1000))
	apply(t, &h, tapAt(&h, 2000))
	apply(t, &h, endAt(&h, 2500))
	apply(t, &h, tapAt(&h, 4000)) // after a break, the next tap starts line 3
	apply(t, &h, tapAt(&h, 5000)) // nothing left waiting: a new line at the end

	lines := h.Project().Lines
	want := []struct {
		text       string
		begin, end Ms // -1 for unset
	}{{"one", 1000, 2000}, {"two", 2000, 2500}, {"three", 4000, 5000}, {"", 5000, -1}}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(lines), len(want))
	}
	for i, w := range want {
		end := Ms(-1)
		if lines[i].End != nil {
			end = *lines[i].End
		}
		if lines[i].Text != w.text || *lines[i].Begin != w.begin || end != w.end {
			t.Errorf("line %d = %q %d–%d, want %q %d–%d", i+1, lines[i].Text, *lines[i].Begin, end, w.text, w.begin, w.end)
		}
	}

	// One undo per tap, back to the paste; one more removes the paste.
	for range 5 {
		h.Undo()
	}
	if got := h.Project(); len(got.Lines) != 3 || got.Lines[0].Begin != nil || got.NextLine() != 0 {
		t.Fatalf("after undoing the taps: %+v", got.Lines)
	}
	h.Undo()
	if len(h.Project().Lines) != 0 {
		t.Fatal("one undo did not remove the whole paste")
	}
}

func TestNextLineSkipsPartlyTimedAndEarlierLines(t *testing.T) {
	end := Ms(9000)
	p := Project{Lines: []Line{
		{Text: "untimed, but before the last started line"},
		{Text: "started", Timing: Timing{Begin: new(Ms(1000))}},
		{Text: "end only", Timing: Timing{End: &end}},
		{Text: "waiting"},
	}}
	if got := p.NextLine(); got != 3 {
		t.Fatalf("NextLine() = %d, want 3", got)
	}
}
