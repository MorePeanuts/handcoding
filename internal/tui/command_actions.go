package tui

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/store"
)

// commandList goes to the navigation root without accumulating list pages.
func (m *model) commandList() {
	m.stack = []screen{screenList}
	m.pageReturns = nil
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
		saved := m.childPage(screenGuide)
		m.openGuide(c.Help)
		m.rememberPage(saved)
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
		if c.Action == "history" {
			m.openHistory(p)
		} else {
			// A submission belongs to a history list, even when reached directly.
			if m.top() == screenSubmission {
				m.pop()
			}
			if m.top() != screenHistory || m.historyProblem() != nil && m.historyProblem().Ref() != sub.Ref() {
				m.openHistory(p)
			}
			for i := range m.subs {
				if m.subs[i].ID == sub.ID {
					m.subCursor = i
					break
				}
			}
			m.current = p
			m.openSubmission(&sub)
		}
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
		if c.Action == "show" || c.Action == "open" {
			if err := m.focusProblem(p); err != nil {
				return nil, err
			}
			m.problemRoot()
			m.lang = l
			if c.Locale != "" {
				m.locales[p.Pack].current = c.Locale
			}
			m.openDetail(p, c.Action == "open")
			return nil, nil
		}
		var saved *model
		if c.EditNote || c.Action == "cases list" && (m.top() != screenDetail || m.detailTab != tabProblemCases) {
			saved = m.savePage()
		} else if c.Action != "cases list" {
			saved = m.childPage(screenResult)
		}
		oldPack := m.pack
		if err = m.commandPack(p.Pack); err != nil {
			return nil, err
		}
		if oldPack != m.pack {
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
		case "cases list":
			m.openDetail(p, false)
			m.detailTab = tabProblemCases
			m.caseList = cases
			m.renderDetail()
		case "test", "submit":
			if c.EditNote {
				m.openNoteForm(store.Submission{Pack: p.Pack, Slug: p.Slug, Note: c.Note})
				m.noteForm.submit = &c
			} else {
				cmd = m.startCommandRun(c, p)
			}
		}
		m.rememberPage(saved)
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
