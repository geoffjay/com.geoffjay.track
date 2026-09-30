package fitness

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Exercise is a strength ('weighted') or cardio ('timed') exercise
// definition. Exercises are a shared catalog, like metrics: they carry no
// user ownership.
type Exercise struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // "weighted" | "timed"
	Muscles   string    `json:"muscles"`
	Equipment string    `json:"equipment"`
	IsSystem  bool      `json:"is_system"`
	CreatedAt time.Time `json:"created_at"`
}

// WorkoutEntry is one exercise performed within a workout. weight_kg/reps
// carry weighted exercises; duration_sec (and optional distance_km) carry
// timed ones. exercise_name/exercise_type are joined from the catalog.
type WorkoutEntry struct {
	ID           int64    `json:"id"`
	WorkoutID    int64    `json:"workout_id"`
	ExerciseID   int64    `json:"exercise_id"`
	ExerciseName string   `json:"exercise_name"`
	ExerciseType string   `json:"exercise_type"`
	WeightKg     *float64 `json:"weight_kg,omitempty"`
	Reps         *int     `json:"reps,omitempty"`
	DurationSec  *float64 `json:"duration_sec,omitempty"`
	DistanceKm   *float64 `json:"distance_km,omitempty"`
	Effort       *int     `json:"effort,omitempty"`
	Notes        string   `json:"notes"`
	Position     int      `json:"position"`
}

// Workout is a logged training session. duration_min is computed from
// ended_at - started_at and is only set once the workout is ended.
type Workout struct {
	ID          int64          `json:"id"`
	UserID      int64          `json:"-"`
	Name        string         `json:"name"`
	StartedAt   time.Time      `json:"started_at"`
	EndedAt     *time.Time     `json:"ended_at,omitempty"`
	Effort      *int           `json:"effort,omitempty"`
	Notes       string         `json:"notes"`
	CreatedAt   time.Time      `json:"created_at"`
	Entries     []WorkoutEntry `json:"entries"`
	DurationMin *float64       `json:"duration_min,omitempty"`
}

// ExerciseStore provides exercise-catalog persistence.
type ExerciseStore struct{ db *sql.DB }

// NewExerciseStore wraps db.
func NewExerciseStore(db *sql.DB) *ExerciseStore { return &ExerciseStore{db: db} }

// validateExercise normalizes and validates catalog input shared by Create
// and Update.
func validateExercise(e Exercise) (Exercise, error) {
	e.Name = strings.TrimSpace(e.Name)
	e.Type = strings.TrimSpace(e.Type)
	e.Muscles = strings.TrimSpace(e.Muscles)
	e.Equipment = strings.TrimSpace(e.Equipment)
	if e.Name == "" {
		return Exercise{}, invalidf("name", "must not be empty")
	}
	if e.Type != "weighted" && e.Type != "timed" {
		return Exercise{}, invalidf("type", `must be "weighted" or "timed"`)
	}
	if e.Muscles == "" {
		e.Muscles = "[]"
	}
	return e, nil
}

// List returns exercises ordered by type, then name.
func (s *ExerciseStore) List(ctx context.Context) ([]Exercise, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, type, muscles, equipment, is_system, created_at
		FROM exercises ORDER BY type, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Exercise
	for rows.Next() {
		var e Exercise
		var isSystem int
		var createdAt sql.NullString
		if err := rows.Scan(&e.ID, &e.Name, &e.Type, &e.Muscles, &e.Equipment, &isSystem, &createdAt); err != nil {
			return nil, err
		}
		if createdAt.Valid {
			t, err := parseTime(createdAt.String)
			if err != nil {
				return nil, fmt.Errorf("parse created_at: %w", err)
			}
			e.CreatedAt = t
		}
		e.IsSystem = isSystem == 1
		out = append(out, e)
	}
	return out, rows.Err()
}

