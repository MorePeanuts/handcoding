package app

import (
	"slices"
	"testing"

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
