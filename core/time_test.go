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
	if h.Project().OpenLine() != -1 {
		t.Fatal("line is still open after setting its end")
	}
	h.Undo()
	if h.Project().OpenLine() != 1 {
		t.Fatal("undo did not reopen the line")
	}
}
