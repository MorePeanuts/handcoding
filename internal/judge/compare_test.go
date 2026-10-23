package judge

import (
	"encoding/json"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

func TestEqual(t *testing.T) {
	tests := []struct {
		name          string
		mode          string
		tol           float64
		expected, got string
		want          bool
	}{
		{"exact ints", problem.CompareExact, 0, `[1,2]`, `[1,2]`, true},
		{"exact order matters", problem.CompareExact, 0, `[1,2]`, `[2,1]`, false},
		{"int vs float literal", problem.CompareExact, 0, `2`, `2.0`, true},
		{"large ints keep precision", problem.CompareExact, 0, `9007199254740993`, `9007199254740992`, false},
		{"float tolerance", problem.CompareExact, 1e-5, `2.5`, `2.500001`, true},
		{"float outside tolerance", problem.CompareExact, 1e-5, `2.5`, `2.6`, false},
		{"null vs empty", problem.CompareExact, 0, `[]`, `null`, false},
		{"strings", problem.CompareExact, 0, `"abc"`, `"abc"`, true},
		{"unordered outer", problem.CompareUnordered, 0, `[[1,2],[3]]`, `[[3],[1,2]]`, true},
		{"unordered keeps inner order", problem.CompareUnordered, 0, `[[1,2],[3]]`, `[[3],[2,1]]`, false},
		{"unordered multiset", problem.CompareUnordered, 0, `[1,1,2]`, `[1,2,2]`, false},
		{"unordered deep", problem.CompareUnorderedDeep, 0, `[["eat","tea"],["bat"]]`, `[["bat"],["tea","eat"]]`, true},
		{"unordered with tolerance", problem.CompareUnordered, 1e-5, `[1.0,2.0]`, `[2.000001,1.0]`, true},
		{"any of", problem.CompareAnyOf, 0, `[[0,1],[1,0]]`, `[1,0]`, true},
		{"any of miss", problem.CompareAnyOf, 0, `[[0,1],[1,0]]`, `[1,1]`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Equal(Options{Mode: tt.mode, Tolerance: tt.tol}, json.RawMessage(tt.expected), json.RawMessage(tt.got))
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("Equal(%s, %s) = %v, want %v", tt.expected, tt.got, got, tt.want)
			}
		})
	}
}
