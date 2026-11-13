package leetcode

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Session is a signed-in LeetCode session: the value of the LEETCODE_SESSION
// cookie of a browser signed in to the site. Running and submitting
// solutions need one.
type Session struct {
	Token string `json:"session"`
}

// ParseSession finds the session in text: the value of LEETCODE_SESSION in
// a Cookie header, a cURL command or anything else that names it, else text
// itself, a bare value. Its errors never show text, a secret.
func ParseSession(text string) (Session, error) {
	const name = "LEETCODE_SESSION="
	if i := strings.Index(text, name); i >= 0 {
		v := text[i+len(name):]
		if j := strings.IndexAny(v, "; \t\r\n'\"\\"); j >= 0 {
			v = v[:j]
		}
		if v == "" {
			return Session{}, errors.New("LEETCODE_SESSION is empty in what was given")
		}
		return Session{Token: v}, nil
	}
	v := strings.TrimSpace(text)
	switch {
	case v == "":
		return Session{}, errors.New("no LEETCODE_SESSION given")
	case strings.ContainsAny(v, "; \t\r\n'\""):
		return Session{}, errors.New("no LEETCODE_SESSION cookie in what was given")
	}
	return Session{Token: v}, nil
}

var (
	// ErrNotSignedIn is returned when LeetCode does not accept a session.
	ErrNotSignedIn = errors.New("leetcode: the session is not signed in")
	// ErrBlocked is returned when LeetCode refuses a request (HTTP 401 or
	// 403, or a web page instead of JSON): the session has expired, or the
	// site blocks the client.
	ErrBlocked = errors.New("leetcode: the request was refused (the session has expired, or LeetCode blocked the client)")
	// ErrRateLimited is returned for HTTP 429.
	ErrRateLimited = errors.New("leetcode: too many requests (HTTP 429); wait a little and retry")
)

// auth adds the session's cookie and a CSRF token to req, sent from the page
// at referer.
func (c *Client) auth(req *http.Request, referer string) {
	req.Header.Set("Referer", c.Site.Base+referer)
	req.Header.Set("Origin", c.Site.Base)
	if c.Session != nil {
		csrf := c.csrfToken()
		req.Header.Set("Cookie", "LEETCODE_SESSION="+c.Session.Token+"; csrftoken="+csrf)
		req.Header.Set("X-CSRFToken", csrf)
	}
}

// csrfToken is the CSRF token the client sends, as both the csrftoken
// cookie and the X-CSRFToken header. LeetCode does not tie it to the
// session (leetcode.cn does not even check it), so the client makes its
// own, of Django's form: 32 letters and digits.
func (c *Client) csrfToken() string {
	c.csrfOnce.Do(func() {
		const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		b := make([]byte, 32)
		_, _ = rand.Read(b) // never fails
		for i := range b {
			b[i] = chars[int(b[i])%len(chars)]
		}
		c.csrf = string(b)
	})
	return c.csrf
}

const userStatusQuery = `query globalData {
  userStatus { isSignedIn username }
}`

// Username returns the name of the user the client's session signs in, or
// ErrNotSignedIn.
func (c *Client) Username(ctx context.Context) (string, error) {
	var data struct {
		UserStatus struct {
			IsSignedIn bool   `json:"isSignedIn"`
			Username   string `json:"username"`
		} `json:"userStatus"`
	}
	if err := c.graphql(ctx, "globalData", userStatusQuery, map[string]any{}, "/", &data); err != nil {
		return "", err
	}
	if !data.UserStatus.IsSignedIn {
		return "", ErrNotSignedIn
	}
	return data.UserStatus.Username, nil
}

const questionMetaQuery = `query questionMeta($titleSlug: String!) {
  question(titleSlug: $titleSlug) {
    questionId questionFrontendId titleSlug isPaidOnly exampleTestcases metaData
  }
}`

// QuestionMeta fetches what running and submitting a solution need: the
// question's internal id (QuestionID, not its frontend id) and its example
// testcases.
func (c *Client) QuestionMeta(ctx context.Context, slug string) (*Question, error) {
	var data struct {
		Question *Question `json:"question"`
	}
	if err := c.graphql(ctx, "questionMeta", questionMetaQuery, map[string]any{"titleSlug": slug}, "/problems/"+slug+"/", &data); err != nil {
		return nil, err
	}
	if data.Question == nil {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, slug)
	}
	return data.Question, nil
}

// Code is a solution for LeetCode to run.
type Code struct {
	// Slug is the question's slug, and QuestionID its internal id.
	Slug       string
	QuestionID string
	// Lang is LeetCode's name for the language, e.g. python3 or golang.
	Lang string
	Code string
}

// Interpret runs code on input, testcases in LeetCode's format (one JSON
// value per line, a case's lines after the other's), and returns the id to
// pass to Wait.
func (c *Client) Interpret(ctx context.Context, code Code, input string) (string, error) {
	var resp struct {
		ID json.RawMessage `json:"interpret_id"`
	}
	body := map[string]any{
		"lang": code.Lang, "question_id": code.QuestionID, "typed_code": code.Code,
		"data_input": input, "judge_type": "large", "test_mode": false,
	}
	if err := c.post(ctx, "/problems/"+code.Slug+"/interpret_solution/", code.Slug, body, &resp); err != nil {
		return "", err
	}
	return idOf(resp.ID, "interpret_id")
}

// Submit submits code and returns the submission's id to pass to Wait.
func (c *Client) Submit(ctx context.Context, code Code) (string, error) {
	var resp struct {
		ID json.RawMessage `json:"submission_id"`
	}
	body := map[string]any{
		"lang": code.Lang, "question_id": code.QuestionID, "typed_code": code.Code, "test_mode": false,
	}
	if err := c.post(ctx, "/problems/"+code.Slug+"/submit/", code.Slug, body, &resp); err != nil {
		return "", err
	}
	return idOf(resp.ID, "submission_id")
}

