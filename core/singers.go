package core

import (
	"errors"
	"fmt"
	"slices"
)

var errNoSuchSinger = errors.New("no such singer")

// NewAddSinger returns the command that adds a singer named name (which may
// be empty). The first one added is the second singer: with none yet, the
// lyrics already had one, unnamed. If line is not -1, that line is given to
// the new singer in the same step, so one undo takes both back.
func NewAddSinger(p *Project, name string, line int) (Command, error) {
	if line != -1 && (line < 0 || line >= len(p.Lines)) {
		return nil, fmt.Errorf("%w: %d", errNoSuchLine, line)
	}
	singers := slices.Clone(p.Singers)
	if len(singers) == 0 {
		singers = append(singers, Singer{})
	}
	singers = append(singers, Singer{Name: name})
	c := &setSingers{old: p.Singers, singers: singers, line: line}
	if line >= 0 {
		c.oldSinger, c.singer = p.Lines[line].Singer, len(singers)-1
	}

	return c, nil
}

// NewRenameSinger returns the command that renames singer i. An empty name
// goes back to "Singer 2" and so on.
func NewRenameSinger(p *Project, i int, name string) (Command, error) {
	if i < 0 || i >= max(len(p.Singers), 1) {
		return nil, fmt.Errorf("%w: %d", errNoSuchSinger, i)
	}
	singers := slices.Clone(p.Singers)
	if len(singers) == 0 {
		singers = append(singers, Singer{}) // the one singer the lyrics always have
	}
	singers[i].Name = name

	return &setSingers{old: p.Singers, singers: singers, line: -1}, nil
}

// setSingers replaces the singer list, and gives one line a new singer
// unless line is -1.
type setSingers struct {
	old, singers      []Singer
	line              int
	oldSinger, singer int
}

func (c *setSingers) Do(p *Project) {
	p.Singers = c.singers
	if c.line >= 0 {
		p.Lines[c.line].Singer = c.singer
	}
}

func (c *setSingers) Undo(p *Project) {
	p.Singers = c.old
	if c.line >= 0 {
		p.Lines[c.line].Singer = c.oldSinger
	}
}

// NewSetSinger returns the command that gives line i to singer (an index
// into p.Singers, or Everyone).
func NewSetSinger(p *Project, i, singer int) (Command, error) {
	if i < 0 || i >= len(p.Lines) {
		return nil, fmt.Errorf("%w: %d", errNoSuchLine, i)
	}
	if singer != Everyone && (singer < 0 || singer >= max(len(p.Singers), 1)) {
		return nil, fmt.Errorf("%w: %d", errNoSuchSinger, singer)
	}

	return &setSinger{line: i, old: p.Lines[i].Singer, singer: singer}, nil
}

type setSinger struct {
	line        int
	old, singer int
}

func (c *setSinger) Do(p *Project)   { p.Lines[c.line].Singer = c.singer }
func (c *setSinger) Undo(p *Project) { p.Lines[c.line].Singer = c.old }

// NewRemoveSinger returns the command that removes singer i, such as one
// added by mistake. Lines it sang go to the first singer left, and the
// singers after it move up one. The last singer can't be removed.
func NewRemoveSinger(p *Project, i int) (Command, error) {
	if i < 0 || i >= len(p.Singers) || len(p.Singers) < 2 {
		return nil, fmt.Errorf("%w: %d", errNoSuchSinger, i)
	}
	c := &removeSinger{
		old:     p.Singers,
		singers: slices.Delete(slices.Clone(p.Singers), i, i+1),
		oldBy:   make([]int, len(p.Lines)),
		by:      make([]int, len(p.Lines)),
	}
	for j, line := range p.Lines {
		c.oldBy[j] = line.Singer
		switch {
		case line.Singer == i:
			c.by[j] = 0
		case line.Singer > i:
			c.by[j] = line.Singer - 1
		default:
			c.by[j] = line.Singer // earlier singers and Everyone keep their lines
		}
	}

	return c, nil
}

// removeSinger replaces the singer list and who sings each line.
type removeSinger struct {
	old, singers []Singer
	oldBy, by    []int // each line's singer
}

func (c *removeSinger) Do(p *Project)   { c.set(p, c.singers, c.by) }
func (c *removeSinger) Undo(p *Project) { c.set(p, c.old, c.oldBy) }

func (*removeSinger) set(p *Project, singers []Singer, by []int) {
	p.Singers = singers
	for j, singer := range by {
		p.Lines[j].Singer = singer
	}
}
