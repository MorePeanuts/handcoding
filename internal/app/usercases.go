package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/version"
)

// The user's own cases (issue #159) are the cases a user keeps for a problem
// of an installed pack, which is read-only, in cases/<pack>/<slug>.jsonl of
// the workspace (workspace.UserCasesPath): lines of cases.jsonl with an
// input, and optionally an expected output and a note. They follow the
// pack's cases, numbered after them (Cases). hc test runs them by default
// and hc submit never does, so that a submission's verdict means the pack's
// cases, the same for everyone. An expected output the user did not give
// comes from a reference solution when the case runs (expectUser), so that
// it follows the pack's updates.

// KeepsUserCases reports whether the user's own cases of p are kept apart
// from its cases: p's pack is installed, so read-only. The cases of a
// writable pack are the user's already.
func (a *App) KeepsUserCases(p *problem.Problem) bool { return a.WS.CheckWritable(p) != nil }

// UserCases reads the user's own cases of p, marked User; none for a
// problem of a writable pack.
func (a *App) UserCases(p *problem.Problem) ([]problem.Case, error) {
	if !a.KeepsUserCases(p) {
		return nil, nil
	}
	path := a.WS.UserCasesPath(p)
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	name := a.WS.Rel(path)
	cases, err := problem.ParseCases(f, name)
	if err != nil {
		return nil, err
	}
	for i := range cases {
		if err := checkUserCase(cases[i]); err != nil {
			return nil, fmt.Errorf("%s: case %d of the file: %w", name, i+1, err)
		}
		cases[i].User = true
	}
	return cases, nil
}

// checkUserCase checks what only the user's own cases forbid: they are
// stored inputs, neither generated nor examples of the statement.
func checkUserCase(c problem.Case) error {
	switch {
	case c.Generated():
		return errors.New("your own cases cannot be generated: give an input (hc test --generate runs a generated input once)")
	case c.Example:
		return errors.New("your own cases cannot be examples: hc test runs them anyway")
	}
	return nil
}

// Cases are p's cases followed by the user's own (see UserCases): the cases
// hc test, hc debug and hc cases list number. hc submit judges p's alone.
func (a *App) Cases(p *problem.Problem) ([]problem.Case, error) {
	cases, err := p.LoadCases()
	if err != nil {
		return nil, err
	}
	user, err := a.UserCases(p)
	if err != nil {
		return nil, err
	}
	return append(cases, user...), nil
}

// CheckUserCases checks the user's own cases of p against the problem, as
// adding them would, after the user edited their file.
func (a *App) CheckUserCases(p *problem.Problem) error {
	cases, err := p.LoadCases()
	if err != nil {
		return err
	}
	user, err := a.UserCases(p)
	if err != nil {
		return err
	}
	var errs []error
	for i, c := range user {
		if err := p.ValidateCase(c); err != nil {
			errs = append(errs, fmt.Errorf("case %d: %w", len(cases)+i+1, err))
		}
	}
	return errors.Join(errs...)
}

// Added is what AddCase did.
type Added struct {
	// Index is the case's number (see Cases), and User whether it is one
	// of the user's own cases.
	Index int
	User  bool
	// Build is what computing the missing expected outputs of a writable
	// problem's cases did (see BuildCases), nil when it did not run.
	Build *BuildResult
	// Expected is the expected output that a reference solution, in Ref's
	// language, computed for one of the user's own cases; nil when none
	// did (see expectUser).
	Expected json.RawMessage
	Ref      lang.Language
}

// AddCase appends c to p's cases, or to the user's own cases of p when its
// pack is read-only (see KeepsUserCases). In a writable pack, the
// reference solution then computes the missing expected outputs, as hc
// cases build does. One of the user's own cases keeps the expected output
// it has; without one, a reference solution computes it now, to check that
// the input is one the problem can judge, unless p has no reference (or no
// judge rule yet): the case then shows the solution's output unjudged. w
// receives what setting up and running the reference prints.
func (a *App) AddCase(ctx context.Context, p *problem.Problem, c problem.Case, w io.Writer) (*Added, error) {
	if a.KeepsUserCases(p) {
		return a.addUserCase(ctx, p, c, w)
	}
	if err := p.ValidateCase(c); err != nil {
		return nil, err
	}
	cases, err := p.LoadCases()
	if err != nil {
		return nil, err
	}
	cases = append(cases, c)
	if err := p.SaveCases(cases); err != nil {
		return nil, err
	}
	added := &Added{Index: len(cases)}
	if c.Expected == nil {
		if _, _, err := a.ReferenceLang(p, ""); err == nil {
			added.Build, err = a.BuildCases(ctx, p, "", false)
			return added, err
		}
	}
	return added, nil
}

