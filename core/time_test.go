package core

import (
	"errors"
	"testing"
)

func TestParseMs(t *testing.T) {
	valid := map[string]Ms{
		"0:00.000":    0,
		"1:05.250":    65_250,
		"05:00.000":   300_000,
		"59:59.999":   3_599_999,
		"1:00:00.000": 3_600_000,
		"1:02:03.004": 3_723_004,
	}
	for text, want := range valid {
		got, err := ParseMs(text)
		if err != nil || got != want {
			t.Errorf("ParseMs(%q) = %d, %v; want %d", text, got, err, want)
		}
		if text[0] != '0' && got.String() != text {
			t.Errorf("%d.String() = %q, want %q", got, got.String(), text)
		}
	}

	invalid := []string{
		"", "-:--.---", "1:05", "1:05.25", "1:05.2500", "1:5.250", "1:65.000",
		"60:00.000", "0:01:00.000", "1:60:00.000", "-0:01.000", " 1:05.250", "75.5s", "1:05,250",
	}
	for _, text := range invalid {
		if _, err := ParseMs(text); !errors.Is(err, ErrBadTime) {
			t.Errorf("ParseMs(%q) error = %v, want %v", text, err, ErrBadTime)
		}
	}
}

func TestSetTime(t *testing.T) {
	// Lines 1–2, 2–3 (open end later set), 3–4; times in seconds.
	var h History
	for _, at := range []Ms{1000, 2000, 3000} {
		apply(t, &h, tapAt(&h, at))
	}
	apply(t, &h, endAt(&h, 4000))
	const songEnd = 10_000

	tests := []struct {
		name string
		line int
		edge Edge
		at   Ms
		want error
	}{
		{"end runs past the next start", 0, End, 2500, nil},
		{"start equal to the previous start", 1, Begin, 1000, nil},
		{"start just before its end", 1, Begin, 2999, nil},
		{"end at the song end", 2, End, songEnd, nil},
		{"end before its start", 1, End, 1500, ErrEndNotAfterStart},
		{"end at its start", 1, End, 2000, ErrEndNotAfterStart},
		{"start at its end", 0, Begin, 2000, ErrStartNotBeforeEnd},
		{"start before the previous start", 1, Begin, 999, ErrStartOrder},
		{"start after the next start", 1, Begin, 3001, ErrStartNotBeforeEnd},
		{"start after the next start, end later", 0, Begin, 1500, nil},
		{"past the song end", 2, End, songEnd + 1, ErrPastSongEnd},
	}
	for _, tt := range tests {
		_, err := NewSetTime(h.Project(), tt.line, tt.edge, tt.at, songEnd)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s: error = %v, want %v", tt.name, err, tt.want)
		}
	}
}

func TestSetTimeOrderAgainstNextLine(t *testing.T) {
	var h History
	apply(t, &h, tapAt(&h, 1000))
	apply(t, &h, endAt(&h, 5000)) // line 1 runs long, so its own end doesn't block
	apply(t, &h, tapAt(&h, 2000))

	if _, err := NewSetTime(h.Project(), 0, Begin, 2000, 0); err != nil {
		t.Fatalf("start equal to the next line's start: error = %v, want nil", err)
	}
	if _, err := NewSetTime(h.Project(), 0, Begin, 2001, 0); !errors.Is(err, ErrStartOrder) {
		t.Fatalf("start after the next line's start: error = %v, want %v", err, ErrStartOrder)
	}
}

