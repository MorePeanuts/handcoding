package python

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/testenv"
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
	py := testenv.LookPath(t, "python3")
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

// rngOracle is Rng as it was first written, drawing one number at a time:
// the numbers every faster Rng must give.
const rngOracle = `_MASK64 = (1 << 64) - 1


class Oracle:
    def __init__(self, seed=0):
        self._state = seed & _MASK64

    def _next(self):
        self._state = (self._state + 0x9E3779B97F4A7C15) & _MASK64
        z = self._state
        z = ((z ^ (z >> 30)) * 0xBF58476D1CE4E5B9) & _MASK64
        z = ((z ^ (z >> 27)) * 0x94D049BB133111EB) & _MASK64
        return z ^ (z >> 31)

    def _below(self, n):
        if n <= 0:
            raise ValueError("empty range")
        words = (n.bit_length() + 63) // 64
        span = 1 << (64 * words)
        limit = span - span % n
        while True:
            r = 0
            for _ in range(words):
                r = (r << 64) | self._next()
            if r < limit:
                return r % n

    def random(self):
        return (self._next() >> 11) * (1.0 / (1 << 53))

    def randint(self, a, b):
        if not isinstance(a, int) or not isinstance(b, int):
            raise TypeError("randint takes integers")
        if a > b:
            raise ValueError("randint(%d, %d): empty range" % (a, b))
        return a + self._below(b - a + 1)

    def uniform(self, a, b):
        return a + (b - a) * self.random()

    def choice(self, seq):
        if len(seq) == 0:
            raise IndexError("cannot choose from an empty sequence")
        return seq[self._below(len(seq))]

    def shuffle(self, x):
        for i in range(len(x) - 1, 0, -1):
            j = self._below(i + 1)
            x[i], x[j] = x[j], x[i]

    def sample(self, population, k):
        n = len(population)
        if not 0 <= k <= n:
            raise ValueError("sample larger than population or negative")
        if 3 * k <= n:
            picked, out = set(), []
            while len(out) < k:
                j = self._below(n)
                if j not in picked:
                    picked.add(j)
                    out.append(population[j])
            return out
        pool = list(population)
        for i in range(k):
            j = i + self._below(n - i)
            pool[i], pool[j] = pool[j], pool[i]
        return pool[:k]
`

