package app_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/testenv"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// Only corpus tests need the executable. Build it once per test process,
// rather than recompiling hc for each language or problem. Business fixtures
// that deliberately model an unmanaged pack still use regression.Install.
var regressionCLI struct {
	once sync.Once
	dir  string
	file string
	err  error
}

func TestMain(m *testing.M) {
	code := m.Run()
	if regressionCLI.dir != "" {
		if err := os.RemoveAll(regressionCLI.dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			code = 1
		}
	}
	os.Exit(code)
}

func regressionExecutable(t *testing.T) string {
	t.Helper()
	goTool := testenv.LookPath(t, "go")
	regressionCLI.once.Do(func() {
		regressionCLI.dir, regressionCLI.err = os.MkdirTemp("", "hc-regression-cli-")
		if regressionCLI.err != nil {
			return
		}
		regressionCLI.file = filepath.Join(regressionCLI.dir, "hc")
		cmd := exec.CommandContext(t.Context(), goTool, "build", "-o", regressionCLI.file, "./cmd/hc")
		cmd.Dir = filepath.Join(regression.Dir(), "..", "..")
		if output, err := cmd.CombinedOutput(); err != nil {
			regressionCLI.err = fmt.Errorf("build hc: %w\n%s", err, output)
		}
	})
	if regressionCLI.err != nil {
		t.Fatal(regressionCLI.err)
	}
	return regressionCLI.file
}

// newRegressionWorkspace exercises the actual pack-install command before
// reference runs, including discovery, staging and registration in hc.yaml.
func newRegressionWorkspace(t *testing.T) *app.App {
	t.Helper()
	binary := regressionExecutable(t)
	ws, err := workspace.Create(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, corpus := range regression.Packs() {
		cmd := exec.CommandContext(t.Context(), binary, "-w", ws.Root, "pack", "install", corpus.Dir)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("hc pack install %s: %v\n%s", corpus.Name, err, output)
		}
	}
	ws, err = workspace.Open(ws.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, corpus := range regression.Packs() {
		source, ok := ws.Config.PackSource(corpus.Name)
		if !ok || source.Source != filepath.Clean(corpus.Dir) || !source.Copied() {
			t.Fatalf("pack install did not register %s: %+v", corpus.Name, source)
		}
		p, err := ws.Pack(corpus.Name)
		if err != nil || p.Err != nil || p.Writable {
			t.Fatalf("installed %s is not a valid read-only pack: %+v, %v", corpus.Name, p, err)
		}
	}
	return regressionApp(t, ws)
}
