package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

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
	// ready, has no cases, cases without expected outputs or digests,
	// stored cases over problem.MaxStoredCase or no reference, or a
	// reference that cannot be run for want of its toolchain, is an error
	// rather than a warning.
	Strict bool
	// Stderr receives the output of reference runs, such as the creation of
	// the Python environment. Nil discards it.
	Stderr io.Writer
	// Jobs is how many problems ValidateWorkspace and ValidateProblems
	// validate at once; 0 is 1. While more than one are, the output of a
	// problem's reference runs goes to Stderr just before its finding is
	// reported, and the runs add nothing to the references' baselines
	// for time limits (see untimed). Findings are reported in the same order either way.
	Jobs int
}

// DefaultJobs is the number of problems hc validate and hc pack check
// validate at once by default: one per CPU, at most 4. A problem with
// references in two languages, a generator or a checker keeps more than
// one process busy at times, and more jobs mostly add memory and
// contention.
func DefaultJobs() int { return min(runtime.NumCPU(), 4) }

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
// pack by pack, and passes each finding to report, in that order, as soon
// as it and those before it are made: the manifest, its groups against the
// problems, problems that fail to load, and every problem (see
// ValidateProblem), o.Jobs problems at once.
func (a *App) ValidateWorkspace(ctx context.Context, o ValidateOptions, report func(Finding)) (ValidateSummary, error) {
	var sum ValidateSummary
	packs, err := a.WS.Packs()
	if err != nil {
		return sum, err
	}
	all, _ := a.WS.Problems()
	qualify := workspace.SpansPacks(all)
	var items []validateItem
	for _, k := range packs {
		label := "pack " + k.Name
		if k.Err != nil {
			items = append(items, validateItem{f: Finding{Name: label, Errors: lines(k.Err)}})
			continue
		}
		probs, failed, err := k.Problems()
		if err != nil {
			// Returned once the findings before it are reported.
			items = append(items, validateItem{err: err})
			break
		}
		if err := k.Manifest.ValidateProblems(probs); err != nil {
			items = append(items, validateItem{f: Finding{Name: label, Errors: lines(err)}})
		}
		for _, f := range failed {
			items = append(items, validateItem{f: Finding{Name: a.WS.Rel(f.Dir), Errors: lines(f)}, failed: true})
		}
		for _, p := range probs {
			name := p.Slug
			if qualify {
				name = p.Ref()
			}
			items = append(items, validateItem{p: p, peers: probs, name: name})
		}
	}
	err = a.validateItems(ctx, items, o, func(it validateItem) {
		switch {
		case it.p != nil || it.failed:
			sum.Problems++
			if !it.f.OK() {
				sum.Failed++
			}
		default:
			sum.PackErrors++
		}
		report(it.f)
	})
	return sum, err
}

// ValidateProblems validates the problems probs (see ValidateProblem), each
// named as WS.Ref names it, o.Jobs at once, and passes each finding to
// report, in their order, as soon as it and those before it are made.
func (a *App) ValidateProblems(ctx context.Context, probs []*problem.Problem, o ValidateOptions, report func(Finding)) ValidateSummary {
	var sum ValidateSummary
	items := make([]validateItem, len(probs))
	for i, p := range probs {
		items[i] = validateItem{p: p, name: a.WS.Ref(p)}
	}
	_ = a.validateItems(ctx, items, o, func(it validateItem) {
		sum.Problems++
		if !it.f.OK() {
			sum.Failed++
		}
		report(it.f)
	})
	return sum
}

// validateItem is a problem to validate (p), a finding made already (f: a
// pack's, or of a problem that failed to load), or an error that ends the
// validation.
type validateItem struct {
	p      *problem.Problem
	peers  []*problem.Problem
	name   string
	f      Finding
	failed bool
	err    error
}

