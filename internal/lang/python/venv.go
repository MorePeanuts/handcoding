package python

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/MorePeanuts/handcoding/internal/lang"
)

// Venv is a virtual environment hc manages for running solutions, so they
// do not depend on the python3 on PATH (macOS ships 3.9) and debugpy can be
// installed without touching the user's Python (PEP 668 forbids that on
// Homebrew and recent Debian).
//
// It is created by hc init and hc pack init, or else on first use: with uv
// when it is installed, which downloads a Python if there is none, otherwise
// with the newest Python MinVersion+ on PATH (or of Version, when it is
// set), without pip if its venv module cannot install it (Debian's
// python3.X-venv is missing): judging needs no third-party package, only
// headless debugging (debugpy) does. With neither, it fails with a
// *SetupError that says how to get one. A virtual environment cannot be
// moved, so one whose interpreter no longer runs is created again, and so
// is one older than MinVersion (made by an older hc) when a Python
// MinVersion+ can be had.
type Venv struct {
	// Dir is the environment's directory.
	Dir string
	// Label names the environment in messages and the toolchain, e.g.
	// ".hc/venv".
	Label string
	// Pyright, when set, is a pyrightconfig.json pointing pyright at the
	// environment. It is written only if it does not exist.
	Pyright string
	// Ty, when set, is a ty.toml pointing ty at the environment, which ty
	// does not find by itself (it looks for .venv). It is written only if
	// it does not exist.
	Ty string
	// Version, when set, is the minor version of Python the environment
	// has, such as "3.12". A pack checkout's has MinVersion, which the
	// pack's content must run on. An environment of another version is
	// created again when one of Version can be had.
	Version string
	// Log receives progress messages when Ensure is given no writer; nil
	// means os.Stderr.
	Log io.Writer
	// Runtime, when set, is the shared runtime's directory (see
	// Language.Runtime), which the environment puts on sys.path with a
	// PthFile, and the pyrightconfig.json it writes lists in extraPaths, so
	// that editors resolve `from hc_runtime import …` in solutions.
	Runtime string

	mu    sync.Mutex
	ready bool
	// failed is the error of the last attempt to create the environment,
	// returned again until Retry.
	failed error
}

// Python is the environment's interpreter.
func (v *Venv) Python() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(v.Dir, "Scripts", "python.exe")
	}
	return filepath.Join(v.Dir, "bin", "python")
}

// Ensure creates the environment unless its interpreter runs. It checks
// once per process, and tries to create the environment once: after a
// failure it returns the same error until Retry. Progress goes to w, or to
// Log if w is nil.
func (v *Venv) Ensure(ctx context.Context, w io.Writer) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.ready {
		return nil
	}
	if v.failed != nil {
		return v.failed
	}
	w = v.writer(w)
	ver, err := version(ctx, v.Python())
	recreate := err != nil
	switch {
	case err != nil:
		if _, err := os.Stat(v.Dir); err == nil {
			fmt.Fprintf(w, "%s no longer runs (moved workspace or removed Python?); creating it again\n", v.Label)
		}
	case v.Version != "" && !sameMinor(ver, v.Version) && v.canGet(ctx):
		fmt.Fprintf(w, "%s has Python %s, not %s; creating it again\n", v.Label, ver, v.Version)
		recreate = true
	case v.Version == "" && !lang.AtLeast(ver, MinVersion) && v.canGet(ctx):
		// Made by an hc of an older minimum: it cannot run generators
		// or checkers.
		fmt.Fprintf(w, "%s has Python %s, older than %s; creating it again\n", v.Label, ver, MinVersion)
		recreate = true
	}
	if recreate {
		if err := v.create(ctx, w); err != nil {
			v.failed = err
			return err
		}
	}
	v.writeEditorConfigs(w)
	v.ready = true
	return nil
}

// Rebuild creates the environment from scratch.
func (v *Venv) Rebuild(ctx context.Context, w io.Writer) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	w = v.writer(w)
	v.ready, v.failed = false, nil
	if err := v.create(ctx, w); err != nil {
		v.failed = err
		return err
	}
	v.writeEditorConfigs(w)
	v.ready = true
	return nil
}

