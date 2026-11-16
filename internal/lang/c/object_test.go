package c

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MorePeanuts/handcoding/internal/testenv"
)

func TestDependencyObject(t *testing.T) {
	// The wrapper counts actual compilations and can fail one; preprocessing
	// and object generation still use the real compiler and real headers.
	compiler := testenv.LookPath(t, DefaultCompiler())
	root := t.TempDir()
	log := filepath.Join(root, "compiles")
	t.Setenv("HC_OBJECT_TEST_COMPILER", compiler)
	t.Setenv("HC_OBJECT_TEST_LOG", log)
	wrapper := filepath.Join(root, "cc")
	script := `#!/bin/sh
for arg do
    if [ "$arg" = -c ]; then
        echo compile >> "$HC_OBJECT_TEST_LOG"
        if [ "$HC_OBJECT_TEST_FAIL" = 1 ]; then exit 1; fi
    fi
done
exec "$HC_OBJECT_TEST_COMPILER" "$@"
`
	if err := os.WriteFile(wrapper, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	l := Language{Compiler: wrapper, Environment: filepath.Join(root, "env")}
	env := l.environment("")
	if err := os.MkdirAll(env, 0755); err != nil {
		t.Fatal(err)
	}
	put := func(name, value string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(env, name), []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	put("config.h", "#define VALUE 1\n")
	put("yyjson.c", "#include \"config.h\"\nint cache_value(void) { return VALUE; }\n")
	flags := []string{"-std=c11", "-O2"}
	var wg sync.WaitGroup
	paths, errs := make([]string, 4), make([]error, 4)
	for i := range paths {
		dir := t.TempDir()
		wg.Go(func() { paths[i], errs[i] = l.dependencyObject(context.Background(), dir, flags) })
	}
	wg.Wait()
	for i := range paths {
		if errs[i] != nil || paths[i] != paths[0] {
			t.Fatalf("concurrent build: path=%s error=%v", paths[i], errs[i])
		}
	}
	count := func(want int) {
		t.Helper()
		b, err := os.ReadFile(log)
		if err != nil || strings.Count(string(b), "compile\n") != want {
			t.Fatalf("compilations: %q, want %d; %v", b, want, err)
		}
	}
	count(1)
	build := func(flags []string) string {
		t.Helper()
		path, err := l.dependencyObject(context.Background(), t.TempDir(), flags)
		if err != nil {
			t.Fatal(err)
		}
		return path
	}
	if path := build(flags); path != paths[0] {
		t.Fatal("another solution directory did not reuse the dependency")
	}
	count(1)
	put("config.h", "#define VALUE 2\n")
	if path := build(flags); path == paths[0] {
		t.Fatal("changed header reused a stale object")
	}
	count(2)
	build(append(flags, "-g"))
	count(3)
	otherCompiler := filepath.Join(root, "other-cc")
	if err := os.WriteFile(otherCompiler, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	l.Compiler = otherCompiler
	build(flags)
	count(4)
	// Failed compilations must not publish an object that a retry can reuse.
	t.Setenv("HC_OBJECT_TEST_FAIL", "1")
	if _, err := l.dependencyObject(context.Background(), root, flags); err == nil {
		t.Fatal("failed compilation succeeded")
	}
	if _, err := l.dependencyObject(context.Background(), root, flags); err == nil {
		t.Fatal("failed compilation was cached")
	}
	count(6)
}

func TestObjectLockCancellation(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "object.lock")
	unlock, err := lockFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if release, err := lockFile(ctx, path); err == nil {
		release()
		t.Fatal("lock wait ignored cancellation")
	}
}
