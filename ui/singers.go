package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"ttml-app/core"
)

// singerButton shows who sings a line and opens the singer menu. Every row's
// button is as wide as the longest name, so the text boxes line up.
type singerButton struct {
	widget.Button
	width float32
}

func newSingerButton(tapped func()) *singerButton {
	b := &singerButton{}
	b.ExtendBaseWidget(b)
	b.Icon = theme.MenuDropDownIcon()
	b.IconPlacement = widget.ButtonIconTrailingText
	b.Alignment = widget.ButtonAlignLeading
	b.Importance = widget.LowImportance
	b.OnTapped = tapped

	return b
}

func (b *singerButton) MinSize() fyne.Size {
	return b.Button.MinSize().Max(fyne.NewSize(b.width, 0))
}

// singerWidth is the width of a singer button that fits every choice.
func singerWidth(p *core.Project) float32 {
	var width float32
	for _, i := range singerChoices(p) {
		width = max(width, widget.NewButtonWithIcon(p.SingerName(i), theme.MenuDropDownIcon(), nil).MinSize().Width)
	}

	return width
}

// singerChoices lists who can sing a line: each singer, then everyone.
func singerChoices(p *core.Project) []int {
	choices := make([]int, 0, len(p.Singers)+1)
	for i := range max(len(p.Singers), 1) {
		choices = append(choices, i)
	}

	return append(choices, core.Everyone)
}

// showSingerMenu opens the singer menu under line i's singer button.
func (m *mainWindow) showSingerMenu(i int, button fyne.CanvasObject) {
	widget.ShowPopUpMenuAtRelativePosition(m.singerMenu(i), m.window.Canvas(), fyne.NewPos(0, button.Size().Height), button)
}

// singerMenu is the menu of line i's singer button: pick who sings the line,
// add a singer for it, rename its singer, or remove any singer (one added by
// mistake, say).
func (m *mainWindow) singerMenu(i int) *fyne.Menu {
	p := m.history.Project()
	current := p.Lines[i].Singer
	var items []*fyne.MenuItem
	for _, singer := range singerChoices(p) {
		item := fyne.NewMenuItem(p.SingerName(singer), func() { m.setSinger(i, singer) })
		item.Checked = singer == current
		items = append(items, item)
	}
	items = append(items, fyne.NewMenuItemSeparator(), fyne.NewMenuItem("Add singer…", func() { m.showAddSinger(i) }))
	if current != core.Everyone {
		items = append(items, fyne.NewMenuItem("Rename "+p.SingerName(current)+"…", func() { m.showRenameSinger(current) }))
	}
	if len(p.Singers) > 1 {
		remove := fyne.NewMenuItem("Remove singer", nil)
		remove.ChildMenu = fyne.NewMenu("")
		for singer := range p.Singers {
			remove.ChildMenu.Items = append(remove.ChildMenu.Items,
				fyne.NewMenuItem(p.SingerName(singer), func() { m.removeSinger(singer) }))
		}
		items = append(items, remove)
	}

	return fyne.NewMenu("", items...)
}

// removeSinger removes a singer. Their lines go to the first singer left;
// undo brings everything back.
func (m *mainWindow) removeSinger(singer int) {
	c, err := core.NewRemoveSinger(m.history.Project(), singer)
	if err != nil {
		fyne.LogError("remove singer", err)

		return
	}
	m.apply(c)
}

func (m *mainWindow) setSinger(i, singer int) {
	c, err := core.NewSetSinger(m.history.Project(), i, singer)
	if err != nil {
		fyne.LogError("set singer", err)

		return
	}
	m.apply(c)
}

// showAddSinger asks for a new singer's name and adds them. With line i not
// -1, that line goes to the new singer. Once there are two singers, each row
// gets a singer button.
func (m *mainWindow) showAddSinger(i int) {
	m.askSingerName("Add singer", "Add", "", func(name string) {
		c, err := core.NewAddSinger(m.history.Project(), name, i)
		if err != nil {
			fyne.LogError("add singer", err)

			return
		}
		m.apply(c)
	})
}

func (m *mainWindow) showRenameSinger(singer int) {
	var name string
	if p := m.history.Project(); singer < len(p.Singers) {
		name = p.Singers[singer].Name
	}
	m.askSingerName("Rename "+m.history.Project().SingerName(singer), "Rename", name, func(name string) {
		c, err := core.NewRenameSinger(m.history.Project(), singer, name)
		if err != nil {
			fyne.LogError("rename singer", err)

			return
		}
		m.apply(c)
	})
}

// askSingerName shows a dialog with a name box, then hands the keys back to
// S, E and Space.
func (m *mainWindow) askSingerName(title, confirm, name string, then func(string)) {
	entry := widget.NewEntry()
	entry.SetText(name)
	entry.SetPlaceHolder("Optional, like Clean or Scream")
	form := dialog.NewForm(title, confirm, "Cancel", []*widget.FormItem{widget.NewFormItem("Name", entry)}, func(ok bool) {
		if ok {
			then(entry.Text)
		}
		m.window.Canvas().Unfocus()
	}, m.window)
	entry.OnSubmitted = func(string) { form.Submit() }
	form.Resize(fyne.NewSize(400, form.MinSize().Height))
	form.Show()
	m.window.Canvas().Focus(entry)
}
