package tui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"

	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

// problemKey handles the actions available wherever a problem is in focus
// (list, detail and result screens). It reports whether key was handled.
func (m *model) problemKey(key string, p *problem.Problem) (tea.Cmd, bool) {
	switch key {
	case "e":
		return m.edit(p), true
	case "t":
		return m.startRun(runTest, p), true
	case "s":
		return m.startRun(runSubmit, p), true
	case "d":
		return m.debug(p), true
	case "h":
		m.openHistory(p)
		return nil, true
	case "y":
		if path, ok := m.ensureSolution(p); ok {
			m.copyPath(path)
		}
		return nil, true
	case "L":
		m.switchLang()
		return nil, true
	case "r":
		if !m.app.HasSolution(p, m.lang) {
			m.message = "no " + m.lang.Name() + " solution to reset"
			return nil, true
		}
		l := m.lang
		m.ask(fmt.Sprintf("Reset your %s solution of %s to the template?", l.Name(), p.Slug), func() tea.Cmd {
			if _, err := m.app.Reset(p, l); err != nil {
				m.message = ui.Red.Render(err.Error())
				return nil
			}
			m.message = ui.Green.Render("reset " + p.Slug + " (" + l.Name() + ")")
			m.refresh()
			return nil
		})
		return nil, true
	case "D":
		m.ask(fmt.Sprintf("Discard %s? Deletes your solution files (all languages) and its submission history.", p.Slug), func() tea.Cmd {
			n, err := m.app.Discard(m.ctx, p)
			if err != nil {
				m.message = ui.Red.Render(err.Error())
				return nil
			}
			m.message = ui.Green.Render(fmt.Sprintf("discarded %s: solution files and %s removed", p.Slug, plural(int(n), "submission")))
			if m.run != nil && m.run.slug == p.Slug {
				m.run = nil
			}
			// Leave screens that showed the discarded state.
			for m.top() != screenList && m.top() != screenDetail {
				m.pop()
			}
			m.refresh()
			if m.top() == screenDetail {
				m.renderDetail()
			}
			return nil
		})
		return nil, true
	}
	return nil, false
}

// ensureSolution creates the solution file if needed and returns its path.
func (m *model) ensureSolution(p *problem.Problem) (string, bool) {
	path, created, err := m.app.Start(p, m.lang)
	if err != nil {
		m.message = ui.Red.Render(err.Error())
		return "", false
	}
	if created {
		m.refresh()
	}
	return path, true
}

// copyPath puts path on the clipboard: the system clipboard when available,
// otherwise an OSC 52 escape sequence (works over SSH in most terminals).
func (m *model) copyPath(path string) {
	label := "copied: "
	if err := m.copy(path); err != nil {
		termenv.NewOutput(os.Stdout).Copy(path)
		label = "copied (OSC 52): "
	}
	m.message = ui.Green.Render(label) + clipLeft(displayPath(path), m.width-len(label))
}

func (m *model) switchLang() {
	all := m.app.Langs.All()
	for i, l := range all {
		if l.ID() == m.lang.ID() {
			m.lang = all[(i+1)%len(all)]
			break
		}
	}
	m.message = "language: " + m.lang.Name()
	if m.top() == screenDetail && m.current != nil {
		if path, ok := m.ensureSolution(m.current); ok {
			m.copyPath(path)
		}
		m.renderDetail()
	}
}

func (m *model) edit(p *problem.Problem) tea.Cmd {
	path, ok := m.ensureSolution(p)
	if !ok {
		return nil
	}
	cmd, err := m.opts.EditorCmd(path)
	if err != nil {
		m.message = ui.Red.Render(err.Error())
		return nil
	}
	cmd.Dir = filepath.Dir(path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return execDoneMsg{err} })
}

// debug starts the debugger on the first failing case of the last run of p,
// or on case 1.
func (m *model) debug(p *problem.Problem) tea.Cmd {
	if !m.app.HasSolution(p, m.lang) {
		m.message = "no " + m.lang.Name() + " solution yet — press enter or e first"
		return nil
	}
	caseNum := m.debugCase(p)
	cmd := exec.Command(m.opts.Exe, "-w", m.app.WS.Root, "debug", p.Slug, "-l", m.lang.ID(), "-c", strconv.Itoa(caseNum))
	m.message = fmt.Sprintf("debugged case %d", caseNum)
	return tea.Exec(&clearScreenCmd{Cmd: cmd}, func(err error) tea.Msg {
		// The debugger's exit status is not ours to report.
		if _, ok := errors.AsType[*exec.ExitError](err); ok {
			err = nil
		}
		return execDoneMsg{err}
	})
}

func (m *model) debugCase(p *problem.Problem) int {
	if m.run == nil || m.run.report == nil || m.run.slug != p.Slug || m.run.langID != m.lang.ID() {
		return 1
	}
	for _, c := range m.run.report.Cases {
		if c.Verdict != runner.Accepted && c.Verdict != runner.NoExpected {
			return c.Index
		}
	}
	return 1
}

// clearScreenCmd clears the terminal before running an interactive program,
// so its session does not scroll below the output of earlier ones.
type clearScreenCmd struct{ *exec.Cmd }

func (c *clearScreenCmd) Run() error {
	if c.Stdout != nil {
		fmt.Fprint(c.Stdout, "\x1b[H\x1b[2J\x1b[3J")
	}
	return c.Cmd.Run()
}

func (c *clearScreenCmd) SetStdin(r io.Reader)  { c.Stdin = r }
func (c *clearScreenCmd) SetStdout(w io.Writer) { c.Stdout = w }
func (c *clearScreenCmd) SetStderr(w io.Writer) { c.Stderr = w }

// displayPath shortens the home directory to ~.
func displayPath(path string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + path[len(home):]
	}
	return path
}
