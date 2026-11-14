package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/mattn/go-runewidth"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

// The tabs of the detail screen: the statement, and the problem's cases.
const (
	tabStatement = iota
	tabProblemCases
)

// caseList is what the cases tab of the detail screen shows: the problem's
// cases followed by the user's own (app.Cases), and the selected one,
// whose details open under it.
type caseList struct {
	scroll  caseScroll
	numbers []int // original case numbers for command-filtered views
	verbose bool
	cases   []problem.Case
	err     error
	cursor  int
	inputs  map[int]string // the cases' inputs as shown, by index
}

// loadCases reads p's cases for the cases tab, keeping the cursor where
// it was, within them.
func (m *model) loadCases(p *problem.Problem) {
	cases, err := m.app.Cases(p)
	m.caseList = caseList{cases: cases, err: err, cursor: min(m.caseList.cursor, max(len(cases)-1, 0))}
}

// selectedCase is the case under the cursor of the cases tab, and its
// number; nil without one.
func (m *model) selectedCase() (*problem.Case, int) {
	cl := &m.caseList
	if cl.cursor < 0 || cl.cursor >= len(cl.cases) {
		return nil, 0
	}
	return &cl.cases[cl.cursor], cl.number(cl.cursor)
}

func (cl *caseList) number(i int) int {
	if len(cl.numbers) == len(cl.cases) {
		return cl.numbers[i]
	}
	return i + 1
}

// detailTabs draws the tabs of the detail screen.
func (m *model) detailTabs() string {
	cases := "Cases"
	if n := len(m.caseList.cases); n > 0 {
		cases = fmt.Sprintf("Cases %d", n)
	}
	var s strings.Builder
	for i, name := range []string{"Statement", cases} {
		if i == m.detailTab {
			s.WriteString(m.st.tabOn.Render(" " + name + " "))
		} else {
			s.WriteString(m.st.tab.Render(" " + name + " "))
		}
	}
	return s.String()
}

// renderCaseList draws the cases tab: a summary, a line per case and the
// selected case's details in a box under it.
func (m *model) renderCaseList() {
	p, cl := m.current, &m.caseList
	cl.scroll.starts = nil
	width := m.casesView.Width()
	if cl.err != nil {
		m.casesView.SetContent(wrap(ui.Red.Render(cl.err.Error()), width))
		return
	}
	user := 0
	for _, c := range cl.cases {
		if c.User {
			user++
		}
	}
	var l lines
	summary := "No cases yet: a adds one."
	switch {
	case m.app.KeepsUserCases(p):
		summary = fmt.Sprintf("%d of the pack, %d of yours · t runs the examples and yours, s only the pack's", len(cl.cases)-user, user)
	case len(cl.cases) > 0:
		summary = fmt.Sprintf("%s · t runs the examples, s all of them", plural(len(cl.cases), "case"))
	}
	l.add(wrap(ui.Faint.Render(summary), width))
	l.add("")
	top, bottom := 0, 0
	for i, c := range cl.cases {
		cl.scroll.starts = append(cl.scroll.starts, l.n)
		cur := i == cl.cursor
		r := newRow(nil)
		if cur {
			r = newRow(m.st.sel)
			top = l.n
		}
		m.cursorBar(r, cur)
		r.plain(fmt.Sprintf("Case %-4d", cl.number(i)))
		var tags []string
		if c.Example {
			r.add(ui.Cyan, "example ")
		}
		if c.User {
			r.add(m.st.accent, "yours ")
		}
		if c.Generated() {
			r.add(ui.Faint, "generated ")
		}
		if c.Note != "" {
			tags = append(tags, c.Note)
		}
		r.add(ui.Faint, strings.Join(tags, " · "))
		l.add(clip(r.fill(width), width))
		if cur {
			l.add(m.caseDetails(p, c, cl.number(i), width))
			bottom = l.n - 1
		}
	}
	m.casesView.SetContent(l.b.String())
	cl.scroll.place(&m.casesView, cl.cursor, top, bottom)
}

