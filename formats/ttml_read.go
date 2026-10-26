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
	xmlNS  = "http://www.w3.org/XML/1998/namespace"
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
// begin and end, its text, its singer (ttm:agent) and its background vocal
// (x-bg spans, with their own times where those differ from the line's).
// Person agents become the singers, in the order the head declares them, and
// group agents sing for everyone. AMLL metadata (amll:meta) and the stated
// song length (body dur) are kept. Word timing, translations and song parts
// are not supported yet; they are left out and named in Dropped. A p without
// begin or end gets an unset time.
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
		lineNode int // count of <p> elements, for error messages
		agents   agentList
		agent    *agentDecl // the ttm:agent being read, if any
		inName   bool       // inside its ttm:name
		bg       bgVocal    // the background vocal of the line being read
		bgDepth  int        // depth inside an x-bg span
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
			if t.Name.Space == ttmlMetaNS && t.Name.Local == "agent" && !inP {
				agent = agents.declare(attr(t, xmlNS, "id"), attr(t, "", "type"))

				continue
			}
			if t.Name.Space == ttmlMetaNS && t.Name.Local == "name" && agent != nil {
				inName = true

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
				bg = bgVocal{}
				line = core.Line{}
				if line.Begin, err = timeAttr(t, "begin", lineNode); err != nil {
					return nil, err
				}
				if line.End, err = timeAttr(t, "end", lineNode); err != nil {
					return nil, err
				}
				agents.lines = append(agents.lines, attr(t, ttmlMetaNS, "agent"))

				continue
			}
			if !inP {
				continue
			}
			role := attr(t, ttmlMetaNS, "role")
			if bgDepth > 0 && role == "" {
				bgDepth++
				if err := bg.add(t, lineNode); err != nil {
					return nil, err
				}

				continue
			}
			switch role {
			case "x-bg":
				if bgDepth > 0 {
					bgDepth++ // an x-bg inside another counts as part of it
				} else {
					bgDepth = 1
					bg.text.WriteByte(' ') // between this and an earlier x-bg span
				}
				if err := bg.add(t, lineNode); err != nil {
					return nil, err
				}
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
			case bgDepth > 0:
				bgDepth--
			case inP && isTTML(t.Name, "p"):
				inP = false
				line.Text = strings.Join(strings.Fields(text.String()), " ")
				if bg.wordTimed() {
					dropped["word timing"] = true
				}
				bg.setOn(&line)
				result.Project.Lines = append(result.Project.Lines, line)
			case agent != nil && t.Name.Space == ttmlMetaNS && t.Name.Local == "name":
				inName = false
			case agent != nil && t.Name.Space == ttmlMetaNS && t.Name.Local == "agent":
				agent = nil
			}
		case xml.CharData:
			switch {
			case skip > 0:
			case bgDepth > 0:
				bg.text.Write(t)
			case inP:
				text.Write(t)
			case inName:
				agent.name += string(t)
			}
		}
	}
	if !sawTT {
		return nil, ErrNotTTML
	}
	if agents.assign(&result.Project) {
		dropped["several groups of singers"] = true
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

// agentDecl is a ttm:agent from the head.
type agentDecl struct {
	id, name string
	group    bool
}

// agentList collects the agents a file declares and the agent of each line,
// to turn into singers once the whole file is read.
type agentList struct {
	declared []*agentDecl
	lines    []string // each line's ttm:agent, "" for none
}

func (a *agentList) declare(id, kind string) *agentDecl {
	d := &agentDecl{id: id, group: kind == "group"}
	a.declared = append(a.declared, d)

	return d
}

// assign sets the project's singers and each line's singer: person agents in
// the order declared, then agents lines name without declaring them; a line
// with no agent is the first singer's. Group agents become Everyone. It
// reports whether lines used more than one group, which the editor can't
// tell apart.
func (a *agentList) assign(p *core.Project) (severalGroups bool) {
	var persons []*agentDecl
	byID := map[string]*agentDecl{}
	for _, d := range a.declared {
		if _, dup := byID[d.id]; dup {
			continue
		}
		byID[d.id] = d
		if !d.group {
			persons = append(persons, d)
		}
	}
	for _, id := range a.lines {
		if _, ok := byID[id]; !ok && id != "" {
			byID[id] = &agentDecl{id: id}
			persons = append(persons, byID[id])
		}
	}

	groups := map[string]bool{}
	for i, id := range a.lines {
		switch d := byID[id]; {
		case d == nil:
			p.Lines[i].Singer = 0
		case d.group:
			p.Lines[i].Singer = core.Everyone
			groups[id] = true
		default:
			p.Lines[i].Singer = slices.Index(persons, d)
		}
	}

	// One unnamed singer is what a project has with no singers listed.
	if len(persons) > 1 || len(persons) == 1 && persons[0].name != "" {
		for _, d := range persons {
			p.Singers = append(p.Singers, core.Singer{Name: strings.Join(strings.Fields(d.name), " ")})
		}
	}

	return len(groups) > 1
}

// bgVocal collects a line's x-bg spans: their text, and the earliest start
// and latest end among them and the timed spans inside them.
type bgVocal struct {
	text       strings.Builder
	begin, end *core.Ms
	timed      int // timed spans inside x-bg spans, one per word or syllable
}

// add takes the times of an x-bg span or an element inside one.
func (v *bgVocal) add(e xml.StartElement, lineNode int) error {
	begin, err := timeAttr(e, "begin", lineNode)
	if err != nil {
		return err
	}
	end, err := timeAttr(e, "end", lineNode)
	if err != nil {
		return err
	}
	if begin != nil && attr(e, ttmlMetaNS, "role") == "" {
		v.timed++
	}
	if begin != nil && (v.begin == nil || *begin < *v.begin) {
		v.begin = begin
	}
	if end != nil && (v.end == nil || *end > *v.end) {
		v.end = end
	}

	return nil
}

// wordTimed reports whether the background vocal was timed word by word,
// which the editor can't hold yet.
func (v *bgVocal) wordTimed() bool {
	return v.timed > 1
}

// setOn gives line the background vocal, with its own times only where they
// differ from the line's.
func (v *bgVocal) setOn(line *core.Line) {
	line.Background = strings.Join(strings.Fields(v.text.String()), " ")
	if line.Background == "" {
		return
	}
	if v.begin != nil && (line.Begin == nil || *v.begin != *line.Begin) {
		line.BackgroundTiming.Begin = v.begin
	}
	if v.end != nil && (line.End == nil || *v.end != *line.End) {
		line.BackgroundTiming.End = v.end
	}
}
