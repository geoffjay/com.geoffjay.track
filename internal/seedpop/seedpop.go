// Package seedpop populates reference (catalog) tables at boot. Seeds are
// idempotent: rows are inserted only when missing, so user data and edits
// are safe. System rows are never modified once created.
package seedpop

import (
	"context"
	"database/sql"
	"fmt"
)

// Metric is a seed row for the metrics table.
type Metric struct {
	Name     string // unique slug, e.g. "weight"
	Label    string // display label, e.g. "Weight"
	Unit     string // e.g. "kg", "cm"
	Category string // e.g. "body"
}

// Exercise is a seed row for the exercises table.
type Exercise struct {
	Name      string
	Type      string // "weighted" | "timed"
	Muscles   string // JSON array text, e.g. `["quads","glutes"]`
	Equipment string
}

// Food is a seed row for the foods table.
type Food struct {
	Name        string
	Brand       string
	ServingSize float64
	ServingUnit string
	Calories    float64
	Protein     float64
	Carbs       float64
	Fat         float64
}

// Metrics seeds the body-metric catalog.
var Metrics = []Metric{
	{Name: "weight", Label: "Weight", Unit: "kg", Category: "body"},
	{Name: "height", Label: "Height", Unit: "cm", Category: "body"},
	{Name: "waist", Label: "Waist", Unit: "cm", Category: "body"},
	{Name: "hips", Label: "Hips", Unit: "cm", Category: "body"},
	{Name: "chest", Label: "Chest", Unit: "cm", Category: "body"},
	{Name: "bicep", Label: "Bicep", Unit: "cm", Category: "body"},
	{Name: "forearm", Label: "Forearm", Unit: "cm", Category: "body"},
	{Name: "thigh", Label: "Thigh", Unit: "cm", Category: "body"},
	{Name: "quadricep", Label: "Quadricep", Unit: "cm", Category: "body"},
	{Name: "calf", Label: "Calf", Unit: "cm", Category: "body"},
	{Name: "neck", Label: "Neck", Unit: "cm", Category: "body"},
	{Name: "body_fat", Label: "Body Fat", Unit: "%", Category: "body"},
	{Name: "resting_hr", Label: "Resting Heart Rate", Unit: "bpm", Category: "vitals"},
	{Name: "blood_pressure_sys", Label: "Systolic BP", Unit: "mmHg", Category: "vitals"},
	{Name: "blood_pressure_dia", Label: "Diastolic BP", Unit: "mmHg", Category: "vitals"},
}

