package api_test

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestAPI exercises the cross-cutting /api/v1 contract end-to-end: auth,
// metrics/measurements CRUD, and the error envelope (validation -> 400 with
// field, not found -> 404, conflict -> 409, system rows -> 403, cross-user
// isolation -> 404). Slice-specific behavior lives in slices_test.go.
func TestAPI(t *testing.T) {

	ts, call := apiFixture(t)
	code, _, _ := call("GET", "/api/v1/metrics", "", "", nil)
	if code != 401 {
		t.Fatalf("unauthenticated metrics: got %d, want 401", code)
	}
	code, _, _ = call("GET", "/api/v1/metrics", "geoff", "wrong", nil)
	if code != 401 {
		t.Fatalf("bad password: got %d, want 401", code)
	}

	// --- metrics catalog ---
	code, body, _ := call("GET", "/api/v1/metrics", "geoff", "pw", nil)
	if code != 200 {
		t.Fatalf("list metrics: got %d", code)
	}
	metrics := body["data"].([]any)
	if len(metrics) == 0 {
		t.Fatal("expected seeded metrics")
	}
	var weightID float64
	for _, m := range metrics {
		mm := m.(map[string]any)
		if mm["name"] == "weight" {
			weightID = mm["id"].(float64)
		}
	}
	if weightID == 0 {
		t.Fatal("weight metric not seeded")
	}

	// Custom metric create + conflict + delete.
	code, created, _ := call("POST", "/api/v1/metrics", "geoff", "pw", map[string]any{
		"name": "wingspan", "label": "Wingspan", "unit": "cm",
	})
	if code != 201 || created["name"] != "wingspan" {
		t.Fatalf("create metric: %d %v", code, created)
	}
	code, _, _ = call("POST", "/api/v1/metrics", "geoff", "pw", map[string]any{
		"name": "wingspan", "unit": "cm",
	})
	if code != 409 {
		t.Fatalf("duplicate metric: got %d, want 409", code)
	}
	code, _, _ = call("DELETE", fmt.Sprintf("/api/v1/metrics/%d", int64(weightID)), "geoff", "pw", nil)
	if code != 403 {
		t.Fatalf("delete system metric: got %d, want 403", code)
	}
	code, _, _ = call("DELETE", fmt.Sprintf("/api/v1/metrics/%d", int64(created["id"].(float64))), "geoff", "pw", nil)
	if code != 200 {
		t.Fatalf("delete custom metric: got %d", code)
	}

	// --- measurements CRUD ---
	code, m, _ := call("POST", "/api/v1/measurements", "geoff", "pw", map[string]any{
		"metric_id": weightID, "value": 82.5, "measured_at": "2026-09-28T07:30:00Z",
	})
	if code != 201 || m["value"] != 82.5 {
		t.Fatalf("create measurement: %d %v", code, m)
	}
	measID := m["id"].(float64)

	// Validation: bad metric, negative value.
	code, m, _ = call("POST", "/api/v1/measurements", "geoff", "pw", map[string]any{
		"metric_id": 99999, "value": 80,
	})
	if code != 400 || m["field"] != "metric_id" {
		t.Fatalf("unknown metric: %d %v", code, m)
	}
	code, m, _ = call("POST", "/api/v1/measurements", "geoff", "pw", map[string]any{
		"metric_id": weightID, "value": -1,
	})
	if code != 400 || m["field"] != "value" {
		t.Fatalf("negative value: %d %v", code, m)
	}

	// List with filter.
	code, body, _ = call("GET", "/api/v1/measurements?metric_id=99999", "geoff", "pw", nil)
	if code != 200 || len(body["data"].([]any)) != 0 {
		t.Fatalf("filtered list should be empty: %d", code)
	}
	code, body, _ = call("GET", "/api/v1/measurements", "geoff", "pw", nil)
	if code != 200 || len(body["data"].([]any)) != 1 {
		t.Fatalf("list measurements: %d", code)
	}

	// Latest.
	code, body, _ = call("GET", "/api/v1/measurements/latest", "geoff", "pw", nil)
	if code != 200 {
		t.Fatalf("latest: got %d", code)
	}
	if latest := body["data"].([]any); len(latest) != 1 || latest[0].(map[string]any)["metric_name"] != "weight" {
		t.Fatalf("latest wrong: %v", latest)
	}

	// Update.
	code, m, _ = call("PATCH", fmt.Sprintf("/api/v1/measurements/%d", int64(measID)), "geoff", "pw", map[string]any{
		"value": 83.0, "measured_at": "2026-09-29T07:30:00Z",
	})
	if code != 200 || m["value"] != 83.0 {
		t.Fatalf("update measurement: %d %v", code, m)
	}

	// Cross-user isolation.
	code, _, _ = call("GET", fmt.Sprintf("/api/v1/measurements/%d", int64(measID)), "misty", "pw", nil)
	if code != 404 {
		t.Fatalf("cross-user get: got %d, want 404", code)
	}
	code, _, _ = call("DELETE", fmt.Sprintf("/api/v1/measurements/%d", int64(measID)), "misty", "pw", nil)
	if code != 404 {
		t.Fatalf("cross-user delete: got %d, want 404", code)
	}

	// Delete own.
	code, m, _ = call("DELETE", fmt.Sprintf("/api/v1/measurements/%d", int64(measID)), "geoff", "pw", nil)
	if code != 200 || m["deleted"] != true {
		t.Fatalf("delete measurement: %d %v", code, m)
	}
	code, _, _ = call("GET", fmt.Sprintf("/api/v1/measurements/%d", int64(measID)), "geoff", "pw", nil)
	if code != 404 {
		t.Fatalf("get deleted: got %d, want 404", code)
	}

	// Bad id param + malformed JSON (raw string body).
	code, m, _ = call("GET", "/api/v1/measurements/abc", "geoff", "pw", nil)
	if code != 400 || m["field"] != "id" {
		t.Fatalf("bad id: %d %v", code, m)
	}
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/measurements", strings.NewReader("{invalid"))
	req.SetBasicAuth("geoff", "pw")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	rawB, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 400 || !strings.Contains(string(rawB), `"field":"body"`) {
		t.Fatalf("malformed JSON: %d %s", resp.StatusCode, rawB)
	}

	// Bad from/to query params.
	code, _, _ = call("GET", "/api/v1/measurements?from=notadate", "geoff", "pw", nil)
	if code != 400 {
		t.Fatalf("bad from param: got %d, want 400", code)
	}

	fmt.Println("API OK")
}
