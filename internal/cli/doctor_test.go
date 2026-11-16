package cli

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/testenv"
)

func doctorScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func doctorSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		files[path] = info.ModTime().String() + info.Mode().String()
		if info.Mode().IsRegular() {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files[path] += string(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func doctorReport(t *testing.T, dir string, args ...string) (*app.DoctorReport, error) {
	t.Helper()
	out, err := capture(t, append([]string{"-w", dir, "doctor", "--json"}, args...)...)
	var rep app.DoctorReport
	if decodeErr := json.Unmarshal([]byte(out), &rep); decodeErr != nil {
		t.Fatalf("%s: %v (%v)", out, decodeErr, err)
	}
	return &rep, err
}

func TestDoctorReadOnlyAndConfiguredTools(t *testing.T) {
	dir := t.TempDir()
	config := "language: go\ngo:\n  command: tools/custom-go\n  debugger: tools/dlv\n"
	if err := os.WriteFile(filepath.Join(dir, "hc.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	doctorScript(t, filepath.Join(dir, "tools/custom-go"), "[ \"$GOTOOLCHAIN\" = local ] || exit 9\nprintf 'go1.26.0\\nlinux\\namd64\\n\\n0\\n\\n'")
	// These must never be invoked by diagnosis, even with a missing venv.
	doctorScript(t, filepath.Join(dir, "tools/uv"), "touch \"$0.called\"; exit 1")
	t.Setenv("PATH", filepath.Join(dir, "tools"))
	before := doctorSnapshot(t, dir)
	rep, err := doctorReport(t, dir)
	if err == nil || rep.OK || len(rep.Checks) != 2 {
		t.Fatalf("%+v, %v", rep, err)
	}
	if c := rep.Checks[0]; c.Status != "ok" || c.Version != "1.26.0" || c.Path != filepath.Join(dir, "tools/custom-go") || c.Source != "hc.yaml: go.command" {
		t.Fatalf("%+v", c)
	}
	if rep.Checks[1].Status != "uninitialized" {
		t.Fatalf("%+v", rep.Checks[1])
	}
	if after := doctorSnapshot(t, dir); !reflect.DeepEqual(before, after) {
		t.Fatal("doctor changed the workspace")
	}

	// A valid managed interpreter needs neither uv nor debugpy for ordinary checks.
	doctorScript(t, filepath.Join(dir, ".hc/venv/bin/python"), "case \"$*\" in *debugpy*) exit 1;; *pdb*) echo available;; *) echo 3.12.9;; esac")
	before = doctorSnapshot(t, dir)
	rep, err = doctorReport(t, dir)
	if err != nil || !rep.OK {
		t.Fatalf("%+v, %v", rep, err)
	}
	rep, err = doctorReport(t, dir, "--debug")
	if err == nil || rep.Checks[1].Role != "debugger" || rep.Checks[1].Status != "missing" {
		t.Fatalf("%+v, %v", rep, err)
	}
	rep, err = doctorReport(t, dir, "--lang", "py", "--debug")
	if err != nil || !rep.OK || rep.Checks[1].Role != "pdb" {
		t.Fatalf("%+v, %v", rep, err)
	}
	rep, err = doctorReport(t, dir, "--lang", "py", "--debug", "--headless")
	if err == nil || rep.Checks[1].Role != "debugpy" {
		t.Fatalf("%+v, %v", rep, err)
	}
	if after := doctorSnapshot(t, dir); !reflect.DeepEqual(before, after) {
		t.Fatal("doctor changed the workspace")
	}
}

func TestDoctorExternalPythonAndFailures(t *testing.T) {
	dir := t.TempDir()
	config := "language: python\npython:\n  interpreter: tools/python\ngo:\n  command: missing-go\n"
	if err := os.WriteFile(filepath.Join(dir, "hc.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	py := filepath.Join(dir, "tools/python")
	for _, tc := range []struct{ body, status string }{
		{"echo 3.12.9", "ok"}, {"echo 3.9.6", "too_old"}, {"echo nonsense", "error"}, {"exit 7", "error"},
	} {
		doctorScript(t, py, tc.body)
		rep, err := doctorReport(t, dir)
		if len(rep.Checks) != 1 || rep.Checks[0].Status != tc.status || rep.Checks[0].Path != py || (err == nil) != (tc.status == "ok") {
			t.Fatalf("%+v, %v", rep, err)
		}
	}
	if err := os.Remove(py); err != nil {
		t.Fatal(err)
	}
	rep, err := doctorReport(t, dir)
	if err == nil || rep.Checks[0].Status != "missing" {
		t.Fatalf("%+v, %v", rep, err)
	}
	if _, err := capture(t, "-w", dir, "doctor", "--headless"); err == nil {
		t.Fatal("headless without debug accepted")
	}
	if _, err := capture(t, "-w", dir, "doctor", "--lang", "unknown"); err == nil {
		t.Fatal("unknown language accepted")
	}
}

func TestDoctorNoWorkspace(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	t.Setenv("HC_WORKSPACE", "")
	doctorScript(t, filepath.Join(dir, "python3"), "echo 3.12.9")
	t.Setenv("PATH", dir)
	out, err := capture(t, "doctor", "--lang", "py", "--json")
	if err != nil || !strings.Contains(out, `"ok": true`) {
		t.Fatalf("%s: %v", out, err)
	}
	if _, err := capture(t, "-w", filepath.Join(dir, "missing"), "doctor"); err == nil {
		t.Fatal("explicit missing workspace ignored")
	}
	t.Setenv("HC_WORKSPACE", filepath.Join(dir, "missing"))
	if _, err := capture(t, "doctor"); err == nil {
		t.Fatal("missing HC_WORKSPACE ignored")
	}
}

func TestDoctorRealPythonDoesNotWrite(t *testing.T) {
	py := testenv.LookPath(t, "python3")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hc.yaml"), []byte("language: python\npython:\n  interpreter: "+py+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := doctorSnapshot(t, dir)
	rep, err := doctorReport(t, dir, "--debug")
	if rep.Checks[0].Status == "too_old" {
		testenv.Skip(t, rep.Checks[0].Message)
	}
	if err != nil || !rep.OK {
		t.Fatalf("%+v, %v", rep, err)
	}
	if after := doctorSnapshot(t, dir); !reflect.DeepEqual(before, after) {
		t.Fatal("Python probe wrote files")
	}
}

func TestDoctorCPPConfiguredTools(t *testing.T) {
	dir := t.TempDir()
	config := "language: cpp\ncpp:\n  command: tools/compiler\n  debugger: tools/debugger\n  standard: c++26\n  sanitizers: undefined\npython:\n  interpreter: tools/python\n"
	if err := os.WriteFile(filepath.Join(dir, "hc.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	doctorScript(t, filepath.Join(dir, "tools/compiler"), `if [ "$1" = --version ]; then echo 'clang version 17.0.6'; else [ "$1" = -std=c++26 ] || exit 7; fi`)
	doctorScript(t, filepath.Join(dir, "tools/debugger"), "echo 'lldb-1700'")
	doctorScript(t, filepath.Join(dir, "tools/python"), "echo 3.12.9")
	before := doctorSnapshot(t, dir)
	rep, err := doctorReport(t, dir, "--debug")
	if err != nil || !rep.OK || len(rep.Checks) != 3 {
		t.Fatalf("%+v %v", rep, err)
	}
	if c := rep.Checks[0]; c.Path != filepath.Join(dir, "tools/compiler") || c.Source != "hc.yaml: cpp.command" {
		t.Fatal(c)
	}
	if c := rep.Checks[1]; c.Path != filepath.Join(dir, "tools/debugger") || c.Source != "hc.yaml: cpp.debugger" {
		t.Fatal(c)
	}
	rep, err = doctorReport(t, dir, "--debug", "--headless")
	if err == nil || rep.OK || rep.Checks[1].Status != "unsupported" {
		t.Fatalf("%+v %v", rep, err)
	}
	if after := doctorSnapshot(t, dir); !reflect.DeepEqual(before, after) {
		t.Fatal("C++ doctor changed the workspace")
	}
}

func TestDoctorConfiguredJavaJDK(t *testing.T) {
	dir := t.TempDir()
	config := "language: java\njava:\n  home: tools/jdk\n  jvm_args: [-Xss16m]\npython:\n  interpreter: tools/python\n"
	if err := os.WriteFile(filepath.Join(dir, "hc.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	for tool, version := range map[string]string{"java": "openjdk version 17.0.21", "javac": "javac 17.0.21", "jdb": "jdb 17.0.21"} {
		doctorScript(t, filepath.Join(dir, "tools", "jdk", "bin", tool), "echo '"+version+"'")
	}
	doctorScript(t, filepath.Join(dir, "tools", "python"), "echo '3.12.0'")
	before := doctorSnapshot(t, dir)
	rep, err := doctorReport(t, dir, "--debug")
	if err != nil || !rep.OK || len(rep.Checks) != 4 {
		t.Fatalf("%+v %v", rep, err)
	}
	for _, c := range rep.Checks[:3] {
		if c.Source != "hc.yaml: java.home" || !strings.HasPrefix(c.Path, filepath.Join(dir, "tools", "jdk", "bin")) {
			t.Fatalf("JDK selection: %+v", c)
		}
	}
	if after := doctorSnapshot(t, dir); !reflect.DeepEqual(before, after) {
		t.Fatal("doctor modified workspace")
	}
}
