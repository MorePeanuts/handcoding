package app_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/skill"
	"github.com/MorePeanuts/handcoding/internal/testenv"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// workspace.GeneratedFiles, which a practice workspace's .gitignore leaves
// out, are what the drivers write next to a solution: every file Prepare
// writes for any problem of the regression set is listed, and every file
// listed is written (but __pycache__/, which Python writes when it runs,
// and the LegacyFiles of older hc versions).
// With git on PATH, git itself ignores them, and not the solutions.
func TestGitignoreCoversGenerated(t *testing.T) {
	t.Parallel()
	a := newWorkspace(t)
	problems, err := a.WS.Problems()
	if err != nil {
		t.Fatal(err)
	}
	written := map[string]bool{}
	var generated, solutions []string
	for _, l := range a.Langs.All() {
		for _, p := range problems {
			path, _, err := a.Start(p, l)
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Dir(path)
			if err := l.Prepare(dir, p); err != nil {
				t.Fatalf("%s %s: %v", l.ID(), p.Slug, err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			rel, _ := filepath.Rel(a.WS.Root, dir)
			solutions = append(solutions, filepath.Join(rel, l.SolutionFile()))
			for _, e := range entries {
				name := e.Name()
				if name == l.SolutionFile() {
					continue
				}
				if e.IsDir() {
					name += "/"
				}
				written[name] = true
				if !slices.Contains(workspace.GeneratedFiles, name) {
					t.Errorf("%s's driver writes %s for %s, which .gitignore does not leave out", l.ID(), name, p.Slug)
				}
				generated = append(generated, filepath.Join(rel, e.Name()))
			}
		}
	}
	for _, name := range workspace.GeneratedFiles {
		if !written[name] && name != "__pycache__/" && !slices.Contains(workspace.LegacyFiles, name) {
			t.Errorf("no driver writes %s, which .gitignore leaves out", name)
		}
	}

	// Skips without git, after the checks above.
	testenv.LookPath(t, "git")
	git := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = a.WS.Root
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.Output()
		return string(out), err
	}
	if _, err := git("init", "-q"); err != nil {
		t.Fatal(err)
	}
	out, _ := git(append([]string{"check-ignore", "--no-index", "--"}, generated...)...)
	if got := strings.Fields(out); len(got) != len(generated) {
		t.Errorf("git ignores %d of the %d generated files: %v", len(got), len(generated), got)
	}
	if out, _ := git(append([]string{"check-ignore", "--no-index", "--"}, solutions...)...); out != "" {
		t.Errorf("git ignores solutions: %s", out)
	}
}

// An outdated block of .gitignore is refreshed when a release of hc opens
// the workspace, keeping the user's lines.
func TestSyncWorkspaceIgnore(t *testing.T) {
	t.Parallel()
	a := skillsWorkspace(t)
	file := filepath.Join(a.WS.Root, workspace.IgnoreFile)
	writeFile(t, file, "*.log\n# hc:begin\n.hc/\n# hc:end\n")
	res, err := a.SyncWorkspace(context.Background(), newHC)
	if err != nil {
		t.Fatal(err)
	}
	if got := changed(res); len(got) != 1 || got[workspace.IgnoreFile] != skill.Updated {
		t.Errorf("sync: %+v, changes %v", res, got)
	}
	if data, _ := os.ReadFile(file); string(data) != "*.log\n"+workspace.IgnoreBlock() {
		t.Errorf("%s:\n%s", workspace.IgnoreFile, data)
	}

	// A development build leaves it alone.
	writeFile(t, file, "# hc:begin\n.hc/\n# hc:end\n")
	if res, err := a.SyncWorkspace(context.Background(), "dev"); res != nil || err != nil {
		t.Errorf("dev: %+v, %v", res, err)
	}
}
