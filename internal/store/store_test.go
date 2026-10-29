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

// TestPrefs checks that preferences are kept per pack and for the
// workspace, survive reopening, and go with a purged pack.
func TestPrefs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hc.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, p := range [][3]string{{"", "pack", "leetcode"}, {"leetcode", "locale", "en"}, {"leetcode", "locale", "zh-CN"}, {"local", "locale", "en"}} {
		if err := s.SetPref(ctx, p[0], p[1], p[2]); err != nil {
			t.Fatal(err)
		}
	}
	s.Close()
	if s, err = Open(path); err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	pref := func(pack, key string) string {
		t.Helper()
		v, err := s.Pref(ctx, pack, key)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if got := [3]string{pref("", "pack"), pref("leetcode", "locale"), pref("local", "locale")}; got != [3]string{"leetcode", "zh-CN", "en"} {
		t.Errorf("prefs after reopening: %q", got)
	}
	if pref("", "locale") != "" || pref("regression", "locale") != "" {
		t.Error("an unset preference is not empty")
	}
	if _, err := s.DeletePack(ctx, "leetcode"); err != nil {
		t.Fatal(err)
	}
	if pref("leetcode", "locale") != "" || pref("local", "locale") != "en" || pref("", "pack") != "leetcode" {
		t.Error("DeletePack removed the wrong preferences")
	}
	if err := s.SetPref(ctx, "local", "locale", ""); err != nil || pref("local", "locale") != "" {
		t.Errorf("SetPref to empty did not remove it: %v", err)
	}
}
