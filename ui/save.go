package ui

import (
	"errors"
	"fmt"
	"io"
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

// maxProblemsShown caps the list in the "can't save" dialog.
const maxProblemsShown = 10

// save exports the lines as TTML. Lines with problems block the save and are
// listed. The first save asks where to write; later saves overwrite that file.
func (m *mainWindow) save() {
	if problems := formats.Validate(m.history.Project(), m.songEnd()); len(problems) > 0 {
		m.showProblems(problems)

		return
	}
	if m.savePath == "" {
		m.askSavePath()

		return
	}
	m.saveTo(m.savePath)
}

func (m *mainWindow) songEnd() core.Ms {
	return core.Ms(m.duration.Milliseconds())
}

// askSavePath asks where to save. It suggests <audio name>.ttml beside the
// audio, or, for an import that left content out, a new name beside the
// imported file so the original isn't replaced by accident.
func (m *mainWindow) askSavePath() {
	base := m.audioPath
	name := strings.TrimSuffix(filepath.Base(base), filepath.Ext(base)) + ".ttml"
	if base == "" {
		name = "lyrics.ttml" // pasted lyrics with no file open
	}
	if m.ttmlPath != "" {
		base = m.ttmlPath
		name = strings.TrimSuffix(filepath.Base(base), filepath.Ext(base)) + " (lines).ttml"
	}
	d := dialog.NewFileSave(func(file fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, m.window)

			return
		}
		if file == nil {
			return // the user cancelled
		}
		// Fyne has already created (or emptied) the chosen file. Close it and
		// let saveTo replace it with a temp-file-then-rename write.
		if err := file.Close(); err != nil {
			fyne.LogError("close chosen TTML file", err)
		}
		if file.URI().Scheme() != "file" {
			dialog.ShowError(errNotALocalFile, m.window)

			return
		}
		m.saveTo(file.URI().Path())
	}, m.window)
	d.SetTitleText("Save TTML")
	d.SetFileName(name)
	if dir, err := storage.ListerForURI(storage.NewFileURI(filepath.Dir(base))); base != "" && err == nil {
		d.SetLocation(dir)
	}
	d.SetFilter(storage.NewExtensionFileFilter([]string{".ttml"}))
	d.Show()
}

// saveTo writes the TTML file and, once it worked, remembers path for later
// saves. Metadata the lyrics lack (title, artist, album, ISRC) comes from the
// audio's tags.
func (m *mainWindow) saveTo(path string) {
	export := *m.history.Project()
	export.Meta = core.FillMeta(export.Meta, m.audioMeta())
	err := formats.WriteFileAtomic(path, func(w io.Writer) error {
		return formats.WriteTTML(w, &export, m.songEnd())
	})
	if err != nil {
		var invalid *formats.InvalidError
		if errors.As(err, &invalid) {
			m.showProblems(invalid.Problems)

			return
		}
		dialog.ShowError(fmt.Errorf("save TTML: %w", err), m.window)

		return
	}
	m.savePath = path
	m.history.MarkSaved()
	m.refreshLyrics()
}

func (m *mainWindow) showProblems(problems []formats.Problem) {
	shown := make([]string, 0, maxProblemsShown+1)
	for _, p := range problems[:min(len(problems), maxProblemsShown)] {
		shown = append(shown, p.String())
	}
	if more := len(problems) - maxProblemsShown; more > 0 {
		shown = append(shown, fmt.Sprintf("…and %d more", more))
	}
	// A custom dialog with plain labels: the information dialog wraps its
	// message to a narrow column, which splits each problem across lines.
	list := widget.NewLabel(strings.Join(shown, "\n"))
	list.TextStyle = fyne.TextStyle{Bold: true}
	content := container.NewVBox(widget.NewLabel("Fix these first:"), list)
	dialog.ShowCustom("Can't save yet", "OK", content, m.window)
}