// Exercises seeds the exercise catalog: common weighted and timed lifts and
// cardio movements.
var Exercises = []Exercise{
	// Weighted.
	{Name: "back_squat", Type: "weighted", Muscles: `["quads","glutes","core"]`, Equipment: "barbell"},
	{Name: "front_squat", Type: "weighted", Muscles: `["quads","glutes","core"]`, Equipment: "barbell"},
	{Name: "deadlift", Type: "weighted", Muscles: `["hamstrings","glutes","back"]`, Equipment: "barbell"},
	{Name: "romanian_deadlift", Type: "weighted", Muscles: `["hamstrings","glutes"]`, Equipment: "barbell"},
	{Name: "bench_press", Type: "weighted", Muscles: `["chest","triceps","shoulders"]`, Equipment: "barbell"},
	{Name: "incline_bench_press", Type: "weighted", Muscles: `["chest","shoulders","triceps"]`, Equipment: "barbell"},
	{Name: "overhead_press", Type: "weighted", Muscles: `["shoulders","triceps"]`, Equipment: "barbell"},
	{Name: "bicep_curl", Type: "weighted", Muscles: `["biceps"]`, Equipment: "dumbbell"},
	{Name: "hammer_curl", Type: "weighted", Muscles: `["biceps","forearms"]`, Equipment: "dumbbell"},
	{Name: "tricep_pushdown", Type: "weighted", Muscles: `["triceps"]`, Equipment: "cable"},
	{Name: "lat_pulldown", Type: "weighted", Muscles: `["lats","biceps"]`, Equipment: "cable"},
	{Name: "pull_up", Type: "weighted", Muscles: `["lats","biceps","core"]`, Equipment: "bodyweight"},
	{Name: "bent_over_row", Type: "weighted", Muscles: `["back","lats","biceps"]`, Equipment: "barbell"},
	{Name: "lunge", Type: "weighted", Muscles: `["quads","glutes"]`, Equipment: "dumbbell"},
	{Name: "leg_press", Type: "weighted", Muscles: `["quads","glutes"]`, Equipment: "machine"},
	{Name: "leg_curl", Type: "weighted", Muscles: `["hamstrings"]`, Equipment: "machine"},
	{Name: "leg_extension", Type: "weighted", Muscles: `["quads"]`, Equipment: "machine"},
	{Name: "calf_raise", Type: "weighted", Muscles: `["calves"]`, Equipment: "machine"},
	{Name: "lateral_raise", Type: "weighted", Muscles: `["shoulders"]`, Equipment: "dumbbell"},
	{Name: "face_pull", Type: "weighted", Muscles: `["rear delts","upper back"]`, Equipment: "cable"},
	{Name: "hip_thrust", Type: "weighted", Muscles: `["glutes","hamstrings"]`, Equipment: "barbell"},
	{Name: "dumbbell_shrug", Type: "weighted", Muscles: `["traps"]`, Equipment: "dumbbell"},
	{Name: "chest_fly", Type: "weighted", Muscles: `["chest"]`, Equipment: "dumbbell"},
	{Name: "plank", Type: "weighted", Muscles: `["core"]`, Equipment: "bodyweight"},

	// Timed.
	{Name: "row", Type: "timed", Muscles: `["back","legs","cardio"]`, Equipment: "rowing machine"},
	{Name: "jog", Type: "timed", Muscles: `["legs","cardio"]`, Equipment: "none"},
	{Name: "run", Type: "timed", Muscles: `["legs","cardio"]`, Equipment: "none"},
	{Name: "bike", Type: "timed", Muscles: `["legs","cardio"]`, Equipment: "stationary bike"},
	{Name: "elliptical", Type: "timed", Muscles: `["legs","cardio"]`, Equipment: "machine"},
	{Name: "swim", Type: "timed", Muscles: `["full body","cardio"]`, Equipment: "none"},
	{Name: "stair_climber", Type: "timed", Muscles: `["legs","cardio"]`, Equipment: "machine"},
	{Name: "jump_rope", Type: "timed", Muscles: `["calves","cardio"]`, Equipment: "jump rope"},
	{Name: "treadmill", Type: "timed", Muscles: `["legs","cardio"]`, Equipment: "machine"},
	{Name: "brisk_walk", Type: "timed", Muscles: `["legs","cardio"]`, Equipment: "none"},
}

