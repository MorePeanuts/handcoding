package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/MorePeanuts/handcoding/internal/ui"
)

// GuideTopics describes TUI behavior, rather than duplicating CLI reference
// text. The adapter appends command pages from the actual command definitions.
func GuideTopics() []HelpTopic {
	return []HelpTopic{
		{"overview", "Start here", `# Make progress without leaving the TUI

Browse a problem, read its statement, open a solution in your editor, then test, submit and review.

## A first session

1. Choose a pack and highlight a problem.
2. Press v to read it without creating files, or Enter to create a missing solution template and copy its path.
3. Press e to edit, t to test examples, and s to judge and record a submission.
4. Inspect Cases and Output with Tab. Use h for submission history.

## Find your way

Esc goes back. :list returns to the problem list. :pack list opens the pack picker. :q exits the TUI.

? shows the current page's shortcut reference. This guide explains the workflow. Use Tab / Shift+Tab to change topics, j/k or Up/Down to scroll, Ctrl+d/u to scroll half a page, Page Up/Down to scroll a full page, and / to search topic names and content. :help search opens a topic directly.`},
		{"navigation", "Browse, view & open", `# Navigate with keys or commands

- :list / :ls returns to the list and retains its valid search, filters and position.
- :list --pack regression or --group user:practice chooses another list and clears the previous search.
- :show two-sum / :view two-sum only reads the problem. No template is created and no path is copied.
- :open two-sum creates a template only if missing, opens the same detail page and copies its path, just like Enter in the list. Existing solutions are never overwritten.
- :cases list two-sum opens its cases; Tab switches between cases and statement.
- :pack list opens the pack picker; :group opens the current pack's groups.

Use pack/slug when a problem name is ambiguous. A command can open a different problem or pack; Esc returns to the original page, search and scroll position. :list and the picker commands go to navigation roots, so repeated use does not build a stack of identical lists.

L changes solution language; i changes statement locale when translations exist. Switching language while only viewing a statement does not create a template.`},
		{"search", "Search while browsing", `# Find a problem, then act on it

Press / in the problem list, group picker or group editor. Results update while you type. Search titles, slugs and tags; problem searches also accept ID expressions such as id:707 or id:700..800.

## While typing

- Up/Down or Ctrl+p/n moves the highlighted match without leaving the field.
- Ctrl+o opens the selected problem, chooses a group, or toggles selection in a group-editor draft.
- Enter finishes typing and leaves you browsing the filtered list, on the same match.
- Esc clears the query. Returning from a problem opened with Ctrl+o restores the active search.

A colon remains text in search fields, including id:707. Finish the search before opening command mode.

In this guide, / searches topic titles and their full text. Tab / Shift+Tab chooses a matching topic; Enter reads it. Esc clears the search before returning to the previous page.`},
		{"groups", "Personal & pack groups", `# Organize practice

Press c in the problem list or run :group to choose a group. :group show user:practice opens that group's problems in stored order. Use --pack to disambiguate groups across packs.

Pack groups are read-only. Personal groups live in your workspace; they do not modify the installed pack.

## In the picker

- n creates a personal group; e edits one.
- C copies a group into an editable personal draft.
- x deletes a personal group after confirmation; solutions and history stay.
- / searches group names and localized titles.

## In the editor

Search available problems, use Space or Ctrl+o during search to toggle a selection, x to remove a member, K/J to reorder, and Ctrl+s to save. Esc cancels the draft. Command mode is unavailable while a draft is open.

Missing problems remain listed as unavailable after a pack update. Remove obsolete memberships explicitly.`},
		{"practice", "Test, submit & debug", `# Run locally and inspect the result

:test runs examples and your personal cases. :test --all runs every case without recording a submission. :test -c 2 chooses case 2. :test --input '{"nums":[2,7],"target":9}' tries a one-off input; --generate/--args/--seed build one with the problem's generator.

:submit judges the current solution and records its code and result in history. Installed packs use their official cases for submissions. -m and --notes attach a note; -e opens a TUI draft before submission. Save to run, or Esc to cancel without recording anything.

## Results

The target and language appear above the running/result view. Tab switches Cases and Output; arrows select a case. --verbose opens all case details. Esc cancels an active run, waits for cancellation to finish, then returns. Command mode stays unavailable while a run is active.

--file tests another file; relative paths start at the workspace and language comes from its extension unless explicitly given. A file/custom-input result is not a shortcut to debugging a different stored case: use :debug with the intended input instead.

Debugging hands the terminal to the debugger. Local :submit and online :leetcode submit are different operations; online commands still show their terminal output.`},
		{"history", "Review past submissions", `# Review without changing today's solution

:history opens the entire workspace's history. :history two-sum limits it to that problem; bare :history never assumes the highlighted problem. h on a problem opens its own history.

Enter opens a submission, or use :history show 42 directly. Tab switches its report and saved code. Notes describe your approach; c edits a note. r restores historical code only after confirmation; x deletes a submission after confirmation.

Viewing historical code does not execute or restore it. On history and submission screens, commands that run a solution require an explicit problem, and act on its current solution file. Submission IDs and destructive targets are always explicit.

History remains available when its problem or pack was removed; operations needing that missing problem explain why they cannot proceed.`},
		{"commands", "Command mode & context", `# The same TUI, another way to act

Press : on a browsing page. Omit hc. Enter executes; Esc dismisses the input. Tab/Shift+Tab cycle completion; Up/Down or Ctrl+p/n recall this session's command history. Quotes and backslashes preserve spaces and JSON. No shell, pipelines, redirection or command chaining is executed.

## Defaults

Commands use this workspace. A required problem may default to the highlighted list item or displayed problem, with the current language. Explicit targets and language flags take precedence. Pickers and history provide no implicit solution target. Optional arguments retain their meaning: :history covers the workspace, :validate validates it, and :pack update updates all managed packs.

## Native pages and terminal operations

Navigation and local test/submit use TUI pages. Fetching, pack changes, online judging, agents and interactive tools keep their existing terminal flow. Press Enter after a terminal command to return; even failures refresh workspace data. Ctrl+c cancels a child command without closing the TUI.

Explicit --plain, --raw, --json and cases --expand request terminal output. Ordinary browsing commands never start a nested TUI. init and pack init are unavailable; --workspace and --pause cannot override the session.

:q / :quit exits. :help opens this guide; :help submit and :submit --help show the same command page. ? remains a quick reference for the current screen.`},
	}
}

