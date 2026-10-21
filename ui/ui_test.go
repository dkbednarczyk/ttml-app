package ui

import (
	"context"
	"errors"
	"image"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ttml-app/audio"
	"ttml-app/core"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var errTestLoad = errors.New("load failed")

const (
	testAudioPath     = "/music/Song.MP3"
	testAudioDuration = 2 * time.Minute
)

func TestNewMainWindow(t *testing.T) {
	a := test.NewTempApp(t)
	w := NewMainWindow(a)
	if got := w.Title(); got != windowTitle {
		t.Fatalf("Title() = %q, want %q", got, windowTitle)
	}
	markup := test.RenderToMarkup(w.Canvas())
	if strings.Contains(markup, "Hello, world!") {
		t.Fatalf("canvas markup %q still contains the greeting", markup)
	}
	for _, want := range []string{"Load audio", "Import TTML", emptyLabel} {
		if !strings.Contains(markup, want) {
			t.Fatalf("canvas markup %q does not contain %q", markup, want)
		}
	}
}

func TestLoadAudio(t *testing.T) {
	mw := newTestMainWindow(t)
	path := testAudioPath

	if err := mw.loadAudio(path); err != nil {
		t.Fatalf("loadAudio(%q) error: %v", path, err)
	}
	if mw.audioPath != path {
		t.Fatalf("audioPath = %q, want %q", mw.audioPath, path)
	}
	if got := mw.trackInfo.String(); got != "Song.MP3" {
		t.Fatalf("header = %q, want %q", got, "Song.MP3")
	}
	if mw.trackInfo.artist.Visible() {
		t.Fatal("artist label is visible for a file without tags")
	}
	if !mw.header.Visible() || mw.emptyState.Visible() {
		t.Fatal("track header is not shown in place of the empty state after loading")
	}
	if want := 2 * time.Minute; mw.duration != want {
		t.Fatalf("duration = %v, want %v", mw.duration, want)
	}
	if mw.position != 0 {
		t.Fatalf("position = %v, want 0", mw.position)
	}
	if want := 120.0; mw.slider.Max != want {
		t.Fatalf("slider.Max = %v, want %v", mw.slider.Max, want)
	}
	if mw.slider.Disabled() {
		t.Fatal("slider is disabled after loading audio")
	}
	if mw.playButton.Disabled() {
		t.Fatal("play button is disabled after loading audio")
	}
	if got := mw.timeLabel.Text; got != "0:00 / 2:00" {
		t.Fatalf("timeLabel.Text = %q, want %q", got, "0:00 / 2:00")
	}
}

func TestLoadAudioFailureKeepsState(t *testing.T) {
	mw := newTestMainWindow(t)
	mw.openAudio = func(string) (audioPlayer, error) {
		return nil, errTestLoad
	}

	if err := mw.loadAudio(testAudioPath); !errors.Is(err, errTestLoad) {
		t.Fatalf("loadAudio() error = %v, want %v", err, errTestLoad)
	}
	if mw.audioPath != "" {
		t.Fatalf("audioPath = %q after failed load, want empty", mw.audioPath)
	}
	if mw.header.Visible() || !mw.emptyState.Visible() {
		t.Fatal("empty state is not shown after a failed first load")
	}
	if !mw.slider.Disabled() {
		t.Fatal("slider is enabled after a failed load")
	}
}

func TestSeekbarUpdatesTime(t *testing.T) {
	mw := newTestMainWindow(t)
	if err := mw.loadAudio(testAudioPath); err != nil {
		t.Fatalf("loadAudio() error: %v", err)
	}

	mw.slider.SetValue(90)
	if want := 90 * time.Second; mw.position != want {
		t.Fatalf("position = %v, want %v", mw.position, want)
	}
	if got := mw.timeLabel.Text; got != "1:30 / 2:00" {
		t.Fatalf("timeLabel.Text = %q, want %q", got, "1:30 / 2:00")
	}
	if got := fakeFor(t, mw).state().position; got != 90*time.Second {
		t.Fatalf("player position = %v, want %v", got, 90*time.Second)
	}
}

func TestPlayPause(t *testing.T) {
	mw := newTestMainWindow(t)
	if err := mw.loadAudio(testAudioPath); err != nil {
		t.Fatalf("loadAudio() error: %v", err)
	}

	mw.togglePlayback()
	if !mw.playing {
		t.Fatal("playing = false after pressing play")
	}
	if got := mw.playButton.Text; got != "Pause" {
		t.Fatalf("playButton.Text = %q while playing, want %q", got, "Pause")
	}
	if !fakeFor(t, mw).state().playing {
		t.Fatal("player is not playing after pressing play")
	}

	mw.togglePlayback()
	if mw.playing {
		t.Fatal("playing = true after pressing pause")
	}
	if got := mw.playButton.Text; got != "Play" {
		t.Fatalf("playButton.Text = %q while paused, want %q", got, "Play")
	}
	if mw.clockCancel != nil {
		t.Fatal("clockCancel is still set after pausing")
	}
	if fakeFor(t, mw).state().playing {
		t.Fatal("player is still playing after pausing")
	}
}

func TestPlaybackStopsAtEnd(t *testing.T) {
	mw := newTestMainWindow(t)
	if err := mw.loadAudio(testAudioPath); err != nil {
		t.Fatalf("loadAudio() error: %v", err)
	}

	if err := mw.play(); err != nil {
		t.Fatalf("play() error: %v", err)
	}
	fakeFor(t, mw).setPosition(mw.duration)
	mw.syncPosition()
	if want := 2 * time.Minute; mw.position != want {
		t.Fatalf("position = %v, want %v", mw.position, want)
	}
	if mw.playing {
		t.Fatal("playing = true at the end of the track")
	}
	if got := mw.timeLabel.Text; got != "2:00 / 2:00" {
		t.Fatalf("timeLabel.Text = %q, want %q", got, "2:00 / 2:00")
	}
}

func TestTrackInfo(t *testing.T) {
	tests := []struct {
		tags          audio.Tags
		title, artist string
	}{
		{audio.Tags{Title: "Shine On", Artist: "Pink Floyd"}, "Shine On", "Pink Floyd"},
		{audio.Tags{Title: "", Artist: "Pink Floyd"}, "Song.MP3", "Pink Floyd"},
		{audio.Tags{Title: "", Artist: ""}, "Song.MP3", ""},
	}
	for _, tt := range tests {
		title, artist := trackInfo(testAudioPath, tt.tags)
		if title != tt.title || artist != tt.artist {
			t.Errorf("trackInfo(%+v) = %q, %q, want %q, %q", tt.tags, title, artist, tt.title, tt.artist)
		}
	}
}

func TestTrackLineFitsOneLine(t *testing.T) {
	info := newTrackLine()
	info.set("Shine On You Crazy Diamond (Parts I–V)", "Pink Floyd", nil)
	full := textWidth(info.title)

	info.Resize(fyne.NewSize(2000, 36)) // room for everything
	if info.title.Size().Width != full || info.artist.Position().X >= full {
		t.Fatalf("with room: title %v wide at full %v, artist at %v; want the artist right after the title",
			info.title.Size().Width, full, info.artist.Position().X)
	}

	info.Resize(fyne.NewSize(300, 36)) // too narrow: the title gives way first
	if w := info.title.Size().Width; w >= full || w < 150 {
		t.Fatalf("narrow: title %v wide, want cut off but at least half (150)", w)
	}
	if end := info.artist.Position().X + info.artist.Size().Width; end > 300 {
		t.Fatalf("narrow: artist ends at %v, past the 300 available", end)
	}
}

// Fyne cuts off a label's text if it gets exactly its width (it rounds the
// space down), so check what is drawn, not just the sizes.
func TestTrackLineDrawsWholeTextWhenItFits(t *testing.T) {
	mw := newTestMainWindow(t)
	mw.window.Resize(fyne.NewSize(editorW, editorH))
	mw.pasteLyrics("x")
	mw.showTrack("Say It With Ya Chest", "Amarionette", nil)

	markup := test.RenderObjectToMarkup(mw.trackInfo.Container)
	for _, want := range []string{">Say It With Ya Chest<", ">— Amarionette<"} {
		if !strings.Contains(markup, want) {
			t.Errorf("header does not draw %s in full:\n%s", want, markup)
		}
	}
}

func TestTrackLineDashSpacing(t *testing.T) {
	info := newTrackLine()
	info.set("Say It With Ya Chest", "Amarionette", nil)
	info.Resize(fyne.NewSize(900, 36))

	pad := theme.Size(theme.SizeNameInnerPadding)
	textSize := theme.Size(theme.SizeNameText)
	titleEnd := pad + fyne.MeasureText(info.title.Text, textSize, info.title.TextStyle).Width
	dashStart := info.artist.Position().X + pad
	space := fyne.MeasureText(" ", textSize, fyne.TextStyle{}).Width
	// After the dash comes a space character, so the gap before it must be one space wide too.
	if gap := dashStart - titleEnd; math.Abs(float64(gap-space)) > 0.01 {
		t.Fatalf("gap before the dash is %.2f, want one space (%.2f) like the gap after it", gap, space)
	}
}

func TestTrackLineWithArt(t *testing.T) {
	info := newTrackLine()
	art := image.NewRGBA(image.Rect(0, 0, 300, 300))
	info.set("Say It With Ya Chest", "Amarionette", art)
	info.Resize(fyne.NewSize(600, info.MinSize().Height))

	side := info.art.Size()
	if !info.art.Visible() || side.Width != side.Height || side.Height != info.MinSize().Height {
		t.Fatalf("art visible %t, size %v; want a square as tall as the header (%v)", info.art.Visible(), side, info.MinSize().Height)
	}
	if info.artist.Text != "Amarionette" {
		t.Errorf("stacked artist reads %q, want it without the dash", info.artist.Text)
	}
	titleMid := info.title.Position().Y + info.title.Size().Height/2
	artistMid := info.artist.Position().Y + info.artist.Size().Height/2
	if titleMid != side.Height/4 || artistMid != side.Height*3/4 || info.title.Position().X != side.Width {
		t.Errorf("title centred at %v and artist at %v beside the art; want %v and %v at x=%v",
			titleMid, artistMid, side.Height/4, side.Height*3/4, side.Width)
	}
	if got := info.String(); got != "Say It With Ya Chest — Amarionette" {
		t.Errorf("String() = %q", got)
	}

	info.set("Untitled lyrics", "", nil) // art goes away again
	if info.art.Visible() || info.artist.Visible() {
		t.Error("art or artist still shown for a track with neither")
	}
}

func TestLoadAudioFindsArt(t *testing.T) {
	const song = "../example/Say It With Ya Chest.flac"
	if _, err := os.Stat(song); err != nil {
		t.Skip("example song not present")
	}
	mw := newTestMainWindow(t)
	mw.openAudio = openAudio // the real decoder, for the real tags and art
	if err := mw.loadAudio(song); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mw.closePlayer)
	if !mw.trackInfo.art.Visible() || mw.trackInfo.art.Image == nil {
		t.Fatal("the example song's embedded art is not shown")
	}
	if got := mw.trackInfo.String(); got != "Say It With Ya Chest — Amarionette" {
		t.Errorf("header = %q", got)
	}
}

