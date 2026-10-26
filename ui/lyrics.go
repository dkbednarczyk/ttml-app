package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"ttml-app/core"
)

const (
	startLineLabel = "Start line (S)"
	nextLineLabel  = "Next line (S)"
	endLineLabel   = "End line (E)"
)

var saveShortcut = &desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierShortcutDefault}

// typedKey handles keys pressed while no widget has focus, so typing in a
// lyric box never triggers them.
func (m *mainWindow) typedKey(ev *fyne.KeyEvent) {
	switch ev.Name {
	case fyne.KeySpace:
		m.togglePlayback()
	case fyne.KeyS:
		m.tap()
	case fyne.KeyE:
		m.endLine()
	case fyne.KeyLeftBracket:
		m.stepSpeed(-1)
	case fyne.KeyRightBracket:
		m.stepSpeed(+1)
	}
}

type shortcut struct {
	keys fyne.Shortcut
	run  func()
}

// shortcuts lists the window's keyboard shortcuts.
func (m *mainWindow) shortcuts() []shortcut {
	return []shortcut{
		{&fyne.ShortcutUndo{}, m.undo}, // Ctrl+Z
		{&fyne.ShortcutRedo{}, m.redo}, // Ctrl+Y
		{&desktop.CustomShortcut{KeyName: fyne.KeyZ, Modifier: fyne.KeyModifierShortcutDefault | fyne.KeyModifierShift}, m.redo},
		{saveShortcut, m.save},
	}
}

func (m *mainWindow) addShortcuts() {
	for _, s := range m.shortcuts() {
		m.window.Canvas().AddShortcut(s.keys, func(fyne.Shortcut) { s.run() })
	}
}

// tap starts the line waiting at the playback position, ending the line
// before it if that is still open: the next pasted line, a line cleared for
// a re-tap, or with none waiting a new line at the end.
func (m *mainWindow) tap() {
	at, ok := m.markTime()
	if !ok {
		return
	}
	next := m.history.Project().NextLineAt(at)
	if !m.mark(core.NewTap, at) {
		return
	}
	if next < 0 {
		next = len(m.history.Project().Lines) - 1 // the tap added a line
	}
	m.showRow(next)
}

// endLine ends the open line without starting another, for a break in the vocals.
func (m *mainWindow) endLine() {
	if at, ok := m.markTime(); ok {
		m.mark(core.NewEndLine, at)
	}
}

// markTime returns the playback position for a mark, or false while paused:
// marks only count while the song plays; while paused they would land
// wherever the playhead happens to sit, such as 0:00 right after loading.
func (m *mainWindow) markTime() (core.Ms, bool) {
	return m.playhead(), m.playing
}

// playhead is the playback position: the player's own when there is one,
// which runs ahead of the last clock tick while playing.
func (m *mainWindow) playhead() core.Ms {
	if m.player != nil {
		return core.Ms(m.player.Position().Milliseconds())
	}

	return core.Ms(m.position.Milliseconds())
}

// mark applies a tap-style command at time at and reports whether it did anything.
func (m *mainWindow) mark(newCommand func(*core.Project, core.Ms) (core.Command, error), at core.Ms) bool {
	c, err := newCommand(m.history.Project(), at)
	if err != nil {
		// Nothing to mark here: no open line to end, or no line waiting
		// for a tap after seeking back.
		return false
	}
	m.apply(c)

	return true
}

// seekToLine skips the song to line i's start and hands the keys back to S,
// E and Space. It reports false when the line has no start or there's no audio.
func (m *mainWindow) seekToLine(i int) bool {
	begin := m.history.Project().Lines[i].Begin
	if m.player == nil || begin == nil {
		return false
	}
	m.seekTo(*begin)
	m.window.Canvas().Unfocus()

	return true
}

// showRow scrolls the Edit list so row i is in view, with the row before it
// above for context, so tapping down a pasted song keeps your place visible.
func (m *mainWindow) showRow(i int) {
	rows := m.lyricRows.Objects
	if i < 0 || i >= len(rows) {
		return
	}
	y := rows[max(i-1, 0)].Position().Y
	maxY := m.lyricsScroll.Content.MinSize().Height - m.lyricsScroll.Size().Height
	m.lyricsScroll.ScrollToOffset(fyne.NewPos(0, max(0, min(y, maxY))))
	m.lyricsScroll.Refresh()
}

