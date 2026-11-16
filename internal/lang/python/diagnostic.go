package python

import (
	"context"
	"errors"
	"os"
	"regexp"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/lang"
)

var diagnosticVersionRE = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// Diagnose deliberately bypasses Toolchain and Setup: even a missing or broken
// managed venv is only reported. -B keeps imports from writing bytecode.
func (l Language) Diagnose(ctx context.Context, opts lang.DiagnosticOptions) []lang.Diagnostic {
	d := lang.Diagnostic{Language: "python", Role: "interpreter", Command: l.python(), Source: "PATH", Minimum: MinVersion}
	switch {
	case l.Venv != nil:
		d.Source = "hc-managed .hc/venv"
	case l.Interpreter != "":
		d.Source = "hc.yaml: python.interpreter"
	}
	if d.Resolve() {
		out, err := lang.Probe(ctx, d.Path, nil, "-B", "-c", "import array, importlib.util, json, os, sys, threading, time, traceback; "+versionScript)
		d.Version = strings.TrimSpace(string(out))
		switch {
		case err != nil:
			d.Status, d.Message = "error", err.Error()
		case !diagnosticVersionRE.MatchString(d.Version):
			d.Status, d.Message = "error", "cannot read the Python version"
		default:
			d.CheckVersion()
		}
	} else if l.Venv != nil {
		if _, err := os.Lstat(l.Venv.Dir); errors.Is(err, os.ErrNotExist) {
			d.Status, d.Message = "uninitialized", "Python environment is not initialized; run hc env sync"
		} else {
			d.Message += "; the managed environment is unavailable (hc env sync recreates it)"
		}
	}
	checks := []lang.Diagnostic{d}
	if opts.Debug {
		module := "pdb"
		if opts.Headless {
			module = "debugpy"
		}
		debug := lang.Diagnostic{Language: "python", Role: module, Command: d.Command, Path: d.Path, Source: d.Source}
		if d.Status != "ok" {
			debug.Status, debug.Message = "unavailable", "requires a supported Python interpreter"
		} else {
			script := "import pdb; print('available')"
			if opts.Headless {
				script = "import debugpy; print(debugpy.__version__)"
			}
			out, err := lang.Probe(ctx, d.Path, nil, "-B", "-c", script)
			if err != nil {
				debug.Status, debug.Message = "error", err.Error()
				if opts.Headless && l.Venv != nil {
					debug.Message += "; hc env sync recreates the managed environment and retries debugpy installation"
				}
			} else {
				debug.Status = "ok"
				if opts.Headless {
					debug.Version = strings.TrimSpace(string(out))
				}
			}
		}
		checks = append(checks, debug)
	}
	return checks
}
