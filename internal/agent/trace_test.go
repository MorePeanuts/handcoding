package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fake is an agent and the workspace it runs in.
type fake struct {
	ag *Agent
	ws string
}

// fakeAgent is an agent of format that prints the lines of out to standard
// output, "oops" to standard error, and exits with code.
func fakeAgent(t *testing.T, format Format, out []string, code int) fake {
	t.Helper()
	// The workspace, where the agent runs: WS in out stands for it.
	ws := t.TempDir()
	file := filepath.Join(t.TempDir(), "out")
	if err := os.WriteFile(file, []byte(strings.ReplaceAll(strings.Join(out, "\n"), "WS", ws)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "cat " + file + "; echo oops >&2; exit " + string(rune('0'+code))
	return fake{&Agent{Name: "fake", Argv: []string{"/bin/sh", "-c", script}, Format: format}, ws}
}

// ran is what a run of a fake agent gave.
type ran struct {
	res      *Result
	progress []string
	stdout   string
	log      []map[string]any
	err      error
}

// run runs f's agent in its workspace.
func run(t *testing.T, f fake) ran {
	t.Helper()
	var log, stdout, stderr bytes.Buffer
	var progress []string
	res, err := f.ag.Run(context.Background(), RunOptions{
		Dir: f.ws, Prompt: "the prompt", Log: &log,
		Header: map[string]any{"task": "gen"},
		Stdout: &stdout, Stderr: &stderr,
		Progress: func(s string) { progress = append(progress, s) },
	})
	if stderr.String() != "oops\n" {
		t.Errorf("stderr %q", stderr.String())
	}
	var lines []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(log.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("log line %q: %v", l, err)
		}
		if m["time"] == nil {
			t.Errorf("log line without a time: %q", l)
		}
		lines = append(lines, m)
	}
	return ran{res, progress, stdout.String(), lines, err}
}

func TestRunClaudeStream(t *testing.T) {
	long := strings.Repeat("x", 200<<10)
	out := []string{
		`{"type":"system","subtype":"init","session_id":"s1"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Let me look."},{"type":"tool_use","name":"Bash","input":{"command":"hc validate two-sum --run\nmore"}}]},"session_id":"s1"}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","content":"` + long + `"}]},"session_id":"s1"}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write","input":{"file_path":"/ws/cases.jsonl"}},{"type":"tool_use","name":"TaskStop","input":{"task_id":"x"}}]}}`,
		`{"type":"system","subtype":"permission_denied","tool_name":"Bash","decision_reason":"Compound command contains cd with write operation","message":"Commands that change directories need approval"}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"cd WS && sh WS/.hc/scratch/run.sh"}}]}}`,
		`not an event`,
		`{"type":"result","subtype":"success","is_error":false,"num_turns":7,"total_cost_usd":0.42,"result":"All done.","session_id":"s1"}`,
	}
	r := run(t, fakeAgent(t, ClaudeStream, out, 0))
	res, err, progress, stdout, log := r.res, r.err, r.progress, r.stdout, r.log
	if err != nil {
		t.Fatal(err)
	}
	if res.Message != "All done." || res.Turns != 7 || res.CostUSD != 0.42 || res.SessionID != "s1" || res.Duration <= 0 {
		t.Errorf("result %+v", res)
	}
	want := []string{"Bash hc validate two-sum --run …", "Write /ws/cases.jsonl", "TaskStop",
		"denied Bash: Compound command contains cd with write operation", "Bash sh .hc/scratch/run.sh"}
	if strings.Join(progress, "|") != strings.Join(want, "|") {
		t.Errorf("progress %q, want %q", progress, want)
	}
	if stdout != "not an event\n" {
		t.Errorf("stdout %q", stdout)
	}

	if len(log) != len(out)+3 {
		t.Fatalf("%d log lines, want %d", len(log), len(out)+3)
	}
	if h := log[0]; h["type"] != "start" || h["agent"] != "fake" || h["prompt"] != "the prompt" || h["task"] != "gen" {
		t.Errorf("start line %v", h)
	}
	// Lines of standard output and error interleave; each stream keeps its
	// order.
	var events []map[string]any
	var texts, stderr []any
	for _, l := range log[1 : len(log)-1] {
		switch {
		case l["type"] == "stderr":
			stderr = append(stderr, l["text"])
		case l["event"] != nil:
			events = append(events, l["event"].(map[string]any))
		default:
			texts = append(texts, l["text"])
		}
	}
	if len(events) != 7 || events[0]["subtype"] != "init" || !strings.Contains(string(mustJSON(t, events[2])), long) {
		t.Errorf("%d events logged, or the long one not whole", len(events))
	}
	if len(texts) != 1 || texts[0] != "not an event" || len(stderr) != 1 || stderr[0] != "oops" {
		t.Errorf("text lines %q, stderr %q", texts, stderr)
	}
	if end := log[len(log)-1]; end["type"] != "end" || end["exit_code"] != 0.0 || end["error"] != nil {
		t.Errorf("end line %v", end)
	}
}

func TestRunClaudeError(t *testing.T) {
	out := []string{`{"type":"result","is_error":true,"result":"Credit balance is too low"}`}
	r := run(t, fakeAgent(t, ClaudeStream, out, 1))
	res, err, log := r.res, r.err, r.log
	if err == nil || res.Message != "Credit balance is too low" {
		t.Errorf("result %+v, err %v", res, err)
	}
	if end := log[len(log)-1]; end["exit_code"] != 1.0 || end["error"] == nil {
		t.Errorf("end line %v", end)
	}
}

func TestRunCodexJSON(t *testing.T) {
	out := []string{
		`{"type":"thread.started","thread_id":"th1"}`,
		`{"type":"turn.started"}`,
		`{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"I'll run it."}}`,
		`{"type":"item.started","item":{"id":"item_1","type":"command_execution","command":"/bin/zsh -lc 'hc cases build two-sum'","status":"in_progress"}}`,
		`{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"/bin/zsh -lc 'hc cases build two-sum'","aggregated_output":"ok\n","exit_code":0}}`,
		`{"type":"item.completed","item":{"id":"item_2","type":"file_change","changes":[{"path":"cases.jsonl","kind":"update"}]}}`,
		`{"type":"item.completed","item":{"id":"item_3","type":"agent_message","text":"done"}}`,
		`{"type":"turn.completed","usage":{"input_tokens":10,"output_tokens":2}}`,
	}
	r := run(t, fakeAgent(t, CodexJSON, out, 0))
	res, err, progress, log := r.res, r.err, r.progress, r.log
	if err != nil || res.Message != "done" || res.SessionID != "th1" {
		t.Errorf("result %+v, err %v", res, err)
	}
	if want := "/bin/zsh -lc 'hc cases build two-sum'|edit cases.jsonl"; strings.Join(progress, "|") != want {
		t.Errorf("progress %q, want %q", progress, want)
	}
	if len(log) != len(out)+3 {
		t.Errorf("%d log lines, want %d", len(log), len(out)+3)
	}

	out = append(out[:2], `{"type":"turn.failed","error":{"message":"usage limit reached"}}`)
	if r := run(t, fakeAgent(t, CodexJSON, out, 0)); r.err == nil || !strings.Contains(r.err.Error(), "usage limit reached") {
		t.Errorf("failed turn: err %v", r.err)
	}
}

// A command of the user's is shown as it prints, and logged as text.
func TestRunText(t *testing.T) {
	out := []string{`working`, `{"looks":"like json"}`}
	r := run(t, fakeAgent(t, Text, out, 0))
	res, err, progress, stdout, log := r.res, r.err, r.progress, r.stdout, r.log
	if err != nil || res.Message != "" || len(progress) != 0 {
		t.Errorf("result %+v, err %v, progress %q", res, err, progress)
	}
	if stdout != "working\n{\"looks\":\"like json\"}\n" {
		t.Errorf("stdout %q", stdout)
	}
	for _, l := range log {
		if l["event"] != nil {
			t.Errorf("a text line logged as an event: %v", l)
		}
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
