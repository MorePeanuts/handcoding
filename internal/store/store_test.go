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
	// A submission whose run stopped early has no time.
	ms := int64(42)
	for _, want := range []*int64{nil, &ms} {
		sub := &Submission{Pack: "local", Slug: "timed", Lang: "go", Verdict: "Accepted", ElapsedMs: want}
		if err := s.Add(ctx, sub); err != nil {
			t.Fatal(err)
		}
		got, err := s.Get(ctx, sub.ID)
		if err != nil {
			t.Fatal(err)
		}
		if (got.ElapsedMs == nil) != (want == nil) || want != nil && *got.ElapsedMs != *want {
			t.Errorf("elapsed %v, want %v", got.ElapsedMs, want)
		}
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

// TestStars checks that stars are kept per problem, survive reopening, and
// go with a purged pack but not with a problem's history.
func TestStars(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hc.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, p := range [][2]string{{"local", "two-sum"}, {"leetcode", "two-sum"}, {"leetcode", "lru-cache"}, {"leetcode", "lru-cache"}} {
		if err := s.SetStar(ctx, p[0], p[1], true); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetStar(ctx, "leetcode", "two-sum", false); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if s, err = Open(path); err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	stars := func() map[string]bool {
		t.Helper()
		m, err := s.Stars(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	if got := stars(); len(got) != 2 || !got["local/two-sum"] || !got["leetcode/lru-cache"] {
		t.Errorf("stars after reopening: %v", got)
	}
	if _, err := s.DeleteProblem(ctx, "local", "two-sum"); err != nil || !stars()["local/two-sum"] {
		t.Errorf("DeleteProblem removed the star: %v", err)
	}
	if _, err := s.DeletePack(ctx, "leetcode"); err != nil || stars()["leetcode/lru-cache"] {
		t.Errorf("DeletePack kept the star: %v", err)
	}
}

// TestNotes checks that notes are recorded with a submission or set later,
// changed, removed and deleted with their submissions.
func TestNotes(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "hc.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	note := func(id int64) Note {
		t.Helper()
		sub, err := s.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return sub.Note
	}
	a := &Submission{Pack: "local", Slug: "lru", Lang: "go", Verdict: "Accepted", Note: Note{Comment: "doubly linked list", Notes: "O(1) get and put"}}
	b := &Submission{Pack: "local", Slug: "lru", Lang: "go", Verdict: "Accepted"}
	for _, sub := range []*Submission{a, b} {
		if err := s.Add(ctx, sub); err != nil {
			t.Fatal(err)
		}
	}
	if got := note(a.ID); got != a.Note {
		t.Errorf("note of a: %+v, want %+v", got, a.Note)
	}
	if got := note(b.ID); !got.Empty() {
		t.Errorf("note of b: %+v, want none", got)
	}
	queue := Note{Comment: "queue"}
	if err := s.SetNote(ctx, b.ID, queue); err != nil {
		t.Fatal(err)
	}
	if subs, err := s.List(ctx, "local", "lru"); err != nil || subs[0].Note != queue || subs[1].Note != a.Note {
		t.Fatalf("List: %+v, %v", subs, err)
	}
	if err := s.SetNote(ctx, a.ID, Note{}); err != nil || !note(a.ID).Empty() {
		t.Errorf("removing a's note: %v, %+v", err, note(a.ID))
	}
	if err := s.SetNote(ctx, a.ID, Note{Notes: "orphan"}); !errors.Is(err, ErrNoComment) {
		t.Errorf("notes without a comment: %v", err)
	}
	if err := s.SetNote(ctx, 99, queue); !errors.Is(err, ErrNotFound) {
		t.Errorf("SetNote on a missing submission: %v", err)
	}
	for _, del := range []func() error{
		func() error { return s.Delete(ctx, b.ID) },
		func() error { _, err := s.DeleteProblem(ctx, "local", "lru"); return err },
	} {
		if err := del(); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM submission_notes`).Scan(&n); err != nil || n != 0 {
		t.Errorf("notes left after deleting their submissions: %d, %v", n, err)
	}
}

func TestNewNote(t *testing.T) {
	for _, tc := range []struct {
		comment, notes string
		want           Note
		err            bool
	}{
		{" queue \n", "\n  O(n)\n", Note{Comment: "queue", Notes: "O(n)"}, false},
		{"", " ", Note{}, false},
		{"", "orphan", Note{}, true},
		{"two\nlines", "", Note{}, true},
	} {
		got, err := NewNote(tc.comment, tc.notes)
		if got != tc.want || (err != nil) != tc.err {
			t.Errorf("NewNote(%q, %q) = %+v, %v", tc.comment, tc.notes, got, err)
		}
	}
}

// Adding notes must leave a history written before their table existed
// readable, including its code and judge details.
func TestNotesExistingHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hc.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(table); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO submissions
		(pack, slug, lang, verdict, passed, total, elapsed_ms, code, details, created_at)
		VALUES ('local', 'two-sum', 'go', 'Accepted', 3, 3, 42, 'original code', 'original report', 123)`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	sub, err := s.Get(ctx, 1)
	if err != nil || !sub.Note.Empty() || sub.Code != "original code" || sub.Details != "original report" || sub.ElapsedMs == nil || *sub.ElapsedMs != 42 {
		t.Fatalf("existing history: %+v, %v", sub, err)
	}
	if err := s.SetNote(ctx, 1, Note{Comment: "annotated later"}); err != nil {
		t.Fatal(err)
	}
	if sub, err := s.Get(ctx, 1); err != nil || sub.Note.Comment != "annotated later" || sub.Code != "original code" {
		t.Fatalf("annotated history: %+v, %v", sub, err)
	}
}

func TestDeleteNotes(t *testing.T) {
	for _, scope := range []string{"submission", "problem", "pack"} {
		t.Run(scope, func(t *testing.T) {
			s, err := Open(filepath.Join(t.TempDir(), "hc.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			ctx := context.Background()
			for _, sub := range []*Submission{
				{Pack: "local", Slug: "two-sum", Note: Note{Comment: "a"}},
				{Pack: "local", Slug: "two-sum", Note: Note{Comment: "b"}},
				{Pack: "local", Slug: "other", Note: Note{Comment: "c"}},
				{Pack: "installed", Slug: "two-sum", Note: Note{Comment: "keep"}},
			} {
				if err := s.Add(ctx, sub); err != nil {
					t.Fatal(err)
				}
			}
			want := 0
			switch scope {
			case "submission":
				err, want = s.Delete(ctx, 1), 3
			case "problem":
				_, err = s.DeleteProblem(ctx, "local", "two-sum")
				want = 2
			case "pack":
				_, err = s.DeletePack(ctx, "local")
				want = 1
			}
			if err != nil {
				t.Fatal(err)
			}
			var n int
			if err := s.db.QueryRow(`SELECT COUNT(*) FROM submission_notes`).Scan(&n); err != nil || n != want {
				t.Fatalf("remaining notes: %d, %v; want %d", n, err, want)
			}
			if sub, err := s.Get(ctx, 4); err != nil || sub.Note.Comment != "keep" {
				t.Fatalf("deleted another pack's note: %+v, %v", sub, err)
			}
		})
	}
}
