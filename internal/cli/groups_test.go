package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPersonalGroupCommands(t *testing.T) {
	dir := packWorkspace(t)
	run := func(args ...string) string {
		t.Helper()
		out, err := capture(t, append([]string{"-w", dir}, args...)...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return out
	}
	fail := func(want string, args ...string) {
		t.Helper()
		_, err := capture(t, append([]string{"-w", dir}, args...)...)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%v: %v, want %s", args, err, want)
		}
	}
	run("group", "create", "review", "--pack", "regression", "--title", "复习")
	run("group", "add", "user:review", "146", "1", "--pack", "regression")
	out := run("list", "--plain", "--group", "user:review")
	if strings.Index(out, "lru-cache") > strings.Index(out, "two-sum") || !strings.Contains(out, "two-sum") {
		t.Fatalf("order:\n%s", out)
	}
	run("group", "order", "review", "two-sum", "lru-cache")
	out = run("group", "show", "review")
	if !strings.Contains(out, "复习") || strings.Index(out, "two-sum") > strings.Index(out, "lru-cache") {
		t.Fatalf("show:\n%s", out)
	}
	fail("exactly once", "group", "order", "review", "two-sum")
	run("group", "copy", "pack:design", "design", "--pack", "regression")
	fail("ambiguous", "list", "--plain", "--pack", "regression", "--group", "design")
	fail("pack group", "group", "rm", "pack:design", "--pack", "regression")
	run("group", "rename", "user:design", "review-copy", "--title", "副本")
	run("group", "remove", "review-copy", "lru-cache")
	run("group", "rm", "review-copy")
	out = run("group", "list", "--pack", "regression")
	if !strings.Contains(out, "user:review") || !strings.Contains(out, "pack:design") {
		t.Fatalf("sources:\n%s", out)
	}
	// Direct editing can retain unavailable references; show reports them.
	path := filepath.Join(dir, "groups", "regression.yaml")
	if err := os.WriteFile(path, []byte("groups:\n  - name: review\n    problems: [gone, two-sum]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := run("group", "show", "review"); !strings.Contains(out, "gone (unavailable)") {
		t.Fatalf("missing:\n%s", out)
	}
	if out := run("list", "--plain", "--group", "review"); !strings.Contains(out, "two-sum") {
		t.Fatalf("available member:\n%s", out)
	}
	run("group", "remove", "review", "gone")
	run("group", "create", "review", "--pack", "local")
	run("group", "add", "review", "two-sum", "--pack", "local")
	fail("ambiguous", "group", "show", "review")
	run("group", "rm", "review", "--pack", "local")
	run("group", "rm", "review", "--pack", "regression")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("last group file: %v", err)
	}
}