// caseDetails draws case index of p in a box: its input, or its generator
// call, and its expected output.
func (m *model) caseDetails(p *problem.Problem, c problem.Case, index int, width int) string {
	width = max(width, 30)
	var s strings.Builder
	head := fmt.Sprintf("Case %d", index)
	what := ""
	switch {
	case c.User:
		what = "yours"
	case c.Example:
		what = "example"
	}
	top := ui.Faint.Render("╭─ ") + ui.Bold.Render(head)
	used := 3 + len(head)
	if what != "" {
		top += ui.Faint.Render(" · " + what)
		used += 3 + len(what)
	}
	s.WriteString(clip(top+ui.Faint.Render(" "+strings.Repeat("─", max(width-used-2, 0))+"╮"), width) + "\n")
	const labelW = 12
	inner := width - 2
	field := func(label, value string, vs ui.Style) {
		for i, line := range wrapCells(value, inner-labelW-1) {
			lab := ""
			if i == 0 {
				lab = label
			}
			pad := inner - labelW - runewidth.StringWidth(line)
			s.WriteString(ui.Faint.Render("│") + ui.Faint.Render(runewidth.FillRight("  "+lab, labelW)) + vs.Render(line) +
				strings.Repeat(" ", max(pad, 0)) + ui.Faint.Render("│") + "\n")
		}
	}
	more := fmt.Sprintf("hc cases list %s -c %d shows all", m.app.WS.Ref(p), index)
	trim := func(s string) string {
		if m.caseList.verbose {
			return s
		}
		return truncateWith(s, more)
	}
	if c.Generated() {
		field("Generate", c.Generate.String(), ui.Plain)
		field("Input", "built by generator.py when the case runs", ui.Faint)
	} else {
		field("Input", trim(m.caseInput(p, c, index)), ui.Plain)
	}
	expected := ui.ExpectedLabel(p)
	switch {
	case c.HasExpected():
		field(expected, trim(string(c.Expected)), ui.Green)
	case (c.User || c.Generated()) && len(m.app.ReferenceLangs(p)) > 0:
		field(expected, "computed by the reference solution when the case runs", ui.Faint)
	case c.User:
		field(expected, "none: without a reference solution, the case shows your output", ui.Faint)
	default:
		field(expected, "missing", ui.Yellow)
	}
	if c.Note != "" {
		field("Note", c.Note, ui.Faint)
	}
	s.WriteString(ui.Faint.Render("╰" + strings.Repeat("─", inner) + "╯"))
	return s.String()
}

// caseInput formats case index's input once: the box is drawn again on
// every key.
func (m *model) caseInput(p *problem.Problem, c problem.Case, index int) string {
	cl := &m.caseList
	if s, ok := cl.inputs[index]; ok {
		return s
	}
	if cl.inputs == nil {
		cl.inputs = map[int]string{}
	}
	s := p.FormatInput(c)
	cl.inputs[index] = s
	return s
}

// truncateWith shortens a long value, saying how much is left out and
// where to see all of it.
func truncateWith(s, more string) string {
	if len(s) <= maxValue {
		return s
	}
	return fmt.Sprintf("%s… (%d more bytes, %s)", s[:maxValue], len(s)-maxValue, more)
}

// updateCaseList handles the keys of the cases tab that are its own,
// reporting whether key was one.
func (m *model) updateCaseList(key string) (tea.Cmd, bool) {
	if m.caseList.scroll.update(key, &m.caseList.cursor, &m.casesView, m.renderCaseList) {
		return nil, true
	}
	switch key {
	case "x":
		m.removeCase()
	case "d":
		if _, index := m.selectedCase(); index > 0 {
			return m.debug(m.current, m.lang, index), true
		}
		return m.debug(m.current, m.lang, 0), true
	default:
		return nil, false
	}
	m.renderCaseList()
	return nil, true
}

// removable reports whether the user may remove c: one of their own, or
// any case of a writable pack.
func (m *model) removable(c *problem.Case) bool {
	return c != nil && (c.User || !m.app.KeepsUserCases(m.current))
}

// removeCase asks to remove the selected case, then removes it.
func (m *model) removeCase() {
	p := m.current
	c, index := m.selectedCase()
	if c == nil {
		return
	}
	if !m.removable(c) {
		m.message = fmt.Sprintf("case %d is the pack's, which is read-only: only your own cases can be removed", index)
		return
	}
	what := fmt.Sprintf("case %d from %s", index, problem.CasesFile)
	if c.User {
		what = fmt.Sprintf("your case %d", index)
	}
	m.ask("Remove "+what+"?", func() tea.Cmd {
		if _, err := m.app.RemoveCases(p, []int{index}); err != nil {
			m.message = ui.Red.Render(err.Error())
			return nil
		}
		m.loadCases(p)
		m.renderCaseList()
		m.message = ui.Red.Render("removed") + " " + what + "; later cases were renumbered"
		return nil
	})
}

