package leetcode

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

var templateFiles = map[string]string{
	"go": "solution.go", "python": "solution.py", "c": "solution.c",
	"cpp": "solution.cpp", "java": "Solution.java", "javascript": "solution.js", "typescript": "solution.ts",
}

// TemplateLanguages validates a selection, returning all supported languages
// for an empty selection. Pack languages deliberately do not limit migration.
func TemplateLanguages(languages []string) ([]string, error) {
	if len(languages) == 0 {
		return problem.Languages(problem.Protocol), nil
	}
	var result []string
	for _, name := range languages {
		switch name {
		case "js":
			name = "javascript"
		case "ts":
			name = "typescript"
		}
		if _, ok := templateFiles[name]; !ok {
			return nil, fmt.Errorf("unknown template language %q", name)
		}
		if !slices.Contains(result, name) {
			result = append(result, name)
		}
	}
	return result, nil
}

// TemplateUpdate is a preview of additions to one existing problem. Conflicts
// block the entire update, including otherwise compatible languages.
type TemplateUpdate struct {
	Added, Unchanged, Missing, Conflicts []string
	Bindings                             []string
	Files                                map[string][]byte
}

// PlanTemplates reads existing files but never writes. Only actual native
// snippets are candidates; the normal converter derives their shared ABI and
// C bindings, exactly as on a fresh import.
func PlanTemplates(p *problem.Problem, q *Question, site Site, languages []string) (*TemplateUpdate, error) {
	languages, err := TemplateLanguages(languages)
	if err != nil {
		return nil, err
	}
	question := *q
	question.CodeSnippets = slices.Clone(q.CodeSnippets)
	question.CodeSnippets = slices.DeleteFunc(question.CodeSnippets, func(s struct {
		LangSlug string `json:"langSlug"`
		Code     string `json:"code"`
	}) bool {
		return strings.TrimSpace(s.Code) == "" || s.LangSlug == "c" && !slices.Contains(languages, "c")
	})
	c, err := Convert(&question, site)
	if err != nil {
		return nil, err
	}
	u := &TemplateUpdate{Files: map[string][]byte{}}
	if p.Slug != c.Problem.Slug || p.ID != c.Problem.ID {
		u.Conflicts = append(u.Conflicts, "problem identity differs from LeetCode")
	}
	if !bytes.Equal(templateABI(p), templateABI(c.Problem)) {
		u.Conflicts = append(u.Conflicts, "shared signature or output binding differs from LeetCode")
	}
	for _, language := range languages {
		name := templateFiles[language]
		code := c.Templates[name]
		if strings.TrimSpace(code) == "" {
			u.Missing = append(u.Missing, language)
			continue
		}
		rel := filepath.Join(problem.TemplatesDir, name)
		data, err := os.ReadFile(filepath.Join(p.Dir, rel))
		switch {
		case os.IsNotExist(err):
			u.Added = append(u.Added, rel)
			u.Files[rel] = []byte(code)
		case err != nil:
			return nil, err
		case string(data) == code:
			u.Unchanged = append(u.Unchanged, rel)
		default:
			u.Conflicts = append(u.Conflicts, rel+": existing template differs (kept)")
		}
		if language == "c" {
			meta, err := os.ReadFile(filepath.Join(p.Dir, problem.MetaFile))
			if err != nil {
				return nil, err
			}
			updated, fields, err := mergeTemplateC(meta, p, c.Problem.C)
			if err != nil {
				u.Conflicts = append(u.Conflicts, err.Error())
			} else if len(fields) > 0 {
				u.Bindings = fields
				u.Files[problem.MetaFile] = updated
			}
		}
	}
	return u, nil
}

func templateABI(p *problem.Problem) []byte {
	f := p.Function
	if f != nil {
		copy := *f
		// This is an author-selected judging rule, not a calling convention.
		copy.ReturnsCopy = ""
		f = &copy
	}
	data, _ := yaml.Marshal(struct {
		Kind                 problem.Kind
		Function             *problem.Function
		Class                *problem.Class
		Output, OutputLength string
		RoundTrip            *problem.RoundTrip
	}{p.Kind, f, p.Class, p.Judge.Output, p.Judge.OutputLength, p.Judge.RoundTrip})
	return data
}

