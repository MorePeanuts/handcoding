package problem

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Case is a single test case, stored as one line of cases.jsonl.
//
// For function problems Input maps parameter names to values. For class
// problems Input has two keys: "ops" (method names, the first being the class
// name) and "args" (argument arrays, one per op); Expected is the array of
// return values with null for constructors and void methods. For round-trip
// problems (Judge.RoundTrip) Input maps the encoder's parameter name to the
// value, and Expected is the decoder's result.
//
// A generated case (Generate) has neither in cases.jsonl: generator.py
// builds its input and a reference solution its expected output when it is
// judged, and hc sets both on the Case in memory (materializing it).
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
	// Generate names the generator that builds the input of a generated
	// case.
	Generate *Generate `json:"generate,omitempty"`
	// Digest is InputDigest of the generated input, as the author's
	// generator built it; hc cases build writes it.
	Digest string `json:"digest,omitempty"`
	// User marks a case the user keeps apart from the problem's cases, for
	// a problem whose pack is read-only. It is not part of the protocol:
	// hc sets it on the cases it reads from the user's file.
	User bool `json:"-"`
}

// Generate is a call of a function of generator.py: name(rng, **args), with
// rng seeded with Seed.
type Generate struct {
	Name string                     `json:"name"`
	Args map[string]json.RawMessage `json:"args,omitempty"`
	Seed int64                      `json:"seed,omitempty"`
}

// String renders the call for humans, e.g. `descending(n=100000)` or
// `distinct(hi=9, lo=0, n=5) seed 7`.
func (g *Generate) String() string {
	keys := slices.Sorted(maps.Keys(g.Args))
	args := make([]string, len(keys))
	for i, k := range keys {
		args[i] = k + "=" + compact(g.Args[k])
	}
	s := g.Name + "(" + strings.Join(args, ", ") + ")"
	if g.Seed != 0 {
		s += fmt.Sprintf(" seed %d", g.Seed)
	}
	return s
}

// MaxStoredCase is the size above which a stored case (its line in
// cases.jsonl) belongs in a generator instead.
const MaxStoredCase = 100 << 10

// HasExpected reports whether the expected output is known. An explicit
// `"expected": null` (e.g. an empty tree) counts as known: json.RawMessage
// keeps the literal "null", while a missing field leaves it nil.
func (c *Case) HasExpected() bool {
	return c.Expected != nil
}

// Generated reports whether the case is built by generator.py.
func (c *Case) Generated() bool { return c.Generate != nil }

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

// ParseCase parses one line of cases.jsonl.
func ParseCase(line []byte) (Case, error) { return parseCase(line) }

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
		case "digest":
			return c, errors.New(`"digest" must be a string`)
		case "generate":
			return c, errors.New(`"generate" must be an object with name, args and seed`)
		case "generate.name":
			return c, errors.New(`"generate.name" must be a string`)
		case "generate.args":
			return c, errors.New(`"generate.args" must be an object mapping argument names to values`)
		case "generate.seed":
			return c, errors.New(`"generate.seed" must be an integer`)
		case "":
			return c, errors.New("a case must be a JSON object with an \"input\" field")
		}
	}
	switch {
	case errors.Is(err, io.ErrUnexpectedEOF):
		return c, errors.New("incomplete JSON (missing a closing bracket or brace?)")
	case err != nil && strings.HasPrefix(err.Error(), "json: unknown field"):
		return c, fmt.Errorf("%s (a case has input, expected, example and note, or generate, digest and note; generate has name, args and seed)", strings.TrimPrefix(err.Error(), "json: "))
	case err != nil:
		return c, err
	}
	if c.Generated() {
		return c, checkGenerated(c)
	}
	if c.Input == nil {
		return c, errors.New(`missing "input"`)
	}
	if c.Digest != "" {
		return c, errors.New(`"digest" is only for generated cases (with "generate")`)
	}
	return c, nil
}

// checkGenerated checks the fields of a generated case as cases.jsonl has
// them.
func checkGenerated(c Case) error {
	switch {
	case c.Input != nil:
		return errors.New(`a case has "input" or "generate", not both`)
	case c.Expected != nil:
		return errors.New(`a generated case has no "expected": the reference solution computes it when the case is judged`)
	case c.Example:
		return errors.New("a generated case cannot be an example")
	case !identRE.MatchString(c.Generate.Name):
		return fmt.Errorf(`"generate.name" %q must name a function of %s`, c.Generate.Name, GeneratorFile)
	case c.Digest != "" && !digestRE.MatchString(c.Digest):
		return fmt.Errorf(`"digest" %q must be "sha256:" and 64 lowercase hex digits (hc cases build writes it)`, c.Digest)
	}
	for k := range c.Generate.Args {
		if !identRE.MatchString(k) || k == "rng" {
			return fmt.Errorf("generate.args: %q is not a valid argument name", k)
		}
	}
	return nil
}

