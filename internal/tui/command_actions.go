package tui

import (
	"fmt"
	"maps"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/store"
)

// A native command can open another problem on an already-open detail page.
// Save navigation state separately from live workspace data and operations.
// Ordinary shortcuts keep using the existing screen stack.
type commandReturn struct {
	depth    int
	saved    *model
	casesKey string
}

func (m *model) saveCommandPage() *model {
	s := *m
	s.commandReturns, s.command, s.commandHistory = nil, nil, nil
	s.stack = slices.Clone(m.stack)
	s.groups = maps.Clone(m.groups)
	s.histories = slices.Clone(m.histories)
	s.subs = slices.Clone(m.subs)
	return &s
}

func (m *model) rememberCommandPage(s *model) {
	top := m.top()
	m.stack = append(slices.Clone(s.stack), top)
	m.commandReturns = append(m.commandReturns, commandReturn{depth: len(m.stack), saved: s, casesKey: s.commandCasesKey()})
	m.layout()
}

func (m *model) popCommandPage() bool {
	n := len(m.commandReturns)
	if n == 0 || m.commandReturns[n-1].depth != len(m.stack) {
		return false
	}
	frame := m.commandReturns[n-1]
	s := frame.saved
	m.commandReturns[n-1] = commandReturn{}
	m.commandReturns = m.commandReturns[:n-1]
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

// commandList goes to the navigation root without accumulating list pages.
func (m *model) commandList() {
	m.stack = []screen{screenList}
	m.commandReturns = nil
	m.histories = nil
	m.picking, m.groupSearching, m.groupTarget = false, false, nil
	m.searching = false
	m.search.Blur()
	m.layout()
}

func (m *model) commandPack(name string) error {
	if name == "" {
		return nil
	}
	k, err := m.app.WS.Pack(name)
	if err != nil {
		return err
	}
	if k.Err != nil {
		return k.Err
	}
	if slices.IndexFunc(m.tabs, func(t packTab) bool { return t.pack.Name == name }) < 0 {
		// An explicitly requested empty pack is still a valid destination.
		m.opts.Pack = name
		if err := m.reload(); err != nil {
			return err
		}
	}
	m.pack = name
	return m.loadGroup(name)
}

func (m *model) nativeProblem(c Command) (*problem.Problem, lang.Language, error) {
	p, err := m.app.WS.Problem(c.Problem)
	if err != nil {
		return nil, nil, err
	}
	var l lang.Language
	if c.File != "" {
		l, err = m.app.FileLang(c.File, c.Language)
		if err == nil {
			err = m.app.CheckLang(p, l)
		}
	} else {
		l, _, err = m.app.SolutionLang(p, c.Language)
	}
	if err != nil {
		return nil, nil, err
	}
	if c.Locale != "" {
		k, e := m.app.WS.Pack(p.Pack)
		if e != nil {
			return nil, nil, e
		}
		if e = app.CheckLocale(k, c.Locale); e != nil {
			return nil, nil, e
		}
	}
	return p, l, nil
}

func (m *model) nativeCommand(c Command) (tea.Cmd, error) {
	switch c.Action {
	case "quit":
		return tea.Quit, nil
	case "help":
		saved := m.saveCommandPage()
		m.openGuide(c.Help)
		m.rememberCommandPage(saved)
		return nil, nil
	case "list", "group", "group list", "group show", "pack list":
		packName := c.Pack
		groupName := c.Group
		if groupName != "" {
			g, err := m.app.ResolveGroup(packName, groupName)
			if err != nil {
				return nil, err
			}
			packName, groupName = g.Pack, g.Key()
		}
		var l lang.Language
		if c.Language != "" {
			var err error
			l, err = m.app.Lang(c.Language)
			if err != nil {
				return nil, err
			}
		}
		if err := m.commandPack(packName); err != nil {
			return nil, err
		}
		if l != nil {
			m.lang = l
		}
		if packName != "" || groupName != "" {
			if groupName != "" {
				m.groups[m.pack] = groupName
			}
			m.search.SetValue("")
			m.filter = filterAll
			m.show()
		}
		m.commandList()
		switch c.Action {
		case "group", "group list":
			m.openGroups()
		case "pack list":
			m.stack = []screen{screenPacks}
			m.packCursor = max(slices.IndexFunc(m.tabs, func(t packTab) bool { return t.pack.Name == m.pack }), 0)
		}
		return nil, nil
	case "history", "history show":
		var p *problem.Problem
		var sub store.Submission
		var err error
		if c.Action == "history show" {
			st, e := m.app.Store()
			if e != nil {
				return nil, e
			}
			sub, err = st.Get(m.ctx, c.ID)
			if err == nil {
				p = m.commandProblem(sub.Ref())
			}
		} else if c.Problem != "" {
			p, err = m.app.WS.Problem(c.Problem)
		}
		if err != nil {
			return nil, err
		}
		saved := m.saveCommandPage()
		// openHistory normally deduplicates a history screen. Commands may
		// intentionally replace its scope; preserve the previous scope for Esc.
		if m.top() == screenHistory {
			m.stack = append(slices.Clone(m.stack), screenList)
		}
		if c.Action == "history" {
			m.openHistory(p)
		} else {
			m.current = p
			m.openSubmission(&sub)
		}
		m.rememberCommandPage(saved)
		return nil, nil
	case "show", "open", "cases list", "test", "submit":
		p, l, err := m.nativeProblem(c)
		if err != nil {
			return nil, err
		}
		if c.Action == "test" && c.File == "" || c.Action == "submit" {
			if !m.app.HasSolution(p, l) {
				return nil, fmt.Errorf("no %s solution yet; use :open %s -l %s", l.Name(), p.Ref(), l.ID())
			}
			if c.Action == "submit" {
				if err = m.app.SubmitError(p, l); err != nil {
					return nil, err
				}
			}
		}
		var cases caseList
		if c.Action == "cases list" {
			all, e := m.app.Cases(p)
			if e != nil {
				return nil, e
			}
			sel, e := app.Select(all, c.Cases, true)
			if e != nil {
				return nil, e
			}
			cases.verbose = c.Verbose
			for _, s := range sel {
				cases.cases = append(cases.cases, s.Case)
				if len(c.Cases) > 0 {
					cases.numbers = append(cases.numbers, s.Index)
				}
			}
		}
		saved := m.saveCommandPage()
		if err = m.commandPack(p.Pack); err != nil {
			return nil, err
		}
		if saved.pack != m.pack {
			m.search.SetValue("")
			m.searching = false
			m.filter = filterAll
			m.show()
		}
		m.lang = l
		if c.Locale != "" {
			m.locales[p.Pack].current = c.Locale
		}
		m.current = p
		var cmd tea.Cmd
		switch c.Action {
		case "show", "open", "cases list":
			m.openDetail(p, c.Action == "open")
			if c.Action == "cases list" {
				m.detailTab = tabProblemCases
				m.caseList = cases
				m.renderDetail()
			}
		case "test", "submit":
			if c.EditNote {
				m.openNoteForm(store.Submission{Pack: p.Pack, Slug: p.Slug, Note: c.Note})
				m.noteForm.submit = &c
			} else {
				cmd = m.startCommandRun(c, p)
			}
		}
		m.rememberCommandPage(saved)
		return cmd, nil
	}
	return nil, fmt.Errorf("unknown TUI action %q", c.Action)
}

func (m *model) startCommandRun(c Command, p *problem.Problem) tea.Cmd {
	kind := runTest
	if c.Action == "submit" {
		kind = runSubmit
	}
	cmd := m.runWith(kind, p, c.Run, c.Note, c.Verbose, c.File)
	m.run.alternate = c.Custom || c.File != ""
	return cmd
}
