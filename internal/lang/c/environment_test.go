package c

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/MorePeanuts/handcoding/internal/lang"
)

func TestEnvironmentLifecycle(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	var bad atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if bad.Load() {
			fmt.Fprint(w, "corrupt")
			return
		}
		fmt.Fprint(w, r.URL.Path)
	}))
	defer server.Close()
	deps := []dependency{}
	for _, n := range []string{"yyjson.h", "yyjson.c", "uthash.h"} {
		deps = append(deps, dependency{Name: n, URL: server.URL + "/" + n, SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("/"+n)))})
	}
	l := Language{Environment: t.TempDir()}
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for range 6 {
		wg.Go(func() { errs <- l.ensureEnvironment(context.Background(), "", io.Discard, deps) })
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if requests.Load() != 3 || !ready(l.environment(""), deps) {
		t.Fatal("concurrent install was not reused")
	}
	bad.Store(true)
	if e := l.ensureEnvironment(context.Background(), "", io.Discard, deps); e != nil {
		t.Fatal(e)
	}
	if requests.Load() != 3 {
		t.Fatal("ready environment requested network")
	}
	missing := filepath.Join(l.environment(""), "uthash.h")
	if e := os.Remove(missing); e != nil {
		t.Fatal(e)
	}
	if e := l.ensureEnvironment(context.Background(), "", io.Discard, deps); e == nil {
		t.Fatal("bad checksum accepted")
	}
	if b, _ := os.ReadFile(filepath.Join(l.environment(""), "yyjson.h")); string(b) != "/yyjson.h" {
		t.Fatal("failed install damaged existing environment")
	}
	bad.Store(false)
	if e := l.ensureEnvironment(context.Background(), "", io.Discard, deps); e != nil {
		t.Fatal(e)
	}
	if !ready(l.environment(""), deps) {
		t.Fatal("repair incomplete")
	}
}
func TestEnvironmentLockAndDiagnosis(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	l := Language{Environment: filepath.Join(root, "c"), Compiler: filepath.Join(root, "missing-compiler"), GLib: "disabled"}
	checks := l.Diagnose(context.Background(), lang.DiagnosticOptions{})
	found := false
	for _, d := range checks {
		if d.Role == "dependencies" {
			found = true
			if d.Status != "uninitialized" {
				t.Fatal(d)
			}
		}
	}
	if !found {
		t.Fatal(checks)
	}
	if _, e := os.Stat(l.Environment); !os.IsNotExist(e) {
		t.Fatal("doctor wrote environment")
	}
	if e := os.MkdirAll(l.Environment, 0755); e != nil {
		t.Fatal(e)
	}
	lock, e := os.OpenFile(l.environment("")+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		t.Fatal(e)
	}
	defer lock.Close()
	if e := unix.Flock(int(lock.Fd()), unix.LOCK_EX); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if e := l.Setup(ctx, io.Discard); e == nil || !strings.Contains(e.Error(), "deadline") {
		t.Fatal(e)
	}
}
func TestGLibFlagsAndMissingTools(t *testing.T) {
	t.Parallel()
	l := Language{PkgConfig: filepath.Join(t.TempDir(), "missing"), GLib: "auto"}
	if flags, _, e := l.glibFlags(context.Background(), false); e != nil || len(flags) != 0 {
		t.Fatalf("%v %v", flags, e)
	}
	for _, required := range []bool{true, false} {
		l.GLib = "required"
		if _, _, e := l.glibFlags(context.Background(), required); e == nil || !strings.Contains(e.Error(), "install them locally") {
			t.Fatal(e)
		}
	}
	l.GLib = "disabled"
	if _, _, e := l.glibFlags(context.Background(), true); e == nil {
		t.Fatal("ignored explicit GLib requirement")
	}
	flags, e := splitFlags(`-I'/path with spaces/include' -DHELLO=1 -L/path\ with\ spaces/lib -lglib-2.0`)
	if e != nil || len(flags) != 4 || flags[0] != "-I/path with spaces/include" || flags[2] != "-L/path with spaces/lib" {
		t.Fatalf("%v %v", flags, e)
	}
	script := filepath.Join(t.TempDir(), "pkg-config")
	if e := os.WriteFile(script, []byte("#!/bin/sh\ncase \"$1\" in --cflags) echo \"-I'/path with spaces/include'\";; --libs) echo '-lglib-2.0';; --modversion) echo 2.80.0;; esac\n"), 0755); e != nil {
		t.Fatal(e)
	}
	l = Language{PkgConfig: script, GLib: "required"}
	flags, libs, e := l.glibFlags(context.Background(), false)
	if e != nil || len(flags) != 2 || flags[1] != "-DHC_GLIB=1" || len(libs) != 1 {
		t.Fatalf("%v %v %v", flags, libs, e)
	}
}
