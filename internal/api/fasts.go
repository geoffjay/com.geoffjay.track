package api

import (
	"net/http"
	"time"

	"com.geoffjay.track/internal/fitness"

	"github.com/gin-gonic/gin"
)

// fastRoutes wires the fast log under rg. /fasts/active is registered
// before /fasts/:id so the static path wins.
func fastRoutes(rg *gin.RouterGroup, h *Handlers) {
	rg.GET("/fasts/active", h.activeFast)
	rg.GET("/fasts", h.listFasts)
	rg.POST("/fasts", h.createFast)
	rg.GET("/fasts/:id", h.getFast)
	rg.PATCH("/fasts/:id", h.updateFast)
	rg.POST("/fasts/:id/end", h.endFast)
	rg.DELETE("/fasts/:id", h.deleteFast)
}

// fastPayload is the create/update body. Pointer / zero semantics on
// PATCH: absent keeps the existing value; present replaces it.
type fastPayload struct {
	StartedAt   *time.Time `json:"started_at"`
	EndedAt     *time.Time `json:"ended_at"`
	TargetHours *float64   `json:"target_hours"`
	Notes       *string    `json:"notes"`
}

func (h *Handlers) activeFast(c *gin.Context) {
	user := mustUser(c)
	f, err := h.Fasts.Active(c.Request.Context(), user.ID)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, f)
}

func (h *Handlers) listFasts(c *gin.Context) {
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
	fs, err := h.Fasts.ListByUser(c.Request.Context(), user.ID, orZero(from, hasFrom), orZero(to, hasTo), queryLimit(c))
	if err != nil {
		writeErr(c, err)
		return
	}
	listJSON(c, fs)
}

func (h *Handlers) createFast(c *gin.Context) {
	user := mustUser(c)
	var p fastPayload
	if err := bind(c, &p); err != nil {
		writeErr(c, err)
		return
	}
	f, err := h.Fasts.Create(c.Request.Context(), user.ID, p.toFast())
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, f)
}

func (h *Handlers) getFast(c *gin.Context) {
	user := mustUser(c)
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	f, err := h.Fasts.Get(c.Request.Context(), id, user.ID)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, f)
}

func (h *Handlers) updateFast(c *gin.Context) {
	user := mustUser(c)
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	var p fastPayload
	if err := bind(c, &p); err != nil {
		writeErr(c, err)
		return
	}
	in := p.toFast()
	if p.Notes == nil {
		// PATCH semantics: absent notes keeps the existing value.
		if cur, err := h.Fasts.Get(c.Request.Context(), id, user.ID); err == nil {
			in.Notes = cur.Notes
		}
	}
	f, err := h.Fasts.Update(c.Request.Context(), id, user.ID, in)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, f)
}

func (h *Handlers) endFast(c *gin.Context) {
	user := mustUser(c)
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	var p fastPayload
	if err := bind(c, &p); err != nil {
		writeErr(c, err)
		return
	}
	var ended time.Time
	if p.EndedAt != nil {
		ended = *p.EndedAt
	}
	f, err := h.Fasts.End(c.Request.Context(), id, user.ID, ended)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, f)
}

func (h *Handlers) deleteFast(c *gin.Context) {
	user := mustUser(c)
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	if err := h.Fasts.Delete(c.Request.Context(), id, user.ID); err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, gin.H{"deleted": true})
}

// toFast converts the payload to a domain Fast. Nil pointers stay nil so
// the store's "nil = keep existing" semantics apply.
func (p fastPayload) toFast() fitness.Fast {
	f := fitness.Fast{TargetHours: p.TargetHours, EndedAt: p.EndedAt}
	if p.StartedAt != nil {
		f.StartedAt = *p.StartedAt
	}
	if p.Notes != nil {
		f.Notes = *p.Notes
	}
	return f
}
