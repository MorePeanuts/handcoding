package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
