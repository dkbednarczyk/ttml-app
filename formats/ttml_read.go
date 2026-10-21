package formats

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"ttml-app/core"
)

const (
	ttmlNS     = "http://www.w3.org/ns/ttml"
	ttmlMetaNS = "http://www.w3.org/ns/ttml#metadata"
	itunesNS   = "http://music.apple.com/lyric-ttml-internal"
	// amllNS is AMLL's metadata namespace, copied from files in the AMLL TTML
	// database (github.com/amll-dev/amll-ttml-db). It is a placeholder URL, used as is.
	amllNS = "http://www.example.com/ns/amll"
)

var (
	// ErrNotTTML means the file is not a TTML document.
	ErrNotTTML = errors.New("not a TTML file")
	// ErrBadTTMLTime means a begin or end attribute is not a time the reader understands.
	ErrBadTTMLTime = errors.New("not a TTML time")
)

// Imported is a project read from a file, with what was left out.
type Imported struct {
	Project core.Project
	// Duration is the song length the file states (body dur), or nil.
	Duration *core.Ms
	// Dropped names the kinds of content the editor can't hold yet and left
	// out, such as "word timing"; empty when nothing was lost.
	Dropped []string
}

// ReadTTML reads the lines of a TTML file: each <p> becomes a line with its
// begin and end and its text. AMLL metadata (amll:meta) and the stated song
// length (body dur) are kept. Word timing, background vocals, translations,
// several singers and song parts are not supported yet; they are left out
// and named in Dropped. A p without begin or end gets an unset time.
//
// AMLL writes <div xmlns="">, which takes the lines out of the TTML
// namespace, so inside <tt> elements with no namespace count as TTML too.
func ReadTTML(r io.Reader) (*Imported, error) {
	d := xml.NewDecoder(r)
	var (
		result   Imported
		dropped  = map[string]bool{}
		sawTT    bool
		inP      bool
		skip     int // depth inside an element whose content is left out
		text     strings.Builder
		line     core.Line
		agents   = map[string]bool{}
		lineNode int // count of <p> elements, for error messages
	)
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read TTML: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if !sawTT {
				if t.Name.Space != ttmlNS || t.Name.Local != "tt" {
					return nil, ErrNotTTML
				}
				sawTT = true

				continue
			}
			if skip > 0 {
				skip++

				continue
			}
			if t.Name.Space == amllNS && t.Name.Local == "meta" {
				if key := attr(t, "", "key"); key != "" {
					result.Project.Meta = append(result.Project.Meta, core.MetaEntry{Key: key, Value: attr(t, "", "value")})
				}

				continue
			}
			if isTTML(t.Name, "body") {
				if result.Duration, err = timeAttr(t, "dur", 0); err != nil {
					return nil, err
				}
			}
			if isTTML(t.Name, "div") && attr(t, itunesNS, "songPart") != "" {
				dropped["song parts"] = true
			}
			if isTTML(t.Name, "p") && !inP {
				lineNode++
				inP = true
				text.Reset()
				line = core.Line{}
				if line.Begin, err = timeAttr(t, "begin", lineNode); err != nil {
					return nil, err
				}
				if line.End, err = timeAttr(t, "end", lineNode); err != nil {
					return nil, err
				}
				if agent := attr(t, ttmlMetaNS, "agent"); agent != "" {
					agents[agent] = true
				}

				continue
			}
			if !inP {
				continue
			}
			switch role := attr(t, ttmlMetaNS, "role"); role {
			case "x-bg":
				dropped["background vocals"] = true
				skip = 1
			case "x-translation":
				dropped["translations"] = true
				skip = 1
			case "x-roman":
				dropped["romanizations"] = true
				skip = 1
			default:
				if t.Name.Local == "br" {
					text.WriteByte(' ')
				}
				if attr(t, "", "begin") != "" {
					dropped["word timing"] = true
				}
			}
		case xml.EndElement:
			switch {
			case skip > 0:
				skip--
			case inP && isTTML(t.Name, "p"):
				inP = false
				line.Text = strings.Join(strings.Fields(text.String()), " ")
				result.Project.Lines = append(result.Project.Lines, line)
			}
		case xml.CharData:
			if inP && skip == 0 {
				text.Write(t)
			}
		}
	}
	if !sawTT {
		return nil, ErrNotTTML
	}
	if len(agents) > 1 {
		dropped["separate singers"] = true
	}

	for kind := range dropped {
		result.Dropped = append(result.Dropped, kind)
	}
	slices.Sort(result.Dropped)

	return &result, nil
}

func isTTML(name xml.Name, local string) bool {
	return (name.Space == ttmlNS || name.Space == "") && name.Local == local
}

func attr(e xml.StartElement, space, local string) string {
	for _, a := range e.Attr {
		if a.Name.Space == space && a.Name.Local == local {
			return a.Value
		}
	}

	return ""
}

func timeAttr(e xml.StartElement, name string, lineNode int) (*core.Ms, error) {
	value := attr(e, "", name)
	if value == "" {
		return nil, nil // an absent time is unset, not an error
	}
	ms, err := ParseTTMLTime(value)
	if err != nil {
		if lineNode == 0 {
			return nil, fmt.Errorf("body %s: %w", name, err)
		}

		return nil, fmt.Errorf("line %d %s: %w", lineNode, name, err)
	}

	return &ms, nil
}

var (
	clockPattern  = regexp.MustCompile(`^(?:(\d+):)?(\d+):(\d+(?:\.\d+)?)$`)
	offsetPattern = regexp.MustCompile(`^(\d+(?:\.\d+)?)(h|m|s|ms)?$`)
)

// ParseTTMLTime reads a TTML time: clock time (hh:mm:ss.fff, or Apple's
// shorter m:ss.fff), or an offset such as 12.345s, 12345ms, 1.5m, or plain
// seconds (12.345), which Apple's files use. Frame and tick times are not
// supported. Fractions are rounded to the millisecond.
func ParseTTMLTime(text string) (core.Ms, error) {
	if m := clockPattern.FindStringSubmatch(text); m != nil {
		hours, _ := strconv.ParseFloat(orZero(m[1]), 64)
		minutes, _ := strconv.ParseFloat(m[2], 64)
		seconds, _ := strconv.ParseFloat(m[3], 64)
		if minutes >= 60 || seconds >= 60 {
			return 0, fmt.Errorf("%w: %q", ErrBadTTMLTime, text)
		}

		return msFrom(((hours*60)+minutes)*60 + seconds), nil
	}
	if m := offsetPattern.FindStringSubmatch(text); m != nil {
		value, _ := strconv.ParseFloat(m[1], 64)
		unit := map[string]float64{"h": 3600, "m": 60, "s": 1, "": 1, "ms": 0.001}[m[2]]

		return msFrom(value * unit), nil
	}

	return 0, fmt.Errorf("%w: %q", ErrBadTTMLTime, text)
}

func orZero(digits string) string {
	if digits == "" {
		return "0"
	}

	return digits
}

func msFrom(seconds float64) core.Ms {
	return core.Ms(math.Round(seconds * 1000))
}
