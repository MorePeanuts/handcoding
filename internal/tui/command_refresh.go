package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

// finishCommand reconciles the live view even when the command failed: pack
// updates and agent commands can have made useful partial changes.
func (m *model) finishCommand(msg commandDoneMsg) tea.Cmd {
	top := m.top()
	detailOffset, casesOffset := m.detail.YOffset(), m.casesView.YOffset()
	resultOffset, subOffset := m.result.YOffset(), m.subView.YOffset()
	m.message = ""
	if msg.err != nil {
		m.message = ui.Yellow.Render("command finished: " + msg.err.Error())
	}
	if err := m.reloadCommand(); err != nil {
		m.message = ui.Red.Render("cannot refresh workspace: " + err.Error())
		m.commandFallback()
	}
	if msg.casesKey != "" && m.run != nil && msg.casesKey != m.commandCasesKey() {
		m.run = nil
		if m.top() == screenResult {
			m.pop()
		}
		m.message = ui.Yellow.Render("the problem's cases changed; run the solution again")
	}
	m.opts.Images.Forget()
	m.layout()
	if m.top() == top {
		m.detail.SetYOffset(detailOffset)
		m.casesView.SetYOffset(casesOffset)
		m.result.SetYOffset(resultOffset)
		m.subView.SetYOffset(subOffset)
	}
	return m.themeReports()
}

func (m *model) reloadCommand() error {
	oldPack, oldGroup := m.pack, m.groups[m.pack]
	listOffset := m.offset
	var caseSelection *problem.Case
	if c, _ := m.selectedCase(); c != nil {
		copy := *c
		caseSelection = &copy
	}
	var packChoice, groupChoice string
	if m.packCursor >= 0 && m.packCursor < len(m.tabs) {
		packChoice = m.tabs[m.packCursor].pack.Name
	}
	if m.picking && m.tab() != nil {
		choices := m.groupChoices()
		if m.groupCursor >= 0 && m.groupCursor < len(choices) && choices[m.groupCursor] >= 0 {
			groupChoice = m.tab().groups()[choices[m.groupCursor]].Key()
		}
	}
	current := m.current
	var subID int64
	if s := m.selectedSub(); s != nil {
		subID = s.ID
	}
	if err := m.app.ReloadWorkspace(); err != nil {
		return err
	}
	if m.lang != nil {
		l, err := m.app.Lang(m.lang.ID())
		if err != nil {
			return err
		}
		m.lang = l
	}
	if err := m.reload(); err != nil {
		return err
	}
	if m.pack == oldPack {
		m.offset = min(listOffset, max(len(m.visible)-1, 0))
	}
	m.packCursor = max(slices.IndexFunc(m.tabs, func(t packTab) bool { return t.pack.Name == packChoice }), 0)
	if oldPack != m.pack {
		m.picking, m.groupTarget, m.groupSearching = false, nil, false
		m.message = ui.Yellow.Render("pack " + oldPack + " is no longer available")
	} else if oldGroup != "" && m.groups[m.pack] == "" {
		m.message = ui.Yellow.Render("group " + oldGroup + " is no longer available; showing all problems")
	}
	if m.picking {
		if m.tab() == nil {
			m.picking, m.groupTarget = false, nil
		} else {
			choices := m.groupChoices()
			m.groupCursor = min(m.groupCursor, max(len(choices)-1, 0))
			for i, n := range choices {
				if n == -1 && groupChoice == "" || n >= 0 && m.tab().groups()[n].Key() == groupChoice {
					m.groupCursor = i
					break
				}
			}
		}
	}
	if m.groupTarget != nil {
		m.groupTarget = m.commandProblem(m.groupTarget.Ref())
		if m.groupTarget == nil {
			m.picking = false
			m.message = ui.Yellow.Render("the problem to add is no longer available")
		}
	}
	if current != nil {
		m.current = m.commandProblem(current.Ref())
		if m.current == nil && (slices.Contains(m.stack, screenDetail) || slices.Contains(m.stack, screenResult)) {
			m.commandFallback()
			m.message = ui.Yellow.Render("problem " + current.Ref() + " is no longer available")
		}
	}
	if m.run != nil && m.commandProblem(m.run.ref) == nil {
		m.run = nil
	}
	if m.current != nil {
		m.loadCases(m.current)
		if caseSelection != nil {
			m.caseList.cursor = 0
			for i, c := range m.caseList.cases {
				if reflect.DeepEqual(c, *caseSelection) {
					m.caseList.cursor = i
					break
				}
			}
		}
	}
	for i := range m.histories {
		if p := m.histories[i].p; p != nil {
			if fresh := m.commandProblem(p.Ref()); fresh != nil {
				m.histories[i].p = fresh
			}
			// A removed problem can still have history; retain its identity
			// instead of silently switching to workspace-wide history.
		}
	}
	if len(m.histories) > 0 {
		m.loadHistory()
		if i := slices.IndexFunc(m.subs, func(s store.Submission) bool { return s.ID == subID }); i >= 0 {
			m.subCursor = i
		}
	}
	if m.sub != nil {
		st, err := m.app.Store()
		if err != nil {
			return err
		}
		s, err := st.Get(m.ctx, m.sub.ID)
		if err != nil {
			id := m.sub.ID
			m.sub, m.subRep, m.subCases = nil, nil, nil
			if m.top() == screenSubmission {
				m.pop()
				m.message = ui.Yellow.Render(fmt.Sprintf("submission %d is no longer available", id))
			}
		} else {
			m.sub = &s
			m.subRep = app.DecodeReport(s.Details)
			m.subCases = nil
			if m.current != nil {
				m.subCases = app.SubmissionCases(m.current, s.Details)
			}
		}
	}
	return nil
}

// commandCasesKey detects changes that could make the previous result's case
// indexes point at different inputs after a command. Cases stay compact here:
// generated cases are hashed as their generator specifications, not expanded.
func (m *model) commandCasesKey() string {
	if m.run == nil {
		return ""
	}
	p := m.commandProblem(m.run.ref)
	if p == nil {
		return ""
	}
	cases, err := m.app.Cases(p)
	if err != nil {
		return ""
	}
	data, err := json.Marshal(struct {
		Problem *problem.Problem
		Cases   []problem.Case
	}{p, cases})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (m *model) commandProblem(ref string) *problem.Problem {
	for _, tab := range m.tabs {
		for _, p := range tab.probs {
			if p.Ref() == ref {
				return p
			}
		}
	}
	return nil
}

func (m *model) commandFallback() {
	m.commandReturns = nil
	m.stack = []screen{screenList}
	m.current, m.run, m.sub, m.subRep, m.subCases = nil, nil, nil, nil, nil
	m.histories = nil
	m.picking, m.groupTarget, m.groupSearching = false, nil, false
}
