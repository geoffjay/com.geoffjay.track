// Package web wires the gin router, handlers, and templ rendering.
package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"com.geoffjay.track/internal/api"
	"com.geoffjay.track/internal/auth"
	"com.geoffjay.track/internal/config"
	"com.geoffjay.track/internal/middleware"
	"com.geoffjay.track/internal/track"
	"com.geoffjay.track/internal/web/views"
	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
)

// Users lists the predefined users in chart display order.
var Users = []string{"geoff", "misty"}

// Server carries the singletons handlers need.
type Server struct {
	cfg     config.Config
	auth    *auth.Store
	tracker *track.Store
	api     *api.Handlers
}

// New builds the Server and the gin engine. apiHandlers may be nil for tests
// that only exercise the UI.
func New(cfg config.Config, authStore *auth.Store, tracker *track.Store, apiHandlers *api.Handlers) *Server {
	return &Server{cfg: cfg, auth: authStore, tracker: tracker, api: apiHandlers}
}

// Router builds the gin engine with all routes and middleware.
func (s *Server) Router() *gin.Engine {
	if s.cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// Stylesheet is embedded at build time so the binary serves it from any
	// working directory (tests, Fly machine, local dev alike).
	r.GET("/assets/styles.css", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=3600")
		c.Data(http.StatusOK, "text/css; charset=utf-8", stylesCSS)
	})
	r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })

	authed := r.Group("/", middleware.BasicAuth(s.auth, s.cfg.Realm))
	{
		authed.GET("/", s.dashboard)
		authed.GET("/checkin", s.checkinForm)
		authed.POST("/checkin", s.checkinCreate)
		authed.GET("/history", s.history)
		authed.POST("/checkins/:id/delete", s.checkinDelete)
	}

	// Versioned JSON API for mobile clients, sharing the same basic auth.
	if s.api != nil {
		api.Register(r.Group("/api/v1", middleware.BasicAuth(s.auth, s.cfg.Realm)), s.api)
	}

	return r
}

// page builds the views.Page from the request context.
func page(c *gin.Context) views.Page {
	p := views.Page{}
	if u := middleware.UserFrom(c); u != nil {
		p.User = u.Username
	}
	if raw := c.Query("flash"); raw != "" {
		color := "success"
		if c.Query("flash_color") == "error" {
			color = "error"
		}
		p.Flash = &views.Flash{Color: color, Message: raw}
	}
	return p
}

// dashboard renders the scoreboard page.
func (s *Server) dashboard(c *gin.Context) {
	totals, err := s.tracker.Totals(c.Request.Context())
	if err != nil {
		s.serverError(c, err)
		return
	}
	recent, err := s.tracker.Recent(c.Request.Context(), 5)
	if err != nil {
		s.serverError(c, err)
		return
	}
	series, err := s.tracker.DailySeries(c.Request.Context(), Users, 30)
	if err != nil {
		s.serverError(c, err)
		return
	}
	chart, err := views.BuildChartView(series, weeklyFrom(series, Users), Users)
	if err != nil {
		s.serverError(c, err)
		return
	}
	render(c, http.StatusOK, views.Dashboard(page(c), totals, recent, chart, Users))
}

// checkinForm renders the standalone check-in page.
func (s *Server) checkinForm(c *gin.Context) {
	render(c, http.StatusOK, views.CheckIn(page(c)))
}

// checkinCreate handles the POST from the quick check-in form.
func (s *Server) checkinCreate(c *gin.Context) {
	miles, err := strconv.ParseFloat(strings.TrimSpace(c.PostForm("miles")), 64)
	if err != nil || miles <= 0 {
		redirect(c, "/", "Enter a number of miles greater than 0.", "error")
		return
	}
	u := middleware.UserFrom(c)
	if u == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	var rowedAt time.Time
	if raw := c.PostForm("rowed_at"); raw != "" {
		if t, err := time.Parse("2006-01-02T15:04", raw); err == nil {
			rowedAt = t
		}
	}
	if _, err := s.tracker.Create(c.Request.Context(), u.ID, miles, rowedAt); err != nil {
		redirect(c, "/", "Could not save that check-in. Try again.", "error")
		return
	}
	redirect(c, "/", fmt.Sprintf("Logged %.1f miles. Keep it up!", miles), "success")
}

// checkinDelete removes one of the signed-in user's check-ins.
func (s *Server) checkinDelete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		redirect(c, "/history", "Unknown check-in.", "error")
		return
	}
	u := middleware.UserFrom(c)
	if u == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if err := s.tracker.Delete(c.Request.Context(), id, u.ID); err != nil {
		redirect(c, "/history", "Could not delete that check-in.", "error")
		return
	}
	redirect(c, "/history", "Check-in deleted.", "success")
}

// history renders the all check-ins table.
func (s *Server) history(c *gin.Context) {
	checkins, err := s.tracker.Recent(c.Request.Context(), 200)
	if err != nil {
		s.serverError(c, err)
		return
	}
	render(c, http.StatusOK, views.History(page(c), checkins))
}

// weeklyFrom reshapes the daily series into an ordered list of the last 7
// days, each carrying that day's (not cumulative) miles per user. Ordered
// output keeps the bar chart's x axis chronological instead of map-random.
func weeklyFrom(series map[string][]track.LinePoint, users []string) []views.WeekDay {
	var out []views.WeekDay
	anyPts := series[users[0]]
	if len(anyPts) == 0 {
		return out
	}
	for i := len(anyPts) - 7; i < len(anyPts); i++ {
		if i < 0 {
			continue
		}
		day := views.WeekDay{Label: anyPts[i].Day.Format("Mon"), Miles: map[string]float64{}}
		for _, u := range users {
			pts := series[u]
			if i >= len(pts) {
				continue
			}
			prev := 0.0
			if i > 0 {
				prev = pts[i-1].Miles
			}
			day.Miles[u] = pts[i].Miles - prev
		}
		out = append(out, day)
	}
	return out
}

// redirect sends a flash message through the query string (simple PRG
// pattern; no session store needed for two users).
func redirect(c *gin.Context, path, msg, color string) {
	c.Redirect(http.StatusSeeOther, path+"?flash="+urlEscape(msg)+"&flash_color="+color)
}

// urlEscape escapes a flash message for a query value.
func urlEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&', '?', '=', '#', '+', '%':
			fmt.Fprintf(&b, "%%%02X", r)
		case ' ':
			b.WriteByte('+')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// render writes a templ component with the standard headers.
func render(c *gin.Context, status int, component templ.Component) {
	c.Status(status)
	_ = component.Render(c.Request.Context(), c.Writer)
}

// serverError logs and returns a plain 500.
func (s *Server) serverError(c *gin.Context, err error) {
	c.Error(err)
	c.String(http.StatusInternalServerError, "Something went wrong.")
}