var (
	identRE  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	digestRE = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// InputDigest is the digest of a case input: "sha256:" and the hex SHA-256
// of its canonical JSON, the encoding MarshalCase writes inputs in (keys
// sorted, no spaces, number literals as they are, strings unescaped).
func InputDigest(input map[string]json.RawMessage) (string, error) {
	o := make(map[string]any, len(input))
	for k, v := range input {
		d, err := decodeValue(v)
		if err != nil {
			return "", fmt.Errorf("input %q: %w", k, err)
		}
		o[k] = d
	}
	data, err := encodeCompact(o)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
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
// sorted order. Values are re-encoded so that cases.jsonl stays readable
// whoever wrote them: strings hold & < > and non-ASCII characters as they
// are, not as \u escapes (Go's encoder escapes the former, Python's the
// latter), and numbers keep their literal text.
func MarshalCase(c Case) ([]byte, error) {
	if c.Generated() {
		return marshalGenerated(c)
	}
	type out struct {
		Input    map[string]any `json:"input"`
		Expected any            `json:"expected,omitempty"`
		Example  bool           `json:"example,omitempty"`
		Note     string         `json:"note,omitempty"`
	}
	o := out{Input: make(map[string]any, len(c.Input)), Example: c.Example, Note: c.Note}
	for k, v := range c.Input {
		d, err := decodeValue(v)
		if err != nil {
			return nil, fmt.Errorf("input %q: %w", k, err)
		}
		o.Input[k] = d
	}
	if c.HasExpected() {
		d, err := decodeValue(c.Expected)
		if err != nil {
			return nil, fmt.Errorf("expected: %w", err)
		}
		// A literal null stays: it is a known expected output.
		o.Expected = json.RawMessage("null")
		if d != nil {
			o.Expected = d
		}
	}
	return encodeCompact(o)
}

// marshalGenerated encodes a generated case as cases.jsonl holds it: the
// input and expected output it was materialized with are left out.
func marshalGenerated(c Case) ([]byte, error) {
	type gen struct {
		Name string         `json:"name"`
		Args map[string]any `json:"args,omitempty"`
		Seed int64          `json:"seed,omitempty"`
	}
	type out struct {
		Generate gen    `json:"generate"`
		Digest   string `json:"digest,omitempty"`
		Note     string `json:"note,omitempty"`
	}
	o := out{Generate: gen{Name: c.Generate.Name, Seed: c.Generate.Seed}, Digest: c.Digest, Note: c.Note}
	if len(c.Generate.Args) > 0 {
		o.Generate.Args = make(map[string]any, len(c.Generate.Args))
		for k, v := range c.Generate.Args {
			d, err := decodeValue(v)
			if err != nil {
				return nil, fmt.Errorf("generate.args %q: %w", k, err)
			}
			o.Generate.Args[k] = d
		}
	}
	return encodeCompact(o)
}

// encodeCompact encodes v as one line without escaping & < >.
func encodeCompact(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

// decodeValue decodes a JSON value, keeping numbers as json.Number so that
// re-encoding them yields the same text.
func decodeValue(raw json.RawMessage) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// InputKeys are the keys a case input must provide: for function problems
// the parameters, except copies, then the hidden inputs; for round-trip
// problems the encoder's parameter.
func (p *Problem) InputKeys() []string {
	if enc, _, ok := p.RoundTripMethods(); ok {
		return []string{enc.Params[0].Name}
	}
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
//
// A generated case not yet materialized (no Input) is checked for what
// cases.jsonl holds; once materialized, its input and expected output are
// checked like a stored case's.
func (p *Problem) ValidateCase(c Case) error {
	if c.Generated() {
		if p.Judge.Compare == CompareAnyOf {
			return errors.New("judge.compare any_of cannot judge generated cases: a reference finds one answer, and any_of needs all of them (use unordered or a checker)")
		}
		if c.Input == nil {
			return nil
		}
	}
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
	if enc, _, ok := p.RoundTripMethods(); ok {
		prm := enc.Params[0]
		if err := CheckValue(prm.Type, c.Input[prm.Name]); err != nil {
			return fmt.Errorf("input %q: %w", prm.Name, err)
		}
		return p.validateExpected(c)
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
	return p.validateExpected(c)
}

// ValidateExpected checks the expected output of a case, if it has one,
// whose input is valid: it has the shape the judge compares outputs in, as
// ValidateCase checks it. It is how the expected outputs a reference
// computes for generated cases are checked, whose inputs were checked when
// they were built.
func (p *Problem) ValidateExpected(c Case) error {
	if !c.HasExpected() {
		return nil
	}
	if _, _, ok := p.RoundTripMethods(); ok || p.Kind != KindClass {
		return p.validateExpected(c)
	}
	ops, err := classOps(c)
	if err != nil {
		return err
	}
	sigs, err := p.classSigs(ops)
	if err != nil {
		return err
	}
	return validateClassExpected(c, sigs)
}

// validateExpected checks the shape of a function or round-trip problem's
// expected output, if the case has one.
func (p *Problem) validateExpected(c Case) error {
	if !c.HasExpected() {
		return nil
	}
	want := outputForm(p.judgedType())
	if p.Judge.Compare == CompareAnyOf {
		want += "[]"
	}
	if p.Function != nil && p.Function.ReturnsRef != "" {
		want = typeNodeRef + strings.TrimPrefix(want, TypeInteger)
	}
	if err := checkOutput(want, c.Expected); err != nil {
		if p.Judge.Compare == CompareAnyOf {
			return fmt.Errorf("expected (any_of: an array of acceptable answers): %w", err)
		}
		return fmt.Errorf("expected: %w", err)
	}
	return nil
}

// validateClassCase checks a class case. A design problem's case can make
// hundreds of thousands of calls, so scalar arguments and return values,
// most of them, are checked on their text (see scalarOK).
func (p *Problem) validateClassCase(c Case) error {
	ops, err := classOps(c)
	if err != nil {
		return err
	}
	var args [][]json.RawMessage
	if err := json.Unmarshal(c.Input["args"], &args); err != nil {
		return errors.New(`"args" must be an array of argument arrays, one per op`)
	}
	if len(ops) != len(args) {
		return fmt.Errorf("ops has %d entries but args has %d", len(ops), len(args))
	}
	sigs, err := p.classSigs(ops)
	if err != nil {
		return err
	}
	for i, m := range sigs {
		if len(args[i]) != len(m.Params) {
			return fmt.Errorf("op %d (%s): takes %d argument(s), got %d", i+1, m.Name, len(m.Params), len(args[i]))
		}
		for j, prm := range m.Params {
			if scalarOK(prm.Type, args[i][j]) {
				continue
			}
			if err := CheckValue(prm.Type, args[i][j]); err != nil {
				return fmt.Errorf("op %d (%s) argument %q: %w", i+1, m.Name, prm.Name, err)
			}
		}
	}
	return validateClassExpected(c, sigs)
}

// classOps decodes a class case's ops.
func classOps(c Case) ([]string, error) {
	var ops []string
	if err := json.Unmarshal(c.Input["ops"], &ops); err != nil {
		return nil, errors.New(`"ops" must be an array of method names`)
	}
	return ops, nil
}

// classSigs are the signatures of a class case's ops: the constructor
// first, then methods of the class.
func (p *Problem) classSigs(ops []string) ([]Method, error) {
	if len(ops) == 0 || ops[0] != p.Class.Name {
		return nil, fmt.Errorf("first op must be the constructor %q", p.Class.Name)
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
			return nil, fmt.Errorf("op %d: unknown method %q", i+2, op)
		}
		sigs[i+1] = m
	}
	return sigs, nil
}

// validateClassExpected checks a class case's expected output, if it has
// one, against the signatures of its ops.
func validateClassExpected(c Case, sigs []Method) error {
	if !c.HasExpected() {
		return nil
	}
	expected, ok := arrayElements(c.Expected)
	if !ok {
		if err := json.Unmarshal(c.Expected, &expected); err != nil {
			return errors.New("expected: must be an array with one return value per op")
		}
	}
	if len(expected) != len(sigs) {
		return fmt.Errorf("expected has %d entries but there are %d ops", len(expected), len(sigs))
	}
	for i, m := range sigs {
		if m.Returns == TypeVoid {
			if string(bytes.TrimSpace(expected[i])) != "null" {
				return fmt.Errorf("expected[%d] (%s): must be null for the constructor and void methods", i, m.Name)
			}
			continue
		}
		// Scalars have no other output form than their input form.
		t := outputForm(m.Returns)
		if scalarOK(t, expected[i]) {
			continue
		}
		if err := checkOutput(t, expected[i]); err != nil {
			return fmt.Errorf("expected[%d] (%s): %w", i, m.Name, err)
		}
	}
	return nil
}

// FormatInput renders an input for humans, e.g. `nums = [2,7], target = 9`.
// A generated case starts with its generator, `descending(n=5): nums = …`,
// or is only that before it is materialized.
func (p *Problem) FormatInput(c Case) string {
	parts := make([]string, 0, len(c.Input))
	for _, k := range p.InputKeys() {
		parts = append(parts, fmt.Sprintf("%s = %s", k, compact(c.Input[k])))
	}
	s := strings.Join(parts, ", ")
	if c.Generated() {
		if c.Input == nil {
			return c.Generate.String()
		}
		return c.Generate.String() + ": " + s
	}
	return s
}

func compact(raw json.RawMessage) string {
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return string(raw)
	}
	return b.String()
}
