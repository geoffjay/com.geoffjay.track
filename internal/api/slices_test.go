package api_test

import (
	"fmt"

	"testing"
)

// TestAPIMealsFoods covers the foods catalog + meals slice: ad-hoc vs
// catalog items, nutrition denormalization, totals, item replacement,
// portion validation, and cross-user isolation.
func TestAPIMealsFoods(t *testing.T) {
	ts, call := apiFixture(t)

	// --- foods catalog ---
	code, body, _ := call("GET", "/api/v1/foods", "geoff", "pw", nil)
	if code != 200 {
		t.Fatalf("list foods: %d", code)
	}
	foods := body["data"].([]any)
	if len(foods) == 0 {
		t.Fatal("expected seeded foods")
	}
	var chickenID, riceID float64
	for _, f := range foods {
		ff := f.(map[string]any)
		switch ff["name"] {
		case "chicken_breast":
			chickenID = ff["id"].(float64)
		case "white_rice_cooked":
			riceID = ff["id"].(float64)
		}
	}
	if chickenID == 0 || riceID == 0 {
		t.Fatal("seeded foods missing")
	}

	// Custom food.
	code, f, _ := call("POST", "/api/v1/foods", "geoff", "pw", map[string]any{
		"name": "protein_bar_x", "serving_size": 60, "serving_unit": "g",
		"calories": 210, "protein": 20, "carbs": 22, "fat": 7,
	})
	if code != 201 || f["is_system"] != false {
		t.Fatalf("create food: %d %v", code, f)
	}
	foodID := f["id"].(float64)
	code, _, _ = call("POST", "/api/v1/foods", "geoff", "pw", map[string]any{
		"name": "protein_bar_x", "serving_size": 60, "serving_unit": "g", "calories": 210,
	})
	if code != 409 {
		t.Fatalf("duplicate food: %d, want 409", code)
	}
	// System food immutable.
	code, _, _ = call("PATCH", fmt.Sprintf("/api/v1/foods/%d", int64(chickenID)), "geoff", "pw", map[string]any{
		"calories": 0,
	})
	if code != 403 {
		t.Fatalf("patch system food: %d, want 403", code)
	}
	code, _, _ = call("PATCH", fmt.Sprintf("/api/v1/foods/%d", int64(foodID)), "geoff", "pw", map[string]any{
		"name": "protein_bar_x2", "serving_size": 60, "serving_unit": "g", "calories": 200, "protein": 20, "carbs": 22, "fat": 7,
	})
	if code != 200 {
		t.Fatalf("patch custom food: %d", code)
	}

	// --- meal: mixed catalog + ad-hoc items ---
	// chicken 150g = 165 * 150/100 = 247.5 kcal; rice 200g = 130 * 200/100 = 260 kcal.
	code, meal, _ := call("POST", "/api/v1/meals", "geoff", "pw", map[string]any{
		"name": "lunch", "meal_type": "lunch", "eaten_at": "2026-09-30T12:30:00Z",
		"items": []any{
			map[string]any{"food_id": chickenID, "quantity": 150},
			map[string]any{"food_id": riceID, "quantity": 200},
			map[string]any{"label": "olive oil drizzle", "calories": 40, "fat": 4.5},
		},
	})
	if code != 201 {
		t.Fatalf("create meal: %d %v", code, meal)
	}
	if got := meal["calories"].(float64); got != 247.5+260+40 {
		t.Fatalf("meal calories: want %.1f, got %v", 247.5+260+40, meal["calories"])
	}
	wantProtein := 31.0*1.5 + 2.7*2.0 + 0
	if got := meal["protein"].(float64); got < wantProtein-0.01 || got > wantProtein+0.01 {
		t.Fatalf("meal protein: want ~%.2f, got %v", wantProtein, meal["protein"])
	}
	items := meal["items"].([]any)
	if len(items) != 3 || items[0].(map[string]any)["position"] != float64(0) {
		t.Fatalf("items wrong: %v", items)
	}
	mealID := int64(meal["id"].(float64))

	// Ad-hoc item with no nutrition at all -> 400.
	code, e, _ := call("POST", "/api/v1/meals", "geoff", "pw", map[string]any{
		"items": []any{map[string]any{"label": "mystery"}},
	})
	if code != 400 {
		t.Fatalf("empty item: %d %v", code, e)
	}
	// Unknown food -> 400.
	code, _, _ = call("POST", "/api/v1/meals", "geoff", "pw", map[string]any{
		"items": []any{map[string]any{"food_id": 99999, "quantity": 1}},
	})
	if code != 400 {
		t.Fatalf("unknown food: %d", code)
	}
	// Bad portion -> 400.
	code, _, _ = call("POST", "/api/v1/meals", "geoff", "pw", map[string]any{
		"portion": "huge", "items": []any{map[string]any{"label": "x", "calories": 100}},
	})
	if code != 400 {
		t.Fatalf("bad portion: %d", code)
	}

	// Get + items subresource.
	code, meal2, _ := call("GET", fmt.Sprintf("/api/v1/meals/%d", mealID), "geoff", "pw", nil)
	if code != 200 || meal2["calories"] != meal["calories"] {
		t.Fatalf("get meal: %d", code)
	}
	code, ilist, _ := call("GET", fmt.Sprintf("/api/v1/meals/%d/items", mealID), "geoff", "pw", nil)
	if code != 200 || len(ilist["data"].([]any)) != 3 {
		t.Fatalf("meal items: %d", code)
	}

	// Update meal metadata only (items absent -> kept).
	code, meal3, _ := call("PATCH", fmt.Sprintf("/api/v1/meals/%d", mealID), "geoff", "pw", map[string]any{
		"name": "big lunch", "notes": "post-workout",
	})
	if code != 200 || meal3["name"] != "big lunch" {
		t.Fatalf("patch meal: %d %v", code, meal3)
	}
	if len(meal3["items"].([]any)) != 3 {
		t.Fatal("patch should keep items when absent")
	}

	// Replace items.
	code, meal4, _ := call("PATCH", fmt.Sprintf("/api/v1/meals/%d", mealID), "geoff", "pw", map[string]any{
		"items": []any{map[string]any{"label": "just coffee", "calories": 5}},
	})
	if code != 200 || len(meal4["items"].([]any)) != 1 || meal4["calories"].(float64) != 5 {
		t.Fatalf("replace items: %d %v", code, meal4)
	}

	// List with time bounds.
	code, l, _ := call("GET", "/api/v1/meals?from=2026-09-30T00:00:00Z&to=2026-09-30T23:59:59Z", "geoff", "pw", nil)
	if code != 200 || len(l["data"].([]any)) != 1 {
		t.Fatalf("list meals: %d", code)
	}
	code, l, _ = call("GET", "/api/v1/meals?from=2020-01-01&to=2020-12-31", "geoff", "pw", nil)
	if code != 200 || len(l["data"].([]any)) != 0 {
		t.Fatalf("list meals empty range: %d", code)
	}

	// Cross-user isolation.
	code, _, _ = call("GET", fmt.Sprintf("/api/v1/meals/%d", mealID), "misty", "pw", nil)
	if code != 404 {
		t.Fatalf("cross-user meal: %d, want 404", code)
	}

	// Delete.
	code, _, _ = call("DELETE", fmt.Sprintf("/api/v1/meals/%d", mealID), "geoff", "pw", nil)
	if code != 200 {
		t.Fatalf("delete meal: %d", code)
	}
	code, _, _ = call("GET", fmt.Sprintf("/api/v1/meals/%d", mealID), "geoff", "pw", nil)
	if code != 404 {
		t.Fatal("deleted meal should 404")
	}

	_ = ts
}

