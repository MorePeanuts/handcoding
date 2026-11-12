// Package store persists submission history, the notes on submissions,
// starred problems and the preferences hc remembers across runs, in SQLite.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure Go SQLite driver
)

// Submission is a recorded `hc submit`.
type Submission struct {
	ID int64
	// Pack and Slug identify the problem.
	Pack    string
	Slug    string
	Lang    string
	Verdict string
	Passed  int
	Total   int
	// ElapsedMs is the total time of the run, nil when it stopped early (see
	// runner.Report.Stopped).
	ElapsedMs *int64
	Code      string
	// Details is the judge report as JSON (see app.EncodeReport).
	Details   string
	CreatedAt time.Time
	// Note is what the user wrote about the submission, empty without a
	// note.
	Note Note
}

// Ref names the submission's problem as pack/slug.
func (s Submission) Ref() string { return s.Pack + "/" + s.Slug }

// Accepted reports whether the submission passed every case.
func (s Submission) Accepted() bool { return s.Verdict == "Accepted" }

// Note is what the user wrote about a submission, like a commit's summary
// and description: a comment of one line, and optional notes of any length.
// A note without a comment is empty (see NewNote).
type Note struct {
	Comment string
	Notes   string
}

// Empty reports whether n is no note.
func (n Note) Empty() bool { return n.Comment == "" }

// ErrNoComment is returned for notes without a comment.
var ErrNoComment = errors.New("a note needs a comment, which the notes only add to")

// NewNote is the note of comment and notes, trimmed. Both empty make the
// empty note; notes without a comment, or a comment of several lines, are
// an error.
func NewNote(comment, notes string) (Note, error) {
	n := Note{Comment: strings.TrimSpace(comment), Notes: strings.TrimSpace(notes)}
	switch {
	case n.Comment == "" && n.Notes != "":
		return Note{}, ErrNoComment
	case strings.ContainsAny(n.Comment, "\r\n"):
		return Note{}, errors.New("the comment is one line; put the rest in the notes")
	}
	return n, nil
}

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

// prefs holds what hc remembers between runs, such as the pack the TUI
// showed last: per pack, or for the workspace when pack is "".
const prefs = `
CREATE TABLE IF NOT EXISTS prefs (
	pack  TEXT NOT NULL,
	key   TEXT NOT NULL,
	value TEXT NOT NULL,
	PRIMARY KEY (pack, key)
)`

// stars holds the problems the user starred, by pack and slug.
const stars = `
CREATE TABLE IF NOT EXISTS stars (
	pack TEXT NOT NULL,
	slug TEXT NOT NULL,
	PRIMARY KEY (pack, slug)
)`

// notes holds the notes on submissions, by submission id. A table of its
// own, rather than columns of submissions, keeps the histories written
// before notes existed readable, and a note can change while a submission
// does not. Submission ids are never reused (AUTOINCREMENT), so a note
// left behind could never be taken for another submission's; the deletions
// still delete theirs.
const notes = `
CREATE TABLE IF NOT EXISTS submission_notes (
	id      INTEGER PRIMARY KEY,
	comment TEXT NOT NULL,
	notes   TEXT NOT NULL
)`

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
	if _, err := db.Exec(index); err != nil {
		return err
	}
	for _, t := range []string{prefs, stars, notes} {
		if _, err := db.Exec(t); err != nil {
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
	// elapsed_ms is -1 for no time, which keeps the column NOT NULL as in
	// the databases already written.
	elapsed := int64(-1)
	if sub.ElapsedMs != nil {
		elapsed = *sub.ElapsedMs
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO submissions (pack, slug, lang, verdict, passed, total, elapsed_ms, code, details, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sub.Pack, sub.Slug, sub.Lang, sub.Verdict, sub.Passed, sub.Total, elapsed, sub.Code, sub.Details, sub.CreatedAt.UnixMilli())
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	if err := setNote(ctx, tx, id, sub.Note); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	sub.ID = id
	return nil
}

// SetNote sets the note on submission id, or removes it when n is empty.
func (s *Store) SetNote(ctx context.Context, id int64, n Note) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var one int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM submissions WHERE id = ?`, id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: #%d", ErrNotFound, id)
	}
	if err != nil {
		return err
	}
	if err := setNote(ctx, tx, id, n); err != nil {
		return err
	}
	return tx.Commit()
}

func setNote(ctx context.Context, tx *sql.Tx, id int64, n Note) error {
	if n.Empty() {
		if n.Notes != "" {
			return ErrNoComment
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM submission_notes WHERE id = ?`, id)
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO submission_notes (id, comment, notes) VALUES (?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET comment = excluded.comment, notes = excluded.notes`, id, n.Comment, n.Notes)
	return err
}

// List returns the submissions of the problem pack/slug, newest first. An
// empty pack lists every submission.
func (s *Store) List(ctx context.Context, pack, slug string) ([]Submission, error) {
	q := selectSubmissions
	var args []any
	if pack != "" {
		q += ` WHERE s.pack = ? AND s.slug = ?`
		args = append(args, pack, slug)
	}
	q += ` ORDER BY s.created_at DESC, s.id DESC`
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
	row := s.db.QueryRowContext(ctx, selectSubmissions+` WHERE s.id = ?`, id)
	sub, err := scan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return sub, fmt.Errorf("%w: #%d", ErrNotFound, id)
	}
	return sub, err
}

// Delete removes a submission and its note.
func (s *Store) Delete(ctx context.Context, id int64) error {
	n, err := s.deleteSubmissions(ctx, `id = ?`, id)
	if err == nil && n == 0 {
		return fmt.Errorf("%w: #%d", ErrNotFound, id)
	}
	return err
}

// DeleteProblem removes every submission of the problem pack/slug, with
// their notes, and returns how many were deleted.
func (s *Store) DeleteProblem(ctx context.Context, pack, slug string) (int64, error) {
	return s.deleteSubmissions(ctx, `pack = ? AND slug = ?`, pack, slug)
}

// deleteSubmissions removes the submissions where cond holds, with their
// notes, and returns how many were deleted.
func (s *Store) deleteSubmissions(ctx context.Context, cond string, args ...any) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM submission_notes WHERE id IN (SELECT id FROM submissions WHERE `+cond+`)`, args...); err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM submissions WHERE `+cond, args...)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, tx.Commit()
}

// DeletePack removes every submission of the problems of the pack called
// pack, with their notes, its preferences and its stars, and returns how
// many submissions were deleted.
func (s *Store) DeletePack(ctx context.Context, pack string) (int64, error) {
	if pack == "" {
		return 0, errors.New("no pack named")
	}
	n, err := s.deleteSubmissions(ctx, `pack = ?`, pack)
	if err != nil {
		return 0, err
	}
	for _, table := range []string{"prefs", "stars"} {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM `+table+` WHERE pack = ?`, pack); err != nil {
			return 0, err
		}
	}
	return n, nil
}