func TestShowTrack(t *testing.T) {
	mw := newTestMainWindow(t)

	mw.showTrack("Shine On", "Pink Floyd", nil)
	if got := mw.trackInfo.String(); got != "Shine On — Pink Floyd" {
		t.Fatalf("header = %q, want the title and artist on one line", got)
	}
}

func TestMarkKeys(t *testing.T) {
	mw := newPlayingTestWindow(t)
	fake := fakeFor(t, mw)

	fake.setPosition(time.Second)
	pressKey(mw, fyne.KeyS)
	if got := mw.tapButton.Text; got != nextLineLabel {
		t.Fatalf("tap button = %q with a line open, want %q", got, nextLineLabel)
	}
	if mw.endButton.Disabled() {
		t.Fatal("end button is disabled with a line open")
	}
	fake.setPosition(2 * time.Second)
	pressKey(mw, fyne.KeyS)
	fake.setPosition(2500 * time.Millisecond)
	pressKey(mw, fyne.KeyE)

	if got := mw.tapButton.Text; got != startLineLabel {
		t.Fatalf("tap button = %q after ending a line, want %q", got, startLineLabel)
	}
	if !mw.endButton.Disabled() {
		t.Fatal("end button is enabled with no line open")
	}
	checkRows(t, mw, []string{"  1 0:01.000 – 0:02.000", "  2 0:02.000 – 0:02.500"})
}