// Retry lets the next Ensure try again to create the environment after a
// failed attempt. A command tries once, so that its steps do not each
// report the same failure; a long-running front end such as the TUI calls
// Retry before each run, as the user may have fixed the cause (installed
// uv, say) in the meantime.
func (v *Venv) Retry() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.failed = nil
}

func (v *Venv) writer(w io.Writer) io.Writer {
	switch {
	case w != nil:
		return w
	case v.Log != nil:
		return v.Log
	}
	return os.Stderr
}

func (v *Venv) create(ctx context.Context, w io.Writer) error {
	if err := os.RemoveAll(v.Dir); err != nil {
		return &SetupError{Env: v.Label, Cause: err}
	}
	if err := os.MkdirAll(filepath.Dir(v.Dir), 0o755); err != nil {
		return &SetupError{Env: v.Label, Cause: err}
	}
	var install []string
	if uv, err := exec.LookPath("uv"); err == nil {
		want, label := ">="+MinVersion, MinVersion+"+"
		if v.Version != "" {
			want, label = v.Version, v.Version
		}
		fmt.Fprintf(w, "Creating %s with uv (it downloads Python %s if none is installed)…\n", v.Label, label)
		if err := run(ctx, uv, "venv", "--no-project", "--python", want, v.Dir); err != nil {
			os.RemoveAll(v.Dir)
			return &SetupError{Env: v.Label, Tool: "uv", Cause: err}
		}
		install = []string{uv, "pip", "install", "--python", v.Python(), "debugpy"}
	} else {
		py, ver, found := findPython(ctx, v.Version)
		if py == "" {
			return &SetupError{Env: v.Label, Found: found}
		}
		fmt.Fprintf(w, "Creating %s with %s (Python %s)…\n", v.Label, py, ver)
		if v.Version != "" && !sameMinor(ver, v.Version) {
			fmt.Fprintf(w, "warning: no Python %s on PATH, so the pack's Python is not run on the oldest version the protocol supports here, only in its CI; install uv, or Python %s\n", v.Version, v.Version)
		}
		if err := run(ctx, py, "-m", "venv", v.Dir); err != nil {
			// Debian and Ubuntu (WSL included) ship ensurepip, which
			// venv installs pip with, apart, as python3.X-venv. Judging
			// needs no third-party package, so an environment without
			// pip works; only debugpy is missing.
			os.RemoveAll(v.Dir)
			if err2 := run(ctx, py, "-m", "venv", "--without-pip", v.Dir); err2 != nil {
				os.RemoveAll(v.Dir)
				return &SetupError{Env: v.Label, Tool: py, Cause: err}
			}
			minor := minorOf(ver)
			fmt.Fprintf(w, "warning: %s -m venv cannot install pip, so %s has no debugpy and headless debugging (hc debug --headless) will not work; "+
				"to get it, install the python%s-venv package (on Debian and Ubuntu: sudo apt install python%s-venv) or uv, then run `hc env sync`\n",
				py, v.Label, minor, minor)
			return nil
		}
		install = []string{v.Python(), "-m", "pip", "install", "--disable-pip-version-check", "debugpy"}
	}
	// Only headless debugging needs debugpy, so judging works offline even
	// if this fails.
	fmt.Fprintf(w, "Installing debugpy into %s (needs network access, once)…\n", v.Label)
	if err := run(ctx, install[0], install[1:]...); err != nil {
		fmt.Fprintf(w, "warning: debugpy not installed, so headless debugging will not work; retry with `hc env sync`: %v\n", err)
	}
	return nil
}

// minorOf is the minor version of version ver: 3.12 of 3.12.4.
func minorOf(ver string) string {
	if parts := strings.SplitN(ver, ".", 3); len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return ver
}

// SetupError is a failure to create the virtual environment. Every command
// that needs Python reports it, prefixed with why it does. Its message ends
// with what to do: without Cause, when there was neither uv nor a Python
// MinVersion+ on PATH, how to get one; then run `hc env sync`.
type SetupError struct {
	// Env names the environment, e.g. ".hc/venv".
	Env string
	// Tool is what failed to create it: "uv", or the Python found on PATH.
	Tool string
	// Found are the Pythons on PATH too old to use, e.g.
	// "/usr/bin/python3 is 3.9.6".
	Found []string
	// Cause is why Tool failed, nil when there was none.
	Cause error
}