// TestAPIFasts covers the fasts slice: start/active/end lifecycle, conflict
// rules, duration computation, and cross-user isolation.
func TestAPIFasts(t *testing.T) {
	_, call := apiFixture(t)

	// No active fast yet -> 404.
	code, _, _ := call("GET", "/api/v1/fasts/active", "geoff", "pw", nil)
	if code != 404 {
		t.Fatalf("no active fast: %d, want 404", code)
	}

	// Start one.
	code, f, _ := call("POST", "/api/v1/fasts", "geoff", "pw", map[string]any{
		"started_at": "2026-09-30T20:00:00Z", "target_hours": 16, "notes": "evening fast",
	})
	if code != 201 || f["ended_at"] != nil {
		t.Fatalf("start fast: %d %v", code, f)
	}
	fastID := int64(f["id"].(float64))
	if f["duration_hours"] == nil {
		t.Fatal("active fast should carry duration_hours")
	}

	// Second active fast -> 409.
	code, _, _ = call("POST", "/api/v1/fasts", "geoff", "pw", map[string]any{})
	if code != 409 {
		t.Fatalf("second active fast: %d, want 409", code)
	}

	// Active lookup.
	code, f2, _ := call("GET", "/api/v1/fasts/active", "geoff", "pw", nil)
	if code != 200 || int64(f2["id"].(float64)) != fastID {
		t.Fatalf("active fast: %d %v", code, f2)
	}

	// Other user unaffected.
	code, _, _ = call("GET", "/api/v1/fasts/active", "misty", "pw", nil)
	if code != 404 {
		t.Fatalf("misty active fast: %d, want 404", code)
	}

	// End it at 12h.
	code, f3, _ := call("POST", fmt.Sprintf("/api/v1/fasts/%d/end", fastID), "geoff", "pw", map[string]any{
		"ended_at": "2026-09-30T08:00:00Z", // wait: started 20:00 on 09-30; ended must be after start
	})
	// started_at 2026-09-30T20:00Z, ended 2026-09-30T08:00Z is BEFORE start -> 400.
	if code != 400 {
		t.Fatalf("end before start should 400: %d %v", code, f3)
	}
	code, f3, _ = call("POST", fmt.Sprintf("/api/v1/fasts/%d/end", fastID), "geoff", "pw", map[string]any{
		"ended_at": "2026-10-01T12:00:00Z",
	})
	if code != 200 {
		t.Fatalf("end fast: %d %v", code, f3)
	}
	// 2026-09-30T20:00 -> 2026-10-01T12:00 = 16h.
	if got := f3["duration_hours"].(float64); got != 16.0 {
		t.Fatalf("duration: want 16, got %v", got)
	}

	// Double end -> 409.
	code, _, _ = call("POST", fmt.Sprintf("/api/v1/fasts/%d/end", fastID), "geoff", "pw", nil)
	if code != 409 {
		t.Fatalf("double end: %d, want 409", code)
	}

	// Active is gone again.
	code, _, _ = call("GET", "/api/v1/fasts/active", "geoff", "pw", nil)
	if code != 404 {
		t.Fatalf("active after end: %d, want 404", code)
	}

	// Patch notes (absent fields keep values).
	code, f4, _ := call("PATCH", fmt.Sprintf("/api/v1/fasts/%d", fastID), "geoff", "pw", map[string]any{
		"notes": "felt great",
	})
	if code != 200 || f4["notes"] != "felt great" || f4["target_hours"].(float64) != 16 {
		t.Fatalf("patch fast: %d %v", code, f4)
	}

	// List.
	code, l, _ := call("GET", "/api/v1/fasts", "geoff", "pw", nil)
	if code != 200 || len(l["data"].([]any)) != 1 {
		t.Fatalf("list fasts: %d", code)
	}

	// Cross-user isolation.
	code, _, _ = call("GET", fmt.Sprintf("/api/v1/fasts/%d", fastID), "misty", "pw", nil)
	if code != 404 {
		t.Fatalf("cross-user fast: %d, want 404", code)
	}

	// Delete.
	code, _, _ = call("DELETE", fmt.Sprintf("/api/v1/fasts/%d", fastID), "geoff", "pw", nil)
	if code != 200 {
		t.Fatalf("delete fast: %d", code)
	}
}

