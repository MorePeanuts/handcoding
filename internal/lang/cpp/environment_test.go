package cpp

import (
	"context"
	"crypto/sha256"
	"errors"
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

	"github.com/MorePeanuts/handcoding/internal/problem"
)

func TestDependencyLifecycle(t *testing.T) {
	t.Parallel()
	payload := "// fixture JSON header\n"
	var requests atomic.Int32
	var broken atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if broken.Load() {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, payload)
	}))
	defer srv.Close()
	dep := dependency{version: "fixture", url: srv.URL, sha256: fmt.Sprintf("%x", sha256.Sum256([]byte(payload)))}
	l := Language{Environment: filepath.Join(t.TempDir(), ".hc", "cpp")}
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Go(func() { errs <- l.ensureEnvironment(ctx, "", io.Discard, dep) })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if requests.Load() != 1 {
		t.Fatalf("concurrent installs: %d requests", requests.Load())
	}
	target := l.environment("", dep)
	if !dependencyReady(target, dep) {
		t.Fatal("installed header not ready")
	}
	broken.Store(true)
	if err := l.ensureEnvironment(ctx, "", io.Discard, dep); err != nil {
		t.Fatal("offline reuse:", err)
	}
	if requests.Load() != 1 {
		t.Fatal("ready environment used network")
	}
	header := headerPath(target)
	// Same-size corruption must be detected, not just a missing file.
	corrupt := strings.Repeat("x", len(payload))
	if err := os.WriteFile(header, []byte(corrupt), 0644); err != nil {
		t.Fatal(err)
	}
	if dependencyReady(target, dep) {
		t.Fatal("same-size corruption accepted")
	}
	if err := l.ensureEnvironment(ctx, "", io.Discard, dep); err == nil || !strings.Contains(err.Error(), "hc env sync --lang cpp") {
		t.Fatalf("failed repair: %v", err)
	}
	if b, _ := os.ReadFile(header); string(b) != corrupt {
		t.Fatal("failed repair changed existing file")
	}
	note := filepath.Join(target, "notes.txt")
	if err := os.WriteFile(note, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	broken.Store(false)
	if err := l.ensureEnvironment(ctx, "", io.Discard, dep); err != nil {
		t.Fatal(err)
	}
	if !dependencyReady(target, dep) {
		t.Fatal("repair failed")
	}
	if b, _ := os.ReadFile(note); string(b) != "keep" {
		t.Fatal("unrelated file changed")
	}
	if err := os.Remove(header); err != nil {
		t.Fatal(err)
	}
	if err := l.ensureEnvironment(ctx, "", io.Discard, dep); err != nil {
		t.Fatal(err)
	}
}

func TestDependencyRejectsBadDownloads(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"status", "checksum", "oversize", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch kind {
				case "status":
					http.Error(w, "missing", http.StatusNotFound)
				case "checksum":
					fmt.Fprint(w, "bad")
				case "oversize":
					fmt.Fprint(w, strings.Repeat("x", maxHeaderSize+1))
				case "cancel":
					<-r.Context().Done()
				}
			}))
			defer srv.Close()
			dep := dependency{version: "fixture", url: srv.URL, sha256: fmt.Sprintf("%x", sha256.Sum256([]byte("correct")))}
			l := Language{Environment: filepath.Join(t.TempDir(), "cpp")}
			timeout := 5 * time.Second
			if kind == "cancel" {
				timeout = 100 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			err := l.ensureEnvironment(ctx, "", io.Discard, dep)
			if err == nil {
				t.Fatal("bad download accepted")
			}
			want := map[string]string{"status": "HTTP 404", "checksum": "SHA-256 mismatch", "oversize": "exceeds"}[kind]
			if want != "" && !strings.Contains(err.Error(), want) {
				t.Fatalf("wrong rejection: %v", err)
			}
			if kind == "cancel" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal(err)
			}
			if _, err := os.Stat(headerPath(l.environment("", dep))); !os.IsNotExist(err) {
				t.Fatalf("failed download published header: %v", err)
			}
		})
	}
}

func TestDependencyLockCancellation(t *testing.T) {
	t.Parallel()
	l := Language{Environment: t.TempDir()}
	lock, err := os.OpenFile(l.environment("", jsonDependency)+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := l.Setup(ctx, io.Discard); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("lock wait: %v", err)
	}
}

func TestDependencyDiagnosisAndPrepareAreReadOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	l := Language{Environment: filepath.Join(root, "cpp")}
	solution := filepath.Join(root, "solution")
	if err := os.Mkdir(solution, 0755); err != nil {
		t.Fatal(err)
	}
	// Upgrade replaces the old generated full header with a tiny include wrapper.
	if err := os.WriteFile(filepath.Join(solution, "hc_json.hpp"), []byte(strings.Repeat("x", 4096)), 0644); err != nil {
		t.Fatal(err)
	}
	p := &problem.Problem{Kind: problem.KindFunction, Function: &problem.Function{Name: "f", Returns: "integer"}}
	if err := l.Prepare(solution, p); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(solution, "hc_json.hpp"))
	if err != nil || len(b) > 200 || !strings.Contains(string(b), "#include <nlohmann/json.hpp>") {
		t.Fatalf("wrapper: %s %v", b, err)
	}
	checks := l.diagnoseEnvironment()
	if len(checks) != 1 || checks[0].Status != "uninitialized" || !strings.Contains(checks[0].Message, "hc env sync --lang cpp") {
		t.Fatal(checks)
	}
	if _, err := os.Stat(l.Environment); !os.IsNotExist(err) {
		t.Fatalf("doctor created environment: %v", err)
	}
	if got := (Language{}).diagnoseEnvironment(); len(got) != 0 {
		t.Fatal("PATH-only doctor checks workspace dependencies")
	}
}
