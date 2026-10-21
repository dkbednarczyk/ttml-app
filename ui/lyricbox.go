package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// lyricBox is a line's text box, a little roomier than Fyne's default, with a
// delete icon as tall as the box at its right end while the pointer is over it.
type lyricBox struct {
	widget.BaseWidget
	entry *lyricEntry
	roomy *container.ThemeOverride
	trash *trashIcon
}

func newLyricBox(entry *lyricEntry, onDelete func()) *lyricBox {
	b := &lyricBox{entry: entry, roomy: container.NewThemeOverride(entry, roomyTheme{})}
	b.trash = newTrashIcon(onDelete, b.setHovered)
	b.trash.Hide()
	b.ExtendBaseWidget(b)

	return b
}

func (b *lyricBox) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.New(cornerLayout{}, b.roomy, b.trash))
}

// The text box itself isn't hoverable, so the pointer over it counts as over
// the lyricBox; the trash icon reports in itself, since the pointer is only
// ever "in" the innermost hoverable widget.
func (b *lyricBox) MouseIn(*desktop.MouseEvent)  { b.setHovered(true) }
func (*lyricBox) MouseMoved(*desktop.MouseEvent) {}
func (b *lyricBox) MouseOut()                    { b.setHovered(false) }

func (b *lyricBox) setHovered(hovered bool) {
	if hovered {
		b.trash.Show()
	} else {
		b.trash.Hide()
	}
}

// cornerLayout fills the space with the first object and puts the second at
// its right end, as a square as tall as the space, inset a little so it
// clears the box's border.
type cornerLayout struct{}

func (cornerLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return objects[0].MinSize()
}

func (cornerLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(size)

	inset := theme.Size(theme.SizeNameInnerPadding) / 2
	side := size.Height - 2*inset
	objects[1].Move(fyne.NewPos(size.Width-side-inset, inset))
	objects[1].Resize(fyne.NewSquareSize(side))
}

// trashIcon deletes its line when clicked.
type trashIcon struct {
	widget.BaseWidget
	icon     *widget.Icon
	onDelete func()
	onHover  func(bool)
}

func newTrashIcon(onDelete func(), onHover func(bool)) *trashIcon {
	t := &trashIcon{icon: widget.NewIcon(theme.DeleteIcon()), onDelete: onDelete, onHover: onHover}
	t.ExtendBaseWidget(t)

	return t
}

func (t *trashIcon) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(t.icon)
}

func (t *trashIcon) MinSize() fyne.Size {
	return fyne.NewSquareSize(theme.Size(theme.SizeNameInlineIcon))
}

func (t *trashIcon) Tapped(*fyne.PointEvent)      { t.onDelete() }
func (*trashIcon) Cursor() desktop.Cursor         { return desktop.PointerCursor }
func (t *trashIcon) MouseIn(*desktop.MouseEvent)  { t.onHover(true) }
func (*trashIcon) MouseMoved(*desktop.MouseEvent) {}
func (t *trashIcon) MouseOut()                    { t.onHover(false) }

// roomyTheme is the app theme with 1.5 times the padding inside a text box,
// which makes the lyric boxes a little larger.
type roomyTheme struct{}

func (roomyTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameInnerPadding {
		return 1.5 * appTheme().Size(name)
	}

	return appTheme().Size(name)
}

func (roomyTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return appTheme().Color(name, variant)
}
func (roomyTheme) Font(style fyne.TextStyle) fyne.Resource    { return appTheme().Font(style) }
func (roomyTheme) Icon(name fyne.ThemeIconName) fyne.Resource { return appTheme().Icon(name) }

func appTheme() fyne.Theme { return fyne.CurrentApp().Settings().Theme() }
