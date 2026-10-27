package judge

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// CheckerCase is what a problem's checker receives for one case: the case's
// input object, the solution's output and the reference's expected output.
type CheckerCase struct {
	ID       int             `json:"id"`
	Input    json.RawMessage `json:"input"`
	Output   json.RawMessage `json:"output"`
	Expected json.RawMessage `json:"expected"`
}

// CheckerResult is the line the checker helper writes for a case: whether
// check accepted the output and why not, or Error when check raised or
// returned something else than None, a bool or a string. ID 0 reports a
// checker that cannot be loaded.
type CheckerResult struct {
	ID      int    `json:"id"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// CheckerError reports a broken checker. It is an error of the problem, not
// a verdict on the solution.
type CheckerError struct {
	Path string // the checker's path, as shown to the user
	// Case is the case the checker failed on, or 0 when it failed to load.
	Case   int
	Reason string
	// Detail is the traceback or the checker's output.
	Detail string
}

func (e *CheckerError) Error() string {
	where := "cannot be loaded"
	if e.Case > 0 {
		where = fmt.Sprintf("failed on case %d", e.Case)
	}
	msg := fmt.Sprintf("%s %s", e.Path, where)
	if e.Reason != "" {
		msg += ": " + e.Reason
	}
	msg += " (the problem's checker is broken; this is not a verdict on your solution)"
	if d := strings.TrimSpace(e.Detail); d != "" {
		msg += "\n" + d
	}
	return msg
}

// CheckerRun is the outcome of one checker process.
type CheckerRun struct {
	Path string
	// Name is how errors refer to the checker, e.g. its path relative to the
	// workspace; Path if empty. It also replaces Path in tracebacks.
	Name    string
	Results []CheckerResult
	// Output is the tail of what the process printed.
	Output   string
	TimedOut bool
	Limit    time.Duration
}

// Checked decides the cases ids from a checker run: for each case, whether
// the output is accepted and the checker's message. It fails with a
// *CheckerError for the first case the checker raised on or left without a
// result, and when the checker could not be loaded.
func Checked(ids []int, run CheckerRun) (map[int]CheckerResult, error) {
	results := make(map[int]CheckerResult, len(run.Results))
	for _, r := range run.Results {
		if r.ID == 0 {
			return nil, run.fail(0, "", r.Error)
		}
		results[r.ID] = r
	}
	for _, id := range ids {
		r, ok := results[id]
		switch {
		case ok && r.Error != "":
			return nil, run.fail(id, "", r.Error)
		case !ok && run.TimedOut:
			return nil, run.fail(id, fmt.Sprintf("time limit of %s exceeded", run.Limit), run.Output)
		case !ok:
			return nil, run.fail(id, "exited without a result", run.Output)
		}
	}
	return results, nil
}

func (run CheckerRun) fail(id int, reason, detail string) *CheckerError {
	name := run.Name
	if name == "" {
		name = run.Path
	} else if run.Path != "" {
		detail = strings.ReplaceAll(detail, run.Path, name)
	}
	return &CheckerError{Path: name, Case: id, Reason: reason, Detail: detail}
}
