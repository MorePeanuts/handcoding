package regression_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/regression"
)

// TestProtocolCoverage requires the regression set
// to exercise the whole problem protocol: every type as an input and as an
// output, every kind, compare mode and round-trip instances value, every
// field of problem.yaml, the default tolerance and the structure inputs on
// each node type that allows them. The lists come from the protocol's code
// wherever they can (the constants of internal/problem, the fields of its
// structs, its validator), so a new protocol feature fails this test until a
// problem of the set uses it. Kept by hand: the constant name prefixes, the
// file that declares the judge-defined types (to tell scalars apart), the
// default tolerance feature and the exemptions. It needs no toolchain: TestRegressionReferences
// (internal/app) checks that the problems' references pass.
func TestProtocolCoverage(t *testing.T) {
	probs := loadSet(t)
	consts := protocolConstants(t)

	used := map[string][]string{} // feature -> slugs of the problems using it
	for _, p := range probs {
		for f := range features(p) {
			used[f] = append(used[f], p.Slug)
		}
	}

	var missing []string
	for _, f := range required(t, consts) {
		if len(used[f]) == 0 {
			missing = append(missing, f)
		} else if n := len(used[f]); n > 3 {
			t.Logf("%s: %s and %d more", f, strings.Join(used[f][:3], ", "), n-3)
		} else {
			t.Logf("%s: %s", f, strings.Join(used[f], ", "))
		}
	}
	if len(missing) > 0 {
		t.Errorf("no problem of the regression set covers:\n  %s\nadd a problem that needs each (see CLAUDE.md, \"Regression set\")", strings.Join(missing, "\n  "))
	}
}

// exempt lists the protocol features no problem of the set has to use, with
// the reason. It is kept by hand.
var exempt = map[string]string{
	typeAs(problem.TypeEmployee, out):       "LeetCode only passes employees in (690); no problem returns one",
	arrayOf(problem.TypeEmployee, out):      "LeetCode only passes employees in (690); no problem returns one",
	arrayOf(problem.TypeNestedInteger, out): "LeetCode returns a NestedInteger (385) but never an array of them",
	field("Judge", "timeout"): "hc-cases says to leave judge.timeout unset: it guards against hangs, " +
		"and a reference must run well within the default",
}

// Directions of a value: decoded from a case, or encoded from a result.
const (
	in  = "an input"
	out = "an output"
)

// Feature names, as the failure lists them.
func typeAs(t, dir string) string    { return "type " + t + " as " + dir }
func scalar(t, dir string) string    { return "scalar " + t + " (not in an array) as " + dir }
func arrayOf(t, dir string) string   { return "array of " + t + " as " + dir }
func depth(d int, dir string) string { return "array of depth " + strconv.Itoa(d) + " as " + dir }
func returns(t string) string        { return "return type " + t }
func field(typ, name string) string  { return "problem.yaml field " + typ + "." + name }
func refTo(t string) string          { return "ref to a " + t }
func returnsRefTo(t string) string   { return "returns_ref into a " + t }

const defaultTolerance = "the default judge.tolerance on a double result"

// constant groups of internal/problem, by name prefix.
const (
	prefixType         = "Type"
	prefixKind         = "Kind"
	prefixCompare      = "Compare"
	prefixInstances    = "Instances"
	prefixOutputLength = "OutputLength"
)

