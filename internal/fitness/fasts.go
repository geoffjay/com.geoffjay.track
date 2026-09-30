package fitness

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"
)

// Fast is one fasting window. ended_at NULL means the fast is active; the
// store guarantees at most one active fast per user. DurationHours is the
// computed elapsed time in hours ((ended_at or now) - started_at, rounded
// to 2 decimals), populated on read.
type Fast struct {
	ID            int64      `json:"id"`
	UserID        int64      `json:"-"`
	StartedAt     time.Time  `json:"started_at"`
	EndedAt       *time.Time `json:"ended_at,omitempty"`
	TargetHours   *float64   `json:"target_hours,omitempty"`
	Notes         string     `json:"notes"`
	CreatedAt     time.Time  `json:"created_at"`
	DurationHours *float64   `json:"duration_hours,omitempty"`
}

// FastStore provides fast persistence.
type FastStore struct{ db *sql.DB }

// NewFastStore wraps db.
func NewFastStore(db *sql.DB) *FastStore { return &FastStore{db: db} }

const fastCols = `id, user_id, started_at, ended_at, target_hours, notes, created_at`

// scanFast reads one fast row and populates the computed duration.
func scanFast(scan interface{ Scan(...any) error }) (Fast, error) {
	var f Fast
	var started, ended, createdAt sql.NullString
	var target sql.NullFloat64
	if err := scan.Scan(&f.ID, &f.UserID, &started, &ended, &target, &f.Notes, &createdAt); err != nil {
		return Fast{}, err
	}
	startedAt, err := parseTime(started.String)
	if err != nil {
		return Fast{}, err
	}
	f.StartedAt = startedAt
	if ended.Valid {
		if t, err := parseTime(ended.String); err == nil {
			f.EndedAt = &t
		}
	}
	if target.Valid {
		f.TargetHours = &target.Float64
	}
	if createdAt.Valid {
		if t, err := parseTime(createdAt.String); err == nil {
			f.CreatedAt = t
		}
	}
	f.DurationHours = durationHours(f.StartedAt, f.EndedAt)
	return f, nil
}

// durationHours returns elapsed hours from started until ended (or now),
// rounded to 2 decimals. It returns nil only if started_at is unset, which
// never happens for stored rows.
func durationHours(started time.Time, ended *time.Time) *float64 {
	if started.IsZero() {
		return nil
	}
	end := time.Now().UTC()
	if ended != nil {
		end = *ended
	}
	h := math.Round(end.Sub(started).Hours()*100) / 100
	return &h
}

// validateTarget rejects unusable target_hours values.
func validateTarget(t *float64) error {
	if t == nil {
		return nil
	}
	if !finite(*t) {
		return invalidf("target_hours", "must be a finite number")
	}
	if *t <= 0 {
		return invalidf("target_hours", "must be > 0")
	}
	return nil
}

// Create starts a fast. started_at zero means now. Creating while the user
// already has an active fast is a conflict.
func (s *FastStore) Create(ctx context.Context, userID int64, in Fast) (Fast, error) {
	if err := validateTarget(in.TargetHours); err != nil {
		return Fast{}, err
	}
	if in.StartedAt.IsZero() {
		in.StartedAt = time.Now().UTC()
	}
	if in.EndedAt != nil && in.EndedAt.Before(in.StartedAt) {
		return Fast{}, invalidf("ended_at", "cannot be before started_at")
	}
	// One active fast per user.
	var one int64
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM fasts WHERE user_id = ? AND ended_at IS NULL`, userID).Scan(&one)
	if err == nil {
		return Fast{}, fmt.Errorf("active fast already exists: %w", ErrConflict)
	}
	if err != sql.ErrNoRows {
		return Fast{}, err
	}
	var ended any
	if in.EndedAt != nil {
		ended = formatTime(*in.EndedAt)
	}
	var target any
	if in.TargetHours != nil {
		target = *in.TargetHours
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO fasts (user_id, started_at, ended_at, target_hours, notes) VALUES (?, ?, ?, ?, ?)`,
		userID, formatTime(in.StartedAt), ended, target, strings.TrimSpace(in.Notes))
	if err != nil {
		return Fast{}, fmt.Errorf("insert fast: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Fast{}, err
	}
	return s.Get(ctx, id, userID)
}

