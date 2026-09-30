package fitness

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Food is a row of the shared food catalog. Nutrition is per serving_size
// serving_unit. System rows (seeds) are API-immutable.
type Food struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Brand       string    `json:"brand"`
	ServingSize float64   `json:"serving_size"`
	ServingUnit string    `json:"serving_unit"`
	Calories    float64   `json:"calories"`
	Protein     float64   `json:"protein"`
	Carbs       float64   `json:"carbs"`
	Fat         float64   `json:"fat"`
	IsSystem    bool      `json:"is_system"`
	CreatedAt   time.Time `json:"created_at"`
}

// validateFood normalizes and checks user-supplied food fields. Shared by
// Create and Update; the caller has already enforced mutability.
func validateFood(f Food) (Food, error) {
	f.Name = strings.TrimSpace(f.Name)
	f.Brand = strings.TrimSpace(f.Brand)
	f.ServingUnit = strings.TrimSpace(f.ServingUnit)
	if f.Name == "" {
		return Food{}, invalidf("name", "must not be empty")
	}
	if !finite(f.ServingSize) {
		return Food{}, invalidf("serving_size", "must be a finite number")
	}
	if f.ServingSize == 0 {
		f.ServingSize = 100
	}
	if f.ServingSize < 0 {
		return Food{}, invalidf("serving_size", "must be > 0")
	}
	if f.ServingUnit == "" {
		f.ServingUnit = "g"
	}
	for field, v := range map[string]float64{
		"calories": f.Calories, "protein": f.Protein, "carbs": f.Carbs, "fat": f.Fat,
	} {
		if !finite(v) || v < 0 {
			return Food{}, invalidf(field, "must be a finite number >= 0")
		}
	}
	return f, nil
}

// FoodStore provides food-catalog persistence.
type FoodStore struct{ db *sql.DB }

// NewFoodStore wraps db.
func NewFoodStore(db *sql.DB) *FoodStore { return &FoodStore{db: db} }

const foodCols = `id, name, brand, serving_size, serving_unit, calories, protein, carbs, fat, is_system, created_at`

func scanFood(row interface{ Scan(...any) error }) (Food, error) {
	var f Food
	var isSystem int
	var createdAt string
	if err := row.Scan(&f.ID, &f.Name, &f.Brand, &f.ServingSize, &f.ServingUnit,
		&f.Calories, &f.Protein, &f.Carbs, &f.Fat, &isSystem, &createdAt); err != nil {
		return Food{}, err
	}
	if t, err := parseTime(createdAt); err == nil {
		f.CreatedAt = t
	}
	f.IsSystem = isSystem == 1
	return f, nil
}

// List returns all foods ordered by name.
func (s *FoodStore) List(ctx context.Context) ([]Food, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+foodCols+` FROM foods ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Food
	for rows.Next() {
		f, err := scanFood(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// Get returns one food by id.
func (s *FoodStore) Get(ctx context.Context, id int64) (Food, error) {
	f, err := scanFood(s.db.QueryRowContext(ctx, `SELECT `+foodCols+` FROM foods WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return Food{}, ErrNotFound
	}
	if err != nil {
		return Food{}, err
	}
	return f, nil
}

// Create adds a custom (non-system) food definition.
func (s *FoodStore) Create(ctx context.Context, in Food) (Food, error) {
	in, err := validateFood(in)
	if err != nil {
		return Food{}, err
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO foods (name, brand, serving_size, serving_unit, calories, protein, carbs, fat, is_system)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		in.Name, in.Brand, in.ServingSize, in.ServingUnit, in.Calories, in.Protein, in.Carbs, in.Fat)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Food{}, ErrConflict
		}
		return Food{}, fmt.Errorf("insert food: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Food{}, err
	}
	return s.Get(ctx, id)
}

