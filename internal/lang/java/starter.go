package java

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

const imports = "import java.util.*;\nimport java.math.*;\nimport javafx.util.Pair;\n\n"

func javaType(t string) string {
	base, depth := problem.SplitArray(t)
	s := map[string]string{"integer": "int", "long": "long", "double": "double", "boolean": "boolean", "character": "char", "string": "String", "void": "void"}[base]
	if s == "" {
		s = base
	}
	if problem.IsNodeVariant(base) {
		s = "Node"
	}
	if depth > 0 && (base == "NestedInteger" || base == "Employee") {
		s = "List<" + s + ">"
		depth--
	}
	return s + strings.Repeat("[]", depth)
}
func params(ps []problem.Param) string {
	out := []string{}
	for i, p := range ps {
		out = append(out, fmt.Sprintf("%s arg%d", javaType(p.Type), i))
	}
	return strings.Join(out, ", ")
}
func (Language) Starter(p *problem.Problem) (string, error) {
	b, err := os.ReadFile(filepath.Join(p.Dir, "templates", "Solution.java"))
	if err == nil {
		source := string(b)
		if p.Kind == problem.KindClass && p.Class.Name != "Solution" {
			// LeetCode sometimes declares a design class public. A source named
			// Solution.java requires that top-level class to have package visibility.
			re := regexp.MustCompile(`(?m)^(\s*)public\s+((?:final\s+|abstract\s+)?class\s+` + regexp.QuoteMeta(p.Class.Name) + `\b)`)
			source = re.ReplaceAllString(source, "${1}${2}")
		}
		return imports + source, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	var s strings.Builder
	s.WriteString(imports)
	if p.Kind == problem.KindFunction {
		fmt.Fprintf(&s, "class Solution {\n    public %s %s(%s) {\n        throw new UnsupportedOperationException(\"not implemented\");\n    }\n}\n", javaType(p.Function.Returns), p.Function.Name, params(p.Function.Params))
	} else {
		fmt.Fprintf(&s, "class %s {\n    public %s(%s) {}\n", p.Class.Name, p.Class.Name, params(p.Class.Constructor))
		for _, m := range p.Class.Methods {
			fmt.Fprintf(&s, "    public %s %s(%s) { throw new UnsupportedOperationException(\"not implemented\"); }\n", javaType(m.Returns), m.Name, params(m.Params))
		}
		s.WriteString("}\n")
	}
	return s.String(), nil
}
