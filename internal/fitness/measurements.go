package fitness

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Metric is a measurable quantity definition (weight, waist, ...).
type Metric struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Label     string    `json:"label"`
	Unit      string    `json:"unit"`
	Category  string    `json:"category"`
	IsSystem  bool      `json:"is_system"`
	CreatedAt time.Time `json:"created_at"`
}

// Measurement is one recorded value of a metric at a point in time.
type Measurement struct {
	ID         int64     `json:"id"`
	MetricID   int64     `json:"metric_id"`
	MetricName string    `json:"metric_name"`
	MetricUnit string    `json:"metric_unit"`
	Value      float64   `json:"value"`
	MeasuredAt time.Time `json:"measured_at"`
	Note       string    `json:"note"`
	CreatedAt  time.Time `json:"created_at"`
}

// MetricStore provides metric-catalog persistence.
type MetricStore struct{ db *sql.DB }

// NewMetricStore wraps db.
func NewMetricStore(db *sql.DB) *MetricStore { return &MetricStore{db: db} }

// List returns metrics ordered by category, then label.
func (s *MetricStore) List(ctx context.Context) ([]Metric, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, label, unit, category, is_system, created_at
		FROM metrics ORDER BY category, label`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Metric
	for rows.Next() {
		var m Metric
		var isSystem int
		var createdAt string
		if err := rows.Scan(&m.ID, &m.Name, &m.Label, &m.Unit, &m.Category, &isSystem, &createdAt); err != nil {
			return nil, err
		}
		created, err := parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		m.CreatedAt = created
		m.IsSystem = isSystem == 1
		out = append(out, m)
	}
	return out, rows.Err()
}

// Create adds a custom (non-system) metric definition.
func (s *MetricStore) Create(ctx context.Context, m Metric) (Metric, error) {
	m.Name = strings.TrimSpace(m.Name)
	m.Label = strings.TrimSpace(m.Label)
	m.Unit = strings.TrimSpace(m.Unit)
	if m.Name == "" {
		return Metric{}, invalidf("name", "must not be empty")
	}
	if m.Label == "" {
		m.Label = m.Name
	}
	if m.Unit == "" {
		return Metric{}, invalidf("unit", "must not be empty")
	}
	if m.Category == "" {
		m.Category = "custom"
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO metrics (name, label, unit, category, is_system) VALUES (?, ?, ?, ?, 0)`,
		m.Name, m.Label, m.Unit, m.Category)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Metric{}, ErrConflict
		}
		return Metric{}, fmt.Errorf("insert metric: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Metric{}, err
	}
	m.ID = id
	m.IsSystem = false
	m.CreatedAt = time.Now().UTC()
	return m, nil
}

// Get returns one metric by id.
func (s *MetricStore) Get(ctx context.Context, id int64) (Metric, error) {
	var m Metric
	var isSystem int
	var createdAt string
	err := s.db.QueryRowContext(ctx, `SELECT id, name, label, unit, category, is_system, created_at
		FROM metrics WHERE id = ?`, id).
		Scan(&m.ID, &m.Name, &m.Label, &m.Unit, &m.Category, &isSystem, &createdAt)
	if err == sql.ErrNoRows {
		return Metric{}, ErrNotFound
	}
	if err != nil {
		return Metric{}, err
	}
	created, err := parseTime(createdAt)
	if err != nil {
		return Metric{}, err
	}
	m.CreatedAt = created
	m.IsSystem = isSystem == 1
	return m, nil
}

