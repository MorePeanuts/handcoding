package leetcode

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

var cComments = regexp.MustCompile(`(?s)/\*.*?\*/|//[^\n]*`)
var cFunctions = regexp.MustCompile(`(?m)^[ \t]*((?:[A-Za-z_]\w*[ \t]+)*[A-Za-z_]\w*[ \t*]+)([A-Za-z_]\w*)[ \t]*\([^;{}]*\)[ \t\r\n]*\{`)

// cBindings captures native C symbol differences at import time. Runtime
// conversion still follows problem.yaml, never the user's edited solution.
func cBindings(p *problem.Problem, source string) error {
	if source == "" {
		return nil
	}
	matches := cFunctions.FindAllStringSubmatch(cComments.ReplaceAllString(source, ""), -1)
	symbols := map[string]string{}
	for _, m := range matches {
		symbols[m[2]] = strings.TrimSpace(m[1])
	}
	b := &problem.CBinding{}
	if p.Kind == problem.KindFunction {
		if _, ok := symbols[p.Function.Name]; !ok {
			if len(matches) != 1 {
				return fmt.Errorf("cannot bind C function %s to native template", p.Function.Name)
			}
			b.Function = matches[0][2]
			p.C = b
		}
		return nil
	}
	b.Methods = map[string]string{}
	prefix := ""
	for name, typ := range symbols {
		if strings.HasSuffix(name, "Create") && strings.HasSuffix(typ, "*") {
			if b.Constructor != "" {
				return fmt.Errorf("ambiguous C constructors")
			}
			b.Constructor = name
			prefix = strings.TrimSuffix(name, "Create")
			b.Type = strings.TrimSpace(strings.TrimSuffix(typ, "*"))
			if strings.HasPrefix(b.Type, "struct ") {
				b.Object = "struct"
				b.Type = strings.TrimSpace(strings.TrimPrefix(b.Type, "struct "))
			} else {
				b.Object = "typedef"
			}
		}
	}
	if b.Constructor == "" {
		b.Object = "none"
		if len(p.Class.Constructor) > 0 {
			return fmt.Errorf("native C template has no constructor for %s", p.Class.Name)
		}
	} else {
		destructor := ""
		if _, ok := symbols[prefix+"Free"]; ok {
			destructor = prefix + "Free"
		}
		b.Destructor = &destructor
	}
	for _, m := range p.Class.Methods {
		name := m.Name
		if prefix != "" {
			name = prefix + strings.ToUpper(m.Name[:1]) + m.Name[1:]
		}
		if _, ok := symbols[name]; !ok {
			return fmt.Errorf("cannot bind C method %s to native template", m.Name)
		}
		b.Methods[m.Name] = name
	}
	p.C = b
	return p.ValidateC()
}