// Update modifies a custom food. System foods are immutable; a missing row
// is ErrNotFound.
func (s *FoodStore) Update(ctx context.Context, id int64, in Food) (Food, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return Food{}, err
	}
	if current.IsSystem {
		return Food{}, ErrForbidden
	}
	in, err = validateFood(in)
	if err != nil {
		return Food{}, err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE foods SET name = ?, brand = ?, serving_size = ?, serving_unit = ?, calories = ?, protein = ?, carbs = ?, fat = ?
		WHERE id = ? AND is_system = 0`,
		in.Name, in.Brand, in.ServingSize, in.ServingUnit, in.Calories, in.Protein, in.Carbs, in.Fat, id)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Food{}, ErrConflict
		}
		return Food{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Food{}, ErrNotFound
	}
	return s.Get(ctx, id)
}

// Delete removes a custom food. System foods are immutable. Foods referenced
// by meal items ARE deletable: items carry denormalized nutrition, so
// history survives catalog edits.
func (s *FoodStore) Delete(ctx context.Context, id int64) error {
	var isSystem int
	err := s.db.QueryRowContext(ctx, `SELECT is_system FROM foods WHERE id = ?`, id).Scan(&isSystem)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if isSystem == 1 {
		return ErrForbidden
	}
	// meal_items.food_id has no ON DELETE action, so clear stale
	// references first: items keep their denormalized nutrition.
	if _, err := s.db.ExecContext(ctx, `UPDATE meal_items SET food_id = NULL WHERE food_id = ?`, id); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM foods WHERE id = ? AND is_system = 0`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// MealItem is one line of a meal. It either references a catalog food
// (food_id set; nutrition computed at write time and denormalized so
// history survives later food edits) or carries its own numbers (ad-hoc
// entry, food_id null).
type MealItem struct {
	ID       int64   `json:"id"`
	MealID   int64   `json:"meal_id"`
	FoodID   *int64  `json:"food_id,omitempty"`
	Label    string  `json:"label"`
	Quantity float64 `json:"quantity"`
	Calories float64 `json:"calories"`
	Protein  float64 `json:"protein"`
	Carbs    float64 `json:"carbs"`
	Fat      float64 `json:"fat"`
	Position int     `json:"position"`
}

// Meal is a logged eating event with its items and computed totals.
type Meal struct {
	ID        int64      `json:"id"`
	UserID    int64      `json:"-"`
	Name      string     `json:"name"`
	MealType  string     `json:"meal_type"`
	Portion   string     `json:"portion"`
	Notes     string     `json:"notes"`
	EatenAt   time.Time  `json:"eaten_at"`
	CreatedAt time.Time  `json:"created_at"`
	Items     []MealItem `json:"items"`

	// Totals computed over Items.
	Calories float64 `json:"calories"`
	Protein  float64 `json:"protein"`
	Carbs    float64 `json:"carbs"`
	Fat      float64 `json:"fat"`
}

// computeTotals sums an item's nutrition into the meal's totals.
func (m *Meal) computeTotals() {
	for _, it := range m.Items {
		m.Calories += it.Calories
		m.Protein += it.Protein
		m.Carbs += it.Carbs
		m.Fat += it.Fat
	}
}

// MealStore provides meal persistence. Every query is scoped to a user;
// rows belonging to another user are indistinguishable from missing ones.
type MealStore struct{ db *sql.DB }

// NewMealStore wraps db.
func NewMealStore(db *sql.DB) *MealStore { return &MealStore{db: db} }

// validateMeal normalizes and checks meal-level fields.
func validateMeal(m Meal) (Meal, error) {
	m.Name = strings.TrimSpace(m.Name)
	m.MealType = strings.TrimSpace(m.MealType)
	m.Notes = strings.TrimSpace(m.Notes)
	switch m.Portion = strings.TrimSpace(m.Portion); m.Portion {
	case "", "small", "medium", "large":
	default:
		return Meal{}, invalidf("portion", `must be "" or small, medium, large`)
	}
	if m.EatenAt.IsZero() {
		m.EatenAt = time.Now().UTC()
	}
	return m, nil
}

// prepareItems validates items against the catalog and computes nutrition.
// food_id set: nutrition = food values x (quantity / food.ServingSize),
// denormalized into the item. food_id null: ad-hoc numbers are taken as
// given (defaulting to zero). Positions are assigned in slice order.
func prepareItems(q interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}, ctx context.Context, items []MealItem) ([]MealItem, error) {
	out := make([]MealItem, len(items))
	for i, it := range items {
		it.Label = strings.TrimSpace(it.Label)
		if it.Quantity == 0 {
			it.Quantity = 1
		}
		if !finite(it.Quantity) || it.Quantity <= 0 {
			return nil, invalidf("items", "quantity must be a finite number > 0")
		}
		if it.FoodID != nil {
			f, err := scanFood(q.QueryRowContext(ctx, `SELECT `+foodCols+` FROM foods WHERE id = ?`, *it.FoodID))
			if err == sql.ErrNoRows {
				return nil, invalidf("items", "unknown food %d", *it.FoodID)
			}
			if err != nil {
				return nil, err
			}
			scale := it.Quantity / f.ServingSize
			it.Calories = f.Calories * scale
			it.Protein = f.Protein * scale
			it.Carbs = f.Carbs * scale
			it.Fat = f.Fat * scale
			if it.Label == "" {
				it.Label = f.Name
			}
		} else {
			if it.Label == "" {
				return nil, invalidf("items", "label must not be empty")
			}
			for field, v := range map[string]float64{
				"calories": it.Calories, "protein": it.Protein, "carbs": it.Carbs, "fat": it.Fat,
			} {
				if !finite(v) || v < 0 {
					return nil, invalidf("items", "%s must be a finite number >= 0", field)
				}
			}
			if it.Calories == 0 && it.Protein == 0 && it.Carbs == 0 && it.Fat == 0 {
				return nil, invalidf("items", "an item must carry calories or macros")
			}
		}
		it.Position = i
		out[i] = it
	}
	return out, nil
}

