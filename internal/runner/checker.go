package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MorePeanuts/handcoding/internal/judge"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

// check judges the cases with the problem's checker in one process, which
// gets the run's time limit. A rejected case is a Wrong Answer with the
// checker's message; a broken checker fails the run with a
// *judge.CheckerError.
func check(ctx context.Context, p *problem.Problem, work string, rep *Report, cases []judge.CheckerCase, opts Options) error {
	path, name := p.CheckerPath(), opts.CheckerName
	if name == "" {
		name = path
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("judge.compare is checker, but %s is missing", name)
	}
	if opts.Checker == nil {
		return errors.New("no checker runner configured")
	}
	ids := make([]int, len(cases))
	for i, c := range cases {
		ids[i] = c.ID
	}
	dir := filepath.Join(work, "checker")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(cases)
	if err != nil {
		return err
	}
	items, out := filepath.Join(dir, "items.json"), filepath.Join(dir, "checks.jsonl")
	if err := os.WriteFile(items, data, 0o644); err != nil {
		return err
	}

	runCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	cmd, err := opts.Checker.CheckCommand(runCtx, dir, path, items, out)
	if err != nil {
		return err
	}
	output := &tailBuffer{max: 16 << 10}
	cmd.Stdin = nil
	cmd.Stdout, cmd.Stderr = output, output
	cmd.WaitDelay = time.Second
	runErr := cmd.Run()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	run := judge.CheckerRun{
		Path: path, Name: name, Output: output.String(), Limit: opts.Timeout,
		TimedOut: errors.Is(runCtx.Err(), context.DeadlineExceeded),
	}
	if runErr != nil && !run.TimedOut {
		run.Output = strings.TrimSpace(strings.TrimSpace(run.Output) + "\n" + runErr.Error())
	}
	if run.Results, err = readCheckerResults(out); err != nil {
		return err
	}
	decided, err := judge.Checked(ids, run)
	if err != nil {
		return err
	}
	for i := range rep.Cases {
		cr := &rep.Cases[i]
		r, ok := decided[cr.Index]
		switch {
		case !ok || cr.Verdict != "":
		case r.OK:
			cr.Verdict = Accepted
		default:
			cr.Verdict = WrongAnswer
			cr.Message = r.Message
		}
	}
	return nil
}

func readCheckerResults(path string) ([]judge.CheckerResult, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var results []judge.CheckerResult
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<30)
	for sc.Scan() {
		var r judge.CheckerResult
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			// A partially written last line after a crash; the case is
			// then reported as left without a result.
			continue
		}
		results = append(results, r)
	}
	return results, sc.Err()
}
