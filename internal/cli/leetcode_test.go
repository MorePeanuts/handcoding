package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/leetcode"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// The commands that need no network: logging out, and refusing problems
// before any request.
func TestLeetCodeCommands(t *testing.T) {
	dir := packWorkspace(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))

	// Logout clears the global login, whatever leetcode.site says.
	path, err := app.LeetCodeSessionsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"site":"cn","credentials":{"session":"s"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, "logged out of LeetCode"},
		{nil, "Not logged in to LeetCode."},
	} {
		out, err := capture(t, append([]string{"-w", dir, "leetcode", "logout"}, tc.args...)...)
		if err != nil || !strings.Contains(out, tc.want) {
			t.Errorf("logout %v: %q, %v; want %q", tc.args, out, err, tc.want)
		}
	}
	if _, err := capture(t, "-w", dir, "leetcode", "login", "--paste", "--port", "8765"); err == nil || !strings.Contains(err.Error(), "paste") {
		t.Errorf("login --paste --port: err = %v", err)
	}
	for ref, want := range map[string]string{
		"local/two-sum":      "does not come from LeetCode",
		"regression/two-sum": "not logged in to LeetCode; run hc leetcode login",
	} {
		for _, sub := range []string{"test", "submit"} {
			if _, err := capture(t, "-w", dir, "leetcode", sub, ref, "-l", "go"); err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("leetcode %s %s: err = %v, want %q", sub, ref, err, want)
			}
		}
	}
	for _, sub := range []string{"test", "submit", "status", "logout"} {
		args := []string{"-w", dir, "leetcode", sub, "--site", "cn"}
		if sub == "test" || sub == "submit" {
			args = append(args, "regression/two-sum")
		}
		if _, err := capture(t, args...); err == nil || !strings.Contains(err.Error(), "unknown flag") {
			t.Errorf("%s accepts obsolete --site: %v", sub, err)
		}
	}

}

func TestSplitTestcases(t *testing.T) {
	got := splitTestcases("[2,7]\n9\n[3,3]\n6\n", 2, 2)
	if len(got) != 2 || got[0] != "[2,7]\n9" || got[1] != "[3,3]\n6" {
		t.Errorf("splitTestcases = %q", got)
	}
	if got := splitTestcases("[2,7]\n9\n[3,3]", 2, 2); got != nil {
		t.Errorf("uneven testcases split: %q", got)
	}
}

func TestCaseSuggestion(t *testing.T) {
	dir := packWorkspace(t)
	ws, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := app.New(ws)
	defer a.Close()
	p, err := ws.Problem("regression/two-sum")
	if err != nil {
		t.Fatal(err)
	}
	c, err := problem.ParseCase([]byte(`{"input":{"nums":[3,3],"target":6},"expected":[0,1]}`))
	if err != nil {
		t.Fatal(err)
	}
	out := captureFunc(t, func() { printCaseSuggestion(a, p, c, "Wrong Answer", false) })
	want := `hc cases add regression/two-sum --input '{"nums":[3,3],"target":6}' --expected '[0,1]' --note 'from LeetCode: Wrong Answer'`
	if !strings.Contains(out, want) {
		t.Errorf("suggestion:\n%s\nwant %s", out, want)
	}
}

func TestPrintLeetCodeStatus(t *testing.T) {
	com, cn := leetcode.SiteCom, leetcode.SiteCN
	for _, tc := range []struct {
		name   string
		status *app.LeetCodeSiteStatus
		ok     bool
		want   []string
	}{
		{"none", nil, false, []string{"Not logged in to LeetCode; run `hc leetcode login`."}},
		{"current", &app.LeetCodeSiteStatus{Site: cn, User: "alice"}, true, []string{"leetcode.cn logged in as alice", "Login is kept in /cfg/leetcode.json."}},
		{"expired", &app.LeetCodeSiteStatus{Site: com, Err: leetcode.ErrNotSignedIn}, false, []string{"session no longer accepted; log in again with `hc leetcode login --site com`"}},
		{"unchecked", &app.LeetCodeSiteStatus{Site: cn, Err: io.ErrUnexpectedEOF}, false, []string{"could not check the session: unexpected EOF"}},
	} {
		var ok bool
		out := captureFunc(t, func() { ok = printLeetCodeStatus(tc.status, "/cfg/leetcode.json") })
		if ok != tc.ok {
			t.Errorf("%s: ok = %v", tc.name, ok)
		}
		for _, want := range tc.want {
			if !strings.Contains(out, want) {
				t.Errorf("%s: output lacks %q:\n%s", tc.name, want, out)
			}
		}
		// The file is named only when it holds a session.
		if saved := tc.status != nil; saved != strings.Contains(out, "/cfg/leetcode.json") {
			t.Errorf("%s: sessions file shown = %v:\n%s", tc.name, !saved, out)
		}
	}
}

