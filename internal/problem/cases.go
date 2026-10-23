package problem

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Case is a single test case, stored as one line of cases.jsonl.
//
// For function problems Input maps parameter names to values. For class
// problems Input has two keys: "ops" (method names, the first being the class
// name) and "args" (argument arrays, one per op); Expected is the array of
// return values with null for constructors and void methods.
//
// Values use LeetCode's JSON conventions: ListNode is an array of values and
// TreeNode is a level-order array with null for missing children.
type Case struct {
	Input    map[string]json.RawMessage `json:"input"`
	Expected json.RawMessage            `json:"expected,omitempty"`
	// Example marks cases shown in the statement; `hc test` runs these.
	Example bool `json:"example,omitempty"`
	// Note is a free form remark (e.g. "edge: empty array").
	Note string `json:"note,omitempty"`
}

// HasExpected reports whether the expected output is known. An explicit
// `"expected": null` (e.g. an empty tree) counts as known: json.RawMessage
// keeps the literal "null", while a missing field leaves it nil.
func (c *Case) HasExpected() bool {
	return c.Expected != nil
}

// LoadCases reads cases.jsonl from the problem directory. A missing file
// yields no cases.
func (p *Problem) LoadCases() ([]Case, error) {
	f, err := os.Open(filepath.Join(p.Dir, CasesFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseCases(f, f.Name())
}

// ParseCases parses JSON Lines; blank lines and lines starting with // are
// ignored.
func ParseCases(r interface{ Read([]byte) (int, error) }, name string) ([]Case, error) {
	var cases []Case
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<30)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "//") {
			continue
		}
		var c Case
		if err := json.Unmarshal([]byte(text), &c); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", name, line, err)
		}
		cases = append(cases, c)
	}
	return cases, sc.Err()
}

// SaveCases writes cases.jsonl atomically.
func (p *Problem) SaveCases(cases []Case) error {
	var buf bytes.Buffer
	for _, c := range cases {
		line, err := MarshalCase(c)
		if err != nil {
			return err
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	path := filepath.Join(p.Dir, CasesFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// MarshalCase encodes a case as a single compact JSON line with input keys in
// signature order.
func MarshalCase(c Case) ([]byte, error) {
	type out struct {
		Input    json.RawMessage `json:"input"`
		Expected json.RawMessage `json:"expected,omitempty"`
		Example  bool            `json:"example,omitempty"`
		Note     string          `json:"note,omitempty"`
	}
	in, err := json.Marshal(c.Input)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := json.Compact(&b, in); err != nil {
		return nil, err
	}
	o := out{Input: b.Bytes(), Example: c.Example, Note: c.Note}
	if c.Expected != nil {
		var e bytes.Buffer
		if err := json.Compact(&e, c.Expected); err != nil {
			return nil, err
		}
		o.Expected = e.Bytes()
	}
	return json.Marshal(o)
}

// InputKeys are the keys a case input must provide.
func (p *Problem) InputKeys() []string {
	if p.Kind == KindClass {
		return []string{"ops", "args"}
	}
	keys := make([]string, 0, len(p.Function.Params))
	for _, prm := range p.Function.Params {
		keys = append(keys, prm.Name)
	}
	return keys
}

// ValidateCase checks that a case provides every input of the signature.
func (p *Problem) ValidateCase(c Case) error {
	for _, k := range p.InputKeys() {
		if _, ok := c.Input[k]; !ok {
			return fmt.Errorf("missing input %q", k)
		}
	}
	if p.Kind == KindClass {
		var ops []string
		var args []json.RawMessage
		if err := json.Unmarshal(c.Input["ops"], &ops); err != nil {
			return fmt.Errorf("ops: %w", err)
		}
		if err := json.Unmarshal(c.Input["args"], &args); err != nil {
			return fmt.Errorf("args: %w", err)
		}
		if len(ops) != len(args) {
			return fmt.Errorf("ops has %d entries but args has %d", len(ops), len(args))
		}
		if len(ops) == 0 || ops[0] != p.Class.Name {
			return fmt.Errorf("first op must be the constructor %q", p.Class.Name)
		}
		known := map[string]bool{}
		for _, m := range p.Class.Methods {
			known[m.Name] = true
		}
		for _, op := range ops[1:] {
			if !known[op] {
				return fmt.Errorf("unknown method %q", op)
			}
		}
	}
	return nil
}

// FormatInput renders an input for humans, e.g. `nums = [2,7], target = 9`.
func (p *Problem) FormatInput(c Case) string {
	parts := make([]string, 0, len(c.Input))
	for _, k := range p.InputKeys() {
		parts = append(parts, fmt.Sprintf("%s = %s", k, compact(c.Input[k])))
	}
	return strings.Join(parts, ", ")
}

func compact(raw json.RawMessage) string {
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return string(raw)
	}
	return b.String()
}
