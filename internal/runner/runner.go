// Package runner executes a solution against test cases and judges the
// results.
package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/MorePeanuts/handcoding/internal/judge"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

// Verdict is the outcome of a case or a whole run.
type Verdict string

const (
	Accepted     Verdict = "Accepted"
	WrongAnswer  Verdict = "Wrong Answer"
	RuntimeError Verdict = "Runtime Error"
	TimeLimit    Verdict = "Time Limit Exceeded"
	CompileError Verdict = "Compile Error"
	// NoExpected means the case ran but has no expected output to compare to.
	NoExpected Verdict = "No Expected Output"
	// NotJudged means the case ran, but the problem's judge rule is not
	// decided yet (problem.Ready), so the output is shown without a verdict.
	NotJudged Verdict = "Not Judged"
)

// Selected is a case chosen for a run, with its 1-based index in cases.jsonl.
type Selected struct {
	Index int
	Case  problem.Case
}

// Unjudged reports whether a case ran without being judged: it has no
// expected output, or the judge rule is not decided.
func (v Verdict) Unjudged() bool { return v == NoExpected || v == NotJudged }

// CaseResult is the result of one case.
type CaseResult struct {
	Index    int
	Case     problem.Case
	Verdict  Verdict
	Output   json.RawMessage
	Error    string
	Elapsed  time.Duration
	JudgeErr error
	// Message is the checker's reason for rejecting the output (judge.compare:
	// checker).
	Message string
	// InputText overrides the rendered input; set for results restored from
	// history, where the input may be truncated.
	InputText string
}

// Report is the result of a run.
type Report struct {
	Verdict      Verdict
	Cases        []CaseResult
	Passed       int
	Total        int
	CompileError string
	// Stderr holds the tail of the program's stderr if it crashed.
	Stderr  string
	Elapsed time.Duration
	// Toolchain is what the solution ran with, set by the caller; the zero
	// value when unknown (e.g. a report decoded from history).
	Toolchain lang.Toolchain
}

// Options configure a run.
type Options struct {
	Timeout time.Duration
	// Stdout receives the program's standard output (user prints).
	Stdout io.Writer
	// Stderr receives the program's standard error.
	Stderr io.Writer
	// Checker runs checker.py; required for problems judged by one.
	Checker lang.Checker
}

// Run prepares, builds and runs the solution in dir against the cases.
func Run(ctx context.Context, l lang.Language, p *problem.Problem, dir string, cases []Selected, opts Options) (*Report, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Second
	}
	if err := l.Prepare(dir, p); err != nil {
		return nil, fmt.Errorf("prepare: %w", err)
	}
	work, err := os.MkdirTemp("", "hc-run-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)

	rep := &Report{Total: len(cases)}
	if err := l.Build(ctx, dir, work); err != nil {
		if ce, ok := errors.AsType[*lang.CompileError](err); ok {
			rep.Verdict = CompileError
			rep.CompileError = ce.Output
			return rep, nil
		}
		return nil, fmt.Errorf("build: %w", err)
	}
	casesPath, outPath, err := WriteCases(work, cases)
	if err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	cmd := l.Command(runCtx, dir, work)
	cmd.Env = append(envOrDefault(cmd.Env), lang.EnvCases+"="+casesPath, lang.EnvOut+"="+outPath)
	cmd.Stdin = nil
	cmd.Stdout = orDiscard(opts.Stdout)
	stderr := &tailBuffer{max: 16 << 10}
	cmd.Stderr = io.MultiWriter(orDiscard(opts.Stderr), stderr)
	cmd.WaitDelay = time.Second
	start := time.Now()
	runErr := cmd.Run()
	rep.Elapsed = time.Since(start)
	timedOut := errors.Is(runCtx.Err(), context.DeadlineExceeded)

	results, err := readResults(outPath)
	if err != nil {
		return nil, err
	}
	var toCheck []judge.CheckerCase
	for _, sel := range cases {
		cr := CaseResult{Index: sel.Index, Case: sel.Case}
		res, ok := results[sel.Index]
		switch {
		case !ok && timedOut:
			cr.Verdict = TimeLimit
		case !ok:
			cr.Verdict = RuntimeError
			cr.Error = "program exited before finishing this case"
			if runErr != nil {
				cr.Error += ": " + runErr.Error()
			}
		case res.Error != "":
			cr.Verdict = RuntimeError
			cr.Error = res.Error
			cr.Elapsed = time.Duration(res.ElapsedNs)
		default:
			cr.Output = res.Output
			cr.Elapsed = time.Duration(res.ElapsedNs)
			cr.Verdict = judgeCase(p, sel.Case, res.Output, &cr)
			if cr.Verdict == "" {
				input, err := json.Marshal(sel.Case.Input)
				if err != nil {
					return nil, err
				}
				toCheck = append(toCheck, judge.CheckerCase{ID: sel.Index, Input: input, Output: res.Output, Expected: sel.Case.Expected})
			}
		}
		rep.Cases = append(rep.Cases, cr)
	}
	if len(toCheck) > 0 {
		if err := check(ctx, p, work, rep, toCheck, opts); err != nil {
			return nil, err
		}
	}
	for _, cr := range rep.Cases {
		if cr.Verdict == Accepted {
			rep.Passed++
		}
	}
	if runErr != nil && len(results) < len(cases) {
		rep.Stderr = stderr.String()
	}
	rep.Verdict = overall(rep)
	return rep, nil
}

