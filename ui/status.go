package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"ttml-app/core"
	"ttml-app/formats"
)

var (
	statusOK      = theme.NewSuccessThemedResource(theme.ConfirmIcon())
	statusWarning = theme.NewWarningThemedResource(theme.WarningIcon())
	statusError   = theme.NewErrorThemedResource(theme.CancelIcon())
)

// lyricsStatus is what the status icon reports: problems block a save,
// warnings don't.
type lyricsStatus struct {
	problems []formats.Problem
	warnings []string
}

func (m *mainWindow) checkLyrics() lyricsStatus {
	p := m.history.Project()
	var s lyricsStatus
	s.problems = formats.Validate(p, m.songEnd())
	for _, w := range formats.Warnings(p) {
		s.warnings = append(s.warnings, w.String())
	}
	if m.player != nil {
		s.warnings = append(s.warnings, core.Mismatches(p, m.ttmlDur, m.songEnd(), m.audioTags.ISRC)...)
	}

	return s
}

// updateStatus sets the icon left of Play: a red ✗ if Save would refuse the
// lyrics, a yellow warning if they save but something looks wrong, a green
// check otherwise. It is hidden while there are no lines.
func (m *mainWindow) updateStatus() {
	if shown := len(m.history.Project().Lines) > 0; shown != m.statusButton.Visible() {
		if shown {
			m.statusButton.Show()
		} else {
			m.statusButton.Hide()
		}
		m.transportBar.Refresh() // lay the row out again so Play moves over
	}
	if !m.statusButton.Visible() {
		return
	}
	s := m.checkLyrics()
	switch {
	case len(s.problems) > 0:
		m.statusButton.SetIcon(statusError)
	case len(s.warnings) > 0:
		m.statusButton.SetIcon(statusWarning)
	default:
		m.statusButton.SetIcon(statusOK)
	}
}

// showStatus lists what is behind the status icon.
func (m *mainWindow) showStatus() {
	s := m.checkLyrics()
	title := "Ready to save"
	content := container.NewVBox()
	switch {
	case len(s.problems) > 0:
		title = "Can't save yet"
	case len(s.warnings) > 0:
		title = "Saves, but check these"
	default:
		content.Add(widget.NewLabel("Every line has its times, in order, within the song."))
	}
	add := func(heading string, items []string) {
		if len(items) == 0 {
			return
		}
		content.Add(widget.NewLabel(heading))
		shown := items[:min(len(items), maxProblemsShown)]
		for _, item := range shown {
			content.Add(widget.NewLabelWithStyle(item, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		}
		if more := len(items) - len(shown); more > 0 {
			content.Add(widget.NewLabel(fmt.Sprintf("…and %d more", more)))
		}
	}
	problems := make([]string, len(s.problems))
	for i, p := range s.problems {
		problems[i] = p.String()
	}
	add("Fix these to save:", problems)
	add("Worth a look:", s.warnings)
	dialog.ShowCustom(title, "OK", content, m.window)
}