func TestHoverBoxDeletesLine(t *testing.T) {
	mw := newPlayingTestWindow(t)
	fake := fakeFor(t, mw)
	for _, at := range []time.Duration{time.Second, 2 * time.Second, 3 * time.Second} {
		fake.setPosition(at)
		pressKey(mw, fyne.KeyS)
	}
	fake.setPosition(4 * time.Second)
	pressKey(mw, fyne.KeyE)

	box := mw.lyricRows.Objects[1].(*lyricRow).box
	if box.trash.Visible() {
		t.Fatal("trash icon shows without hover")
	}
	box.MouseIn(&desktop.MouseEvent{})
	if !box.trash.Visible() {
		t.Fatal("hovering the lyric box did not show the trash icon")
	}
	// It sits at the box's right end, nearly as tall as the box.
	inset := theme.Size(theme.SizeNameInnerPadding) / 2
	if got, want := box.trash.Size(), fyne.NewSquareSize(box.Size().Height-2*inset); got != want {
		t.Errorf("trash icon is %v, want a square as tall as the %v box (less a %v inset)", got, box.Size(), inset)
	}
	if right := box.trash.Position().X + box.trash.Size().Width; right != box.Size().Width-inset {
		t.Errorf("trash icon ends at x=%v, want the box's right end (%v)", right, box.Size().Width-inset)
	}
	// Moving onto the icon keeps it up: the box gets MouseOut, the icon MouseIn.
	box.MouseOut()
	box.trash.MouseIn(&desktop.MouseEvent{})
	if !box.trash.Visible() {
		t.Fatal("trash icon hid as the pointer reached it")
	}

	test.Tap(box.trash)
	// Line 2 is gone; lines 1 and 3 keep their times, so a gap stays at 0:02–0:03.
	checkRows(t, mw, []string{"  1 0:01.000 – 0:02.000", "  2 0:03.000 – 0:04.000"})

	box.trash.MouseOut()
	if box.trash.Visible() {
		t.Fatal("trash icon still shows after the pointer left")
	}
	typeShortcut(t, mw, &fyne.ShortcutUndo{})
	checkRows(t, mw, []string{"  1 0:01.000 – 0:02.000", "  2 0:02.000 – 0:03.000", "  3 0:03.000 – 0:04.000"})
}

func TestLyricBoxIsRoomier(t *testing.T) {
	mw := newMarkedTestWindow(t)
	box := mw.lyricRows.Objects[0].(*lyricRow).box
	plain := widget.NewEntry()
	plain.SetPlaceHolder("Lyric text")
	if box.MinSize().Height <= plain.MinSize().Height {
		t.Fatalf("lyric box is %v tall, want taller than a plain entry (%v)", box.MinSize().Height, plain.MinSize().Height)
	}
}

func TestDoubleClickLyricSeeks(t *testing.T) {
	mw := newMarkedTestWindow(t) // lines at 0:01 and 0:02, playing
	text := mw.lyricRows.Objects[1].(*lyricRow).text
	mw.window.Canvas().Focus(text) // the first click of a double-click focuses the box

	test.DoubleTap(text)
	if got := fakeFor(t, mw).state().position; got != 2*time.Second || mw.position != 2*time.Second {
		t.Fatalf("after double-clicking line 2: player at %v, UI at %v; want 0:02", got, mw.position)
	}
	if mw.window.Canvas().Focused() != nil {
		t.Fatal("the lyric box kept focus, so S, E and Space would type into it")
	}

	// An untimed line has nowhere to go: a double-click selects a word as usual.
	mw.insertLine(2)
	untimed := mw.lyricRows.Objects[2].(*lyricRow).text
	untimed.SetText("hello world")
	test.DoubleTap(untimed)
	if mw.position != 2*time.Second || untimed.SelectedText() == "" {
		t.Fatalf("double-click on an untimed line: position %v, selection %q; want no seek and a selected word",
			mw.position, untimed.SelectedText())
	}
}

func TestDeletingKeepsTextWithItsLine(t *testing.T) {
	mw := newPlayingTestWindow(t)
	fake := fakeFor(t, mw)
	for _, at := range []time.Duration{time.Second, 2 * time.Second} {
		fake.setPosition(at)
		pressKey(mw, fyne.KeyS)
	}
	mw.lyricRows.Objects[0].(*lyricRow).text.SetText("first")
	mw.lyricRows.Objects[1].(*lyricRow).text.SetText("second")

	test.Tap(mw.lyricRows.Objects[0].(*lyricRow).box.trash)
	row := mw.lyricRows.Objects[0].(*lyricRow)
	if row.text.Text != "second" {
		t.Fatalf("first row shows %q after deleting line 1, want %q", row.text.Text, "second")
	}
	if got := mw.history.Project().Lines[0].Text; got != "second" {
		t.Fatalf("line 1 text = %q, want %q; refreshing the box must not edit the model", got, "second")
	}
}

func TestEditTime(t *testing.T) {
	mw := newMarkedTestWindow(t)
	field := mw.lyricRows.Objects[0].(*lyricRow).end

	test.Tap(field)
	if !field.editing || field.entry.Hidden || mw.window.Canvas().Focused() != field.entry {
		t.Fatal("tapping a time did not open a focused box")
	}
	if got := field.entry.SelectedText(); got != "0:02.000" {
		t.Fatalf("selected text = %q, want the whole time %q", got, "0:02.000")
	}

	field.entry.SetText("0:02.400")
	field.entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if field.editing {
		t.Fatal("box is still open after a valid time")
	}
	// Only that time moved: line 2 still starts at 0:02.000, overlapping line 1.
	checkRows(t, mw, []string{"  1 0:01.000 – 0:02.400", "  2 0:02.000 – 0:03.000"})

	typeShortcut(t, mw, &fyne.ShortcutUndo{})
	checkRows(t, mw, []string{"  1 0:01.000 – 0:02.000", "  2 0:02.000 – 0:03.000"})
}