// SetStar stars the problem pack/slug, or unstars it.
func (s *Store) SetStar(ctx context.Context, pack, slug string, starred bool) error {
	q := `DELETE FROM stars WHERE pack = ? AND slug = ?`
	if starred {
		q = `INSERT OR IGNORE INTO stars (pack, slug) VALUES (?, ?)`
	}
	_, err := s.db.ExecContext(ctx, q, pack, slug)
	return err
}

// Stars returns the starred problems, keyed by pack/slug (see
// Submission.Ref).
func (s *Store) Stars(ctx context.Context) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT pack, slug FROM stars`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var sub Submission
		if err := rows.Scan(&sub.Pack, &sub.Slug); err != nil {
			return nil, err
		}
		out[sub.Ref()] = true
	}
	return out, rows.Err()
}

// Pref returns the preference key of the pack, or of the workspace when
// pack is "". It is "" when unset.
func (s *Store) Pref(ctx context.Context, pack, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM prefs WHERE pack = ? AND key = ?`, pack, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

// SetPref sets the preference key of the pack, or of the workspace when pack
// is ""; an empty value removes it.
func (s *Store) SetPref(ctx context.Context, pack, key, value string) error {
	if value == "" {
		_, err := s.db.ExecContext(ctx, `DELETE FROM prefs WHERE pack = ? AND key = ?`, pack, key)
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO prefs (pack, key, value) VALUES (?, ?, ?)
		ON CONFLICT (pack, key) DO UPDATE SET value = excluded.value`, pack, key, value)
	return err
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

// columns are the columns of submissions, which create requires.
const columns = `id, pack, slug, lang, verdict, passed, total, elapsed_ms, code, details, created_at`

// selectSubmissions selects submissions (as s) with their notes, for scan.
const selectSubmissions = `SELECT s.id, s.pack, s.slug, s.lang, s.verdict, s.passed, s.total, s.elapsed_ms, s.code, s.details, s.created_at,
	COALESCE(n.comment, ''), COALESCE(n.notes, '')
	FROM submissions s LEFT JOIN submission_notes n ON n.id = s.id`

type scanner interface{ Scan(dest ...any) error }

func scan(r scanner) (Submission, error) {
	var (
		sub     Submission
		elapsed int64
		ts      int64
	)
	err := r.Scan(&sub.ID, &sub.Pack, &sub.Slug, &sub.Lang, &sub.Verdict, &sub.Passed, &sub.Total, &elapsed, &sub.Code, &sub.Details, &ts,
		&sub.Note.Comment, &sub.Note.Notes)
	if elapsed >= 0 {
		sub.ElapsedMs = &elapsed
	}
	sub.CreatedAt = time.UnixMilli(ts)
	return sub, err
}
