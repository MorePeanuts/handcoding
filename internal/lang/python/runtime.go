package python

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// PthFile is the file in a virtual environment's site-packages that puts
// the runtime directory on sys.path, for editors and for code run with the
// environment's Python outside hc (see Venv.Runtime).
const PthFile = "hc_runtime.pth"

// runtimeHash names the directory of this hc's runtime files: a hash of
// their content, so that hc processes of different versions sharing a
// workspace each run their own.
var runtimeHash = func() string {
	h := sha256.New()
	for _, src := range [][]byte{runtimeSrc, checkerSrc, generatorSrc} {
		h.Write(src)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}()

// runtimeRoot is the directory that holds the shared runtime.
func (l Language) runtimeRoot() string {
	if l.Runtime != "" {
		return l.Runtime
	}
	return filepath.Join(os.TempDir(), "hc-runtime", "python")
}

// runtimeDir returns the directory of this hc's runtime files
// (hc_runtime.py, hc_checker.py, hc_generator.py), which every run imports
// hc_runtime from, writing it if it is missing. It also keeps a copy of
// hc_runtime.py in the root directory itself, the one editors resolve
// (through PthFile or pyright's extraPaths), which the newest hc that
// created its directory wrote.
func (l Language) runtimeDir() (string, error) {
	root := l.runtimeRoot()
	dir := filepath.Join(root, runtimeHash)
	files := map[string][]byte{runtimeFile: runtimeSrc, checkerFile: checkerSrc, genFile: generatorSrc}
	if complete(dir, files) {
		return dir, writeEditorCopy(root, false)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	// Built beside its place and renamed into it, so that a run never
	// imports a half-written runtime; another process may get there first.
	tmp, err := os.MkdirTemp(root, ".tmp-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(tmp, name), data, 0o644); err != nil {
			return "", err
		}
	}
	if err := os.Rename(tmp, dir); err != nil && !complete(dir, files) {
		// Explicit cache cleanup preserves unknown files and directories, so
		// an incomplete hash directory can remain. Repair its known files
		// atomically without removing those entries. Concurrent writers use
		// identical sources for this hash; complete still gates readers.
		info, statErr := os.Lstat(dir)
		if statErr != nil {
			return "", statErr
		}
		if !info.IsDir() {
			return "", fmt.Errorf("runtime %s is not a directory", dir)
		}
		for name := range files {
			if err := os.Rename(filepath.Join(tmp, name), filepath.Join(dir, name)); err != nil {
				return "", err
			}
		}
	}
	return dir, writeEditorCopy(root, true)
}

// complete reports whether dir holds every generated file. A runtime hash
// identifies immutable sources; missing files are repaired after cleanup.
func complete(dir string, files map[string][]byte) bool {
	for name := range files {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			return false
		}
	}
	return true
}

// writeEditorCopy writes hc_runtime.py into root when it is missing, or,
// with replace, when it differs from this hc's.
func writeEditorCopy(root string, replace bool) error {
	p := filepath.Join(root, runtimeFile)
	old, err := os.ReadFile(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return err
	case !replace || bytes.Equal(old, runtimeSrc):
		return nil
	}
	f, err := os.CreateTemp(root, "."+runtimeFile+".*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(runtimeSrc)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(f.Name(), 0o644)
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), p)
}
