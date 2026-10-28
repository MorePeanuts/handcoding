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
	ID int64
	// Pack and Slug identify the problem.
	Pack      string
	Slug      string
	Lang      string
	Verdict   string
	Passed    int
	Total     int
	ElapsedMs int64
	Code      string
	// Details is the judge report as JSON (see app.EncodeReport).
	Details   string
	CreatedAt time.Time
}

// Ref names the submission's problem as pack/slug.
func (s Submission) Ref() string { return s.Pack + "/" + s.Slug }

// Accepted reports whether the submission passed every case.
func (s Submission) Accepted() bool { return s.Verdict == "Accepted" }

// Store is a handle to the history database.
type Store struct {
	db *sql.DB
}

const table = `
CREATE TABLE IF NOT EXISTS submissions (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	pack       TEXT    NOT NULL,
	slug       TEXT    NOT NULL,
	lang       TEXT    NOT NULL,
	verdict    TEXT    NOT NULL,
	passed     INTEGER NOT NULL,
	total      INTEGER NOT NULL,
	elapsed_ms INTEGER NOT NULL,
	code       TEXT    NOT NULL,
	details    TEXT    NOT NULL,
	created_at INTEGER NOT NULL
)`

const index = `CREATE INDEX IF NOT EXISTS submissions_problem ON submissions(pack, slug, created_at)`

// ErrOldSchema is returned for a database written by an older hc.
var ErrOldSchema = errors.New("the submission history was written by an older hc, which this one does not read; delete the file to start a new history")

// Open opens the database at path, creating it if needed.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	if err := create(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &Store{db: db}, nil
}

// create creates the schema. Before 1.0 hc does not migrate databases: a
// table that lacks a column was created by an older hc, and is refused.
func create(db *sql.DB) error {
	if _, err := db.Exec(table); err != nil {
		return err
	}
	if _, err := db.Exec(`SELECT ` + columns + ` FROM submissions LIMIT 0`); err != nil {
		return fmt.Errorf("%w (%w)", ErrOldSchema, err)
	}
	_, err := db.Exec(index)
	return err
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Add records a submission and sets its ID.
func (s *Store) Add(ctx context.Context, sub *Submission) error {
	if sub.CreatedAt.IsZero() {
		sub.CreatedAt = time.Now()
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO submissions (pack, slug, lang, verdict, passed, total, elapsed_ms, code, details, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sub.Pack, sub.Slug, sub.Lang, sub.Verdict, sub.Passed, sub.Total, sub.ElapsedMs, sub.Code, sub.Details, sub.CreatedAt.UnixMilli())
	if err != nil {
		return err
	}
	sub.ID, err = res.LastInsertId()
	return err
}

// List returns the submissions of the problem pack/slug, newest first. An
// empty pack lists every submission.
func (s *Store) List(ctx context.Context, pack, slug string) ([]Submission, error) {
	q := `SELECT ` + columns + ` FROM submissions`
	var args []any
	if pack != "" {
		q += ` WHERE pack = ? AND slug = ?`
		args = append(args, pack, slug)
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

// DeleteProblem removes every submission of the problem pack/slug and
// returns how many were deleted.
func (s *Store) DeleteProblem(ctx context.Context, pack, slug string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM submissions WHERE pack = ? AND slug = ?`, pack, slug)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeletePack removes every submission of the problems of the pack called
// pack and returns how many were deleted.
func (s *Store) DeletePack(ctx context.Context, pack string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM submissions WHERE pack = ?`, pack)
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

// Statuses returns a summary per problem, keyed by pack/slug (see
// Submission.Ref).
func (s *Store) Statuses(ctx context.Context) (map[string]Status, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT pack, slug, COUNT(*), MAX(verdict = 'Accepted') FROM submissions GROUP BY pack, slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Status{}
	for rows.Next() {
		var (
			sub    Submission
			st     Status
			solved int
		)
		if err := rows.Scan(&sub.Pack, &sub.Slug, &st.Attempts, &solved); err != nil {
			return nil, err
		}
		st.Solved = solved == 1
		out[sub.Ref()] = st
	}
	return out, rows.Err()
}

const columns = `id, pack, slug, lang, verdict, passed, total, elapsed_ms, code, details, created_at`

type scanner interface{ Scan(dest ...any) error }

func scan(r scanner) (Submission, error) {
	var (
		sub Submission
		ts  int64
	)
	err := r.Scan(&sub.ID, &sub.Pack, &sub.Slug, &sub.Lang, &sub.Verdict, &sub.Passed, &sub.Total, &sub.ElapsedMs, &sub.Code, &sub.Details, &ts)
	sub.CreatedAt = time.UnixMilli(ts)
	return sub, err
}
