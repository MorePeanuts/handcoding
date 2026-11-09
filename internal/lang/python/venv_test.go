package python

import (
	"bytes"
	"context"
	"errors"
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
		f.bin+"/uv venv --no-project --python >=3.12 "+v.Dir,
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
	f.python(filepath.Join(f.bin, "python"), "3.11.9")
	f.python(filepath.Join(f.bin, "python3.12"), "3.12.1")
	f.python(filepath.Join(f.bin, "python3.13"), "3.13.2")
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
		f.bin+"/python3.13 -m venv "+v.Dir,
		v.Python()+" -m pip install --disable-pip-version-check debugpy",
	)
	if strings.Contains(f.logged(), "python3-config") {
		t.Error("ran python3-config")
	}
	if data, _ := os.ReadFile(v.Pyright); string(data) != "{}\n" {
		t.Errorf("pyrightconfig.json overwritten: %s", data)
	}
}

// With Version, the environment has that minor version: uv is asked for
// it, an environment of another version is created again, and without uv
// the newest of that version on PATH creates it.
func TestVenvVersion(t *testing.T) {
	f := newFakeEnv(t)
	f.uv("3.13.4")
	v := f.venv()
	if err := v.Ensure(context.Background(), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	f.uv("3.12.11")
	v = f.venv()
	v.Version = "3.12"
	var out bytes.Buffer
	if err := v.Ensure(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	f.wantLogged(f.bin + "/uv venv --no-project --python 3.12 " + v.Dir)
	if !strings.Contains(out.String(), "has Python 3.13.4, not 3.12; creating it again") {
		t.Errorf("version change not reported:\n%s", out.String())
	}
	if ver, err := version(context.Background(), v.Python()); err != nil || ver != "3.12.11" {
		t.Errorf("environment: %q, %v", ver, err)
	}

	f = newFakeEnv(t)
	f.python(filepath.Join(f.bin, "python3"), "3.13.1")
	f.python(filepath.Join(f.bin, "python3.12"), "3.12.4")
	f.python(filepath.Join(f.bin, "python3.11"), "3.11.2")
	v = f.venv()
	v.Version = "3.12"
	if err := v.Ensure(context.Background(), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	f.wantLogged(f.bin + "/python3.12 -m venv " + v.Dir)
}

// Without a Python of Version, the newest creates the environment, with a
// warning, and is kept: nothing better can be had.
func TestVenvVersionMissing(t *testing.T) {
	f := newFakeEnv(t)
	f.python(filepath.Join(f.bin, "python3"), "3.13.1")
	v := f.venv()
	v.Version = "3.12"
	var out bytes.Buffer
	if err := v.Ensure(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	f.wantLogged(f.bin + "/python3 -m venv " + v.Dir)
	if !strings.Contains(out.String(), "warning: no Python 3.12 on PATH") {
		t.Errorf("no warning:\n%s", out.String())
	}
	before := f.logged()
	v = f.venv()
	v.Version = "3.12"
	out.Reset()
	if err := v.Ensure(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	if strings.Count(f.logged(), "-m venv") != strings.Count(before, "-m venv") || out.Len() != 0 {
		t.Errorf("environment created again:\n%s\n%s", f.logged(), out.String())
	}
}

// existing gives f's workspace an environment whose Python reports
// version ver, as an older hc would have left it.
func (f *fakeEnv) existing(ver string) *Venv {
	f.t.Helper()
	v := f.venv()
	if err := os.MkdirAll(filepath.Dir(v.Python()), 0o755); err != nil {
		f.t.Fatal(err)
	}
	f.python(v.Python(), ver)
	return v
}

// Without Version (a practice workspace), an environment older than
// MinVersion, made by an older hc, is created again when a Python
// MinVersion+ can be had: with uv, or one on PATH. Otherwise it is kept.
func TestVenvBelowMinimum(t *testing.T) {
	f := newFakeEnv(t)
	f.uv("3.12.4")
	v := f.existing("3.10.14")
	var out bytes.Buffer
	if err := v.Ensure(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	f.wantLogged(f.bin + "/uv venv --no-project --python >=3.12 " + v.Dir)
	if !strings.Contains(out.String(), ".hc/venv has Python 3.10.14, older than 3.12; creating it again\n") {
		t.Errorf("recreation not reported:\n%s", out.String())
	}
	if ver, err := version(context.Background(), v.Python()); err != nil || ver != "3.12.4" {
		t.Errorf("environment: %q, %v", ver, err)
	}

	// Without uv, a Python 3.12+ on PATH creates it.
	f = newFakeEnv(t)
	f.python(filepath.Join(f.bin, "python3"), "3.11.9")
	f.python(filepath.Join(f.bin, "python3.13"), "3.13.2")
	v = f.existing("3.11.2")
	out.Reset()
	if err := v.Ensure(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	f.wantLogged(f.bin + "/python3.13 -m venv " + v.Dir)
	if !strings.Contains(out.String(), "has Python 3.11.2, older than 3.12; creating it again") {
		t.Errorf("recreation not reported:\n%s", out.String())
	}

	// With neither, the old environment is kept, silently: nothing better
	// can be had.
	f = newFakeEnv(t)
	f.python(filepath.Join(f.bin, "python3"), "3.11.9")
	v = f.existing("3.10.14")
	out.Reset()
	if err := v.Ensure(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(f.logged(), " venv ") || out.Len() != 0 {
		t.Errorf("environment created again:\n%s\n%s", f.logged(), out.String())
	}
	if ver, err := version(context.Background(), v.Python()); err != nil || ver != "3.10.14" {
		t.Errorf("environment: %q, %v", ver, err)
	}

	// One of MinVersion or later is kept, even when uv could make another.
	f = newFakeEnv(t)
	f.uv("3.13.4")
	v = f.existing("3.12.0")
	out.Reset()
	if err := v.Ensure(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(f.logged(), "uv venv") || out.Len() != 0 {
		t.Errorf("environment created again:\n%s\n%s", f.logged(), out.String())
	}
}

// Without uv or a supported Python, the error is a *SetupError whose
// message says what was found and how to get Python.
func TestVenvNoPython(t *testing.T) {
	f := newFakeEnv(t)
	f.python(filepath.Join(f.bin, "python3"), "3.11.9")
	v := f.venv()
	err := v.Ensure(context.Background(), &bytes.Buffer{})
	se, ok := errors.AsType[*SetupError](err)
	if !ok {
		t.Fatalf("Ensure = %v, want a *SetupError", err)
	}
	if se.Cause != nil || len(se.Found) != 1 || se.Found[0] != f.bin+"/python3 is 3.11.9" {
		t.Errorf("SetupError %+v", se)
	}
	// The first line, without the guide.
	if s := se.Summary(); s != "cannot create .hc/venv: there is no uv, and no Python 3.12 or later on PATH ("+f.bin+"/python3 is 3.11.9)" {
		t.Errorf("Summary = %q", s)
	}
	for _, want := range []string{
		"cannot create .hc/venv: there is no uv, and no Python 3.12 or later on PATH (" + f.bin + "/python3 is 3.11.9).\n",
		"generators and checkers", UVInstallURL, "python.interpreter", "then run `hc env sync`",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q:\n%v", want, err)
		}
	}
	if _, err := (Language{Venv: v}).Toolchain(); err == nil {
		t.Error("toolchain without an environment")
	}
}

// The guide gives the commands that install uv on each system.
func TestGuide(t *testing.T) {
	for goos, want := range map[string][]string{
		"darwin":  {"      brew install uv\n", "      curl -LsSf https://astral.sh/uv/install.sh | sh\n"},
		"linux":   {"      curl -LsSf https://astral.sh/uv/install.sh | sh\n"},
		"windows": {"      winget install --id=astral-sh.uv -e\n", `irm https://astral.sh/uv/install.ps1 | iex`},
	} {
		g := Guide(goos)
		for _, w := range append(want, "Python 3.12 or later", UVInstallURL, "`hc env sync`", "python.interpreter") {
			if !strings.Contains(g, w) {
				t.Errorf("%s: guide lacks %q:\n%s", goos, w, g)
			}
		}
		if goos != "darwin" && strings.Contains(g, "brew") {
			t.Errorf("%s: guide names brew:\n%s", goos, g)
		}
	}
}

// A Python whose venv cannot install pip (Debian's python3.X-venv missing)
// creates the environment without it, and hc warns that only debugpy is
// missing.
func TestVenvWithoutPip(t *testing.T) {
	f := newFakeEnv(t)
	f.script(filepath.Join(f.bin, "python3"), `case "$1" in
-c) echo 3.12.3 ;;
-m) if [ "$3" = --without-pip ]; then /bin/mkdir -p "$4/bin" && /bin/cp "$0" "$4/bin/python"
    else /bin/mkdir -p "$3/bin"; echo "ensurepip is not available" >&2; exit 1; fi ;;
esac
`)
	v := f.venv()
	var out bytes.Buffer
	if err := v.Ensure(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	f.wantLogged(
		f.bin+"/python3 -m venv "+v.Dir,
		f.bin+"/python3 -m venv --without-pip "+v.Dir,
	)
	if strings.Contains(f.logged(), "pip install") {
		t.Errorf("installed debugpy without pip:\n%s", f.logged())
	}
	for _, want := range []string{"warning: " + f.bin + "/python3 -m venv cannot install pip", "headless debugging", "python3.12-venv", "uv", "`hc env sync`"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	if ver, err := version(context.Background(), v.Python()); err != nil || ver != "3.12.3" {
		t.Errorf("environment: %q, %v", ver, err)
	}

	// A venv that fails without pip too is a *SetupError naming the Python
	// and its output.
	f = newFakeEnv(t)
	f.script(filepath.Join(f.bin, "python3"), `case "$1" in
-c) echo 3.12.3 ;;
-m) echo "venv is broken" >&2; exit 1 ;;
esac
`)
	err := f.venv().Ensure(context.Background(), &bytes.Buffer{})
	if se, ok := errors.AsType[*SetupError](err); !ok || se.Tool != f.bin+"/python3" || !strings.Contains(err.Error(), "venv is broken") || !strings.Contains(err.Error(), "`hc env sync`") {
		t.Errorf("Ensure = %v", err)
	} else if s := se.Summary(); s != "cannot create .hc/venv with "+f.bin+"/python3: exit status 1" {
		// The first line of the cause, without what to do.
		t.Errorf("Summary = %q", s)
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
