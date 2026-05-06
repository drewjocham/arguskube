package argocd

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNew_NilOnEmptyURL(t *testing.T) {
	c := New(Config{}, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if c != nil {
		t.Fatal("expected nil client for empty URL")
	}
}

func TestConnected(t *testing.T) {
	c := New(Config{URL: "https://argocd.example.com"}, nil)
	if c == nil || !c.Connected() {
		t.Fatal("expected connected client")
	}

	var nilClient *Client
	if nilClient.Connected() {
		t.Fatal("expected nil client to report not connected")
	}
}

func TestListApps(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/v1/applications" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]interface{}{
				{
					"metadata": map[string]interface{}{
						"name":      "guestbook",
						"namespace": "argocd",
					},
					"spec": map[string]interface{}{
						"project": "default",
						"source": map[string]interface{}{
							"repoURL":        "https://github.com/argoproj/argocd-example-apps",
							"path":           "guestbook",
							"targetRevision": "HEAD",
						},
						"destination": map[string]interface{}{
							"server":    "https://kubernetes.default.svc",
							"namespace": "default",
						},
					},
					"status": map[string]interface{}{
						"sync":   map[string]interface{}{"status": "Synced"},
						"health": map[string]interface{}{"status": "Healthy"},
					},
				},
			},
		})
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, Token: "test-token", Insecure: true}, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	apps, err := c.ListApps(context.Background(), "")
	if err != nil {
		t.Fatalf("ListApps failed: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	if apps[0].Name != "guestbook" {
		t.Errorf("expected name 'guestbook', got %q", apps[0].Name)
	}
	if apps[0].SyncStatus != "Synced" {
		t.Errorf("expected SyncStatus 'Synced', got %q", apps[0].SyncStatus)
	}
	if apps[0].HealthStatus != "Healthy" {
		t.Errorf("expected HealthStatus 'Healthy', got %q", apps[0].HealthStatus)
	}
}

func TestListApps_WithProject(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("project") != "myproject" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"items": []interface{}{}})
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, Token: "test", Insecure: true}, nil)
	apps, err := c.ListApps(context.Background(), "myproject")
	if err != nil {
		t.Fatalf("ListApps with project failed: %v", err)
	}
	if len(apps) != 0 {
		t.Errorf("expected 0 apps, got %d", len(apps))
	}
}

func TestGetApp(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/applications/guestbook" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "guestbook",
				"namespace": "argocd",
			},
			"spec": map[string]interface{}{
				"project": "default",
				"source": map[string]interface{}{
					"repoURL":        "https://github.com/argoproj/argocd-example-apps",
					"path":           "guestbook",
					"targetRevision": "HEAD",
				},
				"destination": map[string]interface{}{
					"server":    "https://kubernetes.default.svc",
					"namespace": "default",
				},
			},
			"status": map[string]interface{}{
				"sync":   map[string]interface{}{"status": "OutOfSync"},
				"health": map[string]interface{}{"status": "Degraded"},
				"summary": map[string]interface{}{
					"images": []string{"nginx:1.25"},
				},
			},
		})
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, Token: "test", Insecure: true}, nil)
	app, err := c.GetApp(context.Background(), "guestbook")
	if err != nil {
		t.Fatalf("GetApp failed: %v", err)
	}
	if app.Name != "guestbook" {
		t.Errorf("expected name 'guestbook', got %q", app.Name)
	}
	if app.SyncStatus != "OutOfSync" {
		t.Errorf("expected SyncStatus 'OutOfSync', got %q", app.SyncStatus)
	}
	if app.HealthStatus != "Degraded" {
		t.Errorf("expected HealthStatus 'Degraded', got %q", app.HealthStatus)
	}
	if app.Image != "nginx:1.25" {
		t.Errorf("expected image 'nginx:1.25', got %q", app.Image)
	}
}

func TestSyncApp(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/applications/guestbook/sync" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"phase":   "Succeeded",
			"message": "successfully synced",
		})
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, Token: "test", Insecure: true}, nil)
	result, err := c.SyncApp(context.Background(), "guestbook")
	if err != nil {
		t.Fatalf("SyncApp failed: %v", err)
	}
	if result.Phase != "Succeeded" {
		t.Errorf("expected phase 'Succeeded', got %q", result.Phase)
	}
}

func TestTestConnection(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"items": []interface{}{}})
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, Token: "test", Insecure: true}, nil)
	if err := c.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection failed: %v", err)
	}
}

func TestHTTPError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid token"}`))
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, Token: "bad-token", Insecure: true}, nil)
	_, err := c.ListApps(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for unauthorized request")
	}
}
