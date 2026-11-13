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

? shows the current page's shortcut reference. This guide explains the workflow. Use Tab to choose topics or read the page, / to search topic names and content, and Ctrl+n/p to change topics. :help search opens a topic directly.`},
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

In this guide, / searches topic titles and their full text. Up/Down chooses a matching topic; Enter reads it. Esc clears the search before returning to the previous page.`},
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
	topics                 []HelpTopic
	choices                []int
	index                  int
	view                   viewport.Model
	search                 textinput.Model
	searching, topicsFocus bool
}

func (m *model) openGuide(topic HelpTopic) {
	g := &guidePage{topics: GuideTopics(), view: viewport.New(), search: textinput.New()}
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
	m.push(screenGuide)
}

func (m *model) layoutGuide() {
	g := m.guide
	if g == nil {
		return
	}
	width := max(m.width-1, 1)
	if m.width >= 90 {
		width -= 29
	}
	g.view.SetWidth(max(width, 1))
	g.view.SetHeight(max(m.height-headerLines-footerLines, 3))
	g.search.SetWidth(max(m.width-3, 1))
	text := "No matching topic. Edit the search or press Esc to clear it."
	if len(g.choices) > 0 {
		t := g.topics[g.choices[g.index]]
		text = m.md.Render(t.Body, "", max(width-1, 1))
	}
	g.view.SetContent(text)
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
	m.layoutGuide()
}

func (m *model) updateGuide(msg tea.Msg) (tea.Model, tea.Cmd) {
	g := m.guide
	key, _ := msg.(tea.KeyPressMsg)
	move := func(delta int) {
		g.index = min(max(g.index+delta, 0), max(len(g.choices)-1, 0))
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
			g.topicsFocus = false
			return m, nil
		case "up", "ctrl+p":
			move(-1)
			return m, nil
		case "down", "ctrl+n":
			move(1)
			return m, nil
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
	case "tab", "shift+tab":
		g.topicsFocus = !g.topicsFocus
		return m, nil
	case "ctrl+n":
		move(1)
		return m, nil
	case "ctrl+p":
		move(-1)
		return m, nil
	case "enter":
		g.topicsFocus = false
		return m, nil
	case "up", "k":
		if g.topicsFocus {
			move(-1)
			return m, nil
		}
	case "down", "j":
		if g.topicsFocus {
			move(1)
			return m, nil
		}
	}
	var cmd tea.Cmd
	g.view, cmd = g.view.Update(msg)
	return m, cmd
}

var guideKeys = []string{"esc back", "/ search", "tab topics/page", "↑↓ scroll", "ctrl+n/p topic"}

func (m *model) viewGuide() string {
	g := m.guide
	subtitle := "No matching topics"
	if len(g.choices) > 0 {
		t := g.topics[g.choices[g.index]]
		subtitle = fmt.Sprintf("%s  ·  %d/%d", t.Title, g.index+1, len(g.choices))
	}
	if g.topicsFocus {
		subtitle += "  ·  choosing topic"
	}
	if g.searching {
		subtitle = g.search.View()
	} else if g.search.Value() != "" {
		subtitle += "  ·  /" + g.search.Value()
	}
	body := withScrollbar(g.view, m.st.accent)
	if m.width >= 90 {
		var menu []string
		first := max(g.index-g.view.Height()+2, 0)
		for i := first; i < min(first+g.view.Height(), len(g.choices)); i++ {
			t := g.topics[g.choices[i]]
			label := fit(t.Title, 25)
			if i == g.index {
				label = m.st.accent.Bold(true).Render("▌ " + label)
			} else {
				label = "  " + label
			}
			menu = append(menu, label)
		}
		for len(menu) < g.view.Height() {
			menu = append(menu, strings.Repeat(" ", 27))
		}
		body = lipgloss.JoinHorizontal(lipgloss.Top, strings.Join(menu, "\n"), "  ", body)
	}
	keys := guideKeys
	if m.width < 90 {
		keys = []string{"esc back", "/ search", "tab topics", "↑↓ scroll", "ctrl+n/p topic"}
	}
	if g.searching {
		keys = []string{"↑↓ topic", "enter read", "esc clear"}
	}
	return m.frame(m.badge()+"  "+ui.Bold.Render("TUI Guide"), subtitle, body, scrollPosition(g.view), keys...)
}
