// Package judge compares program outputs with expected answers.
//
// Comparison is done on decoded JSON values so that every language runner only
// has to print its answer as JSON; the judging rules live in one place.
// Problems with judge.compare: checker are decided by their checker.py
// instead, whose results Checked interprets.
package judge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

// Options control a comparison.
type Options struct {
	Mode      string
	Tolerance float64
}

// OptionsFor derives comparison options from a problem's judge settings.
func OptionsFor(p *problem.Problem) Options {
	return Options{Mode: p.Judge.Compare, Tolerance: p.Tolerance()}
}

// Equal reports whether got matches expected under opts.
func Equal(opts Options, expected, got json.RawMessage) (bool, error) {
	exp, err := decode(expected)
	if err != nil {
		return false, fmt.Errorf("decode expected: %w", err)
	}
	out, err := decode(got)
	if err != nil {
		return false, fmt.Errorf("decode output: %w", err)
	}
	switch opts.Mode {
	case "", problem.CompareExact:
		return equal(exp, out, opts.Tolerance), nil
	case problem.CompareUnordered:
		return equalUnordered(exp, out, opts.Tolerance, false), nil
	case problem.CompareUnorderedDeep:
		return equalUnordered(exp, out, opts.Tolerance, true), nil
	case problem.CompareAnyOf:
		alts, ok := exp.([]any)
		if !ok {
			return false, fmt.Errorf("any_of: expected must be an array of answers")
		}
		for _, a := range alts {
			if equal(a, out, opts.Tolerance) {
				return true, nil
			}
		}
		return false, nil
	case problem.CompareChecker:
		return false, errors.New("checker problems are judged by their checker (see Checked)")
	default:
		return false, fmt.Errorf("unknown compare mode %q", opts.Mode)
	}
}

func decode(raw json.RawMessage) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func equal(a, b any, tol float64) bool {
	switch x := a.(type) {
	case json.Number:
		y, ok := b.(json.Number)
		return ok && numEqual(x, y, tol)
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !equal(x[i], y[i], tol) {
				return false
			}
		}
		return true
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for k, v := range x {
			w, ok := y[k]
			if !ok || !equal(v, w, tol) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}

func numEqual(a, b json.Number, tol float64) bool {
	if a == b {
		return true
	}
	if ai, err := a.Int64(); err == nil {
		if bi, err := b.Int64(); err == nil {
			return ai == bi
		}
	}
	af, err1 := a.Float64()
	bf, err2 := b.Float64()
	if err1 != nil || err2 != nil {
		return false
	}
	if tol == 0 {
		return af == bf
	}
	diff := math.Abs(af - bf)
	return diff <= tol || diff <= tol*math.Max(math.Abs(af), math.Abs(bf))
}

// equalUnordered compares arrays as multisets. With deep set, nested arrays
// are compared as multisets too.
func equalUnordered(a, b any, tol float64, deep bool) bool {
	x, ok1 := a.([]any)
	y, ok2 := b.([]any)
	if !ok1 || !ok2 {
		return equal(a, b, tol)
	}
	if len(x) != len(y) {
		return false
	}
	if deep {
		x = canonicalize(x)
		y = canonicalize(y)
	}
	if tol == 0 {
		// Fast path: sort canonical encodings.
		return slices.Equal(sortedKeys(x), sortedKeys(y))
	}
	used := make([]bool, len(y))
outer:
	for _, xv := range x {
		for j, yv := range y {
			if !used[j] && equal(xv, yv, tol) {
				used[j] = true
				continue outer
			}
		}
		return false
	}
	return true
}

// canonicalize sorts nested arrays recursively so that order-insensitive
// comparison works at every level.
func canonicalize(v []any) []any {
	out := make([]any, len(v))
	for i, e := range v {
		if arr, ok := e.([]any); ok {
			c := canonicalize(arr)
			slices.SortFunc(c, func(a, b any) int { return strings.Compare(key(a), key(b)) })
			out[i] = c
		} else {
			out[i] = e
		}
	}
	return out
}

func sortedKeys(v []any) []string {
	keys := make([]string, len(v))
	for i, e := range v {
		keys[i] = key(e)
	}
	slices.Sort(keys)
	return keys
}

func key(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
