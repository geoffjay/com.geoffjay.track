// Package api serves the versioned JSON API consumed by mobile clients.
// It adapts the fitness domain layer to HTTP: JSON binding, status mapping,
// and route wiring. The existing server-rendered UI is untouched.
package api

import (
	"errors"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"com.geoffjay.track/internal/auth"
	"com.geoffjay.track/internal/fitness"
	"com.geoffjay.track/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Error is the JSON error envelope. `field` is set only for validation errors.
type Error struct {
	Error string `json:"error"`
	Field string `json:"field,omitempty"`
}

// writeJSON writes v as a JSON response with the given status.
func writeJSON(c *gin.Context, status int, v any) {
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.JSON(status, v)
}

// writeErr writes the standard error envelope, mapping domain errors to
// statuses: ValidationError -> 400, ErrForbidden -> 403, ErrNotFound -> 404,
// ErrConflict -> 409, anything else -> 500.
func writeErr(c *gin.Context, err error) {
	var ve *fitness.ValidationError
	switch {
	case errors.As(err, &ve):
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.JSON(http.StatusBadRequest, Error{Error: ve.Reason, Field: ve.Field})
	case errors.Is(err, fitness.ErrForbidden):
		writeJSON(c, http.StatusForbidden, Error{Error: "system rows cannot be modified"})
	case errors.Is(err, fitness.ErrNotFound):
		writeJSON(c, http.StatusNotFound, Error{Error: "not found"})
	case errors.Is(err, fitness.ErrConflict):
		writeJSON(c, http.StatusConflict, Error{Error: err.Error()})
	default:
		// Unexpected: log via gin's context so the request line carries it.
		c.Error(err)
		writeJSON(c, http.StatusInternalServerError, Error{Error: "internal error"})
	}
}

// bind decodes the JSON body (when present) into v. Empty bodies are allowed
// and leave v zero-valued.
func bind(c *gin.Context, v any) error {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return nil
	}
	if err := c.ShouldBindJSON(v); err != nil {
		return &fitness.ValidationError{Field: "body", Reason: "invalid JSON: " + err.Error()}
	}
	return nil
}

// pathID parses a numeric :id route parameter.
func pathID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, &fitness.ValidationError{Field: "id", Reason: "must be a positive integer"}
	}
	return id, nil
}

// queryTime parses an optional timestamp query parameter.
func queryTime(c *gin.Context, name string) (time.Time, bool, error) {
	v := strings.TrimSpace(c.Query(name))
	if v == "" {
		return time.Time{}, false, nil
	}
	parsed, err := fitness.ParseTime(v)
	if err != nil {
		return time.Time{}, false, err
	}
	return parsed, true, nil
}

// mustUser returns the authenticated user; guaranteed present behind
// middleware.BasicAuth. Returns nil only if routing is misconfigured, in
// which case the request has been aborted with 401.
func mustUser(c *gin.Context) *auth.User {
	if u := middleware.UserFrom(c); u != nil {
		return u
	}
	c.AbortWithStatus(http.StatusUnauthorized)
	return nil
}

// orZero returns t, or the zero Time when !ok.
func orZero(t time.Time, ok bool) time.Time {
	if ok {
		return t
	}
	return time.Time{}
}

// listJSON writes a list response, coercing nil slices to [] so clients
// always see a JSON array.
func listJSON(c *gin.Context, v any) {
	writeJSON(c, http.StatusOK, gin.H{"data": nonNil(v)})
}

// nonNil replaces nil slices/maps behind any with their empty equivalents.
func nonNil(v any) any {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return []any{}
	}
	switch rv.Kind() {
	case reflect.Slice, reflect.Map:
		if rv.IsNil() {
			if rv.Kind() == reflect.Slice {
				s := reflect.MakeSlice(rv.Type(), 0, 0)
				return s.Interface()
			}
		}
	}
	return v
}

// queryLimit parses the limit query parameter (0 -> default).
func queryLimit(c *gin.Context) int {
	n, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		return 0
	}
	return n
}
