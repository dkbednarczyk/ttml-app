package ui

import (
	"slices"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// previewTTML has a break, an overlap (line 3 runs past line 4's start), a
// gap after line 4 and enough lines to scroll.
const previewTTML = `<tt xmlns="http://www.w3.org/ns/ttml"><body><div>` +
	`<p begin="00:00:00.000" end="00:00:10.000"></p>` +
	`<p begin="00:00:10.000" end="00:00:20.000">two</p>` +
	`<p begin="00:00:20.000" end="00:00:33.000">three, held</p>` +
	`<p begin="00:00:30.000" end="00:00:40.000">four</p>` +
	`<p begin="00:00:50.000" end="00:01:00.000">five</p>` +
	`<p begin="00:01:00.000" end="00:01:10.000">six</p>` +
	`<p begin="00:01:10.000" end="00:01:20.000">seven</p>` +
	`<p begin="00:01:20.000" end="00:01:30.000">eight</p></div></body></tt>`

func newPreviewTestWindow(t *testing.T) *mainWindow {
	t.Helper()

	mw := newTestMainWindow(t)
	mw.window.Resize(fyne.NewSize(windowW, windowH))
	if err := mw.importTTML(writeTTML(t, previewTTML)); err != nil {
		t.Fatal(err)
	}
	if err := mw.loadAudio(testAudioPath); err != nil {
		t.Fatal(err)
	}
	mw.tabs.SelectIndex(1)

	return mw
}

// litLines returns the 1-based numbers of the lines shown in full colour.
func litLines(mw *mainWindow) []int {
	var lit []int
	for i, o := range mw.preview.lines.Objects {
		if o.(*previewLine).Importance != widget.LowImportance {
			lit = append(lit, i+1)
		}
	}

	return lit
}

func TestPreview(t *testing.T) {
	mw := newPreviewTestWindow(t)
	labels := mw.preview.lines.Objects
	if got := labels[0].(*previewLine).Text; got != breakText {
		t.Fatalf("break line shows %q, want %q", got, breakText)
	}

	tests := []struct {
		name string
		at   time.Duration
		lit  []int
		top  int // 1-based line at the top of the view
	}{
		{"intro break", 5 * time.Second, []int{1}, 1},
		{"a line", 15 * time.Second, []int{2}, 2},
		{"overlap keeps the held line at the top", 31 * time.Second, []int{3, 4}, 3},
		{"held line ended", 35 * time.Second, []int{4}, 4},
		{"gap holds the view", 45 * time.Second, nil, 4},
		{"later line", 75 * time.Second, []int{7}, 7},
		{"seek back", 15 * time.Second, []int{2}, 2},
	}
	for _, tt := range tests {
		mw.setUIPosition(tt.at)
		if got := litLines(mw); !slices.Equal(got, tt.lit) {
			t.Errorf("%s: lit lines %v, want %v", tt.name, got, tt.lit)
		}
		// The test driver finishes animations at once, so the view is at its target.
		wantY := labels[tt.top-1].Position().Y
		maxY := mw.preview.scroll.Content.MinSize().Height - mw.preview.scroll.Size().Height
		if got := mw.preview.scroll.Offset.Y; got != min(wantY, maxY) {
			t.Errorf("%s: scrolled to %v, want line %d at the top (%v)", tt.name, got, tt.top, wantY)
		}
	}
}

func TestPreviewSizesDoNotChange(t *testing.T) {
	mw := newPreviewTestWindow(t)
	mw.setUIPosition(5 * time.Second)
	before := make([]fyne.Size, len(mw.preview.lines.Objects))
	for i, o := range mw.preview.lines.Objects {
		before[i] = o.Size()
	}

	mw.setUIPosition(31 * time.Second) // different lines lit
	for i, o := range mw.preview.lines.Objects {
		if o.Size() != before[i] {
			t.Errorf("line %d resized from %v to %v when the highlight moved", i+1, before[i], o.Size())
		}
	}
}

func TestPreviewFollowsEdits(t *testing.T) {
	mw := newPreviewTestWindow(t)
	mw.lyricRows.Objects[1].(*lyricRow).text.SetText("two, edited")
	if got := mw.preview.lines.Objects[1].(*previewLine).Text; got != "two, edited" {
		t.Fatalf("preview shows %q after an edit", got)
	}
	mw.deleteLine(0)
	if got := len(mw.preview.lines.Objects); got != 7 {
		t.Fatalf("preview has %d lines after a delete, want 7", got)
	}
}

func TestClickLyricSeeks(t *testing.T) {
	mw := newPreviewTestWindow(t)
	if err := mw.play(); err != nil {
		t.Fatal(err)
	}
	line5 := mw.preview.lines.Objects[4].(*previewLine)
	if line5.Cursor() != desktop.PointerCursor {
		t.Error("a timed line does not show the pointer cursor")
	}

	test.Tap(line5)
	if got := fakeFor(t, mw).state().position; got != 50*time.Second {
		t.Fatalf("player at %v after clicking line 5, want 0:50", got)
	}
	if mw.position != 50*time.Second || !mw.playing {
		t.Fatalf("after the click: position %v, playing %t; want 0:50, still playing", mw.position, mw.playing)
	}
	if got := litLines(mw); !slices.Equal(got, []int{5}) {
		t.Errorf("lit lines %v after the click, want [5]", got)
	}
	if got, want := mw.preview.scroll.Offset.Y, line5.Position().Y; got != want {
		t.Errorf("view at %v after the click, want line 5 at the top (%v)", got, want)
	}

	// Paused stays paused.
	mw.pause()
	test.Tap(mw.preview.lines.Objects[1].(*previewLine))
	if mw.position != 10*time.Second || mw.playing {
		t.Fatalf("clicking while paused: position %v, playing %t; want 0:10, paused", mw.position, mw.playing)
	}
}

func TestClickUntimedLyricDoesNothing(t *testing.T) {
	mw := newPreviewTestWindow(t)
	mw.setUIPosition(15 * time.Second)
	mw.insertLine(0) // untimed, at the top
	mw.window.Canvas().Unfocus()
	untimed := mw.preview.lines.Objects[0].(*previewLine)

	test.Tap(untimed)
	if mw.position != 15*time.Second {
		t.Fatalf("clicking an untimed line moved the song to %v", mw.position)
	}
	if untimed.Cursor() != desktop.DefaultCursor {
		t.Error("an untimed line shows the pointer cursor")
	}
}

func TestClickLyricWithoutAudio(t *testing.T) {
	mw := newTestMainWindow(t)
	if err := mw.importTTML(writeTTML(t, previewTTML)); err != nil {
		t.Fatal(err)
	}
	test.Tap(mw.preview.lines.Objects[4].(*previewLine)) // must not panic
	if mw.position != 0 {
		t.Fatalf("position %v without audio", mw.position)
	}
}
