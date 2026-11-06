package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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
	com := leetcode.Site{Name: leetcode.SiteCom.Name, Base: srv.URL}
	cn := leetcode.Site{Name: leetcode.SiteCN.Name, Base: srv.URL}

	// A session LeetCode does not accept, or half of one, is not saved.
	if _, _, err := app.LeetCodeLogin(ctx, leetcode.NewClient(com, ""), leetcode.Session{Token: "bad"}); err == nil || !strings.Contains(err.Error(), "does not accept") {
		t.Errorf("signed-out session: err = %v", err)
	}
	if _, _, err := app.LeetCodeLogin(ctx, leetcode.NewClient(com, ""), leetcode.Session{Token: " \n"}); err == nil {
		t.Error("empty session accepted")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("sessions file written for refused sessions: %v", err)
	}
	if _, err := app.LeetCodeSession(com); err == nil || !strings.Contains(err.Error(), "hc leetcode login --site com") {
		t.Errorf("not logged in: err = %v", err)
	}

	user, saved, err := app.LeetCodeLogin(ctx, leetcode.NewClient(com, ""), leetcode.Session{Token: " good\n"})
	if err != nil || user != "alice" || saved != path {
		t.Fatalf("LeetCodeLogin = %q, %q, %v", user, saved, err)
	}
	if _, _, err := app.LeetCodeLogin(ctx, leetcode.NewClient(cn, ""), leetcode.Session{Token: "good"}); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		for p, want := range map[string]os.FileMode{path: 0o600, filepath.Dir(path): 0o700} {
			if st, err := os.Stat(p); err != nil || st.Mode().Perm() != want {
				t.Errorf("%s: mode %v, %v; want %v", p, st.Mode().Perm(), err, want)
			}
		}
	}
	s, err := app.LeetCodeSession(com)
	if err != nil || s.Token != "good" {
		t.Errorf("LeetCodeSession(com) = %+v, %v", s, err)
	}

	if sites, err := app.LeetCodeSites(); !slices.Equal(sites, []string{"com", "cn"}) || err != nil {
		t.Errorf("LeetCodeSites() = %v, %v", sites, err)
	}

	if removed, err := app.LeetCodeLogout(com); !slices.Equal(removed, []string{"com"}) || err != nil {
		t.Errorf("LeetCodeLogout(com) = %v, %v", removed, err)
	}
	if _, err := app.LeetCodeSession(com); err == nil {
		t.Error("com still logged in")
	}
	if s, err := app.LeetCodeSession(cn); err != nil || s.Token != "good" {
		t.Errorf("cn lost on logout of com: %+v, %v", s, err)
	}
	if removed, err := app.LeetCodeLogout(com); removed != nil || err != nil {
		t.Errorf("second LeetCodeLogout(com) = %v, %v", removed, err)
	}
	// Without a site, every session goes.
	if removed, err := app.LeetCodeLogout(); !slices.Equal(removed, []string{"cn"}) || err != nil {
		t.Errorf("LeetCodeLogout() = %v, %v", removed, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("sessions file left after the last logout: %v", err)
	}
	if removed, err := app.LeetCodeLogout(); removed != nil || err != nil {
		t.Errorf("LeetCodeLogout() logged out = %v, %v", removed, err)
	}
}

// Status checks each saved session with LeetCode, and reports the sites
// without one.
func TestLeetCodeStatus(t *testing.T) {
	path := isolateConfig(t)
	srv := userStatus(t)
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	t.Cleanup(down.Close)
	ctx := context.Background()
	com := leetcode.Site{Name: leetcode.SiteCom.Name, Base: srv.URL}
	cn := leetcode.Site{Name: leetcode.SiteCN.Name, Base: srv.URL}

	status, saved, err := app.LeetCodeStatus(ctx, com, cn)
	if err != nil || saved != path || len(status) != 2 || status[0].Saved || status[1].Saved {
		t.Fatalf("LeetCodeStatus without sessions = %+v, %q, %v", status, saved, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"com": {"session": "good"}, "cn": {"session": "expired"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	status, _, err = app.LeetCodeStatus(ctx, com, cn)
	if err != nil || len(status) != 2 {
		t.Fatalf("LeetCodeStatus = %+v, %v", status, err)
	}
	if s := status[0]; !s.Saved || s.User != "alice" || s.Err != nil {
		t.Errorf("com: %+v", s)
	}
	if s := status[1]; !s.Saved || s.User != "" || !errors.Is(s.Err, leetcode.ErrNotSignedIn) {
		t.Errorf("cn: %+v", s)
	}

	status, _, err = app.LeetCodeStatus(ctx, leetcode.Site{Name: leetcode.SiteCom.Name, Base: down.URL})
	if err != nil || len(status) != 1 || !status[0].Saved || status[0].Err == nil || errors.Is(status[0].Err, leetcode.ErrNotSignedIn) {
		t.Errorf("unreachable site: %+v, %v", status, err)
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
	if s, err := app.LeetCodeSession(c.Site); err != nil || s.Token != "good" {
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
		site, want   string
		wantErr      bool
		wantSiteName string
	}{
		{source: problem.Source{Site: "leetcode.cn", URL: "https://leetcode.cn/problems/two-sum/"}, want: "two-sum", wantSiteName: "cn"},
		{source: problem.Source{Site: "builtin", URL: "https://leetcode.com/problems/add-two-numbers/description/"}, want: "add-two-numbers", wantSiteName: "com"},
		{source: problem.Source{Site: "leetcode.com"}, want: "two-sum", wantSiteName: "com"},
		{source: problem.Source{Site: "leetcode.cn"}, site: "com", want: "two-sum", wantSiteName: "com"},
		{source: problem.Source{Site: "custom"}, wantErr: true},
		{source: problem.Source{}, wantErr: true},
	} {
		p := &problem.Problem{Slug: "two-sum", Source: tc.source}
		got, err := app.LeetCodeProblem(p, tc.site)
		if (err != nil) != tc.wantErr || (err == nil && (got.Slug != tc.want || got.Site.Name != tc.wantSiteName)) {
			t.Errorf("LeetCodeProblem(%+v, %q) = %+v, %v", tc.source, tc.site, got, err)
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

	if _, err := a.LeetCodeSubmit(ctx, c, p, l, "two-sum"); err == nil || !strings.Contains(err.Error(), "hc start") {
		t.Errorf("without a solution: err = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	src := "package main\n\nfunc twoSum(nums []int, target int) []int { return nil }\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	run, err := a.LeetCodeTest(ctx, c, p, l, "two-sum", "")
	if err != nil {
		t.Fatal(err)
	}
	if run.Input != "[2,7,11,15]\n9\n[3,2,4]\n6" || run.Result.CompareResult != "00" {
		t.Errorf("test: %+v", run)
	}
	if typed != "func twoSum(nums []int, target int) []int { return nil }\n" {
		t.Errorf("typed_code %q", typed)
	}

	run, err = a.LeetCodeSubmit(ctx, c, p, l, "two-sum")
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
