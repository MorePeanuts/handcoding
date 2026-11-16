package cpp_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/lang/cpp"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/testenv"
)

func script(t *testing.T, path, source string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+source), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestToolchainDiagnostics(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	compiler, debugger := filepath.Join(dir, "my compiler"), filepath.Join(dir, "my debugger")
	for _, tt := range []struct{ version, minimum string }{
		{"g++ (GCC) 9.5.0", ""}, {"clang version 10.0.0", ""}, {"Apple clang version 16.0.0 (clang-1600.0)", ""},
	} {
		script(t, compiler, "echo '"+tt.version+"'\n")
		script(t, debugger, "echo 'GNU gdb 15.1'\n")
		l := cpp.Language{Compiler: compiler, Debugger: debugger, Standard: "c++26", Sanitizers: "address,undefined"}
		tc, err := l.Toolchain()
		if err != nil || tc.Min != tt.minimum || tc.Path != compiler || !slices.Contains(tc.Flags, "-std=c++26") || !slices.Contains(tc.Flags, "-fsanitize=address,undefined") {
			t.Fatalf("%+v %v", tc, err)
		}
		checks := l.Diagnose(context.Background(), lang.DiagnosticOptions{Debug: true})
		if len(checks) != 2 || checks[0].Minimum != cpp.MinStandard || checks[0].Status != "ok" || checks[1].Status != "ok" {
			t.Fatal(checks)
		}
		checks = l.Diagnose(context.Background(), lang.DiagnosticOptions{Debug: true, Headless: true})
		if checks[1].Status != "unsupported" {
			t.Fatal(checks)
		}
	}
	script(t, compiler, "if [ \"$1\" = --version ]; then echo 'clang version 17.0.6'; else echo 'missing standard headers' >&2; exit 1; fi\n")
	checks := (cpp.Language{Compiler: compiler}).Diagnose(context.Background(), lang.DiagnosticOptions{})
	if checks[0].Status != "error" || !strings.Contains(checks[0].Message, "missing standard headers") {
		t.Fatal(checks)
	}
	script(t, compiler, "echo 'g++ (GCC) 12.3.0'\n")
	checks = (cpp.Language{Compiler: compiler}).Diagnose(context.Background(), lang.DiagnosticOptions{})
	if checks[0].Status != "ok" {
		t.Fatal(checks)
	}
	// A compiler capable of C++17 must be allowed, but an unsupported
	// explicitly selected mode must fail rather than silently downgrade.
	script(t, compiler, `if [ "$1" = --version ]; then echo 'g++ (GCC) 9.4.0'; elif [ "$1" = -std=c++17 ]; then exit 0; else echo 'unsupported standard' >&2; exit 1; fi`)
	checks = (cpp.Language{Compiler: compiler, Standard: "c++17"}).Diagnose(context.Background(), lang.DiagnosticOptions{})
	if checks[0].Status != "ok" {
		t.Fatal(checks)
	}
	checks = (cpp.Language{Compiler: compiler}).Diagnose(context.Background(), lang.DiagnosticOptions{})
	if checks[0].Status != "error" || !strings.Contains(checks[0].Message, "unsupported standard") {
		t.Fatal(checks)
	}
	checks = (cpp.Language{Compiler: filepath.Join(dir, "missing")}).Diagnose(context.Background(), lang.DiagnosticOptions{})
	if checks[0].Status != "missing" {
		t.Fatal(checks)
	}
	for _, s := range []string{"c++11", "c++14", "gnu++23", "c++23 -include evil"} {
		if cpp.CheckStandard(s) == nil {
			t.Errorf("accepted %q", s)
		}
	}
}

