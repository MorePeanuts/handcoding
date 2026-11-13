package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/leetcode"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

// isolateConfig points the user's configuration directory into a temporary
// one, so that tests never touch the real LeetCode sessions. It sets the
// environment, so the tests that call it cannot run in parallel.
func isolateConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("AppData", filepath.Join(dir, "AppData"))
	path, err := app.LeetCodeSessionsPath()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, dir) {
		t.Fatalf("sessions file %s is outside %s", path, dir)
	}
	return path
}

// userStatus answers LeetCode's userStatus query: signed in as alice for the
// session token "good".
func userStatus(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signedIn := strings.Contains(r.Header.Get("Cookie"), "LEETCODE_SESSION=good;")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"userStatus": map[string]any{"isSignedIn": signedIn, "username": map[bool]string{true: "alice"}[signedIn]},
		}})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestLeetCodeSessions(t *testing.T) {
	path := isolateConfig(t)
	srv := userStatus(t)
	ctx := context.Background()
	com := leetcode.Site{Name: "com", Base: srv.URL}
	cn := leetcode.Site{Name: "cn", Base: srv.URL}
	if _, err := app.LeetCodeSession(); err == nil {
		t.Fatal("signed out session accepted")
	}
	user, saved, err := app.LeetCodeLogin(ctx, leetcode.NewClient(com, ""), leetcode.Session{Token: " good\n"})
	if err != nil || user != "alice" || saved != path {
		t.Fatalf("login: %q, %q, %v", user, saved, err)
	}
	before, _ := os.ReadFile(path)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	for _, tc := range []struct {
		ctx   context.Context
		token string
	}{{ctx, "bad"}, {ctx, " "}, {cancelled, "good"}} {
		if _, _, err := app.LeetCodeLogin(tc.ctx, leetcode.NewClient(cn, ""), leetcode.Session{Token: tc.token}); err == nil {
			t.Fatal("failed login succeeded")
		}
		after, _ := os.ReadFile(path)
		if string(before) != string(after) {
			t.Fatal("failed login replaced previous login")
		}
	}
	if _, _, err := app.LeetCodeLogin(ctx, leetcode.NewClient(cn, ""), leetcode.Session{Token: "good"}); err != nil {
		t.Fatal(err)
	}
	account, err := app.LeetCodeSession()
	if err != nil || account.Site.Name != "cn" || account.User != "alice" || account.Session.Token != "good" {
		t.Fatalf("single account: %+v, %v", account, err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "com") {
		t.Fatal("old account retained")
	}
	for p, want := range map[string]os.FileMode{path: 0o600, filepath.Dir(path): 0o700} {
		if st, err := os.Stat(p); err != nil || st.Mode().Perm() != want {
			t.Fatalf("permissions of %s: %v", p, err)
		}
	}
	for _, old := range []string{`{"com":{"session":"good"},"cn":{"session":"good"}}`, `{"cn":{"session":"good"}}`, `{bad`, `{"site":"cn","credentials":{"session":""}}`} {
		if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := app.LeetCodeSession(); err == nil || !strings.Contains(err.Error(), "login") {
			t.Fatal("old/invalid state silently selected an account")
		}
		if _, _, err := app.LeetCodeLogin(ctx, leetcode.NewClient(cn, ""), leetcode.Session{Token: "good"}); err != nil {
			t.Fatalf("login cannot replace old state: %v", err)
		}
	}
	if err := os.WriteFile(path, []byte(`{broken`), 0o600); err != nil {
		t.Fatal(err)
	}
	if removed, err := app.LeetCodeLogout(); !removed || err != nil {
		t.Fatalf("logout broken state: %v, %v", removed, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("login file retained: %v", err)
	}
	if removed, err := app.LeetCodeLogout(); removed || err != nil {
		t.Fatalf("already logged out: %v, %v", removed, err)
	}
}

type leetCodeTransport func(*http.Request) (*http.Response, error)

func (f leetCodeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLeetCodeStatus(t *testing.T) {
	path := isolateConfig(t)
	ctx := context.Background()
	status, saved, err := app.LeetCodeStatus(ctx)
	if err != nil || saved != path || status != nil {
		t.Fatalf("signed out status: %+v, %v", status, err)
	}
	srv := userStatus(t)
	u, _ := url.Parse(srv.URL)
	transport := http.DefaultTransport
	http.DefaultTransport = leetCodeTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "leetcode.cn" {
			t.Errorf("status queried %s", r.URL.Host)
		}
		r = r.Clone(r.Context())
		r.URL.Scheme, r.URL.Host = u.Scheme, u.Host
		return transport.RoundTrip(r)
	})
	t.Cleanup(func() { http.DefaultTransport = transport })
	cn := leetcode.Site{Name: "cn", Base: srv.URL}
	// Login uses its own explicit transport here to avoid the host assertion.
	c := leetcode.NewClient(cn, "")
	c.HTTP.Transport = transport
	if _, _, err := app.LeetCodeLogin(ctx, c, leetcode.Session{Token: "good"}); err != nil {
		t.Fatal(err)
	}
	status, _, err = app.LeetCodeStatus(ctx)
	if err != nil || status == nil || status.Err != nil || status.User != "alice" || status.Site.Name != "cn" {
		t.Fatalf("current status: %+v, %v", status, err)
	}
	if err := os.WriteFile(path, []byte(`{"site":"cn","credentials":{"session":"expired"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	status, _, err = app.LeetCodeStatus(ctx)
	if err != nil || !errors.Is(status.Err, leetcode.ErrNotSignedIn) {
		t.Fatalf("expired status: %+v, %v", status, err)
	}
	http.DefaultTransport = leetCodeTransport(func(r *http.Request) (*http.Response, error) { return nil, io.ErrUnexpectedEOF })
	status, _, err = app.LeetCodeStatus(ctx)
	if err != nil || status.Err == nil || errors.Is(status.Err, leetcode.ErrNotSignedIn) {
		t.Fatalf("unreachable status: %+v, %v", status, err)
	}
}

// The browser login takes the session from the authorize-login redirect, at
// its own path only, once.
func TestLeetCodeBrowserLogin(t *testing.T) {
	isolateConfig(t)
	srv := userStatus(t)
	c := leetcode.NewClient(leetcode.Site{Name: leetcode.SiteCN.Name, Base: srv.URL}, "")
	get := func(u string) (int, string) {
		t.Helper()
		resp, err := http.Get(u)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}
	// start returns a login and its callback URL, as the page redirects to it.
	start := func() (*app.LeetCodeBrowserLogin, string) {
		t.Helper()
		b, err := app.StartLeetCodeBrowserLogin(c, 0)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = b.Close() })
		page, err := url.Parse(b.URL)
		if err != nil || page.Path != "/authorize-login/http/" || !strings.HasPrefix(b.URL, srv.URL) {
			t.Fatalf("URL %s", b.URL)
		}
		callback := page.Query().Get("path")
		if !strings.HasPrefix(callback, fmt.Sprintf("127.0.0.1:%d/callback/", b.Port)) {
			t.Fatalf("callback %s", callback)
		}
		return b, "http://" + callback
	}
	wait := func(b *app.LeetCodeBrowserLogin) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		user, _, err := b.Wait(ctx)
		return user, err
	}

	b, callback := start()
	if code, _ := get(strings.TrimRight(callback, "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567") + "?cookie=LEETCODE_SESSION%3Dgood"); code != http.StatusNotFound {
		t.Errorf("another path: HTTP %d", code)
	}
	cookie := "?cookie=" + url.QueryEscape("LEETCODE_SESSION=good; csrftoken=x")
	if code, body := get(callback + cookie); code != http.StatusOK || !strings.Contains(body, "as alice") {
		t.Errorf("callback: HTTP %d, %q", code, body)
	}
	if user, err := wait(b); user != "alice" || err != nil {
		t.Errorf("Wait = %q, %v", user, err)
	}
	if s, err := app.LeetCodeSession(); err != nil || s.Session.Token != "good" {
		t.Errorf("saved session %+v, %v", s, err)
	}
	if _, body := get(callback + cookie); !strings.Contains(body, "over") {
		t.Errorf("second callback: %q", body)
	}

	for _, q := range []string{"?cookie=" + url.QueryEscape("LEETCODE_SESSION=bad"), ""} {
		b, callback := start()
		if code, _ := get(callback + q); code != http.StatusBadRequest {
			t.Errorf("callback%s: HTTP %d", q, code)
		}
		if _, err := wait(b); err == nil {
			t.Errorf("callback%s: logged in", q)
		}
	}

	b, _ = start()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := b.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Wait canceled: %v", err)
	}
}

func TestLeetCodeProblem(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		source       problem.Source
		want         string
		wantErr      bool
		wantSiteName string
	}{
		{source: problem.Source{Site: "leetcode.cn", URL: "https://leetcode.cn/problems/two-sum/"}, want: "two-sum", wantSiteName: "cn"},
		{source: problem.Source{Site: "builtin", URL: "https://leetcode.com/problems/add-two-numbers/description/"}, want: "add-two-numbers", wantSiteName: "com"},
		{source: problem.Source{Site: "leetcode.com"}, want: "two-sum", wantSiteName: "com"},
		{source: problem.Source{Site: "leetcode.cn"}, want: "two-sum", wantSiteName: "cn"},
		{source: problem.Source{Site: "custom"}, wantErr: true},
		{source: problem.Source{}, wantErr: true},
	} {
		p := &problem.Problem{Slug: "two-sum", Source: tc.source}
		got, err := app.LeetCodeProblem(p)
		if (err != nil) != tc.wantErr || (err == nil && (got.Slug != tc.want || got.Site.Name != tc.wantSiteName)) {
			t.Errorf("LeetCodeProblem(%+v) = %+v, %v", tc.source, got, err)
		}
	}
}

func TestLeetCodeCase(t *testing.T) {
	t.Parallel()
	twoSum := &problem.Problem{Slug: "two-sum", Kind: problem.KindFunction, Function: &problem.Function{
		Name:    "twoSum",
		Params:  []problem.Param{{Name: "nums", Type: "integer[]"}, {Name: "target", Type: "integer"}},
		Returns: "integer[]",
	}, Judge: problem.Judge{Compare: problem.CompareUnordered}}
	c, ok := app.LeetCodeCase(twoSum, "[3,3]\n6\n", "[0,1]")
	if !ok || string(c.Input["nums"]) != "[3,3]" || string(c.Input["target"]) != "6" || string(c.Expected) != "[0,1]" {
		t.Errorf("two-sum: %+v, %v", c, ok)
	}
	for name, in := range map[string]string{
		"a line per parameter": "[3,3]",
		"JSON values":          "[3,3\n6",
		"valid values":         `"x"` + "\n6",
	} {
		if _, ok := app.LeetCodeCase(twoSum, in, "[0,1]"); ok {
			t.Errorf("%s: %q mapped", name, in)
		}
	}
	// An expected output that does not fit is dropped, not the case.
	if c, ok := app.LeetCodeCase(twoSum, "[3,3]\n6", `"no"`); !ok || c.Expected != nil {
		t.Errorf("bad expected: %+v, %v", c, ok)
	}
	anyOf := *twoSum
	anyOf.Judge.Compare = problem.CompareAnyOf
	if c, ok := app.LeetCodeCase(&anyOf, "[3,3]\n6", "[0,1]"); !ok || c.Expected != nil {
		t.Errorf("any_of keeps LeetCode's one answer: %+v, %v", c, ok)
	}

	// Inputs LeetCode gives in another form than hc does not map.
	cycle := &problem.Problem{Slug: "linked-list-cycle", Kind: problem.KindFunction, Function: &problem.Function{
		Name:    "hasCycle",
		Params:  []problem.Param{{Name: "head", Type: "ListNode", Cycle: "pos"}},
		Hidden:  []problem.Param{{Name: "pos", Type: "integer"}},
		Returns: "boolean",
	}}
	clone := &problem.Problem{Slug: "clone-graph", Kind: problem.KindFunction, Function: &problem.Function{
		Name: "cloneGraph", Params: []problem.Param{{Name: "node", Type: "GraphNode"}}, Returns: "GraphNode",
	}}
	lru := &problem.Problem{Slug: "lru-cache", Kind: problem.KindClass, Class: &problem.Class{Name: "LRUCache"}}
	for _, p := range []*problem.Problem{cycle, clone, lru} {
		if c, ok := app.LeetCodeCase(p, "[1]\n0", "true"); ok {
			t.Errorf("%s mapped: %+v", p.Slug, c)
		}
	}
}

// fakeLeetCode is LeetCode for regression/two-sum, which judges every
// submission Wrong Answer on [3,3], 6.
func fakeLeetCode(t *testing.T, typed *string) *leetcode.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch r.URL.Path {
		case "/graphql/":
			_, _ = w.Write([]byte(`{"data": {"question": {"questionId": "1", "questionFrontendId": "1", "titleSlug": "two-sum",
				"exampleTestcases": "[2,7,11,15]\n9\n[3,2,4]\n6"}}}`))
		case "/problems/two-sum/interpret_solution/", "/problems/two-sum/submit/":
			var req map[string]any
			if err := json.Unmarshal(body, &req); err != nil {
				t.Error(err)
			}
			*typed, _ = req["typed_code"].(string)
			if r.URL.Path == "/problems/two-sum/submit/" {
				_, _ = w.Write([]byte(`{"submission_id": 7}`))
				return
			}
			if req["data_input"] != "[2,7,11,15]\n9\n[3,2,4]\n6" {
				t.Errorf("data_input %q", req["data_input"])
			}
			_, _ = w.Write([]byte(`{"interpret_id": "r7"}`))
		case "/submissions/detail/7/check/":
			_, _ = w.Write([]byte(`{"state": "SUCCESS", "status_code": 11, "status_msg": "Wrong Answer", "run_success": true,
				"total_correct": 1, "total_testcases": 3, "last_testcase": "[3,3]\n6", "code_output": "[]", "expected_output": "[0,1]"}`))
		case "/submissions/detail/r7/check/":
			_, _ = w.Write([]byte(`{"state": "SUCCESS", "status_code": 10, "status_msg": "Accepted", "run_success": true,
				"correct_answer": false, "code_answer": ["[]", "[]"], "expected_code_answer": ["[0,1]", "[1,2]"], "compare_result": "00"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	c := leetcode.NewClient(leetcode.Site{Name: leetcode.SiteCom.Name, Base: srv.URL}, "")
	c.Session = &leetcode.Session{Token: "t"}
	c.PollInterval = time.Millisecond
	return c
}

func TestLeetCodeTestAndSubmit(t *testing.T) {
	t.Parallel()
	a := newWorkspace(t)
	for _, l := range a.Langs.All() {
		if _, ok := l.(lang.LeetCoder); !ok {
			t.Errorf("%s solutions cannot run on LeetCode", l.Name())
		}
	}
	p, err := a.WS.Problem("regression/two-sum")
	if err != nil {
		t.Fatal(err)
	}
	l := mustLang(t, a, "go")
	path := a.SolutionPath(p, l)
	var typed string
	c := fakeLeetCode(t, &typed)
	ctx := context.Background()

	if _, err := a.LeetCodeSubmit(ctx, c, p, l); err == nil || !strings.Contains(err.Error(), "hc start") {
		t.Errorf("without a solution: err = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	src := "package main\n\nfunc twoSum(nums []int, target int) []int { return nil }\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	run, err := a.LeetCodeTest(ctx, c, p, l, "")
	if err != nil {
		t.Fatal(err)
	}
	if run.Input != "[2,7,11,15]\n9\n[3,2,4]\n6" || run.Result.CompareResult != "00" {
		t.Errorf("test: %+v", run)
	}
	if typed != "func twoSum(nums []int, target int) []int { return nil }\n" {
		t.Errorf("typed_code %q", typed)
	}

	run, err = a.LeetCodeSubmit(ctx, c, p, l)
	if err != nil {
		t.Fatal(err)
	}
	if run.Result.Accepted() || run.Case == nil {
		t.Fatalf("submit: %+v", run)
	}
	if string(run.Case.Input["nums"]) != "[3,3]" || string(run.Case.Input["target"]) != "6" || string(run.Case.Expected) != "[0,1]" {
		t.Errorf("failing case %+v", run.Case)
	}
}

// Cross-site runs must resolve identity before sending code, and must use the
// destination's internal question ID. Metadata reads on the source are anonymous.
func TestLeetCodeCrossSite(t *testing.T) {
	t.Parallel()
	a := newWorkspace(t)
	p, err := a.WS.Problem("regression/two-sum")
	if err != nil {
		t.Fatal(err)
	}
	// A pack's local ID is not an authoritative cross-site identifier.
	p.ID = "987654"
	l := mustLang(t, a, "go")
	path := a.SolutionPath(p, l)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package main\nfunc twoSum(nums []int, target int) []int {return nil}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	const metadata = `{"name":"twoSum","params":[{"name":"nums","type":"integer[]"},{"name":"target","type":"integer"}],"return":{"type":"integer[]"}}`
	for _, tc := range []struct {
		name, sourceID, targetID, targetSlug, sourceMeta, targetMeta string
		sameSite, missing, sourceFailure, expired, emptyInternal, ok bool
	}{
		{name: "matching", ok: true},
		{name: "metadata key order", targetMeta: `{"return":{"type":"integer[]"},"params":[{"type":"integer[]","name":"nums"},{"type":"integer","name":"target"}],"name":"twoSum"}`, ok: true},
		{name: "same site series", sameSite: true, sourceID: "LCR 001", targetID: "LCR 001", ok: true},
		{name: "series is not mapped", sourceID: "LCR 001", targetID: "LCR 001"},
		{name: "different ID", targetID: "2"},
		{name: "different slug", targetSlug: "similar-two-sum"},
		{name: "different signature", targetMeta: `{"name":"twoSum","params":[],"return":{"type":"integer"}}`},
		{name: "missing signature", targetMeta: `null`},
		{name: "invalid source signature", sourceMeta: `{`},
		{name: "missing destination", missing: true},
		{name: "source unavailable", sourceFailure: true},
		{name: "expired destination", expired: true},
		{name: "missing internal ID", emptyInternal: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, destination := range []leetcode.Site{leetcode.SiteCN, leetcode.SiteCom} {
				for _, submit := range []bool{false, true} {
					source := leetcode.SiteCom
					if destination.Name == "com" {
						source = leetcode.SiteCN
					}
					if tc.sameSite {
						source = destination
					}
					local := *p
					local.Source = problem.Source{URL: source.Base + "/problems/two-sum/"}
					original := leetcode.Question{QuestionID: "101", QuestionFrontendID: "1", TitleSlug: "two-sum", MetaData: metadata}
					target := original
					target.QuestionID = "9001"
					target.ExampleTestcases = "[2,7]\n9"
					if tc.sourceID != "" {
						original.QuestionFrontendID = tc.sourceID
					}
					if tc.targetID != "" {
						target.QuestionFrontendID = tc.targetID
					}
					if tc.targetSlug != "" {
						target.TitleSlug = tc.targetSlug
					}
					if tc.sourceMeta != "" {
						original.MetaData = tc.sourceMeta
					}
					if tc.targetMeta != "" {
						target.MetaData = tc.targetMeta
					}
					if tc.emptyInternal {
						target.QuestionID = ""
					}
					sources, sends := 0, 0
					c := leetcode.NewClient(destination, "")
					c.Session = &leetcode.Session{Token: "destination-only"}
					if !tc.sameSite {
						jar, _ := cookiejar.New(nil)
						u, _ := url.Parse(source.Base)
						jar.SetCookies(u, []*http.Cookie{{Name: "LEETCODE_SESSION", Value: "jar-secret"}})
						c.HTTP.Jar = jar
					}
					c.PollInterval = time.Millisecond
					c.HTTP.Transport = leetCodeTransport(func(r *http.Request) (*http.Response, error) {
						var body []byte
						if r.Body != nil {
							body, _ = io.ReadAll(r.Body)
						}
						respond := func(status int, value any) (*http.Response, error) {
							data, _ := json.Marshal(value)
							return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(data))), Request: r}, nil
						}
						if r.URL.Host != strings.TrimPrefix(destination.Base, "https://") {
							sources++
							if r.URL.Host != strings.TrimPrefix(source.Base, "https://") || r.URL.Path != "/graphql/" {
								t.Errorf("unexpected source request: %s", r.URL)
							}
							if r.Header.Get("Cookie") != "" || strings.Contains(string(body), "typed_code") {
								t.Error("credentials or code sent to source")
							}
							if tc.sourceFailure {
								return respond(503, map[string]any{})
							}
							return respond(200, map[string]any{"data": map[string]any{"question": original}})
						}
						if !strings.Contains(r.Header.Get("Cookie"), "destination-only") {
							t.Error("missing destination session")
						}
						if r.URL.Path == "/graphql/" {
							if tc.expired {
								return respond(401, map[string]any{})
							}
							if tc.missing {
								return respond(200, map[string]any{"data": map[string]any{"question": nil}})
							}
							if !strings.Contains(string(body), "metaData") {
								t.Error("signature not requested")
							}
							return respond(200, map[string]any{"data": map[string]any{"question": target}})
						}
						if strings.HasSuffix(r.URL.Path, "/check/") {
							return respond(200, map[string]any{"state": "SUCCESS", "status_code": 10, "status_msg": "Accepted", "run_success": true})
						}
						sends++
						var req map[string]any
						if err := json.Unmarshal(body, &req); err != nil {
							t.Fatal(err)
						}
						if req["question_id"] != "9001" {
							t.Errorf("wrong destination question ID: %v", req["question_id"])
						}
						if !strings.Contains(fmt.Sprint(req["typed_code"]), "func twoSum") {
							t.Error("missing solution")
						}
						if submit {
							return respond(200, map[string]any{"submission_id": 7})
						}
						return respond(200, map[string]any{"interpret_id": "7"})
					})
					if submit {
						_, err = a.LeetCodeSubmit(context.Background(), c, &local, l)
					} else {
						_, err = a.LeetCodeTest(context.Background(), c, &local, l, "")
					}
					if tc.ok && (err != nil || sends != 1) {
						t.Fatalf("%s submit=%v: sends=%d err=%v", destination.Name, submit, sends, err)
					}
					if !tc.ok && (err == nil || sends != 0) {
						t.Fatalf("unsafe mapping %s submit=%v: sends=%d err=%v", destination.Name, submit, sends, err)
					}
					if tc.sameSite && sources != 0 {
						t.Fatal("same-site run contacted another site")
					}
					if tc.expired && sources != 0 {
						t.Fatal("expired destination fell back to source")
					}
				}
			}
		})
	}
}

func TestLeetCodeBrowserCancelPreservesAccount(t *testing.T) {
	path := isolateConfig(t)
	existing := userStatus(t)
	if _, _, err := app.LeetCodeLogin(context.Background(), leetcode.NewClient(leetcode.Site{Name: "com", Base: existing.URL}, ""), leetcode.Session{Token: "good"}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	started := make(chan struct{})
	release := make(chan struct{})
	pending := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte(`{"data":{"userStatus":{"isSignedIn":true,"username":"bob"}}}`))
	}))
	defer pending.Close()
	defer close(release)
	b, err := app.StartLeetCodeBrowserLogin(leetcode.NewClient(leetcode.Site{Name: "cn", Base: pending.URL}, ""), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	page, _ := url.Parse(b.URL)
	done := make(chan struct{})
	go func() {
		defer close(done)
		resp, err := http.Get("http://" + page.Query().Get("path") + "?cookie=" + url.QueryEscape("LEETCODE_SESSION=good"))
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("authorization did not start")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := b.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not stop authorization")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("cancelled browser authorization replaced the current account")
	}
}
