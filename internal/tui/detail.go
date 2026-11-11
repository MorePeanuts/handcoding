package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

// openDetail shows the statement of p. To start it, the solution file is
// created if needed and its path copied, ready to be opened in an editor
// next door; otherwise the problem is only viewed.
func (m *model) openDetail(p *problem.Problem, start bool) {
	m.current = p
	m.viewing = !start
	if start {
		if path, ok := m.ensureSolution(p); ok {
			m.copyPath(path)
		}
	}
	m.detailTab = tabStatement
	m.caseList = caseList{}
	m.loadCases(p)
	m.push(screenDetail)
	m.detail.GotoTop()
}

// renderDetail draws the detail screen's tab: the statement, or the
// problem's cases.
func (m *model) renderDetail() {
	p := m.current
	if p == nil {
		return
	}
	if m.detailTab == tabProblemCases {
		m.renderCaseList()
		return
	}
	desc, err := m.statement(p)
	if err != nil {
		desc = "_" + err.Error() + "_"
	}
	if desc == "" {
		desc = "_No description._"
	}
	m.detailImages = localImages(p.Dir, desc)
	width := min(max(m.width-2, 20), 120)
	m.detail.SetContent(m.md.Render(desc, p.Dir, width))
}

func (m *model) updateDetail(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.message = ""
	switch key := msg.String(); key {
	case "esc", "q":
		m.pop()
		return m, nil
	case "tab", "shift+tab":
		m.detailTab = 1 - m.detailTab
		m.renderDetail()
		return m, nil
	case "a":
		var from *problem.Case
		if m.detailTab == tabProblemCases {
			from, _ = m.selectedCase()
		}
		m.openCaseForm(m.current, from)
		return m, nil
	case "o":
		m.openImages()
		return m, nil
	case "enter":
		// Back to the latest result, if there is one for this problem.
		if m.run != nil && m.run.ref == m.current.Ref() {
			m.push(screenResult)
		}
		return m, nil
	default:
		if m.detailTab == tabProblemCases {
			if cmd, ok := m.updateCaseList(key); ok {
				return m, cmd
			}
		}
		if cmd, ok := m.problemKey(key, m.current); ok {
			return m, cmd
		}
	}
	var cmd tea.Cmd
	if m.detailTab == tabProblemCases {
		m.casesView, cmd = m.casesView.Update(msg)
		return m, cmd
	}
	m.detail, cmd = m.detail.Update(msg)
	return m, cmd
}

// problemTitle is the first line of a problem's screens: hc's badge, the
// problem's pack, number, title and slug (the name commands take, and the
// only one easy to type for a problem without an ID or with one such as
// "LCR 001"), its difficulty and tags; its star and locale at the end.
func (m *model) problemTitle(p *problem.Problem) string {
	name := m.title(p)
	if p.ID != "" {
		name = p.ID + ". " + name
	}
	left := m.badge() + ui.Faint.Render("  "+p.Pack+" / ") + m.st.title.Render(name) + "  " + ui.Faint.Render(p.Slug)
	if p.Difficulty != "" {
		left += "  " + m.chip(difficultyStyle(p.Difficulty), difficultyLabel(p.Difficulty))
	}
	var right []string
	if m.stars[p.Ref()] {
		right = append(right, ui.Yellow.Render("★"))
	}
	if l := m.localeLabel(p.Pack); l != "" {
		right = append(right, ui.Faint.Render(l))
	}
	end := strings.Join(right, "  ")
	// The tags give way to the star and the locale.
	if room := m.width - lipgloss.Width(left) - lipgloss.Width(end) - 4; len(p.Tags) > 0 && room > 3 {
		left += "  " + ui.Faint.Render(runewidth.Truncate(strings.Join(p.Tags, " · "), room, "…"))
	}
	return padBetween(left, end, m.width)
}

