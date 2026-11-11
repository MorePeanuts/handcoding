package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/testenv"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func TestCasesAddGenerate(t *testing.T) {
	dir := packWorkspace(t)
	casesFile := filepath.Join(dir, "packs", "local", "problems", "two-sum", "cases.jsonl")
	before, _ := os.ReadFile(casesFile)
	for _, tt := range []struct {
		args []string
		err  string
	}{
		{[]string{"--generate", "big", "--input", `{"nums":[1],"target":1}`}, "either --input or --generate"},
		{nil, "either --input or --generate"},
		{[]string{"--generate", "big", "--expected", "[0,1]"}, "no --expected"},
		{[]string{"--generate", "big", "--example"}, "no --expected and cannot be an --example"},
		{[]string{"--generate", "big", "--args", "[1]"}, "--args must be a JSON object"},
		{[]string{"--generate", "big-one"}, `"generate.name" "big-one" must name a function`},
		{[]string{"--input", `{"nums":[1],"target":1}`, "--seed", "3"}, "--args and --seed go with --generate"},
	} {
		_, err := capture(t, append([]string{"-w", dir, "cases", "add", "local/two-sum"}, tt.args...)...)
		if err == nil || !strings.Contains(err.Error(), tt.err) {
			t.Errorf("cases add %q: error %v, want %q", tt.args, err, tt.err)
		}
	}
	if after, _ := os.ReadFile(casesFile); string(after) != string(before) {
		t.Fatalf("a refused cases add changed cases.jsonl:\n%s", after)
	}

	out, err := capture(t, "-w", dir, "cases", "add", "local/two-sum", "--generate", "big", "--args", `{"n": 100000}`, "--seed", "7", "--note", "large")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "no digest and no expected output (no reference solution)") {
		t.Errorf("cases add --generate without a reference:\n%s", out)
	}
	data, _ := os.ReadFile(casesFile)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if got, want := lines[len(lines)-1], `{"generate":{"name":"big","args":{"n":100000},"seed":7},"note":"large"}`; got != want {
		t.Errorf("added line %s, want %s", got, want)
	}
	out, err = capture(t, "-w", dir, "cases", "list", "local/two-sum", "-c", fmt.Sprint(len(lines)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "big(n=100000) seed 7") || !strings.Contains(out, "missing — run hc cases build") {
		t.Errorf("cases list of a generated case:\n%s", out)
	}
	_, err = capture(t, "-w", dir, "validate", "local/two-sum")
	if err == nil {
		t.Error("validate passes a generated case without generator.py")
	}
}

// TestUserCasesCommands checks hc cases add, list and rm on a problem of an
// installed pack: they work on the user's own cases.
func TestUserCasesCommands(t *testing.T) {
	testenv.LookPath(t, "go")
	testenv.LookPath(t, "python3")
	// Not packWorkspace, which leaves only a fake uv on PATH: the
	// references run.
	dir := t.TempDir()
	ws, err := workspace.Create(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.Install(ws); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, workspace.ConfigFile)
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, append(data, "python:\n  interpreter: python3\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	ref := "regression/two-sum"
	for _, tt := range []struct {
		args []string
		err  string
	}{
		{[]string{"--generate", "distinct"}, "cannot be generated"},
		{[]string{"--input", `{"nums":[1,2],"target":3}`, "--example"}, "cannot be examples"},
	} {
		_, err := capture(t, append([]string{"-w", dir, "cases", "add", ref}, tt.args...)...)
		if err == nil || !strings.Contains(err.Error(), tt.err) {
			t.Errorf("cases add %q: error %v, want %q", tt.args, err, tt.err)
		}
	}
	out, err := capture(t, "-w", dir, "cases", "add", ref, "--input", `{"nums":[1,5,3],"target":8}`, "--note", "mine")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "added your case") || !strings.Contains(out, "[1,2]") || !strings.Contains(out, "hc submit does not") {
		t.Errorf("cases add:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "cases", "regression", "two-sum.jsonl")); err != nil {
		t.Fatal(err)
	}
	out, err = capture(t, "-w", dir, "cases", "list", ref)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "yours (mine)") || !strings.Contains(out, "computed by the reference solution when it runs") || !strings.Contains(out, "and 1 of yours") {
		t.Errorf("cases list:\n%s", out)
	}
	if err := runHC(t, "-w", dir, "cases", "rm", ref, "1"); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Errorf("cases rm of the pack's case: %v", err)
	}
	n := strings.Count(out, "Case ")
	out, err = capture(t, "-w", dir, "cases", "rm", ref, fmt.Sprint(n))
	if err != nil || !strings.Contains(out, "0 of yours left") {
		t.Errorf("cases rm %d: %v\n%s", n, err, out)
	}
}
