package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/ui"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// ValidateOptions configure validation.
type ValidateOptions struct {
	// Run runs the reference solutions on every case, which must pass.
	Run bool
	// Strict is for publishing a pack (hc pack check): a problem that is not
	// ready, has no cases, cases without expected outputs or no reference,
	// or a reference that cannot be run for want of its toolchain, is an
	// error rather than a warning.
	Strict bool
	// Stderr receives the output of reference runs, such as the creation of
	// the Python environment. Nil discards it.
	Stderr io.Writer
}

// Finding is what validating one pack or problem found.
type Finding struct {
	// Name names what was validated: "pack <name>", the directory of a
	// problem that failed to load, or the problem as the caller named it.
	Name string
	// Problem is the problem validated; nil for a pack or a problem that
	// failed to load.
	Problem *problem.Problem
	// Cases is the problem's number of cases.
	Cases    int
	Errors   []string
	Warnings []string
}

// OK reports whether nothing is wrong; warnings are allowed.
func (f Finding) OK() bool { return len(f.Errors) == 0 }

// ValidateSummary counts the findings of ValidateWorkspace.
type ValidateSummary struct {
	// Problems is the number of problems found, loaded or not; Failed how
	// many of them have errors.
	Problems, Failed int
	// PackErrors is the number of packs whose manifest has errors.
	PackErrors int
}

// OK reports whether every pack and problem is valid.
func (s ValidateSummary) OK() bool { return s.Failed == 0 && s.PackErrors == 0 }

// ValidateWorkspace validates every pack of the workspace and its problems,
// pack by pack, and passes each finding to report as it is made: the
// manifest, its groups against the problems, problems that fail to load, and
// every problem (see ValidateProblem).
func (a *App) ValidateWorkspace(ctx context.Context, o ValidateOptions, report func(Finding)) (ValidateSummary, error) {
	var sum ValidateSummary
	packs, err := a.WS.Packs()
	if err != nil {
		return sum, err
	}
	all, _ := a.WS.Problems()
	qualify := workspace.SpansPacks(all)
	for _, k := range packs {
		label := "pack " + k.Name
		if k.Err != nil {
			sum.PackErrors++
			report(Finding{Name: label, Errors: lines(k.Err)})
			continue
		}
		probs, failed, err := k.Problems()
		if err != nil {
			return sum, err
		}
		if err := k.Manifest.ValidateProblems(probs); err != nil {
			sum.PackErrors++
			report(Finding{Name: label, Errors: lines(err)})
		}
		for _, f := range failed {
			sum.Problems++
			sum.Failed++
			report(Finding{Name: a.WS.Rel(f.Dir), Errors: lines(f)})
		}
		for _, p := range probs {
			name := p.Slug
			if qualify {
				name = p.Ref()
			}
			f := a.ValidateProblem(ctx, p, probs, name, o)
			sum.Problems++
			if !f.OK() {
				sum.Failed++
			}
			report(f)
		}
	}
	return sum, nil
}

func lines(err error) []string { return strings.Split(err.Error(), "\n") }

