package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestAddBackgroundVocal(t *testing.T) {
	mw := newMarkedTestWindow(t)
	row := mw.lyricRows.Objects[1].(*lyricRow)
	if row.bgRow.Visible() {
		t.Fatal("a background vocal box shows on a line without one")
	}

	test.Tap(row.inserts.Objects[2].(*widget.Button))
	if !row.bgRow.Visible() || mw.window.Canvas().Focused() != row.bgText {
		t.Fatal("adding a background vocal did not open its box with the cursor in it")
	}
	row.bgText.SetText("(let it burn)")
	if got := mw.history.Project().Lines[1].Background; got != "(let it burn)" {
		t.Fatalf("line 2 background = %q, want the typed text", got)
	}
	// With no times of its own it shows the line's, greyed out.
	if row.bgBegin.label.Text != "0:02.000" || row.bgBegin.label.Importance != widget.LowImportance {
		t.Fatalf("background start shows %q (%v), want the line's 0:02.000 greyed", row.bgBegin.label.Text,
			row.bgBegin.label.Importance)
	}

	// Coming in late: its own start.
	test.Tap(row.bgBegin)
	row.bgBegin.entry.SetText("0:02.400")
	row.bgBegin.entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	line := mw.history.Project().Lines[1]
	if line.BackgroundTiming.Begin == nil || *line.BackgroundTiming.Begin != 2400 ||
		row.bgBegin.label.Importance == widget.LowImportance {
		t.Fatalf("background start = %v, label %q; want its own 0:02.400", line.BackgroundTiming.Begin, row.bgBegin.label.Text)
	}
	// It can't end before it starts.
	test.Tap(row.bgEnd)
	row.bgEnd.entry.SetText("0:02.100")
	row.bgEnd.entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if !row.bgEnd.editing || row.reason.Hidden {
		t.Fatal("an end before the background vocal's start was accepted")
	}
	row.bgEnd.entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})

	// An emptied time box goes back to the line's time.
	test.Tap(row.bgBegin)
	row.bgBegin.entry.SetText("")
	row.bgBegin.entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if mw.history.Project().Lines[1].BackgroundTiming.Begin != nil || row.bgBegin.label.Text != "0:02.000" {
		t.Fatalf("after clearing: background start %q, want the line's 0:02.000", row.bgBegin.label.Text)
	}

	test.Tap(row.bgBox.trash)
	if row.bgRow.Visible() || mw.history.Project().Lines[1].Background != "" {
		t.Fatal("the delete icon did not remove the background vocal")
	}
	typeShortcut(t, mw, &fyne.ShortcutUndo{})
	if !row.bgRow.Visible() || row.bgText.Text != "(let it burn)" {
		t.Fatal("undo did not bring back the background vocal")
	}
}

func TestEmptyBackgroundBoxClosesWhenLeft(t *testing.T) {
	mw := newMarkedTestWindow(t)
	row := mw.lyricRows.Objects[0].(*lyricRow)

	test.Tap(row.inserts.Objects[2].(*widget.Button))
	mw.window.Canvas().Unfocus()
	if row.bgRow.Visible() {
		t.Fatal("an empty background vocal box stayed open after leaving it")
	}
	if mw.history.Dirty() {
		t.Error("opening and leaving an empty background vocal box counts as a change")
	}
}

func TestImportedBackgroundVocalShows(t *testing.T) {
	mw := newTestMainWindow(t)
	const doc = `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata"><body><div>` +
		`<p begin="1s" end="3s">I can't let this go<span ttm:role="x-bg" begin="2s" end="3s">(let it burn)</span></p>` +
		`<p begin="3s" end="4s">Next</p></div></body></tt>`
	if err := mw.importTTML(writeTTML(t, doc)); err != nil {
		t.Fatal(err)
	}
	if mw.savePath == "" {
		t.Error("Save would ask for a new file, as if the background vocal were left out")
	}

	first, second := mw.lyricRows.Objects[0].(*lyricRow), mw.lyricRows.Objects[1].(*lyricRow)
	if !first.bgRow.Visible() || first.bgText.Text != "(let it burn)" || first.bgBegin.label.Text != "0:02.000" {
		t.Errorf("line 1 background row shown %v with %q from %s", first.bgRow.Visible(), first.bgText.Text,
			first.bgBegin.label.Text)
	}
	if second.bgRow.Visible() {
		t.Error("line 2 shows a background vocal box it doesn't have")
	}

	// Deleting line 1 moves line 2 up into row 1, which has no background vocal.
	mw.deleteLine(0)
	if first.bgRow.Visible() {
		t.Error("row 1 still shows the deleted line's background vocal box")
	}
}
