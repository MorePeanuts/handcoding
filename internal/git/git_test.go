package git

import (
	"slices"
	"testing"
)

func TestCloneArgs(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		source   string
		progress bool
		want     []string
	}{
		{"https://github.com/MorePeanuts/leetcode.hc", false,
			[]string{"clone", "--quiet", "--filter=blob:none", "--", "https://github.com/MorePeanuts/leetcode.hc", "d"}},
		{"git@github.com:MorePeanuts/leetcode.hc.git", true,
			[]string{"clone", "--progress", "--filter=blob:none", "--", "git@github.com:MorePeanuts/leetcode.hc.git", "d"}},
		// A local repository is cloned whole: git ignores the filter there.
		{dir, false, []string{"clone", "--quiet", "--", dir, "d"}},
	}
	for _, tt := range tests {
		if got := cloneArgs(tt.source, "d", tt.progress); !slices.Equal(got, tt.want) {
			t.Errorf("cloneArgs(%q) = %q, want %q", tt.source, got, tt.want)
		}
	}
}
