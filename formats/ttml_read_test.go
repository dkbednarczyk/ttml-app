package formats

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"ttml-app/core"
)

func TestReadTTMLRoundTrip(t *testing.T) {
	want := &core.Project{Lines: []core.Line{
		line("We've seen this place & before", 27_054, 32_570),
		line("", 32_570, 49_100), // a break
		line(`A "room" <void> of doors`, 49_000, 3_723_004),
	}}
	var out bytes.Buffer
	if err := WriteTTML(&out, want, 3_800_000); err != nil {
		t.Fatalf("WriteTTML() error: %v", err)
	}

	got, err := ReadTTML(&out)
	if err != nil {
		t.Fatalf("ReadTTML() error: %v", err)
	}
	if len(got.Dropped) != 0 {
		t.Errorf("Dropped = %v reading our own export, want nothing", got.Dropped)
	}
	assertLines(t, got.Project.Lines, want.Lines)
}

// The style the user's other editor writes: one line, full clock times,
// empty <p> for breaks, no head.
func TestReadTTMLOtherEditor(t *testing.T) {
	const doc = `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:itunes="http://music.apple.com/lyric-ttml-internal"` +
		` xmlns:ttm="http://www.w3.org/ns/ttml#metadata" itunes:timing="Line" xml:lang="en"><body><div>` +
		`<p begin="00:00:00.000" end="00:00:27.044"></p>` +
		`<p begin="00:00:27.054" end="00:00:32.570">We've seen this place before</p>` +
		`<p begin="00:01:08.411" end="00:01:14.000">When I can't feel anything?</p>` +
		`<p begin="00:01:11.867" end="00:01:15.729">We want change</p></div></body></tt>`

	got, err := ReadTTML(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("ReadTTML() error: %v", err)
	}
	assertLines(t, got.Project.Lines, []core.Line{
		line("", 0, 27_044),
		line("We've seen this place before", 27_054, 32_570),
		line("When I can't feel anything?", 68_411, 74_000),
		line("We want change", 71_867, 75_729), // overlaps the previous line
	})
	if len(got.Dropped) != 0 {
		t.Errorf("Dropped = %v, want nothing", got.Dropped)
	}
}

// Apple's word-timed style, with what the editor can't hold yet.
func TestReadTTMLWordTimed(t *testing.T) {
	const doc = `<?xml version="1.0" encoding="UTF-8"?>
<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata"
    xmlns:itunes="http://music.apple.com/lyric-ttml-internal" itunes:timing="Word">
  <body dur="3:41.250">
    <div begin="12.34" end="45.1" itunes:songPart="Verse">
      <p begin="12.340" end="15.900" ttm:agent="v1" itunes:key="L1">
        <span begin="12.340" end="12.700">Hello</span> <span begin="12.700" end="13.000">won</span><span begin="13.000" end="13.200">der</span>
        <span ttm:role="x-bg"><span begin="15.100" end="15.900">(ooh)</span></span>
        <span ttm:role="x-translation" xml:lang="es">Hola</span>
      </p>
      <p begin="1:16.000" end="80s" ttm:agent="v2">Second<br/>singer</p>
    </div>
  </body>
</tt>`

	got, err := ReadTTML(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("ReadTTML() error: %v", err)
	}
	// Spans with no space between them stay joined ("wonder").
	assertLines(t, got.Project.Lines, []core.Line{
		line("Hello wonder", 12_340, 15_900),
		line("Second singer", 76_000, 80_000),
	})
	want := []string{"background vocals", "separate singers", "song parts", "translations", "word timing"}
	if !slices.Equal(got.Dropped, want) {
		t.Errorf("Dropped = %v, want %v", got.Dropped, want)
	}
}

func TestReadTTMLUntimedAndErrors(t *testing.T) {
	const untimed = `<tt xmlns="http://www.w3.org/ns/ttml"><body><div><p>no times</p></div></body></tt>`
	got, err := ReadTTML(strings.NewReader(untimed))
	if err != nil || len(got.Project.Lines) != 1 || got.Project.Lines[0].Begin != nil {
		t.Fatalf("ReadTTML(untimed) = %+v, %v; want one line with no times", got, err)
	}

	for name, doc := range map[string]string{
		"not xml":      "lyrics, one per line",
		"other xml":    `<html><body><p>hi</p></body></html>`,
		"tt, wrong ns": `<tt><body/></tt>`,
	} {
		if _, err := ReadTTML(strings.NewReader(doc)); err == nil {
			t.Errorf("%s: ReadTTML accepted it", name)
		}
	}
	const badTime = `<tt xmlns="http://www.w3.org/ns/ttml"><body><div><p begin="soon" end="1s">x</p></div></body></tt>`
	if _, err := ReadTTML(strings.NewReader(badTime)); !errors.Is(err, ErrBadTTMLTime) ||
		!strings.Contains(err.Error(), "line 1 begin") {
		t.Errorf("bad time: error = %v, want %v naming line 1 begin", err, ErrBadTTMLTime)
	}
}

