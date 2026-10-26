package core

import (
	"errors"
	"testing"
)

func TestSetBackground(t *testing.T) {
	var h History
	apply(t, &h, tapAt(&h, 1000))
	apply(t, &h, endAt(&h, 2000))

	apply(t, &h, func() (Command, error) { return NewSetBackground(h.Project(), 0, "(ooh)") })
	apply(t, &h, func() (Command, error) { return NewSetBackgroundTime(h.Project(), 0, Begin, new(Ms(1500)), 0) })
	line := h.Project().Lines[0]
	if t2 := line.BackgroundTimes(); line.Background != "(ooh)" || *t2.Begin != 1500 || *t2.End != 2000 {
		t.Fatalf("background = %q %d–%d, want (ooh) 1500–2000 (the line's end)", line.Background, *t2.Begin, *t2.End)
	}

	// Emptying the text removes the background vocal's times too; undo brings both back.
	apply(t, &h, func() (Command, error) { return NewSetBackground(h.Project(), 0, "") })
	if line := h.Project().Lines[0]; line.Background != "" || line.BackgroundTiming.Begin != nil {
		t.Fatalf("after clearing: %q %+v, want no background vocal", line.Background, line.BackgroundTiming)
	}
	h.Undo()
	if line := h.Project().Lines[0]; line.Background != "(ooh)" || line.BackgroundTiming.Begin == nil {
		t.Fatal("undo did not bring back the background vocal and its start")
	}

	// Back to the line's start.
	apply(t, &h, func() (Command, error) { return NewSetBackgroundTime(h.Project(), 0, Begin, nil, 0) })
	if line := h.Project().Lines[0]; line.BackgroundTiming.Begin != nil {
		t.Fatalf("background start = %d, want the line's", *line.BackgroundTiming.Begin)
	}
}

func TestSetBackgroundTimeRules(t *testing.T) {
	var h History
	apply(t, &h, tapAt(&h, 1000))
	apply(t, &h, endAt(&h, 2000))
	apply(t, &h, func() (Command, error) { return NewSetBackground(h.Project(), 0, "(ooh)") })

	tests := []struct {
		name string
		edge Edge
		at   Ms
		want error
	}{
		{"start late, inside the line", Begin, 1800, nil},
		{"end after the line ends", End, 2600, nil},
		{"start before the line", Begin, 500, nil},
		{"start at the line's end", Begin, 2000, ErrBackgroundStartNotBeforeEnd},
		{"end at the line's start", End, 1000, ErrBackgroundEndNotAfterStart},
		{"past the song", End, 9000, ErrPastSongEnd},
	}
	for _, tt := range tests {
		if _, err := NewSetBackgroundTime(h.Project(), 0, tt.edge, new(tt.at), 5000); !errors.Is(err, tt.want) {
			t.Errorf("%s: error = %v, want %v", tt.name, err, tt.want)
		}
	}
}
