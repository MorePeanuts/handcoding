package app

import (
	"encoding/json"
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
	Verdict      string       `json:"verdict"`
	Passed       int          `json:"passed"`
	Total        int          `json:"total"`
	ElapsedNs    int64        `json:"elapsed_ns"`
	CompileError string       `json:"compile_error,omitempty"`
	Stderr       string       `json:"stderr,omitempty"`
	Cases        []storedCase `json:"cases"`
}

// EncodeReport serializes a report for the submission history.
func EncodeReport(p *problem.Problem, rep *runner.Report) string {
	sr := storedReport{
		Verdict: string(rep.Verdict), Passed: rep.Passed, Total: rep.Total,
		ElapsedNs: rep.Elapsed.Nanoseconds(), CompileError: rep.CompileError, Stderr: clipText(rep.Stderr),
	}
	for _, c := range rep.Cases {
		sc := storedCase{Index: c.Index, Verdict: string(c.Verdict), ElapsedNs: c.Elapsed.Nanoseconds(), Note: c.Case.Note}
		if c.Verdict != runner.Accepted {
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
		Elapsed: time.Duration(sr.ElapsedNs), CompileError: sr.CompileError, Stderr: sr.Stderr,
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

func clipText(s string) string {
	if len(s) <= maxStoredValue {
		return s
	}
	return s[:maxStoredValue] + "…"
}