func (e *SetupError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v\nFix that, then run `hc env sync`; or set python.interpreter in hc.yaml to a Python %s or later, which hc then uses as it is.", e.head(), e.Cause, MinVersion)
	}
	return e.Summary() + ".\n\n" + Guide(runtime.GOOS)
}

// Summary is the first line of the message, without what to do: for
// reports that repeat the error, such as hc validate's findings, which
// show the whole message once.
func (e *SetupError) Summary() string {
	if e.Cause != nil {
		cause, _, _ := strings.Cut(e.Cause.Error(), "\n")
		return e.head() + ": " + cause
	}
	s := fmt.Sprintf("%s: there is no uv, and no Python %s or later on PATH", e.head(), MinVersion)
	if len(e.Found) > 0 {
		s += " (" + strings.Join(e.Found, ", ") + ")"
	}
	return s
}

func (e *SetupError) head() string {
	if e.Tool != "" {
		return "cannot create " + e.Env + " with " + e.Tool
	}
	return "cannot create " + e.Env
}

func (e *SetupError) Unwrap() error { return e.Cause }

// UVInstallURL is uv's installation guide.
const UVInstallURL = "https://docs.astral.sh/uv/getting-started/installation/"

// Guide tells the user, on the operating system goos, why hc needs Python
// and how to get it. hc prints the commands and never runs an installer.
func Guide(goos string) string {
	var cmds []string
	switch goos {
	case "darwin":
		cmds = []string{"brew install uv", "curl -LsSf https://astral.sh/uv/install.sh | sh"}
	case "windows":
		cmds = []string{"winget install --id=astral-sh.uv -e", `powershell -ExecutionPolicy ByPass -c "irm https://astral.sh/uv/install.ps1 | iex"`}
	default:
		cmds = []string{"curl -LsSf https://astral.sh/uv/install.sh | sh"}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "hc runs Python solutions, and the generators and checkers that most problems have, whatever your solution's language, in Python %s or later.\n", MinVersion)
	b.WriteString("Install one of these, then run `hc env sync`:\n")
	fmt.Fprintf(&b, "  - uv (recommended: it installs Python as well, see %s), with one of\n", UVInstallURL)
	for _, c := range cmds {
		b.WriteString("      " + c + "\n")
	}
	fmt.Fprintf(&b, "  - Python %s or later, on PATH\n", MinVersion)
	fmt.Fprintf(&b, "Or set python.interpreter in hc.yaml to a Python %s or later, which hc then uses as it is.", MinVersion)
	return b.String()
}

// run runs a command, returning its output with the error.
func run(ctx context.Context, name string, args ...string) error {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return fmt.Errorf("%w\n%s", err, s)
		}
		return err
	}
	return nil
}

func (v *Venv) writePyright(w io.Writer) {
	if v.Pyright == "" {
		return
	}
	if _, err := os.Stat(v.Pyright); !errors.Is(err, os.ErrNotExist) {
		return
	}
	rel, err := filepath.Rel(filepath.Dir(v.Pyright), filepath.Dir(v.Dir))
	if err != nil {
		return
	}
	config := map[string]any{
		"venvPath": filepath.ToSlash(rel),
		"venv":     filepath.Base(v.Dir),
	}
	if rt, err := filepath.Rel(filepath.Dir(v.Pyright), v.Runtime); v.Runtime != "" && err == nil {
		config["extraPaths"] = []string{filepath.ToSlash(rt)}
	}
	data, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(v.Pyright, append(data, '\n'), 0o644); err != nil {
		fmt.Fprintf(w, "warning: %v\n", err)
	}
}

var pythonNameRE = regexp.MustCompile(`^python(3(\.\d+)?)?(\.exe)?$`)

