package app

import (
	"slices"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/lang/python"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// TestProtocolLanguages checks that hc has a driver for every language of the
// protocol it implements, and none for a language outside it.
func TestProtocolLanguages(t *testing.T) {
	got := Languages(&workspace.Workspace{}).IDs()
	want := problem.Languages(problem.Protocol)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("drivers %v, want the languages of protocol %d, %v", got, problem.Protocol, want)
	}
}

// A pack checkout runs its Python on the oldest version the protocol
// supports, as its CI does; a practice workspace on the newest.
func TestPythonVersionByKind(t *testing.T) {
	for kind, want := range map[workspace.Kind]string{workspace.Authoring: python.MinVersion, workspace.Practice: ""} {
		if v := pythonLanguage(&workspace.Workspace{Kind: kind}).Venv; v == nil || v.Version != want {
			t.Errorf("kind %d: venv %+v, want version %q", kind, v, want)
		}
	}
	ws := &workspace.Workspace{Kind: workspace.Authoring}
	ws.Config.Python.Interpreter = "python3"
	if l := pythonLanguage(ws); l.Venv != nil || l.Interpreter != "python3" {
		t.Errorf("python.interpreter not used: %+v", l)
	}
}
