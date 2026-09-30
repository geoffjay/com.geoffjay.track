package api

import (
	"net/http"
	"strconv"
	"time"

	"com.geoffjay.track/internal/fitness"

	"github.com/gin-gonic/gin"
)

// measurementRoutes wires the metrics + measurements CRUD under rg.
func measurementRoutes(rg *gin.RouterGroup, h *Handlers) {
	// Metric catalog.
	rg.GET("/metrics", h.listMetrics)
	rg.POST("/metrics", h.createMetric)
	rg.GET("/metrics/:id", h.getMetric)
	rg.PATCH("/metrics/:id", h.updateMetric)
	rg.DELETE("/metrics/:id", h.deleteMetric)

	// Measurement log.
	rg.GET("/measurements", h.listMeasurements)
	rg.POST("/measurements", h.createMeasurement)
	rg.GET("/measurements/latest", h.latestMeasurements)
	rg.GET("/measurements/:id", h.getMeasurement)
	rg.PATCH("/measurements/:id", h.updateMeasurement)
	rg.DELETE("/measurements/:id", h.deleteMeasurement)
}

// ---- metric catalog ----

type metricPayload struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Unit     string `json:"unit"`
	Category string `json:"category"`
}

func (h *Handlers) listMetrics(c *gin.Context) {
	ms, err := h.Metrics.List(c.Request.Context())
	if err != nil {
		writeErr(c, err)
		return
	}
	listJSON(c, ms)
}

func (h *Handlers) createMetric(c *gin.Context) {
	var p metricPayload
	if err := bind(c, &p); err != nil {
		writeErr(c, err)
		return
	}
	m, err := h.Metrics.Create(c.Request.Context(), fitness.Metric{
		Name: p.Name, Label: p.Label, Unit: p.Unit, Category: p.Category,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, m)
}

func (h *Handlers) getMetric(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	m, err := h.Metrics.Get(c.Request.Context(), id)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, m)
}

func (h *Handlers) updateMetric(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	var p metricPayload
	if err := bind(c, &p); err != nil {
		writeErr(c, err)
		return
	}
	m, err := h.Metrics.Update(c.Request.Context(), id, fitness.Metric{
		Name: p.Name, Label: p.Label, Unit: p.Unit, Category: p.Category,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, m)
}

func (h *Handlers) deleteMetric(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	if err := h.Metrics.Delete(c.Request.Context(), id); err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, gin.H{"deleted": true})
}

// ---- measurement log ----

type measurementPayload struct {
	MetricID   int64     `json:"metric_id"`
	Value      float64   `json:"value"`
	MeasuredAt time.Time `json:"measured_at"`
	Note       string    `json:"note"`
}

func (h *Handlers) listMeasurements(c *gin.Context) {
	user := mustUser(c)
	metricID, _ := strconv.ParseInt(c.Query("metric_id"), 10, 64)
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
	ms, err := h.Measurements.ListByUser(c.Request.Context(), user.ID, metricID, orZero(from, hasFrom), orZero(to, hasTo), queryLimit(c))
	if err != nil {
		writeErr(c, err)
		return
	}
	listJSON(c, ms)
}

func (h *Handlers) createMeasurement(c *gin.Context) {
	user := mustUser(c)
	var p measurementPayload
	if err := bind(c, &p); err != nil {
		writeErr(c, err)
		return
	}
	m, err := h.Measurements.Create(c.Request.Context(), user.ID, fitness.Measurement{
		MetricID: p.MetricID, Value: p.Value, MeasuredAt: p.MeasuredAt, Note: p.Note,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, m)
}

func (h *Handlers) latestMeasurements(c *gin.Context) {
	user := mustUser(c)
	ms, err := h.Measurements.LatestByMetric(c.Request.Context(), user.ID)
	if err != nil {
		writeErr(c, err)
		return
	}
	listJSON(c, ms)
}

func (h *Handlers) getMeasurement(c *gin.Context) {
	user := mustUser(c)
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	m, err := h.Measurements.Get(c.Request.Context(), id, user.ID)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, m)
}

func (h *Handlers) updateMeasurement(c *gin.Context) {
	user := mustUser(c)
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	var p measurementPayload
	if err := bind(c, &p); err != nil {
		writeErr(c, err)
		return
	}
	m, err := h.Measurements.Update(c.Request.Context(), id, user.ID, fitness.Measurement{
		Value: p.Value, MeasuredAt: p.MeasuredAt, Note: p.Note,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, m)
}

func (h *Handlers) deleteMeasurement(c *gin.Context) {
	user := mustUser(c)
	id, err := pathID(c)
	if err != nil {
		writeErr(c, err)
		return
	}
	if err := h.Measurements.Delete(c.Request.Context(), id, user.ID); err != nil {
		writeErr(c, err)
		return
	}
	writeJSON(c, http.StatusOK, gin.H{"deleted": true})
}