func (m *mainWindow) setLineText(i int, text string) {
	if m.history.Project().Lines[i].Text == text {
		return // refreshLyrics set the box to the model's text; nothing changed
	}
	c, err := core.NewSetText(m.history.Project(), i, text)
	if err != nil {
		fyne.LogError("set lyric text", err)

		return
	}
	m.apply(c)
}

// clearTimesFrom clears the times of line i and every line after it, so the
// next S starts line i again: for re-tapping from a tap that went wrong.
func (m *mainWindow) clearTimesFrom(i int) {
	c, err := core.NewClearTimesFrom(m.history.Project(), i)
	if err != nil {
		fyne.LogError("clear times", err)

		return
	}
	m.apply(c)
}

// deleteLine removes line i. The other lines keep their times.
func (m *mainWindow) deleteLine(i int) {
	c, err := core.NewDeleteLine(m.history.Project(), i)
	if err != nil {
		fyne.LogError("delete lyric line", err)

		return
	}
	m.apply(c)
}

func (m *mainWindow) apply(c core.Command) {
	m.history.Apply(c)
	m.refreshLyrics()
}

func (m *mainWindow) undo() {
	if m.history.Undo() {
		m.refreshLyrics()
	}
}

func (m *mainWindow) redo() {
	if m.history.Redo() {
		m.refreshLyrics()
	}
}

// clearLines drops all lines and their undo history.
func (m *mainWindow) clearLines() {
	m.history = core.History{}
	m.refreshLyrics()
}

// confirmDiscard runs then, first asking for confirmation if there are unsaved lines.
func (m *mainWindow) confirmDiscard(action string, then func()) {
	if !m.history.Dirty() {
		then()

		return
	}
	d := dialog.NewConfirm("Unsaved lines", "Your lines are not saved and will be lost.", func(discard bool) {
		if discard {
			then()
		}
	}, m.window)
	d.SetConfirmText(action + " without saving")
	d.SetDismissText("Cancel")
	d.Show()
}

// refreshLyrics brings the rows and buttons in line with the project.
// Rows are positional: row i always shows line i, so after a deletion the
// rows below it show the next lines and the last row goes away.
func (m *mainWindow) refreshLyrics() {
	lines := m.history.Project().Lines
	rows := m.lyricRows.Objects
	for i := len(rows); i < len(lines); i++ {
		rows = append(rows, m.newLyricRow(i))
	}
	rows = rows[:len(lines)]
	p := m.history.Project()
	focused := m.window.Canvas().Focused()
	var singers float32 // the singer buttons' width; 0 hides them
	if p.HasSingers() {
		singers = singerWidth(p)
	}
	timedFrom := false // line i or a line after it has a time to clear
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		timedFrom = timedFrom || line.Timing != (core.Timing{}) || line.BackgroundTiming != (core.Timing{})
		r := rows[i].(*lyricRow)
		setEnabled(r.clearFrom, timedFrom)
	}
	for i, line := range lines {
		r := rows[i].(*lyricRow)
		r.show(i, line, focused == r.bgText)
		r.showSinger(p.SingerName(line.Singer), singers)
	}
	m.lyricRows.Objects = rows
	m.lyricRows.Refresh()
	m.updateMarks()
	m.updatePreview(true)
	m.updateStatus()
}

// updateMarks marks the line the next S starts and sets the mark and save
// buttons. The marks follow the playback position, which decides what a tap
// starts and ends: back in a cleared line's place, that line is next.
func (m *mainWindow) updateMarks() {
	p := m.history.Project()
	at := m.playhead()
	next := p.NextLineAt(at)
	for i, row := range m.lyricRows.Objects {
		row.(*lyricRow).number.setNext(i == next)
	}
	open := p.OpenLineAt(at) >= 0
	label := startLineLabel
	if open {
		label = nextLineLabel
	}
	if m.tapButton.Text != label {
		m.tapButton.SetText(label)
	}
	setEnabled(m.tapButton, m.playing)
	setEnabled(m.endButton, m.playing && open)
	setEnabled(m.saveButton, m.history.Dirty())
}

func setEnabled(w fyne.Disableable, enabled bool) {
	if enabled {
		w.Enable()
	} else {
		w.Disable()
	}
}

