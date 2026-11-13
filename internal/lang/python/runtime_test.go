package python

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/testenv"
)

// Prepare writes only the runner next to the solution, which imports
// hc_runtime from the shared runtime directory; it removes the copy of
// hc_runtime.py an older hc wrote there.
func TestSharedRuntime(t *testing.T) {
	t.Parallel()
	py := testenv.LookPath(t, "python3")
	rt := filepath.Join(t.TempDir(), "runtime")
	l := Language{Runtime: rt}
	p := &problem.Problem{Slug: "add", Kind: problem.KindFunction, Function: &problem.Function{
		Name:    "add",
		Params:  []problem.Param{{Name: "a", Type: problem.TypeInteger}, {Name: "b", Type: problem.TypeInteger}},
		Returns: problem.TypeInteger,
	}}
	dir := t.TempDir()
	files := map[string]string{
		solutionFile: "from hc_runtime import ListNode  # noqa: F401\n\n\nclass Solution:\n    def add(self, a, b):\n        return a + b\n",
		runtimeFile:  "raise SystemExit('the stale copy was imported')\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Prepare(dir, p); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, " ") != runnerFile+" "+solutionFile {
		t.Errorf("solution directory: %v", names)
	}
	for _, name := range []string{runtimeFile, checkerFile, genFile} {
		if _, err := os.Stat(filepath.Join(rt, runtimeHash, name)); err != nil {
			t.Errorf("runtime: %v", err)
		}
	}
	if data, err := os.ReadFile(filepath.Join(rt, runtimeFile)); err != nil || string(data) != string(runtimeSrc) {
		t.Errorf("editor copy: %v", err)
	}

	cases, out := filepath.Join(dir, "cases.json"), filepath.Join(dir, "out.jsonl")
	if err := os.WriteFile(cases, []byte(`[{"id":1,"input":{"a":2,"b":3}}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(py, runnerFile)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), lang.EnvCases+"="+cases, lang.EnvOut+"="+out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	var res struct {
		Output json.RawMessage `json:"output"`
	}
	if data, _ := os.ReadFile(out); json.Unmarshal(data, &res) != nil || string(res.Output) != "5" {
		t.Errorf("output: %s", data)
	}

	// A second hc of the same runtime finds it in place; one of another
	// runtime (a runtime directory written by hand) is left alone.
	other := filepath.Join(rt, "0123456789ab")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if d, err := l.runtimeDir(); err != nil || d != filepath.Join(rt, runtimeHash) {
		t.Errorf("runtimeDir: %s, %v", d, err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Error("removed another hc's runtime")
	}
}

// The environment puts the runtime directory on sys.path, and the
// pyrightconfig.json and ty.toml it writes list it, so that editors resolve
// hc_runtime; an existing configuration is the user's.
func TestVenvRuntimePath(t *testing.T) {
	f := newFakeEnv(t)
	v := f.existing("3.12.4")
	v.Runtime = filepath.Join(f.root, ".hc", "runtime", "python")
	v.Ty = filepath.Join(f.root, "ty.toml")
	site := filepath.Join(v.Dir, "lib", "python3.12", "site-packages")
	if err := os.MkdirAll(site, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := v.Ensure(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(site, PthFile)); err != nil || string(data) != v.Runtime+"\n" {
		t.Errorf("%s: %q, %v", PthFile, data, err)
	}
	var config struct {
		ExtraPaths []string `json:"extraPaths"`
	}
	if data, err := os.ReadFile(v.Pyright); err != nil || json.Unmarshal(data, &config) != nil || strings.Join(config.ExtraPaths, ",") != ".hc/runtime/python" {
		t.Errorf("pyrightconfig.json: %s, %v", data, err)
	}
	want := "[environment]\npython = \".hc/venv\"\nextra-paths = [\".hc/runtime/python\"]\n"
	if data, err := os.ReadFile(v.Ty); err != nil || !strings.HasSuffix(string(data), want) {
		t.Errorf("ty.toml: %q, %v", data, err)
	}

	// The user's own configuration stays as it is.
	f = newFakeEnv(t)
	v = f.existing("3.12.4")
	v.Runtime, v.Ty = filepath.Join(f.root, ".hc", "runtime", "python"), filepath.Join(f.root, "ty.toml")
	for _, p := range []string{v.Ty, v.Pyright} {
		if err := os.WriteFile(p, []byte("# mine\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := v.Ensure(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{v.Ty, v.Pyright} {
		if data, _ := os.ReadFile(p); string(data) != "# mine\n" {
			t.Errorf("%s rewritten: %q", filepath.Base(p), data)
		}
	}
}

// Cleanup can leave bytecode directories and user-owned files. Concurrent
// callers repair only the generated sources, preserving those entries.
func TestRuntimeRepair(t *testing.T) {
	t.Parallel()
	l := Language{Runtime: t.TempDir()}
	dir := filepath.Join(l.Runtime, runtimeHash)
	if err := os.MkdirAll(filepath.Join(dir, "__pycache__"), 0o755); err != nil {
		t.Fatal(err)
	}
	note := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(note, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := l.runtimeDir(); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	for name, want := range map[string][]byte{runtimeFile: runtimeSrc, checkerFile: checkerSrc, genFile: generatorSrc} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != string(want) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if got, err := os.ReadFile(note); err != nil || string(got) != "keep" {
		t.Errorf("user file: %q %v", got, err)
	}
}

func TestRuntimeRepairRefusesSymlink(t *testing.T) {
	t.Parallel()
	l := Language{Runtime: t.TempDir()}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(l.Runtime, runtimeHash)); err != nil {
		t.Fatal(err)
	}
	if _, err := l.runtimeDir(); err == nil {
		t.Fatal("repaired a symlinked runtime")
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatalf("wrote through symlink: %v %v", entries, err)
	}
}
