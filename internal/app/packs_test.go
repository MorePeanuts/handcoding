package app_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/store"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// requireGit skips the test without git, and keeps the user's git
// configuration (signing, hooks, default branch) out of it.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "hc")
	t.Setenv("GIT_AUTHOR_EMAIL", "hc@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "hc")
	t.Setenv("GIT_COMMITTER_EMAIL", "hc@example.com")
}

// gitCmd runs git in dir and returns its output.
func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// tinyProblems are the problems of the regression set in a tiny pack.
var tinyProblems = []string{"0001-two-sum", "0027-remove-element", "2469-convert-the-temperature"}

// writeTinyPack writes a pack called tiny into dir, holding tinyProblems.
func writeTinyPack(t *testing.T, dir string) {
	t.Helper()
	manifest := "protocol: 1\nname: tiny\nversion: 1.0.0\ntitle: Tiny\nlocales: [en, zh-CN]\ngroups:\n  - name: easy\n    problems: [two-sum, remove-element]\n"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, pack.ManifestFile), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, p := range tinyProblems {
		if err := os.CopyFS(filepath.Join(dir, pack.ProblemsDir, p), os.DirFS(filepath.Join(regression.ProblemsDir(), p))); err != nil {
			t.Fatal(err)
		}
	}
}

// tinyRepo creates a git repository holding the tiny pack, on branch main
// with one commit, and returns its directory.
func tinyRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	dir := filepath.Join(t.TempDir(), "tiny")
	writeTinyPack(t, dir)
	gitCmd(t, dir, "init", "-q", "-b", "main")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-q", "-m", "tiny")
	return dir
}

// commitAll commits everything in the repository dir and returns the commit.
func commitAll(t *testing.T, dir, msg string) string {
	t.Helper()
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-q", "-m", msg)
	return gitCmd(t, dir, "rev-parse", "HEAD")
}

// practice creates an empty practice workspace.
func practice(t *testing.T) *app.App {
	t.Helper()
	ws, err := workspace.Create(filepath.Join(t.TempDir(), "ws"), "")
	if err != nil {
		t.Fatal(err)
	}
	a := app.New(ws)
	t.Cleanup(func() { a.Close() })
	return a
}

// reopen opens the workspace of a again, as the next command would.
func reopen(t *testing.T, a *app.App) *app.App {
	t.Helper()
	ws, err := workspace.Open(a.WS.Root)
	if err != nil {
		t.Fatal(err)
	}
	b := app.New(ws)
	t.Cleanup(func() { b.Close() })
	return b
}

