package python

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/testenv"
)

// A solution recurses as deep as the driver's recursion limit lets it
// without raising the limit itself (#147): plain recursion 10^5 calls deep
// on every supported Python, and recursion through C code (functools.cache)
// as deep as the Python allows, which the large stack of the driver's thread
// makes 10^5 calls on Python 3.14 and later; 3.12 and 3.13 cap it at a
// fixed depth whatever the stack.
func TestDriverRecursionLimit(t *testing.T) {
	t.Parallel()
	py := testenv.LookPath(t, "python3")
	out, err := exec.Command(py, "-c", "import sys; print(int(sys.version_info >= (3, 14)))").Output()
	if err != nil {
		t.Fatal(err)
	}
	cacheDepth := 3000
	if strings.TrimSpace(string(out)) == "1" {
		cacheDepth = 100000
	}
	p := &problem.Problem{Slug: "depth", Kind: problem.KindFunction, Function: &problem.Function{
		Name:    "depth",
		Params:  []problem.Param{{Name: "n", Type: problem.TypeInteger}, {Name: "cached", Type: problem.TypeBoolean}},
		Returns: problem.TypeInteger,
	}}
	l := Language{Runtime: filepath.Join(t.TempDir(), "runtime")}
	dir := t.TempDir()
	solution := `import functools


def down(n):
    return 0 if n == 0 else down(n - 1) + 1


@functools.cache
def cached_down(n):
    return 0 if n == 0 else cached_down(n - 1) + 1


class Solution:
    def depth(self, n, cached):
        return cached_down(n) if cached else down(n)
`
	if err := os.WriteFile(filepath.Join(dir, solutionFile), []byte(solution), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := l.Prepare(dir, p); err != nil {
		t.Fatal(err)
	}
	cases := []map[string]any{
		{"id": 1, "input": map[string]any{"n": 100000, "cached": false}},
		{"id": 2, "input": map[string]any{"n": cacheDepth, "cached": true}},
	}
	data, _ := json.Marshal(cases)
	casesPath, outPath := filepath.Join(dir, "cases.json"), filepath.Join(dir, "out.jsonl")
	if err := os.WriteFile(casesPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(py, runnerFile)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), lang.EnvCases+"="+casesPath, lang.EnvOut+"="+outPath)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	lines, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"100000", strconv.Itoa(cacheDepth)}
	for i, line := range strings.Split(strings.TrimSpace(string(lines)), "\n") {
		var res struct {
			Output json.RawMessage `json:"output"`
			Error  string          `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &res); err != nil {
			t.Fatal(err)
		}
		if i >= len(want) || string(res.Output) != want[i] {
			t.Errorf("case %d: output %s, error %q", i+1, res.Output, res.Error)
		}
	}
}
