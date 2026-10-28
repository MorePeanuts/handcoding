// Package problem defines the handcoding problem protocol: the on-disk layout
// of a problem, its signature and its test cases.
//
// A problem lives in its own directory:
//
//	<dir>/
//	  problem.yaml      metadata, signature and judge options
//	  description.md    statement (Markdown)
//	  assets/           optional images referenced by the statement (![](assets/x.png))
//	  cases.jsonl       test cases, one JSON object per line
//	  templates/        optional starter code per language (solution.go, solution.py, ...)
//	  reference/        optional reference solutions used to compute expected outputs
//	  checker.py        the checker, with judge.compare: checker
package problem

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

const (
	MetaFile        = "problem.yaml"
	DescriptionFile = "description.md"
	CasesFile       = "cases.jsonl"
	TemplatesDir    = "templates"
	ReferenceDir    = "reference"
	AssetsDir       = "assets"
	// CheckerFile judges the outputs of a problem with judge.compare:
	// checker. Checkers are Python whatever the solution's language.
	CheckerFile = "checker.py"
)

// Kind is the shape of a problem.
type Kind string

const (
	// KindFunction problems ask for a single function (or Solution method).
	KindFunction Kind = "function"
	// KindClass problems ask for a class driven by a sequence of operations
	// (LeetCode "design" problems such as LRU Cache).
	KindClass Kind = "class"
)

// Difficulty levels.
const (
	Easy   = "easy"
	Medium = "medium"
	Hard   = "hard"
)

// Param is a named, typed parameter.
//
// The structure attributes Cycle, Join, Ref and Copy (function parameters
// only, at most one each) make the driver build the argument from other
// inputs rather than pass the case value as it is.
type Param struct {
	Name string `yaml:"name" json:"name"`
	Type string `yaml:"type" json:"type"`
	// Cycle names an integer hidden input: the ListNode's tail links back to
	// the node at that 0-based index, or nowhere for -1 (Linked List Cycle).
	Cycle string `yaml:"cycle,omitempty" json:"cycle,omitempty"`
	// Join makes this ListNode share its tail with another list
	// (Intersection of Two Linked Lists).
	Join *Join `yaml:"join,omitempty" json:"join,omitempty"`
	// Ref names a ListNode or TreeNode input of the same type. The case gives
	// a node's value, and the solution receives the node of that structure
	// holding it (Lowest Common Ancestor: p and q in root).
	Ref string `yaml:"ref,omitempty" json:"ref,omitempty"`
	// Copy names a parameter of the same node type. The case does not give
	// this parameter: the solution receives a deep copy of the other one.
	Copy string `yaml:"copy,omitempty" json:"copy,omitempty"`
}

// Join describes a list that continues with the nodes of another one: it
// keeps its first Skip nodes, then links to List's node at index At. Skip and
// At name integer hidden inputs. When At equals List's length the lists do
// not intersect, and the joined list keeps all its own nodes.
type Join struct {
	List string `yaml:"list" json:"list"`
	Skip string `yaml:"skip" json:"skip"`
	At   string `yaml:"at" json:"at"`
}

// structural reports whether the driver builds the parameter from other
// inputs.
func (prm Param) structural() bool {
	return prm.Cycle != "" || prm.Join != nil || prm.Ref != "" || prm.Copy != ""
}

// Function describes the signature of a function-kind problem.
type Function struct {
	Name   string  `yaml:"name" json:"name"`
	Params []Param `yaml:"params" json:"params"`
	// Hidden are case inputs the driver uses to build parameters (or judges,
	// with judge.output) but never passes to the solution, e.g. the pos of a
	// list cycle.
	Hidden  []Param `yaml:"hidden,omitempty" json:"hidden,omitempty"`
	Returns string  `yaml:"returns" json:"returns"`
	// ReturnsRef names the ListNode or TreeNode input the returned node
	// belongs to. The output is the node's index in that list, or its value
	// in that tree, and null for no node.
	ReturnsRef string `yaml:"returns_ref,omitempty" json:"returns_ref,omitempty"`
	// ReturnsCopy names the parameter the function deep-copies (Clone
	// Graph): the returned structure must not share a node with it.
	ReturnsCopy string `yaml:"returns_copy,omitempty" json:"returns_copy,omitempty"`
}

