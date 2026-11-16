package lang

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ExecutableName makes paths relative to the workspace, leaving bare command
// names for PATH lookup. It never interprets a shell command or expands variables.
func ExecutableName(root, command string) string {
	if root != "" && !filepath.IsAbs(command) && strings.ContainsRune(command, filepath.Separator) {
		return filepath.Join(root, command)
	}
	return command
}

func ResolveExecutable(command string) (string, error) {
	path, err := exec.LookPath(command)
	if err != nil {
		return "", err
	}
	return filepath.Abs(path)
}

// Diagnostic is one read-only capability check. A version probe does not
// guarantee that every program compiles or that a debugger can attach.
type Diagnostic struct {
	Language string `json:"language"`
	Role     string `json:"role"`
	Command  string `json:"command"`
	Source   string `json:"source"`
	Path     string `json:"path,omitempty"`
	Version  string `json:"version,omitempty"`
	Minimum  string `json:"minimum,omitempty"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
}

type DiagnosticOptions struct {
	Debug    bool
	Headless bool
}

// Diagnoser must not call Setup, prepare files, install tools or run solutions.
type Diagnoser interface {
	Diagnose(context.Context, DiagnosticOptions) []Diagnostic
}

// Probe runs only the requested tool probe, bounded even when the caller has no
// deadline. WaitDelay also bounds waiting on inherited pipes from wrappers.
func Probe(ctx context.Context, path string, env []string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = env
	cmd.WaitDelay = 100 * time.Millisecond
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("probe: %w", ctx.Err())
	}
	if err != nil {
		return nil, fmt.Errorf("probe failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func (d *Diagnostic) Resolve() bool {
	path, err := ResolveExecutable(d.Command)
	if err != nil {
		d.Status, d.Message = "missing", err.Error()
		if !errors.Is(err, exec.ErrNotFound) && !errors.Is(err, os.ErrNotExist) {
			d.Status = "error"
		}
		return false
	}
	d.Path = path
	return true
}

func (d *Diagnostic) CheckVersion() {
	d.Status = "ok"
	if d.Minimum != "" && !AtLeast(d.Version, d.Minimum) {
		d.Status = "too_old"
		d.Message = fmt.Sprintf("requires %s %s or later", d.Language, d.Minimum)
	}
}
