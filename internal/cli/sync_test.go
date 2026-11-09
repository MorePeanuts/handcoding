package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/skill"
	"github.com/MorePeanuts/handcoding/internal/version"
)

// runningHC makes hc report v as its version for the rest of the test.
func runningHC(t *testing.T, v string) {
	t.Helper()
	saved := version.Version
	version.Version = v
	t.Cleanup(func() { version.Version = saved })
}

// A practice workspace is brought in step with a release of hc by any
// command that opens it, which says so on stderr, once.
func TestWorkspaceSyncOnOpen(t *testing.T) {
	fakeUV(t)
	ws := filepath.Join(t.TempDir(), "ws")
	if err := runHC(t, "init", ws); err != nil {
		t.Fatal(err)
	}
	agents := filepath.Join(ws, skill.AgentsFile)
	data, err := os.ReadFile(agents)
	if err != nil {
		t.Fatal(err)
	}
	block := strings.Index(string(data), "<!-- hc:end -->")
	if err := os.WriteFile(agents, []byte(string(data[:block])+"an older guide\n"+string(data[block:])), 0o644); err != nil {
		t.Fatal(err)
	}

	// A development build leaves it as it is.
	_, stderr, err := captureAll(t, "-w", ws, "list")
	if err != nil || strings.Contains(stderr, "updated") {
		t.Errorf("dev build: %v\n%s", err, stderr)
	}
	runningHC(t, "v0.1.0-alpha.11")
	stdout, stderr, err := captureAll(t, "-w", ws, "list")
	if err != nil || !strings.Contains(stderr, "updated the workspace for hc v0.1.0-alpha.11: AGENTS.md\n") || strings.Contains(stdout, "updated") {
		t.Errorf("release: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if data, _ := os.ReadFile(agents); strings.Contains(string(data), "an older guide") {
		t.Error("the block was not refreshed")
	}
	if _, stderr, err := captureAll(t, "-w", ws, "list"); err != nil || strings.Contains(stderr, "updated") {
		t.Errorf("second run: %v\n%s", err, stderr)
	}
}

// A pack checkout's skills are left as its pinned hc wrote them; hc
// validate and hc gen say once that they are another hc's.
func TestPackCheckoutSkillsHint(t *testing.T) {
	checkout := filepath.Join(t.TempDir(), "checkout")
	if err := regression.Copy(checkout); err != nil {
		t.Fatal(err)
	}
	opts := skill.Options{Version: "v0.1.0-alpha.10", Exclusive: true}
	if _, err := skill.Install(checkout, skill.Authoring, skill.Set(skill.Authoring), opts); err != nil {
		t.Fatal(err)
	}
	runningHC(t, "v0.1.0-alpha.11")
	const hint = "this pack's skills come from hc v0.1.0-alpha.10, not from this hc (v0.1.0-alpha.11)"
	agents, _ := os.ReadFile(filepath.Join(checkout, skill.AgentsFile))

	_, stderr, _ := captureAll(t, "-w", checkout, "list", "--plain")
	if strings.Contains(stderr, "pack's skills") || strings.Contains(stderr, "updated") {
		t.Errorf("hc list:\n%s", stderr)
	}
	_, stderr, _ = captureAll(t, "-w", checkout, "validate", "two-sum")
	if strings.Count(stderr, hint) != 1 {
		t.Errorf("hc validate:\n%s", stderr)
	}
	_, stderr, err := captureAll(t, "-w", checkout, "gen", "two-sum", "--dry-run")
	if err != nil || strings.Count(stderr, hint) != 1 {
		t.Errorf("hc gen: %v\n%s", err, stderr)
	}
	for _, in := range skill.List(checkout) {
		if in.State.Managed() && in.Version != "v0.1.0-alpha.10" {
			t.Errorf("%s: installed by %s", in.Dir, in.Version)
		}
	}
	if data, _ := os.ReadFile(filepath.Join(checkout, skill.AgentsFile)); string(data) != string(agents) {
		t.Errorf("AGENTS.md changed:\n%s", data)
	}
}