func (a *App) addUserCase(ctx context.Context, p *problem.Problem, c problem.Case, w io.Writer) (*Added, error) {
	if err := checkUserCase(c); err != nil {
		return nil, err
	}
	if err := p.ValidateCase(c); err != nil {
		return nil, err
	}
	if !c.HasExpected() && p.Judge.Compare == problem.CompareAnyOf {
		return nil, errors.New("give the expected output: this problem is judged with any_of, whose expected output lists every acceptable answer, and a reference solution finds only one")
	}
	cases, err := a.Cases(p)
	if err != nil {
		return nil, err
	}
	c.User = true
	added := &Added{Index: len(cases) + 1, User: true}
	if !c.HasExpected() {
		sel := []runner.Selected{{Index: added.Index, Case: c}}
		ref, failure, err := a.expectUser(ctx, p, nil, sel, w)
		if err != nil {
			return nil, err
		}
		if failure != nil {
			return nil, fmt.Errorf("the reference solution fails on this input (%s), which may break the problem's constraints; give its expected output to keep it anyway", failure)
		}
		if sel[0].Case.HasExpected() {
			added.Expected, added.Ref = sel[0].Case.Expected, ref
		}
	}
	line, err := problem.MarshalCase(c)
	if err != nil {
		return nil, err
	}
	return added, appendLine(a.WS.UserCasesPath(p), line)
}

// appendLine appends line to the file at path, creating it and its
// directory if needed, on a line of its own.
func appendLine(path string, line []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	old, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(old) > 0 && !bytes.HasSuffix(old, []byte("\n")) {
		old = append(old, '\n')
	}
	return replaceFile(path, append(append(old, line...), '\n'))
}

// replaceFile replaces the file at path with data at once.
func replaceFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Removed cases: what RemoveCases did.
type RemovedCases struct {
	// Removed is the number of cases removed, and Left the number left:
	// of p's cases, or of the user's own (User).
	Removed, Left int
	User          bool
}

// RemoveCases removes the cases numbered nums (as Cases numbers them):
// from p's cases.jsonl when its pack is writable, else from the user's own
// cases, since the pack's are read-only.
func (a *App) RemoveCases(p *problem.Problem, nums []int) (RemovedCases, error) {
	var r RemovedCases
	cases, err := p.LoadCases()
	if err != nil {
		return r, err
	}
	user, err := a.UserCases(p)
	if err != nil {
		return r, err
	}
	r.User = a.KeepsUserCases(p)
	first := 1 // the first number nums may name
	if r.User {
		first = len(cases) + 1
	}
	last := len(cases) + len(user)
	drop := map[int]bool{}
	for _, n := range nums {
		switch {
		case n < 1 || n > last:
			return r, fmt.Errorf("case %d out of range (1-%d)", n, last)
		case n < first && len(user) == 0:
			return r, fmt.Errorf("case %d is one of the pack's cases, which are read-only, and %s has none of yours", n, p.Slug)
		case n < first:
			return r, fmt.Errorf("case %d is one of the pack's cases, which are read-only: yours are %s", n, numberRange(first, last))
		}
		drop[n] = true
	}
	r.Removed = len(drop)
	if !r.User {
		var kept []problem.Case
		for i, c := range cases {
			if !drop[i+1] {
				kept = append(kept, c)
			}
		}
		r.Left = len(kept)
		return r, p.SaveCases(kept)
	}
	var buf bytes.Buffer
	for i, c := range user {
		if drop[first+i] {
			continue
		}
		line, err := problem.MarshalCase(c)
		if err != nil {
			return r, err
		}
		buf.Write(line)
		buf.WriteByte('\n')
		r.Left++
	}
	path := a.WS.UserCasesPath(p)
	if r.Left == 0 {
		return r, os.Remove(path)
	}
	return r, replaceFile(path, buf.Bytes())
}

// numberRange is "11" or "11-14".
func numberRange(first, last int) string {
	if first == last {
		return fmt.Sprint(first)
	}
	return fmt.Sprintf("%d-%d", first, last)
}

