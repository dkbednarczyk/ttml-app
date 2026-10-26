package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"ttml-app/core"
)

// backgroundIcon shows a line with a smaller, indented one under it.
var backgroundIcon = theme.NewThemedResource(fyne.NewStaticResource("background-vocal.svg", []byte(
	`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">`+
		`<path fill-rule="evenodd" d="M2 2h20v10H2zm2 2v6h16V4z"/>`+
		`<path d="M8 15h14v2H8zm0 4h10v2H8z"/></svg>`)))

// openBackground shows line i's background vocal box, empty if it has none
// yet, and puts the cursor in it.
func (m *mainWindow) openBackground(i int) {
	r := m.lyricRows.Objects[i].(*lyricRow)
	r.bgOpen = true
	r.bgRow.Show()
	m.lyricRows.Refresh() // the row's height changed
	// From here on the focus keeps the box open until it is left empty.
	m.window.Canvas().Focus(r.bgText)
}

// closeEmptyBackground hides line i's background vocal box when it was left
// empty.
func (m *mainWindow) closeEmptyBackground(i int) {
	r := m.lyricRows.Objects[i].(*lyricRow)
	if r.bgOpen && m.history.Project().Lines[i].Background == "" {
		r.bgOpen = false
		m.refreshLyrics()
	}
}

func (m *mainWindow) setBackground(i int, text string) {
	if m.history.Project().Lines[i].Background == text {
		return // refreshLyrics set the box to the model's text; nothing changed
	}
	c, err := core.NewSetBackground(m.history.Project(), i, text)
	if err != nil {
		fyne.LogError("set background vocal", err)

		return
	}
	m.apply(c)
}

// deleteBackground removes line i's background vocal and closes its box.
func (m *mainWindow) deleteBackground(i int) {
	m.lyricRows.Objects[i].(*lyricRow).bgOpen = false
	m.window.Canvas().Unfocus()
	if m.history.Project().Lines[i].Background == "" {
		m.refreshLyrics()

		return
	}
	m.setBackground(i, "")
}

// setBackgroundTime sets one boundary of line i's background vocal from typed
// text, or says why it can't. An empty box goes back to the line's time.
func (m *mainWindow) setBackgroundTime(i int, edge core.Edge, text string) error {
	var at *core.Ms
	if text != "" {
		ms, err := core.ParseMs(text)
		if err != nil {
			return err
		}
		at = &ms
	}
	c, err := core.NewSetBackgroundTime(m.history.Project(), i, edge, at, m.songEnd())
	if err != nil {
		return err
	}
	m.apply(c)

	return nil
}

// seekToBackground skips the song to the start of line i's background vocal.
func (m *mainWindow) seekToBackground(i int) bool {
	begin := m.history.Project().Lines[i].BackgroundTimes().Begin
	if m.player == nil || begin == nil {
		return false
	}
	m.seekTo(*begin)
	m.window.Canvas().Unfocus()

	return true
}

// spacer takes up the room of another object, so that the background vocal
// row lines up with the line above it. It is hidden while that object is.
type spacer struct {
	widget.BaseWidget
	of fyne.CanvasObject
}

func newSpacer(of fyne.CanvasObject) *spacer {
	s := &spacer{of: of}
	s.ExtendBaseWidget(s)

	return s
}

func (s *spacer) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(&fyne.Container{})
}

func (s *spacer) MinSize() fyne.Size { return s.of.MinSize() }
func (s *spacer) Visible() bool      { return s.of.Visible() }
