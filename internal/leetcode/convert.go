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
	// Lang marks a parameter that exists only in that language's template,
	// such as the length C passes with an array.
	Lang string `json:"lang"`
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
		// Size limits the judged output: "ret" means the first
		// return-value elements.
		Size any `json:"size"`
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
	// Assets are files referenced by the description (path in the problem
	// directory -> content), filled by Client.DownloadImages.
	Assets map[string][]byte
	// Warnings explain what could not be converted automatically.
	Warnings []string
}

// ErrUnsupported marks problems hc cannot drive yet.
var ErrUnsupported = errors.New("not supported by hc yet")

// englishPlaceholder is what leetcode.cn returns as the English statement of
// problems that only exist in Chinese (LCR, LCP, ...).
const englishPlaceholder = "English description is not available"

// Convert maps a question to the problem protocol. preferTranslated selects
// the translated title/statement (leetcode.cn).
//
// Convert records only what LeetCode states for certain: the signature, the
// in-place output and its length. It leaves judge.compare unset, because
// whether several answers are acceptable can only be read from the
// statement; hc gen decides it.
func Convert(q *Question, site Site, preferTranslated bool) (*Converted, error) {
	if q.IsPaidOnly && q.Content == "" && q.TranslatedContent == "" {
		return nil, fmt.Errorf("%s is a premium problem; its content is not available without a subscription", q.TitleSlug)
	}
	var md metaData
	if err := json.Unmarshal([]byte(q.MetaData), &md); err != nil {
		return nil, fmt.Errorf("parse metaData: %w", err)
	}
	english := q.Content
	if strings.Contains(english, englishPlaceholder) {
		english = ""
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
	for _, s := range q.CodeSnippets {
		switch s.LangSlug {
		case "golang":
			conv.Templates["solution.go"] = s.Code
		case "python3":
			conv.Templates["solution.py"] = s.Code
		}
	}
	params := func(ps []metaParam) []problem.Param {
		out := make([]problem.Param, 0, len(ps))
		for _, x := range ps {
			if x.Lang == "" {
				out = append(out, problem.Param{Name: x.Name, Type: x.Type})
			}
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
		if md.Name == "" {
			return nil, fmt.Errorf("LeetCode's metadata for %s names no function or class: %w", q.TitleSlug, ErrUnsupported)
		}
		p.Kind = problem.KindFunction
		p.Function = &problem.Function{Name: md.Name, Params: params(md.Params), Returns: md.Return.Type}
		if md.Output != nil && md.Output.ParamIndex >= 0 && md.Output.ParamIndex < len(md.Params) {
			p.Judge.Output = md.Params[md.Output.ParamIndex].Name
			switch md.Output.Size {
			case nil:
			case "ret":
				p.Judge.OutputLength = problem.OutputLengthReturn
			default:
				return nil, fmt.Errorf("%s judges an output of length %v: %w", p.Slug, md.Output.Size, ErrUnsupported)
			}
		}
	}
	p.Normalize()
	if t := unsupportedType(p); t != "" {
		return nil, fmt.Errorf("%s uses the type %s: %w", p.Slug, t, ErrUnsupported)
	}
	if md.Manual {
		// For custom-judge problems the metadata may describe LeetCode's
		// serialized test data rather than the code the user writes
		// (Clone Graph: edges integer[][] -> boolean, but the template
		// takes a *Node). Trust it only if the template agrees.
		if sig, ok := goSignatureMatches(p, conv.Templates["solution.go"]); !ok {
			return nil, fmt.Errorf("%s is judged by LeetCode's own checker, and its real signature %s cannot be driven by hc: %w", p.Slug, sig, ErrUnsupported)
		}
		conv.Warnings = append(conv.Warnings, "LeetCode judges this problem with its own checker; hc gen has to work out how to judge it")
	}
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("%s cannot be judged by hc: %w", p.Slug, err)
	}

	content := english
	if (preferTranslated || content == "") && q.TranslatedContent != "" {
		content = q.TranslatedContent
		if q.TranslatedTitle != "" {
			p.Title = q.TranslatedTitle
		}
		if !preferTranslated {
			conv.Warnings = append(conv.Warnings, "no English statement on LeetCode; using the Chinese one")
		}
	}
	desc, err := toMarkdown(content)
	if err != nil {
		return nil, err
	}
	conv.Description = fmt.Sprintf("# %s. %s\n\n%s\n", p.ID, p.Title, strings.TrimSpace(desc))

	cases, warn := examples(p, q, english)
	if warn != "" {
		conv.Warnings = append(conv.Warnings, warn)
	}
	conv.Cases, warn = checkExamples(p, cases)
	if warn != "" {
		conv.Warnings = append(conv.Warnings, warn)
	}
	return conv, nil
}

// checkExamples validates the examples against the signature: an expected
// output scraped with the wrong shape is dropped, and so is a case whose
// input does not fit.
func checkExamples(p *problem.Problem, cases []problem.Case) ([]problem.Case, string) {
	var kept []problem.Case
	cleared, dropped := 0, 0
	for _, c := range cases {
		if p.ValidateCase(c) == nil {
			kept = append(kept, c)
			continue
		}
		c.Expected = nil
		if p.ValidateCase(c) == nil {
			kept = append(kept, c)
			cleared++
			continue
		}
		dropped++
	}
	var notes []string
	if cleared > 0 {
		notes = append(notes, fmt.Sprintf("%d example output(s) do not match the signature and were left empty", cleared))
	}
	if dropped > 0 {
		notes = append(notes, fmt.Sprintf("%d example(s) do not match the signature and were dropped", dropped))
	}
	return kept, strings.Join(notes, "; ")
}

// unsupportedType returns the first type in the signature hc cannot handle.
func unsupportedType(p *problem.Problem) string {
	var params []problem.Param
	var returns []string
	if f := p.Function; f != nil {
		params, returns = f.Params, []string{f.Returns}
	}
	if c := p.Class; c != nil {
		params = append(params, c.Constructor...)
		for _, m := range c.Methods {
			params = append(params, m.Params...)
			returns = append(returns, m.Returns)
		}
	}
	for _, prm := range params {
		if problem.ValidateType(prm.Type, false) != nil {
			return prm.Type
		}
	}
	for _, r := range returns {
		if problem.ValidateType(r, true) != nil {
			return r
		}
	}
	return ""
}

var goFuncRE = regexp.MustCompile(`(?m)^func\s+(\w+)\(([^)]*)\)`)

// goSignatureMatches compares a function problem's signature with the
// parameter names of LeetCode's Go template. On a mismatch it returns the
// template's signature and false.
func goSignatureMatches(p *problem.Problem, template string) (string, bool) {
	if p.Kind != problem.KindFunction || template == "" {
		return "", true
	}
	m := goFuncRE.FindStringSubmatch(template)
	if m == nil {
		return "", true
	}
	sig := fmt.Sprintf("%s(%s)", m[1], strings.TrimSpace(m[2]))
	var names []string
	for _, part := range strings.Split(m[2], ",") {
		if fields := strings.Fields(part); len(fields) > 0 {
			names = append(names, fields[0])
		}
	}
	if m[1] != p.Function.Name || len(names) != len(p.Function.Params) {
		return sig, false
	}
	for i, prm := range p.Function.Params {
		if names[i] != prm.Name {
			return sig, false
		}
	}
	return sig, true
}

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
func examples(p *problem.Problem, q *Question, english string) ([]problem.Case, string) {
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

	src := english
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
	for rel, data := range c.Assets {
		target := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
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
