// Package app implements hc's use cases on top of the workspace, languages,
// runner and store. Both the CLI and the TUI call into it.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/MorePeanuts/handcoding/internal/judge"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/lang/golang"
	"github.com/MorePeanuts/handcoding/internal/lang/python"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// App bundles everything a command needs.
type App struct {
	WS    *workspace.Workspace
	Langs *lang.Registry
	store *store.Store
}

// New creates an App for the workspace.
func New(ws *workspace.Workspace) *App {
	return &App{
		WS:    ws,
		Langs: Languages(ws),
	}
}

// Languages returns the registry of supported languages. New languages are
// added here.
func Languages(ws *workspace.Workspace) *lang.Registry {
	return lang.NewRegistry(
		golang.Language{},
		pythonLanguage(ws),
	)
}

// LanguageID resolves a language ID or alias (e.g. "py") to the ID stored in
// hc.yaml, before any workspace exists.
func LanguageID(name string) (string, error) {
	l, err := Languages(&workspace.Workspace{}).Get(name)
	if err != nil {
		return "", err
	}
	return l.ID(), nil
}

// pythonLanguage runs solutions with python.interpreter from hc.yaml if it
// is set, and otherwise in a virtual environment in the workspace.
func pythonLanguage(ws *workspace.Workspace) python.Language {
	if ws.Config.Python.Interpreter != "" {
		return python.Language{Interpreter: ws.Config.Python.Interpreter}
	}
	return python.Language{Venv: &python.Venv{
		Dir:     ws.VenvPath(),
		Label:   filepath.Join(workspace.StateDir, workspace.VenvDir),
		Pyright: filepath.Join(ws.Root, workspace.PyrightConfig),
	}}
}

// SyncEnv creates the Python virtual environment again and returns the
// toolchain it provides. Progress goes to w.
func (a *App) SyncEnv(ctx context.Context, w io.Writer) (lang.Toolchain, error) {
	l, err := a.Langs.Get("python")
	if err != nil {
		return lang.Toolchain{}, err
	}
	py, ok := l.(python.Language)
	if !ok || py.Venv == nil {
		return lang.Toolchain{}, fmt.Errorf("python.interpreter is set in %s, so hc manages no virtual environment", workspace.ConfigFile)
	}
	if err := py.Venv.Rebuild(ctx, w); err != nil {
		return lang.Toolchain{}, err
	}
	return py.Toolchain()
}

// RetryEnv lets the next run try again to create the Python virtual
// environment if an earlier attempt failed. Within one command hc tries
// once; the TUI calls this before each run (see python.Venv.Retry).
func (a *App) RetryEnv() {
	l, err := a.Langs.Get("python")
	if err != nil {
		return
	}
	if py, ok := l.(python.Language); ok && py.Venv != nil {
		py.Venv.Retry()
	}
}

// Close releases resources.
func (a *App) Close() error {
	if a.store != nil {
		return a.store.Close()
	}
	return nil
}

// Store opens the history database lazily.
func (a *App) Store() (*store.Store, error) {
	if a.store != nil {
		return a.store, nil
	}
	if err := os.MkdirAll(a.WS.StatePath(), 0o755); err != nil {
		return nil, err
	}
	s, err := store.Open(filepath.Join(a.WS.StatePath(), "hc.db"))
	if err != nil {
		return nil, err
	}
	a.store = s
	return s, nil
}

// Lang resolves a language name, falling back to the configured default.
func (a *App) Lang(name string) (lang.Language, error) {
	if name == "" {
		name = a.WS.Config.Language
	}
	return a.Langs.Get(name)
}

// SolutionLang resolves the language to solve p in: name, or the default
// from hc.yaml. A pack can limit the languages its problems are solved in
// (pack.yaml's languages): a language it lacks is an error when named, and
// the default falls back to the pack's first language, with a note saying
// so.
func (a *App) SolutionLang(p *problem.Problem, name string) (l lang.Language, note string, err error) {
	l, err = a.Lang(name)
	if err != nil {
		return nil, "", err
	}
	err = a.CheckLang(p, l)
	if err == nil || name != "" {
		return l, "", err
	}
	k, kerr := a.WS.Pack(p.Pack)
	if kerr != nil {
		return nil, "", kerr
	}
	for _, id := range k.Languages() {
		if fb, err := a.Langs.Get(id); err == nil {
			return fb, fmt.Sprintf("pack %s is not solved in %s, using %s", k.Name, l.Name(), fb.Name()), nil
		}
	}
	return nil, "", err
}