// idOf reads an id LeetCode sends as a string or a number.
func idOf(raw json.RawMessage, field string) (string, error) {
	id := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if id == "" || id == "null" {
		return "", fmt.Errorf("leetcode: the reply has no %s", field)
	}
	return id, nil
}

// Result is the outcome of a run (Interpret) or a submission, as LeetCode
// reports it.
type Result struct {
	State string `json:"state"`
	// StatusCode is 10 for Accepted, 11 Wrong Answer, 12 Memory Limit
	// Exceeded, 13 Output Limit Exceeded, 14 Time Limit Exceeded, 15 Runtime
	// Error, 20 Compile Error.
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
	RunSuccess bool   `json:"run_success"`

	TotalCorrect   int `json:"total_correct"`
	TotalTestcases int `json:"total_testcases"`

	StatusRuntime     string  `json:"status_runtime"`
	StatusMemory      string  `json:"status_memory"`
	RuntimePercentile float64 `json:"runtime_percentile"`
	MemoryPercentile  float64 `json:"memory_percentile"`

	CompileError     Text `json:"compile_error"`
	FullCompileError Text `json:"full_compile_error"`
	RuntimeError     Text `json:"runtime_error"`
	FullRuntimeError Text `json:"full_runtime_error"`

	// Of a run: the outputs and LeetCode's expected answers, a case each,
	// what each case printed, and whether each matched ("1") or not ("0").
	CorrectAnswer      bool   `json:"correct_answer"`
	CodeAnswer         Lines  `json:"code_answer"`
	ExpectedCodeAnswer Lines  `json:"expected_code_answer"`
	StdOutputList      Lines  `json:"std_output_list"`
	CompareResult      string `json:"compare_result"`

	// Of a submission that failed: the case it failed on, the output, the
	// expected output and what the solution printed.
	LastTestcase   Text `json:"last_testcase"`
	CodeOutput     Text `json:"code_output"`
	ExpectedOutput Text `json:"expected_output"`
	StdOutput      Text `json:"std_output"`
}

// Accepted reports whether a submission was accepted.
func (r *Result) Accepted() bool { return r.StatusCode == 10 }

// Compile is the compile error, if any.
func (r *Result) Compile() string { return firstText(r.FullCompileError, r.CompileError) }

// Runtime is the runtime error, if any.
func (r *Result) Runtime() string { return firstText(r.FullRuntimeError, r.RuntimeError) }

func firstText(ts ...Text) string {
	for _, t := range ts {
		if s := strings.TrimSpace(string(t)); s != "" {
			return s
		}
	}
	return ""
}

// Text is a field LeetCode sends as a string, or as a list of strings
// (joined by newlines), or not at all.
type Text string

func (t *Text) UnmarshalJSON(b []byte) error {
	var l Lines
	if err := l.UnmarshalJSON(b); err != nil {
		return err
	}
	*t = Text(strings.Join(l, "\n"))
	return nil
}

// Lines is a field LeetCode sends as a list of strings, or as one string.
type Lines []string

func (l *Lines) UnmarshalJSON(b []byte) error {
	switch b = bytes.TrimSpace(b); {
	case len(b) == 0 || string(b) == "null":
		*l = nil
	case b[0] == '"':
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*l = Lines{s}
	case b[0] == '[':
		var raw []json.RawMessage
		if err := json.Unmarshal(b, &raw); err != nil {
			return err
		}
		out := make(Lines, len(raw))
		for i, r := range raw {
			var s string
			if json.Unmarshal(r, &s) != nil {
				s = string(r)
			}
			out[i] = s
		}
		*l = out
	default:
		// A number or a boolean: its text.
		*l = Lines{string(b)}
	}
	return nil
}

// Wait polls the result of a run or submission until LeetCode has judged
// it, about every PollInterval and for at most PollTimeout.
func (c *Client) Wait(ctx context.Context, slug, id string) (*Result, error) {
	interval, limit := c.PollInterval, c.PollTimeout
	if interval <= 0 {
		interval = time.Second
	}
	if limit <= 0 {
		limit = 2 * time.Minute
	}
	deadline := time.Now().Add(limit)
	for {
		var r Result
		if err := c.get(ctx, "/submissions/detail/"+id+"/check/", slug, &r); err != nil {
			return nil, err
		}
		if r.State == "SUCCESS" {
			return &r, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("leetcode: still judging after %s (state %s); see the submission on %s", limit, r.State, c.Site.Base)
		}
		t := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil, ctx.Err()
		case <-t.C:
		}
	}
}

// post sends body as JSON from the page of the question slug, and decodes
// the reply into out.
func (c *Client) post(ctx context.Context, path, slug string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Site.Base+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, slug, out)
}

func (c *Client) get(ctx context.Context, path, slug string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Site.Base+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, slug, out)
}

// do sends req with the session and decodes the JSON reply into out. A
// refusal is ErrBlocked, rate limiting ErrRateLimited, and an error LeetCode
// reports in the reply an error with its message.
func (c *Client) do(req *http.Request, slug string, out any) error {
	c.auth(req, "/problems/"+slug+"/")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return err
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return ErrRateLimited
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("%w (HTTP %d)", ErrBlocked, resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("leetcode: HTTP %d: %s", resp.StatusCode, snippet(data))
	case bytes.HasPrefix(bytes.TrimSpace(data), []byte("<")):
		// A sign-in or challenge page instead of JSON.
		return ErrBlocked
	}
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(data, &e) == nil && e.Error != "" {
		return fmt.Errorf("leetcode: %s", e.Error)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("leetcode: decode response: %w (%s)", err, snippet(data))
	}
	return nil
}