// insertItems writes meal rows into meal_items with the given meal id.
func insertItems(ctx context.Context, tx *sql.Tx, mealID int64, items []MealItem) error {
	const q = `INSERT INTO meal_items (meal_id, food_id, label, quantity, calories, protein, carbs, fat, position)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	for _, it := range items {
		if _, err := tx.ExecContext(ctx, q,
			mealID, it.FoodID, it.Label, it.Quantity, it.Calories, it.Protein, it.Carbs, it.Fat, it.Position); err != nil {
			return err
		}
	}
	return nil
}

// Create records a meal with its items in one transaction. eaten_at zero
// means now; items are validated and nutrition-computed before the insert.
func (s *MealStore) Create(ctx context.Context, userID int64, meal Meal, items []MealItem) (Meal, error) {
	meal, err := validateMeal(meal)
	if err != nil {
		return Meal{}, err
	}
	prepared, err := prepareItems(s.db, ctx, items)
	if err != nil {
		return Meal{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Meal{}, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO meals (user_id, name, meal_type, portion, notes, eaten_at) VALUES (?, ?, ?, ?, ?, ?)`,
		userID, meal.Name, meal.MealType, meal.Portion, meal.Notes, formatTime(meal.EatenAt))
	if err != nil {
		return Meal{}, fmt.Errorf("insert meal: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Meal{}, err
	}
	if err := insertItems(ctx, tx, id, prepared); err != nil {
		return Meal{}, err
	}
	if err := tx.Commit(); err != nil {
		return Meal{}, err
	}
	return s.Get(ctx, id, userID)
}

