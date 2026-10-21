package ui

import (
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestStatusIcon(t *testing.T) {
	mw := newPlayingTestWindow(t) // fake audio, 2:00 long
	if mw.statusButton.Visible() {
		t.Fatal("status icon shows with no lines")
	}
	fake := fakeFor(t, mw)
	press := func(key fyne.KeyName, at time.Duration) {
		fake.setPosition(at)
		pressKey(mw, key)
	}

	press(fyne.KeyS, time.Second) // an open line: no end time
	if mw.statusButton.Icon != mw.statusIcons.err {
		t.Fatal("status is not red with a line missing its end")
	}

	press(fyne.KeyE, 3*time.Second) // timed, but no text: too long to warn about? 2 s is the limit
	if mw.statusButton.Icon != mw.statusIcons.ok {
		t.Fatal("status is not green for a timed line (a 2 s break)")
	}

	press(fyne.KeyS, 5*time.Second)
	press(fyne.KeyS, 5100*time.Millisecond) // double tap: a 0.1 s line
	press(fyne.KeyE, 7*time.Second)
	if mw.statusButton.Icon != mw.statusIcons.warning {
		t.Fatal("status is not yellow with a 0.1 s line")
	}

	mw.undo() // drop the end of line 3: it's open again, an error outranks the warning
	if mw.statusButton.Icon != mw.statusIcons.err {
		t.Fatal("status is not red when errors and warnings mix")
	}
}

func TestStatusShowsMismatch(t *testing.T) {
	mw := newTestMainWindow(t)
	doc := `<tt xmlns="http://www.w3.org/ns/ttml"><body dur="00:04:09.895"><div>` +
		`<p begin="00:00:10.000" end="00:00:20.000">fits</p></div></body></tt>`
	if err := mw.importTTML(writeTTML(t, doc)); err != nil {
		t.Fatal(err)
	}
	if mw.statusButton.Icon != mw.statusIcons.ok {
		t.Fatal("status is not green before audio is loaded")
	}
	if err := mw.loadAudio(testAudioPath); err != nil { // 2:00 long, but the file says 4:09
		t.Fatal(err)
	}
	if mw.statusButton.Icon != mw.statusIcons.warning {
		t.Fatal("status is not yellow when the lyrics are for a longer song")
	}
}

func TestStatusDetails(t *testing.T) {
	mw := newMarkedTestWindow(t) // two timed lines, no text yet: 1 s breaks
	mw.window.Resize(fyne.NewSize(editorW, editorH))
	clearOverlays(mw)
	mw.insertLine(2) // untimed
	clearOverlays(mw)
	mw.window.Canvas().Unfocus()

	test.Tap(mw.statusButton)
	overlay := mw.window.Canvas().Overlays().Top()
	if overlay == nil {
		t.Fatal("tapping the status icon showed nothing")
	}
	markup := test.RenderObjectToMarkup(overlay)
	for _, want := range []string{"Can't save yet", "Line 3: no start or end time", "Line 1: no text, but too short for a break"} {
		if !strings.Contains(markup, want) {
			t.Errorf("status dialog lacks %q", want)
		}
	}
}

func TestSkipToStart(t *testing.T) {
	mw := newPlayingTestWindow(t)
	fake := fakeFor(t, mw)
	mw.seekTo(90_000)

	test.Tap(mw.startButton)
	if got := fake.state().position; got != 0 || mw.position != 0 || !mw.playing {
		t.Fatalf("after skipping to the start: player at %v, UI at %v, playing %t", got, mw.position, mw.playing)
	}
	mw.pause()
	mw.seekTo(30_000)
	test.Tap(mw.startButton)
	if mw.position != 0 || mw.playing {
		t.Fatalf("skipping to the start while paused: UI at %v, playing %t; want 0:00, paused", mw.position, mw.playing)
	}
}

func TestTransportOrder(t *testing.T) {
	mw := newPlayingTestWindow(t)
	mw.pasteLyrics("a line") // so the status icon shows
	mw.window.Resize(fyne.NewSize(editorW, editorH))

	x := func(o fyne.CanvasObject) float32 {
		return fyne.CurrentApp().Driver().AbsolutePositionForObject(o).X
	}
	order := []fyne.CanvasObject{mw.statusButton, mw.playButton, mw.startButton, mw.slider, mw.timeLabel, mw.speedButton}
	names := []string{"status", "play", "start", "slider", "time", "speed"}
	for i := 1; i < len(order); i++ {
		if x(order[i-1]) >= x(order[i]) {
			t.Errorf("%s (x=%v) is not left of %s (x=%v)", names[i-1], x(order[i-1]), names[i], x(order[i]))
		}
	}
}

func TestSyncButtonsLiveInTheEditTab(t *testing.T) {
	mw := newTestMainWindow(t)
	mw.window.Resize(fyne.NewSize(editorW, editorH))
	if err := mw.importTTML(writeTTML(t, testTTML)); err != nil {
		t.Fatal(err)
	}
	if mw.syncBar.Visible() {
		t.Fatal("S/E buttons show without audio")
	}

	if err := mw.loadAudio(testAudioPath); err != nil {
		t.Fatal(err)
	}
	if !mw.syncBar.Visible() {
		t.Fatal("S/E buttons hidden with audio loaded")
	}
	for _, o := range mw.transportBar.Objects {
		if strings.Contains(test.RenderObjectToMarkup(o), startLineLabel) {
			t.Fatal("S/E buttons are still in the playback bar")
		}
	}
	y := func(o fyne.CanvasObject) float32 { return fyne.CurrentApp().Driver().AbsolutePositionForObject(o).Y }
	if y(mw.tapButton) >= y(mw.transportBar) {
		t.Fatalf("S/E buttons (y=%v) are not above the playback bar (y=%v)", y(mw.tapButton), y(mw.transportBar))
	}

	mw.tabs.SelectIndex(1) // Preview: the buttons go with the Edit tab, the keys still work
	if mw.syncBar.Visible() && mw.tabs.Items[0].Content.Visible() {
		t.Fatal("S/E buttons still show on the Preview tab")
	}
	if err := mw.play(); err != nil {
		t.Fatal(err)
	}
	fakeFor(t, mw).setPosition(40 * time.Second)
	pressKey(mw, fyne.KeyS)
	if got := len(mw.history.Project().Lines); got != 4 {
		t.Fatalf("S on the Preview tab made %d lines, want 4", got)
	}
}

func TestNoDividerAbovePlaybar(t *testing.T) {
	mw := newPlayingTestWindow(t)
	for name, o := range map[string]fyne.CanvasObject{"playback bar": mw.transportBar, "S/E bar": mw.syncBar} {
		if strings.Contains(test.RenderObjectToMarkup(o), `type="*widget.Separator"`) {
			t.Errorf("the %s draws a divider", name)
		}
	}
}
