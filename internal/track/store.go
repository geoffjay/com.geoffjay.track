// Package track is the domain layer: check-ins, totals, and the series
// feeding the charts.
package track

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// CheckIn is one logged rowing session.
type CheckIn struct {
	ID        int64
	UserID    int64
	Username  string
	Miles     float64
	RowedAt   time.Time
	CreatedAt time.Time
}

// Totals is one user's aggregate stats.
type Totals struct {
	Username string
	Miles    float64
	CheckIns int
	LastAt   time.Time
	Has      bool
}

// Store provides check-in persistence and aggregate queries.
type Store struct{ db *sql.DB }

// NewStore wraps db.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// parseTime parses the ISO-8601 UTC text stored by SQLite.
func parseTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000Z", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unparseable timestamp %q", s)
}

// Create inserts a check-in. rowedAt may be the zero time (defaults to now).
func (s *Store) Create(ctx context.Context, userID int64, miles float64, rowedAt time.Time) (CheckIn, error) {
	if rowedAt.IsZero() {
		rowedAt = time.Now().UTC()
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO checkins (user_id, miles, rowed_at) VALUES (?, ?, ?)`,
		userID, miles, rowedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return CheckIn{}, fmt.Errorf("insert checkin: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return CheckIn{}, err
	}
	username, err := s.username(ctx, userID)
	if err != nil {
		return CheckIn{}, err
	}
	return CheckIn{ID: id, UserID: userID, Username: username, Miles: miles, RowedAt: rowedAt.UTC()}, nil
}

// Delete removes a check-in owned by userID.
func (s *Store) Delete(ctx context.Context, id, userID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM checkins WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Recent returns the latest n check-ins across both users.
func (s *Store) Recent(ctx context.Context, n int) ([]CheckIn, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.user_id, u.username, c.miles, c.rowed_at, c.created_at
		 FROM checkins c JOIN users u ON u.id = c.user_id
		 ORDER BY c.rowed_at DESC, c.id DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCheckIns(rows)
}

// ForUser returns a user's check-ins, newest first.
func (s *Store) ForUser(ctx context.Context, userID int64, n int) ([]CheckIn, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.user_id, u.username, c.miles, c.rowed_at, c.created_at
		 FROM checkins c JOIN users u ON u.id = c.user_id
		 WHERE c.user_id = ? ORDER BY c.rowed_at DESC, c.id DESC LIMIT ?`, userID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCheckIns(rows)
}

// Totals returns each seeded user's totals, ordered by miles desc. Both users
// always appear (LEFT JOIN against users), so an empty streak still renders.
func (s *Store) Totals(ctx context.Context) ([]Totals, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT u.username,
		        COALESCE(SUM(c.miles), 0),
		        COUNT(c.id),
		        MAX(c.rowed_at)
		 FROM users u LEFT JOIN checkins c ON c.user_id = u.id
		 GROUP BY u.id ORDER BY 2 DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Totals
	for rows.Next() {
		var t Totals
		var last sql.NullString
		if err := rows.Scan(&t.Username, &t.Miles, &t.CheckIns, &last); err != nil {
			return nil, err
		}
		if last.Valid {
			t.LastAt, _ = parseTime(last.String)
		}
		t.Has = t.CheckIns > 0
		out = append(out, t)
	}
	return out, rows.Err()
}

// DailySeries returns per-user cumulative miles by day over the last n days,
// aligned across users (a day with no rows for a user carries the last
// cumulative value forward). This feeds the progress line chart.
func (s *Store) DailySeries(ctx context.Context, users []string, n int) (map[string][]LinePoint, error) {
	since := time.Now().UTC().AddDate(0, 0, -(n - 1)).Truncate(24 * time.Hour)
	rows, err := s.db.QueryContext(ctx,
		`SELECT u.username, c.rowed_at, c.miles
		 FROM checkins c JOIN users u ON u.id = c.user_id
		 WHERE c.rowed_at >= ?
		 ORDER BY c.rowed_at ASC`, since.Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}

type dayMiles struct {
	username string
	day      time.Time
	miles    float64
}
	var events []dayMiles
	for rows.Next() {
		var username, at string
		var miles float64
		if err := rows.Scan(&username, &at, &miles); err != nil {
			return nil, err
		}
		t, err := parseTime(at)
		if err != nil {
			return nil, err
		}
		events = append(events, dayMiles{username, t.Truncate(24 * time.Hour), miles})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Walk day by day from since to today, carrying totals forward.
	out := make(map[string][]LinePoint, len(users))
	totals := make(map[string]float64, len(users))
	days := int(time.Now().UTC().Truncate(24*time.Hour).Sub(since).Hours()/24) + 1
	idx := 0
	for d := range days {
		day := since.AddDate(0, 0, d)
		for idx < len(events) && !events[idx].day.After(day) {
			totals[events[idx].username] += events[idx].miles
			idx++
		}
		for _, u := range users {
			out[u] = append(out[u], LinePoint{Day: day, Miles: totals[u]})
		}
	}
	return out, rows.Err()
}

// LinePoint is one point of the cumulative-miles line chart.
type LinePoint struct {
	Day   time.Time
	Miles float64
}

// username fetches a user's name by id.
func (s *Store) username(ctx context.Context, id int64) (string, error) {
	var name string
	err := s.db.QueryRowContext(ctx, `SELECT username FROM users WHERE id = ?`, id).Scan(&name)
	return name, err
}

// scanCheckIns materializes the shared SELECT column order.
func scanCheckIns(rows *sql.Rows) ([]CheckIn, error) {
	var out []CheckIn
	for rows.Next() {
		var c CheckIn
		var rowed, created string
		if err := rows.Scan(&c.ID, &c.UserID, &c.Username, &c.Miles, &rowed, &created); err != nil {
			return nil, err
		}
		var err error
		if c.RowedAt, err = parseTime(rowed); err != nil {
			return nil, err
		}
		if c.CreatedAt, err = parseTime(created); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}