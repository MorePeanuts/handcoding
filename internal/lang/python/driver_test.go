package python

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/testenv"
)

// decode and encode build a codec per type once; it must do what
// dispatching on the type for every value did.
func TestRuntimeCodecs(t *testing.T) {
	py := testenv.LookPath(t, "python3")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, runtimeFile), runtimeSrc, 0o644); err != nil {
		t.Fatal(err)
	}
	script := `import hc_runtime
from hc_runtime import decode, encode

v = [[1, 2], [3]]
d = decode("integer[][]", v)
assert d == v and d is not v and d[0] is not v[0], d
assert decode("integer", "x") == "x"
d = decode("double[]", [1, 2])
assert d == [1.0, 2.0] and all(type(x) is float for x in d), d
for t, v, msg in [("integer[]", "x", "expected array for integer[], got 'x'"),
                  ("integer[][]", [1], "expected array for integer[], got 1")]:
    try:
        decode(t, v)
        raise AssertionError("no error for %r" % (v,))
    except hc_runtime.InputError as e:
        assert str(e) == msg, str(e)
assert encode("integer[]", None) == []
assert encode("integer[]", (1, 2)) == [1, 2]
assert encode("string[]", "ab") == ["a", "b"]
assert encode("double", None) is None
assert encode("double", 2) == 2.0 and type(encode("double", 2)) is float
assert encode("double[]", [1, None]) == [1.0, None]
assert encode("integer", "x") == "x"
assert encode("ListNode[]", [decode("ListNode", [1, 2]), None]) == [[1, 2], []]
assert encode("TreeNode", decode("TreeNode", [1, None, 2])) == [1, None, 2]
assert encode("NestedInteger[]", decode("NestedInteger[]", [1, [2, [3]]])) == [1, [2, [3]]]
try:
    encode("integer[]", 5)
    raise AssertionError("no error")
except TypeError as e:
    assert str(e) == "'int' object is not iterable", str(e)
print("ok")
`
	cmd := exec.Command(py, "-c", script)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil || strings.TrimSpace(string(out)) != "ok" {
		t.Fatalf("%v\n%s", err, out)
	}
}

// The class driver looks up what a call needs once per method: a method is
// still looked up on its first call, so that one the solution lacks fails
// there, and arguments are still checked and decoded on every call.
func TestClassDriver(t *testing.T) {
	py := testenv.LookPath(t, "python3")
	p := &problem.Problem{Slug: "f", Kind: problem.KindClass, Class: &problem.Class{
		Name:        "Box",
		Constructor: []problem.Param{{Name: "start", Type: "double"}},
		Methods: []problem.Method{
			{Name: "add", Params: []problem.Param{{Name: "x", Type: problem.TypeInteger}, {Name: "nums", Type: "double[]"}}, Returns: problem.TypeVoid},
			{Name: "total", Returns: problem.TypeDouble},
			{Name: "items", Returns: "integer[]"},
			{Name: "name", Returns: problem.TypeString},
			{Name: "gone", Returns: problem.TypeInteger},
		},
	}}
	dir := t.TempDir()
	solution := `class Box:
    def __init__(self, start):
        assert type(start) is float
        self.vals = [start]

    def add(self, x, nums):
        assert type(x) is int and all(type(n) is float for n in nums)
        self.vals += [x] + nums

    def total(self):
        return int(sum(self.vals))

    def items(self):
        return (int(v) for v in self.vals)

    def name(self):
        return "é<"
`
	if err := os.WriteFile(filepath.Join(dir, solutionFile), []byte(solution), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (Language{}).Prepare(dir, p); err != nil {
		t.Fatal(err)
	}
	cases := `[{"id":1,"input":{"ops":["Box","add","total","items","name","add"],"args":[[1],[2,[3,4]],[],[],[],[5,[]]]}},
{"id":2,"input":{"ops":["Box","total","gone"],"args":[[0],[],[]]}},
{"id":3,"input":{"ops":["Box","add"],"args":[[0],[1]]}},
{"id":4,"input":{"ops":["Box","pop"],"args":[[0],[]]}},
{"id":5,"input":{"ops":["Box"],"args":[[]]}}]`
	casesPath, outPath := filepath.Join(dir, "cases.json"), filepath.Join(dir, "out.jsonl")
	if err := os.WriteFile(casesPath, []byte(cases), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(py, runnerFile)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), lang.EnvCases+"="+casesPath, lang.EnvOut+"="+outPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	want := []string{
		`{"id": 1, "output": [null, null, 10.0, [1, 2, 3, 4], "é<", null], "elapsed_ns": `,
		`{"id": 2, "output": null, "elapsed_ns": `,
		`{"id": 3, "output": null, "elapsed_ns": `,
		`{"id": 4, "output": null, "elapsed_ns": `,
		`{"id": 5, "output": null, "elapsed_ns": `,
	}
	errs := []string{
		``,
		`"error": "AttributeError: 'Box' object has no attribute 'gone'"}`,
		`"error": "invalid input: expected 2 arguments, got [1]"}`,
		`"error": "invalid input: unknown method 'pop'"}`,
		`"error": "invalid input: expected 1 arguments, got []"}`,
	}
	if len(lines) != len(want) {
		t.Fatalf("%d result lines, want %d:\n%s", len(lines), len(want), data)
	}
	for i, line := range lines {
		if !strings.HasPrefix(line, want[i]) || !strings.HasSuffix(line, errs[i]) {
			t.Errorf("line %d = %s\nwant %s…%s", i+1, line, want[i], errs[i])
		}
	}
}
