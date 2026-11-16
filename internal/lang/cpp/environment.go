package cpp

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"

	"github.com/MorePeanuts/handcoding/internal/lang"
)

// Only dependency metadata ships with hc; third-party source is downloaded
// into the workspace. A different digest gets its own directory so running
// hc versions never replace each other's dependency.
type dependency struct{ version, url, sha256 string }

var jsonDependency = dependency{
	version: "3.12.0",
	url:     "https://raw.githubusercontent.com/nlohmann/json/v3.12.0/single_include/nlohmann/json.hpp",
	sha256:  "aaf127c04cb31c406e5b04a63f1ae89369fccde6d8fa7cdda1ed4f32dfc5de63",
}

const maxHeaderSize = 2 << 20

func (l Language) environment(dir string, d dependency) string {
	root := l.Environment
	if root == "" {
		root = filepath.Join(dir, "_hc_cpp")
	}
	return filepath.Join(root, "json-"+d.version+"-"+d.sha256[:16])
}
func (l Language) includeDir(dir string) string {
	return filepath.Join(l.environment(dir, jsonDependency), "include")
}
func headerPath(dir string) string { return filepath.Join(dir, "include", "nlohmann", "json.hpp") }
func dependencyReady(dir string, d dependency) bool {
	info, err := os.Lstat(headerPath(dir))
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxHeaderSize {
		return false
	}
	f, err := os.Open(headerPath(dir))
	if err != nil {
		return false
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, maxHeaderSize+1))
	return err == nil && n <= maxHeaderSize && fmt.Sprintf("%x", h.Sum(nil)) == d.sha256
}

// EnvironmentReady is read-only, including when the environment is incomplete.
func (l Language) EnvironmentReady() bool {
	return l.Environment != "" && dependencyReady(l.environment("", jsonDependency), jsonDependency)
}
func (l Language) Setup(ctx context.Context, w io.Writer) error {
	if l.Environment == "" {
		return nil
	} // Standalone drivers know their directory at Build.
	return l.ensureEnvironment(ctx, "", w, jsonDependency)
}
func (l Language) SyncEnvironment(ctx context.Context, w io.Writer) error {
	if l.Environment == "" {
		return fmt.Errorf("cpp environment needs a workspace")
	}
	// The full checksum is checked even on reuse; a valid header needs no download.
	return l.Setup(ctx, w)
}
func (l Language) ensureEnvironment(ctx context.Context, dir string, w io.Writer, d dependency) error {
	target := l.environment(dir, d)
	if dependencyReady(target, d) {
		return nil
	}
	if err := l.installDependency(ctx, target, w, d); err != nil {
		return fmt.Errorf("prepare C++ JSON dependency: %w; check network access, then run hc env sync --lang cpp", err)
	}
	return nil
}
func (l Language) installDependency(ctx context.Context, target string, w io.Writer, d dependency) error {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
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
	defer func() { _ = unix.Flock(int(lock.Fd()), unix.LOCK_UN) }()
	if dependencyReady(target, d) {
		return nil
	}
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprintln(w, "Preparing C++ JSON library in .hc/cpp (downloading nlohmann/json "+d.version+")…")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download JSON header: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxHeaderSize+1))
	if err != nil {
		return err
	}
	if len(data) > maxHeaderSize {
		return fmt.Errorf("JSON header exceeds %d bytes", maxHeaderSize)
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != d.sha256 {
		return fmt.Errorf("JSON header SHA-256 mismatch")
	}
	// Publish only a fully downloaded and verified header. A failed download
	// leaves any existing header and unrelated files untouched.
	path := headerPath(target)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".json-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if err := temp.Chmod(0644); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}
func (l Language) diagnoseEnvironment() []lang.Diagnostic {
	if l.Environment == "" {
		return nil
	}
	d := lang.Diagnostic{Language: l.ID(), Role: "dependencies", Path: l.environment("", jsonDependency), Source: "workspace .hc/cpp", Version: jsonDependency.version, Status: "ok"}
	if !l.EnvironmentReady() {
		d.Status, d.Message = "uninitialized", "C++ JSON header is missing or corrupt; run hc env sync --lang cpp (download needs network)"
	}
	return []lang.Diagnostic{d}
}