// required lists the features the regression set must cover.
func required(t *testing.T, consts map[string][]constant) []string {
	var req []string
	add := func(f string) {
		if _, ok := exempt[f]; !ok && !slices.Contains(req, f) {
			req = append(req, f)
		}
	}

	// Types, each as an input and as an output; a type valid only as a
	// return type (void) as a return type. The scalars (the types that are
	// neither nodes nor declared with the judge-defined types in
	// judgetypes.go) are required outside arrays, where each has its own
	// encoding. The other types count at any array depth, since LeetCode
	// has some of them (Employee, NestedInteger inputs) only in arrays, and
	// those the protocol allows in arrays are also required in an array,
	// which the drivers encode with codecs of their own (ListNode[]).
	for _, c := range consts[prefixType] {
		isScalar := c.file != "judgetypes.go" && !problem.IsNodeType(c.value)
		switch {
		case problem.ValidateType(c.value, false) == nil:
			for _, dir := range []string{in, out} {
				if isScalar {
					add(scalar(c.value, dir))
					continue
				}
				add(typeAs(c.value, dir))
				if problem.ValidateType(c.value+"[]", false) == nil {
					add(arrayOf(c.value, dir))
				}
			}
		case problem.ValidateType(c.value, true) == nil:
			add(returns(c.value))
		}
	}
	// Every array depth the protocol allows, probed with integer arrays.
	for d := 1; problem.ValidateType(problem.TypeInteger+strings.Repeat("[]", d), false) == nil; d++ {
		add(depth(d, in))
		add(depth(d, out))
		if d > 10 {
			t.Fatal("array depth is unbounded")
		}
	}

	for _, c := range consts[prefixKind] {
		add("kind " + c.value)
	}
	for _, c := range consts[prefixCompare] {
		add("judge.compare " + c.value)
	}
	for _, c := range consts[prefixInstances] {
		add("judge.round_trip.instances " + c.value)
	}
	for _, c := range consts[prefixOutputLength] {
		add("judge.output_length " + c.value)
	}
	add(defaultTolerance)

	// Every field of problem.yaml, which covers the structure inputs
	// (hidden, cycle, join, ref, copy, returns_ref, returns_copy), the judge
	// options and round trips.
	for _, f := range yamlFields(reflect.TypeFor[problem.Problem]()) {
		add(f)
	}

	// ref and returns_ref on each type the validator accepts them for.
	var refs, returnsRefs int
	for _, c := range consts[prefixType] {
		if allowsRef(c.value) {
			add(refTo(c.value))
			refs++
		}
		if allowsReturnsRef(c.value) {
			add(returnsRefTo(c.value))
			returnsRefs++
		}
	}
	if refs == 0 || returnsRefs == 0 {
		t.Fatal("the validator accepts ref or returns_ref on no type: fix the probe problems of allowsRef and allowsReturnsRef")
	}
	return req
}

// allowsRef reports whether the validator accepts a ref parameter of type t.
func allowsRef(t string) bool {
	if problem.ValidateType(t, false) != nil {
		return false
	}
	p := probe(problem.Function{
		Params:  []problem.Param{{Name: "a", Type: t}, {Name: "b", Type: t, Ref: "a"}},
		Returns: problem.TypeInteger,
	})
	return p.Validate() == nil
}

// allowsReturnsRef reports whether the validator accepts returns_ref with
// return type t.
func allowsReturnsRef(t string) bool {
	if problem.ValidateType(t, false) != nil {
		return false
	}
	p := probe(problem.Function{
		Params:     []problem.Param{{Name: "a", Type: t}},
		Returns:    t,
		ReturnsRef: "a",
	})
	return p.Validate() == nil
}

func probe(f problem.Function) *problem.Problem {
	f.Name = "f"
	return &problem.Problem{
		Slug:     "probe",
		Title:    "Probe",
		Kind:     problem.KindFunction,
		Function: &f,
		Judge:    problem.Judge{Compare: problem.CompareExact},
	}
}

// features lists what a problem uses, in the vocabulary of required.
func features(p *problem.Problem) map[string]bool {
	fs := map[string]bool{}
	input := func(prm problem.Param) {
		if prm.Ref != "" || prm.Copy != "" {
			// The case gives a node value, or nothing: not a value of the
			// parameter's type.
			return
		}
		typeFeatures(fs, prm.Type, in)
	}
	output := func(t string) {
		if t != problem.TypeVoid {
			typeFeatures(fs, t, out)
		}
	}

	fs["kind "+string(p.Kind)] = true
	if f := p.Function; f != nil {
		for _, prm := range append(slices.Clip(f.Params), f.Hidden...) {
			input(prm)
			if prm.Ref != "" {
				fs[refTo(prm.Type)] = true
			}
		}
		if f.Returns == problem.TypeVoid {
			fs[returns(problem.TypeVoid)] = true
		}
		switch {
		case p.Judge.Output != "":
			prm, _ := f.Input(p.Judge.Output)
			output(prm.Type)
		case f.ReturnsRef != "":
			// Judged as an index or a value, not encoded as the type.
			fs[returnsRefTo(f.Returns)] = true
		default:
			output(f.Returns)
		}
	}
	if c := p.Class; c != nil {
		for _, prm := range c.Constructor {
			input(prm)
		}
		_, dec, roundTrip := p.RoundTripMethods()
		for _, m := range c.Methods {
			for _, prm := range m.Params {
				input(prm)
			}
			if m.Returns == problem.TypeVoid {
				fs[returns(problem.TypeVoid)] = true
			}
			// A round trip judges only the decoder's result.
			if !roundTrip || m.Name == dec.Name {
				output(m.Returns)
			}
		}
	}

	fs["judge.compare "+p.Judge.Compare] = true
	if rt := p.Judge.RoundTrip; rt != nil {
		fs["judge.round_trip.instances "+rt.Instances] = true
	}
	if ol := p.Judge.OutputLength; ol != "" {
		fs["judge.output_length "+ol] = true
	}
	if p.Judge.Tolerance == nil && p.Tolerance() == problem.DefaultTolerance {
		fs[defaultTolerance] = true
	}
	setFields(fs, reflect.ValueOf(p).Elem())
	return fs
}

