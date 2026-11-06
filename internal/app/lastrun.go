package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/version"
)

// lastRun is what hc remembers of the latest run of a solution on its
// problem's cases, so that debugging can start at its first failure without
// running again while nothing has changed.
type lastRun struct {
	// Key identifies the code that ran and the problem it ran on (runKey).
	Key string `json:"key"`
	// Failed is the first failing case, 0 when none failed, and Verdict its
	// verdict.
	Failed  int            `json:"failed,omitempty"`
	Verdict runner.Verdict `json:"verdict,omitempty"`
	// All is whether the run covered every case.
	All bool `json:"all,omitempty"`
}

// lastRunPath is where the latest run of p's solution in l is remembered.
func (a *App) lastRunPath(p *problem.Problem, l lang.Language) string {
	return filepath.Join(a.WS.StatePath(), "cache", "lastrun", p.Pack, p.Slug, l.ID()+".json")
}

// problemKey identifies what the verdicts of a solution on p depend on
// besides its code: hc and the problem's files that judge it.
func problemKey(p *problem.Problem) string {
	h := sha256.New()
	part(h, []byte(version.String()))
	paths := []string{filepath.Join(p.Dir, problem.MetaFile), filepath.Join(p.Dir, problem.CasesFile), p.GeneratorPath(), p.CheckerPath()}
	refs, _ := filepath.Glob(filepath.Join(p.Dir, problem.ReferenceDir, "*"))
	slices.Sort(refs)
	for _, path := range append(paths, refs...) {
		data, _ := os.ReadFile(path) // a missing file counts as empty
		part(h, []byte(filepath.Base(path)))
		part(h, data)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// runKey identifies code run on p (see problemKey).
func runKey(code []byte, problemKey string) string {
	h := sha256.New()
	part(h, code)
	part(h, []byte(problemKey))
	return hex.EncodeToString(h.Sum(nil))
}

// failed is whether a case with verdict v failed: it was judged, and not
// Accepted.
func failed(v runner.Verdict) bool {
	return v != runner.Accepted && v != runner.NotRun && !v.Unjudged()
}

// firstFailure is the first case of rep that failed, nil when none did.
func firstFailure(rep *runner.Report) *runner.CaseResult {
	for i, c := range rep.Cases {
		if failed(c.Verdict) {
			return &rep.Cases[i]
		}
	}
	return nil
}

// recordRun remembers rep, a run of code on sel, best effort.
func (a *App) recordRun(p *problem.Problem, l lang.Language, code []byte, sel []runner.Selected, rep *runner.Report) {
	if rep == nil || rep.Verdict == runner.CompileError {
		return
	}
	cases, err := p.LoadCases()
	if err != nil {
		return
	}
	r := lastRun{Key: runKey(code, problemKey(p)), All: len(sel) == len(cases)}
	if c := firstFailure(rep); c != nil {
		r.Failed, r.Verdict = c.Index, c.Verdict
	}
	writeCacheFile(a.lastRunPath(p, l), r)
}

// DebugCase is the case to debug when none is given.
type DebugCase struct {
	Index int
	// Verdict is the case's verdict, "" when every case passed.
	Verdict runner.Verdict
}

// LastFailure is the case to debug according to the latest run of the
// solution's current code on the problem as it is: its first failure, or
// case 1 when it covered every case and none failed. ok is false when
// there is no such run, or it ran only some cases and they passed.
func (a *App) LastFailure(p *problem.Problem, l lang.Language) (d DebugCase, ok bool) {
	code, err := os.ReadFile(a.SolutionPath(p, l))
	if err != nil {
		return d, false
	}
	var r lastRun
	data, err := os.ReadFile(a.lastRunPath(p, l))
	if err != nil || json.Unmarshal(data, &r) != nil || r.Key != runKey(code, problemKey(p)) || r.Failed == 0 && !r.All {
		return d, false
	}
	return DebugCase{Index: max(r.Failed, 1), Verdict: r.Verdict}, true
}

// RunFailure runs the solution on every case and returns its first
// failure, case 1 when none fails. The run is remembered like any other.
func (a *App) RunFailure(ctx context.Context, p *problem.Problem, l lang.Language, o RunOptions) (DebugCase, error) {
	cases, err := p.LoadCases()
	if err != nil {
		return DebugCase{}, err
	}
	if len(cases) == 0 {
		return DebugCase{}, fmt.Errorf("%s has no test cases", p.Slug)
	}
	sel, err := Select(cases, nil, true)
	if err != nil {
		return DebugCase{}, err
	}
	rep, err := a.Run(ctx, p, l, sel, o)
	if err != nil {
		return DebugCase{}, err
	}
	if rep.Verdict == runner.CompileError {
		return DebugCase{}, &lang.CompileError{Output: rep.CompileError}
	}
	d := DebugCase{Index: 1}
	if c := firstFailure(rep); c != nil {
		d.Index, d.Verdict = c.Index, c.Verdict
	}
	return d, nil
}

// RestoreSubmission makes s's code the solution of p in l, like Restore.
// A submission ran every case, so while the problem is unchanged since, it
// is remembered as the latest run of that code (see LastFailure).
func (a *App) RestoreSubmission(p *problem.Problem, l lang.Language, s store.Submission) (string, error) {
	path, err := a.Restore(p, l, s.Code)
	if err != nil {
		return "", err
	}
	if r, ok := submissionRun(p, s); ok {
		writeCacheFile(a.lastRunPath(p, l), r)
	}
	return path, nil
}

// submissionRun is s as a run, if the problem has not changed since.
func submissionRun(p *problem.Problem, s store.Submission) (lastRun, bool) {
	var sr storedReport
	key := problemKey(p)
	if s.Details == "" || json.Unmarshal([]byte(s.Details), &sr) != nil || sr.Problem != key || sr.Verdict == string(runner.CompileError) {
		return lastRun{}, false
	}
	r := lastRun{Key: runKey([]byte(s.Code), key), All: true}
	for _, c := range sr.Cases {
		if v := runner.Verdict(c.Verdict); failed(v) {
			r.Failed, r.Verdict = c.Index, v
			break
		}
	}
	return r, true
}
