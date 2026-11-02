package leetcode

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeJudge is LeetCode's judge for one solution: it checks that requests
// carry the session, and answers the checks of id with states, the last of
// which is the result.
func fakeJudge(t *testing.T, states ...string) (*Client, *atomic.Int32) {
	t.Helper()
	var polls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Cookie"); got != "LEETCODE_SESSION=tok; csrftoken=csrf" {
			t.Errorf("%s: Cookie %q", r.URL.Path, got)
		}
		if got := r.Header.Get("X-CSRFToken"); got != "csrf" {
			t.Errorf("%s: X-CSRFToken %q", r.URL.Path, got)
		}
		switch r.URL.Path {
		case "/problems/two-sum/interpret_solution/", "/problems/two-sum/submit/":
			if got := r.Header.Get("Referer"); !strings.HasSuffix(got, "/problems/two-sum/") {
				t.Errorf("Referer %q", got)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body["lang"] != "golang" || body["question_id"] != "1" || body["typed_code"] != "func twoSum() {}" {
				t.Errorf("body %v", body)
			}
			if strings.HasSuffix(r.URL.Path, "/interpret_solution/") {
				if body["data_input"] != "[2,7]\n9" {
					t.Errorf("data_input %q", body["data_input"])
				}
				_, _ = w.Write([]byte(`{"interpret_id": "runcode_1", "test_case": "[2,7]\n9"}`))
				return
			}
			_, _ = w.Write([]byte(`{"submission_id": 42}`))
		case "/submissions/detail/runcode_1/check/", "/submissions/detail/42/check/":
			n := int(polls.Add(1))
			_, _ = w.Write([]byte(states[min(n, len(states))-1]))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	c := NewClient(Site{Name: SiteCom.Name, Base: srv.URL}, "")
	c.Session = &Session{Token: "tok", CSRF: "csrf"}
	c.PollInterval = time.Millisecond
	return c, &polls
}

var code = Code{Slug: "two-sum", QuestionID: "1", Lang: "golang", Code: "func twoSum() {}"}

func TestInterpret(t *testing.T) {
	c, polls := fakeJudge(t,
		`{"state": "PENDING"}`,
		`{"state": "STARTED"}`,
		`{"state": "SUCCESS", "status_code": 10, "status_msg": "Accepted", "run_success": true, "correct_answer": true,
		  "code_answer": ["[0,1]"], "expected_code_answer": ["[0,1]"], "std_output_list": ["hi\n", ""], "compare_result": "1",
		  "total_correct": null, "runtime_percentile": null}`)
	id, err := c.Interpret(context.Background(), code, "[2,7]\n9")
	if err != nil || id != "runcode_1" {
		t.Fatalf("Interpret = %q, %v", id, err)
	}
	r, err := c.Wait(context.Background(), "two-sum", id)
	if err != nil {
		t.Fatal(err)
	}
	if polls.Load() != 3 {
		t.Errorf("polled %d times, want 3", polls.Load())
	}
	if !r.RunSuccess || !r.CorrectAnswer || r.CompareResult != "1" || len(r.CodeAnswer) != 1 || r.CodeAnswer[0] != "[0,1]" || r.StdOutputList[0] != "hi\n" {
		t.Errorf("result %+v", r)
	}
}

func TestSubmitWrongAnswer(t *testing.T) {
	c, _ := fakeJudge(t, `{"state": "SUCCESS", "status_code": 11, "status_msg": "Wrong Answer", "run_success": true,
		"total_correct": 50, "total_testcases": 63, "last_testcase": "[3,3]\n6", "code_output": "[0,0]",
		"expected_output": "[0,1]", "std_output": ""}`)
	id, err := c.Submit(context.Background(), code)
	if err != nil || id != "42" {
		t.Fatalf("Submit = %q, %v", id, err)
	}
	r, err := c.Wait(context.Background(), "two-sum", id)
	if err != nil {
		t.Fatal(err)
	}
	if r.Accepted() || r.TotalCorrect != 50 || r.TotalTestcases != 63 || r.LastTestcase != "[3,3]\n6" || r.CodeOutput != "[0,0]" || r.ExpectedOutput != "[0,1]" {
		t.Errorf("result %+v", r)
	}
}

func TestCompileError(t *testing.T) {
	c, _ := fakeJudge(t, `{"state": "SUCCESS", "status_code": 20, "status_msg": "Compile Error", "run_success": false,
		"compile_error": "Line 1: short", "full_compile_error": "Line 1: undefined: x\nLine 2: more"}`)
	r, err := c.Wait(context.Background(), "two-sum", "42")
	if err != nil {
		t.Fatal(err)
	}
	if r.Compile() != "Line 1: undefined: x\nLine 2: more" || r.Accepted() {
		t.Errorf("result %+v", r)
	}
}

func TestWaitTimeout(t *testing.T) {
	c, _ := fakeJudge(t, `{"state": "PENDING"}`)
	c.PollTimeout = 5 * time.Millisecond
	if _, err := c.Wait(context.Background(), "two-sum", "42"); err == nil || !strings.Contains(err.Error(), "still judging") {
		t.Errorf("err = %v", err)
	}
}

func TestRefusals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   error
		msg    string
	}{
		{"forbidden page", http.StatusForbidden, "<!DOCTYPE html><title>Just a moment...</title>", ErrBlocked, "HTTP 403"},
		{"sign-in page", http.StatusOK, "\n<html><body>Sign in</body></html>", ErrBlocked, ""},
		{"rate limited", http.StatusTooManyRequests, "", ErrRateLimited, ""},
		{"error reply", http.StatusOK, `{"error": "You have attempted to run code too soon."}`, nil, "too soon"},
		{"server error", http.StatusBadGateway, "bad gateway", nil, "HTTP 502"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			c := NewClient(Site{Name: SiteCN.Name, Base: srv.URL}, "")
			c.Session = &Session{Token: "secret-token", CSRF: "secret-csrf"}
			_, err := c.Submit(context.Background(), code)
			if err == nil {
				t.Fatal("no error")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.msg) {
				t.Errorf("err = %v, want it to mention %q", err, tc.msg)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Errorf("err shows the session: %v", err)
			}
		})
	}
}

