package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/argus/api-gov/internal/config"
)

func newTestAPI() *API {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	a := &API{
		Logger: logger,
		Config: config.New(),
	}
	// safe zero-value execute — handles nil services gracefully
	a.histogramLatency = nil
	return a
}

func TestHealthEndpoint(t *testing.T) {
	a := newTestAPI()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{name: "health get", method: "GET", path: "/health", wantStatus: http.StatusOK},
		{name: "ready get", method: "GET", path: "/ready", wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			a.Routes().ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestCORS(t *testing.T) {
	a := newTestAPI()

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantHeader string
	}{
		{name: "options returns 204", path: "/health", wantStatus: http.StatusNoContent, wantHeader: "*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("OPTIONS", tt.path, nil)
			w := httptest.NewRecorder()
			a.Routes().ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if got := w.Header().Get("Access-Control-Allow-Origin"); got != tt.wantHeader {
				t.Errorf("CORS header = %q, want %q", got, tt.wantHeader)
			}
		})
	}
}

func TestContentTypeJSON(t *testing.T) {
	a := newTestAPI()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	a.Routes().ServeHTTP(w, req)

	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}

func TestDriftEndpointRouting(t *testing.T) {
	a := newTestAPI()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "get drift reports", method: "GET", path: "/api/v1/specs/abc/drift"},
		{name: "get drift summary", method: "GET", path: "/api/v1/specs/abc/drift/summary"},
		{name: "request drift scan", method: "POST", path: "/api/v1/specs/abc/drift/scan"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			a.Routes().ServeHTTP(w, req)
			// Routes exist — response is 500 because services are nil,
			// but no 404 means routing works
			if w.Code == 404 {
				t.Errorf("route not found: %s %s", tt.method, tt.path)
			}
		})
	}
}