// noTempDirs fails if packs/ holds anything but the given directories.
func packDirs(t *testing.T, a *app.App) []string {
	t.Helper()
	entries, err := os.ReadDir(a.WS.PacksPath())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestInstallPackGit(t *testing.T) {
	repo := tinyRepo(t)
	a := practice(t)
	ctx := context.Background()
	res, err := a.InstallPack(ctx, repo, app.PackOptions{})
	if err != nil {
		t.Fatal(err)
	}
	head := gitCmd(t, repo, "rev-parse", "HEAD")
	if res.Manifest.Name != "tiny" || res.Problems != len(tinyProblems) || res.Source.Commit != head || res.Source.Source != repo {
		t.Errorf("result %+v, source %+v", res, res.Source)
	}
	if got := packDirs(t, a); !slices.Equal(got, []string{"local", "tiny"}) {
		t.Errorf("packs/ holds %v", got)
	}
	if _, err := os.Stat(filepath.Join(a.WS.PackPath("tiny"), ".git")); err != nil {
		t.Errorf("not a git checkout: %v", err)
	}
	b := reopen(t, a)
	want := []workspace.PackSource{{Name: "tiny", Source: repo, Commit: head}}
	if !slices.Equal(b.WS.Config.Packs, want) {
		t.Errorf("hc.yaml packs %+v, want %+v", b.WS.Config.Packs, want)
	}
	if _, err := b.WS.Problem("tiny/two-sum"); err != nil {
		t.Error(err)
	}
}

func TestInstallPackCopy(t *testing.T) {
	src := filepath.Join(t.TempDir(), "tiny")
	writeTinyPack(t, src)
	// Not part of the pack: left behind.
	if err := os.WriteFile(filepath.Join(src, "README.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := practice(t)
	res, err := a.InstallPack(context.Background(), src, app.PackOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source.Commit != "" || !res.Source.Copied() || res.Problems != len(tinyProblems) {
		t.Errorf("result %+v", res)
	}
	for _, name := range []string{".git", "README.md"} {
		if _, err := os.Stat(filepath.Join(a.WS.PackPath("tiny"), name)); err == nil {
			t.Errorf("%s was copied", name)
		}
	}
	if _, err := a.InstallPack(context.Background(), src, app.PackOptions{Ref: "main"}); err == nil || !strings.Contains(err.Error(), "--ref needs a git source") {
		t.Errorf("--ref on a directory: %v", err)
	}
}

func TestInstallPackRefusals(t *testing.T) {
	a := practice(t)
	ctx := context.Background()
	newer := filepath.Join(t.TempDir(), "newer")
	writeTinyPack(t, newer)
	if err := os.WriteFile(filepath.Join(newer, pack.ManifestFile), []byte("protocol: 99\nname: newer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.InstallPack(ctx, newer, app.PackOptions{}); err == nil || !strings.Contains(err.Error(), "upgrade hc") {
		t.Errorf("newer protocol: %v", err)
	}
	if _, err := a.InstallPack(ctx, t.TempDir(), app.PackOptions{}); err == nil || !strings.Contains(err.Error(), "neither a git repository nor a pack") {
		t.Errorf("empty directory: %v", err)
	}
	if _, err := a.InstallPack(ctx, "no-such-dir", app.PackOptions{}); err == nil || !strings.Contains(err.Error(), "not a git URL") {
		t.Errorf("missing directory: %v", err)
	}
	// Nothing is left behind in packs/.
	if got := packDirs(t, a); !slices.Equal(got, []string{"local"}) {
		t.Errorf("packs/ holds %v", got)
	}

	src := filepath.Join(t.TempDir(), "tiny")
	writeTinyPack(t, src)
	if _, err := a.InstallPack(ctx, src, app.PackOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.InstallPack(ctx, src, app.PackOptions{}); err == nil || !strings.Contains(err.Error(), "already installed") {
		t.Errorf("installed twice: %v", err)
	}
	// A directory of the same name that hc did not install.
	if err := regression.Install(a.WS); err != nil {
		t.Fatal(err)
	}
	if _, err := a.InstallPack(ctx, regression.Dir(), app.PackOptions{}); err == nil || !strings.Contains(err.Error(), "hc pack remove regression") {
		t.Errorf("unmanaged directory of that name: %v", err)
	}
}

func TestInstallMissing(t *testing.T) {
	repo := tinyRepo(t)
	copied := filepath.Join(t.TempDir(), "copied")
	writeTinyPack(t, copied)
	if err := os.WriteFile(filepath.Join(copied, pack.ManifestFile), []byte("protocol: 1\nname: copied\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := practice(t)
	ctx := context.Background()
	res, err := a.InstallPack(ctx, repo, app.PackOptions{})
	if err != nil {
		t.Fatal(err)
	}
	installed := res.Source.Commit
	if _, err := a.InstallPack(ctx, copied, app.PackOptions{}); err != nil {
		t.Fatal(err)
	}
	// The source moves on after the install.
	if err := os.RemoveAll(filepath.Join(repo, pack.ProblemsDir, tinyProblems[0])); err != nil {
		t.Fatal(err)
	}
	commitAll(t, repo, "drop two-sum")

	// Another workspace with the same hc.yaml gets the same packs.
	b := practice(t)
	data, err := os.ReadFile(filepath.Join(a.WS.Root, workspace.ConfigFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.WS.Root, workspace.ConfigFile), data, 0o644); err != nil {
		t.Fatal(err)
	}
	b = reopen(t, b)
	results, err := b.InstallMissing(ctx, app.PackOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("installed %d packs", len(results))
	}
	if head := gitCmd(t, b.WS.PackPath("tiny"), "rev-parse", "HEAD"); head != installed {
		t.Errorf("installed at %s, want the recorded %s", head, installed)
	}
	if _, err := b.WS.Problem("tiny/two-sum"); err != nil {
		t.Errorf("the recorded commit has two-sum: %v", err)
	}
	if _, err := b.WS.Problem("copied/two-sum"); err != nil {
		t.Error(err)
	}
	// Nothing is missing any more.
	if results, err := b.InstallMissing(ctx, app.PackOptions{}); err != nil || len(results) != 0 {
		t.Errorf("second run: %v, %v", results, err)
	}
}

func TestPackList(t *testing.T) {
	repo := tinyRepo(t)
	a := practice(t)
	ctx := context.Background()
	if _, err := a.InstallPack(ctx, repo, app.PackOptions{}); err != nil {
		t.Fatal(err)
	}
	// Listed in hc.yaml but not installed.
	list := append(slices.Clone(a.WS.Config.Packs), workspace.PackSource{Name: "gone", Source: "https://example.com/gone.git", Commit: "abc"})
	if err := a.WS.SavePacks(list); err != nil {
		t.Fatal(err)
	}
	// Installed by hand.
	if err := regression.Install(a.WS); err != nil {
		t.Fatal(err)
	}
	st, err := a.Store()
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"Wrong Answer", "Accepted", "Accepted"} {
		if err := st.Add(ctx, &store.Submission{Pack: "tiny", Slug: "two-sum", Lang: "go", Verdict: v}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.Add(ctx, &store.Submission{Pack: "tiny", Slug: "remove-element", Lang: "go", Verdict: "Wrong Answer"}); err != nil {
		t.Fatal(err)
	}
	packs, err := a.Packs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		name             string
		state            app.PackState
		problems, solved int
	}
	var got []row
	for _, k := range packs {
		got = append(got, row{k.Name, k.State, k.Problems, k.Solved})
	}
	regs, _, _ := pack.LoadProblems(regression.ProblemsDir(), regression.Name)
	want := []row{
		{"local", app.PackLocal, 0, 0},
		{"tiny", app.PackManaged, 3, 1},
		{"gone", app.PackMissing, 0, 0},
		{"regression", app.PackUnmanaged, len(regs), 0},
	}
	if !slices.Equal(got, want) {
		t.Errorf("packs\n got %+v\nwant %+v", got, want)
	}
	if packs[1].Manifest.Version != "1.0.0" || packs[1].Source.Source != repo || packs[2].Source.Commit != "abc" {
		t.Errorf("tiny %+v, gone %+v", packs[1], packs[2])
	}
}

func TestUpdatePack(t *testing.T) {
	repo := tinyRepo(t)
	a := practice(t)
	ctx := context.Background()
	if _, err := a.InstallPack(ctx, repo, app.PackOptions{}); err != nil {
		t.Fatal(err)
	}
	u, err := a.UpdatePack(ctx, "tiny", app.PackOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if u.From != u.To || len(u.Added)+len(u.Removed)+len(u.Orphans) != 0 {
		t.Errorf("nothing new: %+v", u)
	}

	// The user worked on two-sum, which the next commit removes, and on
	// remove-element, whose directory it renames.
	two, err := a.WS.Problem("tiny/two-sum")
	if err != nil {
		t.Fatal(err)
	}
	goLang, _ := a.Langs.Get("go")
	if _, _, err := a.Start(two, goLang); err != nil {
		t.Fatal(err)
	}
	st, _ := a.Store()
	if err := st.Add(ctx, &store.Submission{Pack: "tiny", Slug: "two-sum", Lang: "go", Verdict: "Accepted"}); err != nil {
		t.Fatal(err)
	}
	rem, _ := a.WS.Problem("tiny/remove-element")
	if _, _, err := a.Start(rem, goLang); err != nil {
		t.Fatal(err)
	}
	problems := filepath.Join(repo, pack.ProblemsDir)
	if err := os.RemoveAll(filepath.Join(problems, "0001-two-sum")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, pack.ManifestFile), []byte("protocol: 1\nname: tiny\nversion: 1.1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// remove-element loses its id: its directory is renamed.
	moved := filepath.Join(problems, "remove-element")
	if err := os.Rename(filepath.Join(problems, "0027-remove-element"), moved); err != nil {
		t.Fatal(err)
	}
	meta := filepath.Join(moved, "problem.yaml")
	data, _ := os.ReadFile(meta)
	if err := os.WriteFile(meta, []byte(strings.Replace(string(data), "id: \"27\"\n", "", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(filepath.Join(problems, "0189-rotate-array"), os.DirFS(filepath.Join(regression.ProblemsDir(), "0189-rotate-array"))); err != nil {
		t.Fatal(err)
	}
	head := commitAll(t, repo, "change problems")

	u, err = a.UpdatePack(ctx, "tiny", app.PackOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if u.To.Commit != head || !slices.Equal(u.Added, []string{"rotate-array"}) || !slices.Equal(u.Removed, []string{"two-sum"}) {
		t.Errorf("update %+v", u)
	}
	wantOrphans := []app.Orphan{
		{Slug: "two-sum", Solutions: []string{filepath.Join("solutions", "tiny", "go", "0001-two-sum")}, Submissions: 1},
		{Slug: "remove-element", Solutions: []string{filepath.Join("solutions", "tiny", "go", "0027-remove-element")}},
	}
	if len(u.Orphans) != 2 || !orphanEqual(u.Orphans[0], wantOrphans[0]) || !orphanEqual(u.Orphans[1], wantOrphans[1]) {
		t.Errorf("orphans %+v, want %+v", u.Orphans, wantOrphans)
	}
	// Kept, not deleted.
	if _, err := os.Stat(filepath.Join(a.WS.Root, wantOrphans[0].Solutions[0])); err != nil {
		t.Error(err)
	}
	if got := reopen(t, a).WS.Config.Packs[0].Commit; got != head {
		t.Errorf("hc.yaml records %s, want %s", got, head)
	}

	// Local changes are refused, and --force discards them.
	installedMeta := filepath.Join(a.WS.PackPath("tiny"), pack.ManifestFile)
	if err := os.WriteFile(installedMeta, []byte("protocol: 1\nname: tiny\nversion: 9.9.9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.UpdatePack(ctx, "tiny", app.PackOptions{}); err == nil || !strings.Contains(err.Error(), "local changes") {
		t.Errorf("local changes: %v", err)
	}
	if _, err := a.UpdatePack(ctx, "tiny", app.PackOptions{Force: true}); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(installedMeta); strings.Contains(string(data), "9.9.9") {
		t.Error("--force kept the local change")
	}
}

func orphanEqual(a, b app.Orphan) bool {
	return a.Slug == b.Slug && slices.Equal(a.Solutions, b.Solutions) && a.Submissions == b.Submissions
}

func TestUpdatePackRef(t *testing.T) {
	repo := tinyRepo(t)
	tagged := gitCmd(t, repo, "rev-parse", "HEAD")
	gitCmd(t, repo, "tag", "v1")
	gitCmd(t, repo, "checkout", "-q", "-b", "dev")
	if err := os.RemoveAll(filepath.Join(repo, pack.ProblemsDir, "0001-two-sum")); err != nil {
		t.Fatal(err)
	}
	dev := commitAll(t, repo, "dev")
	gitCmd(t, repo, "checkout", "-q", "main")

	a := practice(t)
	ctx := context.Background()
	res, err := a.InstallPack(ctx, repo, app.PackOptions{Ref: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source.Commit != tagged || res.Source.Ref != "v1" {
		t.Errorf("installed %+v", res.Source)
	}
	// main moves on; the tag stays where it is.
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	main := commitAll(t, repo, "readme")
	u, err := a.UpdatePack(ctx, "tiny", app.PackOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if u.To.Commit != tagged {
		t.Errorf("a tag moved to %s", u.To.Commit)
	}
	// --ref switches to a branch, which then moves on.
	if u, err = a.UpdatePack(ctx, "tiny", app.PackOptions{Ref: "dev"}); err != nil {
		t.Fatal(err)
	}
	if u.To.Commit != dev || u.To.Ref != "dev" || !slices.Equal(u.Removed, []string{"two-sum"}) {
		t.Errorf("switched to dev: %+v", u)
	}
	if u, err = a.UpdatePack(ctx, "tiny", app.PackOptions{Ref: "main"}); err != nil {
		t.Fatal(err)
	}
	if u.To.Commit != main {
		t.Errorf("main at %s, want %s", u.To.Commit, main)
	}
	if _, err := a.UpdatePack(ctx, "tiny", app.PackOptions{Ref: "nope"}); err == nil || !strings.Contains(err.Error(), `no branch or tag "nope"`) {
		t.Errorf("unknown ref: %v", err)
	}
	// An update that breaks pack.yaml is undone.
	if err := os.WriteFile(filepath.Join(repo, pack.ManifestFile), []byte("protocol: 99\nname: tiny\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAll(t, repo, "newer protocol")
	if _, err := a.UpdatePack(ctx, "tiny", app.PackOptions{}); err == nil || !strings.Contains(err.Error(), "upgrade hc") {
		t.Errorf("newer protocol: %v", err)
	}
	if head := gitCmd(t, a.WS.PackPath("tiny"), "rev-parse", "HEAD"); head != main {
		t.Errorf("left at %s, want %s", head, main)
	}
}

func TestUpdateCopiedPack(t *testing.T) {
	src := filepath.Join(t.TempDir(), "tiny")
	writeTinyPack(t, src)
	a := practice(t)
	ctx := context.Background()
	if _, err := a.InstallPack(ctx, src, app.PackOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(src, pack.ProblemsDir, "0001-two-sum")); err != nil {
		t.Fatal(err)
	}
	u, err := a.UpdatePack(ctx, "tiny", app.PackOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(u.Removed, []string{"two-sum"}) {
		t.Errorf("update %+v", u)
	}
	if _, err := os.Stat(filepath.Join(a.WS.PackPath("tiny"), pack.ProblemsDir, "0001-two-sum")); err == nil {
		t.Error("two-sum is still installed")
	}
	if _, err := a.UpdatePack(ctx, "local", app.PackOptions{}); err == nil {
		t.Error("updated local")
	}
	if _, err := a.UpdatePack(ctx, "nope", app.PackOptions{}); err == nil {
		t.Error("updated a pack that is not installed")
	}
}

func TestRemovePack(t *testing.T) {
	src := filepath.Join(t.TempDir(), "tiny")
	writeTinyPack(t, src)
	a := practice(t)
	ctx := context.Background()
	install := func() {
		t.Helper()
		if _, err := a.InstallPack(ctx, src, app.PackOptions{}); err != nil {
			t.Fatal(err)
		}
		p, err := a.WS.Problem("tiny/two-sum")
		if err != nil {
			t.Fatal(err)
		}
		goLang, _ := a.Langs.Get("go")
		if _, _, err := a.Start(p, goLang); err != nil {
			t.Fatal(err)
		}
		st, _ := a.Store()
		if err := st.Add(ctx, &store.Submission{Pack: "tiny", Slug: "two-sum", Lang: "go", Verdict: "Accepted"}); err != nil {
			t.Fatal(err)
		}
	}
	install()
	res, err := a.RemovePack(ctx, "tiny", false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Deleted || !res.Unlisted || res.Solutions || res.Submissions != 0 {
		t.Errorf("remove %+v", res)
	}
	if len(reopen(t, a).WS.Config.Packs) != 0 {
		t.Error("still in hc.yaml")
	}
	sol := filepath.Join(a.WS.Root, workspace.SolutionsDir, "tiny")
	if _, err := os.Stat(sol); err != nil {
		t.Errorf("solutions deleted: %v", err)
	}
	install()
	if res, err = a.RemovePack(ctx, "tiny", true); err != nil {
		t.Fatal(err)
	}
	if !res.Solutions || res.Submissions != 2 {
		t.Errorf("purge %+v", res)
	}
	if _, err := os.Stat(sol); err == nil {
		t.Error("solutions kept with purge")
	}
	if _, err := a.RemovePack(ctx, "tiny", false); err == nil {
		t.Error("removed a pack twice")
	}
	for _, name := range []string{"local", "..", "../ws"} {
		if _, err := a.RemovePack(ctx, name, true); err == nil {
			t.Errorf("removed %q", name)
		}
	}
	if _, err := os.Stat(a.WS.PackPath("local")); err != nil {
		t.Error(err)
	}
}

func TestPackCommentsKept(t *testing.T) {
	src := filepath.Join(t.TempDir(), "tiny")
	writeTinyPack(t, src)
	a := practice(t)
	path := filepath.Join(a.WS.Root, workspace.ConfigFile)
	config := "# my settings\nlanguage: python # I like Python\n\n# the agent\nagent:\n  name: codex\ntimeout: 20s\n"
	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	a = reopen(t, a)
	ctx := context.Background()
	if _, err := a.InstallPack(ctx, src, app.PackOptions{}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	for _, s := range []string{"# my settings\n", "# I like Python", "# the agent\nagent:", "timeout: 20s\npacks:\n  - name: tiny\n"} {
		if !strings.Contains(string(data), s) {
			t.Errorf("hc.yaml lacks %q after install:\n%s", s, data)
		}
	}
	b := reopen(t, a)
	if b.WS.Config.Language != "python" || b.WS.Config.Agent.Name != "codex" || len(b.WS.Config.Packs) != 1 {
		t.Errorf("config %+v", b.WS.Config)
	}
	if _, err := b.RemovePack(ctx, "tiny", false); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if strings.Contains(string(data), "packs") || !strings.Contains(string(data), "# I like Python") || !strings.Contains(string(data), "# the agent") {
		t.Errorf("hc.yaml after remove:\n%s", data)
	}
}

// checkout writes the tiny pack as a pack checkout, an authoring workspace
// that runs python3 rather than a virtual environment, edits it with edit,
// and runs hc pack check on it.
func checkout(t *testing.T, edit func(dir string)) (app.ValidateSummary, map[string]app.Finding) {
	t.Helper()
	dir := t.TempDir()
	writeTinyPack(t, dir)
	if err := os.WriteFile(filepath.Join(dir, workspace.ConfigFile), []byte("python:\n  interpreter: python3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if edit != nil {
		edit(dir)
	}
	ws, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := app.New(ws)
	defer a.Close()
	findings := map[string]app.Finding{}
	sum, err := a.CheckPack(context.Background(), app.ValidateOptions{}, func(f app.Finding) { findings[f.Name] = f })
	if err != nil {
		t.Fatal(err)
	}
	return sum, findings
}

// replaceIn replaces old with new in the file at path, which must hold it.
func replaceIn(t *testing.T, path, old, new string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), old) {
		t.Fatalf("%s does not hold %q", path, old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), old, new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckPack(t *testing.T) {
	requireTools(t, "go", "python3")
	sum, findings := checkout(t, nil)
	if !sum.OK() || sum.Problems != len(tinyProblems) {
		t.Errorf("summary %+v", sum)
		for _, f := range findings {
			t.Logf("%s: %v", f.Name, f.Errors)
		}
	}

	sum, findings = checkout(t, func(dir string) {
		problems := filepath.Join(dir, pack.ProblemsDir)
		replaceIn(t, filepath.Join(problems, "0001-two-sum", "problem.yaml"), "judge:\n  compare: unordered\n", "")
		if err := os.RemoveAll(filepath.Join(problems, "0001-two-sum", "reference")); err != nil {
			t.Fatal(err)
		}
		replaceIn(t, filepath.Join(problems, "2469-convert-the-temperature", "cases.jsonl"), "[309.65,97.7]", "[309.65,97.8]")
		if err := os.Remove(filepath.Join(problems, "2469-convert-the-temperature", "description.zh-CN.md")); err != nil {
			t.Fatal(err)
		}
		replaceIn(t, filepath.Join(dir, pack.ManifestFile), "[two-sum, remove-element]", "[two-sum, nope]")
	})
	if sum.OK() || sum.Failed != 2 || sum.PackErrors != 1 {
		t.Errorf("summary %+v", sum)
	}
	for name, wants := range map[string][]string{
		"two-sum":                 {"not ready", "no reference solution"},
		"convert-the-temperature": {"case 1: Wrong Answer", "no statement in zh-CN: description.zh-CN.md is missing"},
		"pack tiny":               {"group easy: nope is not a problem of the pack"},
	} {
		f := findings[name]
		for _, want := range wants {
			if !strings.Contains(strings.Join(f.Errors, "\n"), want) {
				t.Errorf("%s: errors %q, want %q", name, f.Errors, want)
			}
		}
	}
	if f := findings["remove-element"]; !f.OK() {
		t.Errorf("remove-element: %v", f.Errors)
	}

	// A practice workspace is not a pack checkout.
	a := practice(t)
	if _, err := a.CheckPack(context.Background(), app.ValidateOptions{}, func(app.Finding) {}); err == nil || !strings.Contains(err.Error(), "pass its directory") {
		t.Errorf("practice workspace: %v", err)
	}
}

func TestPackCommandsNeedPractice(t *testing.T) {
	dir := t.TempDir()
	writeTinyPack(t, dir)
	ws, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := app.New(ws)
	defer a.Close()
	ctx := context.Background()
	if _, err := a.InstallPack(ctx, dir, app.PackOptions{}); err == nil || !strings.Contains(err.Error(), "pack checkout") {
		t.Errorf("install: %v", err)
	}
	if _, err := a.RemovePack(ctx, "tiny", false); err == nil {
		t.Error("remove in a checkout")
	}
	packs, err := a.Packs(ctx)
	if err != nil || len(packs) != 1 || packs[0].State != app.PackCheckout || packs[0].Problems != len(tinyProblems) {
		t.Errorf("packs %+v, %v", packs, err)
	}
}