// CheckLang returns an error unless p's pack is solved in l.
func (a *App) CheckLang(p *problem.Problem, l lang.Language) error {
	k, err := a.WS.Pack(p.Pack)
	if err != nil {
		return err
	}
	if langs := k.Languages(); !slices.Contains(langs, l.ID()) {
		return fmt.Errorf("pack %s is solved in %s, not %s", k.Name, strings.Join(langs, ", "), l.ID())
	}
	return nil
}

// SolutionPath is the file the user edits.
func (a *App) SolutionPath(p *problem.Problem, l lang.Language) string {
	return filepath.Join(a.WS.SolutionDir(p, l.ID()), l.SolutionFile())
}

// HasSolution reports whether the user started p in l.
func (a *App) HasSolution(p *problem.Problem, l lang.Language) bool {
	_, err := os.Stat(a.SolutionPath(p, l))
	return err == nil
}

// Start creates the solution file from the starter template if it does not
// exist yet, and (re)generates the driver so editors can resolve symbols.
// The problem's pack must be solved in l (see CheckLang).
func (a *App) Start(p *problem.Problem, l lang.Language) (path string, created bool, err error) {
	if err := a.CheckLang(p, l); err != nil {
		return "", false, err
	}
	path = a.SolutionPath(p, l)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", false, err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		code, err := l.Starter(p)
		if err != nil {
			return "", false, err
		}
		if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
			return "", false, err
		}
		created = true
	} else if err != nil {
		return "", false, err
	}
	return path, created, l.Prepare(dir, p)
}

// Reset overwrites the solution with the starter template.
func (a *App) Reset(p *problem.Problem, l lang.Language) (string, error) {
	if err := a.CheckLang(p, l); err != nil {
		return "", err
	}
	path := a.SolutionPath(p, l)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	path, _, err := a.Start(p, l)
	return path, err
}

// Discard deletes the user's solutions of p in every language and its
// submission history, so the problem is back to "not started". The problem
// itself is kept. It returns the number of deleted submissions.
func (a *App) Discard(ctx context.Context, p *problem.Problem) (int64, error) {
	for _, l := range a.Langs.All() {
		if err := os.RemoveAll(a.WS.SolutionDir(p, l.ID())); err != nil {
			return 0, err
		}
	}
	st, err := a.Store()
	if err != nil {
		return 0, err
	}
	return st.DeleteProblem(ctx, p.Pack, p.Slug)
}

// Removed is what RemoveProblem did.
type Removed struct {
	// Groups are the groups of the pack the problem was taken out of.
	Groups []string
	// Purged is set when the solutions, history and star went too;
	// Submissions is then the number of deleted submissions.
	Purged      bool
	Submissions int64
	// Kept is set when the user's solutions or history of the problem
	// stay, without the problem: purge was not asked for.
	Kept bool
}

// RemoveProblem deletes p from its pack, which must be writable: its
// directory, and its slug from the pack's groups. With purge, the user's
// solutions, submission history and star of p go too; without, they stay,
// and a problem with the same slug added later gets them back.
func (a *App) RemoveProblem(ctx context.Context, p *problem.Problem, purge bool) (Removed, error) {
	var r Removed
	if err := a.WS.CheckWritable(p); err != nil {
		return r, err
	}
	k, err := a.WS.Pack(p.Pack)
	if err != nil {
		return r, err
	}
	// The groups first: a pack whose groups name a problem it lacks is
	// invalid, one with a problem in no group is not.
	if r.Groups, err = k.RemoveFromGroups(p.Slug); err != nil {
		return r, err
	}
	st, err := a.Store()
	if err != nil {
		return r, err
	}
	if purge {
		if r.Submissions, err = a.Discard(ctx, p); err != nil {
			return r, err
		}
		if err := st.SetStar(ctx, p.Pack, p.Slug, false); err != nil {
			return r, err
		}
		r.Purged = true
	} else {
		subs, err := st.List(ctx, p.Pack, p.Slug)
		if err != nil {
			return r, err
		}
		r.Kept = len(subs) > 0 || slices.ContainsFunc(a.Langs.All(), func(l lang.Language) bool { return a.HasSolution(p, l) })
	}
	return r, os.RemoveAll(p.Dir)
}

