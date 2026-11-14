package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

type historyColumn struct {
	title string
	width int
}

// historyTable uses the same column widths for headings and rows. Problem IDs
// come from the loaded pack/slug identities, never from a submission ID or slug.
type historyTable struct {
	columns    []historyColumn
	problemIDs map[string]string
	qualify    bool
}

func (m *model) historyTable() historyTable {
	t := historyTable{columns: []historyColumn{
		{"SUB #", 6}, {"VERDICT", 20}, {"LANG", 8}, {"PASSED", 8}, {"TIME", 8}, {"SUBMITTED", 16},
	}}
	if m.historyAll() {
		t.qualify = m.qualifySubs()
		t.problemIDs = make(map[string]string)
		for _, tab := range m.tabs {
			for _, p := range tab.probs {
				if p.ID != "" {
					t.problemIDs[p.Ref()] = p.ID
				}
			}
		}
		idW, probW, subW := 2, 12, 6
		for _, s := range m.subs {
			idW = max(idW, runewidth.StringWidth(t.problemID(s)))
			probW = max(probW, runewidth.StringWidth(subName(s, t.qualify)))
			subW = max(subW, len(fmt.Sprintf("#%d", s.ID)))
		}
		t.columns = []historyColumn{
			{"ID", min(idW, 12)}, {"PROBLEM", min(probW, 48)}, {"VERDICT", 20}, {"LANG", 8},
			{"PASSED", 8}, {"TIME", 8}, {"SUBMITTED", 16}, {"SUB #", subW},
		}
		// First shorten the problem name, leaving a little space for comments
		// when possible. Then remove whole secondary columns, not clipped tails.
		desiredProblem := t.columns[1].width
		t.columns[1].width = max(12, desiredProblem-max(t.width()+8-m.width, 0))
		for _, title := range []string{"TIME", "PASSED", "SUB #", "SUBMITTED", "LANG"} {
			if t.width() <= m.width {
				break
			}
			i := slices.IndexFunc(t.columns, func(c historyColumn) bool { return c.title == title })
			t.columns = slices.Delete(t.columns, i, i+1)
		}
		// Very small terminals still split their space between the three primary
		// columns. The outer frame handles widths smaller than their headings.
		for _, i := range []int{1, 2, 0} {
			if extra := t.width() - m.width; extra > 0 {
				t.columns[i].width = max(3, t.columns[i].width-extra)
			}
		}
		if spare := m.width - t.width() - 8; spare > 0 {
			t.columns[1].width = min(desiredProblem, t.columns[1].width+spare)
		}
	}
	if room := m.width - t.width(); room >= len("COMMENT") {
		t.columns = append(t.columns, historyColumn{"COMMENT", room})
	}
	return t
}

// width includes the cursor and each column's following separator.
func (t historyTable) width() int {
	w := 2
	for _, c := range t.columns {
		w += c.width + 1
	}
	return w
}

func (t historyTable) problemID(s store.Submission) string {
	if id := t.problemIDs[s.Ref()]; id != "" {
		return id
	}
	return "—"
}

func (t historyTable) header() string {
	var b strings.Builder
	b.WriteString("  ")
	for i, c := range t.columns {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(fit(c.title, c.width))
	}
	return b.String()
}

func (t historyTable) render(r *row, s store.Submission) {
	for i, c := range t.columns {
		if i > 0 {
			r.plain(" ")
		}
		var value string
		style := ui.Plain
		switch c.title {
		case "ID":
			value, style = t.problemID(s), ui.Faint
		case "PROBLEM":
			value = subName(s, t.qualify)
		case "SUB #":
			value, style = fmt.Sprintf("#%d", s.ID), ui.Faint
		case "VERDICT":
			value, style = s.Verdict, ui.VerdictStyle(runner.Verdict(s.Verdict))
		case "LANG":
			value = s.Lang
		case "PASSED":
			value = fmt.Sprintf("%d/%d", s.Passed, s.Total)
		case "TIME":
			value = ui.SubmissionTime(s.ElapsedMs)
		case "SUBMITTED":
			value, style = s.CreatedAt.Format("2006-01-02 15:04"), ui.Faint
		case "COMMENT":
			value = s.Note.Comment
		}
		r.add(style, fit(value, c.width))
	}
}
