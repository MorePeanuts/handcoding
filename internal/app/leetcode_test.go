package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/leetcode"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

// isolateConfig points the user's configuration directory into a temporary
// one, so that tests never touch the real LeetCode sessions.
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
	if _, _, err := app.LeetCodeLogin(ctx, leetcode.NewClient(com, ""), leetcode.Session{Token: "bad", CSRF: "c"}); err == nil || !strings.Contains(err.Error(), "does not accept") {
		t.Errorf("signed-out session: err = %v", err)
	}
	if _, _, err := app.LeetCodeLogin(ctx, leetcode.NewClient(com, ""), leetcode.Session{Token: "good"}); err == nil {
		t.Error("session without csrftoken accepted")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("sessions file written for refused sessions: %v", err)
	}
	if _, err := app.LeetCodeSession(com); err == nil || !strings.Contains(err.Error(), "hc leetcode login --site com") {
		t.Errorf("not logged in: err = %v", err)
	}

	user, saved, err := app.LeetCodeLogin(ctx, leetcode.NewClient(com, ""), leetcode.Session{Token: " good ", CSRF: "csrf-com\n"})
	if err != nil || user != "alice" || saved != path {
		t.Fatalf("LeetCodeLogin = %q, %q, %v", user, saved, err)
	}
	if _, _, err := app.LeetCodeLogin(ctx, leetcode.NewClient(cn, ""), leetcode.Session{Token: "good", CSRF: "csrf-cn"}); err != nil {
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
	if err != nil || s.Token != "good" || s.CSRF != "csrf-com" {
		t.Errorf("LeetCodeSession(com) = %+v, %v", s, err)
	}

	if removed, err := app.LeetCodeLogout(com); !removed || err != nil {
		t.Errorf("LeetCodeLogout(com) = %v, %v", removed, err)
	}
	if _, err := app.LeetCodeSession(com); err == nil {
		t.Error("com still logged in")
	}
	if s, err := app.LeetCodeSession(cn); err != nil || s.CSRF != "csrf-cn" {
		t.Errorf("cn lost on logout of com: %+v, %v", s, err)
	}
	if removed, err := app.LeetCodeLogout(com); removed || err != nil {
		t.Errorf("second LeetCodeLogout(com) = %v, %v", removed, err)
	}
	if removed, err := app.LeetCodeLogout(cn); !removed || err != nil {
		t.Errorf("LeetCodeLogout(cn) = %v, %v", removed, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("sessions file left after the last logout: %v", err)
	}
}

func TestLeetCodeProblem(t *testing.T) {
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
	c.Session = &leetcode.Session{Token: "t", CSRF: "c"}
	c.PollInterval = time.Millisecond
	return c
}

func TestLeetCodeTestAndSubmit(t *testing.T) {
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
