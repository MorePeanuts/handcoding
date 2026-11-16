package javascript

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/MorePeanuts/handcoding/internal/lang"
)

var dependencyHash = func() string {
	h := sha256.New()
	for _, name := range []string{"package.json", "package-lock.json"} {
		b, _ := files.ReadFile("dependencies/" + name)
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:20]
}()

type environmentManifest struct {
	Lock  string           `json:"lock"`
	Files map[string]int64 `json:"files"`
}

func (l Language) environment(dir string) string {
	root := l.Environment
	if root == "" {
		root = filepath.Join(dir, "_hc_js", "node")
	}
	return filepath.Join(root, dependencyHash)
}
func (l Language) npm() string {
	if l.NPM != "" {
		return l.NPM
	}
	return "npm"
}

// EnvironmentReady reads only; diagnosis and scaffolding never install packages.
func (l Language) EnvironmentReady() bool {
	return l.Environment != "" && environmentReady(l.environment(""))
}
func environmentReady(dir string) bool {
	b, err := os.ReadFile(filepath.Join(dir, ".complete.json"))
	if err != nil {
		return false
	}
	var m environmentManifest
	if json.Unmarshal(b, &m) != nil || m.Lock != dependencyHash || len(m.Files) == 0 {
		return false
	}
	for name, size := range m.Files {
		if !fs.ValidPath(name) {
			return false
		}
		info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil || !info.Mode().IsRegular() || info.Size() != size {
			return false
		}
	}
	return true
}
func (l Language) Setup(ctx context.Context, w io.Writer) error {
	if l.Environment == "" {
		return nil
	} // Standalone drivers prepare at Build, once dir is known.
	return l.ensureEnvironment(ctx, "", w, false)
}
func (l Language) SyncEnvironment(ctx context.Context, w io.Writer) error {
	if l.Environment == "" {
		return fmt.Errorf("JS/TS environment needs a workspace")
	}
	return l.ensureEnvironment(ctx, "", w, true)
}
func (l Language) ensureEnvironment(ctx context.Context, dir string, w io.Writer, force bool) error {
	target := l.environment(dir)
	if !force && environmentReady(target) {
		return nil
	}
	if w == nil {
		w = os.Stderr
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	// Cross-process lock; a second first-use command waits, then reuses the result.
	lock, err := os.OpenFile(target+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	for {
		err = unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EINTR) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	defer func() { _ = unix.Flock(int(lock.Fd()), unix.LOCK_UN) }() // Closing lock also releases it.
	if !force && environmentReady(target) {
		return nil
	}
	node, version, err := tool(ctx, l.node())
	if err != nil {
		return fmt.Errorf("prepare JS/TS dependencies: install Node.js 22+ or set javascript.command: %w", err)
	}
	if !lang.AtLeast(version, MinVersion) {
		return fmt.Errorf("prepare JS/TS dependencies: Node.js %s is older than 22", version)
	}
	npm, err := lang.ResolveExecutable(l.npm())
	if err != nil {
		return fmt.Errorf("prepare JS/TS dependencies: install npm or set javascript.npm, then run hc env sync --lang javascript: %w", err)
	}
	stage, err := os.MkdirTemp(filepath.Dir(target), ".install-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	for _, name := range []string{"package.json", "package-lock.json"} {
		b, _ := files.ReadFile("dependencies/" + name)
		if err := write(filepath.Join(stage, name), b); err != nil {
			return err
		}
	}
	fmt.Fprintln(w, "Preparing JS/TS libraries in .hc/node with npm (network may be needed once)…")
	installCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(installCtx, npm, "ci", "--ignore-scripts", "--no-audit", "--no-fund", "--prefer-offline")
	cmd.Dir, cmd.Stdout, cmd.Stderr = stage, w, w
	// npm's env-node launcher uses the configured Node, not an unrelated PATH version.
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(node)+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("JS/TS dependencies were not installed; existing environment kept. Check npm/network, then run hc env sync --lang javascript: %w", err)
	}
	lockBytes, _ := files.ReadFile("dependencies/package-lock.json")
	var locked struct {
		Packages map[string]struct {
			Version string `json:"version"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(lockBytes, &locked); err != nil {
		return err
	}
	for path, expected := range locked.Packages {
		if path == "" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(stage, filepath.FromSlash(path), "package.json"))
		var installed struct {
			Version string `json:"version"`
		}
		if err != nil || json.Unmarshal(b, &installed) != nil || installed.Version != expected.Version {
			return fmt.Errorf("npm did not install locked dependency %s@%s", path, expected.Version)
		}
	}
	m := environmentManifest{Lock: dependencyHash, Files: map[string]int64{}}
	err = filepath.WalkDir(stage, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("unexpected symlink in locked JS dependencies: %s", path)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unexpected dependency file: %s", path)
		}
		rel, err := filepath.Rel(stage, path)
		if err != nil {
			return err
		}
		m.Files[filepath.ToSlash(rel)] = info.Size()
		return nil
	})
	if err != nil {
		return err
	}
	// No completed marker is published without installed packages.
	if _, ok := m.Files["node_modules/lodash/lodash.js"]; !ok {
		return fmt.Errorf("npm completed without required JS libraries")
	}
	b, _ := json.Marshal(m)
	if err = write(filepath.Join(stage, ".complete.json"), b); err != nil {
		return err
	}
	if _, err = os.Stat(target); os.IsNotExist(err) {
		return os.Rename(stage, target)
	}
	// Repair from a clean install, atomically replacing owned files. Existing runs
	// retain the same paths and versions; unrelated files and older lock versions stay.
	for name := range m.Files {
		b, err := os.ReadFile(filepath.Join(stage, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		if err = write(filepath.Join(target, filepath.FromSlash(name)), b); err != nil {
			return err
		}
	}
	return write(filepath.Join(target, ".complete.json"), b)
}

func (l Language) diagnoseEnvironment(ctx context.Context) []lang.Diagnostic {
	if l.Environment == "" {
		return nil
	}
	d := lang.Diagnostic{Language: l.ID(), Role: "dependencies", Path: l.environment(""), Source: "workspace .hc/node", Status: "ok"}
	if l.EnvironmentReady() {
		return []lang.Diagnostic{d}
	}
	d.Status, d.Message = "uninitialized", "JS/TS dependencies are missing or incomplete; run hc env sync --lang "+l.ID()+" (first install may need network)"
	npm := lang.Diagnostic{Language: l.ID(), Role: "package manager", Command: l.npm(), Source: "PATH"}
	if strings.TrimSpace(l.NPM) != "" {
		npm.Source = "hc.yaml: javascript.npm"
	}
	if npm.Resolve() {
		if _, err := lang.Probe(ctx, npm.Path, nil, "--version"); err != nil {
			npm.Status = "error"
			npm.Message = err.Error()
		} else {
			npm.Status = "ok"
		}
	}
	return []lang.Diagnostic{d, npm}
}
