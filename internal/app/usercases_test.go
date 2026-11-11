package app_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
)

// TestUserCases checks the user's own cases of a problem of an installed
// pack: kept in the workspace, numbered after the pack's, their expected
// output computed by the reference unless given, run by default by a test
// and never by a submission, and removable only among themselves.
func TestUserCases(t *testing.T) {
	t.Parallel()
	requireTools(t, "go", "python3")
	a := newWorkspace(t)
	ctx := context.Background()
	p, err := a.WS.Problem("two-sum")
	if err != nil {
		t.Fatal(err)
	}
	if !a.KeepsUserCases(p) {
		t.Fatal("a problem of an installed pack keeps no cases of the user's")
	}
	pack, err := p.LoadCases()
	if err != nil {
		t.Fatal(err)
	}
	n := len(pack)
	userCase := func(in string, expected string) problem.Case {
		c, err := problem.ParseCase([]byte(`{"input":` + in + `}`))
		if err != nil {
			t.Fatal(err)
		}
		if expected != "" {
			c.Expected = []byte(expected)
		}
		return c
	}
	// Without an expected output, the reference computes it.
	added, err := a.AddCase(ctx, p, userCase(`{"nums":[1,5,3],"target":8}`, ""), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !added.User || added.Index != n+1 || string(added.Expected) != "[1,2]" || added.Ref == nil {
		t.Fatalf("added %+v, want your case %d with [1,2] from a reference", added, n+1)
	}
	// Given, it is kept; with a note.
	c := userCase(`{"nums":[4,4],"target":8}`, "[0,1]")
	c.Note = "twins"
	if added, err = a.AddCase(ctx, p, c, nil); err != nil || added.Index != n+2 || added.Expected != nil {
		t.Fatalf("added %+v, %v", added, err)
	}
	data, err := os.ReadFile(a.WS.UserCasesPath(p))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"input":{"nums":[1,5,3],"target":8}}` + "\n" + `{"input":{"nums":[4,4],"target":8},"expected":[0,1],"note":"twins"}` + "\n"
	if string(data) != want {
		t.Fatalf("%s:\n%s\nwant\n%s", a.WS.Rel(a.WS.UserCasesPath(p)), data, want)
	}
	// The pack is untouched.
	if again, _ := p.LoadCases(); len(again) != n {
		t.Fatalf("the pack has %d cases, want %d", len(again), n)
	}

	// What the user's own cases cannot be.
	gen := problem.Case{Generate: &problem.Generate{Name: "distinct"}}
	ex := userCase(`{"nums":[1,2],"target":3}`, "")
	ex.Example = true
	bad := userCase(`{"nums":[1,2]}`, "")
	for name, c := range map[string]problem.Case{"generated": gen, "example": ex, "invalid": bad} {
		if _, err := a.AddCase(ctx, p, c, nil); err == nil {
			t.Errorf("%s case added", name)
		}
	}

	cases, err := a.Cases(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != n+2 || !cases[n].User || !cases[n+1].User || cases[n-1].User {
		t.Fatalf("cases: %d, want the pack's %d and 2 of yours", len(cases), n)
	}
	sel, _ := app.Select(cases, nil, false)
	if last := sel[len(sel)-1]; last.Index != n+2 || !sel[len(sel)-2].Case.User {
		t.Fatalf("the default selection ends with case %d, want your cases", last.Index)
	}

	// A test runs them, judged on the reference's output, and is
	// remembered for debugging; a submission does not run them.
	l := writeSolution(t, a, p, "python", wrongTwoSum)
	rep, err := a.Run(ctx, p, l, sel, app.RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[int]runner.Verdict{}
	for _, c := range rep.Cases {
		got[c.Index] = c.Verdict
	}
	if got[n+1] != runner.WrongAnswer || got[n+2] != runner.Accepted {
		t.Fatalf("your cases: %v, want Wrong Answer then Accepted", got)
	}
	all, _ := app.Select(cases, nil, true)
	if _, err := a.Run(ctx, p, l, all, app.RunOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.LastFailure(p, l); !ok {
		t.Fatal("a run of every case is not remembered")
	}
	ref, _ := p.Reference("solution.py")
	right, err := os.ReadFile(ref)
	if err != nil {
		t.Fatal(err)
	}
	writeSolution(t, a, p, "python", string(right))
	rep, _, err = a.Submit(ctx, p, l, app.RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != n || rep.Verdict != runner.Accepted {
		t.Fatalf("submission: %s on %d cases, want Accepted on the pack's %d", rep.Verdict, rep.Total, n)
	}
	// It did not run every case, so it says nothing of the first failure.
	if d, ok := a.LastFailure(p, l); ok {
		t.Fatalf("after a submission: %+v, but your cases did not run", d)
	}

	// Only the user's own cases can be removed.
	if _, err := a.RemoveCases(p, []int{1}); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("removing a case of the pack: %v", err)
	}
	r, err := a.RemoveCases(p, []int{n + 1})
	if err != nil || r.Removed != 1 || r.Left != 1 || !r.User {
		t.Fatalf("removed %+v, %v", r, err)
	}
	if cases, _ := a.Cases(p); len(cases) != n+1 || cases[n].Note != "twins" {
		t.Fatalf("after removing case %d: %d cases", n+1, len(cases))
	}
	if _, err := a.RemoveCases(p, []int{n + 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.WS.UserCasesPath(p)); !os.IsNotExist(err) {
		t.Fatalf("the file of no cases is left: %v", err)
	}
}

// TestUserCaseReferenceFails checks that an input the reference fails on
// is refused, unless its expected output is given.
func TestUserCaseReferenceFails(t *testing.T) {
	t.Parallel()
	requireTools(t, "python3")
	a := newWorkspace(t)
	ctx := context.Background()
	p, err := a.WS.Problem("two-sum")
	if err != nil {
		t.Fatal(err)
	}
	// A reference that crashes on every input, the only one.
	if goRef, ok := p.Reference("solution.go"); ok {
		if err := os.Remove(goRef); err != nil {
			t.Fatal(err)
		}
	}
	ref, _ := p.Reference("solution.py")
	if err := os.WriteFile(ref, []byte("class Solution:\n    def twoSum(self, nums, target):\n        raise ValueError('no')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := problem.ParseCase([]byte(`{"input":{"nums":[1,2],"target":3}}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddCase(ctx, p, c, nil); err == nil || !strings.Contains(err.Error(), "reference solution fails") {
		t.Fatalf("added on a failing reference: %v", err)
	}
	c.Expected = []byte("[0,1]")
	if _, err := a.AddCase(ctx, p, c, nil); err != nil {
		t.Fatal(err)
	}
}
