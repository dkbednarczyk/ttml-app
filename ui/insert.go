package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"ttml-app/core"
)

// The insert icons show where the new line goes: a plus above a box (the
// hovered line) inserts above it, a box above a plus inserts below.
var (
	insertAboveIcon = theme.NewThemedResource(fyne.NewStaticResource("insert-above.svg", []byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">`+
			`<path d="M11 1h2v3h3v2h-3v3h-2V6H8V4h3z"/>`+
			`<path fill-rule="evenodd" d="M2 12h20v10H2zm2 2v6h16v-6z"/></svg>`)))
	insertBelowIcon = theme.NewThemedResource(fyne.NewStaticResource("insert-below.svg", []byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">`+
			`<path fill-rule="evenodd" d="M2 2h20v10H2zm2 2v6h16V4z"/>`+
			`<path d="M11 15h2v3h3v2h-3v3h-2v-3H8v-2h3z"/></svg>`)))
)

// insertLine inserts an untimed line at index i and puts the cursor in its
// text box, ready for the lyric.
func (m *mainWindow) insertLine(i int) {
	c, err := core.NewInsertLine(m.history.Project(), i)
	if err != nil {
		fyne.LogError("insert lyric line", err)

		return
	}
	m.apply(c)
	m.window.Canvas().Focus(m.lyricRows.Objects[i].(*lyricRow).text)
}

func newInsertButton(icon fyne.Resource, tapped func()) *widget.Button {
	b := widget.NewButtonWithIcon("", icon, tapped)
	b.Importance = widget.LowImportance

	return b
}
