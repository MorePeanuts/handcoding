package java_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/lang/java"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

func TestDiagnostics(t *testing.T) {
	t.Parallel()
	l := jdk(t)
	checks := l.Diagnose(context.Background(), lang.DiagnosticOptions{Debug: true})
	if len(checks) != 3 {
		t.Fatal(checks)
	}
	for _, c := range checks {
		if c.Status != "ok" {
			t.Fatalf("%+v", c)
		}
	}
	l.JVMArgs = []string{"--class-path"}
	checks = l.Diagnose(context.Background(), lang.DiagnosticOptions{})
	if checks[1].Status != "error" || !strings.Contains(checks[1].Message, "java.jvm_args") {
		t.Fatal(checks)
	}
	log := filepath.Join(t.TempDir(), "gc.log")
	l.JVMArgs = []string{"-Xlog:gc:file=" + log}
	checks = l.Diagnose(context.Background(), lang.DiagnosticOptions{})
	if checks[0].Status != "ok" || checks[1].Status != "ok" {
		t.Fatal(checks)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("doctor executed a file-writing VM option", err)
	}

	missing := t.TempDir()
	l.Home = filepath.Join(missing, "jdk")
	checks = l.Diagnose(context.Background(), lang.DiagnosticOptions{Debug: true})
	for _, c := range checks {
		if c.Status != "missing" {
			t.Fatalf("%+v", c)
		}
	}
	entries, _ := os.ReadDir(missing)
	if len(entries) != 0 {
		t.Fatal("doctor wrote environment state")
	}
	for _, args := range [][]string{{"-cp"}, {"@arguments"}, {"Main"}, {"-Djava.class.path=elsewhere"}, {"-Xbootclasspath/a:extra.jar"}} {
		if java.CheckArgs(args) == nil {
			t.Fatalf("accepted launcher override %q", args)
		}
	}
}
func TestMinimumAPIAndLocalJDK(t *testing.T) {
	t.Parallel()
	l := jdk(t)
	dir := t.TempDir()
	p := function("integer")
	p.Dir = dir
	file := filepath.Join(dir, "Solution.java")
	if err := os.WriteFile(file, []byte(`class Solution {public int f(int n){return java.util.List.of(n).getFirst();}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := l.CheckMinVersion(context.Background(), p, file); err == nil || !strings.Contains(err.Error(), "getFirst") {
		t.Fatalf("minimum API check: %v", err)
	}
	tc, _ := l.Toolchain()
	if err := l.Prepare(dir, p); err != nil {
		t.Fatal(err)
	}
	err := l.Build(context.Background(), dir, t.TempDir())
	if lang.AtLeast(tc.Version, "21") {
		if err != nil {
			t.Fatal(err)
		}
	} else if err == nil || !strings.Contains(err.Error(), "getFirst") {
		t.Fatalf("local JDK must decide available APIs: %v", err)
	}
	prefs, _ := os.ReadFile(filepath.Join(dir, ".settings", "org.eclipse.jdt.core.prefs"))
	level := strings.Split(tc.Version, ".")[0]
	if !strings.Contains(string(prefs), "compiler.source="+level+"\n") || !strings.Contains(string(prefs), "compiler.release=disabled") {
		t.Fatalf("editor does not follow JDK %s: %s", tc.Version, prefs)
	}
}
func TestLeetCodeStarter(t *testing.T) {
	t.Parallel()
	p := &problem.Problem{Dir: t.TempDir(), Kind: problem.KindClass, Class: &problem.Class{Name: "Codec"}}
	dir := filepath.Join(p.Dir, "templates")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Solution.java"), []byte("public class Codec {\n public String encode(String s){return s;}\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := java.Language{}
	s, err := l.Starter(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(s, "public class Codec") || !strings.Contains(s, "public String encode") || !strings.Contains(s, "import java.util.*;") {
		t.Fatal(s)
	}
	id, submitted, err := l.LeetCode(p, s)
	if err != nil || id != "java" || submitted != s {
		t.Fatalf("%s %s %v", id, submitted, err)
	}
}
