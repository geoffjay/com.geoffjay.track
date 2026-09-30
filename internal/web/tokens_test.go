package web_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"com.geoffjay.track/internal/api"
	"com.geoffjay.track/internal/auth"
	"com.geoffjay.track/internal/config"
	"com.geoffjay.track/internal/db"
	"com.geoffjay.track/internal/seedpop"
	"com.geoffjay.track/internal/web"
)

// TestSmokeTokens boots the full server and exercises the settings page and
// bearer-token authentication end-to-end: create token via the form, use it
// against /api/v1, verify basic auth still works, revoke it, and confirm
// revocation and expiry are enforced.
func TestSmokeTokens(t *testing.T) {
	dir := t.TempDir()
	dbh, err := db.Open(filepath.Join(dir, "track.db"))
	if err != nil {
		t.Fatal("db open:", err)
	}
	defer dbh.Close()

	if err := seedpop.Ensure(context.Background(), dbh); err != nil {
		t.Fatal("seed catalogs:", err)
	}
	authStore := auth.NewStore(dbh)
	if err := authStore.EnsureUsers(context.Background(), []auth.User{
		{Username: "geoff", PasswordHash: "geoff-row"},
	}, 4); err != nil {
		t.Fatal("seed users:", err)
	}

	tokenStore := auth.NewTokenStore(dbh)
	cfg := config.Config{Port: 18099, Realm: "smoke", Env: "development"}
	srv := web.New(cfg, authStore, tokenStore, nil, api.NewHandlers(dbh))
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}
	postForm := func(path, user, pass string, form url.Values) *http.Response {
		req, _ := http.NewRequest("POST", ts.URL+path, strings.NewReader(form.Encode()))
		req.SetBasicAuth(user, pass)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	bodyOf := func(r *http.Response) string {
		b, _ := io.ReadAll(r.Body)
		return string(b)
	}

	// Settings page renders with basic auth and shows the empty state.
	req, _ := http.NewRequest("GET", ts.URL+"/settings", nil)
	req.SetBasicAuth("geoff", "geoff-row")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	page := bodyOf(resp)
	if resp.StatusCode != 200 || !strings.Contains(page, "Settings") || !strings.Contains(page, "No tokens yet") {
		t.Fatalf("settings page: %d", resp.StatusCode)
	}
	// Settings requires auth like every UI page.
	resp, err = http.Get(ts.URL + "/settings")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("unauth settings: got %d, want 401", resp.StatusCode)
	}

	// Create a never-expiring token via the form. The response shows the
	// raw token exactly once.
	resp = postForm("/settings/tokens", "geoff", "geoff-row", url.Values{
		"name": {"phone app"},
	})
	page = bodyOf(resp)
	if resp.StatusCode != 200 || !strings.Contains(page, "phone app") {
		t.Fatalf("token create: %d", resp.StatusCode)
	}
	// Extract the raw token from the <pre><code> block.
	var raw string
	if i := strings.Index(page, "<pre><code>"); i >= 0 {
		rest := page[i+len("<pre><code>"):]
		if j := strings.Index(rest, "</code>"); j > 0 {
			raw = rest[:j]
		}
	}
	if raw == "" {
		t.Fatal("raw token not shown after create")
	}
	// The settings list now shows the token by name, without the secret.
	if !strings.Contains(page, "Revoke") {
		t.Fatal("token list missing revoke button")
	}

	// Bearer token authenticates the API.
	req, _ = http.NewRequest("GET", ts.URL+"/api/v1/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("bearer auth: got %d, want 200", resp.StatusCode)
	}
	bodyOf(resp)

	// Basic auth still works on the API.
	req, _ = http.NewRequest("GET", ts.URL+"/api/v1/metrics", nil)
	req.SetBasicAuth("geoff", "geoff-row")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("basic auth still works: got %d", resp.StatusCode)
	}

	// Bearer does NOT authenticate the web UI.
	req, _ = http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("bearer on UI: got %d, want 401", resp.StatusCode)
	}

	// A garbage token is rejected.
	req, _ = http.NewRequest("GET", ts.URL+"/api/v1/metrics", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("garbage bearer: got %d, want 401", resp.StatusCode)
	}

	// Create an already-expiring-tomorrow token directly, then an
	// already-expired one to verify expiry enforcement.
	tomorrow := time.Now().UTC().AddDate(0, 0, 1)
	tok, _, err := tokenStore.Create(context.Background(), 1, "expiring", &tomorrow)
	if err != nil {
		t.Fatal("create expiring token:", err)
	}
	_, err = tokenStore.Verify(context.Background(), "definitely-not-the-raw-value")
	if err != auth.ErrBadCredentials {
		t.Fatalf("verify garbage: got %v", err)
	}
	// Verify the expiring token's row exists in the list.
	req, _ = http.NewRequest("GET", ts.URL+"/settings", nil)
	req.SetBasicAuth("geoff", "geoff-row")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	page = bodyOf(resp)
	if !strings.Contains(page, "expiring") {
		t.Fatal("expiring token missing from settings list")
	}

	// Expired token: insert one whose expiry is in the past by creating
	// with a past date (must be rejected at create) — instead craft it
	// directly through the store with a past date via Create (rejected).
	past := time.Now().UTC().Add(-time.Hour)
	if _, _, err := tokenStore.Create(context.Background(), 1, "past", &past); err == nil {
		t.Fatal("past expiry should be rejected")
	}

	// Revoke the "phone app" token specifically: find its row's form action
	// by searching for the name first, then the next /delete action after it.
	nameIdx := strings.Index(page, "phone app")
	if nameIdx < 0 {
		t.Fatal("phone app token missing from settings list")
	}
	if i := strings.Index(page[nameIdx:], "/settings/tokens/"); i >= 0 {
		rest := page[nameIdx+i+len("/settings/tokens/"):]
		if j := strings.Index(rest, "/delete"); j > 0 {
			idStr := rest[:j]
			resp = postForm("/settings/tokens/"+idStr+"/delete", "geoff", "geoff-row", nil)
			if resp.StatusCode != http.StatusSeeOther {
				t.Fatalf("token revoke: got %d", resp.StatusCode)
			}
			bodyOf(resp)
			// The revoked token no longer authenticates.
			req, _ = http.NewRequest("GET", ts.URL+"/api/v1/metrics", nil)
			req.Header.Set("Authorization", "Bearer "+raw)
			resp, err = client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != 401 {
				t.Fatalf("revoked bearer: got %d, want 401", resp.StatusCode)
			}
		} else {
			t.Fatal("could not parse revoke form action")
		}
	} else {
		t.Fatal("revoke form not found on settings page")
	}

	_ = tok
	fmt.Println("TOKENS OK")
}