// judgeCase returns the case's verdict, or "" when the problem's checker
// has to decide it.
func judgeCase(p *problem.Problem, c problem.Case, out json.RawMessage, cr *CaseResult) Verdict {
	if !c.HasExpected() {
		return NoExpected
	}
	if !p.Ready() {
		return NotJudged
	}
	if p.Judge.Compare == problem.CompareChecker {
		return ""
	}
	ok, err := judge.Equal(judge.OptionsFor(p), c.Expected, out)
	if err != nil {
		cr.JudgeErr = err
		return WrongAnswer
	}
	if ok {
		return Accepted
	}
	return WrongAnswer
}

// overall picks the first failing verdict in case order, as LeetCode does.
// Without any failure, a run in which no case was judged is NotJudged or
// NoExpected rather than Accepted.
func overall(rep *Report) Verdict {
	judged, notJudged := false, false
	for _, c := range rep.Cases {
		switch c.Verdict {
		case Accepted:
			judged = true
		case NotJudged:
			notJudged = true
		case NoExpected:
		default:
			return c.Verdict
		}
	}
	switch {
	case judged || len(rep.Cases) == 0:
		return Accepted
	case notJudged:
		return NotJudged
	}
	return NoExpected
}

type driverCase struct {
	ID    int                        `json:"id"`
	Input map[string]json.RawMessage `json:"input"`
}

type driverResult struct {
	ID        int             `json:"id"`
	Output    json.RawMessage `json:"output"`
	Error     string          `json:"error"`
	ElapsedNs int64           `json:"elapsed_ns"`
}

// WriteCases writes the driver input file into dir and returns the paths of
// the cases file and the (not yet existing) output file.
func WriteCases(dir string, cases []Selected) (casesPath, outPath string, err error) {
	dcs := make([]driverCase, len(cases))
	for i, s := range cases {
		dcs[i] = driverCase{ID: s.Index, Input: s.Case.Input}
	}
	data, err := json.Marshal(dcs)
	if err != nil {
		return "", "", err
	}
	casesPath = filepath.Join(dir, "cases.json")
	outPath = filepath.Join(dir, "results.jsonl")
	return casesPath, outPath, os.WriteFile(casesPath, data, 0o644)
}

func readResults(path string) (map[int]driverResult, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[int]driverResult{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	results := map[int]driverResult{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<30)
	for sc.Scan() {
		var r driverResult
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			// A partially written last line after a crash; ignore it.
			continue
		}
		// Drivers format JSON their own way (Python puts spaces after
		// commas); compact it so outputs read like the expected ones.
		var b bytes.Buffer
		if len(r.Output) > 0 && json.Compact(&b, r.Output) == nil {
			r.Output = b.Bytes()
		}
		results[r.ID] = r
	}
	return results, sc.Err()
}

func envOrDefault(env []string) []string {
	if env == nil {
		return os.Environ()
	}
	return env
}

func orDiscard(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}

// DebugCommand builds a debugger command for the given cases. The returned
// cleanup removes temporary files.
func DebugCommand(l lang.Language, p *problem.Problem, dir string, cases []Selected, opts lang.DebugOptions) (*exec.Cmd, func(), error) {
	if err := l.Prepare(dir, p); err != nil {
		return nil, nil, err
	}
	work, err := os.MkdirTemp("", "hc-debug-*")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { os.RemoveAll(work) }
	casesPath, outPath, err := WriteCases(work, cases)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	cmd, err := l.DebugCommand(dir, work, p, opts)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	cmd.Env = append(envOrDefault(cmd.Env), lang.EnvCases+"="+casesPath, lang.EnvOut+"="+outPath)
	return cmd, cleanup, nil
}

// tailBuffer keeps the last max bytes written to it.
type tailBuffer struct {
	buf bytes.Buffer
	max int
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.buf.Write(p)
	if over := t.buf.Len() - t.max; over > 0 {
		t.buf.Next(over)
	}
	return len(p), nil
}

func (t *tailBuffer) String() string { return t.buf.String() }