func TestEditTimeBlocksInvalidInput(t *testing.T) {
	tests := []struct {
		name, text, reason string
		row                int
		end                bool
	}{
		{"bad format", "0:2.4", "not a valid time", 0, true},
		{"three fraction digits", "0:02.40", "not a valid time", 0, true},
		{"end before start", "0:00.500", "the end must be after the line's start", 0, true},
		{"start before previous start", "0:00.900", "lines must start in order", 1, false},
		{"past the song end", "2:00.001", "the time is past the end of the song", 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := newMarkedTestWindow(t)
			row := mw.lyricRows.Objects[tt.row].(*lyricRow)
			field := row.begin
			if tt.end {
				field = row.end
			}
			old := field.label.Text

			test.Tap(field)
			field.entry.SetText(tt.text)
			field.entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
			if !field.editing {
				t.Fatal("box closed on an invalid time")
			}
			if row.reason.Hidden || !strings.HasPrefix(row.reason.Text, tt.reason) {
				t.Fatalf("reason = %q (hidden %t), want it to start with %q", row.reason.Text, row.reason.Hidden, tt.reason)
			}

			// Typing clears the reason; Esc restores the old time.
			field.entry.TypedRune('0')
			if !row.reason.Hidden {
				t.Fatal("reason still shown after typing")
			}
			field.entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
			if field.editing || field.label.Text != old {
				t.Fatalf("after Esc: editing %t, label %q; want closed showing %q", field.editing, field.label.Text, old)
			}
			if mw.history.Dirty() {
				t.Fatal("a refused time changed the project")
			}
		})
	}
}

func TestEditTimeCancelsOnClickAway(t *testing.T) {
	mw := newMarkedTestWindow(t)
	field := mw.lyricRows.Objects[1].(*lyricRow).begin

	test.Tap(field)
	field.entry.SetText("0:02.500")
	mw.window.Canvas().Unfocus() // clicking elsewhere
	if field.editing || field.label.Text != "0:02.000" {
		t.Fatalf("after clicking away: editing %t, label %q; want closed showing %q", field.editing, field.label.Text, "0:02.000")
	}
}

func TestEditOpenEndClosesLine(t *testing.T) {
	mw := newPlayingTestWindow(t)
	fakeFor(t, mw).setPosition(time.Second)
	pressKey(mw, fyne.KeyS)
	field := mw.lyricRows.Objects[0].(*lyricRow).end

	test.Tap(field)
	if field.entry.Text != "" {
		t.Fatalf("open end starts with %q in the box, want it empty", field.entry.Text)
	}
	field.entry.SetText("0:04.000")
	field.entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if mw.history.Project().OpenLine() != -1 || mw.tapButton.Text != startLineLabel {
		t.Fatal("setting the open line's end did not close it")
	}
}

func TestInsertLineAboveAndBelow(t *testing.T) {
	mw := newMarkedTestWindow(t)
	row := mw.lyricRows.Objects[1].(*lyricRow)

	test.Tap(row.inserts.Objects[1].(*widget.Button)) // below line 2
	checkRows(t, mw, []string{"  1 0:01.000 – 0:02.000", "  2 0:02.000 – 0:03.000", "  3 -:--.--- – -:--.---"})
	newText := mw.lyricRows.Objects[2].(*lyricRow).text
	if mw.window.Canvas().Focused() != newText {
		t.Fatal("the inserted line's text box does not have focus")
	}

	test.Tap(row.inserts.Objects[0].(*widget.Button)) // above line 2
	checkRows(t, mw, []string{
		"  1 0:01.000 – 0:02.000", "  2 -:--.--- – -:--.---", "  3 0:02.000 – 0:03.000", "  4 -:--.--- – -:--.---",
	})
	if !mw.history.Dirty() {
		t.Fatal("inserting lines did not count as an unsaved change")
	}

	typeShortcut(t, mw, &fyne.ShortcutUndo{})
	typeShortcut(t, mw, &fyne.ShortcutUndo{})
	checkRows(t, mw, []string{"  1 0:01.000 – 0:02.000", "  2 0:02.000 – 0:03.000"})
}

func TestTimeAnInsertedLine(t *testing.T) {
	mw := newMarkedTestWindow(t)
	mw.insertLine(1)
	field := mw.lyricRows.Objects[1].(*lyricRow).begin

	test.Tap(field)
	field.entry.SetText("0:01.500")
	field.entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if field.editing {
		t.Fatal("a start between the neighbors' starts was refused")
	}
	checkRows(t, mw, []string{"  1 0:01.000 – 0:02.000", "  2 0:01.500 – -:--.---", "  3 0:02.000 – 0:03.000"})
}

const testTTML = `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:itunes="http://music.apple.com/lyric-ttml-internal"` +
	` itunes:timing="Line" xml:lang="en"><body><div>` +
	`<p begin="00:00:00.000" end="00:00:27.044"></p>` +
	`<p begin="00:00:27.054" end="00:00:32.570">We've seen this place before</p>` +
	`<p begin="00:00:32.570" end="00:00:36.902">A room that's void of doors</p></div></body></tt>`

func writeTTML(t *testing.T, doc string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "Say It.ttml")
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestImportTTMLWithoutAudio(t *testing.T) {
	mw := newTestMainWindow(t)
	path := writeTTML(t, testTTML)

	if err := mw.importTTML(path); err != nil {
		t.Fatalf("importTTML() error: %v", err)
	}
	checkRows(t, mw, []string{"  1 0:00.000 – 0:27.044", "  2 0:27.054 – 0:32.570", "  3 0:32.570 – 0:36.902"})
	if got := mw.lyricRows.Objects[1].(*lyricRow).text.Text; got != "We've seen this place before" {
		t.Errorf("line 2 text = %q", got)
	}
	if mw.transportBar.Visible() {
		t.Fatal("play and mark controls show without audio")
	}
	if !mw.header.Visible() || mw.emptyState.Visible() || mw.trackInfo.String() != "Say It.ttml" {
		t.Fatalf("header visible %t, empty state visible %t, title %q; want the header titled with the file",
			mw.header.Visible(), mw.emptyState.Visible(), mw.trackInfo.String())
	}
	if mw.history.Dirty() || mw.savePath != path {
		t.Fatalf("after import: dirty %t, savePath %q; want clean, saving back to %q", mw.history.Dirty(), mw.savePath, path)
	}

	// Lines can be edited and saved without audio; S, E and Space do nothing.
	pressKey(mw, fyne.KeyS)
	pressKey(mw, fyne.KeySpace)
	if got := len(mw.history.Project().Lines); got != 3 || mw.playing {
		t.Fatalf("keys without audio: %d lines, playing %t; want 3 lines, not playing", got, mw.playing)
	}
	mw.lyricRows.Objects[0].(*lyricRow).text.SetText("(intro)")
	mw.save()
	if written, _ := os.ReadFile(path); mw.history.Dirty() || !strings.Contains(string(written), ">(intro)</p>") ||
		strings.Contains(string(written), "dur=") {
		t.Fatalf("save without audio: dirty %t, file:\n%s", mw.history.Dirty(), written)
	}
}

