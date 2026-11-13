package leetcode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

// examples keeps input and output from the same statement example. The API's
// exampleTestcases may contain different inputs, even with the same count.
// keys map the API's values to protocol inputs (empty keys discard a value).
func examples(p *problem.Problem, q *Question, english string, keys []string, md metaData) ([]problem.Case, string) {
	if keys == nil {
		keys = p.InputKeys()
	}
	names := keys
	var params []string
	for _, prm := range md.Params {
		if prm.Lang == "" {
			params = append(params, prm.Name)
		}
	}
	if len(params) == len(keys) {
		names = params
	}
	src := english
	if src == "" {
		src = q.TranslatedContent
	}
	var cases []problem.Case
	missing, uncertain := 0, 0
	for _, ex := range statementExamples(src) {
		in, ok := statementInput(ex.input, names, keys)
		c := problem.Case{Input: in, Example: true}
		if !ok || p.ValidateCase(c) != nil {
			uncertain++
			continue
		}
		if json.Valid([]byte(ex.output)) {
			c.Expected = json.RawMessage(compactJSON(ex.output))
		} else {
			missing++
		}
		cases = append(cases, c)
	}
	var warnings []string
	if len(cases) == 0 || uncertain > 0 {
		fallback, warn := apiExamples(q.ExampleTestcases, keys)
		if warn != "" {
			warnings = append(warnings, warn)
		}
		// Keep safely parsed statement pairs, supplementing them with API inputs
		// we could not recover. Never borrow an output for a fallback input.
		seen := map[string]bool{}
		for _, c := range cases {
			b, _ := json.Marshal(c.Input)
			seen[string(b)] = true
		}
		for _, c := range fallback {
			b, _ := json.Marshal(c.Input)
			if !seen[string(b)] {
				cases = append(cases, c)
			}
		}
		warnings = append(warnings, "could not verify all statement example inputs; unmatched API inputs have no expected output (run hc gen)")
	}
	if missing > 0 {
		warnings = append(warnings, fmt.Sprintf("%d example output(s) are not plain JSON and were left empty (run hc gen)", missing))
	}
	return cases, strings.Join(warnings, "; ")
}

func apiExamples(raw string, keys []string) ([]problem.Case, string) {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, "no example test cases provided by LeetCode"
	}
	if len(keys) == 0 || len(lines)%len(keys) != 0 {
		return nil, "could not split the example test cases; run hc gen"
	}
	var cases []problem.Case
	for i := 0; i < len(lines); i += len(keys) {
		c := problem.Case{Input: map[string]json.RawMessage{}, Example: true}
		for j, k := range keys {
			v := strings.TrimSpace(lines[i+j])
			if !json.Valid([]byte(v)) {
				return nil, fmt.Sprintf("example input %q is not JSON; run hc gen", v)
			}
			if k != "" {
				c.Input[k] = json.RawMessage(compactJSON(v))
			}
		}
		cases = append(cases, c)
	}
	return cases, ""
}

type statementExample struct{ input, output string }

var exampleLabelRE = regexp.MustCompile(`(?m)^[\t ]*(Input|Output|Explanation|输入|输出|解释|说明)[\t ]*(?:[:：][\t ]*|\n|$)`)
var inputNameRE = regexp.MustCompile(`^([A-Za-z_][A-Za-z_0-9]*)\s*=\s*`)

// statementExamples recognizes LeetCode's old pre blocks and new example-block
// divs. Scope labels to one block so missing fields cannot shift later pairs.
func statementExamples(src string) []statementExample {
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		return nil
	}
	var examples []statementExample
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		block := n.Type == html.ElementNode && n.Data == "pre"
		if n.Type == html.ElementNode && n.Data == "div" {
			for _, a := range n.Attr {
				if a.Key == "class" {
					for _, class := range strings.Fields(a.Val) {
						if class == "example-block" {
							block = true
						}
					}
				}
			}
		}
		if block {
			text := exampleText(n)
			labels := exampleLabelRE.FindAllStringSubmatchIndex(text, -1)
			var ex statementExample
			inputs, outputs := 0, 0
			for i, label := range labels {
				end := len(text)
				if i+1 < len(labels) {
					end = labels[i+1][0]
				}
				value := strings.TrimSpace(text[label[1]:end])
				switch text[label[2]:label[3]] {
				case "Input", "输入":
					ex.input = value
					inputs++
				case "Output", "输出":
					ex.output = value
					outputs++
				}
			}
			if inputs > 0 || outputs > 0 {
				if inputs != 1 || outputs > 1 {
					ex = statementExample{}
				}
				examples = append(examples, ex)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(doc)
	return examples
}

func exampleText(n *html.Node) string {
	var b strings.Builder
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			return
		}
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style":
				return
			case "br", "p", "div":
				b.WriteByte('\n')
			case "strong", "b":
				// Labels may be adjacent in HTML, without a source newline.
				var label strings.Builder
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.TextNode {
						label.WriteString(c.Data)
					}
				}
				switch strings.Trim(strings.TrimSpace(label.String()), ":：") {
				case "Input", "Output", "Explanation", "输入", "输出", "解释", "说明":
					b.WriteByte('\n')
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
		if n.Type == html.ElementNode && (n.Data == "p" || n.Data == "div") {
			b.WriteByte('\n')
		}
	}
	visit(n)
	return strings.ReplaceAll(b.String(), "\u00a0", " ")
}

// statementInput reads complete JSON values, so commas, equals signs and
// escaped quotes inside strings or nested arrays never split parameters.
func statementInput(text string, names, keys []string) (map[string]json.RawMessage, bool) {
	values := map[string]json.RawMessage{}
	used := map[int]bool{}
	count := 0
	named := false
	for text = strings.TrimSpace(text); text != ""; {
		name := ""
		if m := inputNameRE.FindStringSubmatch(text); m != nil {
			name = m[1]
			text = text[len(m[0]):]
		}
		if count == 0 {
			named = name != ""
		} else if named != (name != "") {
			return nil, false
		}
		i := count
		if named {
			i = -1
			for j, n := range names {
				if name == n {
					i = j
					break
				}
			}
			// A single value has only one possible parameter, including aliases
			// such as adjList (statement), edges (metadata), node (template).
			if i < 0 && len(keys) == 1 {
				i = 0
			}
		}
		if i < 0 || i >= len(keys) || used[i] {
			return nil, false
		}
		var v json.RawMessage
		dec := json.NewDecoder(strings.NewReader(text))
		if err := dec.Decode(&v); err != nil {
			return nil, false
		}
		used[i] = true
		if keys[i] != "" {
			values[keys[i]] = json.RawMessage(compactJSON(string(v)))
		}
		count++
		rest := text[dec.InputOffset():]
		text = strings.TrimSpace(rest)
		if text != "" {
			if text[0] == ',' {
				text = strings.TrimSpace(text[1:])
				if text == "" {
					return nil, false
				}
			} else if len(rest) == len(text) {
				return nil, false
			}
		}
	}
	return values, count == len(keys) && count > 0
}

func compactJSON(s string) string {
	var b bytes.Buffer
	if err := json.Compact(&b, []byte(s)); err != nil {
		return s
	}
	return b.String()
}
