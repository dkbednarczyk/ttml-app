package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"ttml-app/core"
	"ttml-app/formats"
)

// untitledLyrics heads the editor for lyrics pasted with no file open.
const untitledLyrics = "Untitled lyrics"

// showPasteDialog asks for the lyrics as plain text, to add as untimed lines.
func (m *mainWindow) showPasteDialog() {
	box := widget.NewMultiLineEntry()
	box.SetPlaceHolder("Paste the lyrics here, one line per lyric.\nBlank lines and markers like [Chorus] are skipped.")
	box.SetMinRowsVisible(14)
	d := dialog.NewCustomConfirm("Paste lyrics", "Add lines", "Cancel", box, func(add bool) {
		if add {
			m.pasteLyrics(box.Text)
		}
		m.window.Canvas().Unfocus() // so S, E and Space work again
	}, m.window)
	d.Resize(fyne.NewSize(560, d.MinSize().Height))
	d.Show()
	m.window.Canvas().Focus(box)
}

// pasteLyrics adds the lines of pasted text after the existing ones, with no
// times: each S then starts the next one. One undo removes the whole paste.
func (m *mainWindow) pasteLyrics(text string) {
	lines := formats.ParseText(text)
	if len(lines) == 0 {
		return
	}
	c, err := core.NewInsertLines(m.history.Project(), len(m.history.Project().Lines), lines)
	if err != nil {
		fyne.LogError("paste lyrics", err)

		return
	}
	if m.emptyState.Visible() {
		m.showTrack(untitledLyrics, "", nil) // nothing was open: show the editor
	}
	m.apply(c)
}