type guidePage struct {
	topics    []HelpTopic
	choices   []int
	index     int
	view      viewport.Model
	menu      viewport.Model
	search    textinput.Model
	searching bool
}

func (m *model) openGuide(topic HelpTopic) {
	g := &guidePage{topics: GuideTopics(), view: viewport.New(), menu: viewport.New(), search: textinput.New()}
	if m.opts.Commands != nil && m.opts.Commands.Topics != nil {
		g.topics = m.opts.Commands.Topics()
	}
	g.search.Prompt = "/ "
	g.search.Placeholder = "Search topics and commands"
	g.search.SetStyles(searchStyles())
	found := false
	for i, t := range g.topics {
		g.choices = append(g.choices, i)
		if t.Name == topic.Name {
			g.index = i
			found = true
		}
	}
	if !found {
		g.topics = append(g.topics, topic)
		g.index = len(g.topics) - 1
		g.choices = append(g.choices, g.index)
	}
	m.guide = g
	m.message = ""
	m.push(screenGuide)
}

const guideMenuWidth = 31 // includes the menu scrollbar; divider takes three more

func (m *model) layoutGuide() {
	g := m.guide
	if g == nil {
		return
	}
	width, menuWidth := max(m.width-1, 1), max(m.width-1, 1)
	if m.width >= 90 {
		width -= guideMenuWidth + 3
		menuWidth = guideMenuWidth - 1
	}
	g.view.SetWidth(max(width, 1))
	g.view.SetHeight(max(m.height-headerLines-footerLines, 3))
	g.menu.SetWidth(menuWidth)
	g.menu.SetHeight(g.view.Height())
	g.search.SetWidth(max(m.width-3, 1))
	text := "No matching topic. Edit the search or press Esc to clear it."
	if len(g.choices) > 0 {
		t := g.topics[g.choices[g.index]]
		// Like the statement page, limit line length on large terminals.
		text = m.md.Render(t.Body, "", min(max(width-1, 1), 100))
	}
	g.view.SetContent(text)
	m.layoutGuideMenu()
}

// layoutGuideMenu uses the same cursor row as the other pickers. Group headers
// are display rows, never selectable topics; keep the selected row in view.
func (m *model) layoutGuideMenu() {
	g := m.guide
	var lines []string
	group, selected := "", 0
	for i, choice := range g.choices {
		t := g.topics[choice]
		section := "Guide"
		command := strings.HasPrefix(t.Name, "command:")
		if command {
			section = "Commands"
		}
		if section != group {
			if group != "" {
				lines = append(lines, "")
			}
			lines = append(lines, titledRule(section, g.menu.Width()))
			group = section
		}
		r := newRow(nil)
		style := ui.Plain
		label := t.Title
		if command {
			if strings.Contains(t.Title, " ") {
				label = "  " + label
			} else {
				style = ui.Bold
			}
		}
		if i == g.index {
			selected = len(lines)
			style = ui.Bold
			r.bg = m.st.sel
		}
		m.cursorBar(r, i == g.index)
		r.add(style, fit(label, g.menu.Width()-2))
		lines = append(lines, r.fill(g.menu.Width()))
	}
	if len(lines) == 0 {
		lines = append(lines, ui.Faint.Render("  No matching topics"))
	}
	g.menu.SetContent(strings.Join(lines, "\n"))
	if selected < g.menu.YOffset() {
		g.menu.SetYOffset(max(selected-1, 0))
	} else if selected >= g.menu.YOffset()+g.menu.Height() {
		g.menu.SetYOffset(selected - g.menu.Height() + 1)
	}
}

