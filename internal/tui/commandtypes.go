package tui

import (
	"context"
	"io"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/store"
)

// Command is either a native action or an explicit terminal handoff. Parsing
// stays in the CLI adapter; the TUI owns navigation, execution and rendering.
type Command struct {
	Action                                 string
	Args                                   []string
	Problem, Language, Pack, Group, Locale string
	File                                   string
	ID                                     int64
	Cases                                  []int
	Help                                   HelpTopic
	Run                                    RunCommand
	Note                                   store.Note
	EditNote                               bool
	Verbose                                bool
	Custom                                 bool
}

type RunCommand func(context.Context, *app.App, *problem.Problem, lang.Language, io.Writer, store.Note) (*runner.Report, *store.Submission, error)

type HelpTopic struct{ Name, Title, Body string }

// CommandContext is the explicit context available to command mode. Problem
// is empty on screens without an unambiguous current solution (including
// submission history). Candidates are qualified names from the workspace.
type CommandContext struct {
	Workspace string
	Problem   string
	Language  string
	Problems  []string
	Packs     []string
	Groups    []string
}

// Commands connects command mode to the existing CLI definitions. The CLI
// supplies this adapter so the TUI does not depend on its command package.
type Commands struct {
	Topics func() []HelpTopic
	// Resolve validates input without executing it and chooses a native
	// action or terminal argv, including explicit target/context defaults.
	Resolve func(string, CommandContext) (Command, error)
	// Complete returns replacement prefixes for input before the text
	// cursor. It accepts unfinished quoting while the user is typing.
	Complete func(string, CommandContext) []string
}
