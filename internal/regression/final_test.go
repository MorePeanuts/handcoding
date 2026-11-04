package regression_test

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

var update = flag.Bool("update", false, "write the snapshot of the final protocol (TestFinalProtocol)")

// snapshotPath is the snapshot of the features of final protocol version v.
func snapshotPath(v int) string {
	return filepath.Join("testdata", fmt.Sprintf("protocol-%d.txt", v))
}

// TestFinalProtocol keeps a final protocol version from changing. A version
// is final once a stable hc release implements it (problem.FinalProtocol),
// and its features are then snapshotted in testdata/protocol-N.txt: the
// feature list TestProtocolCoverage derives from internal/problem, and the
// solution languages. While Protocol is final, the features must equal the
// snapshot, so a new feature needs a new, draft version. Once Protocol is a
// draft beyond it, the final version's features must still all be there,
// since removing one is an incompatible change.
//
// It catches features added or removed, not a feature whose meaning
// changes: that is left to the rule in CLAUDE.md and to review. When a
// version becomes final, write its snapshot with
//
//	go test ./internal/regression -run TestFinalProtocol -update
func TestFinalProtocol(t *testing.T) {
	final := problem.FinalProtocol
	if final > problem.Protocol {
		t.Fatalf("FinalProtocol %d is newer than Protocol %d", final, problem.Protocol)
	}
	if final == 0 || final < problem.MinProtocol {
		// No final version hc still reads.
		return
	}
	got := currentFeatures(t)
	path := snapshotPath(final)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(strings.Join(got, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("protocol %d is final but has no snapshot: write %s with -update", final, path)
	}
	if err != nil {
		t.Fatal(err)
	}
	snapshot := strings.Split(strings.TrimSpace(string(data)), "\n")
	added, removed := diff(got, snapshot)
	if len(removed) > 0 {
		t.Errorf("features of protocol %d, which is final, are gone (an incompatible change: see CLAUDE.md, \"Protocol versions\"):\n  %s",
			final, strings.Join(removed, "\n  "))
	}
	if problem.Protocol == final && len(added) > 0 {
		t.Errorf("protocol %d is final, but hc implements features it lacks: put them in protocol %d (bump problem.Protocol; see CLAUDE.md, \"Protocol versions\"):\n  %s",
			final, final+1, strings.Join(added, "\n  "))
	}
}

// currentFeatures lists, sorted, the features of the protocol hc
// implements.
func currentFeatures(t *testing.T) []string {
	fs := protocolFeatures(t, protocolConstants(t))
	for _, l := range problem.Languages(problem.Protocol) {
		fs = append(fs, "language "+l)
	}
	slices.Sort(fs)
	return fs
}

// diff returns the features of got that want lacks, and those of want that
// got lacks.
func diff(got, want []string) (added, removed []string) {
	for _, f := range got {
		if !slices.Contains(want, f) {
			added = append(added, f)
		}
	}
	for _, f := range want {
		if !slices.Contains(got, f) {
			removed = append(removed, f)
		}
	}
	return added, removed
}

// The feature list is what a snapshot holds, so it must be non-empty and
// have no duplicates, whether or not a version is final yet.
func TestProtocolFeatures(t *testing.T) {
	fs := currentFeatures(t)
	if len(fs) < 50 || len(slices.Compact(slices.Clone(fs))) != len(fs) {
		t.Fatalf("%d features, or duplicates: %q", len(fs), fs)
	}
	for _, want := range []string{"language go", "kind function", "judge.compare checker", field("Judge", "tolerance"), translation} {
		if !slices.Contains(fs, want) {
			t.Errorf("features lack %q", want)
		}
	}
	added, removed := diff([]string{"a", "b"}, []string{"b", "c"})
	if !slices.Equal(added, []string{"a"}) || !slices.Equal(removed, []string{"c"}) {
		t.Errorf("diff = %q, %q", added, removed)
	}
}
