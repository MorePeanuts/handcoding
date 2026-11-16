package runner_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/lang/cpp"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
)

// Re-exec the test binary as a worker so the runner sees real process exits,
// while deterministic elapsed times keep this check independent of machine load.
func TestIsolatedWorker(t *testing.T) {
	if os.Getenv("HC_TEST_ISOLATED_WORKER") != "1" {
		return
	}
	b, err := os.ReadFile(os.Getenv(lang.EnvCases))
	if err != nil {
		os.Exit(2)
	}
	var cs []struct {
		ID int `json:"id"`
	}
	if json.Unmarshal(b, &cs) != nil || len(cs) != 1 {
		os.Exit(3)
	}
	budget, err := strconv.ParseInt(os.Getenv(lang.EnvTimeLimit), 10, 64)
	// Each prior process reported exactly 40ns; budgets must not restart per case.
	if err != nil || budget != 100-int64(cs[0].ID-1)*40 {
		os.Exit(4)
	}
	result := struct {
		ID      int   `json:"id"`
		Output  int   `json:"output"`
		Elapsed int64 `json:"elapsed_ns"`
		Limited bool  `json:"time_limit"`
	}{cs[0].ID, 1, 40, budget < 40}
	b, _ = json.Marshal(result)
	if os.WriteFile(os.Getenv(lang.EnvOut), append(b, '\n'), 0o600) != nil {
		os.Exit(5)
	}
	os.Exit(0)
}

type isolatedLanguage struct {
	cpp.Language
	builds int
}

func (*isolatedLanguage) Prepare(string, *problem.Problem) error        { return nil }
func (l *isolatedLanguage) Build(context.Context, string, string) error { l.builds++; return nil }
func (*isolatedLanguage) Command(ctx context.Context, dir, work string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestIsolatedWorker$")
	cmd.Env = append(os.Environ(), "HC_TEST_ISOLATED_WORKER=1", "GORACE=atexit_sleep_ms=0")
	return cmd
}
func TestIsolatedCumulativeBudget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "reference"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "reference", "solution.cpp"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	p := &problem.Problem{Dir: dir, Kind: problem.KindFunction, Function: &problem.Function{Name: "f", Returns: "integer"}, Judge: problem.Judge{Compare: problem.CompareExact}}
	cs, err := problem.ParseCases(strings.NewReader(strings.Repeat("{\"input\":{},\"expected\":1}\n", 4)), "cases")
	if err != nil {
		t.Fatal(err)
	}
	selected := make([]runner.Selected, len(cs))
	for i, c := range cs {
		selected[i] = runner.Selected{Index: i + 1, Case: c}
	}
	l := &isolatedLanguage{}
	rep, err := runner.Run(context.Background(), l, p, dir, selected, runner.Options{TimeLimit: 100 * time.Nanosecond})
	if err != nil {
		t.Fatal(err)
	}
	if l.builds != 1 || rep.Passed != 2 || rep.Cases[2].Verdict != runner.TimeLimit || rep.Cases[3].Verdict != runner.NotRun {
		t.Fatalf("builds=%d report=%+v cases=%+v", l.builds, rep, rep.Cases)
	}
}
