package c

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
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

//go:embed dependencies/lock.json
var dependencyLock []byte

type dependency struct{ Name, URL, SHA256 string }

var dependencies = func() []dependency {
	var d []dependency
	if err := json.Unmarshal(dependencyLock, &d); err != nil {
		panic(err)
	}
	return d
}()
var dependencyKey = fmt.Sprintf("%x", sha256.Sum256(dependencyLock))[:20]

func (l Language) environment(dir string) string {
	root := l.Environment
	if root == "" {
		root = filepath.Join(dir, "_hc_c")
	}
	return filepath.Join(root, dependencyKey)
}
func ready(dir string, deps []dependency) bool {
	for _, d := range deps {
		p := filepath.Join(dir, d.Name)
		st, e := os.Lstat(p)
		if e != nil || !st.Mode().IsRegular() || st.Size() > 4<<20 {
			return false
		}
		b, e := os.ReadFile(p)
		if e != nil || fmt.Sprintf("%x", sha256.Sum256(b)) != d.SHA256 {
			return false
		}
	}
	return true
}
func (l Language) EnvironmentReady() bool {
	return l.Environment != "" && ready(l.environment(""), dependencies)
}
func (l Language) Setup(ctx context.Context, w io.Writer) error {
	if l.Environment == "" {
		return nil
	}
	return l.ensureEnvironment(ctx, "", w, dependencies)
}
func (l Language) SyncEnvironment(ctx context.Context, w io.Writer) error {
	if l.Environment == "" {
		return fmt.Errorf("c environment needs a workspace")
	}
	return l.Setup(ctx, w)
}
func (l Language) ensureEnvironment(ctx context.Context, dir string, w io.Writer, deps []dependency) error {
	target := l.environment(dir)
	if ready(target, deps) {
		return nil
	}
	if err := l.install(ctx, target, w, deps); err != nil {
		return fmt.Errorf("prepare C dependencies: %w; run hc env sync --lang c to retry", err)
	}
	return nil
}
func (l Language) install(ctx context.Context, target string, w io.Writer, deps []dependency) error {
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
	if ready(target, deps) {
		return nil
	}
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprintln(w, "Preparing yyjson and uthash in .hc/c (first use needs network)…")
	stage, err := os.MkdirTemp(filepath.Dir(target), ".c-install-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	client := &http.Client{Timeout: 2 * time.Minute}
	for _, d := range deps {
		if filepath.Base(d.Name) != d.Name {
			return fmt.Errorf("invalid dependency path")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.URL, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, (4<<20)+1))
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("%s: HTTP %d", d.Name, resp.StatusCode)
		}
		if len(data) > 4<<20 || fmt.Sprintf("%x", sha256.Sum256(data)) != d.SHA256 {
			return fmt.Errorf("%s: SHA-256 mismatch or oversized download", d.Name)
		}
		if err := os.WriteFile(filepath.Join(stage, d.Name), data, 0644); err != nil {
			return err
		}
	}
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return os.Rename(stage, target)
	}
	// All files have been verified before any existing file is replaced. Every
	// file has a pinned digest; concurrent builds can only see this same version.
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}
	for _, d := range deps {
		if err := os.Rename(filepath.Join(stage, d.Name), filepath.Join(target, d.Name)); err != nil {
			return err
		}
	}
	return nil
}
func (l Language) diagnoseEnvironment() []lang.Diagnostic {
	if l.Environment == "" {
		return nil
	}
	d := lang.Diagnostic{Language: "c", Role: "dependencies", Path: l.environment(""), Source: "workspace .hc/c", Status: "ok"}
	if !l.EnvironmentReady() {
		d.Status = "uninitialized"
		d.Message = "yyjson/uthash missing or corrupt; run hc env sync --lang c"
	}
	return []lang.Diagnostic{d}
}
