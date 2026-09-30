package web_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"com.geoffjay.track/internal/api"
	"com.geoffjay.track/internal/auth"
	"com.geoffjay.track/internal/config"
	"com.geoffjay.track/internal/db"
	"com.geoffjay.track/internal/track"
	"com.geoffjay.track/internal/web"
)

// TestSmoke boots the full server against a temp DB and exercises every route
// end-to-end: auth, dashboard, check-in create, delete, history, static, health.
func TestSmoke(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "track.db")

	dbh, err := db.Open(dbPath)
	if err != nil {
		t.Fatal("db open:", err)
	}
	defer dbh.Close()

	authStore := auth.NewStore(dbh)
	if err := authStore.EnsureUsers(context.Background(), []auth.User{
		{Username: "geoff", PasswordHash: "geoff-row"},
		{Username: "misty", PasswordHash: "misty-row"},
	}, 4); err != nil {
		t.Fatal("seed:", err)
	}

	u1, err := authStore.Authenticate(context.Background(), "geoff", "geoff-row")
	if err != nil {
		t.Fatal("authenticate geoff:", err)
	}
	if _, err := authStore.Authenticate(context.Background(), "geoff", "wrong"); err == nil {
		t.Fatal("wrong password should fail")
	}
	u2, err := authStore.Authenticate(context.Background(), "misty", "misty-row")
	if err != nil {
		t.Fatal("authenticate misty:", err)
	}

	tracker := track.NewStore(dbh)
	for _, c := range []struct {
		u     auth.User
		miles float64
	}{{u1, 2.5}, {u1, 3.0}, {u2, 4.2}} {
		if _, err := tracker.Create(context.Background(), c.u.ID, c.miles, time.Now().UTC()); err != nil {
			t.Fatal("create:", err)
		}
	}

	totals, err := tracker.Totals(context.Background())
	if err != nil {
		t.Fatal("totals:", err)
	}
	if len(totals) != 2 || totals[0].Username != "geoff" || totals[0].Miles != 5.5 {
		t.Fatalf("expected geoff leading 5.5, got %+v", totals)
	}

	series, err := tracker.DailySeries(context.Background(), []string{"geoff", "misty"}, 30)
	if err != nil {
		t.Fatal("daily series:", err)
	}
	if len(series["geoff"]) != 30 || series["geoff"][29].Miles != 5.5 {
		t.Fatalf("geoff series wrong: len=%d last=%v", len(series["geoff"]), series["geoff"][29].Miles)
	}

	cfg := config.Config{Port: 18099, Realm: "smoke", Env: "development"}
	srv := web.New(cfg, authStore, auth.NewTokenStore(dbh), tracker, api.NewHandlers(dbh))
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}
	get := func(path, user, pass string) *http.Response {
		req, _ := http.NewRequest("GET", ts.URL+path, nil)
		req.SetBasicAuth(user, pass)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	body := func(r *http.Response) string {
		b, _ := io.ReadAll(r.Body)
		return string(b)
	}

	// Unauthenticated -> 401 with WWW-Authenticate.
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized || !strings.Contains(resp.Header.Get("WWW-Authenticate"), `Basic realm="smoke"`) {
		t.Fatalf("unauth: got %d", resp.StatusCode)
	}

	// Dashboard renders with charts.
	resp = get("/", "geoff", "geoff-row")
	if resp.StatusCode != 200 {
		t.Fatalf("dashboard: got %d", resp.StatusCode)
	}
	page := body(resp)
	if !strings.Contains(page, "Scoreboard") || !strings.Contains(page, "<svg") {
		t.Fatalf("dashboard missing scoreboard/chart")
	}

	// Check-in POST -> PRG redirect with flash.
	form := strings.NewReader("miles=1.5&rowed_at=" + time.Now().Format("2006-01-02T15:04"))
	req, _ := http.NewRequest("POST", ts.URL+"/checkin", form)
	req.SetBasicAuth("misty", "misty-row")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSeeOther || !strings.Contains(resp.Header.Get("Location"), "flash=") {
		t.Fatalf("checkin POST: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}

	// Bad input -> error flash.
	req, _ = http.NewRequest("POST", ts.URL+"/checkin", strings.NewReader("miles=abc"))
	req.SetBasicAuth("misty", "misty-row")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Header.Get("Location"), "flash_color=error") {
		t.Fatal("bad input: expected error flash")
	}

	// History with delete forms.
	resp = get("/history", "geoff", "geoff-row")
	page = body(resp)
	if resp.StatusCode != 200 || !strings.Contains(page, "/checkins/") {
		t.Fatalf("history: got %d", resp.StatusCode)
	}

	// Delete own; cross-user delete blocked.
	own, _ := tracker.ForUser(context.Background(), u1.ID, 10)
	req, _ = http.NewRequest("POST", fmt.Sprintf("%s/checkins/%d/delete", ts.URL, own[0].ID), nil)
	req.SetBasicAuth("geoff", "geoff-row")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("delete: got %d", resp.StatusCode)
	}
	req, _ = http.NewRequest("POST", fmt.Sprintf("%s/checkins/%d/delete", ts.URL, own[1].ID), nil)
	req.SetBasicAuth("misty", "misty-row")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Header.Get("Location"), "flash_color=error") {
		t.Fatal("cross-delete: expected error flash")
	}

	// Static + health.
	resp = get("/assets/styles.css", "geoff", "geoff-row")
	if resp.StatusCode != 200 {
		t.Fatalf("styles.css: got %d", resp.StatusCode)
	}
	resp, err = http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("healthz: got %d", resp.StatusCode)
	}

	// EnsureUsers is idempotent (second run doesn't fail or duplicate).
	if err := authStore.EnsureUsers(context.Background(), []auth.User{
		{Username: "geoff", PasswordHash: "geoff-row"},
		{Username: "misty", PasswordHash: "misty-row"},
	}, 4); err != nil {
		t.Fatal("re-seed:", err)
	}
	var n int
	if err := dbh.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("users: want 2, got %d", n)
	}

	_ = os.Setenv("TRACK_UNUSED", "")
	fmt.Println("SMOKE OK")
}