// caseForm is the form that adds a case to the problem shown: a field per
// input, then the expected output and a note.
type caseForm struct {
	p *problem.Problem
	// user is set when the case is one of the user's own (see
	// app.KeepsUserCases).
	user   bool
	keys   []string // the input keys, the first fields
	fields []textinput.Model
	labels []string
	focus  int
	err    string
	adding bool
}

// openCaseForm opens the form that adds a case to p, its input that of
// from when given.
func (m *model) openCaseForm(p *problem.Problem, from *problem.Case) {
	f := &caseForm{p: p, user: m.app.KeepsUserCases(p), keys: p.InputKeys()}
	hasRef := len(m.app.ReferenceLangs(p)) > 0
	add := func(label, placeholder, value string) {
		ti := textinput.New()
		ti.Prompt = ""
		ti.Placeholder = placeholder
		ti.SetStyles(searchStyles())
		ti.SetValue(value)
		f.labels = append(f.labels, label)
		f.fields = append(f.fields, ti)
	}
	for _, k := range f.keys {
		value := ""
		if from != nil && !from.Generated() {
			if v, ok := from.Input[k]; ok {
				value = compactJSON(v)
			}
		}
		add(k, inputType(p, k), value)
	}
	switch {
	case hasRef && p.Judge.Compare != problem.CompareAnyOf:
		add("expected", "optional: the reference solution computes it", "")
	case f.user:
		add("expected", "optional: without it, the case shows your output", "")
	default:
		add("expected", "optional", "")
	}
	add("note", "optional", "")
	f.fields[0].Focus()
	m.form = f
	m.push(screenCaseForm)
}

// inputType is the type of input key of p, for the form's placeholder.
func inputType(p *problem.Problem, key string) string {
	if enc, _, ok := p.RoundTripMethods(); ok {
		return enc.Params[0].Type
	}
	switch {
	case p.Kind == problem.KindClass && key == "ops":
		return "string[]: the methods called"
	case p.Kind == problem.KindClass:
		return "array: the arguments of each call"
	}
	if prm, ok := p.Function.Input(key); ok {
		return prm.Type
	}
	return "JSON"
}

// compactJSON is raw on one line.
func compactJSON(raw json.RawMessage) string {
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return string(raw)
	}
	return b.String()
}

// caseAddedMsg reports what adding a case did.
type caseAddedMsg struct {
	form  *caseForm
	added *app.Added
	err   error
}

func (m *model) updateCaseForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	f := m.form
	if f == nil {
		m.pop()
		return m, nil
	}
	if key, ok := msg.(tea.KeyPressMsg); ok {
		if f.adding {
			return m, nil
		}
		switch key.String() {
		case "esc":
			m.form = nil
			m.pop()
			m.message = "cancelled"
			return m, nil
		case "tab", "down":
			f.setFocus(f.focus + 1)
			return m, nil
		case "shift+tab", "up":
			f.setFocus(f.focus - 1)
			return m, nil
		case "enter":
			return m, m.submitCaseForm()
		}
	}
	var cmd tea.Cmd
	f.fields[f.focus], cmd = f.fields[f.focus].Update(msg)
	return m, cmd
}

// setFocus focuses field i, wrapping around.
func (f *caseForm) setFocus(i int) {
	n := len(f.fields)
	f.fields[f.focus].Blur()
	f.focus = (i%n + n) % n
	f.fields[f.focus].Focus()
}

