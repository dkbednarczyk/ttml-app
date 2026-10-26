package ui

import (
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

const pastedSong = "[Verse 1]\nWe've seen this place before\nA room that's void of doors\n\n[Chorus]\nIt's commonplace\n"

// nextNumbers returns the 1-based numbers shown in the accent colour.
func nextNumbers(mw *mainWindow) []int {
	var next []int
	for i, o := range mw.lyricRows.Objects {
		if o.(*lyricRow).number.label.Importance == widget.HighImportance {
			next = append(next, i+1)
		}
	}

	return next
}

func TestPasteFromLaunchScreen(t *testing.T) {
	mw := newTestMainWindow(t)
	mw.pasteLyrics(pastedSong)

	checkRows(t, mw, []string{"  1 -:--.--- – -:--.---", "  2 -:--.--- – -:--.---", "  3 -:--.--- – -:--.---"})
	if got := mw.lyricRows.Objects[2].(*lyricRow).text.Text; got != "It's commonplace" {
		t.Errorf("line 3 text = %q", got)
	}
	if !mw.tabs.Visible() || mw.emptyState.Visible() || mw.trackInfo.String() != untitledLyrics {
		t.Fatal("pasting from the launch screen did not open the editor")
	}
	if got := nextNumbers(mw); len(got) != 1 || got[0] != 1 {
		t.Errorf("next line marked %v, want [1]", got)
	}
	if !mw.history.Dirty() {
		t.Error("pasted lines don't count as unsaved")
	}
	// Audio added later keeps the pasted lines.
	if err := mw.loadAudio(testAudioPath); err != nil {
		t.Fatal(err)
	}
	if got := len(mw.history.Project().Lines); got != 3 {
		t.Fatalf("loading audio after a paste left %d lines, want 3", got)
	}
}

func TestTapThroughPastedLyrics(t *testing.T) {
	mw := newPlayingTestWindow(t)
	mw.pasteLyrics(pastedSong)
	fake := fakeFor(t, mw)
	press := func(key fyne.KeyName, at time.Duration) {
		fake.setPosition(at)
		pressKey(mw, key)
	}

	press(fyne.KeyS, time.Second)
	if got := nextNumbers(mw); len(got) != 1 || got[0] != 2 {
		t.Fatalf("after starting line 1, next line marked %v, want [2]", got)
	}
	press(fyne.KeyS, 2*time.Second)
	press(fyne.KeyE, 2500*time.Millisecond) // a break
	press(fyne.KeyS, 4*time.Second)         // starts line 3, not a new line
	checkRows(t, mw, []string{"  1 0:01.000 – 0:02.000", "  2 0:02.000 – 0:02.500", "  3 0:04.000 – -:--.---"})
	if got := mw.lyricRows.Objects[2].(*lyricRow).text.Text; got != "It's commonplace" {
		t.Fatalf("line 3 text = %q; the tap should time the pasted line", got)
	}
	if got := nextNumbers(mw); len(got) != 0 {
		t.Errorf("next line marked %v with none waiting", got)
	}

	press(fyne.KeyS, 5*time.Second) // nothing waiting: a new line, as before
	checkRows(t, mw, []string{
		"  1 0:01.000 – 0:02.000", "  2 0:02.000 – 0:02.500", "  3 0:04.000 – 0:05.000", "  4 0:05.000 – -:--.---",
	})

	typeShortcut(t, mw, &fyne.ShortcutUndo{})
	typeShortcut(t, mw, &fyne.ShortcutUndo{})
	if got := len(mw.history.Project().Lines); got != 3 || mw.history.Project().Lines[2].Begin != nil {
		t.Fatalf("undoing two taps: %d lines, line 3 start %v; want 3 lines with line 3 waiting again",
			got, mw.history.Project().Lines[2].Begin)
	}
}

func TestTappingKeepsTheLineInView(t *testing.T) {
	mw := newPlayingTestWindow(t)
	mw.window.Resize(fyne.NewSize(editorW, editorH))
	mw.pasteLyrics(strings.Repeat("a line of lyrics\n", 40))
	fake := fakeFor(t, mw)
	for i := range 25 {
		fake.setPosition(time.Duration(i+1) * time.Second)
		pressKey(mw, fyne.KeyS)
	}

	// Line 25 was just started; the line before it is at the top of the list.
	if got, want := mw.lyricsScroll.Offset.Y, mw.lyricRows.Objects[23].Position().Y; got != want {
		t.Fatalf("Edit list scrolled to %v, want line 24 at the top (%v)", got, want)
	}
}

func TestPasteAppendsAndUndoesInOneStep(t *testing.T) {
	mw := newMarkedTestWindow(t) // two lines already
	mw.pasteLyrics("three\nfour")
	if got := len(mw.history.Project().Lines); got != 4 {
		t.Fatalf("%d lines after pasting two, want 4", got)
	}
	if got := mw.lyricRows.Objects[2].(*lyricRow).text.Text; got != "three" {
		t.Errorf("pasted lines don't follow the existing ones: line 3 = %q", got)
	}
	mw.undo()
	if got := len(mw.history.Project().Lines); got != 2 {
		t.Fatalf("one undo left %d lines, want the original 2", got)
	}

	mw.pasteLyrics("\n  \n[Chorus]\n") // nothing to add
	if got := len(mw.history.Project().Lines); got != 2 || mw.history.Redo() == false {
		t.Fatalf("an empty paste changed the lines (%d) or cleared redo", got)
	}
}

func TestPasteDialog(t *testing.T) {
	mw := newTestMainWindow(t)
	mw.window.Resize(fyne.NewSize(windowW, windowH))
	mw.showPasteDialog()

	overlay := mw.window.Canvas().Overlays().Top()
	if overlay == nil || !strings.Contains(test.RenderObjectToMarkup(overlay), "Add lines") {
		t.Fatal("no paste dialog")
	}
	box, ok := mw.window.Canvas().Focused().(*widget.Entry)
	if !ok || !box.MultiLine {
		t.Fatal("the paste box does not have focus")
	}
}

func TestPasteSplitsBackgroundVocals(t *testing.T) {
	mw := newTestMainWindow(t)
	mw.pasteLyrics("I can't let this go (let it burn)\nBURN IT DOWN\n")

	first, second := mw.lyricRows.Objects[0].(*lyricRow), mw.lyricRows.Objects[1].(*lyricRow)
	if first.text.Text != "I can't let this go" || first.bgText.Text != "(let it burn)" || !first.bgRow.Visible() {
		t.Errorf("line 1 = %q with background %q (shown %v), want the parentheses as its background vocal",
			first.text.Text, first.bgText.Text, first.bgRow.Visible())
	}
	if second.bgRow.Visible() {
		t.Error("line 2 shows a background vocal box it doesn't have")
	}
}