// Get returns one exercise by id.
func (s *ExerciseStore) Get(ctx context.Context, id int64) (Exercise, error) {
	var e Exercise
	var isSystem int
	var createdAt sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, name, type, muscles, equipment, is_system, created_at
		FROM exercises WHERE id = ?`, id).
		Scan(&e.ID, &e.Name, &e.Type, &e.Muscles, &e.Equipment, &isSystem, &createdAt)
	if err == sql.ErrNoRows {
		return Exercise{}, ErrNotFound
	}
	if err != nil {
		return Exercise{}, err
	}
	if createdAt.Valid {
		t, err := parseTime(createdAt.String)
		if err != nil {
			return Exercise{}, fmt.Errorf("parse created_at: %w", err)
		}
		e.CreatedAt = t
	}
	e.IsSystem = isSystem == 1
	return e, nil
}

// Create adds a custom (non-system) exercise. A duplicate name is ErrConflict.
func (s *ExerciseStore) Create(ctx context.Context, e Exercise) (Exercise, error) {
	e, err := validateExercise(e)
	if err != nil {
		return Exercise{}, err
	}
	e.IsSystem = false
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO exercises (name, type, muscles, equipment, is_system) VALUES (?, ?, ?, ?, 0)`,
		e.Name, e.Type, e.Muscles, e.Equipment)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Exercise{}, ErrConflict
		}
		return Exercise{}, fmt.Errorf("insert exercise: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Exercise{}, err
	}
	return s.Get(ctx, id)
}