// Input returns the parameter or hidden input with the given name.
func (f *Function) Input(name string) (Param, bool) {
	for _, prm := range f.Params {
		if prm.Name == name {
			return prm, true
		}
	}
	for _, h := range f.Hidden {
		if h.Name == name {
			return h, true
		}
	}
	return Param{}, false
}

// IsHidden reports whether name is a hidden input.
func (f *Function) IsHidden(name string) bool {
	for _, h := range f.Hidden {
		if h.Name == name {
			return true
		}
	}
	return false
}

// Method is a method of a class-kind problem.
type Method struct {
	Name    string  `yaml:"name" json:"name"`
	Params  []Param `yaml:"params" json:"params"`
	Returns string  `yaml:"returns" json:"returns"`
}

// Class describes the signature of a class-kind problem.
type Class struct {
	Name        string   `yaml:"name" json:"name"`
	Constructor []Param  `yaml:"constructor" json:"constructor"`
	Methods     []Method `yaml:"methods" json:"methods"`
}

// Compare modes understood by the judge.
const (
	CompareExact         = "exact"
	CompareUnordered     = "unordered"      // outermost array order is ignored
	CompareUnorderedDeep = "unordered_deep" // order is ignored at every array level
	CompareAnyOf         = "any_of"         // expected is an array of acceptable answers
	CompareChecker       = "checker"        // checker.py decides; expected is one valid answer
)

