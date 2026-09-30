package api

import (
	"net/http"
	"time"

	"com.geoffjay.track/internal/fitness"

	"github.com/gin-gonic/gin"
)

// workoutRoutes wires the exercises + workouts CRUD under rg.
func workoutRoutes(rg *gin.RouterGroup, h *Handlers) {
	// Exercise catalog.
	rg.GET("/exercises", h.listExercises)
	rg.POST("/exercises", h.createExercise)
	rg.GET("/exercises/:id", h.getExercise)
	rg.PATCH("/exercises/:id", h.updateExercise)
	rg.DELETE("/exercises/:id", h.deleteExercise)

	// Workout log.
	rg.GET("/workouts", h.listWorkouts)
	rg.POST("/workouts", h.createWorkout)
	rg.GET("/workouts/:id", h.getWorkout)
	rg.PATCH("/workouts/:id", h.updateWorkout)
	rg.POST("/workouts/:id/finish", h.finishWorkout)
	rg.DELETE("/workouts/:id", h.deleteWorkout)
	rg.GET("/workouts/:id/entries", h.listWorkoutEntries)
}

// ---- exercise catalog ----

type exercisePayload struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Muscles   string `json:"muscles"`
	Equipment string `json:"equipment"`
}

func (h *Handlers) listExercises(c *gin.Context) {
	es, err := h.Exercises.List(c.Request.Context())
	if err != nil {
		writeErr(c, err)
		return
	}
	listJSON(c, es)
}

func (h *Handlers) createExercise(c *gin.Context) {
	var p exercisePayload
	if err := bind(c, &p); err != nil {
		writeErr(c, err)
		return
	}
	e, err := h.Exercises.Create(c.Request.Context(), fitness.Exercise{
		Name: p.Name, Type: p.Type, Muscles: p.Muscles, Equipment: p.Equipment,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, e)
}

func (h *Handlers) getExercise(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	e, err := h.Exercises.Get(c.Request.Context(), id)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, e)
}

func (h *Handlers) updateExercise(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	var p exercisePayload
	if err := bind(c, &p); err != nil {
		writeErr(c, err)
		return
	}
	e, err := h.Exercises.Update(c.Request.Context(), id, fitness.Exercise{
		Name: p.Name, Type: p.Type, Muscles: p.Muscles, Equipment: p.Equipment,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, e)
}

func (h *Handlers) deleteExercise(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	if err := h.Exercises.Delete(c.Request.Context(), id); err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, gin.H{"deleted": true})
}

// ---- workout log ----

// workoutEntryPayload is one exercise performed within a workout. weight_kg/
// reps carry weighted exercises; duration_sec (and optional distance_km)
// carry timed ones.
type workoutEntryPayload struct {
	ExerciseID  int64    `json:"exercise_id"`
	WeightKg    *float64 `json:"weight_kg"`
	Reps        *int     `json:"reps"`
	DurationSec *float64 `json:"duration_sec"`
	DistanceKm  *float64 `json:"distance_km"`
	Effort      *int     `json:"effort"`
	Notes       string   `json:"notes"`
}

// workoutPayload is the create/update body. Every field is optional: nil
// pointers default (create: started_at -> now; update: keep the stored
// value). A nil entries slice keeps existing entries on update; a non-nil
// array replaces them wholesale.
type workoutPayload struct {
	Name      *string               `json:"name"`
	StartedAt *time.Time            `json:"started_at"`
	EndedAt   *time.Time            `json:"ended_at"`
	Effort    *int                  `json:"effort"`
	Notes     *string               `json:"notes"`
	Entries   []workoutEntryPayload `json:"entries"`
}

func (h *Handlers) listWorkouts(c *gin.Context) {
	user := mustUser(c)
	from, hasFrom, err := queryTime(c, "from")
	if err != nil {
		writeErr(c, err)
		return
	}
	to, hasTo, err := queryTime(c, "to")
	if err != nil {
		writeErr(c, err)
		return
	}
	ws, err := h.Workouts.ListByUser(c.Request.Context(), user.ID,
		orZero(from, hasFrom), orZero(to, hasTo), queryLimit(c))
	if err != nil {
		writeErr(c, err)
		return
	}
	listJSON(c, ws)
}

