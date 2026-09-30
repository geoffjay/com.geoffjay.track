package web

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"com.geoffjay.track/internal/auth"
	"com.geoffjay.track/internal/middleware"
	"com.geoffjay.track/internal/web/views"

	"github.com/gin-gonic/gin"
)

// settings renders the user settings page: API token list + create form.
func (s *Server) settings(c *gin.Context) {
	user := middleware.UserFrom(c)
	tokens, err := s.tokens.ListByUser(c.Request.Context(), user.ID)
	if err != nil {
		s.serverError(c, err)
		return
	}
	render(c, http.StatusOK, views.Settings(page(c), tokens))
}

// tokenCreate handles the create-token form. The raw token is displayed
// exactly once on the response (no PRG redirect: the secret must not ride
// the URL, and a redirect would lose it).
func (s *Server) tokenCreate(c *gin.Context) {
	user := middleware.UserFrom(c)

	name := c.PostForm("name")

	// Expiry: empty = never; otherwise a date from the form.
	var expires *time.Time
	if v := c.PostForm("expires_at"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			redirect(c, "/settings", "Invalid expiration date.", "error")
			return
		}
		// End of the chosen day, UTC, so the token lasts that whole day.
		eod := t.Add(24*time.Hour - time.Second).UTC()
		expires = &eod
	}

	tok, raw, err := s.tokens.Create(c.Request.Context(), user.ID, name, expires)
	if err != nil {
		if errors.Is(err, auth.ErrBadExpiry) {
			redirect(c, "/settings", "Expiration must be in the future.", "error")
			return
		}
		s.serverError(c, err)
		return
	}

	// Show the secret exactly once. Re-render (not PRG): a redirect would
	// either leak the token through the URL or lose it entirely.
	tokens, err := s.tokens.ListByUser(c.Request.Context(), user.ID)
	if err != nil {
		s.serverError(c, err)
		return
	}
	p := page(c)
	p.Flash = &views.Flash{Color: "success", Message: "Token created. Copy it now — it will not be shown again."}
	render(c, http.StatusOK, views.SettingsWithSecret(p, tokens, tok, raw))
}

// tokenDelete revokes one of the user's tokens.
func (s *Server) tokenDelete(c *gin.Context) {
	user := middleware.UserFrom(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		redirect(c, "/settings", "Invalid token id.", "error")
		return
	}
	if err := s.tokens.Delete(c.Request.Context(), id, user.ID); err != nil {
		if errors.Is(err, auth.ErrTokenNotFound) {
			redirect(c, "/settings", "Token not found.", "error")
			return
		}
		s.serverError(c, err)
		return
	}
	redirect(c, "/settings", "Token revoked.", "success")
}
