package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
)

// maxStoredValue caps inputs/outputs kept in history; large stress cases
// would otherwise bloat the database.
const maxStoredValue = 2048

type storedCase struct {
	Index     int    `json:"index"`
	Verdict   string `json:"verdict"`
	ElapsedNs int64  `json:"elapsed_ns"`
	Note      string `json:"note,omitempty"`
	// The fields below are only kept for failing cases.
	Input    string `json:"input,omitempty"`
	Output   string `json:"output,omitempty"`
	Expected string `json:"expected,omitempty"`
	Error    string `json:"error,omitempty"`
	// Message is the checker's reason for a Wrong Answer.
	Message string `json:"message,omitempty"`
}

type storedReport struct {
	Verdict   string `json:"verdict"`
	Passed    int    `json:"passed"`
	Total     int    `json:"total"`
	ElapsedNs int64  `json:"elapsed_ns"`
	// Problem identifies the problem's files the submission was judged on
	// (problemKey).
	Problem string `json:"problem,omitempty"`
	// CasesKey identifies the problem's cases file (casesKey): passed
	// cases, whose details are not kept, show them from that file while it
	// is unchanged (SubmissionCases).
	CasesKey     string       `json:"cases_key,omitempty"`
	Stopped      bool         `json:"stopped,omitempty"`
	CompileError string       `json:"compile_error,omitempty"`
	Stderr       string       `json:"stderr,omitempty"`
	Cases        []storedCase `json:"cases"`
}

// EncodeReport serializes a report for the submission history.
func EncodeReport(p *problem.Problem, rep *runner.Report) string {
	sr := storedReport{
		Verdict: string(rep.Verdict), Passed: rep.Passed, Total: rep.Total, Problem: problemKey(p), CasesKey: casesKey(p),
		ElapsedNs: rep.Elapsed.Nanoseconds(), Stopped: rep.Stopped, CompileError: rep.CompileError, Stderr: clipText(rep.Stderr),
	}
	for _, c := range rep.Cases {
		sc := storedCase{Index: c.Index, Verdict: string(c.Verdict), ElapsedNs: c.Elapsed.Nanoseconds(), Note: c.Case.Note}
		if c.Verdict != runner.Accepted && c.Verdict != runner.NotRun {
			sc.Input = clipText(p.FormatInput(c.Case))
			sc.Output = clipText(string(c.Output))
			sc.Expected = clipText(string(c.Case.Expected))
			sc.Error = clipText(c.Error)
			sc.Message = clipText(c.Message)
		}
		sr.Cases = append(sr.Cases, sc)
	}
	data, err := json.Marshal(sr)
	if err != nil {
		return ""
	}
	return string(data)
}

// DecodeReport restores a report saved by EncodeReport for display. It
// returns nil for submissions recorded without details.
func DecodeReport(s string) *runner.Report {
	if s == "" {
		return nil
	}
	var sr storedReport
	if err := json.Unmarshal([]byte(s), &sr); err != nil {
		return nil
	}
	rep := &runner.Report{
		Verdict: runner.Verdict(sr.Verdict), Passed: sr.Passed, Total: sr.Total,
		Elapsed: time.Duration(sr.ElapsedNs), Stopped: sr.Stopped, CompileError: sr.CompileError, Stderr: sr.Stderr,
	}
	for _, sc := range sr.Cases {
		cr := runner.CaseResult{
			Index: sc.Index, Verdict: runner.Verdict(sc.Verdict), Elapsed: time.Duration(sc.ElapsedNs),
			Error: sc.Error, Message: sc.Message, InputText: sc.Input, Output: json.RawMessage(sc.Output),
		}
		cr.Case.Note = sc.Note
		if sc.Expected != "" {
			// Only displayed, never re-parsed, so truncated text is fine.
			cr.Case.Expected = json.RawMessage(sc.Expected)
		}
		rep.Cases = append(rep.Cases, cr)
	}
	return rep
}

// SubmissionCases returns p's cases as they were when a submission's
// report (details) was recorded, which a passed case of it, whose details
// are not kept, shows; nil when its cases file changed since, or when the
// report predates the record of it.
func SubmissionCases(p *problem.Problem, details string) []problem.Case {
	var sr storedReport
	if json.Unmarshal([]byte(details), &sr) != nil || sr.CasesKey == "" || sr.CasesKey != casesKey(p) {
		return nil
	}
	cases, err := p.LoadCases()
	if err != nil {
		return nil
	}
	return cases
}

// casesKey identifies the content of p's cases file: case i of a report
// recorded under the same key is the file's case i.
func casesKey(p *problem.Problem) string {
	data, err := os.ReadFile(filepath.Join(p.Dir, problem.CasesFile))
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func clipText(s string) string {
	if len(s) <= maxStoredValue {
		return s
	}
	return s[:maxStoredValue] + "…"
}