func (m *mainWindow) newLyricRow(i int) *lyricRow {
	text := &lyricEntry{save: m.save, seek: func() bool { return m.seekToLine(i) }}
	text.ExtendBaseWidget(text)
	text.SetPlaceHolder("Lyric text")
	text.OnChanged = func(s string) { m.setLineText(i, s) }
	// Enter leaves the box, so Space, S and E work again.
	text.OnSubmitted = func(string) { m.window.Canvas().Unfocus() }

	bgText := &lyricEntry{
		save:      m.save,
		seek:      func() bool { return m.seekToBackground(i) },
		focusLost: func() { m.closeEmptyBackground(i) },
	}
	bgText.ExtendBaseWidget(bgText)
	bgText.SetPlaceHolder("Background vocal")
	bgText.OnChanged = func(s string) { m.setBackground(i, s) }
	bgText.OnSubmitted = func(string) { m.window.Canvas().Unfocus() }

	r := &lyricRow{
		text:   text,
		bgText: bgText,
		reason: widget.NewLabel(""),
	}
	r.number = newLineNumber()
	r.box = newLyricBox(text, func() { m.deleteLine(i) })
	r.bgBox = newLyricBox(bgText, func() { m.deleteBackground(i) })
	r.singer = newSingerButton(func() { m.showSingerMenu(i, r.singer) })
	r.singer.Hide()
	r.clearFrom = newInsertButton(theme.HistoryIcon(), func() { m.clearTimesFrom(i) })
	r.inserts = container.NewHBox(
		newInsertButton(insertAboveIcon, func() { m.insertLine(i) }),
		newInsertButton(insertBelowIcon, func() { m.insertLine(i + 1) }),
		newInsertButton(backgroundIcon, func() { m.openBackground(i) }),
		r.clearFrom,
	)
	r.reason.Importance = widget.DangerImportance
	r.reason.Hide()
	showError := func(err error) {
		r.setReason(err)
		m.lyricRows.Refresh() // the row's height changed
	}
	r.begin = newTimeField(func(s string) error { return m.setTime(i, core.Begin, s) }, showError)
	r.end = newTimeField(func(s string) error { return m.setTime(i, core.End, s) }, showError)
	r.bgBegin = newTimeField(func(s string) error { return m.setBackgroundTime(i, core.Begin, s) }, showError)
	r.bgEnd = newTimeField(func(s string) error { return m.setBackgroundTime(i, core.End, s) }, showError)
	// The background vocal's row lines up under the line's: its times under
	// the line's times, its box under the line's box.
	bgLeft := container.NewCenter(container.NewHBox(
		newSpacer(r.number), r.bgBegin, widget.NewLabel("–"), r.bgEnd, newSpacer(r.singer)))
	r.bgRow = container.NewBorder(nil, nil, bgLeft, newSpacer(r.inserts), r.bgBox)
	r.bgRow.Hide()
	r.ExtendBaseWidget(r)

	return r
}

// setTime sets one boundary of line i from typed text, or says why it can't.
// An empty box unsets the boundary.
func (m *mainWindow) setTime(i int, edge core.Edge, text string) error {
	if text == "" {
		m.clearTime(i, edge)

		return nil
	}
	at, err := core.ParseMs(text)
	if err != nil {
		return err
	}
	c, err := core.NewSetTime(m.history.Project(), i, edge, at, core.Ms(m.duration.Milliseconds()))
	if err != nil {
		return err
	}
	m.apply(c)

	return nil
}

// clearTime unsets one boundary of line i, if it is set.
func (m *mainWindow) clearTime(i int, edge core.Edge) {
	line := m.history.Project().Lines[i]
	if edge == core.Begin && line.Begin == nil || edge == core.End && line.End == nil {
		return // nothing to undo
	}
	c, err := core.NewClearTime(m.history.Project(), i, edge)
	if err != nil {
		fyne.LogError("clear time", err)

		return
	}
	m.apply(c)
}

// lyricEntry is a text box that passes Save on to the window (a focused
// Entry otherwise receives every shortcut, and would swallow Ctrl+S), and
// seeks to its line on a double-click.
type lyricEntry struct {
	widget.Entry
	save      func()
	seek      func() bool // seeks to the line's start; false if it can't
	focusLost func()      // runs when the cursor leaves the box; may be nil
}

func (e *lyricEntry) FocusLost() {
	e.Entry.FocusLost()
	if e.focusLost != nil {
		e.focusLost()
	}
}