// Restore writes code (e.g. from a past submission) into the solution file.
func (a *App) Restore(p *problem.Problem, l lang.Language, code string) (string, error) {
	path, _, err := a.Start(p, l)
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, []byte(code), 0o644)
}

// Select picks cases by 1-based index; with no indices it picks examples (or
// the first three cases if none is marked) unless all is set.
func Select(cases []problem.Case, indices []int, all bool) ([]runner.Selected, error) {
	var sel []runner.Selected
	switch {
	case len(indices) > 0:
		for _, i := range indices {
			if i < 1 || i > len(cases) {
				return nil, fmt.Errorf("case %d out of range (1-%d)", i, len(cases))
			}
			sel = append(sel, runner.Selected{Index: i, Case: cases[i-1]})
		}
	case all:
		for i, c := range cases {
			sel = append(sel, runner.Selected{Index: i + 1, Case: c})
		}
	default:
		for i, c := range cases {
			if c.Example {
				sel = append(sel, runner.Selected{Index: i + 1, Case: c})
			}
		}
		if len(sel) == 0 {
			for i, c := range cases[:min(3, len(cases))] {
				sel = append(sel, runner.Selected{Index: i + 1, Case: c})
			}
		}
	}
	return sel, nil
}

// RunOptions configure Run.
type RunOptions struct {
	Stdout io.Writer
	Stderr io.Writer
}

// Run runs the user's solution against the selected cases.
func (a *App) Run(ctx context.Context, p *problem.Problem, l lang.Language, sel []runner.Selected, o RunOptions) (*runner.Report, error) {
	if err := a.CheckLang(p, l); err != nil {
		return nil, err
	}
	if !a.HasSolution(p, l) {
		return nil, fmt.Errorf("no %s solution for %s yet; run `hc start %s -l %s`", l.Name(), p.Slug, a.WS.Ref(p), l.ID())
	}
	if err := l.Setup(ctx, o.Stderr); err != nil {
		return nil, err
	}
	// The user's code may well run on an old toolchain; the report carries
	// the toolchain so the front ends can warn.
	tc, err := l.Toolchain()
	if err != nil {
		return nil, err
	}
	opts, err := a.runnerOpts(ctx, p, sel, o)
	if err != nil {
		return nil, err
	}
	rep, err := runner.Run(ctx, l, p, filepath.Dir(a.SolutionPath(p, l)), sel, opts)
	if rep != nil {
		rep.Toolchain = tc
	}
	return rep, err
}

func (a *App) runnerOpts(ctx context.Context, p *problem.Problem, sel []runner.Selected, o RunOptions) (runner.Options, error) {
	timeout := a.WS.Config.TimeoutDuration()
	if p.Judge.Timeout != "" {
		if d, err := time.ParseDuration(p.Judge.Timeout); err == nil {
			timeout = d
		}
	}
	opts := runner.Options{Timeout: timeout, Stdout: o.Stdout, Stderr: o.Stderr}
	if p.Judge.Compare == problem.CompareChecker && slices.ContainsFunc(sel, func(s runner.Selected) bool { return s.Case.HasExpected() }) {
		c, err := a.checker(ctx, p, o.Stderr)
		if err != nil {
			return opts, err
		}
		opts.Checker = c
		opts.CheckerName = a.WS.Rel(p.CheckerPath())
	}
	return opts, nil
}

// checker returns what runs the problem's checker.py: the workspace's
// Python, whatever the solution's language. Unlike solutions, checkers are
// part of the problem, so they need a supported Python.
func (a *App) checker(ctx context.Context, p *problem.Problem, w io.Writer) (lang.Checker, error) {
	l, err := a.Langs.Get("python")
	if err != nil {
		return nil, err
	}
	c, ok := l.(lang.Checker)
	if !ok {
		return nil, errors.New("python cannot run checkers")
	}
	needs := fmt.Sprintf("%s is judged by %s, which needs Python %s or later", p.Slug, problem.CheckerFile, python.MinVersion)
	if err := l.Setup(ctx, w); err != nil {
		return nil, fmt.Errorf("%s: %w", needs, err)
	}
	tc, err := l.Toolchain()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", needs, err)
	}
	if err := tc.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", needs, err)
	}
	return c, nil
}

