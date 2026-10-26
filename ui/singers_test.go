package ui

import (
	"slices"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"ttml-app/core"
)

// typeSingerName fills in the open singer name dialog and presses Enter.
func typeSingerName(t *testing.T, mw *mainWindow, name string) {
	t.Helper()

	entry, ok := mw.window.Canvas().Focused().(*widget.Entry)
	if !ok {
		t.Fatalf("focus is on %T, want the singer name box", mw.window.Canvas().Focused())
	}
	entry.SetText(name)
	entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
}

func menuLabels(menu *fyne.Menu) []string {
	var labels []string
	for _, item := range menu.Items {
		label := item.Label
		if item.IsSeparator {
			label = "---"
		}
		if item.Checked {
			label = "* " + label
		}
		labels = append(labels, label)
	}

	return labels
}

func TestSingerButtonsAppearWithSecondSinger(t *testing.T) {
	mw := newMarkedTestWindow(t)
	rows := mw.lyricRows.Objects
	if rows[0].(*lyricRow).singer.Visible() {
		t.Fatal("a singer button shows with only one singer")
	}

	mw.showAddSinger(-1)
	typeSingerName(t, mw, "Scream")
	first, second := rows[0].(*lyricRow).singer, rows[1].(*lyricRow).singer
	if !first.Visible() || first.Text != "Singer 1" || second.Text != "Singer 1" {
		t.Fatalf("after adding a singer: button shown %v, texts %q and %q; want Singer 1 on both",
			first.Visible(), first.Text, second.Text)
	}

	want := []string{"* Singer 1", "Scream", "Everyone", "---", "Add singer…", "Rename Singer 1…", "Remove singer"}
	menu := mw.singerMenu(1)
	if got := menuLabels(menu); !slices.Equal(got, want) {
		t.Fatalf("singer menu = %q, want %q", got, want)
	}
	menu.Items[1].Action() // Scream
	if second.Text != "Scream" || mw.history.Project().Lines[1].Singer != 1 {
		t.Fatalf("after picking Scream: button %q, line singer %d", second.Text, mw.history.Project().Lines[1].Singer)
	}
	if first.MinSize().Width != second.MinSize().Width {
		t.Errorf("singer buttons are %v and %v wide; want the same, so the text boxes line up",
			first.MinSize().Width, second.MinSize().Width)
	}

	// Renaming from the menu renames the singer everywhere.
	mw.singerMenu(0).Items[5].Action()
	typeSingerName(t, mw, "Clean")
	if first.Text != "Clean" {
		t.Errorf("after renaming: line 1 button %q, want Clean", first.Text)
	}

	// Removing Scream gives line 2 back to Clean and hides the singer buttons.
	remove := mw.singerMenu(0).Items[6].ChildMenu
	if got := menuLabels(remove); !slices.Equal(got, []string{"Clean", "Scream"}) {
		t.Fatalf("remove submenu = %q, want both singers", got)
	}
	remove.Items[1].Action()
	if first.Visible() || mw.history.Project().Lines[1].Singer != 0 {
		t.Fatalf("after removing Scream: buttons shown %v, line 2 singer %d", first.Visible(),
			mw.history.Project().Lines[1].Singer)
	}
	typeShortcut(t, mw, &fyne.ShortcutUndo{})
	if second.Text != "Scream" {
		t.Fatalf("undoing the removal: line 2 button %q, want Scream", second.Text)
	}

	// Undo the rename, the pick and the added singer: back to one singer.
	for range 3 {
		typeShortcut(t, mw, &fyne.ShortcutUndo{})
	}
	if first.Visible() || mw.history.Project().HasSingers() {
		t.Error("the singer buttons still show after undoing back to one singer")
	}
}

func TestAddSingerFromRowMenu(t *testing.T) {
	mw := newMarkedTestWindow(t)
	apply(t, mw, func(p *core.Project) (core.Command, error) { return core.NewAddSinger(p, "", -1) })

	mw.singerMenu(1).Items[4].Action() // Add singer…
	typeSingerName(t, mw, "Guest")
	p := mw.history.Project()
	if len(p.Singers) != 3 || p.Lines[1].Singer != 2 {
		t.Fatalf("singers %v, line 2 singer %d; want a third singer, Guest, singing line 2", p.Singers, p.Lines[1].Singer)
	}

	// New lines take the singer of the line before them.
	fake := fakeFor(t, mw)
	fake.setPosition(4 * time.Second)
	pressKey(mw, fyne.KeyS)
	if got := mw.lyricRows.Objects[2].(*lyricRow).singer.Text; got != "Guest" {
		t.Errorf("a tapped line's singer = %q, want Guest like the line before it", got)
	}
}

func TestPreviewPutsSecondSingerOnTheRight(t *testing.T) {
	mw := newMarkedTestWindow(t)
	apply(t, mw, func(p *core.Project) (core.Command, error) { return core.NewAddSinger(p, "", 1) })
	mw.tabs.SelectIndex(1)

	lines := mw.preview.lines.Objects
	if got := lines[0].(*previewLine).main.Alignment; got != fyne.TextAlignLeading {
		t.Errorf("first singer's line aligned %v, want leading", got)
	}
	if got := lines[1].(*previewLine).main.Alignment; got != fyne.TextAlignTrailing {
		t.Errorf("second singer's line aligned %v, want trailing", got)
	}
}

// apply runs a core command on the window's project, as the UI would.
func apply(t *testing.T, mw *mainWindow, newCommand func(*core.Project) (core.Command, error)) {
	t.Helper()

	c, err := newCommand(mw.history.Project())
	if err != nil {
		t.Fatalf("building command: %v", err)
	}
	mw.apply(c)
}
