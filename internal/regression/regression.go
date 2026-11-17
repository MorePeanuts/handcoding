// Package regression locates two independent test packs: LeetCode
// compatibility in testdata/leetcode-regression and hc protocol extensions in
// testdata/protocol-regression. Together they cover the protocol in all seven
// languages. Neither pack is embedded in hc.
//
// Each is also an authoring workspace; tests read only pack.yaml and problems/,
// never developer settings, state or solutions beside them.
package regression

import (
	"path/filepath"
	"runtime"

	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// Name is the LeetCode compatibility pack name.
const Name = "leetcode-regression"

// Dir is the directory of the regression set.
func Dir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", Name)
}

// ProtocolName is the separate synthetic protocol-extension pack.
const ProtocolName = "protocol-regression"

// Corpus identifies an independently installable regression pack.
type Corpus struct{ Name, Dir string }

// Packs returns both corpora; business fixtures may use only Dir/ProblemsDir.
func Packs() []Corpus {
	return []Corpus{{Name, Dir()}, {ProtocolName, filepath.Join(Dir(), "..", ProtocolName)}}
}

// ProblemsDir is the directory of the regression set's problems.
func ProblemsDir() string { return filepath.Join(Dir(), pack.ProblemsDir) }

// Install copies an unmanaged LeetCode pack for business/UI fixtures.
// Corpus execution tests instead run the hc pack install command for both Packs.
func Install(ws *workspace.Workspace) error {
	return Copy(filepath.Join(ws.Root, workspace.PacksDir, Name))
}

// Copy copies the pack, its pack.yaml and problems/ but nothing else in the
// set's directory, to dir, which must not hold them yet.
func Copy(dir string) error { return pack.Copy(Dir(), dir) }
