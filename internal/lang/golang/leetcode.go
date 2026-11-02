package golang

import (
	"regexp"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

// packageRE matches the package clause, with its line.
var packageRE = regexp.MustCompile(`(?m)^package[ \t]+\w+[ \t]*(//.*)?(\r?\n|$)`)

// LeetCode turns a solution into LeetCode's golang code: LeetCode supplies
// the package clause and defines ListNode, TreeNode and the judge-defined
// types itself, as hc's driver does, so only the package clause goes.
func (Language) LeetCode(_ *problem.Problem, src string) (string, string, error) {
	if loc := packageRE.FindStringIndex(src); loc != nil {
		src = src[:loc[0]] + src[loc[1]:]
	}
	return "golang", strings.TrimLeft(src, "\n"), nil
}
