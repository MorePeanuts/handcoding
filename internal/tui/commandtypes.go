package tui

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
	// Resolve validates input without executing it, and returns arguments
	// for the current hc executable, including the workspace and context.
	Resolve func(string, CommandContext) ([]string, error)
	// Complete returns replacement prefixes for input before the text
	// cursor. It accepts unfinished quoting while the user is typing.
	Complete func(string, CommandContext) []string
}