// loadItems queries meal_items rows ordered by position.
func loadItems(ctx context.Context, q interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}, query string, args ...any) ([]MealItem, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MealItem{}
	for rows.Next() {
		var it MealItem
		if err := rows.Scan(&it.ID, &it.MealID, &it.FoodID, &it.Label, &it.Quantity,
			&it.Calories, &it.Protein, &it.Carbs, &it.Fat, &it.Position); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

const mealItemCols = `id, meal_id, food_id, label, quantity, calories, protein, carbs, fat, position`

// Get returns one of the user's meals with its items and totals.
func (s *MealStore) Get(ctx context.Context, id, userID int64) (Meal, error) {
	var m Meal
	var eatenAt, createdAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, meal_type, portion, notes, eaten_at, created_at
		FROM meals WHERE id = ? AND user_id = ?`, id, userID).
		Scan(&m.ID, &m.UserID, &m.Name, &m.MealType, &m.Portion, &m.Notes, &eatenAt, &createdAt)
	if err == sql.ErrNoRows {
		return Meal{}, ErrNotFound
	}
	if err != nil {
		return Meal{}, err
	}
	if t, err := parseTime(eatenAt); err == nil {
		m.EatenAt = t
	}
	if t, err := parseTime(createdAt); err == nil {
		m.CreatedAt = t
	}
	m.Items, err = loadItems(ctx, s.db,
		`SELECT `+mealItemCols+` FROM meal_items WHERE meal_id = ? ORDER BY position`, m.ID)
	if err != nil {
		return Meal{}, err
	}
	m.computeTotals()
	return m, nil
}

// ListByUser returns the user's meals newest first (by eaten_at), each with
// items loaded and totals computed. from/to (when non-zero) bound eaten_at.
func (s *MealStore) ListByUser(ctx context.Context, userID int64, from, to time.Time, limit int) ([]Meal, error) {
	q := `SELECT id, user_id, name, meal_type, portion, notes, eaten_at, created_at FROM meals WHERE user_id = ?`
	args := []any{userID}
	if !from.IsZero() {
		q += " AND eaten_at >= ?"
		args = append(args, formatTime(from))
	}
	if !to.IsZero() {
		q += " AND eaten_at <= ?"
		args = append(args, formatTime(to))
	}
	q += " ORDER BY eaten_at DESC LIMIT ?"
	args = append(args, clampLimit(limit))
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	var out []Meal
	defer rows.Close()
	for rows.Next() {
		var m Meal
		var eatenAt, createdAt string
		if err := rows.Scan(&m.ID, &m.UserID, &m.Name, &m.MealType, &m.Portion, &m.Notes, &eatenAt, &createdAt); err != nil {
			return nil, err
		}
		if t, err := parseTime(eatenAt); err == nil {
			m.EatenAt = t
		}
		if t, err := parseTime(createdAt); err == nil {
			m.CreatedAt = t
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	// Load items per meal in one pass over the fetched slice.
	for i := range out {
		items, err := loadItems(ctx, s.db,
			`SELECT `+mealItemCols+` FROM meal_items WHERE meal_id = ? ORDER BY position`, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Items = items
		out[i].computeTotals()
	}
	return out, nil
}

// Update modifies one of the user's meals. items nil leaves the existing
// items untouched; a non-nil slice replaces all items (fresh positions) in
// the same transaction as the meal row.
func (s *MealStore) Update(ctx context.Context, id, userID int64, meal Meal, items []MealItem) (Meal, error) {
	meal, err := validateMeal(meal)
	if err != nil {
		return Meal{}, err
	}
	var prepared []MealItem
	if items != nil {
		prepared, err = prepareItems(s.db, ctx, items)
		if err != nil {
			return Meal{}, err
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Meal{}, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`UPDATE meals SET name = ?, meal_type = ?, portion = ?, notes = ?, eaten_at = ? WHERE id = ? AND user_id = ?`,
		meal.Name, meal.MealType, meal.Portion, meal.Notes, formatTime(meal.EatenAt), id, userID)
	if err != nil {
		return Meal{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Meal{}, ErrNotFound
	}
	if items != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM meal_items WHERE meal_id = ?`, id); err != nil {
			return Meal{}, err
		}
		if err := insertItems(ctx, tx, id, prepared); err != nil {
			return Meal{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Meal{}, err
	}
	return s.Get(ctx, id, userID)
}

// Delete removes one of the user's meals; items cascade via FK.
func (s *MealStore) Delete(ctx context.Context, id, userID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM meals WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