// Judge holds judging options.
type Judge struct {
	// Compare is one of the Compare* modes. It has no default: an empty value
	// means the judge rule has not been decided yet (see Ready), as for a
	// freshly fetched problem.
	Compare string `yaml:"compare,omitempty" json:"compare,omitempty"`
	// Tolerance is the absolute/relative tolerance for floating point
	// numbers, as written in problem.yaml: nil when absent, so that an
	// explicit 0 (compare exactly) differs from the default. Problem.Tolerance
	// gives the tolerance in effect.
	Tolerance *float64 `yaml:"tolerance,omitempty" json:"tolerance,omitempty"`
	// Output names a parameter whose value after the call is the answer, for
	// problems that modify their input in place (e.g. "rotate the array").
	Output string `yaml:"output,omitempty" json:"output,omitempty"`
	// OutputLength limits the judged Output to a prefix. The only value is
	// OutputLengthReturn: the function returns k, and only the first k
	// elements of Output are judged (LeetCode's "remove duplicates" style).
	OutputLength string `yaml:"output_length,omitempty" json:"output_length,omitempty"`
	// RoundTrip makes a class problem judge an encoder and a decoder
	// together: decode(encode(x)) is judged, never the encoded value.
	RoundTrip *RoundTrip `yaml:"round_trip,omitempty" json:"round_trip,omitempty"`
	// Timeout is the wall clock limit for a whole run, e.g. "10s": it guards
	// against hangs, and cases without a result when it expires are Time
	// Limit Exceeded. The checker gets the same limit. Defaults to timeout in
	// hc.yaml.
	Timeout string `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// OutputLengthReturn judges Output[:k], where k is the return value.
const OutputLengthReturn = "return"

// RoundTrip describes a class problem whose methods encode a value into a
// string in the solution's own format and decode it back (Serialize and
// Deserialize Binary Tree, Encode and Decode TinyURL). A case gives the value
// as Encode's parameter; the driver calls Encode, passes the string it
// returns to Decode, and Decode's result is judged.
type RoundTrip struct {
	Encode string `yaml:"encode" json:"encode"`
	Decode string `yaml:"decode" json:"decode"`
	// Instances is InstancesSeparate or InstancesSame. It has no default,
	// because it decides whether a solution may keep state between the calls.
	Instances string `yaml:"instances" json:"instances"`
}

// Round-trip instances.
const (
	// InstancesSeparate encodes with one instance and decodes with a second
	// one (297: ser = Codec(); deser = Codec()).
	InstancesSeparate = "separate"
	// InstancesSame encodes and decodes with one instance, which may remember
	// what it encoded (535: a URL shortener keeps its mapping).
	InstancesSame = "same"
)

// Source records where a problem came from.
type Source struct {
	Site string `yaml:"site,omitempty" json:"site,omitempty"` // leetcode.com, leetcode.cn, custom
	URL  string `yaml:"url,omitempty" json:"url,omitempty"`
}

// Problem is the content of problem.yaml plus runtime information.
type Problem struct {
	ID         string    `yaml:"id,omitempty"`
	Slug       string    `yaml:"slug"`
	Title      string    `yaml:"title"`
	Difficulty string    `yaml:"difficulty,omitempty"`
	Tags       []string  `yaml:"tags,omitempty"`
	Kind       Kind      `yaml:"kind"`
	Function   *Function `yaml:"function,omitempty"`
	Class      *Class    `yaml:"class,omitempty"`
	Judge      Judge     `yaml:"judge,omitempty"`
	Source     Source    `yaml:"source,omitempty"`

	// Dir is the directory the problem was loaded from.
	Dir string `yaml:"-"`
	// Pack is the name of the pack the problem belongs to, set by the
	// workspace that loaded it; empty for a problem loaded on its own.
	Pack string `yaml:"-"`
}

// Ref is the reference that names the problem in any workspace: pack/slug,
// or the slug alone when the problem belongs to no pack. Solutions and
// history are keyed by it.
func (p *Problem) Ref() string {
	if p.Pack == "" {
		return p.Slug
	}
	return p.Pack + "/" + p.Slug
}

var (
	// slugRE allows upper case because some leetcode.cn slugs use it
	// ("xoh6Oh"); the protocol recommends lower case.
	slugRE = regexp.MustCompile(`^[A-Za-z0-9]+(-[A-Za-z0-9]+)*$`)
	// flowArrayTypeRE spots an unquoted array type inside YAML flow syntax,
	// where "[]" is read as a sequence: {name: nums, type: integer[]}.
	flowArrayTypeRE = regexp.MustCompile(`\{[^}\n]*type:\s*[A-Za-z<>]+\[\]`)
)

// DirName is the canonical directory name for the problem, e.g. "0001-two-sum".
func (p *Problem) DirName() string {
	if n, err := strconv.Atoi(p.ID); err == nil {
		return fmt.Sprintf("%04d-%s", n, p.Slug)
	}
	return p.Slug
}

// Normalize fills defaults and canonicalizes type names.
func (p *Problem) Normalize() {
	p.Difficulty = strings.ToLower(p.Difficulty)
	norm := func(ps []Param) {
		for i := range ps {
			ps[i].Type = NormalizeType(ps[i].Type)
		}
	}
	if f := p.Function; f != nil {
		norm(f.Params)
		norm(f.Hidden)
		f.Returns = NormalizeType(f.Returns)
		if f.Returns == "" {
			f.Returns = TypeVoid
		}
	}
	if c := p.Class; c != nil {
		norm(c.Constructor)
		for i := range c.Methods {
			norm(c.Methods[i].Params)
			c.Methods[i].Returns = NormalizeType(c.Methods[i].Returns)
			if c.Methods[i].Returns == "" {
				c.Methods[i].Returns = TypeVoid
			}
		}
	}
}

// DefaultTolerance is the tolerance of a problem that judges doubles and
// does not set judge.tolerance.
const DefaultTolerance = 1e-5

// Tolerance is the floating point tolerance the judge compares with:
// judge.tolerance when set (0 compares exactly), else DefaultTolerance if a
// judged value has base type double, else 0.
func (p *Problem) Tolerance() float64 {
	if t := p.Judge.Tolerance; t != nil {
		return *t
	}
	if p.judgesDouble() {
		return DefaultTolerance
	}
	return 0
}

// judgesDouble reports whether a judged value has base type double: the
// judged value of a function or round-trip problem (see judgedType), or the
// return value of a method of any other class problem.
func (p *Problem) judgesDouble() bool {
	types := []string{p.judgedType()}
	if _, _, ok := p.RoundTripMethods(); !ok && p.Kind == KindClass && p.Class != nil {
		types = types[:0]
		for _, m := range p.Class.Methods {
			types = append(types, m.Returns)
		}
	}
	for _, t := range types {
		if base, _ := SplitArray(t); base == TypeDouble {
			return true
		}
	}
	return false
}

// errKindRequired is reported for a problem.yaml without kind, which is
// never inferred from the sections present.
var errKindRequired = errors.New("kind is required: function or class")

// Validate checks the problem definition for consistency.
func (p *Problem) Validate() error {
	var errs []error
	if !slugRE.MatchString(p.Slug) {
		errs = append(errs, fmt.Errorf("slug %q must be kebab-case: letters, digits and single hyphens", p.Slug))
	}
	if p.Title == "" {
		errs = append(errs, errors.New("title is required"))
	}
	switch p.Difficulty {
	case "", Easy, Medium, Hard:
	default:
		errs = append(errs, fmt.Errorf("difficulty %q must be easy, medium or hard", p.Difficulty))
	}
	checkParams := func(where string, ps []Param, seen map[string]bool) {
		for _, prm := range ps {
			if prm.Name == "" {
				errs = append(errs, fmt.Errorf("%s: parameter without name", where))
			}
			if seen[prm.Name] {
				errs = append(errs, fmt.Errorf("%s: duplicate parameter %q", where, prm.Name))
			}
			seen[prm.Name] = true
			if err := ValidateType(prm.Type, false); err != nil {
				errs = append(errs, fmt.Errorf("%s: parameter %q: %w", where, prm.Name, err))
			}
		}
	}
	noStructures := func(where string, ps []Param) {
		for _, prm := range ps {
			if prm.structural() {
				errs = append(errs, fmt.Errorf("%s: parameter %q: cycle, join, ref and copy are for function problems only", where, prm.Name))
			}
		}
	}
	switch p.Kind {
	case KindFunction:
		f := p.Function
		if f == nil {
			errs = append(errs, errors.New("kind function requires a function section"))
			break
		}
		if f.Name == "" {
			errs = append(errs, errors.New("function.name is required"))
		}
		if p.Class != nil {
			errs = append(errs, errors.New("kind function must not have a class section"))
		}
		seen := map[string]bool{}
		checkParams("function", f.Params, seen)
		checkParams("function.hidden", f.Hidden, seen)
		if err := ValidateType(f.Returns, true); err != nil {
			errs = append(errs, fmt.Errorf("function.returns: %w", err))
		}
		errs = append(errs, p.validateStructures()...)
		errs = append(errs, p.validateReturnsCopy()...)
		if p.Judge.RoundTrip != nil {
			errs = append(errs, errors.New("judge.round_trip is for class problems only"))
		}
		if p.Judge.Output != "" {
			if prm, ok := f.Input(p.Judge.Output); !ok {
				errs = append(errs, fmt.Errorf("judge.output %q is not a parameter or hidden input", p.Judge.Output))
			} else if prm.Cycle != "" {
				errs = append(errs, fmt.Errorf("judge.output %q may have a cycle and cannot be judged", p.Judge.Output))
			}
		} else if f.Returns == TypeVoid {
			errs = append(errs, errors.New("void functions need judge.output to name the modified parameter"))
		}
		switch p.Judge.OutputLength {
		case "":
		case OutputLengthReturn:
			_, depth := SplitArray(p.judgedType())
			switch {
			case p.Judge.Output == "":
				errs = append(errs, errors.New("judge.output_length needs judge.output"))
			case f.Returns != TypeInteger && f.Returns != TypeLong:
				errs = append(errs, fmt.Errorf("judge.output_length return needs an integer return value, not %s", f.Returns))
			case depth == 0:
				errs = append(errs, errors.New("judge.output_length needs an array judge.output parameter"))
			}
		default:
			errs = append(errs, fmt.Errorf("unknown judge.output_length %q (only \"return\")", p.Judge.OutputLength))
		}
	case KindClass:
		c := p.Class
		if c == nil {
			errs = append(errs, errors.New("kind class requires a class section"))
			break
		}
		if c.Name == "" {
			errs = append(errs, errors.New("class.name is required"))
		}
		if p.Function != nil {
			errs = append(errs, errors.New("kind class must not have a function section"))
		}
		checkParams("class constructor", c.Constructor, map[string]bool{})
		noStructures("class constructor", c.Constructor)
		if len(c.Methods) == 0 {
			errs = append(errs, errors.New("class.methods must not be empty"))
		}
		// Go capitalizes method names, so "get" and "Get" are one method.
		methods := map[string]string{}
		for _, m := range c.Methods {
			key := strings.ToUpper(m.Name[:min(1, len(m.Name))]) + m.Name[min(1, len(m.Name)):]
			switch prev, dup := methods[key]; {
			case m.Name == "":
				errs = append(errs, errors.New("class.methods: method without name"))
			case dup && prev == m.Name:
				errs = append(errs, fmt.Errorf("class.methods: duplicate method %q", m.Name))
			case dup:
				errs = append(errs, fmt.Errorf("class.methods: methods %q and %q differ only in the case of their first letter, and are one method in Go", prev, m.Name))
			}
			methods[key] = m.Name
			checkParams("method "+m.Name, m.Params, map[string]bool{})
			noStructures("method "+m.Name, m.Params)
			if err := ValidateType(m.Returns, true); err != nil {
				errs = append(errs, fmt.Errorf("method %s returns: %w", m.Name, err))
			}
		}
		if p.Judge.RoundTrip != nil {
			errs = append(errs, p.validateRoundTrip()...)
		} else if p.Judge.Compare == CompareAnyOf {
			errs = append(errs, errors.New("judge.compare any_of is not usable for a class problem that is not a round trip: its expected holds one entry per op"))
		}
		if p.Judge.Output != "" {
			errs = append(errs, errors.New("judge.output is for function problems only"))
		}
		if p.Judge.OutputLength != "" {
			errs = append(errs, errors.New("judge.output_length is for function problems only"))
		}
	case "":
		errs = append(errs, errKindRequired)
	default:
		errs = append(errs, fmt.Errorf("unknown kind %q (function or class)", p.Kind))
	}
	var variants []string
	for _, t := range p.BaseTypes() {
		if IsNodeVariant(t) {
			variants = append(variants, t)
		}
	}
	if len(variants) > 1 {
		errs = append(errs, fmt.Errorf("the signature uses %s: solution code calls each of them Node, so a problem can use only one", strings.Join(variants, " and ")))
	}
	switch p.Judge.Compare {
	case "", CompareExact, CompareAnyOf, CompareChecker:
	case CompareUnordered, CompareUnorderedDeep:
		if t := p.judgedType(); t != "" {
			if _, depth := SplitArray(t); depth == 0 {
				errs = append(errs, fmt.Errorf("judge.compare %s needs an array result, but the judged value is %s", p.Judge.Compare, t))
			}
		}
	default:
		errs = append(errs, fmt.Errorf("unknown judge.compare %q (exact, unordered, unordered_deep, any_of or checker)", p.Judge.Compare))
	}
	switch checker := p.HasChecker(); {
	case p.Judge.Compare == CompareChecker && !checker:
		errs = append(errs, fmt.Errorf("judge.compare checker needs %s in the problem directory", CheckerFile))
	case p.Judge.Compare != CompareChecker && checker:
		errs = append(errs, fmt.Errorf("%s is only used with judge.compare: checker; set it, or remove %s", CheckerFile, CheckerFile))
	}
	if t := p.Judge.Tolerance; t != nil && !(*t >= 0 && !math.IsInf(*t, 1)) {
		errs = append(errs, fmt.Errorf("judge.tolerance %v must be a finite number, 0 or more (0 compares exactly)", *t))
	}
	if tl := p.Judge.Timeout; tl != "" {
		if d, err := time.ParseDuration(tl); err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("judge.timeout %q must be a positive duration such as \"5s\" or \"500ms\"", tl))
		}
	}
	return errors.Join(errs...)
}

// judgedType is the type of the value a function or round-trip problem is
// judged on: the return type, the judge.output input's type for in-place
// problems, or the decoder's return type. A returned node reference is
// judged as an integer (see Function.ReturnsRef).
func (p *Problem) judgedType() string {
	if _, dec, ok := p.RoundTripMethods(); ok {
		return dec.Returns
	}
	if p.Kind != KindFunction || p.Function == nil {
		return ""
	}
	if p.Judge.Output != "" {
		prm, _ := p.Function.Input(p.Judge.Output)
		return prm.Type
	}
	if p.Function.ReturnsRef != "" {
		return TypeInteger
	}
	return p.Function.Returns
}

// validateRoundTrip checks judge.round_trip against the class: decode must
// invert encode, which takes one value and returns a string.
func (p *Problem) validateRoundTrip() []error {
	rt, c := p.Judge.RoundTrip, p.Class
	var errs []error
	method := func(field, name string) (Method, bool) {
		if name == "" {
			errs = append(errs, fmt.Errorf("judge.round_trip.%s is required", field))
			return Method{}, false
		}
		i := slices.IndexFunc(c.Methods, func(m Method) bool { return m.Name == name })
		if i < 0 {
			errs = append(errs, fmt.Errorf("judge.round_trip.%s %q is not a method of %s", field, name, c.Name))
			return Method{}, false
		}
		return c.Methods[i], true
	}
	enc, encOK := method("encode", rt.Encode)
	dec, decOK := method("decode", rt.Decode)
	switch {
	case encOK && decOK && enc.Name == dec.Name:
		errs = append(errs, fmt.Errorf("judge.round_trip: encode and decode must be different methods, not both %s", enc.Name))
	case encOK && len(enc.Params) != 1:
		errs = append(errs, fmt.Errorf("judge.round_trip: encode method %s must take exactly one parameter, the value to encode", enc.Name))
	case encOK && enc.Returns != TypeString:
		errs = append(errs, fmt.Errorf("judge.round_trip: encode method %s must return string, not %s", enc.Name, enc.Returns))
	case decOK && (len(dec.Params) != 1 || dec.Params[0].Type != TypeString):
		errs = append(errs, fmt.Errorf("judge.round_trip: decode method %s must take exactly one string parameter, the encoded value", dec.Name))
	case encOK && decOK && dec.Returns != enc.Params[0].Type:
		errs = append(errs, fmt.Errorf("judge.round_trip: decode method %s must return %s, the type of %s's parameter, not %s", dec.Name, enc.Params[0].Type, enc.Name, dec.Returns))
	}
	if len(c.Constructor) > 0 {
		errs = append(errs, fmt.Errorf("judge.round_trip: the constructor of %s must take no parameters, since cases give only the value to encode", c.Name))
	}
	switch rt.Instances {
	case InstancesSeparate, InstancesSame:
	case "":
		errs = append(errs, errors.New("judge.round_trip.instances is required: separate (encode and decode on two instances) or same (one instance)"))
	default:
		errs = append(errs, fmt.Errorf("unknown judge.round_trip.instances %q (separate or same)", rt.Instances))
	}
	return errs
}

// RoundTripMethods returns the encode and decode methods of a round-trip
// problem. ok is false for other problems, and for a round trip whose
// methods are missing or whose encoder does not take one parameter.
func (p *Problem) RoundTripMethods() (encode, decode Method, ok bool) {
	rt := p.Judge.RoundTrip
	if rt == nil || p.Kind != KindClass || p.Class == nil {
		return Method{}, Method{}, false
	}
	var foundEnc, foundDec bool
	for _, m := range p.Class.Methods {
		switch m.Name {
		case rt.Encode:
			encode, foundEnc = m, true
		case rt.Decode:
			decode, foundDec = m, true
		}
	}
	return encode, decode, foundEnc && foundDec && len(encode.Params) == 1
}

// Ready reports whether the judge rule has been decided, i.e. whether
// outputs can be judged at all. hc fetch leaves it undecided for hc gen (or
// the author) to settle.
func (p *Problem) Ready() bool { return p.Judge.Compare != "" }

// NotReadyHint tells the user how to make an undecided problem judgeable.
func (p *Problem) NotReadyHint() string {
	return fmt.Sprintf("the judge rule of %s is not decided yet: run `hc gen %s`, or set judge.compare in problem.yaml", p.Slug, p.Slug)
}

// EntryName is the name of the function or class the user implements.
func (p *Problem) EntryName() string {
	if p.Kind == KindClass && p.Class != nil {
		return p.Class.Name
	}
	if p.Function != nil {
		return p.Function.Name
	}
	return ""
}

// Load reads a problem from its directory.
func Load(dir string) (*Problem, error) {
	data, err := os.ReadFile(filepath.Join(dir, MetaFile))
	if err != nil {
		return nil, err
	}
	var p Problem
	name := filepath.Join(filepath.Base(dir), MetaFile)
	// Unknown fields are errors: a typo such as "judgee" would otherwise fall
	// back to the defaults without a word.
	if err := yaml.UnmarshalWithOptions(data, &p, yaml.DisallowUnknownField()); err != nil {
		if flowArrayTypeRE.Match(data) {
			return nil, fmt.Errorf("%s: %w\nhint: inside {...} or [...] quote array types, e.g. type: \"integer[]\"", name, err)
		}
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if p.Kind == "" {
		// Everything after loading branches on the kind, so a problem without
		// one is not loaded at all.
		return nil, fmt.Errorf("%s: %w", name, errKindRequired)
	}
	p.Dir = dir
	p.Normalize()
	return &p, nil
}

// Save writes problem.yaml into p.Dir.
func (p *Problem) Save() error {
	data, err := yaml.MarshalWithOptions(p, yaml.IndentSequence(true))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(p.Dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(p.Dir, MetaFile), data, 0o644)
}

// Description returns the Markdown statement.
func (p *Problem) Description() (string, error) {
	data, err := os.ReadFile(filepath.Join(p.Dir, DescriptionFile))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(data), err
}

// Template returns the custom starter code for a solution file name, if any.
func (p *Problem) Template(fileName string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(p.Dir, TemplatesDir, fileName))
	if err != nil {
		return "", false
	}
	return string(data), true
}

// CheckerPath is where the problem's checker lives.
func (p *Problem) CheckerPath() string { return filepath.Join(p.Dir, CheckerFile) }

// HasChecker reports whether the problem directory holds a checker.
func (p *Problem) HasChecker() bool {
	if p.Dir == "" {
		return false
	}
	_, err := os.Stat(p.CheckerPath())
	return err == nil
}

// Reference returns the path of the reference solution with the given file
// name, if it exists.
func (p *Problem) Reference(fileName string) (string, bool) {
	path := filepath.Join(p.Dir, ReferenceDir, fileName)
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	return path, true
}