// entryPayloads converts API entry payloads into domain entries. A nil slice
// stays nil (the "keep existing" signal for updates).
func entryPayloads(ps []workoutEntryPayload) []fitness.WorkoutEntry {
	if ps == nil {
		return nil
	}
	out := make([]fitness.WorkoutEntry, len(ps))
	for i, p := range ps {
		out[i] = fitness.WorkoutEntry{
			ExerciseID:  p.ExerciseID,
			WeightKg:    p.WeightKg,
			Reps:        p.Reps,
			DurationSec: p.DurationSec,
			DistanceKm:  p.DistanceKm,
			Effort:      p.Effort,
			Notes:       p.Notes,
		}
	}
	return out
}

func (h *Handlers) createWorkout(c *gin.Context) {
	user := mustUser(c)
	var p workoutPayload
	if err := bind(c, &p); err != nil {
		writeErr(c, err)
		return
	}
	w := fitness.Workout{
		EndedAt: p.EndedAt,
		Effort:  p.Effort,
	}
	if p.Name != nil {
		w.Name = *p.Name
	}
	if p.StartedAt != nil {
		w.StartedAt = *p.StartedAt
	}
	if p.Notes != nil {
		w.Notes = *p.Notes
	}
	w, err := h.Workouts.Create(c.Request.Context(), user.ID, w, entryPayloads(p.Entries))
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, w)
}

func (h *Handlers) getWorkout(c *gin.Context) {
	user := mustUser(c)
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	w, err := h.Workouts.Get(c.Request.Context(), id, user.ID)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, w)
}

// updateWorkout patches one of the user's workouts. Every field is optional:
// nil pointers keep the stored value; a non-nil entries array replaces the
// existing entries wholesale.
func (h *Handlers) updateWorkout(c *gin.Context) {
	user := mustUser(c)
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	var p workoutPayload
	if err := bind(c, &p); err != nil {
		writeErr(c, err)
		return
	}
	cur, err := h.Workouts.Get(c.Request.Context(), id, user.ID)
	if err != nil {
		writeErr(c, err)
		return
	}
	if p.Name != nil {
		cur.Name = *p.Name
	}
	if p.StartedAt != nil {
		cur.StartedAt = *p.StartedAt
	}
	if p.EndedAt != nil {
		cur.EndedAt = p.EndedAt
	}
	if p.Effort != nil {
		cur.Effort = p.Effort
	}
	if p.Notes != nil {
		cur.Notes = *p.Notes
	}
	// nil entries = keep the stored ones (the store skips replacement); a
	// non-nil array replaces them.
	w, err := h.Workouts.Update(c.Request.Context(), id, user.ID, cur, entryPayloads(p.Entries))
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, w)
}

func (h *Handlers) finishWorkout(c *gin.Context) {
	user := mustUser(c)
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	var p struct {
		EndedAt *time.Time `json:"ended_at"`
	}
	if err := bind(c, &p); err != nil {
		writeErr(c, err)
		return
	}
	var endedAt time.Time
	if p.EndedAt != nil {
		endedAt = *p.EndedAt
	}
	w, err := h.Workouts.Finish(c.Request.Context(), id, user.ID, endedAt)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, w)
}

func (h *Handlers) deleteWorkout(c *gin.Context) {
	user := mustUser(c)
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	if err := h.Workouts.Delete(c.Request.Context(), id, user.ID); err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, gin.H{"deleted": true})
}

func (h *Handlers) listWorkoutEntries(c *gin.Context) {
	user := mustUser(c)
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	w, err := h.Workouts.Get(c.Request.Context(), id, user.ID)
	if err != nil {
		writeErr(c, err)
		return
	}
	listJSON(c, w.Entries)
}
