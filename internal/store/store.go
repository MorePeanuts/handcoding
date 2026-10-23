// Package store persists submission history in SQLite.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // pure Go SQLite driver
)

// Submission is a recorded `hc submit`.
type Submission struct {
	ID        int64
	Slug      string
	Lang      string
	Verdict   string
	Passed    int
	Total     int
	ElapsedMs int64
	Code      string
	// Details is the judge report as JSON (see app.EncodeReport); empty for
	// submissions recorded by older versions.
	Details   string
	CreatedAt time.Time
}

// Accepted reports whether the submission passed every case.
func (s Submission) Accepted() bool { return s.Verdict == "Accepted" }

// Store is a handle to the history database.
type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS submissions (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	slug       TEXT    NOT NULL,
	lang       TEXT    NOT NULL,
	verdict    TEXT    NOT NULL,
	passed     INTEGER NOT NULL,
	total      INTEGER NOT NULL,
	elapsed_ms INTEGER NOT NULL,
	code       TEXT    NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS submissions_slug ON submissions(slug, created_at);
`

// Open opens (and migrates) the database at path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate %s: %w", path, err)
	}
	return &Store{db: db}, nil
}

// migrate creates the schema and adds columns introduced after the first
// release, so databases created by older versions keep working.
func migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	rows, err := db.Query(`SELECT name FROM pragma_table_info('submissions')`)
	if err != nil {
		return err
	}
	cols := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		cols[name] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if !cols["details"] {
		if _, err := db.Exec(`ALTER TABLE submissions ADD COLUMN details TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Add records a submission and sets its ID.
func (s *Store) Add(ctx context.Context, sub *Submission) error {
	if sub.CreatedAt.IsZero() {
		sub.CreatedAt = time.Now()
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO submissions (slug, lang, verdict, passed, total, elapsed_ms, code, details, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sub.Slug, sub.Lang, sub.Verdict, sub.Passed, sub.Total, sub.ElapsedMs, sub.Code, sub.Details, sub.CreatedAt.UnixMilli())
	if err != nil {
		return err
	}
	sub.ID, err = res.LastInsertId()
	return err
}

// List returns the submissions of a problem, newest first. An empty slug
// lists every submission.
func (s *Store) List(ctx context.Context, slug string) ([]Submission, error) {
	q := `SELECT ` + columns + ` FROM submissions`
	var args []any
	if slug != "" {
		q += ` WHERE slug = ?`
		args = append(args, slug)
	}
	q += ` ORDER BY created_at DESC, id DESC`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subs []Submission
	for rows.Next() {
		sub, err := scan(rows)
		if err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

// ErrNotFound is returned for unknown submission IDs.
var ErrNotFound = errors.New("submission not found")

// Get returns a submission by ID.
func (s *Store) Get(ctx context.Context, id int64) (Submission, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+columns+` FROM submissions WHERE id = ?`, id)
	sub, err := scan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return sub, fmt.Errorf("%w: #%d", ErrNotFound, id)
	}
	return sub, err
}

// Delete removes a submission.
func (s *Store) Delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM submissions WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: #%d", ErrNotFound, id)
	}
	return nil
}

// DeleteSlug removes every submission of a problem and returns how many
// were deleted.
func (s *Store) DeleteSlug(ctx context.Context, slug string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM submissions WHERE slug = ?`, slug)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Status summarizes the best result per problem.
type Status struct {
	Attempts int
	Solved   bool
}

// Statuses returns a summary per slug.
func (s *Store) Statuses(ctx context.Context) (map[string]Status, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT slug, COUNT(*), MAX(verdict = 'Accepted') FROM submissions GROUP BY slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Status{}
	for rows.Next() {
		var (
			slug   string
			st     Status
			solved int
		)
		if err := rows.Scan(&slug, &st.Attempts, &solved); err != nil {
			return nil, err
		}
		st.Solved = solved == 1
		out[slug] = st
	}
	return out, rows.Err()
}

const columns = `id, slug, lang, verdict, passed, total, elapsed_ms, code, details, created_at`

type scanner interface{ Scan(dest ...any) error }

func scan(r scanner) (Submission, error) {
	var (
		sub Submission
		ts  int64
	)
	err := r.Scan(&sub.ID, &sub.Slug, &sub.Lang, &sub.Verdict, &sub.Passed, &sub.Total, &sub.ElapsedMs, &sub.Code, &sub.Details, &ts)
	sub.CreatedAt = time.UnixMilli(ts)
	return sub, err
}
