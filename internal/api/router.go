package api

import (
	"database/sql"

	"com.geoffjay.track/internal/fitness"

	"github.com/gin-gonic/gin"
)

// Handlers carries the stores backing the API. Each domain slice
// (measurements, meals, fasts, workouts) registers its routes against this
// one struct.
type Handlers struct {
	Metrics      *fitness.MetricStore
	Measurements *fitness.MeasurementStore
	Foods        *fitness.FoodStore
	Meals        *fitness.MealStore
	Fasts        *fitness.FastStore
	Exercises    *fitness.ExerciseStore
	Workouts     *fitness.WorkoutStore
}

// NewHandlers builds every store over dbh.
func NewHandlers(dbh *sql.DB) *Handlers {
	return &Handlers{
		Metrics:      fitness.NewMetricStore(dbh),
		Measurements: fitness.NewMeasurementStore(dbh),
		Foods:        fitness.NewFoodStore(dbh),
		Meals:        fitness.NewMealStore(dbh),
		Fasts:        fitness.NewFastStore(dbh),
		Exercises:    fitness.NewExerciseStore(dbh),
		Workouts:     fitness.NewWorkoutStore(dbh),
	}
}

// Register wires every domain slice's routes onto an existing router group.
// The caller owns middleware (basic auth), keeping a single gin engine for
// UI + API.
func Register(rg *gin.RouterGroup, h *Handlers) {
	measurementRoutes(rg, h)
	mealRoutes(rg, h)
	fastRoutes(rg, h)
	workoutRoutes(rg, h)
}
