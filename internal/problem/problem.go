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
package problem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
type Param struct {
	Name string `yaml:"name" json:"name"`
	Type string `yaml:"type" json:"type"`
}

// Function describes the signature of a function-kind problem.
type Function struct {
	Name    string  `yaml:"name" json:"name"`
	Params  []Param `yaml:"params" json:"params"`
	Returns string  `yaml:"returns" json:"returns"`
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
)

// Judge holds judging options.
type Judge struct {
	// Compare is one of the Compare* modes. It has no default: an empty value
	// means the judge rule has not been decided yet (see Ready), as for a
	// freshly fetched problem.
	Compare string `yaml:"compare,omitempty" json:"compare,omitempty"`
	// Tolerance is the absolute/relative tolerance for floating point numbers.
	// Defaults to 1e-5 when the return type contains double.
	Tolerance float64 `yaml:"tolerance,omitempty" json:"tolerance,omitempty"`
	// Output names a parameter whose value after the call is the answer, for
	// problems that modify their input in place (e.g. "rotate the array").
	Output string `yaml:"output,omitempty" json:"output,omitempty"`
	// OutputLength limits the judged Output to a prefix. The only value is
	// OutputLengthReturn: the function returns k, and only the first k
	// elements of Output are judged (LeetCode's "remove duplicates" style).
	OutputLength string `yaml:"output_length,omitempty" json:"output_length,omitempty"`
	// TimeLimit is the wall clock limit for a whole run, e.g. "10s".
	TimeLimit string `yaml:"time_limit,omitempty" json:"time_limit,omitempty"`
}

// OutputLengthReturn judges Output[:k], where k is the return value.
const OutputLengthReturn = "return"

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
	if p.Kind == "" {
		if p.Class != nil {
			p.Kind = KindClass
		} else {
			p.Kind = KindFunction
		}
	}
	norm := func(ps []Param) {
		for i := range ps {
			ps[i].Type = NormalizeType(ps[i].Type)
		}
	}
	if f := p.Function; f != nil {
		norm(f.Params)
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
	if p.Judge.Tolerance == 0 && p.usesDouble() {
		p.Judge.Tolerance = 1e-5
	}
}

func (p *Problem) usesDouble() bool {
	if p.Function != nil {
		base, _ := SplitArray(p.Function.Returns)
		return base == TypeDouble
	}
	if p.Class != nil {
		for _, m := range p.Class.Methods {
			if base, _ := SplitArray(m.Returns); base == TypeDouble {
				return true
			}
		}
	}
	return false
}

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
	checkParams := func(where string, ps []Param) {
		seen := map[string]bool{}
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
		checkParams("function", f.Params)
		if err := ValidateType(f.Returns, true); err != nil {
			errs = append(errs, fmt.Errorf("function.returns: %w", err))
		}
		if p.Judge.Output != "" {
			found := false
			for _, prm := range f.Params {
				found = found || prm.Name == p.Judge.Output
			}
			if !found {
				errs = append(errs, fmt.Errorf("judge.output %q is not a parameter", p.Judge.Output))
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
		checkParams("class constructor", c.Constructor)
		if len(c.Methods) == 0 {
			errs = append(errs, errors.New("class.methods must not be empty"))
		}
		for _, m := range c.Methods {
			checkParams("method "+m.Name, m.Params)
			if err := ValidateType(m.Returns, true); err != nil {
				errs = append(errs, fmt.Errorf("method %s returns: %w", m.Name, err))
			}
		}
	default:
		errs = append(errs, fmt.Errorf("unknown kind %q", p.Kind))
	}
	switch p.Judge.Compare {
	case "", CompareExact, CompareAnyOf:
	case CompareUnordered, CompareUnorderedDeep:
		if t := p.judgedType(); p.Kind == KindFunction && t != "" {
			if _, depth := SplitArray(t); depth == 0 {
				errs = append(errs, fmt.Errorf("judge.compare %s needs an array result, but the judged value is %s", p.Judge.Compare, t))
			}
		}
	default:
		errs = append(errs, fmt.Errorf("unknown judge.compare %q (exact, unordered, unordered_deep or any_of)", p.Judge.Compare))
	}
	if tl := p.Judge.TimeLimit; tl != "" {
		if d, err := time.ParseDuration(tl); err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("judge.time_limit %q must be a positive duration such as \"5s\" or \"500ms\"", tl))
		}
	}
	return errors.Join(errs...)
}

// judgedType is the type of the value a function problem is judged on: the
// return type, or the judge.output parameter's type for in-place problems.
func (p *Problem) judgedType() string {
	if p.Function == nil {
		return ""
	}
	if p.Judge.Output != "" {
		for _, prm := range p.Function.Params {
			if prm.Name == p.Judge.Output {
				return prm.Type
			}
		}
		return ""
	}
	return p.Function.Returns
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
	// Unknown fields are errors: a typo such as "judgee" would otherwise fall
	// back to the defaults without a word.
	if err := yaml.UnmarshalWithOptions(data, &p, yaml.DisallowUnknownField()); err != nil {
		name := filepath.Join(filepath.Base(dir), MetaFile)
		if flowArrayTypeRE.Match(data) {
			return nil, fmt.Errorf("%s: %w\nhint: inside {...} or [...] quote array types, e.g. type: \"integer[]\"", name, err)
		}
		return nil, fmt.Errorf("%s: %w", name, err)
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

// Reference returns the path of the reference solution with the given file
// name, if it exists.
func (p *Problem) Reference(fileName string) (string, bool) {
	path := filepath.Join(p.Dir, ReferenceDir, fileName)
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	return path, true
}
