package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/argus/api-gov/internal/config"
	"github.com/argus/api-gov/internal/service"
)

// newTrafficAPI returns an API whose agentCli points at a nonexistent
// endpoint. The handler dispatches IngestTraffic in a goroutine, so any
// downstream network failure happens out-of-band and does not affect
// the response we're asserting on.
func newTrafficAPI(t *testing.T) *API {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &API{
		Config:   config.New(),
		Logger:   logger,
		agentCli: service.NewAgentClient("http://127.0.0.1:1/no-such-agent", 0.5, logger),
	}
}

func TestIngestTrafficBatch_AcceptsEveryValidSample(t *testing.T) {
	a := newTrafficAPI(t)

	body := map[string]any{
		"spec_id": "spec-xyz",
		"samples": []map[string]any{
			{
				"method":      "GET",
				"path":        "/users",
				"status_code": 200,
				"headers":     map[string]string{"x-trace": "t1"},
			},
			{
				"method":      "POST",
				"path":        "/users",
				"status_code": 201,
				"request":     map[string]any{"name": "alice"},
				"headers":     map[string]string{},
			},
			{
				"method":      "GET",
				"path":        "/users/1",
				"status_code": 404,
			},
		},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/traffic/batch", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Status   string `json:"status"`
		Accepted int    `json:"accepted"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (raw=%s)", err, w.Body.String())
	}
	if resp.Status != "ingested" {
		t.Errorf("status = %q, want 'ingested'", resp.Status)
	}
	if resp.Accepted != 3 {
		t.Errorf("accepted = %d, want 3", resp.Accepted)
	}
}

func TestIngestTrafficBatch_EmptyBatchIs202NoOp(t *testing.T) {
	a := newTrafficAPI(t)
	raw, _ := json.Marshal(map[string]any{
		"spec_id": "spec-xyz",
		"samples": []map[string]any{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/traffic/batch", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", w.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["accepted"].(float64) != 0 {
		t.Errorf("accepted = %v, want 0", resp["accepted"])
	}
}

func TestIngestTrafficBatch_MissingSpecIDIsBadRequest(t *testing.T) {
	a := newTrafficAPI(t)
	raw, _ := json.Marshal(map[string]any{
		"samples": []map[string]any{{"method": "GET", "path": "/x", "status_code": 200}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/traffic/batch", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestIngestTrafficBatch_MalformedJSONIsBadRequest(t *testing.T) {
	a := newTrafficAPI(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/traffic/batch", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestIngestTrafficBatch_OversizeBatchIsBadRequest(t *testing.T) {
	a := newTrafficAPI(t)
	samples := make([]map[string]any, trafficBatchMaxSamples+1)
	for i := range samples {
		samples[i] = map[string]any{
			"method":      "GET",
			"path":        "/x",
			"status_code": 200,
		}
	}
	raw, _ := json.Marshal(map[string]any{
		"spec_id": "spec-xyz",
		"samples": samples,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/traffic/batch", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("oversize batch should be rejected; status = %d, want 400", w.Code)
	}
}

func TestIngestTrafficBatch_AcceptsCountAtBoundary(t *testing.T) {
	a := newTrafficAPI(t)
	samples := make([]map[string]any, trafficBatchMaxSamples)
	for i := range samples {
		samples[i] = map[string]any{"method": "GET", "path": "/x", "status_code": 200}
	}
	raw, _ := json.Marshal(map[string]any{
		"spec_id": "spec-xyz",
		"samples": samples,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/traffic/batch", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("exactly N samples should be accepted; status = %d, body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["accepted"].(float64) != float64(trafficBatchMaxSamples) {
		t.Errorf("accepted = %v, want %d", resp["accepted"], trafficBatchMaxSamples)
	}
}

func TestIngestTrafficBatch_LegacySingleEndpointStillWorks(t *testing.T) {
	// Backward compatibility: a not-yet-upgraded middleware POSTing to
	// /api/v1/traffic must keep working.
	a := newTrafficAPI(t)
	raw, _ := json.Marshal(map[string]any{
		"spec_id":     "spec-xyz",
		"method":      "GET",
		"path":        "/users",
		"status_code": 200,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/traffic", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("legacy single-sample endpoint should still work; status = %d", w.Code)
	}
}
