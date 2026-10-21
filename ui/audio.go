package ui

import (
	"errors"
	"fmt"
	"image"
	"path/filepath"

	"ttml-app/audio"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
)

var errNotALocalFile = errors.New("selection is not a local file")

// chooseAudio opens a file picker filtered to supported audio formats.
// Replacing loaded audio starts a new song and drops the lines, so that asks
// first about unsaved ones; adding audio to imported lines keeps them.
func (m *mainWindow) chooseAudio() {
	if m.player == nil {
		m.showAudioPicker()

		return
	}
	m.confirmDiscard("Load", m.showAudioPicker)
}

func (m *mainWindow) showAudioPicker() {
	d := dialog.NewFileOpen(m.chosenFile(m.loadAudio), m.window)
	d.SetTitleText("Load audio")
	d.SetFilter(storage.NewExtensionFileFilter(audio.SupportedExtensions()))
	d.Show()
}

// loadAudio opens the selected file and makes it the project's audio source.
// Opening is quick: the player decodes the file as it plays. Lines imported
// without audio are kept; replacing audio starts a new song with no lines.
func (m *mainWindow) loadAudio(path string) error {
	player, err := m.openAudio(path)
	if err != nil {
		return fmt.Errorf("load audio: %w", err)
	}

	newSong := m.player != nil
	if newSong {
		m.closePlayer()
		m.clearLines() // lines belong to the previous song
		m.savePath = ""
		m.ttmlPath = ""
		m.ttmlDur = nil
	}
	m.player = player
	m.player.SetSpeed(m.speed) // the chosen speed carries over to the next song
	m.audioPath = path
	m.audioTags = audio.ReadTags(path)
	m.transportBar.Show()
	m.syncBar.Show()
	title, artist := trackInfo(path, m.audioTags)
	var art image.Image
	if cover := audio.FindCover(path, m.audioTags.Album); cover != nil {
		art = cover.Image
	}
	m.showTrack(title, artist, art)
	m.setDuration(m.player.Duration())
	if m.ttmlPath != "" {
		m.checkMatch() // audio added to imported lyrics
	}

	return nil
}

// trackInfo picks the header text: the tagged title, or the file name without one.
func trackInfo(path string, tags audio.Tags) (title, artist string) {
	title = tags.Title
	if title == "" {
		title = filepath.Base(path)
	}

	return title, tags.Artist
}

// showTrack moves from the centered empty state to the header with the track's details.
func (m *mainWindow) showTrack(title, artist string, art image.Image) {
	m.trackInfo.set(title, artist, art)
	if m.emptyState.Visible() {
		// Leaving the launch screen: grow the window for the editor, keeping
		// any size the user already made larger.
		m.emptyState.Hide()
		size := m.window.Canvas().Size()
		m.window.Resize(fyne.NewSize(max(size.Width, editorW), max(size.Height, editorH)))
	}
	m.header.Show()
	m.tabs.Show()
	m.root.Refresh() // lay out again now that the header takes space
}
