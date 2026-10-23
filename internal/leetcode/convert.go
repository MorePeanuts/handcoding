package leetcode

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

type metaParam struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type metaReturn struct {
	Type string `json:"type"`
}

type metaData struct {
	// function problems
	Name   string      `json:"name"`
	Params []metaParam `json:"params"`
	Return metaReturn  `json:"return"`
	Output *struct {
		ParamIndex int `json:"paramindex"`
	} `json:"output"`
	// class problems
	ClassName   string `json:"classname"`
	Constructor struct {
		Params []metaParam `json:"params"`
	} `json:"constructor"`
	Methods []struct {
		Name   string      `json:"name"`
		Params []metaParam `json:"params"`
		Return metaReturn  `json:"return"`
	} `json:"methods"`
	SystemDesign bool `json:"systemdesign"`
	Manual       bool `json:"manual"`
}

// Converted is a LeetCode question turned into a handcoding problem.
type Converted struct {
	Problem     *problem.Problem
	Description string
	Cases       []problem.Case
	Templates   map[string]string // file name -> code
	// Warnings explain what could not be converted automatically.
	Warnings []string
}

// Convert maps a question to the problem protocol. preferTranslated selects
// the translated title/statement (leetcode.cn).
func Convert(q *Question, site Site, preferTranslated bool) (*Converted, error) {
	if q.IsPaidOnly && q.Content == "" && q.TranslatedContent == "" {
		return nil, fmt.Errorf("%s is a premium problem; its content is not available without a subscription", q.TitleSlug)
	}
	var md metaData
	if err := json.Unmarshal([]byte(q.MetaData), &md); err != nil {
		return nil, fmt.Errorf("parse metaData: %w", err)
	}
	p := &problem.Problem{
		ID:         q.QuestionFrontendID,
		Slug:       q.TitleSlug,
		Title:      q.Title,
		Difficulty: strings.ToLower(q.Difficulty),
		Source:     problem.Source{Site: "leetcode." + site.Name, URL: site.Base + "/problems/" + q.TitleSlug + "/"},
	}
	for _, t := range q.TopicTags {
		p.Tags = append(p.Tags, t.Slug)
	}
	conv := &Converted{Problem: p, Templates: map[string]string{}}
	if md.Manual {
		conv.Warnings = append(conv.Warnings, "LeetCode marks this problem as using a custom judge; generated cases may need care")
	}
	params := func(ps []metaParam) []problem.Param {
		out := make([]problem.Param, len(ps))
		for i, x := range ps {
			out[i] = problem.Param{Name: x.Name, Type: x.Type}
		}
		return out
	}
	if md.ClassName != "" || md.SystemDesign {
		p.Kind = problem.KindClass
		c := &problem.Class{Name: md.ClassName, Constructor: params(md.Constructor.Params)}
		for _, m := range md.Methods {
			c.Methods = append(c.Methods, problem.Method{Name: m.Name, Params: params(m.Params), Returns: m.Return.Type})
		}
		p.Class = c
	} else {
		p.Kind = problem.KindFunction
		p.Function = &problem.Function{Name: md.Name, Params: params(md.Params), Returns: md.Return.Type}
		if md.Output != nil && md.Output.ParamIndex >= 0 && md.Output.ParamIndex < len(md.Params) {
			p.Judge.Output = md.Params[md.Output.ParamIndex].Name
		}
	}
	content := q.Content
	if preferTranslated && q.TranslatedContent != "" {
		content = q.TranslatedContent
		if q.TranslatedTitle != "" {
			p.Title = q.TranslatedTitle
		}
	}
	if anyOrder(q.Content + q.TranslatedContent) {
		p.Judge.Compare = problem.CompareUnordered
		conv.Warnings = append(conv.Warnings, "statement says the answer may be in any order: judge.compare set to unordered (check whether unordered_deep is needed)")
	}
	p.Normalize()
	if err := p.Validate(); err != nil {
		return conv, fmt.Errorf("%s cannot be judged by hc yet: %w", p.Slug, err)
	}

	desc, err := toMarkdown(content)
	if err != nil {
		return nil, err
	}
	conv.Description = fmt.Sprintf("# %s. %s\n\n%s\n", p.ID, p.Title, strings.TrimSpace(desc))

	for _, s := range q.CodeSnippets {
		switch s.LangSlug {
		case "golang":
			conv.Templates["solution.go"] = s.Code
		case "python3":
			conv.Templates["solution.py"] = s.Code
		}
	}

	cases, warn := examples(p, q)
	conv.Cases = cases
	if warn != "" {
		conv.Warnings = append(conv.Warnings, warn)
	}
	return conv, nil
}