// mergeTemplateC adds only missing keys. Rendering is confined to the C
// subtree: every byte outside it (including hand-edited YAML) stays intact.
func mergeTemplateC(data []byte, p *problem.Problem, incoming *problem.CBinding) ([]byte, []string, error) {
	if incoming == nil {
		if p.C != nil && p.C.Function != "" && (p.Function == nil || p.C.Function != p.Function.Name) {
			return nil, nil, fmt.Errorf("c.function: existing binding differs from native template")
		}
		return data, nil, nil
	}
	f, err := parser.ParseBytes(data, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	if len(f.Docs) != 1 {
		return nil, nil, fmt.Errorf("problem.yaml must contain one document")
	}
	root, ok := f.Docs[0].Body.(*ast.MappingNode)
	if !ok || root.IsFlowStyle {
		return nil, nil, fmt.Errorf("template migration requires a block mapping in problem.yaml to add C bindings")
	}
	node, err := yaml.ValueToNode(struct {
		C *problem.CBinding `yaml:"c"`
	}{incoming})
	if err != nil {
		return nil, nil, err
	}
	entry := node.(*ast.MappingNode).Values[0]
	var fields []string
	var current *ast.MappingValueNode
	index := 0
	for i, v := range root.Values {
		if v.Key.String() == "c" {
			current, index = v, i
			break
		}
	}
	if current == nil {
		entry.AddColumn(root.Values[0].Key.GetToken().Position.Column - 1)
		fields = bindingAdditions(entry.Value.(*ast.MappingNode), "c")
	} else {
		old, ok := current.Value.(*ast.MappingNode)
		if !ok {
			return nil, nil, fmt.Errorf("c: expected a binding mapping")
		}
		if err := mergeBindingNodes(old, entry.Value.(*ast.MappingNode), "c", &fields); err != nil {
			return nil, nil, err
		}
		entry = current
	}
	if len(fields) == 0 {
		return data, nil, nil
	}
	// A contradictory combination (e.g. object:none plus a local type) must
	// fail before the matching template is published.
	var binding struct {
		C *problem.CBinding `yaml:"c"`
	}
	if err := yaml.Unmarshal([]byte(entry.String()), &binding); err != nil {
		return nil, nil, err
	}
	check := *p
	check.C = binding.C
	if err := check.ValidateC(); err != nil {
		return nil, nil, err
	}
	lines := strings.SplitAfter(string(data), "\n")
	start := len(lines)
	if f.Docs[0].End != nil {
		start = f.Docs[0].End.Position.Line - 1
	}
	end := start
	if current != nil {
		start = mappingStart(current)
		end = len(lines)
		if index+1 < len(root.Values) {
			end = mappingStart(root.Values[index+1])
		} else if f.Docs[0].End != nil {
			end = f.Docs[0].End.Position.Line - 1
		}
	}
	prefix := strings.Join(lines[:start], "")
	if prefix != "" && !strings.HasSuffix(prefix, "\n") {
		prefix += "\n"
	}
	out := prefix + entry.String() + "\n" + strings.Join(lines[end:], "")
	return []byte(out), fields, nil
}

func mappingStart(v *ast.MappingValueNode) int {
	line := v.Key.GetToken().Position.Line
	if c := v.GetComment(); c != nil && c.GetToken().Position.Line < line {
		line = c.GetToken().Position.Line
	}
	return line - 1
}

func mergeBindingNodes(dst, src *ast.MappingNode, path string, fields *[]string) error {
	for _, add := range src.Values {
		name := path + "." + add.Key.String()
		var existing *ast.MappingValueNode
		for _, v := range dst.Values {
			if v.Key.String() == add.Key.String() {
				existing = v
				break
			}
		}
		if existing == nil {
			if dst.IsFlowStyle {
				add.SetIsFlowStyle(true)
			}
			if len(dst.Values) > 0 {
				add.AddColumn(dst.Values[0].Key.GetToken().Position.Column - add.Key.GetToken().Position.Column)
			}
			dst.Values = append(dst.Values, add)
			if child, ok := add.Value.(*ast.MappingNode); ok {
				*fields = append(*fields, bindingAdditions(child, name)...)
			} else {
				*fields = append(*fields, name+" = "+add.Value.String())
			}
			continue
		}
		if child, ok := add.Value.(*ast.MappingNode); ok {
			old, ok := existing.Value.(*ast.MappingNode)
			if !ok {
				return fmt.Errorf("%s: expected a mapping", name)
			}
			if err := mergeBindingNodes(old, child, name, fields); err != nil {
				return err
			}
			continue
		}
		var a, b any
		if err := yaml.NodeToValue(existing.Value, &a); err != nil {
			return err
		}
		if err := yaml.NodeToValue(add.Value, &b); err != nil {
			return err
		}
		if !reflect.DeepEqual(a, b) {
			return fmt.Errorf("%s: existing binding differs from native template", name)
		}
	}
	return nil
}

func bindingAdditions(node *ast.MappingNode, path string) []string {
	var fields []string
	for _, v := range node.Values {
		name := path + "." + v.Key.String()
		if child, ok := v.Value.(*ast.MappingNode); ok {
			fields = append(fields, bindingAdditions(child, name)...)
		} else {
			fields = append(fields, name+" = "+v.Value.String())
		}
	}
	return fields
}