func TestLoadAudioAfterTTMLKeepsLines(t *testing.T) {
	mw := newTestMainWindow(t)
	path := writeTTML(t, testTTML)
	if err := mw.importTTML(path); err != nil {
		t.Fatalf("importTTML() error: %v", err)
	}
	mw.lyricRows.Objects[0].(*lyricRow).text.SetText("(intro)") // unsaved, but adding audio keeps it

	mw.chooseAudio()
	if overlay := mw.window.Canvas().Overlays().Top(); overlay != nil &&
		strings.Contains(test.RenderObjectToMarkup(overlay), "Unsaved lines") {
		t.Fatal("adding audio to imported lines asked about discarding them")
	}
	clearOverlays(mw)

	if err := mw.loadAudio(testAudioPath); err != nil {
		t.Fatalf("loadAudio() error: %v", err)
	}
	if got := len(mw.history.Project().Lines); got != 3 || mw.history.Project().Lines[0].Text != "(intro)" {
		t.Fatalf("adding audio changed the lines: %d lines, first %q", got, mw.history.Project().Lines[0].Text)
	}
	if !mw.transportBar.Visible() || mw.trackInfo.String() != "Song.MP3" || mw.savePath != path {
		t.Fatalf("after adding audio: controls visible %t, title %q, savePath %q",
			mw.transportBar.Visible(), mw.trackInfo.String(), mw.savePath)
	}

	// A second audio file is a new song: its lines go (after asking).
	mw.chooseAudio()
	if overlay := mw.window.Canvas().Overlays().Top(); overlay == nil ||
		!strings.Contains(test.RenderObjectToMarkup(overlay), "Unsaved lines") {
		t.Fatal("replacing audio with unsaved lines did not ask first")
	}
	clearOverlays(mw)
	if err := mw.loadAudio(testAudioPath); err != nil {
		t.Fatalf("loadAudio() error: %v", err)
	}
	if got := len(mw.history.Project().Lines); got != 0 || mw.savePath != "" || mw.ttmlPath != "" {
		t.Fatalf("new song kept %d lines, savePath %q, ttmlPath %q", got, mw.savePath, mw.ttmlPath)
	}
}

func TestImportTTMLAsksBeforeReplacingEdits(t *testing.T) {
	mw := newPlayingTestWindow(t)
	pressKey(mw, fyne.KeyS)

	mw.chooseTTML()
	overlay := mw.window.Canvas().Overlays().Top()
	if overlay == nil || !strings.Contains(test.RenderObjectToMarkup(overlay), "Import without saving") {
		t.Fatal("importing over unsaved lines did not ask first")
	}
	if got := len(mw.history.Project().Lines); got != 1 {
		t.Fatalf("lines changed before confirming: %d", got)
	}

	// After confirming, the import replaces the lines and keeps the audio.
	if err := mw.importTTML(writeTTML(t, testTTML)); err != nil {
		t.Fatalf("importTTML() error: %v", err)
	}
	if got := len(mw.history.Project().Lines); got != 3 || !mw.transportBar.Visible() || mw.trackInfo.String() != "Song.MP3" {
		t.Fatalf("after import with audio: %d lines, controls visible %t, title %q",
			got, mw.transportBar.Visible(), mw.trackInfo.String())
	}
}

func TestImportTTMLThatLosesContent(t *testing.T) {
	mw := newTestMainWindow(t)
	path := writeTTML(t, `<tt xmlns="http://www.w3.org/ns/ttml"><body><div>`+
		`<p begin="1s" end="2s"><span begin="1s" end="1.5s">Hel</span><span begin="1.5s" end="2s">lo</span></p>`+
		`</div></body></tt>`)

	if err := mw.importTTML(path); err != nil {
		t.Fatalf("importTTML() error: %v", err)
	}
	if mw.savePath != "" {
		t.Fatalf("savePath = %q; saving would overwrite the word timing", mw.savePath)
	}
	overlay := mw.window.Canvas().Overlays().Top()
	if overlay == nil || !strings.Contains(test.RenderObjectToMarkup(overlay), "word timing") {
		t.Fatal("no dialog naming what was left out")
	}
	if got := mw.history.Project().Lines[0].Text; got != "Hello" {
		t.Errorf("line text = %q, want %q", got, "Hello")
	}
}

func TestImportBadTTMLKeepsLines(t *testing.T) {
	mw := newPlayingTestWindow(t)
	pressKey(mw, fyne.KeyS)

	if err := mw.importTTML(writeTTML(t, "not ttml")); err == nil {
		t.Fatal("importTTML accepted a file that isn't TTML")
	}
	if got := len(mw.history.Project().Lines); got != 1 || !mw.history.Dirty() {
		t.Fatalf("a failed import changed the lines: %d lines, dirty %t", got, mw.history.Dirty())
	}
}

func TestMismatchWarning(t *testing.T) {
	// The fake audio is 2:00 long.
	long := `<tt xmlns="http://www.w3.org/ns/ttml"><body><div>` +
		`<p begin="00:00:10.000" end="00:03:05.000">too long</p></div></body></tt>`
	otherLength := `<tt xmlns="http://www.w3.org/ns/ttml"><body dur="00:04:09.895"><div>` +
		`<p begin="00:00:10.000" end="00:00:20.000">fits, but the file is for a longer song</p></div></body></tt>`
	matching := `<tt xmlns="http://www.w3.org/ns/ttml"><body dur="00:02:00.500"><div>` +
		`<p begin="00:00:10.000" end="00:01:55.000">fits</p></div></body></tt>`

	tests := []struct {
		name, doc, want string
		audioFirst      bool
	}{
		{"lyrics past the audio, audio first", long, "The lyrics run to 3:05.000, but the audio is only 2:00.000 long.", true},
		{"lyrics past the audio, lyrics first", long, "The lyrics run to 3:05.000", false},
		{"stated length differs", otherLength, "for a song 4:09.895 long, but the audio is 2:00.000", true},
		{"matching", matching, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := newTestMainWindow(t)
			mw.window.Resize(fyne.NewSize(windowW, windowH))
			path := writeTTML(t, tt.doc)
			if tt.audioFirst {
				if err := mw.loadAudio(testAudioPath); err != nil {
					t.Fatal(err)
				}
			}
			if err := mw.importTTML(path); err != nil {
				t.Fatal(err)
			}
			if !tt.audioFirst {
				if err := mw.loadAudio(testAudioPath); err != nil {
					t.Fatal(err)
				}
			}

			overlay := mw.window.Canvas().Overlays().Top()
			if tt.want == "" {
				if overlay != nil {
					t.Fatalf("matching files showed a dialog: %s", test.RenderObjectToMarkup(overlay))
				}

				return
			}
			if overlay == nil || !strings.Contains(test.RenderObjectToMarkup(overlay), tt.want) {
				t.Fatalf("no warning containing %q", tt.want)
			}
			if got := len(mw.history.Project().Lines); got != 1 {
				t.Fatalf("the warning changed the lines: %d", got)
			}
		})
	}
}