// submitCaseForm checks the form, then adds its case in the background:
// a reference solution may run on it.
func (m *model) submitCaseForm() tea.Cmd {
	f := m.form
	c := problem.Case{Input: map[string]json.RawMessage{}}
	for i, k := range f.keys {
		v := strings.TrimSpace(f.fields[i].Value())
		if v == "" {
			f.err = "give " + k
			f.setFocus(i)
			return nil
		}
		if !json.Valid([]byte(v)) {
			f.err = k + " is not valid JSON"
			f.setFocus(i)
			return nil
		}
		c.Input[k] = json.RawMessage(v)
	}
	n := len(f.keys)
	if v := strings.TrimSpace(f.fields[n].Value()); v != "" {
		if !json.Valid([]byte(v)) {
			f.err = "the expected output is not valid JSON"
			f.setFocus(n)
			return nil
		}
		c.Expected = json.RawMessage(v)
	}
	c.Note = strings.TrimSpace(f.fields[n+1].Value())
	if err := f.p.ValidateCase(c); err != nil {
		f.err = err.Error()
		return nil
	}
	f.err, f.adding = "", true
	a, ctx := m.app, m.ctx
	return func() tea.Msg {
		added, err := a.AddCase(ctx, f.p, c, io.Discard)
		return caseAddedMsg{form: f, added: added, err: err}
	}
}

// caseAdded shows the case added, in the cases tab, or why it was not.
func (m *model) caseAdded(msg caseAddedMsg) {
	f := msg.form
	if f != m.form {
		return
	}
	f.adding = false
	if msg.err != nil {
		f.err = msg.err.Error()
		return
	}
	m.form = nil
	m.pop()
	m.loadCases(f.p)
	m.caseList.cursor = min(msg.added.Index-1, max(len(m.caseList.cases)-1, 0))
	m.detailTab = tabProblemCases
	m.layout()
	added := msg.added
	switch {
	case added.User && added.Expected != nil:
		m.message = ui.Green.Render("added") + fmt.Sprintf(" your case %d, expecting %s (from the %s reference)", added.Index, clip(string(added.Expected), 40), added.Ref.Name())
	case added.User:
		m.message = ui.Green.Render("added") + fmt.Sprintf(" your case %d: t runs it, s does not", added.Index)
	case added.Build != nil && added.Build.Lang != nil:
		m.message = ui.Green.Render("added") + fmt.Sprintf(" case %d to %s, its expected output from the %s reference", added.Index, problem.CasesFile, added.Build.Lang.Name())
	default:
		m.message = ui.Green.Render("added") + fmt.Sprintf(" case %d to %s", added.Index, problem.CasesFile)
	}
}

// formKeys are the keys of the form.
var formKeys = []string{"enter add", "tab next", "shift+tab previous", "esc cancel"}

func (m *model) viewCaseForm() string {
	f := m.form
	p := m.current
	sub := m.chip(m.st.accent.Bold(true), "Add a case")
	if f.user {
		sub += "  " + ui.Faint.Render("one of your own, in "+m.app.WS.Rel(m.app.WS.UserCasesPath(p))+": t runs it, s does not")
	} else {
		sub += "  " + ui.Faint.Render("to "+m.app.WS.Rel(f.p.Dir)+"/"+problem.CasesFile)
	}
	labelW := 0
	for _, l := range f.labels {
		labelW = max(labelW, runewidth.StringWidth(l))
	}
	labelW += 2
	var l lines
	l.add("")
	for i := range f.fields {
		r := newRow(nil)
		m.cursorBar(r, i == f.focus)
		style := ui.Faint
		if i == f.focus {
			style = m.st.accent.Bold(true)
		}
		r.add(style, runewidth.FillRight(f.labels[i], labelW))
		f.fields[i].SetWidth(max(m.width-labelW-4, 10))
		l.add(r.fill(0) + f.fields[i].View())
	}
	l.add("")
	switch {
	case f.adding && f.user:
		l.add(ui.Faint.Render("  Adding… the reference solution runs on it."))
	case f.adding:
		l.add(ui.Faint.Render("  Adding…"))
	case f.err != "":
		for _, line := range wrapCells(f.err, max(m.width-4, 10)) {
			l.add("  " + ui.Red.Render(line))
		}
	default:
		l.add(ui.Faint.Render("  Values are JSON, as in hc cases list: [1,2,3], \"text\", 42."))
	}
	h := max(m.height-headerLines-footerLines, 3)
	body := strings.TrimSuffix(l.b.String(), "\n")
	for n := l.n; n < h; n++ {
		body += "\n"
	}
	return m.frame(m.problemTitle(p), sub, body, "", formKeys...)
}
