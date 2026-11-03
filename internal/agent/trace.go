package agent

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"time"
)

// Format is how an agent reports its work on standard output.
type Format int

const (
	// Text is the output of any other command, shown and logged as it is.
	Text Format = iota
	// ClaudeStream is Claude Code's --output-format stream-json: one JSON
	// event per line, the last a result.
	ClaudeStream
	// CodexJSON is codex exec --json: one JSON event per line.
	CodexJSON
)

// Result is what a run reports at its end.
type Result struct {
	// Message is the agent's final message (empty for Text, whose output
	// was shown as it came).
	Message string
	// SessionID is the agent's own id of the session, to find its records.
	SessionID string
	// Turns and CostUSD are reported by Claude Code only.
	Turns    int
	CostUSD  float64
	Duration time.Duration
}

// trace writes the log of a run: JSON lines, the first ("start") and the
// last ("end") hc's, and in between one per line the agent wrote, with the
// time hc read it ("stdout" with the agent's JSON event, or its text;
// "stderr" with text). Writes are best effort: a log that fails does not
// stop the agent.
type trace struct {
	mu sync.Mutex
	w  io.Writer
}

func (t *trace) write(v map[string]any) {
	if t.w == nil {
		return
	}
	v["time"] = time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00")
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	_, _ = t.w.Write(append(data, '\n'))
}

// line logs a line the agent wrote to stream.
func (t *trace) line(stream string, line []byte, asJSON bool) {
	if asJSON && json.Valid(line) {
		t.write(map[string]any{"type": stream, "event": json.RawMessage(line)})
		return
	}
	t.write(map[string]any{"type": stream, "text": string(line)})
}

// readLines calls fn with each line of r, without its line break, however
// long: an event can hold a whole file.
func readLines(r io.Reader, fn func([]byte)) {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			fn([]byte(strings.TrimRight(string(line), "\r\n")))
		}
		if err != nil {
			return
		}
	}
}

// display shows a run on the terminal from the agent's standard output:
// the tools it calls as progress, and other lines as they are; it keeps
// what the run reports at its end.
type display struct {
	format Format
	// dir is the workspace, which progress lines leave out of paths.
	dir      string
	stdout   io.Writer
	progress func(string)
	res      Result
	// err is a failure the agent reported.
	err error
}

func (d *display) line(line []byte) {
	ok := false
	switch d.format {
	case ClaudeStream:
		ok = d.claude(line)
	case CodexJSON:
		ok = d.codex(line)
	}
	if !ok && d.stdout != nil {
		_, _ = d.stdout.Write(append(line, '\n'))
	}
}

func (d *display) show(s string) {
	if d.progress == nil || s == "" {
		return
	}
	if d.dir != "" {
		for _, cd := range []string{"cd " + d.dir + " && ", "cd " + d.dir + "; "} {
			s = strings.ReplaceAll(s, cd, "")
		}
		s = strings.ReplaceAll(s, d.dir+"/", "")
	}
	d.progress(oneLine(s))
}

// claude reads an event of Claude Code's stream-json, reporting whether it
// was one.
func (d *display) claude(line []byte) bool {
	var e struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
		// Message is the assistant's message, or another event's text.
		Message   json.RawMessage `json:"message"`
		Result    string          `json:"result"`
		IsError   bool            `json:"is_error"`
		NumTurns  int             `json:"num_turns"`
		Cost      float64         `json:"total_cost_usd"`
		SessionID string          `json:"session_id"`
		ToolName  string          `json:"tool_name"`
		Reason    string          `json:"decision_reason"`
	}
	if json.Unmarshal(line, &e) != nil || e.Type == "" {
		return false
	}
	if e.SessionID != "" {
		d.res.SessionID = e.SessionID
	}
	switch e.Type {
	case "assistant":
		var m struct {
			Content []struct {
				Type  string         `json:"type"`
				Name  string         `json:"name"`
				Input map[string]any `json:"input"`
			} `json:"content"`
		}
		_ = json.Unmarshal(e.Message, &m)
		for _, c := range m.Content {
			if c.Type == "tool_use" {
				d.show(toolCall(c.Name, c.Input))
			}
		}
	case "system":
		if e.Subtype == "permission_denied" {
			d.show("denied " + e.ToolName + ": " + e.Reason)
		}
	case "result":
		d.res.Message, d.res.Turns, d.res.CostUSD = e.Result, e.NumTurns, e.Cost
		if e.IsError {
			d.err = errors.New("the agent stopped with an error")
		}
	}
	return true
}

// codex reads an event of codex exec --json, reporting whether it was one.
func (d *display) codex(line []byte) bool {
	var e struct {
		Type     string `json:"type"`
		ThreadID string `json:"thread_id"`
		Message  string `json:"message"`
		Error    struct {
			Message string `json:"message"`
		} `json:"error"`
		Item struct {
			Type    string `json:"type"`
			Text    string `json:"text"`
			Command string `json:"command"`
			Changes []struct {
				Path string `json:"path"`
			} `json:"changes"`
		} `json:"item"`
	}
	if json.Unmarshal(line, &e) != nil || e.Type == "" {
		return false
	}
	switch e.Type {
	case "thread.started":
		d.res.SessionID = e.ThreadID
	case "item.started":
		if e.Item.Type == "command_execution" {
			d.show(e.Item.Command)
		}
	case "item.completed":
		switch e.Item.Type {
		case "agent_message":
			d.res.Message = e.Item.Text
		case "file_change":
			for _, c := range e.Item.Changes {
				d.show("edit " + c.Path)
			}
		}
	case "turn.failed":
		d.err = errors.New("the agent failed: " + e.Error.Message)
	case "error":
		d.err = errors.New("the agent failed: " + e.Message)
	}
	return true
}

// toolCall sums up a call of one of Claude Code's tools in a line.
func toolCall(name string, input map[string]any) string {
	for _, key := range []string{"command", "file_path", "pattern", "query", "url"} {
		if v, ok := input[key].(string); ok && v != "" {
			return name + " " + v
		}
	}
	return name
}

// oneLine is the first line of s, clipped.
func oneLine(s string) string {
	s, _, cut := strings.Cut(strings.TrimSpace(s), "\n")
	const max = 120
	if len(s) > max {
		s, cut = s[:max], true
	}
	if cut {
		s += " …"
	}
	return s
}
