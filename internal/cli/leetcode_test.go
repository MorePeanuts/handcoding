package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
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

	// logout forgets the session of --site, else every session, whatever
	// leetcode.site says.
	path, err := app.LeetCodeSessionsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"cn": {"session": "s"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--site", "com"}, "Not logged in to leetcode.com (logged in to leetcode.cn)."},
		{nil, "logged out of leetcode.cn"},
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
		"regression/two-sum": "not logged in to leetcode.com; run `hc leetcode login --site com`",
	} {
		for _, sub := range []string{"test", "submit"} {
			if _, err := capture(t, "-w", dir, "leetcode", sub, ref, "-l", "go"); err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("leetcode %s %s: err = %v, want %q", sub, ref, err, want)
			}
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
