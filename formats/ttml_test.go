package formats

import (
	"bytes"
	"encoding/xml"
	"errors"
	"strings"
	"testing"

	"ttml-app/core"
)

func ms(v core.Ms) *core.Ms { return &v }

func line(text string, begin, end core.Ms) core.Line {
	return core.Line{Text: text, Timing: core.Timing{Begin: ms(begin), End: ms(end)}}
}

func TestWriteTTML(t *testing.T) {
	p := &core.Project{Lines: []core.Line{
		line("  Remember when you were young  ", 12_340, 15_900),
		line(`Rock & "roll" <loud>`, 15_200, 19_250), // overlaps line 1 on purpose
		line("", 19_250, 60_000),                     // an instrumental break
		line("Past the hour", 3_600_000, 3_723_004),
	}}

	var out bytes.Buffer
	if err := WriteTTML(&out, p, 3_800_000); err != nil {
		t.Fatalf("WriteTTML() error: %v", err)
	}

	want := `<?xml version="1.0" encoding="UTF-8"?>
<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata"` +
		` xmlns:itunes="http://music.apple.com/lyric-ttml-internal" itunes:timing="Line" xml:lang="en">
  <head>
    <metadata>
      <ttm:agent type="person" xml:id="v1"/>
    </metadata>
  </head>
  <body dur="01:03:20.000">
    <div begin="00:00:12.340" end="01:02:03.004">
      <p begin="00:00:12.340" end="00:00:15.900" ttm:agent="v1" itunes:key="L1">Remember when you were young</p>
      <p begin="00:00:15.200" end="00:00:19.250" ttm:agent="v1" itunes:key="L2">Rock &amp; "roll" &lt;loud&gt;</p>
      <p begin="00:00:19.250" end="00:01:00.000" ttm:agent="v1" itunes:key="L3"></p>
      <p begin="01:00:00.000" end="01:02:03.004" ttm:agent="v1" itunes:key="L4">Past the hour</p>
    </div>
  </body>
</tt>
`
	if got := out.String(); got != want {
		t.Fatalf("WriteTTML() output:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteTTMLReadsBack(t *testing.T) {
	p := &core.Project{Lines: []core.Line{line("a < b & c", 1000, 2000), line("second", 2000, 3000)}}
	var out bytes.Buffer
	if err := WriteTTML(&out, p, 0); err != nil {
		t.Fatalf("WriteTTML() error: %v", err)
	}
	if strings.Contains(out.String(), "dur=") {
		t.Error("body has a dur with no song length")
	}

	var doc struct {
		Lang  string `xml:"lang,attr"`
		Lines []struct {
			Begin string `xml:"begin,attr"`
			End   string `xml:"end,attr"`
			Text  string `xml:",chardata"`
		} `xml:"body>div>p"`
	}
	if err := xml.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("output is not well-formed XML: %v", err)
	}
	if doc.Lang != "en" || len(doc.Lines) != 2 || doc.Lines[0].Text != "a < b & c" {
		t.Fatalf("read back lang %q, %d lines, first %q", doc.Lang, len(doc.Lines), doc.Lines[0].Text)
	}
	for i, l := range doc.Lines {
		if want := clockTime(*p.Lines[i].Begin) + "–" + clockTime(*p.Lines[i].End); l.Begin+"–"+l.End != want {
			t.Errorf("line %d times %q–%q, want %s", i+1, l.Begin, l.End, want)
		}
	}
}

func TestWriteTTMLRefusesInvalidProject(t *testing.T) {
	p := &core.Project{Lines: []core.Line{{Text: "untimed"}}}
	var out bytes.Buffer

	var invalid *InvalidError
	if err := WriteTTML(&out, p, 0); !errors.As(err, &invalid) || len(invalid.Problems) == 0 {
		t.Fatalf("WriteTTML() error = %v, want an *InvalidError with problems", err)
	}
	if out.Len() != 0 {
		t.Fatal("WriteTTML wrote output for an invalid project")
	}
}

func TestClockTime(t *testing.T) {
	tests := map[core.Ms]string{
		0:           "00:00:00.000",
		27_054:      "00:00:27.054",
		249_895:     "00:04:09.895",
		3_723_004:   "01:02:03.004",
		360_000_000: "100:00:00.000",
	}
	for in, want := range tests {
		if got := clockTime(in); got != want {
			t.Errorf("clockTime(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestValidate(t *testing.T) {
	open := core.Line{Text: "still being marked", Timing: core.Timing{Begin: ms(9000)}}
	p := &core.Project{Lines: []core.Line{
		line("ok", 1000, 2000),
		line("", 2000, 3000), // no text: an instrumental break, which is fine
		{Text: "inserted"},
		{Text: "end only", Timing: core.Timing{End: ms(4000)}},
		line("backwards", 5000, 5000),
		line("out of order", 1500, 6000),
		line("overlaps the next line, which is fine", 6000, 8000),
		line("too long", 7000, 20_000),
		open,
	}}

	want := []string{
		"Line 3: no start or end time",
		"Line 4: no start time",
		"Line 5: ends at or before its start",
		"Line 6: starts before line 5",
		"Line 8: ends after the song (0:10.000)",
		"Line 9: no end time",
	}
	got := Validate(p, 10_000)
	if len(got) != len(want) {
		t.Fatalf("Validate() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i].String() != want[i] {
			t.Errorf("problem %d = %q, want %q", i, got[i], want[i])
		}
	}

	if got := Validate(&core.Project{}, 0); len(got) != 1 || got[0].String() != "There are no lines to export." {
		t.Errorf("Validate(empty) = %v, want the no-lines problem", got)
	}
}

func TestWarnings(t *testing.T) {
	p := &core.Project{Lines: []core.Line{
		line("fine", 1000, 2000),
		line("double tap", 2000, 2080),
		line("", 2080, 3500),   // short break
		line("", 3500, 30_000), // a real instrumental break
		line("just long enough", 30_000, 30_150),
		{Text: "untimed, an error not a warning"},
	}}
	want := []string{"Line 2: only 80 ms long; a double tap?", "Line 3: no text, but too short for a break; a missing lyric?"}
	got := Warnings(p)
	if len(got) != len(want) {
		t.Fatalf("Warnings() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i].String() != want[i] {
			t.Errorf("warning %d = %q, want %q", i, got[i], want[i])
		}
	}
}