func TestSetTimeChangesOnlyThatBoundary(t *testing.T) {
	var h History
	apply(t, &h, tapAt(&h, 1000))
	apply(t, &h, tapAt(&h, 2000))

	c, err := NewSetTime(h.Project(), 0, End, 2400, 0)
	if err != nil {
		t.Fatalf("NewSetTime() error: %v", err)
	}
	h.Apply(c)
	lines := h.Project().Lines
	if *lines[0].End != 2400 || *lines[1].Begin != 2000 {
		t.Fatalf("after editing line 1's end: line 1 ends %d, line 2 starts %d; want 2400 and 2000",
			*lines[0].End, *lines[1].Begin)
	}

	// Setting the open line's end closes it; undo reopens it.
	c, err = NewSetTime(h.Project(), 1, End, 3000, 0)
	if err != nil {
		t.Fatalf("NewSetTime() on the open end error: %v", err)
	}
	h.Apply(c)
	if h.Project().OpenLineAt(3500) != -1 {
		t.Fatal("line is still open after setting its end")
	}
	h.Undo()
	if h.Project().OpenLineAt(3500) != 1 {
		t.Fatal("undo did not reopen the line")
	}
}

func TestClearTime(t *testing.T) {
	var h History
	apply(t, &h, tapAt(&h, 1000))
	apply(t, &h, tapAt(&h, 2000))
	apply(t, &h, endAt(&h, 3000))

	// Line 1's end and line 2's start are one tap, so both are cleared.
	apply(t, &h, func() (Command, error) { return NewClearTime(h.Project(), 0, End) })
	p := h.Project()
	if p.Lines[0].End != nil || p.Lines[1].Begin != nil || *p.Lines[0].Begin != 1000 || *p.Lines[1].End != 3000 {
		t.Fatalf("after clearing line 1's end: %+v; want it and line 2's start unset, the rest kept", p.Lines)
	}
	if p.OpenLineAt(1500) != 0 || p.NextLineAt(1500) != 1 {
		t.Fatalf("open line %d, next line %d at 1.5 s; want line 1 open and line 2 next",
			p.OpenLineAt(1500), p.NextLineAt(1500))
	}
	apply(t, &h, tapAt(&h, 1800))
	if p := h.Project(); *p.Lines[0].End != 1800 || *p.Lines[1].Begin != 1800 {
		t.Fatalf("retap: line 1 ends %v, line 2 starts %v; want 1800 for both", p.Lines[0].End, p.Lines[1].Begin)
	}
	h.Undo()
	h.Undo()
	if p := h.Project(); *p.Lines[0].End != 2000 || *p.Lines[1].Begin != 2000 {
		t.Fatalf("undo: line 1 ends %v, line 2 starts %v; want 2000 for both", p.Lines[0].End, p.Lines[1].Begin)
	}

	// Clearing the last line's end opens it again for the next tap.
	apply(t, &h, func() (Command, error) { return NewClearTime(h.Project(), 1, End) })
	if got := h.Project().OpenLineAt(3500); got != 1 {
		t.Fatalf("OpenLineAt(3500) after clearing the last end = %d, want 1", got)
	}

	if _, err := NewClearTime(h.Project(), 2, Begin); !errors.Is(err, errNoSuchLine) {
		t.Errorf("NewClearTime past the last line: error = %v, want %v", err, errNoSuchLine)
	}
}

func TestClearStartKeepsAnEndedBreak(t *testing.T) {
	// Line 1 was ended with E before a break, so its end isn't line 2's start.
	var h History
	apply(t, &h, tapAt(&h, 1000))
	apply(t, &h, endAt(&h, 1500))
	apply(t, &h, tapAt(&h, 3000))

	apply(t, &h, func() (Command, error) { return NewClearTime(h.Project(), 1, Begin) })
	if end := h.Project().Lines[0].End; end == nil || *end != 1500 {
		t.Fatalf("line 1 end = %v, want it kept at 1500", end)
	}
}

