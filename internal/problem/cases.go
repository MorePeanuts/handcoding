package problem

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
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
	return ParseCases(f, filepath.Join(filepath.Base(p.Dir), CasesFile))
}

// ParseCases parses JSON Lines; blank lines and lines starting with // are
// ignored. Unknown fields are rejected so that typos such as "expect" do not
// silently drop data.
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
		c, err := parseCase([]byte(text))
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", name, line, err)
		}
		cases = append(cases, c)
	}
	return cases, sc.Err()
}

func parseCase(line []byte) (Case, error) {
	var c Case
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	err := dec.Decode(&c)
	if err == nil && dec.More() {
		err = errors.New("more than one JSON value on the line")
	}
	if te, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
		switch te.Field {
		case "input":
			return c, errors.New(`"input" must be an object mapping parameter names to values`)
		case "example":
			return c, errors.New(`"example" must be true or false`)
		case "note":
			return c, errors.New(`"note" must be a string`)
		case "":
			return c, errors.New("a case must be a JSON object with an \"input\" field")
		}
	}
	switch {
	case errors.Is(err, io.ErrUnexpectedEOF):
		return c, errors.New("incomplete JSON (missing a closing bracket or brace?)")
	case err != nil && strings.HasPrefix(err.Error(), "json: unknown field"):
		return c, fmt.Errorf("%s (a case has input, expected, example and note)", strings.TrimPrefix(err.Error(), "json: "))
	case err != nil:
		return c, err
	}
	if c.Input == nil {
		return c, errors.New(`missing "input"`)
	}
	return c, nil
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

// InputKeys are the keys a case input must provide: for function problems
// the parameters, except copies, then the hidden inputs.
func (p *Problem) InputKeys() []string {
	if p.Kind == KindClass {
		return []string{"ops", "args"}
	}
	keys := make([]string, 0, len(p.Function.Params)+len(p.Function.Hidden))
	for _, prm := range p.Function.Params {
		if prm.Copy == "" {
			keys = append(keys, prm.Name)
		}
	}
	for _, h := range p.Function.Hidden {
		keys = append(keys, h.Name)
	}
	return keys
}

// ValidateCase checks a case against the signature: every input is present,
// there are no unknown inputs, values match their declared types, and the
// expected output (if any) has the shape the judge will compare against.
func (p *Problem) ValidateCase(c Case) error {
	keys := p.InputKeys()
	for _, k := range keys {
		if _, ok := c.Input[k]; !ok {
			return fmt.Errorf("missing input %q", k)
		}
	}
	for k := range c.Input {
		if !slices.Contains(keys, k) {
			if p.Kind == KindFunction {
				if prm, ok := p.Function.Input(k); ok && prm.Copy != "" {
					return fmt.Errorf("input %q must not be given: the solution gets a copy of %q", k, prm.Copy)
				}
			}
			return fmt.Errorf("unknown input %q (expected %s)", k, strings.Join(keys, ", "))
		}
	}
	if p.Kind == KindClass {
		return p.validateClassCase(c)
	}
	f := p.Function
	for _, prm := range append(slices.Clip(f.Params), f.Hidden...) {
		t := prm.Type
		switch {
		case prm.Copy != "":
			continue
		case prm.Ref != "":
			// The value of the referenced node.
			t = TypeInteger
		}
		if err := CheckValue(t, c.Input[prm.Name]); err != nil {
			return fmt.Errorf("input %q: %w", prm.Name, err)
		}
	}
	if err := p.checkStructureCase(c); err != nil {
		return err
	}
	if !c.HasExpected() {
		return nil
	}
	want := p.judgedType()
	if p.Judge.Compare == CompareAnyOf {
		want += "[]"
	}
	if f.ReturnsRef != "" {
		want = typeNodeRef + strings.TrimPrefix(want, TypeInteger)
	}
	if err := CheckValue(want, c.Expected); err != nil {
		if p.Judge.Compare == CompareAnyOf {
			return fmt.Errorf("expected (any_of: an array of acceptable answers): %w", err)
		}
		return fmt.Errorf("expected: %w", err)
	}
	return nil
}

func (p *Problem) validateClassCase(c Case) error {
	var ops []string
	var args [][]json.RawMessage
	if err := json.Unmarshal(c.Input["ops"], &ops); err != nil {
		return errors.New(`"ops" must be an array of method names`)
	}
	if err := json.Unmarshal(c.Input["args"], &args); err != nil {
		return errors.New(`"args" must be an array of argument arrays, one per op`)
	}
	if len(ops) != len(args) {
		return fmt.Errorf("ops has %d entries but args has %d", len(ops), len(args))
	}
	if len(ops) == 0 || ops[0] != p.Class.Name {
		return fmt.Errorf("first op must be the constructor %q", p.Class.Name)
	}
	methods := map[string]Method{}
	for _, m := range p.Class.Methods {
		methods[m.Name] = m
	}
	sigs := make([]Method, len(ops))
	sigs[0] = Method{Name: p.Class.Name, Params: p.Class.Constructor, Returns: TypeVoid}
	for i, op := range ops[1:] {
		m, ok := methods[op]
		if !ok {
			return fmt.Errorf("op %d: unknown method %q", i+2, op)
		}
		sigs[i+1] = m
	}
	for i, m := range sigs {
		if len(args[i]) != len(m.Params) {
			return fmt.Errorf("op %d (%s): takes %d argument(s), got %d", i+1, m.Name, len(m.Params), len(args[i]))
		}
		for j, prm := range m.Params {
			if err := CheckValue(prm.Type, args[i][j]); err != nil {
				return fmt.Errorf("op %d (%s) argument %q: %w", i+1, m.Name, prm.Name, err)
			}
		}
	}
	if !c.HasExpected() {
		return nil
	}
	var expected []json.RawMessage
	if err := json.Unmarshal(c.Expected, &expected); err != nil {
		return errors.New("expected: must be an array with one return value per op")
	}
	if len(expected) != len(ops) {
		return fmt.Errorf("expected has %d entries but there are %d ops", len(expected), len(ops))
	}
	for i, m := range sigs {
		if m.Returns == TypeVoid {
			if string(bytes.TrimSpace(expected[i])) != "null" {
				return fmt.Errorf("expected[%d] (%s): must be null for the constructor and void methods", i, m.Name)
			}
			continue
		}
		if err := CheckValue(m.Returns, expected[i]); err != nil {
			return fmt.Errorf("expected[%d] (%s): %w", i, m.Name, err)
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