func TestSaveWritesMetadata(t *testing.T) {
	mw := newMarkedTestWindow(t)
	mw.audioTags = audio.Tags{Title: "Say It With Ya Chest", Artist: "Amarionette", Album: "Amarionette", ISRC: "USX9P1300004"}
	mw.lyricRows.Objects[0].(*lyricRow).text.SetText("one")
	mw.lyricRows.Objects[1].(*lyricRow).text.SetText("two")
	mw.saveTo(filepath.Join(t.TempDir(), "song.ttml"))

	written, err := os.ReadFile(mw.savePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`xmlns:amll="http://www.example.com/ns/amll"`,
		`<amll:meta key="musicName" value="Say It With Ya Chest"/>`,
		`<amll:meta key="artists" value="Amarionette"/>`,
		`<amll:meta key="album" value="Amarionette"/>`,
		`<amll:meta key="isrc" value="USX9P1300004"/>`,
	} {
		if !strings.Contains(string(written), want) {
			t.Errorf("saved TTML lacks %s:\n%s", want, written)
		}
	}

	// Metadata from an imported file wins over the audio's tags.
	if err := mw.importTTML(writeTTML(t, `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:amll="http://www.example.com/ns/amll">`+
		`<head><metadata><amll:meta key="musicName" value="Imported Title"/></metadata></head>`+
		`<body><div><p begin="1s" end="2s">x</p></div></body></tt>`)); err != nil {
		t.Fatal(err)
	}
	mw.saveTo(mw.ttmlPath)
	written, _ = os.ReadFile(mw.ttmlPath)
	if !strings.Contains(string(written), `value="Imported Title"`) || strings.Contains(string(written), `value="Say It With Ya Chest"`) ||
		!strings.Contains(string(written), `<amll:meta key="isrc" value="USX9P1300004"/>`) {
		t.Errorf("imported metadata did not win, or tags did not fill the gaps:\n%s", written)
	}
}

func TestWindowGrowsWhenLeavingLaunchScreen(t *testing.T) {
	mw := newTestMainWindow(t)
	mw.window.Resize(fyne.NewSize(windowW, windowH))

	if err := mw.loadAudio(testAudioPath); err != nil {
		t.Fatal(err)
	}
	if got := mw.window.Canvas().Size(); got != fyne.NewSize(editorW, editorH) {
		t.Fatalf("window is %v after opening a file, want %vx%v", got, editorW, editorH)
	}

	// Later loads leave the size alone, even one the user made smaller.
	mw.window.Resize(fyne.NewSize(700, 500))
	if err := mw.importTTML(writeTTML(t, testTTML)); err != nil {
		t.Fatal(err)
	}
	if got := mw.window.Canvas().Size(); got != fyne.NewSize(700, 500) {
		t.Fatalf("window is %v after a second file, want the user's 700x500", got)
	}
}

func TestWindowKeepsLargerUserSize(t *testing.T) {
	mw := newTestMainWindow(t)
	mw.window.Resize(fyne.NewSize(1200, 500)) // wider than the editor size, but shorter

	if err := mw.importTTML(writeTTML(t, testTTML)); err != nil {
		t.Fatal(err)
	}
	if got := mw.window.Canvas().Size(); got != fyne.NewSize(1200, editorH) {
		t.Fatalf("window is %v, want the user's width with the editor's height (1200x%v)", got, editorH)
	}
}

func TestPlaybackSpeed(t *testing.T) {
	mw := newPlayingTestWindow(t)
	fake := fakeFor(t, mw)
	if mw.speedButton.Text != "1×" {
		t.Fatalf("speed button reads %q, want 1×", mw.speedButton.Text)
	}

	for _, want := range []string{"0.75×", "0.5×", "1×"} {
		test.Tap(mw.speedButton)
		if mw.speedButton.Text != want {
			t.Fatalf("after a tap the speed button reads %q, want %q", mw.speedButton.Text, want)
		}
	}

	pressKey(mw, fyne.KeyLeftBracket)
	pressKey(mw, fyne.KeyLeftBracket)
	pressKey(mw, fyne.KeyLeftBracket) // already slowest
	if got := fake.state().speed; got != 0.5 || mw.speedButton.Text != "0.5×" {
		t.Fatalf("after [ three times: player at %v, button %q; want 0.5", got, mw.speedButton.Text)
	}
	pressKey(mw, fyne.KeyRightBracket)
	if got := fake.state().speed; got != 0.75 {
		t.Fatalf("after ]: player at %v, want 0.75", got)
	}

	// The speed carries over to the next song.
	if err := mw.loadAudio(testAudioPath); err != nil {
		t.Fatal(err)
	}
	if got := fakeFor(t, mw).state().speed; got != 0.75 {
		t.Fatalf("new song plays at %v, want the chosen 0.75", got)
	}
}

func TestTapsAtSlowSpeedUseSongTime(t *testing.T) {
	mw := newPlayingTestWindow(t)
	mw.setSpeed(0.5)
	fake := fakeFor(t, mw)
	fake.setPosition(10 * time.Second) // the player reports song time at any speed
	pressKey(mw, fyne.KeyS)
	fake.setPosition(12 * time.Second)
	pressKey(mw, fyne.KeyS)

	checkRows(t, mw, []string{"  1 0:10.000 – 0:12.000", "  2 0:12.000 – -:--.---"})
}

// newMarkedTestWindow returns a playing window with two marked lines, counted
// as saved, with no text yet:
// 0:01.000–0:02.000 and 0:02.000–0:03.000.
func newMarkedTestWindow(t *testing.T) *mainWindow {
	t.Helper()

	mw := newPlayingTestWindow(t)
	fake := fakeFor(t, mw)
	for _, at := range []time.Duration{time.Second, 2 * time.Second} {
		fake.setPosition(at)
		pressKey(mw, fyne.KeyS)
	}
	fake.setPosition(3 * time.Second)
	pressKey(mw, fyne.KeyE)
	mw.history.MarkSaved()
	mw.refreshLyrics()

	return mw
}

