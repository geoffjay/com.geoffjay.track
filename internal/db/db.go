package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open opens (creating if needed) the SQLite database at path and applies
// schema migrations. WAL journaling plus a busy timeout let the single
// process handle concurrent requests without SQLITE_BUSY errors.
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := d.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := migrate(d); err != nil {
		return nil, err
	}
	return d, nil
}

// migrate applies idempotent schema DDL.
func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		)`,
		`CREATE TABLE IF NOT EXISTS checkins (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			miles REAL NOT NULL CHECK (miles > 0),
			rowed_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_checkins_user_rowed ON checkins(user_id, rowed_at)`,

		// -- Fitness: metric catalog + point-in-time measurements (weight,
		// -- height, waist, bicep, etc.). A catalog row per metric keeps the
		// -- model open: new body metrics are data, not schema.
		`CREATE TABLE IF NOT EXISTS metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			label TEXT NOT NULL,
			unit TEXT NOT NULL,
			category TEXT NOT NULL DEFAULT 'body',
			is_system INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		)`,
		`CREATE TABLE IF NOT EXISTS measurements (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			metric_id INTEGER NOT NULL REFERENCES metrics(id),
			value REAL NOT NULL,
			measured_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
			note TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_measurements_user_metric_time ON measurements(user_id, metric_id, measured_at)`,

		// -- Fitness: shared food catalog. Nutrition is per serving_size
		// -- serving_unit; is_system marks seed rows (API-immutable).
		`CREATE TABLE IF NOT EXISTS foods (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			brand TEXT NOT NULL DEFAULT '',
			serving_size REAL NOT NULL DEFAULT 100 CHECK (serving_size > 0),
			serving_unit TEXT NOT NULL DEFAULT 'g',
			calories REAL NOT NULL CHECK (calories >= 0),
			protein REAL NOT NULL DEFAULT 0 CHECK (protein >= 0),
			carbs REAL NOT NULL DEFAULT 0 CHECK (carbs >= 0),
			fat REAL NOT NULL DEFAULT 0 CHECK (fat >= 0),
			is_system INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		)`,

		// -- Fitness: meals are containers of items. An item either references
		// -- a catalog food (nutrition computed = food x quantity, denormalized
		// -- at write time so history survives later food edits) or carries its
		// -- own numbers (ad-hoc entry: "calories only", "calories + macros",
		// -- "whole meal, rough portion").
		`CREATE TABLE IF NOT EXISTS meals (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name TEXT NOT NULL DEFAULT '',
			meal_type TEXT NOT NULL DEFAULT '',
			portion TEXT NOT NULL DEFAULT '' CHECK (portion IN ('', 'small', 'medium', 'large')),
			notes TEXT NOT NULL DEFAULT '',
			eaten_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_meals_user_time ON meals(user_id, eaten_at)`,
		`CREATE TABLE IF NOT EXISTS meal_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			meal_id INTEGER NOT NULL REFERENCES meals(id) ON DELETE CASCADE,
			food_id INTEGER REFERENCES foods(id),
			label TEXT NOT NULL,
			quantity REAL NOT NULL DEFAULT 1 CHECK (quantity > 0),
			calories REAL NOT NULL CHECK (calories >= 0),
			protein REAL NOT NULL DEFAULT 0 CHECK (protein >= 0),
			carbs REAL NOT NULL DEFAULT 0 CHECK (carbs >= 0),
			fat REAL NOT NULL DEFAULT 0 CHECK (fat >= 0),
			position INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_meal_items_meal ON meal_items(meal_id, position)`,

		// -- Fitness: fasts. ended_at NULL = active fast; one active fast per
		// -- user is enforced by the store (ErrConflict).
		`CREATE TABLE IF NOT EXISTS fasts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			started_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
			ended_at TEXT,
			target_hours REAL CHECK (target_hours IS NULL OR target_hours > 0),
			notes TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
			CHECK (ended_at IS NULL OR ended_at >= started_at)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_fasts_user_start ON fasts(user_id, started_at)`,
		`CREATE INDEX IF NOT EXISTS idx_fasts_active ON fasts(user_id) WHERE ended_at IS NULL`,

		// -- Fitness: exercise catalog. type is 'weighted' (weight x reps)
		// -- or 'timed' (duration + optional distance).
		`CREATE TABLE IF NOT EXISTS exercises (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL CHECK (type IN ('weighted', 'timed')),
			muscles TEXT NOT NULL DEFAULT '[]',
			equipment TEXT NOT NULL DEFAULT '',
			is_system INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		)`,

		`CREATE TABLE IF NOT EXISTS workouts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name TEXT NOT NULL DEFAULT '',
			started_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
			ended_at TEXT,
			effort INTEGER CHECK (effort IS NULL OR effort BETWEEN 1 AND 10),
			notes TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
			CHECK (ended_at IS NULL OR ended_at >= started_at)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_workouts_user_start ON workouts(user_id, started_at)`,
		`CREATE TABLE IF NOT EXISTS workout_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			workout_id INTEGER NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
			exercise_id INTEGER NOT NULL REFERENCES exercises(id),
			weight_kg REAL CHECK (weight_kg IS NULL OR weight_kg >= 0),
			reps INTEGER CHECK (reps IS NULL OR reps >= 0),
			duration_sec REAL CHECK (duration_sec IS NULL OR duration_sec > 0),
			distance_km REAL CHECK (distance_km IS NULL OR distance_km >= 0),
			effort INTEGER CHECK (effort IS NULL OR effort BETWEEN 1 AND 10),
			notes TEXT NOT NULL DEFAULT '',
			position INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_workout_entries_workout ON workout_entries(workout_id, position)`,

		// API tokens for bearer authentication. The stored hash is
		// SHA-256 of the raw token (tokens are high-entropy random
		// values, so a fast hash is appropriate here, unlike bcrypt
		// for human passwords). expires_at NULL = never expires.
		`CREATE TABLE IF NOT EXISTS api_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			expires_at TEXT,
			last_used_at TEXT,
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_api_tokens_user ON api_tokens(user_id)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}