// rngEquivalence makes the same calls to Rng and to the oracle, from many
// seeds: every method, on many ranges (ranges where a draw is likely to be
// rejected, of several words, calls across batches), errors included. It
// compares every result, the arguments after the call (shuffle), the error
// raised and the state after the call, and prints how many calls it made.
const rngEquivalence = `import random
import sys

from hc_runtime import Rng
from rng_oracle import Oracle

P = 1 << 64
ranges = sorted({1, 2, 3, 5, 7, 10, 26, 100, 1000, 10**6, 10**9, 10**18,
                 3 * 2**62, 2**63 - 1, 2**63, 2**63 + 1, P // 3 + 1,
                 P - 2, P - 1, P, P + 1, 3 * P, 2**100, 2**128 - 1, 2**128, 2**128 + 1,
                 10**40, 2**192 + 12345}
                | {2**k + d for k in range(1, 131) for d in (-1, 0, 1)})
meta = random.Random(126)
seeds = [0, 1, -1, 7, 2**63, P - 1, P, P + 12345, -(2**70), 2**200 + 3]
seeds += [meta.getrandbits(80) - 2**79 for _ in range(14)]


def outcome(rng, name, args):
    args = [list(a) if isinstance(a, list) else a for a in args]
    try:
        result = getattr(rng, name)(*args)
    except Exception as e:
        result = ("raised", type(e).__name__, str(e))
    return repr(result), repr(args), rng._state


class Odd:
    """A sequence whose item 3 cannot be read."""

    def __len__(self):
        return 5

    def __getitem__(self, i):
        if i == 3:
            raise KeyError(i)
        return i


def calls():
    out = []
    for n in ranges:
        out += [("_below", (n,)), ("randint", (0, n - 1)), ("randint", (-n, n)),
                ("randint", (-(10**30), -(10**30) + n - 1))]
    out += [("_below", (0,)), ("_below", (-5,)), ("randint", (3, 2)), ("randint", (1.0, 2)),
            ("randint", (1, "2")), ("randint", (True, 5)), ("randint", (5, 5)),
            ("randint", (-(2**70), 2**70))]
    out += [("random", ()) for _ in range(50)] + [("_next", ()) for _ in range(50)]
    out += [("uniform", (-1, 1)), ("uniform", (0.5, 0.25)), ("uniform", (3, 3)),
            ("uniform", (-(10**20), 10**20)), ("uniform", ("a", "b"))]
    for n in [0, 1, 2, 3, 26, 255, 256, 257, 1000]:
        out += [("choice", (list(range(n)),)), ("choice", ("x" * n,)), ("choice", (range(n),))]
    # A length of 2**62 + 1 rejects a quarter of the draws, 6 * 10**18 one in 41.
    out += [("choice", (range(10**18),)), ("choice", (range(2**62 + 1),)), ("choice", (range(6 * 10**18),)),
            ("choice", (Odd(),)), ("choice", ({},))]
    for n in [0, 1, 2, 3, 10, 100, 255, 256, 257, 600]:
        out += [("shuffle", (list(range(n)),))]
    out += [("shuffle", ((1, 2, 3),)), ("shuffle", (5,))]
    for n in [0, 1, 2, 3, 9, 10, 30, 300, 1000]:
        for k in sorted({0, 1, n // 3, n // 3 + 1, n // 2, n - 1, n, n + 1, -1}):
            out += [("sample", (range(n), k)), ("sample", (list(range(n)), k))]
    out += [("sample", (range(10**9), 1000)), ("sample", (range(10**18), 50)),
            ("sample", (range(2**62 + 1), 50)), ("sample", (range(6 * 10**18), 50)),
            ("sample", ("abcdefghij", 4)), ("sample", (range(10), 2.5)), ("sample", (range(10), 1.5)),
            ("sample", (Odd(), 1)), ("sample", (Odd(), 5)), ("sample", (5, 1))]
    return out


def same(seed, name, args, new, old, i):
    got, want = outcome(new, name, args), outcome(old, name, args)
    if got != want:
        sys.exit("Rng(%r).%s%r, call %d:\n got %r\nwant %r" % (seed, name, args, i, got, want))


base = calls()
made = 0
for seed in seeds:
    for _ in range(3):
        seq = list(base)
        meta.shuffle(seq)
        new, old = Rng(seed), Oracle(seed)
        if new._state != old._state:
            sys.exit("Rng(%r): state %r, oracle %r" % (seed, new._state, old._state))
        for i, (name, args) in enumerate(seq):
            same(seed, name, args, new, old, i)
            made += 1
# Long runs of one method, across many batches.
for seed in seeds[:8]:
    for name, args in [("randint", (1, 6)), ("randint", (0, 3 * 2**62)), ("random", ()),
                       ("choice", ("abc",)), ("_next", ())]:
        new, old = Rng(seed), Oracle(seed)
        for i in range(3000):
            same(seed, name, args, new, old, i)
            made += 1
    for name, args in [("sample", (range(10**9), 20000)), ("sample", (range(3000), 2000)),
                       ("shuffle", (list(range(5000)),))]:
        new, old = Rng(seed), Oracle(seed)
        same(seed, name, args, new, old, 0)
        made += 1
print(made)
`

// TestRngEquivalence checks that Rng gives exactly the numbers its plain
// implementation (rngOracle) gives, method by method: Rng is written for
// speed, and a number it changes changes the generated inputs, and so the
// digests, of every pack.
func TestRngEquivalence(t *testing.T) {
	py := testenv.LookPath(t, "python3")
	dir := t.TempDir()
	files := map[string]string{runtimeFile: string(runtimeSrc), "rng_oracle.py": rngOracle, "check.py": rngEquivalence}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(py, "check.py")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if n, err := strconv.Atoi(strings.TrimSpace(string(out))); err != nil || n < 10000 {
		t.Fatalf("made %s calls, want at least 10000", out)
	}
}