func TestRetapOneLineInTheMiddle(t *testing.T) {
	// Four lines tapped at 1, 2, 3 and 4 s, ended at 5 s; line 2 went wrong.
	var h History
	for _, at := range []Ms{1000, 2000, 3000, 4000} {
		apply(t, &h, tapAt(&h, at))
	}
	apply(t, &h, endAt(&h, 5000))
	apply(t, &h, func() (Command, error) { return NewClearTime(h.Project(), 1, Begin) })
	apply(t, &h, func() (Command, error) { return NewClearTime(h.Project(), 1, End) })

	// Back in line 2's place, S starts it again and then line 3.
	apply(t, &h, tapAt(&h, 2200))
	apply(t, &h, tapAt(&h, 3300))
	want := []struct{ begin, end Ms }{{1000, 2200}, {2200, 3300}, {3300, 4000}, {4000, 5000}}
	for i, w := range want {
		if line := h.Project().Lines[i]; line.Begin == nil || line.End == nil || *line.Begin != w.begin || *line.End != w.end {
			t.Errorf("line %d = %v–%v, want %d–%d", i+1, line.Begin, line.End, w.begin, w.end)
		}
	}

	// With nothing left to re-tap there, a tap mid-song does nothing.
	if _, err := NewTap(h.Project(), 3500); !errors.Is(err, ErrBeforePrevious) {
		t.Errorf("tap with no line waiting at 3.5 s: error = %v, want %v", err, ErrBeforePrevious)
	}
}

func TestClearTimesFromRetaps(t *testing.T) {
	// Lines at 1–2, 2–3 and 3–4 s; the third has a background vocal running late.
	var h History
	for _, at := range []Ms{1000, 2000, 3000} {
		apply(t, &h, tapAt(&h, at))
	}
	apply(t, &h, endAt(&h, 4000))
	apply(t, &h, func() (Command, error) { return NewSetBackground(h.Project(), 2, "ooh") })
	apply(t, &h, func() (Command, error) { return NewSetBackgroundTime(h.Project(), 2, End, new(Ms(4500)), 0) })

	apply(t, &h, func() (Command, error) { return NewClearTimesFrom(h.Project(), 1) })
	p := h.Project()
	for i := 1; i < 3; i++ {
		if line := p.Lines[i]; line.Timing != (Timing{}) || line.BackgroundTiming != (Timing{}) {
			t.Errorf("line %d after clearing: timing %v, background %v; want both unset", i+1, line.Timing, line.BackgroundTiming)
		}
	}
	// Line 1 ended with the tap that started line 2, so it opens again.
	if p.Lines[0].End != nil || p.OpenLineAt(2500) != 0 || p.NextLineAt(2500) != 1 {
		t.Fatalf("line 1 end %v, open line %d, next line %d; want line 1 open and line 2 next",
			p.Lines[0].End, p.OpenLineAt(2500), p.NextLineAt(2500))
	}

	apply(t, &h, tapAt(&h, 2100))
	if p := h.Project(); *p.Lines[0].End != 2100 || *p.Lines[1].Begin != 2100 {
		t.Fatalf("retap: line 1 ends %v, line 2 starts %v; want 2100 for both", p.Lines[0].End, p.Lines[1].Begin)
	}

	h.Undo()
	h.Undo()
	p = h.Project()
	if *p.Lines[0].End != 2000 || *p.Lines[1].Begin != 2000 || *p.Lines[2].End != 4000 || *p.Lines[2].BackgroundTiming.End != 4500 {
		t.Fatalf("undo: lines %v; want the times from before clearing", p.Lines)
	}
}

func TestClearTimesFromKeepsAnEndedBreak(t *testing.T) {
	// Line 1 was ended with E before a break, so its end isn't line 2's start.
	var h History
	apply(t, &h, tapAt(&h, 1000))
	apply(t, &h, endAt(&h, 1500))
	apply(t, &h, tapAt(&h, 3000))

	apply(t, &h, func() (Command, error) { return NewClearTimesFrom(h.Project(), 1) })
	p := h.Project()
	if end := p.Lines[0].End; end == nil || *end != 1500 {
		t.Fatalf("line 1 end = %v, want it kept at 1500", end)
	}
	if p.OpenLineAt(2000) != -1 || p.NextLineAt(2000) != 1 {
		t.Fatalf("open line %d, next line %d; want none open and line 2 next", p.OpenLineAt(2000), p.NextLineAt(2000))
	}
}