// validateItems validates the problems of items, o.Jobs at once, and passes
// the items, with their findings, to report in their order, from the
// calling goroutine, up to an item's error, which it returns.
func (a *App) validateItems(ctx context.Context, items []validateItem, o ValidateOptions, report func(validateItem)) error {
	var todo []int
	for i, it := range items {
		if it.p != nil {
			todo = append(todo, i)
		}
	}
	jobs := min(max(o.Jobs, 1), len(todo))
	if jobs <= 1 {
		for _, it := range items {
			if it.err != nil {
				return it.err
			}
			if it.p != nil {
				it.f = a.ValidateProblem(ctx, it.p, it.peers, it.name, o)
			}
			report(it)
		}
		return nil
	}
	if o.Run {
		a.setUp(ctx, items, o.Stderr)
	}
	// A reference timed while other problems run makes a noisy baseline.
	defer a.untimed()()
	// A problem's output is kept until its finding is reported, so that it
	// comes before it, as it does one problem at a time.
	type result struct {
		f      Finding
		stderr bytes.Buffer
	}
	results := make([]result, len(items))
	done := make([]chan struct{}, len(items))
	next := make(chan int, len(todo))
	for _, i := range todo {
		done[i] = make(chan struct{})
		next <- i
	}
	close(next)
	var wg sync.WaitGroup
	defer wg.Wait()
	for range jobs {
		wg.Go(func() {
			for i := range next {
				po := o
				if o.Stderr != nil {
					po.Stderr = &results[i].stderr
				}
				results[i].f = a.ValidateProblem(ctx, items[i].p, items[i].peers, items[i].name, po)
				close(done[i])
			}
		})
	}
	for i, it := range items {
		if it.err != nil {
			// Only the last item has one, after every problem.
			return it.err
		}
		if it.p != nil {
			<-done[i]
			if o.Stderr != nil {
				_, _ = o.Stderr.Write(results[i].stderr.Bytes())
			}
			it.f = results[i].f
		}
		report(it)
	}
	return nil
}

// setUp prepares, before problems are validated at once, what their
// reference runs would each prepare on first use: the environment of every
// language they run in (the Python environment, whose progress goes to w),
// and the detection of its toolchain. A failure is left to each problem to
// report, as it would be one problem at a time.
func (a *App) setUp(ctx context.Context, items []validateItem, w io.Writer) {
	need := map[string]bool{}
	for _, it := range items {
		if it.p == nil {
			continue
		}
		for _, l := range a.ReferenceLangs(it.p) {
			need[l.ID()] = true
		}
		if it.p.HasGenerator() || it.p.Judge.Compare == problem.CompareChecker {
			need["python"] = true
		}
	}
	for _, l := range a.Langs.All() {
		if need[l.ID()] && l.Setup(ctx, w) == nil {
			_, _ = l.Toolchain()
		}
	}
}

func lines(err error) []string { return strings.Split(err.Error(), "\n") }