// solutionLine is the second line of the statement: the language and the
// solution's path; the problem's status at the end.
func (m *model) solutionLine(p *problem.Problem) string {
	var status string
	switch s := m.statuses[p.Ref()]; {
	case !m.ready(p):
		status = ui.Cyan.Render("? not ready")
	case s == app.StatusSolved:
		status = ui.Green.Render("✔ solved")
	case s == app.StatusAttempted:
		status = ui.Yellow.Render("● attempted")
	}
	label := m.chip(m.st.accent.Bold(true), m.lang.Name()) + "  "
	missing := ""
	if !m.app.HasSolution(p, m.lang) {
		missing = ui.Faint.Render("  (not created)")
	}
	room := m.width - lipgloss.Width(label) - lipgloss.Width(missing) - lipgloss.Width(status) - 2
	path := clipLeft(displayPath(m.app.SolutionPath(p, m.lang)), room)
	dir, file := filepath.Split(path)
	return padBetween(label+ui.Faint.Render(dir)+file+missing, status, m.width)
}

// clipLeft keeps the end of s (the file name matters most in a path).
func clipLeft(s string, width int) string {
	if width <= 1 || runewidth.StringWidth(s) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && runewidth.StringWidth(string(r))+1 > width {
		r = r[1:]
	}
	return "…" + string(r)
}

// detailKeys are the keys of the detail screen's tab, the common ones
// first.
func (m *model) detailKeys() []string {
	p := m.current
	if m.detailTab == tabProblemCases {
		keys := []string{"↑↓ case", "tab statement", "a add"}
		if c, _ := m.selectedCase(); m.removable(c) {
			keys = append(keys, "x remove")
		}
		keys = append(keys, "d debug-case", "t test", "s submit", "e edit", "h history", "esc back",
			"g/G first/last", "pgup/pgdn page", "y copy-path", "L lang", "m star")
		return keys
	}
	var keys []string
	if len(m.detailImages) > 0 {
		keys = append(keys, "o open-images")
	}
	if m.run != nil && m.run.ref == p.Ref() {
		keys = append(keys, "enter last-result")
	}
	keys = append(keys, "t test", "s submit", "d debug", "e edit", "h history", "y copy-path", "L lang", "esc back",
		"tab cases", "a add-case", "↑↓ scroll", "m star", "r reset", "D discard")
	if m.localeLabel(p.Pack) != "" {
		keys = append(keys, "i locale")
	}
	return keys
}

func (m *model) viewDetail() string {
	p := m.current
	body, pos := withScrollbar(m.detail, m.st.accent), scrollPosition(m.detail)
	if m.detailTab == tabProblemCases {
		body, pos = withScrollbar(m.casesView, m.st.accent), ""
		if _, index := m.selectedCase(); index > 0 {
			pos = fmt.Sprintf("case %d of %d", index, len(m.caseList.cases))
		}
	}
	return m.frame(m.problemTitle(p), m.solutionLine(p), m.detailTabs()+"\n"+body, pos, m.detailKeys()...)
}

// localImages lists the image files a statement in dir refers to.
func localImages(dir, desc string) []string {
	var paths []string
	problem.ReplaceImages(desc, func(_, src string) string {
		if !strings.Contains(src, "://") {
			path := filepath.Join(dir, filepath.FromSlash(src))
			if _, err := os.Stat(path); err == nil {
				paths = append(paths, path)
			}
		}
		return ""
	})
	return paths
}

// openImages shows the statement's images in the system image viewer, at
// full resolution (terminals without kitty graphics get a rough preview).
func (m *model) openImages() {
	if len(m.detailImages) == 0 {
		m.message = "this statement has no images"
		return
	}
	if err := m.open(m.detailImages); err != nil {
		m.message = ui.Red.Render(err.Error())
		return
	}
	m.message = "opened " + plural(len(m.detailImages), "image")
}

// openFiles opens files with the desktop's default application.
func openFiles(paths []string) error {
	var cmds []*exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmds = append(cmds, exec.Command("open", paths...))
	case "windows":
		for _, p := range paths {
			cmds = append(cmds, exec.Command("cmd", "/c", "start", "", p))
		}
	default:
		for _, p := range paths {
			cmds = append(cmds, exec.Command("xdg-open", p))
		}
	}
	for _, c := range cmds {
		if err := c.Start(); err != nil {
			return fmt.Errorf("open images: %w", err)
		}
		go func() { _ = c.Wait() }() // reap it; its exit status does not matter
	}
	return nil
}
