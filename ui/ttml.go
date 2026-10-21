package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"ttml-app/core"
	"ttml-app/formats"
)

// chooseTTML asks before replacing unsaved lines, then opens a file picker for TTML.
func (m *mainWindow) chooseTTML() {
	m.confirmDiscard("Import", m.showTTMLPicker)
}

func (m *mainWindow) showTTMLPicker() {
	d := dialog.NewFileOpen(m.chosenFile(m.importTTML), m.window)
	d.SetTitleText("Import TTML")
	d.SetFilter(storage.NewExtensionFileFilter([]string{".ttml"}))
	if m.audioPath != "" {
		if dir, err := storage.ListerForURI(storage.NewFileURI(filepath.Dir(m.audioPath))); err == nil {
			d.SetLocation(dir)
		}
	}
	d.Show()
}

// importTTML replaces the lines with those in a TTML file. Audio stays as it
// is: without audio, the editor shows the lines with nothing that needs it.
func (m *mainWindow) importTTML(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("import TTML: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			fyne.LogError("close TTML file", err)
		}
	}()
	imported, err := formats.ReadTTML(file)
	if err != nil {
		return fmt.Errorf("import %s: %w", filepath.Base(path), err)
	}

	m.history = core.NewHistory(imported.Project)
	m.ttmlPath = path
	m.ttmlDur = imported.Duration
	// Save overwrites the imported file, unless that would lose what was left out.
	m.savePath = path
	if len(imported.Dropped) > 0 {
		m.savePath = ""
	}
	if m.player == nil {
		m.showTrack(filepath.Base(path), "", nil)
	}
	m.refreshLyrics()
	m.lyricsScroll.ScrollToTop()
	if len(imported.Dropped) > 0 {
		m.showDropped(len(imported.Project.Lines), imported.Dropped)
	}
	if m.player != nil {
		m.checkMatch()
	}

	return nil
}

// checkMatch warns, without blocking anything, when the imported lyrics look
// like they belong to a different recording than the loaded audio.
func (m *mainWindow) checkMatch() {
	reasons := core.Mismatches(m.history.Project(), m.ttmlDur, m.songEnd(), m.audioTags.ISRC)
	if len(reasons) == 0 {
		return
	}
	content := container.NewVBox(widget.NewLabel("These lyrics may be for a different version of the song:"))
	for _, r := range reasons {
		content.Add(widget.NewLabelWithStyle(r, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	}
	content.Add(widget.NewLabel("You can keep working. Save still refuses lines that end after the song."))
	dialog.ShowCustom("Lyrics and audio may not match", "OK", content, m.window)
}

// audioMeta is the loaded audio's tags as metadata entries, for filling in
// what the lyrics lack on save.
func (m *mainWindow) audioMeta() []core.MetaEntry {
	if m.player == nil {
		return nil
	}

	return []core.MetaEntry{
		{Key: "musicName", Value: m.audioTags.Title},
		{Key: "artists", Value: m.audioTags.Artist},
		{Key: "album", Value: m.audioTags.Album},
		{Key: "isrc", Value: m.audioTags.ISRC},
	}
}

// showDropped says what an import left out and that Save won't overwrite the file.
func (m *mainWindow) showDropped(lines int, dropped []string) {
	content := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Imported %d lines. The editor only handles line timing so far,", lines)),
		widget.NewLabel("so this was left out:"),
		widget.NewLabelWithStyle(strings.Join(dropped, ", "), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Save will ask for a file, so the original keeps it."),
	)
	dialog.ShowCustom("Some of the file was left out", "OK", content, m.window)
}

// chosenFile adapts a file picker's result to an action on its local path,
// showing any error.
func (m *mainWindow) chosenFile(open func(path string) error) func(fyne.URIReadCloser, error) {
	return func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, m.window)

			return
		}
		if reader == nil {
			return // the user cancelled
		}
		uri := reader.URI()
		if err := reader.Close(); err != nil {
			fyne.LogError("close chosen file", err)
		}
		if uri.Scheme() != "file" {
			dialog.ShowError(errNotALocalFile, m.window)

			return
		}
		if err := open(uri.Path()); err != nil {
			dialog.ShowError(err, m.window)
		}
	}
}