// checkRows compares each row's number and times with want. It reads the
// number rather than the label, which is blank while the row is hovered.
func checkRows(t *testing.T, mw *mainWindow, want []string) {
	t.Helper()

	if got := len(mw.lyricRows.Objects); got != len(want) {
		t.Fatalf("%d lyric rows, want %d", got, len(want))
	}
	for i, w := range want {
		row := mw.lyricRows.Objects[i].(*lyricRow)
		if got := row.number.text + " " + row.begin.label.Text + " – " + row.end.label.Text; got != w {
			t.Errorf("row %d = %q, want %q", i, got, w)
		}
	}
}

func TestSpaceTogglesPlayback(t *testing.T) {
	mw := newTestMainWindow(t)
	if err := mw.loadAudio(testAudioPath); err != nil {
		t.Fatalf("loadAudio() error: %v", err)
	}

	pressKey(mw, fyne.KeySpace)
	if !mw.playing {
		t.Fatal("Space did not start playback")
	}
	pressKey(mw, fyne.KeySpace)
	if mw.playing {
		t.Fatal("Space did not pause playback")
	}
	if got := len(mw.history.Project().Lines); got != 0 {
		t.Fatalf("Space made %d lines, want 0", got)
	}
}

func TestMarksNeedPlayback(t *testing.T) {
	mw := newTestMainWindow(t)
	pressKey(mw, fyne.KeyS)
	if err := mw.loadAudio(testAudioPath); err != nil {
		t.Fatalf("loadAudio() error: %v", err)
	}
	// Paused at 0:00 right after loading: a mark here would be the stray
	// 0:00.000 line.
	pressKey(mw, fyne.KeyS)
	mw.tap()

	if got := len(mw.history.Project().Lines); got != 0 {
		t.Fatalf("marks while not playing made %d lines, want 0", got)
	}
	if !mw.tapButton.Disabled() || !mw.endButton.Disabled() {
		t.Fatal("mark buttons are enabled while paused")
	}
}

func TestUndoRedoShortcuts(t *testing.T) {
	mw := newPlayingTestWindow(t)
	pressKey(mw, fyne.KeyS)
	fakeFor(t, mw).setPosition(time.Second)
	pressKey(mw, fyne.KeyS)

	typeShortcut(t, mw, &fyne.ShortcutUndo{})
	if got := len(mw.lyricRows.Objects); got != 1 {
		t.Fatalf("%d rows after Ctrl+Z, want 1", got)
	}
	typeShortcut(t, mw, &desktop.CustomShortcut{KeyName: fyne.KeyZ, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift})
	if got := len(mw.lyricRows.Objects); got != 2 {
		t.Fatalf("%d rows after Ctrl+Shift+Z, want 2", got)
	}
	typeShortcut(t, mw, &fyne.ShortcutUndo{})
	typeShortcut(t, mw, &fyne.ShortcutRedo{})
	if got := len(mw.lyricRows.Objects); got != 2 {
		t.Fatalf("%d rows after Ctrl+Z, Ctrl+Y, want 2", got)
	}
}

func TestSaveWritesTTML(t *testing.T) {
	mw := newMarkedTestWindow(t)
	if !mw.saveButton.Disabled() {
		t.Fatal("save button is enabled with nothing to save")
	}
	mw.lyricRows.Objects[0].(*lyricRow).text.SetText("Remember when you were young")
	mw.lyricRows.Objects[1].(*lyricRow).text.SetText("You shone like the sun")
	if mw.saveButton.Disabled() {
		t.Fatal("save button is disabled with unsaved changes")
	}
	mw.savePath = filepath.Join(t.TempDir(), "song.ttml") // as if chosen on an earlier save

	typeShortcut(t, mw, &desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl})
	if mw.history.Dirty() || !mw.saveButton.Disabled() {
		t.Fatal("Ctrl+S did not save")
	}
	written, err := os.ReadFile(mw.savePath)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	for _, want := range []string{
		`<body dur="00:02:00.000">`,
		`<p begin="00:00:01.000" end="00:00:02.000" ttm:agent="v1" itunes:key="L1">Remember when you were young</p>`,
	} {
		if !strings.Contains(string(written), want) {
			t.Errorf("saved TTML lacks %s:\n%s", want, written)
		}
	}

	// Ctrl+S also works while typing in a lyric box.
	text := mw.lyricRows.Objects[1].(*lyricRow).text
	text.SetText("You shone like the sun!")
	text.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl})
	if mw.history.Dirty() {
		t.Fatal("Ctrl+S in a lyric box did not save")
	}
	if written, _ := os.ReadFile(mw.savePath); !strings.Contains(string(written), "sun!") {
		t.Fatal("second save did not overwrite the file")
	}
}

func TestSaveAsksForFileFirst(t *testing.T) {
	mw := newMarkedTestWindow(t)
	mw.lyricRows.Objects[0].(*lyricRow).text.SetText("one")
	mw.lyricRows.Objects[1].(*lyricRow).text.SetText("two")

	mw.save()
	if mw.window.Canvas().Overlays().Top() == nil {
		t.Fatal("first save did not open a file dialog")
	}
	if !mw.history.Dirty() {
		t.Fatal("lines count as saved before a file was chosen")
	}

	// Choosing a file in the dialog lands here.
	path := filepath.Join(t.TempDir(), "chosen.ttml")
	mw.saveTo(path)
	if mw.savePath != path || mw.history.Dirty() {
		t.Fatalf("after saving to the chosen file: savePath %q, dirty %t", mw.savePath, mw.history.Dirty())
	}
}

func TestSaveBlockedByProblems(t *testing.T) {
	mw := newMarkedTestWindow(t)
	mw.insertLine(1) // untimed
	mw.savePath = filepath.Join(t.TempDir(), "song.ttml")

	mw.save()
	if !mw.history.Dirty() {
		t.Fatal("a save with problems marked the lines saved")
	}
	if _, err := os.Stat(mw.savePath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a save with problems wrote a file (stat error %v)", err)
	}
	overlay := mw.window.Canvas().Overlays().Top()
	if overlay == nil || !strings.Contains(test.RenderObjectToMarkup(overlay), "Line 2: no start or end time") {
		t.Fatalf("no dialog listing the problem with line 2: %s", test.RenderObjectToMarkup(overlay))
	}
}

