package python

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

var (
	// runtimeImportRE matches the first line of an import from hc_runtime.
	runtimeImportRE = regexp.MustCompile(`^\s*(from\s+hc_runtime\s+import\b|import\s+hc_runtime\b)`)
	// runtimeUseRE matches a use of hc_runtime other than importing names.
	runtimeUseRE  = regexp.MustCompile(`\bhc_runtime\s*\.`)
	solutionClass = regexp.MustCompile(`(?m)^class\s+Solution\b`)
)

// LeetCode turns a solution into LeetCode's python3 code: LeetCode defines
// ListNode, TreeNode and the judge-defined types itself, so the imports from
// hc_runtime go. LeetCode calls a function problem's method on Solution(),
// so a module-level function, which hc accepts, cannot run there.
func (Language) LeetCode(p *problem.Problem, src string) (string, string, error) {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !runtimeImportRE.MatchString(line) {
			out = append(out, line)
			continue
		}
		// Drop the rest of the statement too: a parenthesized list of
		// names, or lines continued with a backslash.
		open := strings.Contains(line, "(") && !strings.Contains(line, ")")
		cont := !open && continued(line)
		for (open || cont) && i+1 < len(lines) {
			i++
			if open {
				open = !strings.Contains(lines[i], ")")
			}
			cont = !open && continued(lines[i])
		}
	}
	code := strings.Join(out, "\n")
	if runtimeUseRE.MatchString(code) {
		return "", "", fmt.Errorf("%s uses hc_runtime, which LeetCode does not have; import the names you need from it instead", solutionFile)
	}
	if f := p.Function; f != nil && !solutionClass.MatchString(code) {
		return "", "", fmt.Errorf("LeetCode calls Solution().%s, but %s has no class Solution; make %s a method of it", f.Name, solutionFile, f.Name)
	}
	return "python3", strings.TrimLeft(code, "\n"), nil
}

func continued(line string) bool { return strings.HasSuffix(strings.TrimRight(line, " \t\r"), `\`) }
