package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
)

type scrollKind int

const (
	scrollItem scrollKind = iota
	scrollLine
	scrollHalf
	scrollPage
	scrollEdge
)

// scrollKey describes browsing keys only. Text fields keep their own bindings.
func scrollKey(key string, height int) (scrollKind, int, bool) {
	switch key {
	case "j", "down":
		return scrollItem, 1, true
	case "k", "up":
		return scrollItem, -1, true
	case "ctrl+e":
		return scrollLine, 1, true
	case "ctrl+y":
		return scrollLine, -1, true
	case "ctrl+d":
		return scrollHalf, max(height/2, 1), true
	case "ctrl+u":
		return scrollHalf, -max(height/2, 1), true
	case "ctrl+f", "pgdown":
		return scrollPage, max(height-2, 1), true
	case "ctrl+b", "pgup":
		return scrollPage, -max(height-2, 1), true
	case "g", "home":
		return scrollEdge, -1, true
	case "G", "end":
		return scrollEdge, 1, true
	}
	return 0, 0, false
}

func bound(n, lo, hi int) int { return min(max(n, lo), max(lo, hi)) }

// keepVisible reconciles selection and viewport after resize/filter changes.
func keepVisible(cursor, offset *int, count, height int) {
	height = max(height, 1)
	*cursor = bound(*cursor, 0, count-1)
	*offset = bound(*offset, 0, count-height)
	if *cursor < *offset {
		*offset = *cursor
	}
	if *cursor >= *offset+height {
		*offset = *cursor - height + 1
	}
}

// moveScroll applies Neovim-style viewport/selection movement to lines.
// A half page moves both; line scrolling retains selection while visible;
// a full page lands at the entering edge, with two lines of overlap.
func moveScroll(kind scrollKind, delta int, cursor, offset *int, count, height int) {
	height = max(height, 1)
	keepVisible(cursor, offset, count, height)
	switch kind {
	case scrollItem:
		*cursor += delta
	case scrollEdge:
		if delta < 0 {
			*cursor = 0
		} else {
			*cursor = max(count-1, 0)
		}
	case scrollHalf:
		*cursor = bound(*cursor+delta, 0, count-1)
		*offset = bound(*offset+delta, 0, count-height)
	case scrollLine, scrollPage:
		next := bound(*offset+delta, 0, count-height)
		switch {
		case kind == scrollPage && next != *offset:
			if delta > 0 {
				*cursor = next
			} else {
				*cursor = min(next+height-1, count-1)
			}
		case kind == scrollPage:
			// At a boundary still allow the last/first item to be reached.
			if delta > 0 {
				*cursor = max(count-1, 0)
			} else {
				*cursor = 0
			}
		default:
			*cursor = bound(*cursor, next, min(next+height-1, count-1))
		}
		*offset = next
	}
	keepVisible(cursor, offset, count, height)
}

func scrollList(key string, cursor, offset *int, count, height int) bool {
	kind, delta, ok := scrollKey(key, height)
	if ok {
		moveScroll(kind, delta, cursor, offset, count, height)
	}
	return ok
}

// searchPage adds paging without stealing editing commands such as Ctrl+u.
func searchPage(key string, cursor, offset *int, count, height int) bool {
	if key != "pgup" && key != "pgdown" {
		return false
	}
	return scrollList(key, cursor, offset, count, height)
}

func scrollText(vp *viewport.Model, key string) bool {
	// Keep the viewport's existing pager aliases in text-only areas.
	switch key {
	case "f", "space":
		key = "ctrl+f"
	case "b":
		key = "ctrl+b"
	case "d":
		key = "ctrl+d"
	case "u":
		key = "ctrl+u"
	}
	kind, delta, ok := scrollKey(key, vp.Height())
	if !ok {
		return false
	}
	if kind == scrollEdge {
		if delta < 0 {
			vp.GotoTop()
		} else {
			vp.GotoBottom()
		}
	} else {
		vp.SetYOffset(vp.YOffset() + delta)
	}
	return true
}

// caseScroll anchors reading inside a case, whose expanded height can change
// when another case is selected or the terminal is resized. starts are actual
// rendered line offsets, not case counts.
type caseScroll struct {
	starts      []int
	manual      bool
	within, row int
}

func (s *caseScroll) place(vp *viewport.Model, cursor, top, bottom int) {
	if len(s.starts) == 0 {
		return
	}
	if !s.manual {
		follow(vp, top, bottom)
		return
	}
	cursor = bound(cursor, 0, len(s.starts)-1)
	end := vp.TotalLineCount()
	if cursor+1 < len(s.starts) {
		end = s.starts[cursor+1]
	}
	anchor := bound(s.starts[cursor]+s.within, 0, end-1)
	s.row = bound(s.row, 0, vp.Height()-1)
	vp.SetYOffset(anchor - s.row)
	s.within = anchor - s.starts[cursor]
	s.row = anchor - vp.YOffset()
}

func (s *caseScroll) update(key string, cursor *int, vp *viewport.Model, render func()) bool {
	// Existing pager aliases should move the same reading anchor. Plain d
	// remains the case's debug action, so it is deliberately not an alias.
	switch key {
	case "f", "space":
		key = "ctrl+f"
	case "b":
		key = "ctrl+b"
	case "u":
		key = "ctrl+u"
	}
	kind, delta, ok := scrollKey(key, vp.Height())
	if !ok {
		return false
	}
	if len(s.starts) == 0 {
		return scrollText(vp, key)
	}
	if kind == scrollItem || kind == scrollEdge {
		if kind == scrollEdge {
			if delta < 0 {
				*cursor = 0
			} else {
				*cursor = len(s.starts) - 1
			}
		} else {
			*cursor = bound(*cursor+delta, 0, len(s.starts)-1)
		}
		s.manual = false
		render()
		return true
	}
	*cursor = bound(*cursor, 0, len(s.starts)-1)
	anchor := s.starts[*cursor]
	if s.manual {
		anchor += s.within
	}
	offset := vp.YOffset()
	anchor = bound(anchor, offset, min(offset+vp.Height()-1, vp.TotalLineCount()-1))
	moveScroll(kind, delta, &anchor, &offset, vp.TotalLineCount(), vp.Height())
	target := 0
	for i, start := range s.starts {
		if start <= anchor {
			target = i
		} else {
			break
		}
	}
	s.manual, s.within, s.row = true, anchor-s.starts[target], anchor-offset
	*cursor = target
	render()
	return true
}

var scrollKeys = []string{"j/k,↓/↑ next/previous item or text line", "ctrl+e/y scroll down/up one line", "ctrl+d/u half page (move selection with view)", "ctrl+f/b,pgdown/pgup page down/up (2-line overlap)", "g/G,home/end first/last"}

// Routine navigation belongs in full help, not the scarce footer space.
func navigationHint(hint string) bool {
	key, _, _ := strings.Cut(hint, " ")
	switch key {
	case "↑↓", "j/k", "j/k,↑↓", "ctrl+d/u", "ctrl+e/y", "ctrl+f/b", "pgup/pgdn", "pgup/dn", "pgup/pgdown", "g/G":
		return true
	}
	return false
}