var anyOrderRE = regexp.MustCompile(`(?i)in any order|任意顺序|任何顺序`)

func anyOrder(content string) bool { return anyOrderRE.MatchString(content) }

var supRE = regexp.MustCompile(`<sup>(.*?)</sup>`)

func toMarkdown(content string) (string, error) {
	// html-to-markdown drops <sup>; keep exponents readable (10^4).
	content = supRE.ReplaceAllString(content, "^$1")
	content = strings.ReplaceAll(content, "&nbsp;", " ")
	return htmltomarkdown.ConvertString(content)
}

var (
	tagRE    = regexp.MustCompile(`<[^>]+>`)
	outputRE = regexp.MustCompile(`(?m)(?:Output|输出)\s*[:：]?\s*\n?\s*(.+)$`)
)

// examples builds the statement examples: inputs from exampleTestcases,
// expected outputs scraped from the statement when they parse as JSON.
func examples(p *problem.Problem, q *Question) ([]problem.Case, string) {
	lines := strings.Split(strings.TrimSpace(q.ExampleTestcases), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, "no example test cases provided by LeetCode"
	}
	perCase := 2
	if p.Kind == problem.KindFunction {
		perCase = len(p.Function.Params)
	}
	if perCase == 0 || len(lines)%perCase != 0 {
		return nil, "could not split the example test cases; run hc gen"
	}
	keys := p.InputKeys()
	var cases []problem.Case
	for i := 0; i < len(lines); i += perCase {
		c := problem.Case{Input: map[string]json.RawMessage{}, Example: true}
		for j, k := range keys {
			v := strings.TrimSpace(lines[i+j])
			if !json.Valid([]byte(v)) {
				return nil, fmt.Sprintf("example input %q is not JSON; run hc gen", v)
			}
			c.Input[k] = json.RawMessage(v)
		}
		cases = append(cases, c)
	}

	src := q.Content
	if src == "" {
		src = q.TranslatedContent
	}
	text := html.UnescapeString(tagRE.ReplaceAllString(src, ""))
	var outputs []string
	for _, m := range outputRE.FindAllStringSubmatch(text, -1) {
		outputs = append(outputs, strings.TrimSpace(m[1]))
	}
	if len(outputs) != len(cases) {
		return cases, "could not match example outputs to inputs; expected outputs left empty (run hc gen)"
	}
	missing := 0
	for i, out := range outputs {
		out = strings.ReplaceAll(out, " ", " ")
		if json.Valid([]byte(out)) {
			cases[i].Expected = json.RawMessage(compactJSON(out))
		} else {
			missing++
		}
	}
	if missing > 0 {
		return cases, fmt.Sprintf("%d example output(s) are not plain JSON and were left empty (run hc gen)", missing)
	}
	return cases, ""
}

func compactJSON(s string) string {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return s
	}
	return string(b)
}

// Save writes a converted problem into dir.
func (c *Converted) Save(dir string) error {
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%s already exists", dir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	c.Problem.Dir = dir
	if err := c.Problem.Save(); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, problem.DescriptionFile), []byte(c.Description), 0o644); err != nil {
		return err
	}
	if len(c.Templates) > 0 {
		if err := os.MkdirAll(filepath.Join(dir, problem.TemplatesDir), 0o755); err != nil {
			return err
		}
		for name, code := range c.Templates {
			if err := os.WriteFile(filepath.Join(dir, problem.TemplatesDir, name), []byte(code), 0o644); err != nil {
				return err
			}
		}
	}
	return c.Problem.SaveCases(c.Cases)
}
