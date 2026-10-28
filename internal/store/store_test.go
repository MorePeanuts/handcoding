package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

// TestProblems checks that submissions are kept per problem, by pack and
// slug: the same slug in two packs is two problems.
func TestProblems(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hc.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	for _, sub := range []*Submission{
		{Pack: "local", Slug: "two-sum", Lang: "go", Verdict: "Wrong Answer", Code: "a", Details: `{"verdict":"Wrong Answer"}`},
		{Pack: "local", Slug: "two-sum", Lang: "go", Verdict: "Accepted", Code: "b"},
		{Pack: "leetcode", Slug: "two-sum", Lang: "python", Verdict: "Wrong Answer", Code: "c"},
	} {
		if err := s.Add(ctx, sub); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Get(ctx, 1)
	if err != nil || got.Ref() != "local/two-sum" || got.Details != `{"verdict":"Wrong Answer"}` {
		t.Fatalf("Get: %+v, %v", got, err)
	}
	if subs, err := s.List(ctx, "local", "two-sum"); err != nil || len(subs) != 2 || subs[0].Code != "b" {
		t.Fatalf("List local/two-sum: %+v, %v", subs, err)
	}
	if subs, err := s.List(ctx, "", ""); err != nil || len(subs) != 3 {
		t.Fatalf("List all: %d, %v", len(subs), err)
	}
	st, err := s.Statuses(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st["local/two-sum"] != (Status{Attempts: 2, Solved: true}) || st["leetcode/two-sum"] != (Status{Attempts: 1}) {
		t.Errorf("Statuses: %+v", st)
	}
	if n, err := s.DeleteProblem(ctx, "local", "two-sum"); err != nil || n != 2 {
		t.Fatalf("DeleteProblem: %d, %v", n, err)
	}
	if subs, _ := s.List(ctx, "leetcode", "two-sum"); len(subs) != 1 {
		t.Errorf("DeleteProblem removed another pack's submissions")
	}
	// Reopening must work.
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s2.Close()
}

// TestOldSchema checks that a database of an older hc, without the pack
// column, is refused rather than migrated.
func TestOldSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hc.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE submissions (
		id INTEGER PRIMARY KEY AUTOINCREMENT, slug TEXT NOT NULL, lang TEXT NOT NULL,
		verdict TEXT NOT NULL, passed INTEGER NOT NULL, total INTEGER NOT NULL,
		elapsed_ms INTEGER NOT NULL, code TEXT NOT NULL, details TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	if s, err := Open(path); !errors.Is(err, ErrOldSchema) {
		if s != nil {
			s.Close()
		}
		t.Fatalf("Open: %v, want ErrOldSchema", err)
	}
}
