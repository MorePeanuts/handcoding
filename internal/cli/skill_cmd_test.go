package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/skill"
)

func installed(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, skill.Path(name)))
	return err == nil
}

// A practice workspace and a pack checkout get their own skill sets, and
// `hc skill install` without names brings a workspace back to its set.
func TestSkillSetsByContext(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	practice := filepath.Join(t.TempDir(), "ws")
	if err := runHC(t, "init", practice); err != nil {
		t.Fatal(err)
	}
	if installed(practice, skill.Pack) {
		t.Error("init installed hc-pack in a practice workspace")
	}
	out, err := capture(t, "-w", practice, "skill", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "This is a practice workspace; it gets hc-problem, hc-cases.\n") {
		t.Errorf("list in a practice workspace:\n%s", out)
	}
	for _, line := range []string{
		"hc-cases    yes  .claude/skills  up to date",
		"hc-pack     no   .claude/skills  not installed",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("list lacks %q:\n%s", line, out)
		}
	}

	// A skill installed by name stays until the next hc skill install.
	if err := runHC(t, "-w", practice, "skill", "install", skill.Pack); err != nil {
		t.Fatal(err)
	}
	if !installed(practice, skill.Pack) {
		t.Fatal("hc skill install hc-pack did not install it")
	}
	// hc gen only adds the set's skills; it keeps one installed by name.
	if err := runHC(t, "-w", practice, "new", "demo"); err != nil {
		t.Fatal(err)
	}
	if _, err := capture(t, "-w", practice, "gen", "demo", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if !installed(practice, skill.Pack) {
		t.Fatal("hc gen removed hc-pack, which was installed by name")
	}
	if out, err := capture(t, "-w", practice, "skill", "install"); err != nil || !strings.Contains(out, "removed "+skill.ClaudeDir+"/"+skill.Pack) {
		t.Errorf("hc skill install: %v\n%s", err, out)
	}
	if installed(practice, skill.Pack) {
		t.Error("hc skill install kept hc-pack in a practice workspace")
	}

	checkout := filepath.Join(t.TempDir(), "checkout")
	if err := regression.Copy(checkout); err != nil {
		t.Fatal(err)
	}
	// hc gen syncs the checkout's set before it starts the agent.
	if _, err := capture(t, "-w", checkout, "gen", "two-sum", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	for _, name := range skill.Set(skill.Authoring) {
		if !installed(checkout, name) {
			t.Errorf("gen in a pack checkout did not install %s", name)
		}
	}
	agents, err := os.ReadFile(filepath.Join(checkout, skill.AgentsFile))
	if err != nil || !strings.Contains(string(agents), "`"+skill.Pack+"`") {
		t.Errorf("AGENTS.md does not list hc-pack: %v\n%s", err, agents)
	}
	out, err = capture(t, "-w", checkout, "skill", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "This is a pack checkout; it gets hc-problem, hc-cases, hc-pack.\n") || !strings.Contains(out, "hc-pack     yes  .claude/skills  up to date") {
		t.Errorf("list in a pack checkout:\n%s", out)
	}
}
