package api

import (
	"net/http"
	"time"

	"com.geoffjay.track/internal/fitness"

	"github.com/gin-gonic/gin"
)

// mealRoutes wires the food catalog + meal log CRUD under rg.
func mealRoutes(rg *gin.RouterGroup, h *Handlers) {
	// Food catalog.
	rg.GET("/foods", h.listFoods)
	rg.POST("/foods", h.createFood)
	rg.GET("/foods/:id", h.getFood)
	rg.PATCH("/foods/:id", h.updateFood)
	rg.DELETE("/foods/:id", h.deleteFood)

	// Meal log.
	rg.GET("/meals", h.listMeals)
	rg.POST("/meals", h.createMeal)
	rg.GET("/meals/:id", h.getMeal)
	rg.PATCH("/meals/:id", h.updateMeal)
	rg.DELETE("/meals/:id", h.deleteMeal)
	rg.GET("/meals/:id/items", h.listMealItems)
}

// ---- food catalog ----

type foodPayload struct {
	Name        string  `json:"name"`
	Brand       string  `json:"brand"`
	ServingSize float64 `json:"serving_size"`
	ServingUnit string  `json:"serving_unit"`
	Calories    float64 `json:"calories"`
	Protein     float64 `json:"protein"`
	Carbs       float64 `json:"carbs"`
	Fat         float64 `json:"fat"`
}

func (h *Handlers) listFoods(c *gin.Context) {
	fs, err := h.Foods.List(c.Request.Context())
	if err != nil {
		writeErr(c, err)
		return
	}
	listJSON(c, fs)
}

func (h *Handlers) createFood(c *gin.Context) {
	var p foodPayload
	if err := bind(c, &p); err != nil {
		writeErr(c, err)
		return
	}
	f, err := h.Foods.Create(c.Request.Context(), fitness.Food{
		Name: p.Name, Brand: p.Brand, ServingSize: p.ServingSize, ServingUnit: p.ServingUnit,
		Calories: p.Calories, Protein: p.Protein, Carbs: p.Carbs, Fat: p.Fat,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, f)
}

func (h *Handlers) getFood(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	f, err := h.Foods.Get(c.Request.Context(), id)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, f)
}

func (h *Handlers) updateFood(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	var p foodPayload
	if err := bind(c, &p); err != nil {
		writeErr(c, err)
		return
	}
	f, err := h.Foods.Update(c.Request.Context(), id, fitness.Food{
		Name: p.Name, Brand: p.Brand, ServingSize: p.ServingSize, ServingUnit: p.ServingUnit,
		Calories: p.Calories, Protein: p.Protein, Carbs: p.Carbs, Fat: p.Fat,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, f)
}

func (h *Handlers) deleteFood(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	if err := h.Foods.Delete(c.Request.Context(), id); err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, gin.H{"deleted": true})
}

// ---- meal log ----

type mealItemPayload struct {
	FoodID   *int64  `json:"food_id"`
	Label    string  `json:"label"`
	Quantity float64 `json:"quantity"`
	Calories float64 `json:"calories"`
	Protein  float64 `json:"protein"`
	Carbs    float64 `json:"carbs"`
	Fat      float64 `json:"fat"`
}

type mealPayload struct {
	Name     string             `json:"name"`
	MealType string             `json:"meal_type"`
	Portion  string             `json:"portion"`
	Notes    string             `json:"notes"`
	EatenAt  time.Time          `json:"eaten_at"`
	Items    *[]mealItemPayload `json:"items"`
}

// mealItems adapts the payload items to domain items. A nil slice means
// "items omitted" (Update keeps the existing rows); an empty non-nil slice
// means "replace with none".
func mealItems(items []mealItemPayload, present bool) []fitness.MealItem {
	if !present {
		return nil
	}
	out := make([]fitness.MealItem, len(items))
	for i, it := range items {
		out[i] = fitness.MealItem{
			FoodID: it.FoodID, Label: it.Label, Quantity: it.Quantity,
			Calories: it.Calories, Protein: it.Protein, Carbs: it.Carbs, Fat: it.Fat,
		}
	}
	return out
}

func (h *Handlers) listMeals(c *gin.Context) {
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
	ms, err := h.Meals.ListByUser(c.Request.Context(), user.ID, orZero(from, hasFrom), orZero(to, hasTo), queryLimit(c))
	if err != nil {
		writeErr(c, err)
		return
	}
	listJSON(c, ms)
}

func (h *Handlers) createMeal(c *gin.Context) {
	user := mustUser(c)
	var p mealPayload
	if err := bind(c, &p); err != nil {
		writeErr(c, err)
		return
	}
	m, err := h.Meals.Create(c.Request.Context(), user.ID, fitness.Meal{
		Name: p.Name, MealType: p.MealType, Portion: p.Portion, Notes: p.Notes, EatenAt: p.EatenAt,
	}, mealItems(deref(p.Items), p.Items != nil))
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, m)
}

func (h *Handlers) getMeal(c *gin.Context) {
	user := mustUser(c)
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	m, err := h.Meals.Get(c.Request.Context(), id, user.ID)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, m)
}

func (h *Handlers) updateMeal(c *gin.Context) {
	user := mustUser(c)
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	var p mealPayload
	if err := bind(c, &p); err != nil {
		writeErr(c, err)
		return
	}
	m, err := h.Meals.Update(c.Request.Context(), id, user.ID, fitness.Meal{
		Name: p.Name, MealType: p.MealType, Portion: p.Portion, Notes: p.Notes, EatenAt: p.EatenAt,
	}, mealItems(deref(p.Items), p.Items != nil))
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, m)
}

func (h *Handlers) deleteMeal(c *gin.Context) {
	user := mustUser(c)
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	if err := h.Meals.Delete(c.Request.Context(), id, user.ID); err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, gin.H{"deleted": true})
}

func (h *Handlers) listMealItems(c *gin.Context) {
	user := mustUser(c)
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	m, err := h.Meals.Get(c.Request.Context(), id, user.ID)
	if err != nil {
		writeErr(c, err)
		return
	}
	listJSON(c, m.Items)
}

// deref returns the pointed-to slice, or nil when p is nil.
func deref(p *[]mealItemPayload) []mealItemPayload {
	if p == nil {
		return nil
	}
	return *p
}