// Submit runs all cases and records the submission.
func (a *App) Submit(ctx context.Context, p *problem.Problem, l lang.Language, o RunOptions) (*runner.Report, *store.Submission, error) {
	if !p.Ready() {
		return nil, nil, errors.New("cannot submit: " + p.NotReadyHint(a.WS.Ref(p)) + "; `hc test` shows your output meanwhile")
	}
	cases, err := p.LoadCases()
	if err != nil {
		return nil, nil, err
	}
	if len(cases) == 0 {
		return nil, nil, fmt.Errorf("%s has no test cases; generate some with `hc gen %s`", p.Slug, a.WS.Ref(p))
	}
	for i, c := range cases {
		if !c.HasExpected() {
			return nil, nil, fmt.Errorf("case %d has no expected output; run `hc cases build %s` or `hc gen %s`", i+1, a.WS.Ref(p), a.WS.Ref(p))
		}
	}
	sel, _ := Select(cases, nil, true)
	rep, err := a.Run(ctx, p, l, sel, o)
	if err != nil {
		return nil, nil, err
	}
	code, err := os.ReadFile(a.SolutionPath(p, l))
	if err != nil {
		return nil, nil, err
	}
	var total time.Duration
	for _, c := range rep.Cases {
		total += c.Elapsed
	}
	sub := &store.Submission{
		Pack: p.Pack, Slug: p.Slug, Lang: l.ID(), Verdict: string(rep.Verdict),
		Passed: rep.Passed, Total: rep.Total, ElapsedMs: total.Milliseconds(), Code: string(code),
		Details: EncodeReport(p, rep),
	}
	st, err := a.Store()
	if err != nil {
		return rep, nil, err
	}
	return rep, sub, st.Add(ctx, sub)
}

// ReferenceLang returns the language of the problem's reference solution,
// preferring name if given.
func (a *App) ReferenceLang(p *problem.Problem, name string) (lang.Language, string, error) {
	if name != "" {
		l, err := a.Langs.Get(name)
		if err != nil {
			return nil, "", err
		}
		path, ok := p.Reference(l.SolutionFile())
		if !ok {
			return nil, "", fmt.Errorf("%s has no %s reference solution (%s/%s)", p.Slug, l.Name(), problem.ReferenceDir, l.SolutionFile())
		}
		return l, path, nil
	}
	for _, l := range a.Langs.All() {
		if path, ok := p.Reference(l.SolutionFile()); ok {
			return l, path, nil
		}
	}
	return nil, "", fmt.Errorf("%s has no reference solution in %s/", p.Slug, filepath.Join(p.Dir, problem.ReferenceDir))
}

// ReferenceLangs returns every language the problem has a reference solution
// in, in registry order.
func (a *App) ReferenceLangs(p *problem.Problem) []lang.Language {
	var out []lang.Language
	for _, l := range a.Langs.All() {
		if _, ok := p.Reference(l.SolutionFile()); ok {
			out = append(out, l)
		}
	}
	return out
}

// RunReference runs the reference solution in a scratch directory.
func (a *App) RunReference(ctx context.Context, p *problem.Problem, langName string, sel []runner.Selected, o RunOptions) (*runner.Report, lang.Language, error) {
	l, path, err := a.ReferenceLang(p, langName)
	if err != nil {
		return nil, nil, err
	}
	if err := l.Setup(ctx, o.Stderr); err != nil {
		return nil, nil, err
	}
	// Expected outputs and pack references must come from a toolchain the
	// protocol supports.
	tc, err := l.Toolchain()
	if err != nil {
		return nil, nil, err
	}
	if err := tc.Err(); err != nil {
		return nil, nil, err
	}
	dir, err := os.MkdirTemp("", "hc-ref-*")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(dir)
	code, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, l.SolutionFile()), code, 0o644); err != nil {
		return nil, nil, err
	}
	opts, err := a.runnerOpts(ctx, p, sel, o)
	if err != nil {
		return nil, nil, err
	}
	rep, err := runner.Run(ctx, l, p, dir, sel, opts)
	if rep != nil {
		rep.Toolchain = tc
	}
	return rep, l, err
}