// Get returns one of the user's fasts, with the computed duration.
func (s *FastStore) Get(ctx context.Context, id, userID int64) (Fast, error) {
	f, err := scanFast(s.db.QueryRowContext(ctx,
		`SELECT `+fastCols+` FROM fasts WHERE id = ? AND user_id = ?`, id, userID))
	if err == sql.ErrNoRows {
		return Fast{}, ErrNotFound
	}
	if err != nil {
		return Fast{}, err
	}
	return f, nil
}

// Active returns the user's active (not yet ended) fast. If there is none,
// it returns ErrNotFound — the API maps that to 404 "no active fast".
func (s *FastStore) Active(ctx context.Context, userID int64) (Fast, error) {
	f, err := scanFast(s.db.QueryRowContext(ctx,
		`SELECT `+fastCols+` FROM fasts WHERE user_id = ? AND ended_at IS NULL`, userID))
	if err == sql.ErrNoRows {
		return Fast{}, ErrNotFound
	}
	if err != nil {
		return Fast{}, err
	}
	return f, nil
}

// ListByUser returns the user's fasts ordered newest first by started_at.
// from/to (when non-zero) bound started_at; limit is clamped.
func (s *FastStore) ListByUser(ctx context.Context, userID int64, from, to time.Time, limit int) ([]Fast, error) {
	q := `SELECT ` + fastCols + ` FROM fasts WHERE user_id = ?`
	args := []any{userID}
	if !from.IsZero() {
		q += " AND started_at >= ?"
		args = append(args, formatTime(from))
	}
	if !to.IsZero() {
		q += " AND started_at <= ?"
		args = append(args, formatTime(to))
	}
	q += " ORDER BY started_at DESC LIMIT ?"
	args = append(args, clampLimit(limit))
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Fast
	for rows.Next() {
		f, err := scanFast(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// Update modifies one of the user's fasts. Nil / zero fields keep their
// existing values: a zero started_at keeps the stored start, a nil
// target_hours keeps the stored target (a non-nil pointer replaces it),
// and notes are always replaced with the trimmed value — callers passing
// a "keep" semantic must load the current row first.
func (s *FastStore) Update(ctx context.Context, id, userID int64, in Fast) (Fast, error) {
	if err := validateTarget(in.TargetHours); err != nil {
		return Fast{}, err
	}
	cur, err := s.Get(ctx, id, userID)
	if err != nil {
		return Fast{}, err
	}
	started := cur.StartedAt
	if !in.StartedAt.IsZero() {
		started = in.StartedAt
	}
	target := cur.TargetHours
	if in.TargetHours != nil {
		target = in.TargetHours
	}
	ended := cur.EndedAt
	if ended != nil && ended.Before(started) {
		return Fast{}, invalidf("started_at", "cannot be after ended_at")
	}
	var t any
	if target != nil {
		t = *target
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE fasts SET started_at = ?, target_hours = ?, notes = ? WHERE id = ? AND user_id = ?`,
		formatTime(started), t, strings.TrimSpace(in.Notes), id, userID)
	if err != nil {
		return Fast{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Fast{}, ErrNotFound
	}
	return s.Get(ctx, id, userID)
}

// End closes one of the user's fasts. endedAt zero means now. Ending an
// already-ended fast is a conflict; ending before the start is a
// validation error.
func (s *FastStore) End(ctx context.Context, id, userID int64, endedAt time.Time) (Fast, error) {
	cur, err := s.Get(ctx, id, userID)
	if err != nil {
		return Fast{}, err
	}
	if cur.EndedAt != nil {
		return Fast{}, fmt.Errorf("fast already ended: %w", ErrConflict)
	}
	if endedAt.IsZero() {
		endedAt = time.Now().UTC()
	}
	if endedAt.Before(cur.StartedAt) {
		return Fast{}, invalidf("ended_at", "cannot end before start")
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE fasts SET ended_at = ? WHERE id = ? AND user_id = ? AND ended_at IS NULL`,
		formatTime(endedAt), id, userID)
	if err != nil {
		return Fast{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Fast{}, ErrConflict
	}
	return s.Get(ctx, id, userID)
}

// Delete removes one of the user's fasts.
func (s *FastStore) Delete(ctx context.Context, id, userID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM fasts WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
