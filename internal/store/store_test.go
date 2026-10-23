package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// TestMigrateOldSchema opens a database created by the first release (no
// details column) and checks that it is upgraded in place.
func TestMigrateOldSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hc.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE submissions (
		id INTEGER PRIMARY KEY AUTOINCREMENT, slug TEXT NOT NULL, lang TEXT NOT NULL,
		verdict TEXT NOT NULL, passed INTEGER NOT NULL, total INTEGER NOT NULL,
		elapsed_ms INTEGER NOT NULL, code TEXT NOT NULL, created_at INTEGER NOT NULL);
		INSERT INTO submissions (slug, lang, verdict, passed, total, elapsed_ms, code, created_at)
		VALUES ('two-sum', 'go', 'Accepted', 3, 3, 1, 'code', 0);`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	old, err := s.Get(ctx, 1)
	if err != nil || old.Details != "" || old.Code != "code" {
		t.Fatalf("old row: %+v, %v", old, err)
	}
	sub := &Submission{Slug: "two-sum", Lang: "go", Verdict: "Wrong Answer", Code: "x", Details: `{"verdict":"Wrong Answer"}`}
	if err := s.Add(ctx, sub); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, sub.ID)
	if err != nil || got.Details != sub.Details {
		t.Fatalf("details round trip: %+v, %v", got, err)
	}
	n, err := s.DeleteSlug(ctx, "two-sum")
	if err != nil || n != 2 {
		t.Fatalf("DeleteSlug: %d, %v", n, err)
	}
	// Reopening an already migrated database must be a no-op.
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s2.Close()
}
