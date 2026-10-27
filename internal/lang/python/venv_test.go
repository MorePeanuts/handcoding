package python

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The tests drive Venv with shell scripts standing in for uv and Python on
// PATH, so they create no real environment and need no network. Every
// script appends its command line to a log.

type fakeEnv struct {
	t    *testing.T
	bin  string // the only directory on PATH
	lib  string // scripts outside PATH
	log  string
	root string // the workspace
}

func newFakeEnv(t *testing.T) *fakeEnv {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake executables are shell scripts")
	}
	dir := t.TempDir()
	f := &fakeEnv{t: t, bin: filepath.Join(dir, "bin"), lib: filepath.Join(dir, "lib"), log: filepath.Join(dir, "log"), root: filepath.Join(dir, "ws")}
	for _, d := range []string{f.bin, f.lib, f.root} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", f.bin)
	return f
}

func (f *fakeEnv) script(path, body string) string {
	f.t.Helper()
	src := "#!/bin/sh\necho \"$0 $*\" >> '" + f.log + "'\n" + body
	if err := os.WriteFile(path, []byte(src), 0o755); err != nil {
		f.t.Fatal(err)
	}
	return path
}

// python writes a Python reporting version ver. Its -m venv copies itself
// into the environment, so the environment has the same version.
func (f *fakeEnv) python(path, ver string) string {
	return f.script(path, `case "$1" in
-c) echo `+ver+` ;;
-m) if [ "$2" = venv ]; then /bin/mkdir -p "$3/bin" && /bin/cp "$0" "$3/bin/python"; fi ;;
esac
`)
}

// uv writes a uv whose venv installs a Python reporting version ver.
func (f *fakeEnv) uv(ver string) {
	py := f.python(filepath.Join(f.lib, "python"+ver), ver)
	f.script(filepath.Join(f.bin, "uv"), `if [ "$1" = venv ]; then
  for a; do dir=$a; done
  /bin/mkdir -p "$dir/bin" && /bin/cp '`+py+`' "$dir/bin/python"
fi
`)
}

func (f *fakeEnv) venv() *Venv {
	return &Venv{
		Dir:     filepath.Join(f.root, ".hc", "venv"),
		Label:   ".hc/venv",
		Pyright: filepath.Join(f.root, "pyrightconfig.json"),
	}
}

func (f *fakeEnv) logged() string {
	data, _ := os.ReadFile(f.log)
	return string(data)
}

func (f *fakeEnv) wantLogged(lines ...string) {
	f.t.Helper()
	log := f.logged()
	for _, l := range lines {
		if !strings.Contains(log, l+"\n") {
			f.t.Errorf("log lacks %q:\n%s", l, log)
		}
	}
}

func TestVenvWithUV(t *testing.T) {
	f := newFakeEnv(t)
	f.uv("3.12.4")
	v := f.venv()
	var out bytes.Buffer
	if err := v.Ensure(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	f.wantLogged(
		f.bin+"/uv venv --no-project --python >=3.10 "+v.Dir,
		f.bin+"/uv pip install --python "+v.Python()+" debugpy",
	)
	if !strings.Contains(out.String(), "network") {
		t.Errorf("no word on network access:\n%s", out.String())
	}
	data, err := os.ReadFile(v.Pyright)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(strings.Fields(string(data)), " "); got != `{ "venv": "venv", "venvPath": ".hc" }` {
		t.Errorf("pyrightconfig.json = %s", data)
	}
	tc, err := Language{Venv: v}.Toolchain()
	if err != nil {
		t.Fatal(err)
	}
	if tc.String() != "Python 3.12.4 (.hc/venv)" || !tc.Supported() {
		t.Errorf("toolchain %q", tc)
	}

	// A later process finds the environment working and leaves it alone.
	before := f.logged()
	if err := f.venv().Ensure(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	if after := f.logged(); strings.Count(after, "uv venv") != strings.Count(before, "uv venv") {
		t.Errorf("environment created again:\n%s", after)
	}
}

// An environment whose interpreter no longer runs is created again.
func TestVenvRepair(t *testing.T) {
	f := newFakeEnv(t)
	f.uv("3.12.4")
	v := f.venv()
	if err := os.MkdirAll(filepath.Dir(v.Python()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(v.Python(), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := v.Ensure(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "no longer runs") {
		t.Errorf("repair not reported:\n%s", out.String())
	}
	if ver, err := version(context.Background(), v.Python()); err != nil || ver != "3.12.4" {
		t.Errorf("repaired environment: %q, %v", ver, err)
	}
}

// Without uv, the newest supported Python on PATH creates the environment.
func TestVenvWithoutUV(t *testing.T) {
	f := newFakeEnv(t)
	f.python(filepath.Join(f.bin, "python3"), "3.9.6")
	f.python(filepath.Join(f.bin, "python"), "3.10.0")
	f.python(filepath.Join(f.bin, "python3.11"), "3.11.2")
	f.python(filepath.Join(f.bin, "python3.12"), "3.12.1")
	f.python(filepath.Join(f.bin, "python3-config"), "0")
	v := f.venv()
	// A pyrightconfig.json of the user's own is kept.
	if err := os.WriteFile(v.Pyright, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := v.Ensure(context.Background(), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	f.wantLogged(
		f.bin+"/python3.12 -m venv "+v.Dir,
		v.Python()+" -m pip install --disable-pip-version-check debugpy",
	)
	if strings.Contains(f.logged(), "python3-config") {
		t.Error("ran python3-config")
	}
	if data, _ := os.ReadFile(v.Pyright); string(data) != "{}\n" {
		t.Errorf("pyrightconfig.json overwritten: %s", data)
	}
}

func TestVenvNoPython(t *testing.T) {
	f := newFakeEnv(t)
	f.python(filepath.Join(f.bin, "python3"), "3.9.6")
	v := f.venv()
	err := v.Ensure(context.Background(), &bytes.Buffer{})
	if err == nil {
		t.Fatal("no error")
	}
	for _, want := range []string{"uv", "3.10", "is 3.9.6"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q: %v", want, err)
		}
	}
	if _, err := (Language{Venv: v}).Toolchain(); err == nil {
		t.Error("toolchain without an environment")
	}
}

// A command tries to create the environment once: later steps get the same
// error without running uv again, until Retry (the TUI's next run).
func TestVenvFailsOnce(t *testing.T) {
	f := newFakeEnv(t)
	f.script(filepath.Join(f.bin, "uv"), "echo 'no network' >&2\nexit 1\n")
	v := f.venv()
	var out bytes.Buffer
	first := v.Ensure(context.Background(), &out)
	if first == nil || !strings.Contains(first.Error(), "no network") {
		t.Fatalf("Ensure = %v", first)
	}
	if _, err := (Language{Venv: v}).Toolchain(); err == nil || err.Error() != first.Error() {
		t.Errorf("Toolchain after a failure: %v", err)
	}
	if err := v.Ensure(context.Background(), &out); err == nil || err.Error() != first.Error() {
		t.Errorf("second Ensure: %v", err)
	}
	if n := strings.Count(f.logged(), "uv venv"); n != 1 {
		t.Errorf("uv venv ran %d times:\n%s", n, f.logged())
	}
	if n := strings.Count(out.String(), "Creating"); n != 1 {
		t.Errorf("creation reported %d times:\n%s", n, out.String())
	}

	// The user fixes the cause; the next run tries again.
	f.uv("3.12.4")
	v.Retry()
	if err := v.Ensure(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(f.logged(), "uv venv"); n != 2 {
		t.Errorf("uv venv ran %d times after Retry:\n%s", n, f.logged())
	}
}
