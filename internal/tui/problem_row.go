package tui

import (
	"strings"

	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

// problemColumns is shared by the problem list and the group editor. Widths
// exclude the cursor and status marks, which each screen supplies separately.
type problemColumns struct {
	id, title, level, tags int
}

func compactProblemColumns(width int) problemColumns {
	c := problemColumns{id: 6, level: 8}
	// Keep difficulty visible before allocating space to tags. Narrow panes
	// use the remaining space for the title; wide panes match the main list.
	c.title = min(max(width-c.id-c.level-2, 1), 48)
	if remaining := width - c.id - c.title - c.level - 3; remaining >= 12 {
		c.tags = remaining
	}
	return c
}

func (c problemColumns) header(prefix int) string {
	r := newRow(nil)
	r.plain(strings.Repeat(" ", prefix))
	c.cells(r, "#", "TITLE", "LEVEL", "TAGS", ui.Faint, ui.Faint)
	return r.b.String()
}

func (c problemColumns) cells(r *row, id, title, level, tags string, titleStyle, levelStyle ui.Style) {
	r.add(ui.Faint, fit(id, c.id)+" ")
	r.add(titleStyle, fit(title, c.title)+" ")
	r.add(levelStyle, fit(level, c.level))
	if c.tags > 0 {
		r.add(ui.Faint, " "+fit(tags, c.tags))
	}
}

func (m *model) problemCells(r *row, p *problem.Problem, c problemColumns, current bool) {
	titleStyle := ui.Plain
	if current {
		titleStyle = titleStyle.Bold(true)
	}
	c.cells(r, p.ID, m.title(p), difficultyLabel(p.Difficulty), strings.Join(p.Tags, " · "), titleStyle, difficultyStyle(p.Difficulty))
}