func TestLoadingAudioForgetsSavePath(t *testing.T) {
	mw := newMarkedTestWindow(t)
	mw.savePath = "/music/old.ttml"
	if err := mw.loadAudio(testAudioPath); err != nil {
		t.Fatalf("loadAudio() error: %v", err)
	}
	if mw.savePath != "" {
		t.Fatalf("savePath = %q after loading new audio, want empty", mw.savePath)
	}
}

func TestUnsavedLinesAskBeforeDiscarding(t *testing.T) {
	mw := newPlayingTestWindow(t)
	ran := false
	mw.confirmDiscard("Load", func() { ran = true })
	if !ran {
		t.Fatal("confirmDiscard asked with nothing unsaved")
	}

	pressKey(mw, fyne.KeyS)
	ran = false
	mw.confirmDiscard("Load", func() { ran = true })
	if ran {
		t.Fatal("confirmDiscard went ahead with unsaved lines")
	}
	if mw.window.Canvas().Overlays().Top() == nil {
		t.Fatal("no confirmation dialog for unsaved lines")
	}
}

func TestLoadingAudioClearsLines(t *testing.T) {
	mw := newPlayingTestWindow(t)
	pressKey(mw, fyne.KeyS)

	if err := mw.loadAudio(testAudioPath); err != nil {
		t.Fatalf("loadAudio() error: %v", err)
	}
	if got := len(mw.history.Project().Lines); got != 0 {
		t.Fatalf("%d lines after loading a new file, want 0", got)
	}
	if got := len(mw.lyricRows.Objects); got != 0 {
		t.Fatalf("%d rows after loading a new file, want 0", got)
	}
	if mw.history.Dirty() {
		t.Fatal("history is dirty after loading a new file")
	}
}

func TestLyricTextGoesThroughHistory(t *testing.T) {
	mw := newPlayingTestWindow(t)
	pressKey(mw, fyne.KeyS)
	row := mw.lyricRows.Objects[0].(*lyricRow)

	row.text.SetText("Remember when you were young")
	if got := mw.history.Project().Lines[0].Text; got != "Remember when you were young" {
		t.Fatalf("line text = %q after typing, want the typed text", got)
	}

	mw.undo()
	if row.text.Text != "" {
		t.Fatalf("text box = %q after undo, want empty", row.text.Text)
	}
	// Refreshing the box must not record an edit, which would clear redo.
	if !mw.history.Redo() {
		t.Fatal("redo is empty after refreshing the rows")
	}
}

func TestFormatBoundary(t *testing.T) {
	ms := func(v core.Ms) *core.Ms { return &v }
	tests := []struct {
		in   *core.Ms
		want string
	}{
		{nil, "-:--.---"},
		{ms(0), "0:00.000"},
		{ms(61_234), "1:01.234"},
		{ms(3_723_004), "1:02:03.004"},
	}
	for _, tt := range tests {
		if got := formatBoundary(tt.in); got != tt.want {
			t.Errorf("formatBoundary(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatTime(t *testing.T) {
	tests := map[time.Duration]string{
		0:                           "0:00",
		-time.Second:                "0:00",
		59 * time.Second:            "0:59",
		time.Minute + 5*time.Second: "1:05",
		time.Hour + 2*time.Minute + 3*time.Second: "1:02:03",
	}
	for input, want := range tests {
		if got := formatTime(input); got != want {
			t.Errorf("formatTime(%v) = %q, want %q", input, got, want)
		}
	}
}

type fakeState struct {
	position time.Duration
	playing  bool
	speed    float64
}

type fakePlayer struct {
	mu       sync.Mutex
	position time.Duration
	playing  bool
	speed    float64
}

func (p *fakePlayer) SetSpeed(speed float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.speed = speed
}

func (p *fakePlayer) Play() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.playing = true
}

func (p *fakePlayer) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.playing = false
}

func (p *fakePlayer) Seek(position time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.position = position

	return nil
}

func (p *fakePlayer) Position() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.position
}

func (*fakePlayer) Duration() time.Duration {
	return testAudioDuration
}

func (*fakePlayer) Close() error {
	return nil
}

func (p *fakePlayer) setPosition(position time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.position = position
}

func (p *fakePlayer) state() fakeState {
	p.mu.Lock()
	defer p.mu.Unlock()

	return fakeState{position: p.position, playing: p.playing, speed: p.speed}
}

func newTestMainWindow(t *testing.T) *mainWindow {
	t.Helper()

	mw := newMainWindow(test.NewTempWindow(t, widget.NewLabel("")))
	mw.window.SetContent(mw.content()) // time boxes need a canvas to take focus
	// The real clock updates widgets from its own goroutine every 100 ms, which
	// races the test; tests call syncPosition themselves instead.
	mw.clock = func(context.Context) {}
	mw.openAudio = func(string) (audioPlayer, error) {
		return new(fakePlayer), nil
	}
	t.Cleanup(mw.close)

	return mw
}

func fakeFor(t *testing.T, mw *mainWindow) *fakePlayer {
	t.Helper()

	fake, ok := mw.player.(*fakePlayer)
	if !ok {
		t.Fatalf("player is %T, want *fakePlayer", mw.player)
	}

	return fake
}

func pressKey(mw *mainWindow, key fyne.KeyName) {
	mw.window.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: key})
}

// typeShortcut runs the window shortcut for s. The test canvas does not
// expose its shortcut handler, so this goes through the same table.
func typeShortcut(t *testing.T, mw *mainWindow, s fyne.Shortcut) {
	t.Helper()

	for _, sc := range mw.shortcuts() {
		if sc.keys.ShortcutName() == s.ShortcutName() {
			sc.run()

			return
		}
	}
	t.Fatalf("no shortcut %s", s.ShortcutName())
}

// newPlayingTestWindow returns a window with audio loaded and playing.
func newPlayingTestWindow(t *testing.T) *mainWindow {
	t.Helper()

	mw := newTestMainWindow(t)
	if err := mw.loadAudio(testAudioPath); err != nil {
		t.Fatalf("loadAudio() error: %v", err)
	}
	if err := mw.play(); err != nil {
		t.Fatalf("play() error: %v", err)
	}

	return mw
}

// clearOverlays closes any dialogs, as if dismissed.
func clearOverlays(mw *mainWindow) {
	overlays := mw.window.Canvas().Overlays()
	for top := overlays.Top(); top != nil; top = overlays.Top() {
		overlays.Remove(top)
	}
}
