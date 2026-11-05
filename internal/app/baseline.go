package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/ui"
	"github.com/MorePeanuts/handcoding/internal/version"
)

// hc's time limit (the protocol's "Time limit"): a run may take TimeFactor
// times what the reference in its language takes on the same cases, on the
// same machine, that time counted as at least BaselineFloor, below which it
// is mostly noise. The limit is so never under TimeFactor x BaselineFloor
// (50ms). Both come from measurements on laptops and CI runners (#33).
const (
	TimeFactor    = 40
	BaselineFloor = 1250 * time.Microsecond
)

// baselineRuns is how many runs of the reference a case's baseline is the
// fastest of.
const baselineRuns = 3

// timeLimit is the time limit of a run of a solution in l on sel, 0 for
// none: only a problem ready for l has one. When the reference cannot be
// timed, the run has none either, with a warning on o.Stderr.
func (a *App) timeLimit(ctx context.Context, p *problem.Problem, l lang.Language, sel []runner.Selected, o RunOptions) time.Duration {
	if a.timeFactor <= 0 || !p.ReadyFor(l.SolutionFile()) {
		return 0
	}
	b, err := a.baseline(ctx, p, l, sel, o.Stderr)
	if err != nil {
		fmt.Fprintf(orDiscard(o.Stderr), "%s the run has no time limit: %v\n", ui.Yellow.Render("warning:"), err)
		return 0
	}
	return time.Duration(a.timeFactor) * max(b, a.baselineFloor)
}

// baseline is the time the reference in l takes on sel: the sum, over the
// cases, of the fastest of baselineRuns runs. The times are kept by input
// in .hc/cache/baseline/, so only cases without enough runs there run the
// reference, best effort.
func (a *App) baseline(ctx context.Context, p *problem.Problem, l lang.Language, sel []runner.Selected, w io.Writer) (time.Duration, error) {
	path := a.baselinePath(p, l)
	key, err := a.baselineKey(p, l)
	if err != nil {
		return 0, err
	}
	c := readBaseline(path, key)
	digests := make([]string, len(sel))
	for i, s := range sel {
		if digests[i], err = caseDigest(s.Case); err != nil {
			return 0, err
		}
	}
	for round := range baselineRuns {
		var todo []runner.Selected
		var todoDigests []string
		seen := map[string]bool{}
		for i, s := range sel {
			d := digests[i]
			if c.Cases[d].Runs >= baselineRuns || seen[d] {
				continue
			}
			seen[d] = true
			// The reference is only timed: without expected outputs there
			// is nothing to judge, nor a checker to start.
			s.Case.Expected = nil
			todo, todoDigests = append(todo, s), append(todoDigests, d)
		}
		if len(todo) == 0 {
			break
		}
		if round == 0 && w != nil {
			fmt.Fprintln(w, ui.Faint.Render(fmt.Sprintf("Timing the %s reference on %d case(s)…", l.Name(), len(todo))))
		}
		rep, _, err := a.RunReference(ctx, p, l.ID(), todo, RunOptions{Stderr: w})
		if err != nil {
			return 0, err
		}
		if err := c.record(l, rep, todoDigests); err != nil {
			return 0, err
		}
	}
	writeCacheFile(path, c)
	var b time.Duration
	for _, d := range digests {
		b += time.Duration(c.Cases[d].Ns)
	}
	return b, nil
}

// recordBaseline adds the times of a run of the reference in l on sel to
// its baseline, best effort: a run made anyway, such as the one computing
// the expected outputs of generated cases, saves one.
func (a *App) recordBaseline(p *problem.Problem, l lang.Language, sel []runner.Selected, rep *runner.Report) {
	key, err := a.baselineKey(p, l)
	if err != nil {
		return
	}
	digests := make([]string, len(sel))
	for i, s := range sel {
		if digests[i], err = caseDigest(s.Case); err != nil {
			return
		}
	}
	path := a.baselinePath(p, l)
	c := readBaseline(path, key)
	if c.record(l, rep, digests) == nil {
		writeCacheFile(path, c)
	}
}

// baselinePath is the cache of the times of p's reference in l.
func (a *App) baselinePath(p *problem.Problem, l lang.Language) string {
	return filepath.Join(a.WS.StatePath(), "cache", "baseline", p.Pack, p.Slug, l.ID()+".json")
}

// baselineKey identifies what the times of p's reference in l depend on
// besides the inputs: hc, the toolchain, problem.yaml and the reference.
func (a *App) baselineKey(p *problem.Problem, l lang.Language) (string, error) {
	tc, err := l.Toolchain()
	if err != nil {
		return "", err
	}
	ref, ok := p.Reference(l.SolutionFile())
	if !ok {
		return "", fmt.Errorf("%s has no %s reference solution", p.Slug, l.Name())
	}
	h := sha256.New()
	part(h, []byte(version.String()))
	part(h, []byte(tc.Name+" "+tc.Version))
	for _, path := range []string{filepath.Join(p.Dir, problem.MetaFile), ref} {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		part(h, data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// caseDigest identifies a case's input, as its digest does a generated
// case's.
func caseDigest(c problem.Case) (string, error) {
	if c.Generated() && c.Digest != "" {
		return c.Digest, nil
	}
	return problem.InputDigest(c.Input)
}

type baselineCache struct {
	Key string `json:"key"`
	// Cases are the times of the reference by the digest of the input.
	Cases map[string]baselineTime `json:"cases"`
}

type baselineTime struct {
	// Ns is the fastest of Runs runs.
	Ns   int64 `json:"ns"`
	Runs int   `json:"runs"`
}

// readBaseline reads the baseline cache at path, empty unless it is for key.
func readBaseline(path, key string) *baselineCache {
	c := &baselineCache{}
	if data, err := os.ReadFile(path); err == nil && json.Unmarshal(data, c) == nil && c.Key == key && c.Cases != nil {
		return c
	}
	return &baselineCache{Key: key, Cases: map[string]baselineTime{}}
}

// record adds the times of a reference run rep, whose cases have the input
// digests digests. A reference that fails on a case cannot be timed on it.
func (c *baselineCache) record(l lang.Language, rep *runner.Report, digests []string) error {
	if rep.Verdict == runner.CompileError {
		return fmt.Errorf("the %s reference solution does not compile", l.Name())
	}
	if len(rep.Cases) != len(digests) {
		return fmt.Errorf("the %s reference solution ran %d case(s) of %d", l.Name(), len(rep.Cases), len(digests))
	}
	for i, cr := range rep.Cases {
		switch cr.Verdict {
		case runner.RuntimeError, runner.TimeLimit, runner.NotRun:
			msg := fmt.Sprintf("the %s reference solution fails on case %d: %s", l.Name(), cr.Index, cr.Verdict)
			if cr.Error != "" {
				msg += ": " + cr.Error
			}
			return fmt.Errorf("%s", msg)
		}
		t, ns := c.Cases[digests[i]], cr.Elapsed.Nanoseconds()
		if t.Runs == 0 || ns < t.Ns {
			t.Ns = ns
		}
		t.Runs++
		c.Cases[digests[i]] = t
	}
	return nil
}

func orDiscard(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}