func TestEditorDatabaseAndLeetCodeSource(t *testing.T) {
	t.Parallel()
	testenv.LookPath(t, cpp.DefaultCompiler())
	dir := t.TempDir()
	p := function([]problem.Param{{Name: "root", Type: "TreeNode"}}, "TreeNode")
	code := `#include <nlohmann/json.hpp>
class Solution {public: TreeNode* f(TreeNode* root) { vector<int> nums{1}; return root ? root : new TreeNode(nums[0]); }};`
	if err := os.WriteFile(filepath.Join(dir, "solution.cpp"), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	l := cpp.Language{Standard: cpp.MinStandard, Environment: filepath.Join(t.TempDir(), "C++ environment with spaces")}
	if err := l.Setup(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if err := l.Prepare(dir, p); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "compile_commands.json"))
	if err != nil {
		t.Fatal(err)
	}
	var db []struct {
		Directory, File string
		Arguments       []string
	}
	if err := json.Unmarshal(data, &db); err != nil {
		t.Fatal(err)
	}
	if len(db) != 1 || db[0].File != filepath.Join(dir, "solution.cpp") || db[0].Directory != dir {
		t.Fatal(string(data))
	}
	// Compile the exact editor translation unit: no driver includes can hide a missing type/header.
	args := append(slices.Clone(db[0].Arguments[1:]), "-o", filepath.Join(dir, "editor.o"))
	cmd := exec.Command(db[0].Arguments[0], args...)
	cmd.Dir = dir
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v", b, err)
	}
	id, submitted, err := l.LeetCode(p, code)
	if err != nil || id != "cpp" || submitted != code {
		t.Fatalf("%s %s %v", id, submitted, err)
	}
}

func TestDebuggerCommands(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	compiler, debugger := filepath.Join(dir, "compiler"), filepath.Join(dir, "debugger")
	// Building is tested with the real compiler elsewhere. Here wrappers let us verify both OS debugger commands.
	script(t, compiler, "exit 0\n")
	p := function(nil, "integer")
	l := cpp.Language{Compiler: compiler, Debugger: debugger}
	for _, tt := range []struct{ version, flag, breakpoint string }{
		{"lldb-1700", "--", "breakpoint set -n Solution::f"},
		{"GNU gdb 15.1", "--args", "break Solution::f"},
	} {
		script(t, debugger, "echo '"+tt.version+"'\n")
		cmd, err := l.DebugCommand(dir, dir, p, lang.DebugOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if cmd.Path != debugger || !slices.Contains(cmd.Args, tt.flag) || !slices.Contains(cmd.Args, tt.breakpoint) {
			t.Fatal(cmd.Args)
		}
	}
	if _, err := l.DebugCommand(dir, dir, p, lang.DebugOptions{Headless: true}); err == nil {
		t.Fatal("headless accepted")
	}
}

func TestExhaustedWorkerBudget(t *testing.T) {
	t.Parallel()
	testenv.LookPath(t, cpp.DefaultCompiler())
	dir := t.TempDir()
	p := function(nil, "integer")
	code := `class Solution {public:int f(){volatile unsigned n=0;for(;;)n=n+1;}};`
	if err := os.WriteFile(filepath.Join(dir, "solution.cpp"), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	l := cpp.Language{}
	if err := l.Prepare(dir, p); err != nil {
		t.Fatal(err)
	}
	if err := l.Build(context.Background(), dir, dir); err != nil {
		t.Fatal(err)
	}
	cases, out := filepath.Join(dir, "cases.json"), filepath.Join(dir, "out.jsonl")
	if err := os.WriteFile(cases, []byte(`[{"id":1,"input":{}}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := l.Command(ctx, dir, dir)
	cmd.Env = append(cmd.Env, lang.EnvCases+"="+cases, lang.EnvOut+"="+out, lang.EnvTimeLimit+"=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("exhausted budget did not stop the worker: %s %v", b, err)
	}
	data, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(data), `"time_limit":true`) {
		t.Fatalf("%s %v", data, err)
	}
}

func TestMinimumStandard(t *testing.T) {
	t.Parallel()
	testenv.LookPath(t, cpp.DefaultCompiler())
	for _, standard := range []string{"c++17", "c++20", "c++23", "c++26", "c++2c"} {
		if err := cpp.CheckStandard(standard); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "solution.cpp")
	p := function(nil, "integer")
	// This API exists in C++20, while the user's configured default is C++23.
	code := `class Solution { public: int f(){set<int> s{1};return s.contains(1);} };`
	if err := os.WriteFile(file, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	l := cpp.Language{}
	if err := l.CheckMinVersion(context.Background(), p, file); err == nil || !strings.Contains(err.Error(), "contains") {
		t.Fatalf("minimum API check: %v", err)
	}
	if err := l.Prepare(dir, p); err != nil {
		t.Fatal(err)
	}
	if err := l.Build(context.Background(), dir, dir); err != nil {
		t.Fatal(err)
	}
}
