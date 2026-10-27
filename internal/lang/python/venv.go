package python

import (
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
// It is created on first use: with uv when it is installed, which downloads
// a Python if there is none, otherwise with the newest Python MinVersion+ on
// PATH. A virtual environment cannot be moved, so one whose interpreter no
// longer runs is created again.
type Venv struct {
	// Dir is the environment's directory.
	Dir string
	// Label names the environment in messages and the toolchain, e.g.
	// ".hc/venv".
	Label string
	// Pyright, when set, is a pyrightconfig.json pointing pyright at the
	// environment. It is written only if it does not exist.
	Pyright string
	// Log receives progress messages when Ensure is given no writer; nil
	// means os.Stderr.
	Log io.Writer

	mu    sync.Mutex
	ready bool
}

// Python is the environment's interpreter.
func (v *Venv) Python() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(v.Dir, "Scripts", "python.exe")
	}
	return filepath.Join(v.Dir, "bin", "python")
}

// Ensure creates the environment unless its interpreter runs. It checks
// once per process. Progress goes to w, or to Log if w is nil.
func (v *Venv) Ensure(ctx context.Context, w io.Writer) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.ready {
		return nil
	}
	w = v.writer(w)
	if _, err := version(ctx, v.Python()); err != nil {
		if _, err := os.Stat(v.Dir); err == nil {
			fmt.Fprintf(w, "%s no longer runs (moved workspace or removed Python?); creating it again\n", v.Label)
		}
		if err := v.create(ctx, w); err != nil {
			return err
		}
	}
	v.writePyright(w)
	v.ready = true
	return nil
}

// Rebuild creates the environment from scratch.
func (v *Venv) Rebuild(ctx context.Context, w io.Writer) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	w = v.writer(w)
	v.ready = false
	if err := v.create(ctx, w); err != nil {
		return err
	}
	v.writePyright(w)
	v.ready = true
	return nil
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
		return err
	}
	if err := os.MkdirAll(filepath.Dir(v.Dir), 0o755); err != nil {
		return err
	}
	var install []string
	if uv, err := exec.LookPath("uv"); err == nil {
		fmt.Fprintf(w, "Creating %s with uv (it downloads Python %s+ if none is installed)…\n", v.Label, MinVersion)
		if err := run(ctx, uv, "venv", "--no-project", "--python", ">="+MinVersion, v.Dir); err != nil {
			os.RemoveAll(v.Dir)
			return fmt.Errorf("create %s with uv: %w", v.Label, err)
		}
		install = []string{uv, "pip", "install", "--python", v.Python(), "debugpy"}
	} else {
		py, ver, err := findPython(ctx)
		if err != nil {
			return fmt.Errorf("cannot create %s: %w", v.Label, err)
		}
		fmt.Fprintf(w, "Creating %s with %s (Python %s)…\n", v.Label, py, ver)
		if err := run(ctx, py, "-m", "venv", v.Dir); err != nil {
			os.RemoveAll(v.Dir)
			return fmt.Errorf("create %s with %s (on Debian and Ubuntu this needs the python3-venv package): %w", v.Label, py, err)
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
	data, _ := json.MarshalIndent(map[string]string{
		"venvPath": filepath.ToSlash(rel),
		"venv":     filepath.Base(v.Dir),
	}, "", "  ")
	if err := os.WriteFile(v.Pyright, append(data, '\n'), 0o644); err != nil {
		fmt.Fprintf(w, "warning: %v\n", err)
	}
}

var pythonNameRE = regexp.MustCompile(`^python(3(\.\d+)?)?(\.exe)?$`)

// findPython returns the newest Python MinVersion+ among python3, python and
// python3.N on PATH.
func findPython(ctx context.Context) (path, ver string, err error) {
	var older []string
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
			case path == "" || !lang.AtLeast(ver, v):
				path, ver = p, v
			}
		}
	}
	if path != "" {
		return path, ver, nil
	}
	msg := fmt.Sprintf("no Python %s or later on PATH", MinVersion)
	if len(older) > 0 {
		msg += " (" + strings.Join(older, ", ") + ")"
	}
	return "", "", fmt.Errorf("%s; install uv (https://docs.astral.sh/uv/), which installs Python as well, or Python %s+, or set python.interpreter in hc.yaml", msg, MinVersion)
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