func TestUsername(t *testing.T) {
	for _, tc := range []struct {
		body, user string
		err        error
	}{
		{`{"data": {"userStatus": {"isSignedIn": true, "username": "alice"}}}`, "alice", nil},
		{`{"data": {"userStatus": {"isSignedIn": false, "username": ""}}}`, "", ErrNotSignedIn},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/graphql/" || r.Header.Get("Cookie") != "LEETCODE_SESSION=tok; csrftoken=csrf" {
				t.Errorf("%s with Cookie %q", r.URL.Path, r.Header.Get("Cookie"))
			}
			_, _ = w.Write([]byte(tc.body))
		}))
		c := NewClient(Site{Name: SiteCom.Name, Base: srv.URL}, "")
		c.Session = &Session{Token: "tok", CSRF: "csrf"}
		user, err := c.Username(context.Background())
		srv.Close()
		if user != tc.user || !errors.Is(err, tc.err) {
			t.Errorf("Username = %q, %v; want %q, %v", user, err, tc.user, tc.err)
		}
	}
}

func TestLinesAndText(t *testing.T) {
	var v struct {
		A, B, C, D Lines
		E, F, G    Text
	}
	data := `{"A": ["x", "y"], "B": "one", "C": null, "D": [1, "z"], "E": "s", "F": ["l1", "l2"], "G": 7}`
	if err := json.Unmarshal([]byte(data), &v); err != nil {
		t.Fatal(err)
	}
	if strings.Join(v.A, "|") != "x|y" || strings.Join(v.B, "|") != "one" || v.C != nil || strings.Join(v.D, "|") != "1|z" {
		t.Errorf("lines %+v", v)
	}
	if v.E != "s" || v.F != "l1\nl2" || v.G != "7" {
		t.Errorf("text %+v", v)
	}
}