func (m *model) filterGuide() {
	g := m.guide
	q := strings.ToLower(strings.TrimSpace(g.search.Value()))
	g.choices = nil
	for i, t := range g.topics {
		if strings.Contains(strings.ToLower(t.Name+" "+t.Title+" "+t.Body), q) {
			g.choices = append(g.choices, i)
		}
	}
	g.index = 0
	g.view.GotoTop()
	g.menu.GotoTop()
	m.layoutGuide()
}

func (m *model) updateGuide(msg tea.Msg) (tea.Model, tea.Cmd) {
	g := m.guide
	key, _ := msg.(tea.KeyPressMsg)
	move := func(delta int) {
		if len(g.choices) < 2 {
			return
		}
		g.index = (g.index + delta + len(g.choices)) % len(g.choices)
		g.view.GotoTop()
		m.layoutGuide()
	}
	if g.searching {
		switch key.String() {
		case "esc":
			g.search.SetValue("")
			g.searching = false
			g.search.Blur()
			m.filterGuide()
			return m, nil
		case "enter":
			g.searching = false
			g.search.Blur()
			return m, nil
		case "shift+tab":
			move(-1)
			return m, nil
		case "tab":
			move(1)
			return m, nil
		case "up", "down", "pgup", "pgdown", "ctrl+d", "ctrl+u":
			var cmd tea.Cmd
			g.view, cmd = g.view.Update(msg)
			return m, cmd
		}
		before := g.search.Value()
		var cmd tea.Cmd
		g.search, cmd = g.search.Update(msg)
		if before != g.search.Value() {
			m.filterGuide()
		}
		return m, cmd
	}
	switch key.String() {
	case "esc", "q":
		if g.search.Value() != "" {
			g.search.SetValue("")
			m.filterGuide()
		} else {
			m.pop()
		}
		return m, nil
	case "/":
		g.searching = true
		return m, g.search.Focus()
	case "tab":
		move(1)
		return m, nil
	case "shift+tab":
		move(-1)
		return m, nil
	case "g", "home":
		g.view.GotoTop()
		return m, nil
	case "G", "end":
		g.view.GotoBottom()
		return m, nil
	}
	var cmd tea.Cmd
	g.view, cmd = g.view.Update(msg)
	return m, cmd
}

var guideKeys = []string{"tab/shift+tab next/previous topic", "j/k,↑↓ scroll", "ctrl+d/u scroll half a page", "pgup/pgdown scroll a page", "g/G top/bottom", "/ search", "esc back"}

func (m *model) viewGuide() string {
	g := m.guide
	subtitle := "No matching topics"
	if len(g.choices) > 0 {
		t := g.topics[g.choices[g.index]]
		subtitle = fmt.Sprintf("%s  ·  %d/%d", t.Title, g.index+1, len(g.choices))
	}
	if g.searching {
		subtitle = g.search.View()
	} else if g.search.Value() != "" {
		subtitle += "  ·  /" + g.search.Value()
	}
	body := withScrollbar(g.view, m.st.accent)
	if m.width >= 90 {
		divider := ui.Faint.Render(strings.TrimSuffix(strings.Repeat(" │ \n", g.view.Height()), "\n"))
		body = lipgloss.JoinHorizontal(lipgloss.Top, withScrollbar(g.menu, m.st.accent), divider, body)
	} else if g.searching {
		body = withScrollbar(g.menu, m.st.accent)
	}
	keys := []string{"tab/⇧tab topic", "j/k scroll", "/ search", "? keys"}
	if m.width >= 90 {
		keys = []string{"tab/⇧tab topic", "j/k,↑↓ scroll", "ctrl+d/u ½page", "pgup/dn page", "/ search", "? keys", "esc back"}
	}
	if g.searching {
		keys = []string{"tab/⇧tab topic", "enter read", "esc clear"}
	} else {
		subtitle = padBetween(subtitle, ui.Faint.Render(scrollPosition(g.view)), m.width)
	}
	return m.frame(m.badge()+"  "+ui.Bold.Render("TUI Guide")+ui.Faint.Render(" · Workflows & commands"), subtitle, body, "", keys...)
}
