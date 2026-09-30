// Package fitness is the health/fitness domain layer: catalogs (metrics,
// exercises, foods) and per-user logs (measurements, meals, fasts, workouts).
// It is transport-agnostic — the api package adapts it to JSON over HTTP.
package fitness

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// Sentinel errors mapped by the api package to HTTP statuses.
var (
	// ErrNotFound is returned when a row does not exist or belongs to another
	// user (ownership is not distinguished, to avoid leaking existence).
	ErrNotFound = errors.New("not found")
	// ErrConflict is returned on uniqueness or state violations (duplicate
	// name, ending an already-ended fast, starting a second active fast).
	ErrConflict = errors.New("conflict")
	// ErrForbidden is returned when an operation targets a system (seeded)
	// row that the API keeps immutable.
	ErrForbidden = errors.New("forbidden")
)

// ValidationError is a field-scoped input error mapped to HTTP 400.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Reason }

// invalidf builds a *ValidationError.
func invalidf(field, format string, args ...any) *ValidationError {
	return &ValidationError{Field: field, Reason: fmt.Sprintf(format, args...)}
}

// ErrValidation reports whether err is a validation error.
func ErrValidation(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

// ValidationField extracts the offending field from err, if it is one.
func ValidationField(err error) string {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve.Field
	}
	return ""
}

// ParseTime parses timestamps accepted from API clients (RFC3339 with or
// without fractional seconds, or a bare date) and returns UTC. It is the
// exported counterpart of parseTime for use by the api package.
func ParseTime(s string) (time.Time, error) { return parseTime(s) }

// parseTime parses timestamps accepted from API clients (RFC3339 with or
// without fractional seconds, or a bare date) and returns UTC.
func parseTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, invalidf("timestamp", "must be RFC3339 or YYYY-MM-DD, got %q", s)
}

// formatTime renders a time the way rows are stored / returned.
func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

// finite reports whether v is a usable real number.
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// clampLimit normalizes pagination limits.
func clampLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}