func TestPrintLeetCodeResults(t *testing.T) {
	dir := packWorkspace(t)
	ws, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := app.New(ws)
	defer a.Close()
	p, err := ws.Problem("regression/two-sum")
	if err != nil {
		t.Fatal(err)
	}

	var ok bool
	run := &app.LeetCodeRun{Input: "[2,7]\n9\n[3,3]\n6", Result: &leetcode.Result{
		StatusCode: 10, StatusMsg: "Accepted", RunSuccess: true, CompareResult: "10",
		CodeAnswer: leetcode.Lines{"[0,1]", "[]"}, ExpectedCodeAnswer: leetcode.Lines{"[0,1]", "[0,1]"}, StdOutputList: leetcode.Lines{"", "debug\n"},
	}}
	out := captureFunc(t, func() { ok = printLeetCodeTest(p, run, false) })
	for _, want := range []string{"Case 1", "Case 2", "Wrong Answer", "[3,3]", "debug", "1/2 match LeetCode's answers"} {
		if !strings.Contains(out, want) {
			t.Errorf("test output lacks %q:\n%s", want, out)
		}
	}
	if ok {
		t.Error("a run with a wrong case is reported as passing")
	}

	run = &app.LeetCodeRun{Result: &leetcode.Result{
		StatusCode: 10, StatusMsg: "Accepted", TotalCorrect: 63, TotalTestcases: 63,
		StatusRuntime: "0 ms", RuntimePercentile: 100, StatusMemory: "4.3 MB", MemoryPercentile: 75.25,
	}}
	out = captureFunc(t, func() { ok = printLeetCodeSubmit(a, p, run, false) })
	if !ok || !strings.Contains(out, "63/63 testcases passed · runtime 0 ms (beats 100.0%) · memory 4.3 MB (beats 75.2%)") {
		t.Errorf("accepted submission (%v):\n%s", ok, out)
	}

	run = &app.LeetCodeRun{Result: &leetcode.Result{
		StatusCode: 20, StatusMsg: "Compile Error", FullCompileError: "Line 3: undefined: x",
	}}
	out = captureFunc(t, func() { ok = printLeetCodeSubmit(a, p, run, false) })
	if ok || !strings.Contains(out, "Compile Error") || !strings.Contains(out, "Line 3: undefined: x") {
		t.Errorf("compile error (%v):\n%s", ok, out)
	}
}

// captureFunc returns what fn prints to standard output.
func captureFunc(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	fn()
	w.Close()
	os.Stdout = stdout
	return <-done
}

func TestLeetCodeCurrentLoginChoosesDestination(t *testing.T) {
	dir := packWorkspace(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	path, err := app.LeetCodeSessionsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"site":"cn","user":"alice","credentials":{"session":"current"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	root := &rootOptions{workspace: dir}
	defaultLogin, err := defaultSite(root, "")
	if err != nil || defaultLogin.Name != "com" {
		t.Fatalf("fixture default: %+v, %v", defaultLogin, err)
	}
	called := false
	err = withLeetCode(root, "regression/two-sum", &leetCodeOptions{lang: "go"}, func(a *app.App, p *problem.Problem, l lang.Language, source app.LeetCodeTarget, c *leetcode.Client) error {
		called = true
		if source.Site.Name != "com" || c.Site.Name != "cn" || c.Session.Token != "current" {
			t.Fatal("source/default overrode the current account")
		}
		return nil
	})
	if err != nil || !called {
		t.Fatalf("current login: %v", err)
	}
}
