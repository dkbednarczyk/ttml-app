package formats

import (
	"fmt"
	"io"
	"strings"

	"ttml-app/core"
)

// ttmlLang is the lyrics language. Fixed for now; a setting comes later.
const ttmlLang = "en"

// textEscaper escapes only what XML text requires, so apostrophes and quotes
// in lyrics stay readable ("We've", not "We&#39;ve"). attrEscaper also covers
// the double quotes that delimit attribute values.
var (
	textEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	attrEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
)

// WriteTTML writes p as line-timed TTML in the Apple / AMLL profile: one
// agent, the project's metadata as amll:meta entries, one div, and a p per
// line with plain text (empty for an instrumental break). songEnd, the length of the audio, becomes the body's
// dur. A project with problems is not written; the error is an
// *InvalidError listing them.
func WriteTTML(w io.Writer, p *core.Project, songEnd core.Ms) error {
	if problems := Validate(p, songEnd); len(problems) > 0 {
		return &InvalidError{Problems: problems}
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata"` +
		` xmlns:itunes="http://music.apple.com/lyric-ttml-internal"`)
	if len(p.Meta) > 0 {
		b.WriteString(` xmlns:amll="` + amllNS + `"`)
	}
	b.WriteString(` itunes:timing="Line" xml:lang="` + ttmlLang + `">` + "\n")
	b.WriteString("  <head>\n    <metadata>\n      <ttm:agent type=\"person\" xml:id=\"v1\"/>\n")
	for _, e := range p.Meta {
		fmt.Fprintf(&b, "      <amll:meta key=\"%s\" value=\"%s\"/>\n", attrEscaper.Replace(e.Key), attrEscaper.Replace(e.Value))
	}
	b.WriteString("    </metadata>\n  </head>\n")
	if songEnd > 0 {
		fmt.Fprintf(&b, "  <body dur=%q>\n", clockTime(songEnd))
	} else {
		b.WriteString("  <body>\n")
	}

	// Validate guarantees every line has both times and that starts are in
	// order, so the div runs from the first start to the latest end.
	lines := p.Lines
	divEnd := *lines[0].End
	for _, line := range lines {
		divEnd = max(divEnd, *line.End)
	}
	fmt.Fprintf(&b, "    <div begin=%q end=%q>\n", clockTime(*lines[0].Begin), clockTime(divEnd))
	for i, line := range lines {
		fmt.Fprintf(&b, "      <p begin=%q end=%q ttm:agent=\"v1\" itunes:key=\"L%d\">",
			clockTime(*line.Begin), clockTime(*line.End), i+1)
		b.WriteString(textEscaper.Replace(strings.TrimSpace(line.Text)))
		b.WriteString("</p>\n")
	}
	b.WriteString("    </div>\n  </body>\n</tt>\n")

	if _, err := io.WriteString(w, b.String()); err != nil {
		return fmt.Errorf("write TTML: %w", err)
	}

	return nil
}

// clockTime writes a time as TTML clock time, hh:mm:ss.mmm. Strict TTML
// requires the hours field, which the editor's shorter m:ss.mmm leaves out.
func clockTime(ms core.Ms) string {
	total := int64(ms)

	return fmt.Sprintf("%02d:%02d:%02d.%03d", total/3_600_000, total/60_000%60, total/1000%60, total%1000)
}