// typeFeatures marks the type features of a value of type t in direction
// dir.
func typeFeatures(fs map[string]bool, t, dir string) {
	b, d := problem.SplitArray(t)
	fs[typeAs(b, dir)] = true
	if d == 0 {
		fs[scalar(b, dir)] = true
	} else {
		fs[depth(d, dir)] = true
		fs[arrayOf(b, dir)] = true
	}
}

// yamlName is a struct field's name in problem.yaml, or "" when the field
// is not in it.
func yamlName(sf reflect.StructField) string {
	name, _, _ := strings.Cut(sf.Tag.Get("yaml"), ",")
	if name == "-" || !sf.IsExported() {
		return ""
	}
	if name == "" {
		return strings.ToLower(sf.Name)
	}
	return name
}

// yamlFields lists the fields of every struct type of problem.yaml.
func yamlFields(root reflect.Type) []string {
	var out []string
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type)
	walk = func(typ reflect.Type) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || seen[typ] {
			return
		}
		seen[typ] = true
		for i := range typ.NumField() {
			sf := typ.Field(i)
			if name := yamlName(sf); name != "" {
				out = append(out, field(typ.Name(), name))
				walk(sf.Type)
			}
		}
	}
	walk(root)
	return out
}

// setFields marks the problem.yaml fields that v sets (non-zero values).
func setFields(fs map[string]bool, v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			setFields(fs, v.Elem())
		}
	case reflect.Slice:
		for i := range v.Len() {
			setFields(fs, v.Index(i))
		}
	case reflect.Struct:
		for i := range v.NumField() {
			sf := v.Type().Field(i)
			name := yamlName(sf)
			if name == "" {
				continue
			}
			fv := v.Field(i)
			if !fv.IsZero() && (fv.Kind() != reflect.Slice || fv.Len() > 0) {
				fs[field(v.Type().Name(), name)] = true
			}
			setFields(fs, fv)
		}
	}
}

// loadSet loads the regression set as hc loads an installed pack, and
// requires the pack to be valid.
func loadSet(t *testing.T) []*problem.Problem {
	t.Helper()
	m, err := pack.Load(regression.Dir())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	if m.Name != regression.Name {
		t.Fatalf("pack.yaml names the pack %s, want %s", m.Name, regression.Name)
	}
	probs, failed, err := pack.LoadProblems(m.ProblemsPath(), m.Name)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range failed {
		t.Error(f)
	}
	if err := m.ValidateProblems(probs); err != nil {
		t.Error(err)
	}
	for _, p := range probs {
		for _, err := range pack.IdentityErrors(p, probs) {
			t.Errorf("%s: %v", p.Slug, err)
		}
	}
	if len(probs) == 0 {
		t.Fatal("no problems in the regression set")
	}
	return probs
}

// constant is a string constant of internal/problem.
type constant struct {
	name, value, file string
}

// protocolConstants reads the exported string constants of internal/problem
// from its source, grouped by the prefixes required uses. The package keeps
// some of these lists unexported (its base types), so its declarations are
// the one place that lists them all.
func protocolConstants(t *testing.T) map[string][]constant {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "problem", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	prefixes := []string{prefixType, prefixKind, prefixCompare, prefixInstances, prefixOutputLength}
	out := map[string][]constant{}
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				for _, id := range vs.Names {
					k := slices.IndexFunc(prefixes, func(p string) bool { return strings.HasPrefix(id.Name, p) })
					if k < 0 || !id.IsExported() {
						continue
					}
					var lit *ast.BasicLit
					if len(vs.Names) == 1 && len(vs.Values) == 1 {
						lit, _ = vs.Values[0].(*ast.BasicLit)
					}
					if lit == nil || lit.Kind != token.STRING {
						t.Fatalf("%s: constant %s is not a string literal; teach protocolConstants to read it", fset.Position(id.Pos()), id.Name)
					}
					s, err := strconv.Unquote(lit.Value)
					if err != nil {
						t.Fatal(err)
					}
					out[prefixes[k]] = append(out[prefixes[k]], constant{id.Name, s, filepath.Base(path)})
				}
			}
		}
	}
	for _, p := range prefixes {
		if len(out[p]) == 0 {
			t.Fatalf("found no constant %s* in internal/problem", p)
		}
	}
	return out
}
