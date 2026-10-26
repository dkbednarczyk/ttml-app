package ui

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"ttml-app/core"
)

const (
	// previewScrollTime is how long the view glides to a newly started line.
	previewScrollTime = 400 * time.Millisecond
	// breakText stands in for a line with no text, an instrumental break.
	breakText = "•••"
)

// preview shows the lyrics the way a music app's lyric view does: the lines
// playing now in full colour, the rest greyed out, with the latest line to
// start kept at the top. If the line before it is still playing (a held word,
// a second singer), that one stays at the top so both are in view. While it
// plays only colours change, so nothing shifts; the view glides to each new
// line instead of jumping. Clicking a line skips the song to its start.
type preview struct {
	lines   *fyne.Container // one previewLine per line
	spacer  *canvas.Rectangle
	scroll  *container.Scroll
	current int // the line held at the top; -1 for none
	anim    *fyne.Animation
	seek    func(core.Ms)
}

func newPreview(seek func(core.Ms)) *preview {
	p := &preview{
		lines:   container.NewVBox(),
		spacer:  canvas.NewRectangle(color.Transparent),
		current: -1,
		seek:    seek,
	}
	// The spacer below the lines lets the last one scroll up to the top.
	p.scroll = container.NewVScroll(container.NewPadded(container.NewVBox(p.lines, p.spacer)))

	return p
}

// show brings the view in line with the project's lines and the playback
// position. With animate false (the tab just opened, say) it moves straight
// there. With several singers, as in a music app, the second singer's lines
// (and the fourth's, and so on) sit on the right.
func (p *preview) show(project *core.Project, at core.Ms, animate bool) {
	lines := project.Lines
	duet := project.HasSingers()
	for i := len(p.lines.Objects); i < len(lines); i++ {
		p.lines.Add(newPreviewLine(p.seek))
	}
	if len(p.lines.Objects) > len(lines) {
		p.lines.Objects = p.lines.Objects[:len(lines)]
		p.lines.Refresh()
	}

	top, before := -1, -1 // the latest line to start, and the one started before it
	for i, line := range lines {
		label := p.lines.Objects[i].(*previewLine)
		label.begin = line.Begin
		text := line.Text
		if text == "" && line.Background == "" {
			text = breakText
		}
		importance := widget.LowImportance
		if line.Begin != nil && *line.Begin <= at {
			top, before = i, top // starts are in order, so the last one started wins
			if playing(line.Timing, at) {
				importance = widget.MediumImportance
			}
		}
		align := fyne.TextAlignLeading
		if duet && line.Singer%2 == 1 {
			align = fyne.TextAlignTrailing
		}
		bgImportance := widget.LowImportance
		if playing(line.BackgroundTimes(), at) {
			bgImportance = widget.MediumImportance
		}
		label.set(label.main, text, importance, align)
		label.set(label.bg, line.Background, bgImportance, align)
	}

	if height := p.scroll.Size().Height; p.spacer.MinSize().Height != height {
		p.spacer.SetMinSize(fyne.NewSize(0, height))
		p.scroll.Refresh()
	}
	if before >= 0 && playing(lines[before].Timing, at) {
		top = before
	}
	if top != p.current || !animate {
		p.current = top
		p.scrollTo(top, animate)
	}
}

func playing(t core.Timing, at core.Ms) bool {
	return t.Begin != nil && t.End != nil && *t.Begin <= at && at < *t.End
}

// scrollTo puts line i at the top of the view (the very top for -1).
func (p *preview) scrollTo(i int, animate bool) {
	var y float32
	if i >= 0 && i < len(p.lines.Objects) {
		y = p.lines.Objects[i].Position().Y
	}
	maxY := p.scroll.Content.MinSize().Height - p.scroll.Size().Height
	y = max(0, min(y, maxY))

	if p.anim != nil {
		p.anim.Stop()
		p.anim = nil
	}
	if !animate {
		p.scroll.ScrollToOffset(fyne.NewPos(0, y))
		p.scroll.Refresh()

		return
	}
	from := p.scroll.Offset.Y
	p.anim = fyne.NewAnimation(previewScrollTime, func(done float32) {
		p.scroll.ScrollToOffset(fyne.NewPos(0, from+(y-from)*done))
		p.scroll.Refresh()
	})
	p.anim.Curve = fyne.AnimationEaseInOut
	p.anim.Start()
}

// previewLine is one line of the preview, with its background vocal in
// smaller type under it. Clicking it skips the song to the line's start; an
// untimed line has nowhere to go and ignores clicks.
type previewLine struct {
	widget.BaseWidget
	main, bg *widget.Label
	begin    *core.Ms
	seek     func(core.Ms)
}

func newPreviewLine(seek func(core.Ms)) *previewLine {
	l := &previewLine{seek: seek, main: widget.NewLabel(""), bg: widget.NewLabel("")}
	l.ExtendBaseWidget(l)
	l.main.SizeName = theme.SizeNameHeadingText
	l.main.TextStyle = fyne.TextStyle{Bold: true}
	l.main.Wrapping = fyne.TextWrapWord
	l.bg.SizeName = theme.SizeNameSubHeadingText
	l.bg.Wrapping = fyne.TextWrapWord
	l.bg.Hide()

	return l
}

func (l *previewLine) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewVBox(l.main, l.bg))
}

// set shows text in one of the line's labels, which is hidden while empty.
func (*previewLine) set(label *widget.Label, text string, importance widget.Importance, align fyne.TextAlign) {
	if label.Text == text && label.Importance == importance && label.Alignment == align {
		return
	}
	label.Text = text
	label.Importance = importance
	label.Alignment = align
	label.Refresh()
	if text == "" {
		label.Hide()
	} else {
		label.Show()
	}
}

func (l *previewLine) Tapped(*fyne.PointEvent) {
	if l.begin != nil {
		l.seek(*l.begin)
	}
}

func (l *previewLine) Cursor() desktop.Cursor {
	if l.begin == nil {
		return desktop.DefaultCursor
	}

	return desktop.PointerCursor
}
