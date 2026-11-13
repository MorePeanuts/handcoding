package tui

import (
	"cmp"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

const searchExamples = "id:707 | id:70* | id:700..800"

// parseSearch compiles a query once per edit. An incomplete or invalid ID
// query returns a hint instead of a matcher, so it cannot select a problem.
func parseSearch(q string) (func(*problem.Problem) bool, string) {
	q = strings.TrimSpace(q)
	if len(q) < 3 || !strings.EqualFold(q[:3], "id:") {
		q = strings.ToLower(q)
		return func(p *problem.Problem) bool { return q == "" || matches(p, q) }, ""
	}
	id := strings.TrimSpace(q[3:])
	if id == "" {
		return nil, "enter an ID: " + searchExamples
	}
	if lo, hi, ok := strings.Cut(id, ".."); ok {
		lo, hi = strings.TrimSpace(lo), strings.TrimSpace(hi)
		if lo == "" {
			return nil, "enter the range's lower bound"
		}
		if hi == "" {
			return nil, "enter the range's upper bound"
		}
		lo, loOK := decimalID(lo)
		hi, hiOK := decimalID(hi)
		if !loOK || !hiOK {
			return nil, "range bounds must be non-negative decimal integers: id:700..800"
		}
		if compareIDs(lo, hi) > 0 {
			return nil, "range lower bound must not exceed upper bound"
		}
		return func(p *problem.Problem) bool {
			n, ok := decimalID(p.ID)
			return ok && compareIDs(lo, n) <= 0 && compareIDs(n, hi) <= 0
		}, ""
	}
	if strings.Contains(id, "*") {
		prefix, ok := strings.CutSuffix(id, "*")
		if !ok || strings.Contains(prefix, "*") {
			return nil, "use a single trailing * for an ID prefix: id:70*"
		}
		if prefix == "" {
			return nil, "enter an ID prefix before *: id:70*"
		}
		return func(p *problem.Problem) bool { return strings.HasPrefix(p.ID, prefix) }, ""
	}
	if prefix, ok := strings.CutSuffix(id, "."); ok {
		if _, numeric := decimalID(prefix); numeric {
			return nil, "complete the range with '..' and an upper bound: id:700..800"
		}
	}
	return func(p *problem.Problem) bool { return p.ID == id }, ""
}

// decimalID normalizes digit-only IDs for numeric comparison without a
// machine-integer size limit. Exact and prefix searches still use their text.
func decimalID(s string) (string, bool) {
	if s == "" {
		return "", false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return "", false
		}
	}
	s = strings.TrimLeft(s, "0")
	if s == "" {
		s = "0"
	}
	return s, true
}

func compareIDs(a, b string) int {
	if n := cmp.Compare(len(a), len(b)); n != 0 {
		return n
	}
	return strings.Compare(a, b)
}