// BuildResult summarizes BuildCases.
type BuildResult struct {
	Lang    lang.Language
	Filled  int
	Changed []int
	Report  *runner.Report
}

// BuildCases fills missing expected outputs (all of them with force) by
// running the reference solution.
func (a *App) BuildCases(ctx context.Context, p *problem.Problem, langName string, force bool) (*BuildResult, error) {
	cases, err := p.LoadCases()
	if err != nil {
		return nil, err
	}
	var sel []runner.Selected
	for i, c := range cases {
		if force {
			// The old expected output is about to be replaced, so a
			// malformed one must not block the rebuild.
			c.Expected = nil
		}
		if err := p.ValidateCase(c); err != nil {
			if c.HasExpected() && p.ValidateCase(problem.Case{Input: c.Input}) == nil {
				return nil, fmt.Errorf("case %d: %w (rerun with --force to recompute it)", i+1, err)
			}
			return nil, fmt.Errorf("case %d: %w", i+1, err)
		}
		if !c.HasExpected() {
			sel = append(sel, runner.Selected{Index: i + 1, Case: c})
		}
	}
	res := &BuildResult{}
	if len(sel) == 0 {
		return res, nil
	}
	if p.Judge.Compare == problem.CompareAnyOf {
		// A reference finds one answer, but any_of needs all of them: storing
		// the one answer would reject every other correct one.
		nums := make([]string, len(sel))
		for i, s := range sel {
			nums[i] = fmt.Sprint(s.Index)
		}
		return nil, fmt.Errorf("judge.compare is any_of, so expected must list every acceptable answer, which a reference solution cannot compute; write it by hand for case(s) %s (`hc validate --run` checks the reference against it)", strings.Join(nums, ", "))
	}
	rep, l, err := a.RunReference(ctx, p, langName, sel, RunOptions{Stderr: os.Stderr})
	if err != nil {
		return nil, err
	}
	res.Lang, res.Report = l, rep
	if rep.Verdict == runner.CompileError {
		return res, fmt.Errorf("reference solution does not compile:\n%s", rep.CompileError)
	}
	var failed []string
	for _, cr := range rep.Cases {
		if cr.Verdict != runner.NoExpected {
			failed = append(failed, fmt.Sprintf("case %d: %s %s", cr.Index, cr.Verdict, cr.Error))
			continue
		}
		old := cases[cr.Index-1].Expected
		if old != nil {
			opts := judge.OptionsFor(p)
			if opts.Mode == problem.CompareChecker {
				// Expected is the reference's own answer.
				opts.Mode = problem.CompareExact
			}
			if same, _ := judge.Equal(opts, old, cr.Output); !same {
				res.Changed = append(res.Changed, cr.Index)
			}
		}
		cases[cr.Index-1].Expected = cr.Output
		res.Filled++
	}
	if err := p.SaveCases(cases); err != nil {
		return res, err
	}
	if len(failed) > 0 {
		return res, fmt.Errorf("reference failed on %d case(s):\n  %s", len(failed), strings.Join(failed, "\n  "))
	}
	return res, nil
}

// ProblemStatus is the progress on a problem.
type ProblemStatus int

const (
	StatusNew ProblemStatus = iota
	StatusAttempted
	StatusSolved
)

// Statuses computes the status of every problem: solved if any submission
// was accepted, attempted if a solution file or submission exists.
func (a *App) Statuses(ctx context.Context, probs []*problem.Problem) (map[string]ProblemStatus, error) {
	st, err := a.Store()
	if err != nil {
		return nil, err
	}
	subs, err := st.Statuses(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]ProblemStatus, len(probs))
	for _, p := range probs {
		s := subs[p.Ref()]
		switch {
		case s.Solved:
			out[p.Ref()] = StatusSolved
		case s.Attempts > 0 || slices.ContainsFunc(a.Langs.All(), func(l lang.Language) bool { return a.HasSolution(p, l) }):
			out[p.Ref()] = StatusAttempted
		default:
			out[p.Ref()] = StatusNew
		}
	}
	return out, nil
}