// TestAPIWorkouts covers the exercises catalog + workouts slice: entry shape
// validation per exercise type, finish lifecycle, and cross-user isolation.
func TestAPIWorkouts(t *testing.T) {
	_, call := apiFixture(t)

	// --- exercise catalog ---
	code, body, _ := call("GET", "/api/v1/exercises", "geoff", "pw", nil)
	if code != 200 {
		t.Fatalf("list exercises: %d", code)
	}
	var squatID, rowID float64
	for _, e := range body["data"].([]any) {
		ee := e.(map[string]any)
		switch ee["name"] {
		case "back_squat":
			squatID = ee["id"].(float64)
		case "row":
			rowID = ee["id"].(float64)
		}
	}
	if squatID == 0 || rowID == 0 {
		t.Fatal("seeded exercises missing")
	}

	// Custom exercise + conflict + system immutable.
	code, e, _ := call("POST", "/api/v1/exercises", "geoff", "pw", map[string]any{
		"name": "kettlebell_swing", "type": "weighted",
	})
	if code != 201 {
		t.Fatalf("create exercise: %d", code)
	}
	customID := int64(e["id"].(float64))
	code, _, _ = call("POST", "/api/v1/exercises", "geoff", "pw", map[string]any{"name": "kettlebell_swing", "type": "weighted"})
	if code != 409 {
		t.Fatalf("duplicate exercise: %d, want 409", code)
	}
	code, _, _ = call("POST", "/api/v1/exercises", "geoff", "pw", map[string]any{"name": "odd_thing", "type": "strange"})
	if code != 400 {
		t.Fatalf("bad type: %d, want 400", code)
	}
	code, _, _ = call("DELETE", fmt.Sprintf("/api/v1/exercises/%d", int64(squatID)), "geoff", "pw", nil)
	if code != 403 {
		t.Fatalf("delete system exercise: %d, want 403", code)
	}

	// --- workout with mixed entries ---
	code, w, _ := call("POST", "/api/v1/workouts", "geoff", "pw", map[string]any{
		"name": "leg + cardio", "started_at": "2026-09-30T17:00:00Z", "effort": 8,
		"entries": []any{
			map[string]any{"exercise_id": squatID, "weight_kg": 100, "reps": 5, "effort": 9},
			map[string]any{"exercise_id": rowID, "duration_sec": 1800, "distance_km": 6.5},
		},
	})
	if code != 201 {
		t.Fatalf("create workout: %d %v", code, w)
	}
	wid := int64(w["id"].(float64))
	entries := w["entries"].([]any)
	if len(entries) != 2 {
		t.Fatalf("entries: %v", entries)
	}
	e0 := entries[0].(map[string]any)
	if e0["exercise_name"] != "back_squat" || e0["weight_kg"].(float64) != 100 || e0["reps"] != float64(5) {
		t.Fatalf("weighted entry wrong: %v", e0)
	}
	e1 := entries[1].(map[string]any)
	if e1["distance_km"].(float64) != 6.5 || e1["duration_sec"].(float64) != 1800 {
		t.Fatalf("timed entry wrong: %v", e1)
	}

	// Shape violations: weighted without reps; timed without duration.
	code, _, _ = call("POST", "/api/v1/workouts", "geoff", "pw", map[string]any{
		"entries": []any{map[string]any{"exercise_id": squatID, "weight_kg": 100}},
	})
	if code != 400 {
		t.Fatalf("weighted without reps: %d, want 400", code)
	}
	code, _, _ = call("POST", "/api/v1/workouts", "geoff", "pw", map[string]any{
		"entries": []any{map[string]any{"exercise_id": rowID, "distance_km": 5}},
	})
	if code != 400 {
		t.Fatalf("timed without duration: %d, want 400", code)
	}
	code, _, _ = call("POST", "/api/v1/workouts", "geoff", "pw", map[string]any{
		"entries": []any{map[string]any{"exercise_id": 99999, "weight_kg": 10, "reps": 10}},
	})
	if code != 400 {
		t.Fatalf("unknown exercise: %d, want 400", code)
	}
	// Bad workout effort.
	code, _, _ = call("POST", "/api/v1/workouts", "geoff", "pw", map[string]any{
		"effort": 11, "entries": []any{map[string]any{"exercise_id": squatID, "weight_kg": 10, "reps": 10}},
	})
	if code != 400 {
		t.Fatalf("effort 11: %d, want 400", code)
	}

	// Finish.
	code, w2, _ := call("POST", fmt.Sprintf("/api/v1/workouts/%d/finish", wid), "geoff", "pw", map[string]any{
		"ended_at": "2026-09-30T18:30:00Z",
	})
	if code != 200 || w2["duration_min"].(float64) != 90 {
		t.Fatalf("finish workout: %d %v", code, w2)
	}
	code, _, _ = call("POST", fmt.Sprintf("/api/v1/workouts/%d/finish", wid), "geoff", "pw", nil)
	if code != 409 {
		t.Fatalf("double finish: %d, want 409", code)
	}

	// Update keep vs replace entries.
	code, w3, _ := call("PATCH", fmt.Sprintf("/api/v1/workouts/%d", wid), "geoff", "pw", map[string]any{
		"name": "renamed session",
	})
	if code != 200 || len(w3["entries"].([]any)) != 2 || w3["name"] != "renamed session" {
		t.Fatalf("patch keep entries: %d %v", code, w3)
	}
	code, w4, _ := call("PATCH", fmt.Sprintf("/api/v1/workouts/%d", wid), "geoff", "pw", map[string]any{
		"entries": []any{map[string]any{"exercise_id": squatID, "weight_kg": 105, "reps": 3}},
	})
	if code != 200 || len(w4["entries"].([]any)) != 1 {
		t.Fatalf("patch replace entries: %d %v", code, w4)
	}

	// List + cross-user + entries subresource (before adding the second workout).
	code, l, _ := call("GET", "/api/v1/workouts", "geoff", "pw", nil)
	if code != 200 || len(l["data"].([]any)) != 1 {
		t.Fatalf("list workouts: %d", code)
	}
	code, _, _ = call("GET", fmt.Sprintf("/api/v1/workouts/%d", wid), "misty", "pw", nil)
	if code != 404 {
		t.Fatalf("cross-user workout: %d, want 404", code)
	}
	code, el, _ := call("GET", fmt.Sprintf("/api/v1/workouts/%d/entries", wid), "geoff", "pw", nil)
	if code != 200 || len(el["data"].([]any)) != 1 {
		t.Fatalf("entries list: %d", code)
	}

	// Referenced custom exercise is not deletable: create one and log it.
	code, e2, _ := call("POST", "/api/v1/exercises", "geoff", "pw", map[string]any{
		"name": "hex_bar_deadlift", "type": "weighted",
	})
	if code != 201 {
		t.Fatalf("create referencing exercise: %d", code)
	}
	usedID := int64(e2["id"].(float64))
	code, w5, _ := call("POST", "/api/v1/workouts", "geoff", "pw", map[string]any{
		"entries": []any{map[string]any{"exercise_id": usedID, "weight_kg": 80, "reps": 6}},
	})
	if code != 201 {
		t.Fatalf("create referencing workout: %d", code)
	}
	refWID := int64(w5["id"].(float64))
	code, _, _ = call("DELETE", fmt.Sprintf("/api/v1/exercises/%d", usedID), "geoff", "pw", nil)
	if code != 409 {
		t.Fatalf("delete referenced exercise: %d, want 409", code)
	}

	// Delete the referencing workout releases the exercise.
	code, _, _ = call("DELETE", fmt.Sprintf("/api/v1/workouts/%d", refWID), "geoff", "pw", nil)
	if code != 200 {
		t.Fatalf("delete referencing workout: %d", code)
	}
	code, _, _ = call("DELETE", fmt.Sprintf("/api/v1/exercises/%d", usedID), "geoff", "pw", nil)
	if code != 200 {
		t.Fatalf("delete exercise after workout delete: %d", code)
	}

	// Now unreferenced; the first custom exercise is deletable too.
	code, _, _ = call("DELETE", fmt.Sprintf("/api/v1/exercises/%d", customID), "geoff", "pw", nil)
	if code != 200 {
		t.Fatalf("delete custom exercise after workout delete: %d", code)
	}
}
