package ui

import (
	"image"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// trackLine is the header's track info. Without album art it is one line,
// the title in bold, then "— artist". With art, the art sits on the left and
// the title is stacked on top of the artist.
//
// It uses two labels rather than one rich text, because Fyne's rich text cuts
// each part off on its own (no "…", and the second part vanishes) when space
// runs out. Add the Container to a layout, not the trackLine: Fyne only draws
// the children of its own container and widget types.
type trackLine struct {
	*fyne.Container
	art           *canvas.Image
	title, artist *widget.Label
	artistName    string
}

func newTrackLine() *trackLine {
	t := &trackLine{
		art:    canvas.NewImageFromImage(nil),
		title:  widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		artist: widget.NewLabel(""),
	}
	t.art.FillMode = canvas.ImageFillContain
	t.art.CornerRadius = theme.Size(theme.SizeNameInputRadius)
	t.art.Hide()
	t.title.Truncation = fyne.TextTruncateEllipsis
	t.artist.Truncation = fyne.TextTruncateEllipsis
	t.Container = container.New(trackLineLayout{}, t.art, t.title, t.artist)

	return t
}

// set shows a track; art may be nil.
func (t *trackLine) set(title, artist string, art image.Image) {
	t.artistName = artist
	t.title.SetText(title)
	t.art.Image = art
	if art != nil {
		t.art.Show()
		t.artist.SetText(artist)
	} else {
		t.art.Hide()
		t.artist.SetText("— " + artist)
	}
	if artist == "" {
		t.artist.Hide()
	} else {
		t.artist.Show()
	}
	t.art.Refresh()
	t.Refresh()
}

// String is the track as read, "Title — Artist".
func (t *trackLine) String() string {
	if t.artistName == "" {
		return t.title.Text
	}

	return t.title.Text + " — " + t.artistName
}

// trackLineLayout lays out the art and the two labels.
type trackLineLayout struct{}

// artSide is the album art's size: the height of two lines of text.
func artSide(label fyne.CanvasObject) float32 {
	return 2 * (label.MinSize().Height - theme.Size(theme.SizeNameInnerPadding))
}

func (trackLineLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	art, title := objects[0], objects[1]
	if art.Visible() {
		side := artSide(title)

		return fyne.NewSize(side+title.MinSize().Width, side)
	}

	return title.MinSize()
}

func (l trackLineLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	art, title, artist := objects[0], objects[1].(*widget.Label), objects[2].(*widget.Label)
	if art.Visible() {
		l.stacked(art, title, artist, size)
	} else {
		l.oneLine(title, artist, size)
	}
}

// stacked puts the art on the left and centres the title's and artist's
// lines on its top and bottom halves.
func (trackLineLayout) stacked(art fyne.CanvasObject, title, artist *widget.Label, size fyne.Size) {
	side := artSide(title)
	art.Move(fyne.NewPos(0, 0))
	art.Resize(fyne.NewSquareSize(side))

	labelH := title.MinSize().Height
	textW := size.Width - side
	title.Move(fyne.NewPos(side, side/4-labelH/2))
	title.Resize(fyne.NewSize(textW, labelH))
	artist.Move(fyne.NewPos(side, side*3/4-labelH/2))
	artist.Resize(fyne.NewSize(textW, labelH))
}

// oneLine puts "— artist" right after the title, one space from its end, so
// the dash has the same space on both sides. When the line is too long, the
// title is cut off first (down to half the width), then the artist.
func (trackLineLayout) oneLine(title, artist *widget.Label, size fyne.Size) {
	pad := theme.Size(theme.SizeNameInnerPadding)
	textSize := theme.Size(theme.SizeNameText)
	space := fyne.MeasureText(" ", textSize, fyne.TextStyle{}).Width
	titleText := fyne.MeasureText(title.Text, textSize, title.TextStyle).Width

	titleW := labelWidth(titleText)
	if artist.Visible() {
		artistText := fyne.MeasureText(artist.Text, textSize, artist.TextStyle).Width
		room := size.Width - (space + labelWidth(artistText) - pad) // the artist's left padding overlaps the title
		titleW = min(titleW, max(room, size.Width/2))
	}
	titleW = min(titleW, size.Width)
	title.Move(fyne.NewPos(0, 0))
	title.Resize(fyne.NewSize(titleW, size.Height))

	// Where the drawn title ends: its full text, or the cut-off text and "…".
	titleEnd := min(pad+titleText, titleW-pad)
	artistX := titleEnd + space - pad
	artist.Move(fyne.NewPos(artistX, 0))
	artist.Resize(fyne.NewSize(max(size.Width-artistX, 0), size.Height))
}

// labelWidth is the width a truncating label needs to show text this wide.
// It rounds up with a pixel to spare: Fyne's truncation check rounds the
// space down to whole pixels, so text given exactly its width gets cut off.
func labelWidth(text float32) float32 {
	return float32(math.Ceil(float64(text))) + 1 + 2*theme.Size(theme.SizeNameInnerPadding)
}

// textWidth is the width a truncating label needs to show all its text.
func textWidth(l *widget.Label) float32 {
	return labelWidth(fyne.MeasureText(l.Text, theme.Size(theme.SizeNameText), l.TextStyle).Width)
}