// Update modifies a custom metric. System metrics are immutable; a missing
// row is ErrNotFound.
func (s *MetricStore) Update(ctx context.Context, id int64, m Metric) (Metric, error) {
	var isSystem int
	err := s.db.QueryRowContext(ctx, `SELECT is_system FROM metrics WHERE id = ?`, id).Scan(&isSystem)
	if err == sql.ErrNoRows {
		return Metric{}, ErrNotFound
	}
	if err != nil {
		return Metric{}, err
	}
	if isSystem == 1 {
		return Metric{}, ErrForbidden
	}
	m.Name = strings.TrimSpace(m.Name)
	m.Label = strings.TrimSpace(m.Label)
	m.Unit = strings.TrimSpace(m.Unit)
	if m.Name == "" {
		return Metric{}, invalidf("name", "must not be empty")
	}
	if m.Label == "" {
		m.Label = m.Name
	}
	if m.Unit == "" {
		return Metric{}, invalidf("unit", "must not be empty")
	}
	if m.Category == "" {
		m.Category = "custom"
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE metrics SET name = ?, label = ?, unit = ?, category = ? WHERE id = ?`,
		m.Name, m.Label, m.Unit, m.Category, id)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Metric{}, ErrConflict
		}
		return Metric{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Metric{}, ErrNotFound
	}
	return s.Get(ctx, id)
}

// Delete removes a custom metric. System metrics are immutable. Metrics with
// existing measurements are not deletable (historic data must stay
// interpretable); delete the measurements first.
func (s *MetricStore) Delete(ctx context.Context, id int64) error {
	var isSystem int
	err := s.db.QueryRowContext(ctx, `SELECT is_system FROM metrics WHERE id = ?`, id).Scan(&isSystem)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if isSystem == 1 {
		return ErrForbidden
	}
	var used int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM measurements WHERE metric_id = ?`, id).Scan(&used); err != nil {
		return err
	}
	if used > 0 {
		return ErrConflict
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM metrics WHERE id = ? AND is_system = 0`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// MeasurementStore provides measurement persistence.
type MeasurementStore struct{ db *sql.DB }

// NewMeasurementStore wraps db.
func NewMeasurementStore(db *sql.DB) *MeasurementStore { return &MeasurementStore{db: db} }

// Create records one measurement. measuredAt zero means now.
func (s *MeasurementStore) Create(ctx context.Context, userID int64, in Measurement) (Measurement, error) {
	if !finite(in.Value) {
		return Measurement{}, invalidf("value", "must be a finite number")
	}
	if in.Value <= 0 {
		return Measurement{}, invalidf("value", "must be > 0")
	}
	if in.MetricID == 0 {
		return Measurement{}, invalidf("metric_id", "is required")
	}
	var name, unit string
	err := s.db.QueryRowContext(ctx, `SELECT name, unit FROM metrics WHERE id = ?`, in.MetricID).Scan(&name, &unit)
	if err == sql.ErrNoRows {
		return Measurement{}, invalidf("metric_id", "unknown metric %d", in.MetricID)
	}
	if err != nil {
		return Measurement{}, err
	}
	if in.MeasuredAt.IsZero() {
		in.MeasuredAt = time.Now().UTC()
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO measurements (user_id, metric_id, value, measured_at, note) VALUES (?, ?, ?, ?, ?)`,
		userID, in.MetricID, in.Value, formatTime(in.MeasuredAt), strings.TrimSpace(in.Note))
	if err != nil {
		return Measurement{}, fmt.Errorf("insert measurement: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Measurement{}, err
	}
	in.ID = id
	in.MetricName = name
	in.MetricUnit = unit
	in.CreatedAt = time.Now().UTC()
	return in, nil
}

// ListByUser returns measurements for a user ordered newest first. metricID
// > 0 filters by metric; from/to (when non-zero) bound measured_at.
func (s *MeasurementStore) ListByUser(ctx context.Context, userID, metricID int64, from, to time.Time, limit int) ([]Measurement, error) {
	q := `SELECT m.id, m.metric_id, mt.name, mt.unit, m.value, m.measured_at, m.note, m.created_at
		FROM measurements m JOIN metrics mt ON mt.id = m.metric_id
		WHERE m.user_id = ?`
	args := []any{userID}
	if metricID > 0 {
		q += " AND m.metric_id = ?"
		args = append(args, metricID)
	}
	if !from.IsZero() {
		q += " AND m.measured_at >= ?"
		args = append(args, formatTime(from))
	}
	if !to.IsZero() {
		q += " AND m.measured_at <= ?"
		args = append(args, formatTime(to))
	}
	q += " ORDER BY m.measured_at DESC LIMIT ?"
	args = append(args, clampLimit(limit))
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Measurement
	for rows.Next() {
		var m Measurement
		var measuredAt, createdAt string
		if err := rows.Scan(&m.ID, &m.MetricID, &m.MetricName, &m.MetricUnit, &m.Value, &measuredAt, &m.Note, &createdAt); err != nil {
			return nil, err
		}
		measured, err := parseTime(measuredAt)
		if err != nil {
			return nil, err
		}
		created, err := parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		m.MeasuredAt, m.CreatedAt = measured, created
		out = append(out, m)
	}
	return out, rows.Err()
}

// Get returns one of the user's measurements.
func (s *MeasurementStore) Get(ctx context.Context, id, userID int64) (Measurement, error) {
	var m Measurement
	var measuredAt, createdAt string
	err := s.db.QueryRowContext(ctx, `SELECT m.id, m.metric_id, mt.name, mt.unit, m.value, m.measured_at, m.note, m.created_at
		FROM measurements m JOIN metrics mt ON mt.id = m.metric_id
		WHERE m.id = ? AND m.user_id = ?`, id, userID).
		Scan(&m.ID, &m.MetricID, &m.MetricName, &m.MetricUnit, &m.Value, &measuredAt, &m.Note, &createdAt)
	if err == sql.ErrNoRows {
		return Measurement{}, ErrNotFound
	}
	if err != nil {
		return Measurement{}, err
	}
	measured, err := parseTime(measuredAt)
	if err != nil {
		return Measurement{}, err
	}
	created, err := parseTime(createdAt)
	if err != nil {
		return Measurement{}, err
	}
	m.MeasuredAt, m.CreatedAt = measured, created
	return m, nil
}

// Update modifies one of the user's measurements (value, measured_at, note).
func (s *MeasurementStore) Update(ctx context.Context, id, userID int64, in Measurement) (Measurement, error) {
	if !finite(in.Value) {
		return Measurement{}, invalidf("value", "must be a finite number")
	}
	if in.Value <= 0 {
		return Measurement{}, invalidf("value", "must be > 0")
	}
	if in.MeasuredAt.IsZero() {
		in.MeasuredAt = time.Now().UTC()
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE measurements SET value = ?, measured_at = ?, note = ? WHERE id = ? AND user_id = ?`,
		in.Value, formatTime(in.MeasuredAt), strings.TrimSpace(in.Note), id, userID)
	if err != nil {
		return Measurement{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Measurement{}, ErrNotFound
	}
	return s.Get(ctx, id, userID)
}

// Delete removes one of the user's measurements.
func (s *MeasurementStore) Delete(ctx context.Context, id, userID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM measurements WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// LatestByMetric returns the most recent measurement per metric for a user,
// useful for a dashboard "current stats" summary. Metrics with no data are
// omitted.
func (s *MeasurementStore) LatestByMetric(ctx context.Context, userID int64) ([]Measurement, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT m.id, m.metric_id, mt.name, mt.unit, m.value, m.measured_at, m.note, m.created_at
		FROM measurements m
		JOIN metrics mt ON mt.id = m.metric_id
		WHERE m.user_id = ?
		  AND m.measured_at = (
			SELECT MAX(m2.measured_at) FROM measurements m2
			WHERE m2.user_id = m.user_id AND m2.metric_id = m.metric_id
		  )
		ORDER BY mt.category, mt.label`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Measurement
	for rows.Next() {
		var m Measurement
		var measuredAt, createdAt string
		if err := rows.Scan(&m.ID, &m.MetricID, &m.MetricName, &m.MetricUnit, &m.Value, &measuredAt, &m.Note, &createdAt); err != nil {
			return nil, err
		}
		measured, err := parseTime(measuredAt)
		if err != nil {
			return nil, err
		}
		created, err := parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		m.MeasuredAt, m.CreatedAt = measured, created
		out = append(out, m)
	}
	return out, rows.Err()
}
