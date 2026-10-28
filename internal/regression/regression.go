// Package regression locates hc's regression set for tests: the pack in
// testdata/regression at the root of the repository, whose problems
// together use the whole problem protocol, each with Go and Python
// references. hc itself does not ship it; only tests import this package.
package regression

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// Name is the regression set's pack name.
const Name = "regression"

// Dir is the directory of the regression set.
func Dir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", Name)
}

// ProblemsDir is the directory of the regression set's problems.
func ProblemsDir() string { return filepath.Join(Dir(), pack.ProblemsDir) }

// Install copies the regression set into the workspace ws as the installed
// pack packs/regression. It is a copy, so tests cannot change the set.
func Install(ws *workspace.Workspace) error {
	return os.CopyFS(filepath.Join(ws.Root, workspace.PacksDir, Name), os.DirFS(Dir()))
}