// ValidateProblem validates p against the protocol: problem.yaml,
// cases.jsonl, starter generation, its statements and titles in the pack's
// locales, their images and its identity among peers, the problems of its pack (nil loads them). name is how hints
// name the problem. With o.Run, every reference runs on every case.
func (a *App) ValidateProblem(ctx context.Context, p *problem.Problem, peers []*problem.Problem, name string, o ValidateOptions) Finding {
	f := Finding{Name: name, Problem: p}
	var errs, warns []string
	if err := p.Validate(); err != nil {
		errs = append(errs, lines(err)...)
	}
	cases, casesErr := p.LoadCases()
	if casesErr != nil {
		errs = append(errs, casesErr.Error())
	}
	f.Cases = len(cases)
	missing, examples := 0, 0
	if len(errs) == 0 {
		for i, c := range cases {
			if err := p.ValidateCase(c); err != nil {
				errs = append(errs, fmt.Sprintf("case %d: %v", i+1, err))
			}
		}
	}
	for _, c := range cases {
		if !c.HasExpected() {
			missing++
		}
		if c.Example {
			examples++
		}
	}
	if len(errs) == 0 {
		for _, l := range a.Langs.All() {
			if _, err := l.Starter(p); err != nil {
				errs = append(errs, err.Error())
			}
		}
	}
	// A published problem must be one users can submit, and whose expected
	// outputs a reference has computed, so strict mode fails what plain
	// validation only warns about.
	gap := func(msg string) {
		if o.Strict {
			errs = append(errs, msg)
		} else {
			warns = append(warns, msg)
		}
	}
	if len(cases) == 0 && casesErr == nil {
		gap("no test cases (hc gen " + name + ")")
	}
	if missing > 0 {
		gap(fmt.Sprintf("%d case(s) without expected output (hc cases build %s)", missing, name))
	}
	if len(cases) > 0 && examples == 0 {
		warns = append(warns, "no case marked as example; hc test runs the first 3")
	}
	if !p.Ready() {
		msg := "judge rule not decided (judge.compare): run `hc gen " + name + "`, or set it by hand"
		if o.Strict {
			msg = "not ready: " + msg
		}
		gap(msg)
	}
	warns = append(warns, imageWarnings(p)...)
	refs := a.ReferenceLangs(p)
	if len(refs) == 0 {
		gap("no reference solution")
	} else if o.Run && len(errs) == 0 && len(cases) > 0 {
		sel, _ := Select(cases, nil, true)
		for _, l := range refs {
			tc, err := l.Toolchain()
			if err != nil {
				gap(fmt.Sprintf("%s reference not run: %v", l.Name(), err))
				continue
			}
			if err := tc.Err(); err != nil {
				errs = append(errs, fmt.Sprintf("%s reference not run: %v", l.Name(), err))
				continue
			}
			refErrs := a.checkReference(ctx, p, l, sel, o.Stderr)
			// The reference ran on the local toolchain, which may well be
			// newer than the protocol's oldest.
			if mc, ok := l.(lang.MinVersionChecker); ok && len(refErrs) == 0 {
				file, _ := p.Reference(l.SolutionFile())
				if err := mc.CheckMinVersion(ctx, p, file); err != nil {
					refErrs = append(refErrs, fmt.Sprintf("%s reference does not work on %s %s: %v", l.Name(), l.Name(), tc.Min, err))
				}
			}
			errs = append(errs, refErrs...)
		}
	}
	// Conflicts with the rest of the pack come last, so that they do not
	// keep the checks above from running.
	k, err := a.WS.Pack(p.Pack)
	if err == nil {
		// Statements and titles in the pack's locales.
		if err := p.ValidateLocales(k.Manifest.Locales); err != nil {
			errs = append(errs, lines(err)...)
		}
	}
	if peers == nil && k != nil {
		peers, _, _ = k.Problems()
	}
	for _, err := range pack.IdentityErrors(p, peers) {
		errs = append(errs, err.Error())
	}
	f.Errors, f.Warnings = errs, warns
	return f
}

// checkReference runs one reference solution on every case. Cases without an
// expected output only have to run.
func (a *App) checkReference(ctx context.Context, p *problem.Problem, l lang.Language, sel []runner.Selected, stderr io.Writer) []string {
	if stderr == nil {
		stderr = io.Discard
	}
	rep, _, err := a.RunReference(ctx, p, l.ID(), sel, RunOptions{Stderr: stderr})
	switch {
	case err != nil:
		return []string{fmt.Sprintf("%s reference: %v", l.Name(), err)}
	case rep.Verdict == runner.CompileError:
		return []string{fmt.Sprintf("%s reference does not compile:\n%s", l.Name(), strings.TrimRight(rep.CompileError, "\n"))}
	}
	var errs []string
	for _, c := range rep.Cases {
		switch c.Verdict {
		case runner.Accepted, runner.NoExpected, runner.NotJudged:
		case runner.WrongAnswer:
			e := fmt.Sprintf("%s reference, case %d: Wrong Answer\n%-9s %s\n%-9s %s",
				l.Name(), c.Index, "output", clip(string(c.Output), 120),
				strings.ToLower(ui.ExpectedLabel(p)), clip(string(c.Case.Expected), 120))
			if c.Message != "" {
				e += fmt.Sprintf("\n%-9s %s", "checker", c.Message)
			}
			errs = append(errs, e)
		default:
			errs = append(errs, fmt.Sprintf("%s reference, case %d: %s %s", l.Name(), c.Index, c.Verdict, c.Error))
		}
	}
	return errs
}

// clip shortens s to n bytes.
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + fmt.Sprintf("… (+%d bytes)", len(s)-n)
}

// imageWarnings reports local images in the statements, in every locale,
// that do not exist.
func imageWarnings(p *problem.Problem) []string {
	files := []string{problem.DescriptionFile}
	translations, _ := p.Translations()
	for _, l := range translations {
		files = append(files, problem.TranslationFile(l))
	}
	var warns []string
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(p.Dir, file))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			warns = append(warns, err.Error())
			continue
		}
		problem.ReplaceImages(string(data), func(_, src string) string {
			if strings.Contains(src, "://") || strings.HasPrefix(src, "data:") {
				return ""
			}
			if _, err := os.Stat(filepath.Join(p.Dir, filepath.FromSlash(src))); err != nil {
				warns = append(warns, fmt.Sprintf("%s: image %s not found", file, src))
			}
			return ""
		})
	}
	return warns
}
