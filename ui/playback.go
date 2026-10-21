package ui

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"

	"ttml-app/audio"
	"ttml-app/core"
)

const playbackTick = 100 * time.Millisecond

// speeds are the playback speeds offered, slowest first.
var speeds = []float64{0.5, 0.75, 1}

type audioPlayer interface {
	SetSpeed(speed float64)
	Play()
	Pause()
	Seek(position time.Duration) error
	Position() time.Duration
	Duration() time.Duration
	Close() error
}

func openAudio(path string) (audioPlayer, error) {
	player, err := audio.Open(path)
	if err != nil {
		return nil, err
	}

	return player, nil
}

func (m *mainWindow) transport() *fyne.Container {
	left := container.NewHBox(m.statusButton, m.playButton, m.startButton)
	bar := container.NewBorder(nil, nil, left, container.NewHBox(m.timeLabel, m.speedButton), m.slider)

	return container.NewPadded(bar)
}

func (m *mainWindow) sliderChanged(value float64) {
	m.position = m.clampPosition(time.Duration(value * float64(time.Second)))
	if m.player != nil {
		if err := m.player.Seek(m.position); err != nil {
			fyne.LogError("seek audio", err)
		}
	}
	m.updateTimeLabel()
	m.updatePreview(true)
	if m.position >= m.duration {
		m.pause()
	}
}

func (m *mainWindow) setUIPosition(position time.Duration) {
	m.position = m.clampPosition(position)
	m.slider.Value = m.position.Seconds()
	m.slider.Refresh()
	m.updateTimeLabel()
	m.updatePreview(true)
}

// skipToStart goes back to the beginning of the song, playing or paused as it was.
func (m *mainWindow) skipToStart() {
	m.seekTo(0)
}

// cycleSpeed is the speed button: 1×, then slower, then back to 1×.
func (m *mainWindow) cycleSpeed() {
	i := slices.Index(speeds, m.speed)
	m.setSpeed(speeds[(i+len(speeds)-1)%len(speeds)])
}

// stepSpeed moves one speed slower (-1) or faster (+1), stopping at the ends.
func (m *mainWindow) stepSpeed(step int) {
	i := slices.Index(speeds, m.speed) + step
	m.setSpeed(speeds[max(0, min(i, len(speeds)-1))])
}

func (m *mainWindow) setSpeed(speed float64) {
	m.speed = speed
	m.speedButton.SetText(formatSpeed(speed))
	if m.player != nil {
		m.player.SetSpeed(speed)
	}
}

func formatSpeed(speed float64) string {
	return strconv.FormatFloat(speed, 'f', -1, 64) + "×"
}

// seekTo skips the song to a lyric's start, keeping it playing or paused as it was.
func (m *mainWindow) seekTo(at core.Ms) {
	if m.player == nil {
		return
	}
	position := m.clampPosition(time.Duration(at) * time.Millisecond)
	if err := m.player.Seek(position); err != nil {
		fyne.LogError("seek to lyric", err)

		return
	}
	m.setUIPosition(position)
}

// updatePreview shows the playback position in the Preview tab.
func (m *mainWindow) updatePreview(animate bool) {
	m.preview.show(m.history.Project().Lines, core.Ms(m.position.Milliseconds()), animate)
}

func (m *mainWindow) clampPosition(position time.Duration) time.Duration {
	if position < 0 {
		return 0
	}
	if position > m.duration {
		return m.duration
	}

	return position
}

func (m *mainWindow) setDuration(duration time.Duration) {
	m.duration = duration
	m.position = 0
	m.slider.Max = duration.Seconds()
	m.slider.Value = 0
	m.slider.Refresh()
	m.slider.Enable()
	m.playButton.Enable()
	m.updateTimeLabel()
	m.updatePreview(false)
	m.updateStatus() // the song's length and tags feed the checks
}

func (m *mainWindow) togglePlayback() {
	if m.playing {
		m.pause()

		return
	}
	if err := m.play(); err != nil {
		dialog.ShowError(err, m.window)
	}
}

func (m *mainWindow) play() error {
	if m.playing || m.player == nil || m.duration <= 0 {
		return nil
	}
	if m.position >= m.duration {
		if err := m.player.Seek(0); err != nil {
			return fmt.Errorf("rewind audio: %w", err)
		}
		m.setUIPosition(0)
	}
	m.player.Play()

	m.playing = true
	m.playButton.SetText("Pause")
	m.playButton.SetIcon(theme.MediaPauseIcon())
	m.updateButtons()

	ctx, cancel := context.WithCancel(context.Background())
	m.clockCancel = cancel

	go m.clock(ctx)

	return nil
}

func (m *mainWindow) pause() {
	if m.player != nil {
		m.player.Pause()
		if m.playing {
			m.setUIPosition(m.player.Position())
		}
	}
	if m.clockCancel != nil {
		m.clockCancel()
		m.clockCancel = nil
	}
	if !m.playing {
		return
	}

	m.playing = false
	m.playButton.SetText("Play")
	m.playButton.SetIcon(theme.MediaPlayIcon())
	m.updateButtons()
}

func (m *mainWindow) close() {
	m.closePlayer()
}

func (m *mainWindow) closePlayer() {
	m.pause()
	if m.player == nil {
		return
	}
	if err := m.player.Close(); err != nil {
		fyne.LogError("close audio", err)
	}
	m.player = nil
}

func (m *mainWindow) runClock(ctx context.Context) {
	ticker := time.NewTicker(playbackTick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fyne.Do(func() {
				m.syncPosition()
			})
		}
	}
}

func (m *mainWindow) syncPosition() {
	if m.player == nil || !m.playing {
		return
	}

	position := m.player.Position()
	m.setUIPosition(position)
	if position >= m.duration {
		m.pause()
	}
}

func (m *mainWindow) updateTimeLabel() {
	m.timeLabel.SetText(formatTime(m.position) + " / " + formatTime(m.duration))
}

func formatTime(d time.Duration) string {
	if d < 0 {
		d = 0
	}

	total := int64(d / time.Second)
	hours := total / 3600
	minutes := (total % 3600) / 60
	seconds := total % 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}

	return fmt.Sprintf("%d:%02d", minutes, seconds)
}
