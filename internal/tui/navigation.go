package tui

import (
	"fmt"
	"maps"
	"slices"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

// problemRoot replaces the previous problem flow with the destination list.
// Keep the initial pack picker, when present, as the list's parent.
func (m *model) problemRoot() {
	picker := m.fromPicker()
	m.commandList()
	if picker {
		m.stack = []screen{screenPacks, screenList}
	}
	m.sub, m.subRep, m.subCases = nil, nil, nil
}

// focusProblem keeps every list constraint that includes p, relaxing only
// those that would hide it. Both startup and command navigation use this path.
func (m *model) focusProblem(p *problem.Problem) error {
	if m.pack != p.Pack {
		if err := m.commandPack(p.Pack); err != nil {
			return err
		}
	}
	t := m.tab()
	if t == nil || !slices.ContainsFunc(t.probs, func(q *problem.Problem) bool { return q.Ref() == p.Ref() }) {
		return fmt.Errorf("problem %s is no longer in the pack", p.Ref())
	}
	m.selectProblems()
	if !slices.ContainsFunc(m.probs, func(q *problem.Problem) bool { return q.Ref() == p.Ref() }) {
		m.groups[m.pack] = ""
		m.selectProblems()
	}
	match, hint := parseSearch(m.search.Value())
	if hint != "" || !match(p) {
		m.search.SetValue("")
	}
	if !m.matchesFilter(p) {
		m.filter = filterAll
	}
	m.searching = false
	m.search.Blur()
	m.applyFilter()
	for i, idx := range m.visible {
		if m.probs[idx].Ref() == p.Ref() {
			m.cursor = i
			break
		}
	}
	h := max(m.listHeight(), 1)
	m.offset = max(0, min(m.offset, m.cursor))
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	return m.remember()
}

// Temporary pages keep navigation state separately from live workspace data
// and operations. Peer replacements retain the existing return destination.
type pageReturn struct {
	depth    int
	saved    *model
	casesKey string
}

func (m *model) savePage() *model {
	s := *m
	s.pageReturns, s.command, s.commandHistory = nil, nil, nil
	s.stack = slices.Clone(m.stack)
	s.visible = slices.Clone(m.visible)
	s.groups = maps.Clone(m.groups)
	s.histories = slices.Clone(m.histories)
	s.subs = slices.Clone(m.subs)
	return &s
}

// childPage saves a caller only when opening a different kind of page.
// Replacing a peer must not create another return step.
func (m *model) childPage(target screen) *model {
	if m.top() == target {
		return nil
	}
	return m.savePage()
}

func (m *model) rememberPage(s *model) {
	if s == nil {
		return
	}
	top := m.top()
	m.stack = append(slices.Clone(s.stack), top)
	m.pageReturns = append(m.pageReturns, pageReturn{depth: len(m.stack), saved: s, casesKey: s.commandCasesKey()})
	m.layout()
}

func (m *model) popPage() bool {
	n := len(m.pageReturns)
	if n == 0 || m.pageReturns[n-1].depth != len(m.stack) {
		return false
	}
	frame := m.pageReturns[n-1]
	s := frame.saved
	m.pageReturns[n-1] = pageReturn{}
	m.pageReturns = m.pageReturns[:n-1]
	m.stack, m.pack, m.groups = s.stack, s.pack, s.groups
	m.current, m.lang, m.viewing = s.current, s.lang, s.viewing
	m.search, m.searching, m.filter = s.search, s.searching, s.filter
	m.probs, m.visible = s.probs, s.visible
	m.opts.Pack = s.opts.Pack
	m.cursor, m.offset = s.cursor, s.offset
	m.picking, m.groupTarget, m.groupSearch, m.groupSearching, m.groupCursor = s.picking, s.groupTarget, s.groupSearch, s.groupSearching, s.groupCursor
	m.packCursor = s.packCursor
	m.detailTab, m.caseList = s.detailTab, s.caseList
	m.histories, m.subs, m.subCursor, m.subOffset = s.histories, s.subs, s.subCursor, s.subOffset
	m.sub, m.subRep, m.subCases, m.subBr = s.sub, s.subRep, s.subCases, s.subBr
	if s.top() == screenResult {
		m.run, m.resBr = s.run, s.resBr
	}
	m.guide = s.guide
	// Reconcile saved identities with data refreshed by a command above them.
	if err := m.reloadCommand(); err != nil {
		m.commandFallback()
		m.message = err.Error()
	}
	if m.current != nil && m.top() == screenDetail {
		m.caseList.verbose = s.caseList.verbose
		if len(s.caseList.numbers) > 0 {
			if selected, err := app.Select(m.caseList.cases, s.caseList.numbers, true); err == nil {
				m.caseList.cases = nil
				m.caseList.numbers = slices.Clone(s.caseList.numbers)
				m.caseList.inputs = nil
				for _, c := range selected {
					m.caseList.cases = append(m.caseList.cases, c.Case)
				}
				m.caseList.cursor = min(s.caseList.cursor, max(len(selected)-1, 0))
			}
		}
	}
	if frame.casesKey != "" && m.top() == screenResult && frame.casesKey != m.commandCasesKey() {
		m.run = nil
		m.stack[len(m.stack)-1] = screenDetail
		m.message = "the problem's cases changed; run the solution again"
	}
	if err := m.remember(); err != nil {
		m.message = err.Error()
	}
	m.layout()
	m.detail.SetYOffset(s.detail.YOffset())
	m.casesView.SetYOffset(s.casesView.YOffset())
	m.result.SetYOffset(s.result.YOffset())
	m.subView.SetYOffset(s.subView.YOffset())
	return true
}
