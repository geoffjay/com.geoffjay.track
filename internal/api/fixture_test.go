package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"com.geoffjay.track/internal/api"
	"com.geoffjay.track/internal/auth"
	"com.geoffjay.track/internal/db"
	"com.geoffjay.track/internal/middleware"
	"com.geoffjay.track/internal/seedpop"

	"github.com/gin-gonic/gin"
)

// apiFixture boots a fresh API server against a temp SQLite DB seeded with
// both users and the fitness catalogs. The returned call helper performs a
// JSON request with basic auth and decodes JSON object responses.
func apiFixture(t *testing.T) (*httptest.Server, func(method, path, user, pass string, body any) (int, map[string]any, string)) {
	t.Helper()
	dir := t.TempDir()
	dbh, err := db.Open(filepath.Join(dir, "track.db"))
	if err != nil {
		t.Fatal("db open:", err)
	}
	t.Cleanup(func() { dbh.Close() })

	if err := seedpop.Ensure(context.Background(), dbh); err != nil {
		t.Fatal("seed catalogs:", err)
	}
	authStore := auth.NewStore(dbh)
	if err := authStore.EnsureUsers(context.Background(), []auth.User{
		{Username: "geoff", PasswordHash: "pw"},
		{Username: "misty", PasswordHash: "pw"},
	}, 4); err != nil {
		t.Fatal("seed users:", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())
	api.Register(r.Group("/api/v1", middleware.BasicAuth(authStore, "test")), api.NewHandlers(dbh))
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)

	client := &http.Client{}
	call := func(method, path, user, pass string, body any) (int, map[string]any, string) {
		var rd io.Reader
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			rd = bytes.NewReader(b)
		}
		req, err := http.NewRequest(method, ts.URL+path, rd)
		if err != nil {
			t.Fatal(err)
		}
		if user != "" {
			req.SetBasicAuth(user, pass)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		var m map[string]any
		trimmed := bytes.TrimLeft(raw, " \t\r\n")
		if len(trimmed) > 0 && trimmed[0] == '{' {
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatalf("%s %s: non-JSON response: %s", method, path, raw)
			}
		}
		return resp.StatusCode, m, string(raw)
	}
	return ts, call
}
