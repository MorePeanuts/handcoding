package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

type packOperation struct {
	result string
	title  string
	output *syncBuffer
	cancel context.CancelFunc // nil only after the worker has finished
}

type packDoneMsg struct {
	op   *packOperation
	name string
	err  error
}

type packTickMsg struct{ op *packOperation }

func packTick(op *packOperation) tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(time.Time) tea.Msg { return packTickMsg{op} })
}

// The worker owns a separate App and workspace. SavePacks mutates Config, so
// sharing the UI's App would race with drawing and command context reads.
func (m *model) startPackOperation(action, target, ref string) tea.Cmd {
	if !m.packWritable() {
		return nil
	}
	ctx, cancel := context.WithCancel(m.ctx)
	op := &packOperation{title: action + " pack", output: &syncBuffer{max: 256 * 1024}, cancel: cancel}
	m.packs.operation = op
	m.packs.details = false
	m.packs.view.GotoTop()
	m.message = ""
	root := m.app.WS.Root
	work := func() tea.Msg {
		defer cancel()
		ws, err := workspace.Open(root)
		if err != nil {
			return packDoneMsg{op: op, err: err}
		}
		if err := ctx.Err(); err != nil {
			return packDoneMsg{op: op, err: err}
		}
		a := app.New(ws)
		defer a.Close()
		name := target
		opts := app.PackOptions{Ref: ref, Progress: op.output}
		switch action {
		case "install":
			// Relative directories, like configured tool paths, use this workspace.
			source := target
			if !filepath.IsAbs(source) {
				candidate := filepath.Join(root, source)
				// An existing local path takes priority over Git's host:path
				// syntax, including directory names containing a colon.
				if _, e := os.Stat(candidate); e == nil || !strings.Contains(source, ":") {
					source = candidate
				}
			}
			var result *app.InstallResult
			result, err = a.InstallPack(ctx, source, opts)
			if result != nil {
				name = result.Manifest.Name
				packInstalled(op, result)
			}
		case "restore":
			var results []*app.InstallResult
			results, err = a.InstallMissing(ctx, opts)
			for _, result := range results {
				name = result.Manifest.Name
				packInstalled(op, result)
			}
			if err == nil && len(results) == 0 {
				fmt.Fprintln(op.output, "No missing packs.")
			}
		case "update":
			var result *app.PackUpdate
			result, err = a.UpdatePack(ctx, target, opts)
			if result != nil {
				fmt.Fprintf(op.output, "Updated %s: %d added, %d removed.\n", result.Name, len(result.Added), len(result.Removed))
				if len(result.Orphans) > 0 {
					fmt.Fprintf(op.output, "Personal work for %d removed or moved problems was kept.\n", len(result.Orphans))
				}
				if result.GroupWarning != "" {
					fmt.Fprintln(op.output, result.GroupWarning)
				}
				for _, failed := range result.Failed {
					fmt.Fprintln(op.output, failed)
				}
			}
		case "remove":
			_, err = a.RemovePack(ctx, target, false)
			if err == nil {
				fmt.Fprintf(op.output, "Uninstalled %s. Solutions, personal cases, groups and history were kept.\n", target)
			}
		}
		return packDoneMsg{op: op, name: name, err: err}
	}
	return tea.Batch(work, packTick(op))
}

func packInstalled(op *packOperation, r *app.InstallResult) {
	fmt.Fprintf(op.output, "Installed %s: %d problems.\n", r.Manifest.Name, r.Problems)
	for _, failed := range r.Failed {
		fmt.Fprintln(op.output, failed)
	}
}

func (m *model) finishPackOperation(msg packDoneMsg) {
	if m.packs == nil || m.packs.operation != msg.op {
		return
	}
	msg.op.cancel = nil
	if msg.err != nil {
		msg.op.result = fmt.Sprintf("\nFailed: %s\n", msg.err)
	} else {
		msg.op.result = "\nDone.\n"
	}
	// Reconcile even on failure: an operation may have made partial progress.
	if err := m.reloadCommand(); err != nil {
		msg.op.result += fmt.Sprintf("\nCannot refresh workspace: %s\n", err)
	}
	if msg.err == nil && msg.name != "" {
		if err := m.loadPackItems(msg.name); err != nil {
			msg.op.result += fmt.Sprintf("\nCannot refresh packs: %s\n", err)
		}
	}
	m.message = "Press Esc to return to pack management"
}