// ValidateProblem validates p against the protocol: problem.yaml,
// cases.jsonl (with generator.py, and the size of stored cases), starter
// generation, its statements and titles in the pack's
// locales, their images and its identity among peers, the problems of its
// pack (nil loads them). name is how hints name the problem. With o.Run,
// every reference runs on every case, generated cases built afresh.
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
	missing, examples, generated, undigested := 0, 0, 0, 0
	var large []string
	if len(errs) == 0 {
		for i, c := range cases {
			if err := p.ValidateCase(c); err != nil {
				errs = append(errs, fmt.Sprintf("case %d: %v", i+1, err))
			}
		}
	}
	for i, c := range cases {
		switch {
		case c.Generated():
			generated++
			if c.Digest == "" {
				undigested++
			}
		case !c.HasExpected():
			missing++
		}
		if c.Example {
			examples++
		}
		if line, err := problem.MarshalCase(c); err == nil && len(line) > problem.MaxStoredCase && !c.Generated() {
			large = append(large, fmt.Sprintf("case %d (%d KB)", i+1, len(line)>>10))
		}
	}
	switch hasGen := p.HasGenerator(); {
	case generated > 0 && !hasGen:
		errs = append(errs, fmt.Sprintf("%d case(s) are generated, but %s is missing", generated, problem.GeneratorFile))
	case generated == 0 && hasGen && casesErr == nil:
		errs = append(errs, fmt.Sprintf("%s is only used by generated cases (\"generate\" in %s); add some, or remove it", problem.GeneratorFile, problem.CasesFile))
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
	if undigested > 0 {
		gap(fmt.Sprintf("%d generated case(s) without digest (hc cases build %s)", undigested, name))
	}
	if len(large) > 0 {
		gap(fmt.Sprintf("stored cases over %d KB: %s; build large inputs with %s instead (\"generate\")", problem.MaxStoredCase>>10, strings.Join(large, ", "), problem.GeneratorFile))
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
	} else {
		// A solution is judged only with a reference in its own language,
		// so every language the pack is solved in needs one.
		for _, l := range a.missingReferences(p) {
			msg := fmt.Sprintf("no %s reference solution (%s/%s), so %s solutions are not judged: run `hc gen %s`, or write it", l.Name(), problem.ReferenceDir, l.SolutionFile(), l.Name(), name)
			if o.Strict {
				msg = "not ready: " + msg
			}
			gap(msg)
		}
	}
	if len(refs) > 0 && o.Run && len(errs) == 0 && len(cases) > 0 {
		sel, _ := Select(cases, nil, true)
		// Generated cases are built afresh, never from the cache, so that
		// their digests check the generator; the first reference that runs
		// computes their expected outputs, in the run that checks it on
		// every case, and the others must match them. Every reference so
		// runs every case once.
		if err := a.generateInputs(ctx, p, sel, true, o.Stderr); err != nil {
			errs = append(errs, lines(err)...)
			refs = nil
		}
		expect := generated > 0
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
			refErrs, ok := a.checkReference(ctx, p, l, sel, expect, o.Stderr)
			if !ok {
				// The other references have no expected outputs to match
				// on the generated cases.
				errs = append(errs, refErrs...)
				break
			}
			expect = false
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

// missingReferences are the languages p's pack is solved in that p has no
// reference solution in, in the pack's order.
func (a *App) missingReferences(p *problem.Problem) []lang.Language {
	k, err := a.WS.Pack(p.Pack)
	if err != nil {
		return nil
	}
	var out []lang.Language
	for _, id := range k.Languages() {
		l, err := a.Langs.Get(id)
		if err != nil {
			continue
		}
		if _, ok := p.Reference(l.SolutionFile()); !ok {
			out = append(out, l)
		}
	}
	return out
}

// checkReference runs one reference solution on every case. Cases without an
// expected output only have to run. With expect, the reference computes the
// expected outputs of the generated cases in that run, and sets them in sel:
// it is judged on them trivially, or by the checker (see
// runner.Options.ExpectGenerated), and the run times it on them, as
// expectGenerated's run would. ok is false when it could not compute them
// all, which leaves the other references nothing to match on them.
func (a *App) checkReference(ctx context.Context, p *problem.Problem, l lang.Language, sel []runner.Selected, expect bool, stderr io.Writer) (errs []string, ok bool) {
	if stderr == nil {
		stderr = io.Discard
	}
	rep, _, err := a.runReference(ctx, p, l.ID(), sel, RunOptions{Stderr: stderr}, expect)
	switch {
	case err != nil:
		return []string{fmt.Sprintf("%s reference: %v", l.Name(), err)}, !expect
	case rep.Verdict == runner.CompileError:
		return []string{fmt.Sprintf("%s reference does not compile:\n%s", l.Name(), strings.TrimRight(rep.CompileError, "\n"))}, !expect
	}
	// unexpected is whether a case is generated and got no expected output.
	unexpected := func(c runner.CaseResult) bool { return expect && c.Case.Generated() && !c.Case.HasExpected() }
	if expect {
		var gen []runner.Selected
		var times []runner.CaseResult
		ok = true
		for i, c := range rep.Cases {
			switch {
			case unexpected(c):
				ok = false
			case c.Case.Generated():
				sel[i].Case.Expected = c.Case.Expected
				gen, times = append(gen, sel[i]), append(times, c)
			}
		}
		if ok {
			a.recordBaseline(p, l, gen, &runner.Report{Cases: times})
		}
	} else {
		ok = true
	}
	reported := false
	for _, c := range rep.Cases {
		if unexpected(c) && c.Verdict != runner.NotRun {
			// The first generated case the reference fails on: the
			// problem's generated cases cannot be built.
			if !reported {
				file, _ := p.Reference(l.SolutionFile())
				ge := &runner.GeneratedCaseError{Path: a.WS.Rel(file), Case: c.Index, Reason: "the reference solution: " + string(c.Verdict), Detail: c.Error}
				errs = append(errs, lines(ge)...)
				reported = true
			}
			continue
		}
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
	return errs, ok
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