// canGet reports whether an environment of Version, or without Version of
// MinVersion or later, can be created: uv downloads any version, or one is
// on PATH.
func (v *Venv) canGet(ctx context.Context) bool {
	if _, err := exec.LookPath("uv"); err == nil {
		return true
	}
	py, ver, _ := findPython(ctx, v.Version)
	return py != "" && (v.Version == "" || sameMinor(ver, v.Version))
}

// sameMinor reports whether version ver (3.12.4) is of minor version minor
// (3.12).
func sameMinor(ver, minor string) bool {
	return ver == minor || strings.HasPrefix(ver, minor+".")
}

// findPython returns the newest Python MinVersion+ among python3, python and
// python3.N on PATH: of minor version want if there is one and want is set.
// Without one, path is empty, and older lists the Pythons on PATH too old to
// use ("/usr/bin/python3 is 3.9.6").
func findPython(ctx context.Context, want string) (path, ver string, older []string) {
	var wantPath, wantVer string
	seen := map[string]bool{}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !pythonNameRE.MatchString(e.Name()) {
				continue
			}
			p := filepath.Join(dir, e.Name())
			if seen[p] {
				continue
			}
			seen[p] = true
			v, err := version(ctx, p)
			switch {
			case err != nil:
			case !lang.AtLeast(v, MinVersion):
				older = append(older, fmt.Sprintf("%s is %s", p, v))
			default:
				if path == "" || !lang.AtLeast(ver, v) {
					path, ver = p, v
				}
				if want != "" && sameMinor(v, want) && (wantPath == "" || !lang.AtLeast(wantVer, v)) {
					wantPath, wantVer = p, v
				}
			}
		}
	}
	if wantPath != "" {
		return wantPath, wantVer, nil
	}
	return path, ver, older
}

const versionScript = "import sys; print('.'.join(map(str, sys.version_info[:3])))"

// version runs the interpreter py to ask its version.
func version(ctx context.Context, py string) (string, error) {
	out, err := exec.CommandContext(ctx, py, "-c", versionScript).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// writeEditorConfigs points editors at the environment and the shared
// runtime: pyright's and ty's configuration, unless they exist, and the
// PthFile.
func (v *Venv) writeEditorConfigs(w io.Writer) {
	v.writePyright(w)
	v.writeTy(w)
	v.writePth(w)
}

// writeTy writes Ty, unless it exists.
func (v *Venv) writeTy(w io.Writer) {
	if v.Ty == "" {
		return
	}
	if _, err := os.Stat(v.Ty); !errors.Is(err, os.ErrNotExist) {
		return
	}
	base := filepath.Dir(v.Ty)
	py, err := filepath.Rel(base, v.Dir)
	if err != nil {
		return
	}
	var b strings.Builder
	b.WriteString("# Written by hc: ty, the type checker, uses the Python environment hc manages.\n")
	fmt.Fprintf(&b, "[environment]\npython = %s\n", pyString(filepath.ToSlash(py)))
	if rt, err := filepath.Rel(base, v.Runtime); v.Runtime != "" && err == nil {
		fmt.Fprintf(&b, "extra-paths = [%s]\n", pyString(filepath.ToSlash(rt)))
	}
	if err := os.WriteFile(v.Ty, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintf(w, "warning: %v\n", err)
	}
}

// writePth writes the PthFile that puts Runtime on sys.path into the
// environment's site-packages, unless it is there.
func (v *Venv) writePth(w io.Writer) {
	if v.Runtime == "" {
		return
	}
	data := []byte(v.Runtime + "\n")
	for _, dir := range v.sitePackages() {
		p := filepath.Join(dir, PthFile)
		if old, err := os.ReadFile(p); err == nil && bytes.Equal(old, data) {
			continue
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			fmt.Fprintf(w, "warning: %v\n", err)
		}
	}
}

// sitePackages returns the environment's site-packages directories.
func (v *Venv) sitePackages() []string {
	pattern := filepath.Join(v.Dir, "lib", "python3*", "site-packages")
	if runtime.GOOS == "windows" {
		pattern = filepath.Join(v.Dir, "Lib", "site-packages")
	}
	dirs, _ := filepath.Glob(pattern)
	return dirs
}
