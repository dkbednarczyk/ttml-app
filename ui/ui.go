// Package ui provides the Fyne graphical user interface for the application.
package ui

import (
	"context"
	"time"

	"ttml-app/audio"
	"ttml-app/core"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	appID       = "app.ttml"
	windowTitle = "TTML Lyric Sync"
	windowW     = 640 // the launch screen
	windowH     = 480
	editorW     = 960 // once a file is open; fits a 1366×768 screen
	editorH     = 680
	emptyLabel  = "Load a song, or import or paste lyrics to start"
)

type mainWindow struct {
	window       fyne.Window
	trackInfo    *trackLine
	header       *fyne.Container // track info and file buttons; shown once something is open
	emptyState   *fyne.Container // centered file buttons; shown until something is open
	transportBar *fyne.Container // play and seek controls; shown once audio is loaded
	syncBar      *fyne.Container // the S and E buttons, under the Edit list; shown once audio is loaded
	root         *fyne.Container
	history      core.History
	lyricRows    *fyne.Container // one lyricRow per line
	lyricsScroll *container.Scroll
	preview      *preview
	tabs         *container.AppTabs // Edit and Preview; shown once something is open
	tapButton    *widget.Button
	endButton    *widget.Button
	saveButton   *widget.Button
	audioPath    string
	savePath     string     // the TTML file Save writes; asked for on the first save
	ttmlPath     string     // the imported TTML file, if any
	ttmlDur      *core.Ms   // the song length the imported TTML states, if any
	audioTags    audio.Tags // the loaded audio's tags
	duration     time.Duration
	position     time.Duration
	playing      bool
	clockCancel  context.CancelFunc
	clock        func(context.Context) // moves the seek bar during playback; tests drive it by hand
	slider       *widget.Slider
	timeLabel    *widget.Label
	playButton   *widget.Button
	speedButton  *widget.Button
	startButton  *widget.Button // back to the beginning
	statusButton *widget.Button // whether the lyrics would save; click for details
	statusIcons  statusIcons
	speed        float64 // playback speed; the pitch moves with it
	player       audioPlayer
	openAudio    func(path string) (audioPlayer, error)
}

// NewMainWindow creates and configures the main application window.
func NewMainWindow(a fyne.App) fyne.Window {
	w := a.NewWindow(windowTitle)
	w.SetContent(newMainWindow(w).content())
	w.Resize(fyne.NewSize(windowW, windowH))

	return w
}

func newMainWindow(w fyne.Window) *mainWindow {
	slider := widget.NewSlider(0, 0)
	timeLabel := widget.NewLabel(formatTime(0) + " / " + formatTime(0))
	playButton := widget.NewButtonWithIcon("Play", theme.MediaPlayIcon(), nil)
	trackInfo := newTrackLine()
	headerLoad := widget.NewButton("Load audio", nil)
	headerImport := widget.NewButton("Import TTML", nil)
	emptyLoad := widget.NewButton("Load audio", nil)
	emptyImport := widget.NewButton("Import TTML", nil)
	headerPaste := widget.NewButton("Paste lyrics", nil)
	emptyPaste := widget.NewButton("Paste lyrics", nil)
	saveButton := widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), nil)
	// Lines have one singer until a second is added; then each row can pick.
	addSinger := widget.NewButtonWithIcon("Add singer", theme.ContentAddIcon(), nil)

	buttons := container.NewVBox(container.NewHBox(saveButton, headerLoad, headerImport, headerPaste, addSinger))
	header := container.NewVBox(
		container.NewPadded(container.NewBorder(nil, nil, nil, buttons, trackInfo.Container)),
		widget.NewSeparator(),
	)
	header.Hide()

	m := &mainWindow{
		window:    w,
		trackInfo: trackInfo,
		header:    header,
		emptyState: container.NewCenter(container.NewVBox(
			container.NewCenter(container.NewHBox(emptyLoad, emptyImport, emptyPaste)), // the label is wider
			widget.NewLabelWithStyle(emptyLabel, fyne.TextAlignCenter, fyne.TextStyle{}),
		)),
		audioPath:   "",
		duration:    0,
		position:    0,
		playing:     false,
		clockCancel: nil,
		slider:      slider,
		timeLabel:   timeLabel,
		playButton:  playButton,
		player:      nil,
		openAudio:   openAudio,
		lyricRows:   container.NewVBox(),
		tapButton:   widget.NewButton(startLineLabel, nil),
		endButton:   widget.NewButton(endLineLabel, nil),
		saveButton:  saveButton,
	}
	// Keep the rows clear of the scrollbar, which would cover the insert buttons.
	scrollBar := theme.Size(theme.SizeNameScrollBar)
	m.lyricsScroll = container.NewVScroll(container.New(layout.NewCustomPaddedLayout(0, 0, 0, scrollBar), m.lyricRows))
	m.preview = newPreview(m.seekTo)
	previewTab := container.NewTabItem("Preview", m.preview.scroll)
	m.syncBar = container.NewPadded(container.NewCenter(container.NewHBox(m.tapButton, m.endButton)))
	m.syncBar.Hide()
	editTab := container.NewBorder(nil, m.syncBar, nil, nil, m.lyricsScroll)
	m.tabs = container.NewAppTabs(container.NewTabItem("Edit", editTab), previewTab)
	m.tabs.OnSelected = func(tab *container.TabItem) {
		if tab == previewTab {
			m.updatePreview(false) // just opened: go straight to the current line
		}
	}
	m.tabs.Hide()
	m.clock = m.runClock
	m.speed = 1
	m.speedButton = widget.NewButton(formatSpeed(1), m.cycleSpeed)
	m.startButton = widget.NewButtonWithIcon("", theme.MediaSkipPreviousIcon(), m.skipToStart)
	m.statusIcons = newStatusIcons()
	m.statusButton = widget.NewButtonWithIcon("", m.statusIcons.ok, m.showStatus)
	m.statusButton.Importance = widget.LowImportance
	m.statusButton.Hide()
	m.tapButton.OnTapped = m.tap
	m.endButton.OnTapped = m.endLine
	saveButton.OnTapped = m.save
	m.updateMarks()
	w.Canvas().SetOnTypedKey(m.typedKey)
	m.addShortcuts()
	headerLoad.OnTapped = m.chooseAudio
	emptyLoad.OnTapped = m.chooseAudio
	headerImport.OnTapped = m.chooseTTML
	emptyImport.OnTapped = m.chooseTTML
	headerPaste.OnTapped = m.showPasteDialog
	addSinger.OnTapped = func() { m.showAddSinger(-1) }
	emptyPaste.OnTapped = m.showPasteDialog
	slider.OnChanged = m.sliderChanged
	// A clicked slider keeps focus and would swallow Space, S and E.
	slider.OnChangeEnded = func(float64) { w.Canvas().Unfocus() }
	slider.Disable()
	playButton.OnTapped = m.togglePlayback
	playButton.Disable()
	m.transportBar = m.transport()
	m.transportBar.Hide()
	m.root = container.NewBorder(m.header, m.transportBar, nil, nil, container.NewStack(m.emptyState, m.tabs))
	w.SetCloseIntercept(func() { m.confirmDiscard("Close", w.Close) })
	w.SetOnClosed(m.close)

	return m
}

func (m *mainWindow) content() fyne.CanvasObject {
	return m.root
}

// Run starts the graphical application and blocks until the window is closed.
func Run() {
	configureCursorTheme()

	a := app.NewWithID(appID)
	w := NewMainWindow(a)
	if err := audio.Init(); err != nil {
		dialog.ShowError(err, w)
	}
	defer audio.Close()
	w.ShowAndRun()
}
