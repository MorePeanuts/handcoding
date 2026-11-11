package tui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

// problemKey handles the actions available wherever a problem is in focus
// (list, detail and result screens; runKeys only in a problem, and the
// list handles h itself). It reports whether key was handled.
func (m *model) problemKey(key string, p *problem.Problem) (tea.Cmd, bool) {
	switch key {
	case "e":
		return m.edit(p), true
	case "t":
		return m.startRun(runTest, p), true
	case "s":
		return m.startRun(runSubmit, p), true
	case "d":
		return m.debug(p, m.lang), true
	case "h":
		m.openHistory(p)
		return nil, true
	case "y":
		if path, ok := m.ensureSolution(p); ok {
			m.copyPath(path)
		}
		return nil, true
	case "m":
		m.toggleStar(p)
		return nil, true
	case "L":
		m.switchLang()
		return nil, true
	case "i":
		m.switchLocale(p.Pack)
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
			if m.run != nil && m.run.ref == p.Ref() {
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

// toggleStar stars p, or unstars it.
func (m *model) toggleStar(p *problem.Problem) {
	starred := !m.stars[p.Ref()]
	st, err := m.app.Store()
	if err == nil {
		err = st.SetStar(m.ctx, p.Pack, p.Slug, starred)
	}
	if err != nil {
		m.message = ui.Red.Render(err.Error())
		return
	}
	m.stars[p.Ref()] = starred
	if starred {
		m.message = ui.Yellow.Render("★") + " starred " + p.Slug
	} else {
		m.message = "unstarred " + p.Slug
	}
	if m.filter == filterStarred {
		m.applyFilter()
	}
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
		if !m.viewing {
			if path, ok := m.ensureSolution(m.current); ok {
				m.copyPath(path)
			}
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
	return tea.Exec(m.handOff(cmd, false), func(err error) tea.Msg { return execDoneMsg{err} })
}

// runKeys are the keys of problemKey that run the solution, which the
// problem list leaves to the problem's own screens, as LeetCode does.
var runKeys = []string{"t", "s", "d"}

// debug starts the debugger on p's solution in l, on the case hc debug
// picks: the first that fails.
func (m *model) debug(p *problem.Problem, l lang.Language) tea.Cmd {
	if !m.app.HasSolution(p, l) {
		m.message = "no " + l.Name() + " solution yet — press enter or e first"
		return nil
	}
	cmd := exec.Command(m.opts.Exe, "-w", m.app.WS.Root, "debug", p.Ref(), "-l", l.ID(), "--pause")
	return tea.Exec(m.handOff(cmd, true), func(err error) tea.Msg {
		// The debugger's exit status is not ours to report, but hc debug
		// failing before it starts (a compile error) is.
		if _, ok := errors.AsType[*exec.ExitError](err); ok {
			err = nil
		}
		return execDoneMsg{err}
	})
}

// handOff runs cmd, an interactive program, in the terminal: the editor or
// the debugger.
func (m *model) handOff(cmd *exec.Cmd, clear bool) *handOffCmd {
	return &handOffCmd{Cmd: cmd, clear: clear, themeReports: m.opts.FollowTheme}
}

// handOffCmd prepares the terminal for an interactive program. It turns off
// the theme reports (see themeReports), which the program would read as
// input, and with clear, clears the terminal, so that a session does not
// scroll below the output of earlier ones.
type handOffCmd struct {
	*exec.Cmd
	clear, themeReports bool
}

func (c *handOffCmd) Run() error {
	if c.Stdout != nil {
		var seq string
		if c.themeReports {
			seq += ansi.ResetModeLightDark
		}
		if c.clear {
			seq += "\x1b[H\x1b[2J\x1b[3J"
		}
		fmt.Fprint(c.Stdout, seq)
	}
	return c.Cmd.Run()
}

// The setters give the program the TUI's terminal, as tea.ExecProcess
// does: only the streams cmd leaves unset.

func (c *handOffCmd) SetStdin(r io.Reader) {
	if c.Stdin == nil {
		c.Stdin = r
	}
}

func (c *handOffCmd) SetStdout(w io.Writer) {
	if c.Stdout == nil {
		c.Stdout = w
	}
}

func (c *handOffCmd) SetStderr(w io.Writer) {
	if c.Stderr == nil {
		c.Stderr = w
	}
}

// displayPath shortens the home directory to ~.
func displayPath(path string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + path[len(home):]
	}
	return path
}