// Update modifies a custom exercise. System exercises are immutable; a
// missing row is ErrNotFound; a duplicate name is ErrConflict.
func (s *ExerciseStore) Update(ctx context.Context, id int64, e Exercise) (Exercise, error) {
	var isSystem int
	err := s.db.QueryRowContext(ctx, `SELECT is_system FROM exercises WHERE id = ?`, id).Scan(&isSystem)
	if err == sql.ErrNoRows {
		return Exercise{}, ErrNotFound
	}
	if err != nil {
		return Exercise{}, err
	}
	if isSystem == 1 {
		return Exercise{}, ErrForbidden
	}
	e, err = validateExercise(e)
	if err != nil {
		return Exercise{}, err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE exercises SET name = ?, type = ?, muscles = ?, equipment = ? WHERE id = ? AND is_system = 0`,
		e.Name, e.Type, e.Muscles, e.Equipment, id)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Exercise{}, ErrConflict
		}
		return Exercise{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Exercise{}, ErrNotFound
	}
	return s.Get(ctx, id)
}

// Delete removes a custom exercise. System exercises are immutable.
// Exercises referenced by workout entries are not deletable (history stays
// interpretable); delete the entries first.
func (s *ExerciseStore) Delete(ctx context.Context, id int64) error {
	var isSystem int
	err := s.db.QueryRowContext(ctx, `SELECT is_system FROM exercises WHERE id = ?`, id).Scan(&isSystem)
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
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workout_entries WHERE exercise_id = ?`, id).Scan(&used); err != nil {
		return err
	}
	if used > 0 {
		return ErrConflict
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM exercises WHERE id = ? AND is_system = 0`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// WorkoutStore provides workout persistence. Every query filters on user_id;
// rows belonging to another user surface as ErrNotFound.
type WorkoutStore struct{ db *sql.DB }

// NewWorkoutStore wraps db.
func NewWorkoutStore(db *sql.DB) *WorkoutStore { return &WorkoutStore{db: db} }

// nullFloatPtr converts a scanned nullable float into a pointer (nil = NULL).
func nullFloatPtr(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	f := v.Float64
	return &f
}

// nullIntPtr converts a scanned nullable int into a pointer (nil = NULL).
func nullIntPtr(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int64)
	return &i
}

// timeArg renders an optional timestamp for a TEXT column (nil = NULL).
func timeArg(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatTime(*t)
}

// intArg renders an optional int for a nullable column (nil = NULL).
func intArg(i *int) any {
	if i == nil {
		return nil
	}
	return *i
}

// validateWorkout normalizes and validates workout fields. startedAt zero
// means now; endedAt (when set) must not precede startedAt.
func validateWorkout(w Workout) (Workout, error) {
	w.Name = strings.TrimSpace(w.Name)
	w.Notes = strings.TrimSpace(w.Notes)
	if w.Effort != nil && (*w.Effort < 1 || *w.Effort > 10) {
		return Workout{}, invalidf("effort", "must be between 1 and 10")
	}
	if w.StartedAt.IsZero() {
		w.StartedAt = time.Now().UTC()
	}
	if w.EndedAt != nil && w.EndedAt.Before(w.StartedAt) {
		return Workout{}, invalidf("ended_at", "cannot end before start")
	}
	return w, nil
}

// validateEntries normalizes entries and checks each against its exercise:
// the exercise must exist, and the entry shape must match the exercise type
// (weighted: weight_kg + reps; timed: duration_sec, optional distance_km).
// Unknown exercises are reported per-entry as a ValidationError.
func (s *WorkoutStore) validateEntries(ctx context.Context, entries []WorkoutEntry) ([]WorkoutEntry, error) {
	if entries == nil {
		entries = []WorkoutEntry{}
	}
	exercises := &ExerciseStore{db: s.db}
	cache := make(map[int64]Exercise, len(entries))
	out := make([]WorkoutEntry, len(entries))
	for i, e := range entries {
		if e.ExerciseID <= 0 {
			return nil, invalidf("entries", "unknown exercise %d", e.ExerciseID)
		}
		ex, ok := cache[e.ExerciseID]
		if !ok {
			x, err := exercises.Get(ctx, e.ExerciseID)
			if err == ErrNotFound {
				return nil, invalidf("entries", "unknown exercise %d", e.ExerciseID)
			}
			if err != nil {
				return nil, err
			}
			cache[e.ExerciseID] = x
			ex = x
		}
		e.ExerciseName = ex.Name
		e.ExerciseType = ex.Type
		switch ex.Type {
		case "weighted":
			if e.WeightKg == nil || e.Reps == nil || e.DurationSec != nil || e.DistanceKm != nil ||
				!finite(*e.WeightKg) || *e.WeightKg < 0 || *e.Reps < 1 {
				return nil, invalidf("entries", "exercise %q is type %q; expected %s fields",
					ex.Name, ex.Type, "weight_kg and reps")
			}
		case "timed":
			if e.DurationSec == nil || e.WeightKg != nil || e.Reps != nil ||
				!finite(*e.DurationSec) || *e.DurationSec <= 0 ||
				(e.DistanceKm != nil && (!finite(*e.DistanceKm) || *e.DistanceKm < 0)) {
				return nil, invalidf("entries", "exercise %q is type %q; expected %s fields",
					ex.Name, ex.Type, "duration_sec")
			}
		}
		if e.Effort != nil && (*e.Effort < 1 || *e.Effort > 10) {
			return nil, invalidf("entries", "effort must be between 1 and 10")
		}
		e.Notes = strings.TrimSpace(e.Notes)
		out[i] = e
	}
	return out, nil
}

// insertEntries writes entries under workoutID with positions 0..n-1.
func insertEntries(ctx context.Context, tx *sql.Tx, workoutID int64, entries []WorkoutEntry) error {
	for i, e := range entries {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO workout_entries
				(workout_id, exercise_id, weight_kg, reps, duration_sec, distance_km, effort, notes, position)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			workoutID, e.ExerciseID, e.WeightKg, e.Reps, e.DurationSec, e.DistanceKm, e.Effort, e.Notes, i)
		if err != nil {
			return fmt.Errorf("insert workout entry: %w", err)
		}
	}
	return nil
}

// rowScanner is implemented by both *sql.Row and *sql.Rows.
type rowScanner interface{ Scan(dest ...any) error }

// scanWorkout reads one workouts row, resolving the nullable ended_at/effort
// and computing duration_min when the workout is ended. Timestamps are
// stored as TEXT and parsed explicitly (the sqlite driver returns strings).
func scanWorkout(scan rowScanner) (Workout, error) {
	var w Workout
	var started, ended, createdAt sql.NullString
	var effort sql.NullInt64
	if err := scan.Scan(&w.ID, &w.UserID, &w.Name, &started, &ended, &effort, &w.Notes, &createdAt); err != nil {
		return Workout{}, err
	}
	var err error
	if w.StartedAt, err = parseTime(started.String); err != nil {
		return Workout{}, fmt.Errorf("parse started_at: %w", err)
	}
	if createdAt.Valid {
		if w.CreatedAt, err = parseTime(createdAt.String); err != nil {
			return Workout{}, fmt.Errorf("parse created_at: %w", err)
		}
	}
	if ended.Valid {
		t, err := parseTime(ended.String)
		if err != nil {
			return Workout{}, fmt.Errorf("parse ended_at: %w", err)
		}
		w.EndedAt = &t
		min := w.EndedAt.Sub(w.StartedAt).Minutes()
		w.DurationMin = &min
	}
	w.Effort = nullIntPtr(effort)
	w.Entries = []WorkoutEntry{}
	return w, nil
}

// scanEntry reads one workout_entries row (with joined exercise fields).
func scanEntry(scan rowScanner) (WorkoutEntry, error) {
	var e WorkoutEntry
	var weightKg, durationSec, distanceKm sql.NullFloat64
	var reps, effort sql.NullInt64
	if err := scan.Scan(&e.ID, &e.WorkoutID, &e.ExerciseID, &e.ExerciseName, &e.ExerciseType,
		&weightKg, &reps, &durationSec, &distanceKm, &effort, &e.Notes, &e.Position); err != nil {
		return WorkoutEntry{}, err
	}
	e.WeightKg = nullFloatPtr(weightKg)
	e.Reps = nullIntPtr(reps)
	e.DurationSec = nullFloatPtr(durationSec)
	e.DistanceKm = nullFloatPtr(distanceKm)
	e.Effort = nullIntPtr(effort)
	return e, nil
}

// Create records a workout with its entries in one transaction. startedAt
// zero means now. Entries are stored in slice order (position 0..n-1) and
// must match their exercise's type (weighted: weight_kg + reps; timed:
// duration_sec, optional distance_km).
func (s *WorkoutStore) Create(ctx context.Context, userID int64, w Workout, entries []WorkoutEntry) (Workout, error) {
	w, err := validateWorkout(w)
	if err != nil {
		return Workout{}, err
	}
	entries, err = s.validateEntries(ctx, entries)
	if err != nil {
		return Workout{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Workout{}, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO workouts (user_id, name, started_at, ended_at, effort, notes) VALUES (?, ?, ?, ?, ?, ?)`,
		userID, w.Name, formatTime(w.StartedAt), timeArg(w.EndedAt), intArg(w.Effort), w.Notes)
	if err != nil {
		return Workout{}, fmt.Errorf("insert workout: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Workout{}, err
	}
	if err := insertEntries(ctx, tx, id, entries); err != nil {
		return Workout{}, err
	}
	if err := tx.Commit(); err != nil {
		return Workout{}, err
	}
	return s.Get(ctx, id, userID)
}

// Get returns one of the user's workouts with its entries ordered by
// position. A workout owned by another user is ErrNotFound.
func (s *WorkoutStore) Get(ctx context.Context, id, userID int64) (Workout, error) {
	w, err := scanWorkout(s.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, started_at, ended_at, effort, notes, created_at
		FROM workouts WHERE id = ? AND user_id = ?`, id, userID))
	if err == sql.ErrNoRows {
		return Workout{}, ErrNotFound
	}
	if err != nil {
		return Workout{}, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT we.id, we.workout_id, we.exercise_id, e.name, e.type,
			we.weight_kg, we.reps, we.duration_sec, we.distance_km, we.effort, we.notes, we.position
		FROM workout_entries we JOIN exercises e ON e.id = we.exercise_id
		WHERE we.workout_id = ?
		ORDER BY we.position`, id)
	if err != nil {
		return Workout{}, err
	}
	defer rows.Close()
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return Workout{}, err
		}
		w.Entries = append(w.Entries, e)
	}
	if err := rows.Err(); err != nil {
		return Workout{}, err
	}
	return w, nil
}

