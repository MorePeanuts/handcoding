package app

import (
	"context"
	"fmt"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/lang/python"
)

type DoctorOptions struct {
	Language string
	Debug    bool
	Headless bool
}

type DoctorReport struct {
	Workspace string            `json:"workspace,omitempty"`
	Backend   string            `json:"backend"`
	OK        bool              `json:"ok"`
	Checks    []lang.Diagnostic `json:"checks"`
}

// Doctor only inspects executables. It must not set up environments, sync the
// workspace, open history, or run pack code. Python is also hc infrastructure.
func (a *App) Doctor(ctx context.Context, opts DoctorOptions) (*DoctorReport, error) {
	if opts.Headless && !opts.Debug {
		return nil, fmt.Errorf("--headless requires --debug")
	}
	name := opts.Language
	if name == "" {
		name = a.WS.Config.Language
	}
	if name == "" {
		name = "go"
	}
	selected, err := a.Langs.Get(name)
	if err != nil {
		return nil, err
	}
	languages := []lang.Language{selected}
	if selected.ID() != "python" {
		py, err := a.Langs.Get("python")
		if err != nil {
			return nil, err
		}
		languages = append(languages, py)
	}
	rep := &DoctorReport{Workspace: a.WS.Root, Backend: "local", OK: true}
	for _, l := range languages {
		// There is no managed workspace environment to inspect in PATH-only mode.
		if a.WS.Root == "" && l.ID() == "python" && a.WS.Config.Python.Interpreter == "" {
			l = python.Language{}
		}
		d, ok := l.(lang.Diagnoser)
		if !ok {
			return nil, fmt.Errorf("%s does not provide environment diagnostics", l.Name())
		}
		checks := d.Diagnose(ctx, lang.DiagnosticOptions{Debug: opts.Debug && l.ID() == selected.ID(), Headless: opts.Headless})
		rep.Checks = append(rep.Checks, checks...)
		for _, c := range checks {
			if c.Status != "ok" {
				rep.OK = false
			}
		}
	}
	return rep, nil
}