// DoubleTapped skips the song to the line. With no time or no audio to seek
// in, it selects a word as a text box normally does.
func (e *lyricEntry) DoubleTapped(ev *fyne.PointEvent) {
	if e.seek() {
		return
	}
	e.Entry.DoubleTapped(ev)
}

func (e *lyricEntry) TypedShortcut(s fyne.Shortcut) {
	if s.ShortcutName() == saveShortcut.ShortcutName() {
		e.save()

		return
	}
	e.Entry.TypedShortcut(s)
}

// lyricRow shows one line's number, boundaries, singer (once there are
// several) and text box; under them its background vocal, if it has one or
// one is being typed; and under that the reason a typed time was refused
// while there is one.
type lyricRow struct {
	widget.BaseWidget
	number         *lineNumber
	begin, end     *timeField
	singer         *singerButton
	text           *lyricEntry
	box            *lyricBox       // the text box, with its delete icon
	inserts        *fyne.Container // insert above / below, add a background vocal, clear times from here
	clearFrom      *widget.Button  // clears the times from this line on; disabled with none
	bgBegin, bgEnd *timeField
	bgText         *lyricEntry
	bgBox          *lyricBox
	bgRow          *fyne.Container
	bgOpen         bool // the background vocal box shows, even while empty
	reason         *widget.Label
}

func (r *lyricRow) CreateRenderer() fyne.WidgetRenderer {
	// Centred, so the number and times line up with the text in the taller box.
	left := container.NewCenter(container.NewHBox(r.number, r.begin, widget.NewLabel("–"), r.end, r.singer))

	return widget.NewSimpleRenderer(container.NewVBox(container.NewBorder(nil, nil, left, r.inserts, r.box), r.bgRow, r.reason))
}

func (r *lyricRow) setReason(err error) {
	if err == nil {
		r.reason.Hide()

		return
	}
	r.reason.SetText(err.Error())
	r.reason.Show()
}

// show brings the row in line with line i. typingBG says the cursor is in
// the background vocal box, which then stays open even if emptied.
func (r *lyricRow) show(i int, line core.Line, typingBG bool) {
	r.number.setNumber(i + 1)
	r.begin.setTime(line.Begin)
	r.end.setTime(line.End)
	if r.text.Text != line.Text {
		r.text.SetText(line.Text)
	}

	// Rows are positional, so an open box left over from a deleted line closes.
	r.bgOpen = line.Background != "" || r.bgOpen && typingBG
	if r.bgOpen {
		r.bgRow.Show()
	} else {
		r.bgRow.Hide()
	}
	bg := line.BackgroundTimes()
	r.bgBegin.setInheritedTime(bg.Begin, line.BackgroundTiming.Begin == nil)
	r.bgEnd.setInheritedTime(bg.End, line.BackgroundTiming.End == nil)
	if r.bgText.Text != line.Background {
		r.bgText.SetText(line.Background)
	}
}

// showSinger sets the singer button to name at width, or hides it for width 0.
func (r *lyricRow) showSinger(name string, width float32) {
	if width == 0 {
		r.singer.Hide()

		return
	}
	if r.singer.Text != name || r.singer.width != width {
		r.singer.width = width
		r.singer.SetText(name)
	}
	r.singer.Show()
}

// lineNumber shows a line's number.
type lineNumber struct {
	widget.BaseWidget
	label *widget.Label
	text  string
}

func newLineNumber() *lineNumber {
	n := &lineNumber{label: widget.NewLabelWithStyle("", fyne.TextAlignTrailing, fyne.TextStyle{Monospace: true})}
	n.ExtendBaseWidget(n)

	return n
}

func (n *lineNumber) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(n.label)
}

// setNext marks the line the next S starts, in the accent colour; only the
// colour changes, so the row doesn't move.
func (n *lineNumber) setNext(next bool) {
	importance := widget.MediumImportance
	if next {
		importance = widget.HighImportance
	}
	if n.label.Importance != importance {
		n.label.Importance = importance
		n.label.Refresh()
	}
}

func (n *lineNumber) setNumber(number int) {
	n.text = fmt.Sprintf("%3d", number)
	n.label.SetText(n.text)
}

// formatBoundary writes a time as m:ss.mmm (h:mm:ss.mmm past an hour), or dashes if unset.
func formatBoundary(ms *core.Ms) string {
	if ms == nil {
		return "-:--.---"
	}

	return ms.String()
}