// ListByUser returns the user's workouts ordered newest first, each with its
// entries loaded. from/to (when non-zero) bound started_at; limit is clamped.
func (s *WorkoutStore) ListByUser(ctx context.Context, userID int64, from, to time.Time, limit int) ([]Workout, error) {
	q := `SELECT id, user_id, name, started_at, ended_at, effort, notes, created_at
		FROM workouts WHERE user_id = ?`
	args := []any{userID}
	if !from.IsZero() {
		q += " AND started_at >= ?"
		args = append(args, formatTime(from))
	}
	if !to.IsZero() {
		q += " AND started_at <= ?"
		args = append(args, formatTime(to))
	}
	q += " ORDER BY started_at DESC, id DESC LIMIT ?"
	args = append(args, clampLimit(limit))
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Workout
	for rows.Next() {
		w, err := scanWorkout(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}

	// Load every listed workout's entries in one query, grouped by workout_id.
	placeholders := make([]string, len(out))
	byID := make(map[int64]*Workout, len(out))
	args = args[:0]
	for i, w := range out {
		placeholders[i] = "?"
		args = append(args, w.ID)
		byID[w.ID] = &out[i]
	}
	rows2, err := s.db.QueryContext(ctx,
		`SELECT we.id, we.workout_id, we.exercise_id, e.name, e.type,
			we.weight_kg, we.reps, we.duration_sec, we.distance_km, we.effort, we.notes, we.position
		FROM workout_entries we JOIN exercises e ON e.id = we.exercise_id
		WHERE we.workout_id IN (`+strings.Join(placeholders, ", ")+`)
		ORDER BY we.workout_id, we.position`, args...)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		e, err := scanEntry(rows2)
		if err != nil {
			return nil, err
		}
		if w := byID[e.WorkoutID]; w != nil {
			w.Entries = append(w.Entries, e)
		}
	}
	if err := rows2.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// Update modifies one of the user's workouts. Workout fields are
// full-replace: the caller resolves patch payloads (nil = keep existing)
// against the current row before calling. A nil entries slice keeps the
// existing entries; a non-nil slice replaces them wholesale (delete +
// reinsert, fresh positions, same validation as Create) in one transaction.
func (s *WorkoutStore) Update(ctx context.Context, id, userID int64, w Workout, entries []WorkoutEntry) (Workout, error) {
	w, err := validateWorkout(w)
	if err != nil {
		return Workout{}, err
	}
	if entries != nil {
		entries, err = s.validateEntries(ctx, entries)
		if err != nil {
			return Workout{}, err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Workout{}, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx,
		`UPDATE workouts SET name = ?, started_at = ?, ended_at = ?, effort = ?, notes = ?
		WHERE id = ? AND user_id = ?`,
		w.Name, formatTime(w.StartedAt), timeArg(w.EndedAt), intArg(w.Effort), w.Notes, id, userID)
	if err != nil {
		return Workout{}, fmt.Errorf("update workout: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Workout{}, ErrNotFound
	}
	if entries != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM workout_entries WHERE workout_id = ?`, id); err != nil {
			return Workout{}, fmt.Errorf("delete workout entries: %w", err)
		}
		if err := insertEntries(ctx, tx, id, entries); err != nil {
			return Workout{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Workout{}, err
	}
	return s.Get(ctx, id, userID)
}

// Finish marks a workout ended. endedAt zero means now. Ending an
// already-ended workout is ErrConflict; an endedAt before the workout's
// start is a ValidationError.
func (s *WorkoutStore) Finish(ctx context.Context, id, userID int64, endedAt time.Time) (Workout, error) {
	var startedAt time.Time
	var started, ended sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT started_at, ended_at FROM workouts WHERE id = ? AND user_id = ?`,
		id, userID).Scan(&started, &ended)
	if err == sql.ErrNoRows {
		return Workout{}, ErrNotFound
	}
	if err != nil {
		return Workout{}, err
	}
	if started.Valid {
		t, err := parseTime(started.String)
		if err != nil {
			return Workout{}, fmt.Errorf("parse started_at: %w", err)
		}
		startedAt = t
	}
	if ended.Valid {
		return Workout{}, fmt.Errorf("%w: workout already ended", ErrConflict)
	}
	if endedAt.IsZero() {
		endedAt = time.Now().UTC()
	}
	if endedAt.Before(startedAt) {
		return Workout{}, invalidf("ended_at", "cannot end before start")
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE workouts SET ended_at = ? WHERE id = ? AND user_id = ? AND ended_at IS NULL`,
		formatTime(endedAt), id, userID)
	if err != nil {
		return Workout{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Workout{}, fmt.Errorf("%w: workout already ended", ErrConflict)
	}
	return s.Get(ctx, id, userID)
}

// Delete removes one of the user's workouts. Entries cascade.
func (s *WorkoutStore) Delete(ctx context.Context, id, userID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM workouts WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
