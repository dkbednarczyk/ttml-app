package core

import (
	"errors"
	"slices"
	"testing"
)

func TestAddAndRenameSingers(t *testing.T) {
	var h History
	apply(t, &h, tapAt(&h, 1000))
	apply(t, &h, tapAt(&h, 2000))
	if h.Project().HasSingers() {
		t.Fatal("HasSingers() with no singer added")
	}

	// The first singer added is the second: the lyrics already had one.
	apply(t, &h, func() (Command, error) { return NewAddSinger(h.Project(), "Scream", 1) })
	p := h.Project()
	if want := []Singer{{}, {Name: "Scream"}}; !slices.Equal(p.Singers, want) {
		t.Fatalf("Singers = %v, want %v", p.Singers, want)
	}
	if !p.HasSingers() || p.Lines[0].Singer != 0 || p.Lines[1].Singer != 1 {
		t.Fatalf("after adding a singer for line 2: HasSingers %v, line singers %d, %d",
			p.HasSingers(), p.Lines[0].Singer, p.Lines[1].Singer)
	}
	if got := []string{p.SingerName(0), p.SingerName(1), p.SingerName(Everyone)}; !slices.Equal(got,
		[]string{"Singer 1", "Scream", "Everyone"}) {
		t.Errorf("singer names = %q", got)
	}

	apply(t, &h, func() (Command, error) { return NewRenameSinger(h.Project(), 0, "Clean") })
	if got := h.Project().SingerName(0); got != "Clean" {
		t.Errorf("SingerName(0) after rename = %q, want Clean", got)
	}

	// One undo takes back both the singer and the line given to it.
	h.Undo()
	h.Undo()
	if p := h.Project(); p.Singers != nil || p.Lines[1].Singer != 0 || p.HasSingers() {
		t.Fatalf("after undoing: Singers %v, line 2 singer %d", p.Singers, p.Lines[1].Singer)
	}

	if _, err := NewAddSinger(h.Project(), "", 2); !errors.Is(err, errNoSuchLine) {
		t.Errorf("NewAddSinger for line 3 of 2: error = %v, want %v", err, errNoSuchLine)
	}
	if _, err := NewRenameSinger(h.Project(), 1, "x"); !errors.Is(err, errNoSuchSinger) {
		t.Errorf("NewRenameSinger(1) with one singer: error = %v, want %v", err, errNoSuchSinger)
	}
}

func TestSetSinger(t *testing.T) {
	var h History
	apply(t, &h, tapAt(&h, 1000))
	apply(t, &h, func() (Command, error) { return NewAddSinger(h.Project(), "", -1) })

	apply(t, &h, func() (Command, error) { return NewSetSinger(h.Project(), 0, Everyone) })
	if got := h.Project().Lines[0].Singer; got != Everyone {
		t.Fatalf("line 1 singer = %d, want Everyone", got)
	}
	h.Undo()
	if got := h.Project().Lines[0].Singer; got != 0 {
		t.Fatalf("line 1 singer after undo = %d, want 0", got)
	}

	for _, singer := range []int{-2, 2} {
		if _, err := NewSetSinger(h.Project(), 0, singer); !errors.Is(err, errNoSuchSinger) {
			t.Errorf("NewSetSinger(%d) with two singers: error = %v, want %v", singer, err, errNoSuchSinger)
		}
	}
	// A line sung by everyone counts as telling singers apart, even with one singer.
	p := Project{Lines: []Line{{Singer: Everyone}}}
	if !p.HasSingers() {
		t.Error("HasSingers() = false with a line sung by everyone")
	}
}

func TestNewLinesTakeThePreviousSinger(t *testing.T) {
	var h History
	apply(t, &h, tapAt(&h, 1000))
	apply(t, &h, func() (Command, error) { return NewAddSinger(h.Project(), "", 0) })

	apply(t, &h, tapAt(&h, 2000)) // adds line 2
	apply(t, &h, func() (Command, error) { return NewInsertLine(h.Project(), 2) })
	apply(t, &h, func() (Command, error) {
		return NewInsertLines(h.Project(), 0, []Line{{Text: "pasted above"}})
	})

	got := make([]int, 0, 4)
	for _, l := range h.Project().Lines {
		got = append(got, l.Singer)
	}
	// The pasted line at the top has no line before it: the first singer.
	if want := []int{0, 1, 1, 1}; !slices.Equal(got, want) {
		t.Errorf("line singers = %v, want %v", got, want)
	}
}

func TestRemoveSinger(t *testing.T) {
	end := Ms(1000)
	timed := func(singer int) Line { return Line{Timing: Timing{Begin: new(Ms(0)), End: &end}, Singer: singer} }
	h := NewHistory(Project{
		Singers: []Singer{{Name: "Clean"}, {Name: "Scream"}, {Name: "Guest"}},
		Lines:   []Line{timed(0), timed(1), timed(2), timed(Everyone)},
	})

	apply(t, &h, func() (Command, error) { return NewRemoveSinger(h.Project(), 1) })
	p := h.Project()
	if want := []Singer{{Name: "Clean"}, {Name: "Guest"}}; !slices.Equal(p.Singers, want) {
		t.Fatalf("Singers = %v, want %v", p.Singers, want)
	}
	got := []int{p.Lines[0].Singer, p.Lines[1].Singer, p.Lines[2].Singer, p.Lines[3].Singer}
	// Scream's line goes to Clean; Guest moves up to singer 1.
	if want := []int{0, 0, 1, Everyone}; !slices.Equal(got, want) {
		t.Errorf("line singers = %v, want %v", got, want)
	}

	h.Undo()
	p = h.Project()
	got = []int{p.Lines[0].Singer, p.Lines[1].Singer, p.Lines[2].Singer, p.Lines[3].Singer}
	if len(p.Singers) != 3 || !slices.Equal(got, []int{0, 1, 2, Everyone}) {
		t.Fatalf("after undo: singers %v, line singers %v", p.Singers, got)
	}

	// Removing the first singer gives its lines to the next one.
	apply(t, &h, func() (Command, error) { return NewRemoveSinger(h.Project(), 0) })
	if p := h.Project(); p.Singers[0].Name != "Scream" || p.Lines[0].Singer != 0 || p.Lines[2].Singer != 1 {
		t.Errorf("after removing the first singer: singers %v, line singers %d, %d", p.Singers, p.Lines[0].Singer,
			p.Lines[2].Singer)
	}

	one := Project{Singers: []Singer{{Name: "Solo"}}}
	if _, err := NewRemoveSinger(&one, 0); !errors.Is(err, errNoSuchSinger) {
		t.Errorf("removing the only singer: error = %v, want %v", err, errNoSuchSinger)
	}
}