func TestParseTTMLTime(t *testing.T) {
	valid := map[string]core.Ms{
		"00:00:27.054": 27_054,
		"01:02:03.004": 3_723_004,
		"1:02:03.004":  3_723_004,
		"0:27.054":     27_054,
		"12:27.05":     747_050,
		"27.054":       27_054,
		"27":           27_000,
		"12.3456":      12_346,
		"12.345s":      12_345,
		"12345ms":      12_345,
		"1.5m":         90_000,
		"1h":           3_600_000,
	}
	for text, want := range valid {
		if got, err := ParseTTMLTime(text); err != nil || got != want {
			t.Errorf("ParseTTMLTime(%q) = %d, %v; want %d", text, got, err, want)
		}
	}
	for _, text := range []string{"", "soon", "1:60.000", "00:00:00:10", "10f", "-1s", "1 s"} {
		if _, err := ParseTTMLTime(text); !errors.Is(err, ErrBadTTMLTime) {
			t.Errorf("ParseTTMLTime(%q) error = %v, want %v", text, err, ErrBadTTMLTime)
		}
	}
}

func assertLines(t *testing.T, got, want []core.Line) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Text != strings.TrimSpace(want[i].Text) ||
			*got[i].Begin != *want[i].Begin || *got[i].End != *want[i].End {
			t.Errorf("line %d = %q %d–%d, want %q %d–%d", i+1, got[i].Text, *got[i].Begin, *got[i].End,
				want[i].Text, *want[i].Begin, *want[i].End)
		}
	}
}

// Shaped like files in the AMLL TTML database: <metadata xmlns=""> and
// <div xmlns=""> (which take the lines out of the TTML namespace), amll:meta
// entries with repeated keys, mm:ss.mmm times, word timing and romanization.
func TestReadTTMLAMLL(t *testing.T) {
	const doc = `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata"` +
		` xmlns:itunes="http://music.apple.com/lyric-ttml-internal" xmlns:amll="http://www.example.com/ns/amll">` +
		`<head><metadata xmlns=""><ttm:agent type="person" xml:id="v1"/>` +
		`<amll:meta key="ncmMusicId" value="123"/><amll:meta key="isrc" value="AA1111111111"/>` +
		`<amll:meta key="isrc" value="BB2222222222"/><amll:meta key="musicName" value="Test Song"/>` +
		`<amll:meta key="artists" value="Singer A"/><amll:meta key="artists" value="Singer B"/></metadata></head>` +
		`<body dur="03:31.123"><div xmlns="" begin="00:19.405" end="03:31.123">` +
		`<p begin="00:19.405" end="00:25.257" ttm:agent="v1" itunes:key="L1">` +
		`<span begin="00:19.405" end="00:19.769">First</span> <span begin="00:19.769" end="00:25.257">line</span>` +
		`<span ttm:role="x-roman">fa-su-to</span></p>` +
		`<p begin="00:25.257" end="03:31.123" ttm:agent="v1" itunes:key="L2">Second line</p></div></body></tt>`

	got, err := ReadTTML(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("ReadTTML() error: %v", err)
	}
	assertLines(t, got.Project.Lines, []core.Line{line("First line", 19_405, 25_257), line("Second line", 25_257, 211_123)})
	if got.Duration == nil || *got.Duration != 211_123 {
		t.Errorf("Duration = %v, want 3:31.123", got.Duration)
	}
	if isrc := got.Project.MetaValues("isrc"); !slices.Equal(isrc, []string{"AA1111111111", "BB2222222222"}) {
		t.Errorf("isrc = %v", isrc)
	}
	if artists := got.Project.MetaValues("artists"); !slices.Equal(artists, []string{"Singer A", "Singer B"}) {
		t.Errorf("artists = %v", artists)
	}
	if want := []string{"romanizations", "word timing"}; !slices.Equal(got.Dropped, want) {
		t.Errorf("Dropped = %v, want %v", got.Dropped, want)
	}
}

func TestMetaRoundTrip(t *testing.T) {
	want := &core.Project{
		Meta: []core.MetaEntry{
			{Key: "musicName", Value: `Rock & "Roll" <Live>`},
			{Key: "artists", Value: "A"},
			{Key: "artists", Value: "B"},
			{Key: "isrc", Value: "USRC17607839"},
			{Key: "spotifyId", Value: "7M0l6bMEuWPPEGprXHdt8c"},
		},
		Lines: []core.Line{line("x", 1000, 2000)},
	}
	var out bytes.Buffer
	if err := WriteTTML(&out, want, 249_895); err != nil {
		t.Fatalf("WriteTTML() error: %v", err)
	}
	for _, s := range []string{
		`xmlns:amll="http://www.example.com/ns/amll"`,
		`<amll:meta key="musicName" value="Rock &amp; &quot;Roll&quot; &lt;Live&gt;"/>`,
	} {
		if !strings.Contains(out.String(), s) {
			t.Errorf("output lacks %s:\n%s", s, out.String())
		}
	}

	got, err := ReadTTML(&out)
	if err != nil {
		t.Fatalf("ReadTTML() error: %v", err)
	}
	if !slices.Equal(got.Project.Meta, want.Meta) {
		t.Errorf("Meta = %v, want %v", got.Project.Meta, want.Meta)
	}
	if got.Duration == nil || *got.Duration != 249_895 {
		t.Errorf("Duration = %v, want 4:09.895", got.Duration)
	}
}