// userCasesKey identifies the user's own cases of p, for the key of a run
// on them (see runKey): their file, "" without one.
func (a *App) userCasesKey(p *problem.Problem) string {
	if !a.KeepsUserCases(p) {
		return ""
	}
	data, err := os.ReadFile(a.WS.UserCasesPath(p))
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// expectUser sets the expected outputs of the user's own cases in sel that
// have none to the outputs of a reference solution: the one in l when p
// has one (nil: any), whose language it returns. They are kept in the
// cache (.hc/cache/cases/<pack>/<slug>/<lang>.user.json) by input, keyed
// by hc's version, problem.yaml and the reference. Nothing happens when p
// has no reference, or a judge rule a reference cannot give expected
// outputs for (not decided yet, or any_of): those cases run unjudged. The
// first case the reference fails on is returned, as its verdict and error.
func (a *App) expectUser(ctx context.Context, p *problem.Problem, l lang.Language, sel []runner.Selected, w io.Writer) (lang.Language, *ReferenceFailure, error) {
	release, lockErr := a.WS.UseCaches()
	if lockErr != nil {
		return nil, nil, lockErr
	}
	defer release()
	var todo []int
	for i, s := range sel {
		if s.Case.User && !s.Case.HasExpected() {
			todo = append(todo, i)
		}
	}
	if len(todo) == 0 || !p.Ready() || p.Judge.Compare == problem.CompareAnyOf || len(a.ReferenceLangs(p)) == 0 {
		return nil, nil, nil
	}
	ref, err := a.generatedRef(p, l)
	if err != nil {
		return nil, nil, err
	}
	key, err := a.userCacheKey(p, ref)
	if err != nil {
		return nil, nil, err
	}
	path := filepath.Join(a.WS.StatePath(), "cache", "cases", p.Pack, p.Slug, ref.ID()+".user.json")
	cache := readUserCache(path, key)
	var run []runner.Selected
	var pos []int
	var digests []string
	for _, i := range todo {
		d, err := problem.InputDigest(sel[i].Case.Input)
		if err != nil {
			return nil, nil, err
		}
		if exp, ok := cache[d]; ok {
			sel[i].Case.Expected = exp
			continue
		}
		run, pos, digests = append(run, sel[i]), append(pos, i), append(digests, d)
	}
	if len(run) == 0 {
		return ref, nil, nil
	}
	if w == nil {
		w = io.Discard
	}
	rep, _, err := a.RunReference(ctx, p, ref.ID(), run, RunOptions{Stderr: w})
	if err != nil {
		return nil, nil, err
	}
	file, _ := p.Reference(ref.SolutionFile())
	if rep.Verdict == runner.CompileError {
		return nil, nil, fmt.Errorf("the reference solution %s does not compile:\n%s", a.WS.Rel(file), rep.CompileError)
	}
	for j, cr := range rep.Cases {
		if cr.Verdict != runner.NoExpected {
			return ref, &ReferenceFailure{Index: cr.Index, Path: a.WS.Rel(file), Verdict: cr.Verdict, Error: cr.Error}, nil
		}
		sel[pos[j]].Case.Expected = cr.Output
		cache[digests[j]] = cr.Output
	}
	writeCacheFile(path, userCache{Key: key, Expected: cache})
	// The run also times the reference on these inputs.
	a.recordBaseline(p, ref, run, rep)
	return ref, nil, nil
}

// ReferenceFailure is a case of the user's own that the reference solution
// at Path fails on: the case cannot be judged.
type ReferenceFailure struct {
	Index   int
	Path    string
	Verdict runner.Verdict
	Error   string
}

func (f *ReferenceFailure) String() string {
	if f.Error == "" {
		return string(f.Verdict)
	}
	return fmt.Sprintf("%s: %s", f.Verdict, f.Error)
}

// userCacheKey identifies what computes the expected outputs of the user's
// own cases with the reference in l: hc, problem.yaml and the reference.
func (a *App) userCacheKey(p *problem.Problem, l lang.Language) (string, error) {
	h := sha256.New()
	part(h, []byte(version.String()))
	ref, _ := p.Reference(l.SolutionFile())
	for _, path := range []string{filepath.Join(p.Dir, problem.MetaFile), ref} {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		part(h, data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type userCache struct {
	Key string `json:"key"`
	// Expected are the expected outputs by the digest of their input
	// (problem.InputDigest).
	Expected map[string]json.RawMessage `json:"expected"`
}

// readUserCache reads the expected outputs of the cache file at path, none
// unless it holds them for key.
func readUserCache(path, key string) map[string]json.RawMessage {
	var c userCache
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &c) != nil || c.Key != key || c.Expected == nil {
		return map[string]json.RawMessage{}
	}
	return c.Expected
}
