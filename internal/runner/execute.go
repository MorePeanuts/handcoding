package runner

import (
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/MorePeanuts/handcoding/internal/lang"
)

// execute keeps isolation below judging: build once, then collect the same
// driver results as a batch language. A fatal exit stops the batch; ordinary
// caught exceptions can still report a result and proceed to the next case.
func execute(ctx context.Context, l lang.Language, dir, work string, cases []Selected, opts Options, timing *Timing) (map[int]driverResult, error, bool, *tailBuffer, error) {
	batches := [][]Selected{cases}
	isolate, ok := l.(lang.CaseIsolator)
	isolated := ok && isolate.IsolatedCases()
	if isolated {
		batches = nil
		for _, c := range cases {
			batches = append(batches, []Selected{c})
		}
	}
	results := map[int]driverResult{}
	stderr := &tailBuffer{max: 16 << 10}
	var used time.Duration
	for _, batch := range batches {
		casesPath, outPath, err := WriteCases(work, batch)
		if err != nil {
			return nil, nil, false, stderr, err
		}
		if err := os.Remove(outPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, nil, false, stderr, err
		}
		runCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
		cmd := l.Command(runCtx, dir, work)
		cmd.Env = append(envOrDefault(cmd.Env), lang.EnvCases+"="+casesPath, lang.EnvOut+"="+outPath)
		if opts.TimeLimit > 0 {
			cmd.Env = append(cmd.Env, lang.EnvTimeLimit+"="+strconv.FormatInt((opts.TimeLimit-used).Nanoseconds(), 10))
		}
		cmd.Stdin = nil
		cmd.Stdout = orDiscard(opts.Stdout)
		cmd.Stderr = io.MultiWriter(orDiscard(opts.Stderr), stderr)
		cmd.WaitDelay = time.Second
		started := time.Now()
		runErr := cmd.Run()
		timing.WallElapsed += time.Since(started)
		timedOut := errors.Is(runCtx.Err(), context.DeadlineExceeded)
		cancel()
		rs, err := readResults(outPath)
		if err != nil {
			return nil, runErr, timedOut, stderr, err
		}
		// A worker's case also owns process teardown. A destructor crash or
		// hang after writing the result must not become Accepted.
		if isolated && runErr != nil {
			return results, runErr, timedOut, stderr, nil
		}
		stop := false
		for _, c := range batch {
			r, ok := rs[c.Index]
			if !ok {
				stop = true
				break
			}
			results[c.Index] = r
			used += time.Duration(r.ElapsedNs)
			if r.TimeLimit || opts.TimeLimit > 0 && used > opts.TimeLimit {
				stop = true
				break
			}
		}
		if stop || runErr != nil {
			return results, runErr, timedOut, stderr, nil
		}
	}
	return results, nil, false, stderr, nil
}
