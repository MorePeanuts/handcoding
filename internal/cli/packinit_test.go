package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/skill"
)

// TestPackInit scaffolds a pack, checks it, and lints the hc commands its
// workflow runs and its agent files cite against the command tree, as
// TestSkillsLint does for the skills: CI and agents run them literally.
func TestPackInit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := filepath.Join(t.TempDir(), "my-pack")
	out, err := capture(t, "pack", "init", dir, "--title", "Mine")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Created the pack my-pack", app.WorkflowFile, "git init", "hc pack check"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if err := runHC(t, "pack", "check", dir); err != nil {
		t.Errorf("hc pack check: %v", err)
	}
	if err := runHC(t, "pack", "init", dir); err == nil || !strings.Contains(err.Error(), "already holds pack.yaml") {
		t.Errorf("second init: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(app.WorkflowFile)))
	if err != nil {
		t.Fatal(err)
	}
	errs, n := lintWorkflow(newRootCmd(), data)
	for _, err := range errs {
		t.Errorf("%s: %v", app.WorkflowFile, err)
	}
	if n < 3 {
		t.Errorf("%s: only %d hc commands found", app.WorkflowFile, n)
	}
	files := map[string][]byte{}
	for _, f := range []string{skill.AgentsFile, "CLAUDE.md"} {
		if files[f], err = os.ReadFile(filepath.Join(dir, f)); err != nil {
			t.Fatal(err)
		}
	}
	for _, err := range lintSkill(newRootCmd(), skill.Skill{Name: "pack init", Files: files}) {
		t.Errorf("agent files: %v", err)
	}
}

func TestLintWorkflowCatchesStaleCommands(t *testing.T) {
	wf := "# Run `hc pack verify` first.\njobs:\n  check:\n    steps:\n      - run: hc pack check --strict\n      - run: |\n          hc skill install && git diff\n"
	errs, n := lintWorkflow(newRootCmd(), []byte(wf))
	var got []string
	for _, err := range errs {
		got = append(got, err.Error())
	}
	want := "`hc pack check --strict`: hc pack check has no flag --strict\n`hc pack verify`: hc pack has no subcommand verify"
	if strings.Join(got, "\n") != want || n != 3 {
		t.Errorf("%d commands, got\n%s\nwant\n%s", n, strings.Join(got, "\n"), want)
	}
}

// lintWorkflow checks the hc command lines of a GitHub Actions workflow: its
// steps' run scripts, and the code spans of its comments. It returns the
// errors and the number of hc commands it checked.
func lintWorkflow(root *cobra.Command, data []byte) ([]error, int) {
	var wf struct {
		Jobs map[string]struct {
			Steps []struct {
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return []error{err}, 0
	}
	var lines []string
	for _, job := range wf.Jobs {
		for _, s := range job.Steps {
			lines = append(lines, strings.Split(s.Run, "\n")...)
		}
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if comment, ok := strings.CutPrefix(strings.TrimSpace(line), "#"); ok {
			for _, m := range codeSpan.FindAllStringSubmatch(comment, -1) {
				lines = append(lines, m[1])
			}
		}
	}
	var (
		errs []error
		n    int
	)
	for _, line := range lines {
		for _, part := range strings.FieldsFunc(line, func(r rune) bool { return r == ';' || r == '|' || r == '&' }) {
			args := shellFields(strings.TrimSpace(part))
			if len(args) == 0 || args[0] != "hc" {
				continue
			}
			n++
			if err := checkCommand(root, args[1:]); err != nil {
				errs = append(errs, fmt.Errorf("`%s`: %w", strings.Join(args, " "), err))
			}
		}
	}
	return errs, n
}
