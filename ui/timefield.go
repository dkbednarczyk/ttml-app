package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"ttml-app/core"
)

// timeField shows one line boundary. Tapping it opens a box with the whole
// time selected. Enter commits; a time that can't be used keeps the box open
// with the reason shown. Esc or clicking away restores the old time.
type timeField struct {
	widget.BaseWidget
	label   *widget.Label
	entry   *timeEntry
	box     *container.ThemeOverride // the entry, with a red border while refused
	editing bool
	refused bool
	// commit tries the typed text and returns why it can't be used, if so.
	commit func(text string) error
	// showError shows the reason under the row, or clears it for nil.
	showError func(error)
}

func newTimeField(commit func(string) error, showError func(error)) *timeField {
	f := &timeField{
		label:     widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Monospace: true}),
		commit:    commit,
		showError: showError,
	}
	f.entry = &timeEntry{field: f}
	f.entry.ExtendBaseWidget(f.entry)
	f.entry.TextStyle = fyne.TextStyle{Monospace: true}
	f.entry.SetPlaceHolder("0:00.000")
	f.entry.OnChanged = func(string) { f.setError(nil) }
	f.entry.OnSubmitted = f.submit
	f.box = container.NewThemeOverride(f.entry, refusedTheme{f})
	f.box.Hide()
	f.ExtendBaseWidget(f)

	return f
}

func (f *timeField) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(f.label, f.box))
}

// MinSize fits both the label and the box, so opening the box does not shift the row.
func (f *timeField) MinSize() fyne.Size {
	f.ExtendBaseWidget(f)

	return f.label.MinSize().Max(f.box.MinSize())
}

func (*timeField) Cursor() desktop.Cursor { return desktop.TextCursor }

// setTime shows the model's time. An open box keeps what the user is typing.
func (f *timeField) setTime(ms *core.Ms) {
	f.label.SetText(formatBoundary(ms))
}

func (f *timeField) Tapped(*fyne.PointEvent) {
	if f.editing {
		return
	}
	f.editing = true
	text := f.label.Text
	if _, err := core.ParseMs(text); err != nil {
		text = "" // an unset end shows dashes; start from an empty box instead
	}
	f.entry.SetText(text)
	f.label.Hide()
	f.box.Show()
	if c := fyne.CurrentApp().Driver().CanvasForObject(f); c != nil {
		c.Focus(f.entry)
	}
	f.entry.TypedShortcut(&fyne.ShortcutSelectAll{})
}

func (f *timeField) submit(text string) {
	if err := f.commit(text); err != nil {
		f.setError(err)

		return
	}
	f.close()
	f.unfocus()
}

// close hides the box. The label still holds the model's time, so closing
// without a commit restores the old value.
func (f *timeField) close() {
	if !f.editing {
		return
	}
	f.editing = false
	f.setError(nil)
	f.box.Hide()
	f.label.Show()
}

func (f *timeField) unfocus() {
	if c := fyne.CurrentApp().Driver().CanvasForObject(f); c != nil {
		c.Unfocus()
	}
}

func (f *timeField) setError(err error) {
	if refused := err != nil; refused != f.refused {
		f.refused = refused
		f.box.Refresh()
	}
	f.showError(err)
}

// refusedTheme is the app theme, except that the box's focus border is red
// while the typed time is refused. Fyne's own validation border only shows
// on an unfocused entry, and this box keeps focus until the time is fixed.
type refusedTheme struct{ field *timeField }

func (t refusedTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNamePrimary && t.field.refused {
		name = theme.ColorNameError
	}

	return t.base().Color(name, variant)
}

func (t refusedTheme) Font(style fyne.TextStyle) fyne.Resource    { return t.base().Font(style) }
func (t refusedTheme) Icon(name fyne.ThemeIconName) fyne.Resource { return t.base().Icon(name) }
func (t refusedTheme) Size(name fyne.ThemeSizeName) float32       { return t.base().Size(name) }

func (refusedTheme) base() fyne.Theme { return fyne.CurrentApp().Settings().Theme() }

// timeEntry is the box inside a timeField: Esc cancels, and losing focus
// (clicking elsewhere) cancels too.
type timeEntry struct {
	widget.Entry
	field *timeField
}

func (e *timeEntry) TypedKey(key *fyne.KeyEvent) {
	if key.Name == fyne.KeyEscape {
		e.field.close()
		e.field.unfocus()

		return
	}
	e.Entry.TypedKey(key)
}

func (e *timeEntry) FocusLost() {
	e.Entry.FocusLost()
	e.field.close()
}
