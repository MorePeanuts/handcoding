package golang

import (
	"context"
	"regexp"

	"github.com/MorePeanuts/handcoding/internal/lang"
)

var delveVersionRE = regexp.MustCompile(`(?m)^Version:\s*(\S+)`)

func (l Language) Diagnose(ctx context.Context, opts lang.DiagnosticOptions) []lang.Diagnostic {
	d := lang.Diagnostic{Language: "go", Role: "compiler", Command: l.compilerName(), Source: "PATH", Minimum: MinVersion}
	if l.GoCommand != "" {
		d.Source = "hc.yaml: go.command"
	}
	if d.Resolve() {
		env, err := readGoEnv(ctx, d.Path)
		if err == nil {
			var tc lang.Toolchain
			tc, err = toolchain(d.Path, env)
			d.Version = tc.Version
		}
		if err != nil {
			d.Status, d.Message = "error", err.Error()
		} else {
			d.CheckVersion()
		}
	}
	if d.Status != "ok" {
		d.Message += "; provide a supported Go toolchain or set go.command in hc.yaml"
	}
	checks := []lang.Diagnostic{d}
	if opts.Debug {
		d = lang.Diagnostic{Language: "go", Role: "debugger", Command: l.debuggerName(), Source: "PATH"}
		if l.Debugger != "" {
			d.Source = "hc.yaml: go.debugger"
		}
		if d.Resolve() {
			out, err := lang.Probe(ctx, d.Path, localToolchain(), "version")
			switch m := delveVersionRE.FindSubmatch(out); {
			case err != nil:
				d.Status, d.Message = "error", err.Error()
			case len(m) == 0:
				d.Status, d.Message = "error", "cannot read the Delve version"
			default:
				d.Version, d.Status = string(m[1]), "ok"
			}
		}
		if d.Status != "ok" {
			d.Message += "; provide Delve or set go.debugger in hc.yaml"
		}
		checks = append(checks, d)
	}
	return checks
}