// Foods seeds a small starter food catalog (per stated serving).
var Foods = []Food{
	{Name: "chicken_breast", ServingSize: 100, ServingUnit: "g", Calories: 165, Protein: 31, Carbs: 0, Fat: 3.6},
	{Name: "white_rice_cooked", ServingSize: 100, ServingUnit: "g", Calories: 130, Protein: 2.7, Carbs: 28, Fat: 0.3},
	{Name: "brown_rice_cooked", ServingSize: 100, ServingUnit: "g", Calories: 112, Protein: 2.6, Carbs: 24, Fat: 0.9},
	{Name: "whole_egg", ServingSize: 1, ServingUnit: "egg", Calories: 72, Protein: 6.3, Carbs: 0.4, Fat: 4.8},
	{Name: "greek_yogurt", ServingSize: 170, ServingUnit: "g", Calories: 100, Protein: 17, Carbs: 6, Fat: 0.7},
	{Name: "oats_dry", ServingSize: 40, ServingUnit: "g", Calories: 156, Protein: 5.4, Carbs: 27, Fat: 2.8},
	{Name: "banana", ServingSize: 1, ServingUnit: "medium", Calories: 105, Protein: 1.3, Carbs: 27, Fat: 0.4},
	{Name: "apple", ServingSize: 1, ServingUnit: "medium", Calories: 95, Protein: 0.5, Carbs: 25, Fat: 0.3},
	{Name: "broccoli", ServingSize: 100, ServingUnit: "g", Calories: 34, Protein: 2.8, Carbs: 7, Fat: 0.4},
	{Name: "sweet_potato", ServingSize: 100, ServingUnit: "g", Calories: 86, Protein: 1.6, Carbs: 20, Fat: 0.1},
	{Name: "salmon_fillet", ServingSize: 100, ServingUnit: "g", Calories: 208, Protein: 20, Carbs: 0, Fat: 13},
	{Name: "ground_beef_85", ServingSize: 100, ServingUnit: "g", Calories: 250, Protein: 26, Carbs: 0, Fat: 15},
	{Name: "whole_milk", ServingSize: 240, ServingUnit: "ml", Calories: 149, Protein: 7.7, Carbs: 12, Fat: 8},
	{Name: "almonds", ServingSize: 28, ServingUnit: "g", Calories: 164, Protein: 6, Carbs: 6, Fat: 14},
	{Name: "peanut_butter", ServingSize: 32, ServingUnit: "g", Calories: 188, Protein: 8, Carbs: 6, Fat: 16},
	{Name: "olive_oil", ServingSize: 14, ServingUnit: "g", Calories: 119, Protein: 0, Carbs: 0, Fat: 13.5},
	{Name: "whey_protein_scoop", ServingSize: 30, ServingUnit: "g", Calories: 120, Protein: 24, Carbs: 3, Fat: 1.5},
	{Name: "black_beans", ServingSize: 100, ServingUnit: "g", Calories: 132, Protein: 8.9, Carbs: 24, Fat: 0.5},
	{Name: "avocado", ServingSize: 100, ServingUnit: "g", Calories: 160, Protein: 2, Carbs: 9, Fat: 15},
	{Name: "bread_slice", ServingSize: 1, ServingUnit: "slice", Calories: 80, Protein: 4, Carbs: 14, Fat: 1},
}

// Ensure runs all catalog seeds. Call once after db.Open.
func Ensure(ctx context.Context, dbh *sql.DB) error {
	if err := ensureMetrics(ctx, dbh); err != nil {
		return fmt.Errorf("seed metrics: %w", err)
	}
	if err := ensureExercises(ctx, dbh); err != nil {
		return fmt.Errorf("seed exercises: %w", err)
	}
	if err := ensureFoods(ctx, dbh); err != nil {
		return fmt.Errorf("seed foods: %w", err)
	}
	return nil
}

// ensureMetrics inserts metric definitions that are missing by name.
func ensureMetrics(ctx context.Context, dbh *sql.DB) error {
	const q = `INSERT INTO metrics (name, label, unit, category, is_system)
		SELECT ?, ?, ?, ?, 1
		WHERE NOT EXISTS (SELECT 1 FROM metrics WHERE name = ?)`
	for _, m := range Metrics {
		if _, err := dbh.ExecContext(ctx, q, m.Name, m.Label, m.Unit, m.Category, m.Name); err != nil {
			return err
		}
	}
	return nil
}

// ensureExercises inserts exercise definitions that are missing by name.
func ensureExercises(ctx context.Context, dbh *sql.DB) error {
	const q = `INSERT INTO exercises (name, type, muscles, equipment, is_system)
		SELECT ?, ?, ?, ?, 1
		WHERE NOT EXISTS (SELECT 1 FROM exercises WHERE name = ?)`
	for _, e := range Exercises {
		if _, err := dbh.ExecContext(ctx, q, e.Name, e.Type, e.Muscles, e.Equipment, e.Name); err != nil {
			return err
		}
	}
	return nil
}

// ensureFoods inserts food definitions that are missing by name.
func ensureFoods(ctx context.Context, dbh *sql.DB) error {
	const q = `INSERT INTO foods (name, brand, serving_size, serving_unit, calories, protein, carbs, fat, is_system)
		SELECT ?, ?, ?, ?, ?, ?, ?, ?, 1
		WHERE NOT EXISTS (SELECT 1 FROM foods WHERE name = ?)`
	for _, f := range Foods {
		if _, err := dbh.ExecContext(ctx, q, f.Name, f.Brand, f.ServingSize, f.ServingUnit, f.Calories, f.Protein, f.Carbs, f.Fat, f.Name); err != nil {
			return err
		}
	}
	return nil
}
