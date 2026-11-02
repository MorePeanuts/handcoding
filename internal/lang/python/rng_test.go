package python

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// splitmix64 is the generator Rng implements, written independently.
func splitmix64(seed uint64, n int) []uint64 {
	out := make([]uint64, n)
	for i := range out {
		seed += 0x9E3779B97F4A7C15
		z := seed
		z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
		z = (z ^ (z >> 27)) * 0x94D049BB133111EB
		out[i] = z ^ (z >> 31)
	}
	return out
}

// TestRng pins the numbers hc_runtime's Rng gives for a seed: generated
// cases are recorded by the digests of these numbers, so any change to Rng
// changes every pack's generated inputs.
func TestRng(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not installed")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, runtimeFile), runtimeSrc, 0o644); err != nil {
		t.Fatal(err)
	}
	script := `from hc_runtime import Rng
r = Rng(7)
print(" ".join(str(r._next()) for _ in range(3)))
r = Rng(-1)
print([r.randint(1, 6) for _ in range(8)], r.randint(0, 2**100), repr(r.random()), repr(r.uniform(-1, 1)))
x = list(range(10))
r.shuffle(x)
print(x, r.choice("abc"), r.sample(range(10**9), 3), r.sample([1, 2, 3, 4], 4))
`
	cmd := exec.Command(py, "-c", script)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var want []string
	for _, v := range splitmix64(7, 3) {
		want = append(want, strconv.FormatUint(v, 10))
	}
	if lines[0] != strings.Join(want, " ") {
		t.Errorf("Rng(7) gives %s, splitmix64 %s", lines[0], strings.Join(want, " "))
	}
	pinned := []string{
		"[3, 4, 2, 1, 1, 2, 2, 3] 640457526264113274651329528748 0.014437948846938942 0.612956204835444",
		"[0, 9, 8, 3, 6, 7, 2, 1, 4, 5] a [919476739, 566143790, 353373922] [3, 4, 2, 1]",
	}
	for i, w := range pinned {
		if lines[i+1] != w {
			t.Errorf("Rng(-1) line %d:\n got %s\nwant %s", i+1, lines[i+1], w)
		}
	}
}
