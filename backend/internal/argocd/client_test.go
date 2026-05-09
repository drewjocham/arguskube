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
	c := New(Config{URL: "https://argocd.example.com"}, slog.New(slog.NewTextHandler(os.Stderr, nil)))
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

	c := New(Config{URL: srv.URL, Token: "test", Insecure: true}, slog.New(slog.NewTextHandler(os.Stderr, nil)))
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

	c := New(Config{URL: srv.URL, Token: "test", Insecure: true}, slog.New(slog.NewTextHandler(os.Stderr, nil)))
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

	c := New(Config{URL: srv.URL, Token: "test", Insecure: true}, slog.New(slog.NewTextHandler(os.Stderr, nil)))
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

	c := New(Config{URL: srv.URL, Token: "test", Insecure: true}, slog.New(slog.NewTextHandler(os.Stderr, nil)))
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

	c := New(Config{URL: srv.URL, Token: "bad-token", Insecure: true}, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	_, err := c.ListApps(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for unauthorized request")
	}
}

// TestGetAppParsesHistory verifies that status.history entries are surfaced
// on App.History (newest-first) so the UI can offer rollback.
func TestGetAppParsesHistory(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/applications/guestbook" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"metadata": map[string]interface{}{"name": "guestbook"},
			"spec": map[string]interface{}{
				"project": "default",
				"source":  map[string]interface{}{"repoURL": "https://github.com/example/app"},
			},
			"status": map[string]interface{}{
				"sync":    map[string]interface{}{"status": "Synced"},
				"health":  map[string]interface{}{"status": "Healthy"},
				"summary": map[string]interface{}{"images": []string{"nginx:1.25"}},
				"history": []map[string]interface{}{
					{"id": 1, "revision": "aaaaaaa", "deployedAt": "2026-01-01T10:00:00Z", "source": map[string]interface{}{"repoURL": "https://github.com/example/app", "path": "deploy"}},
					{"id": 2, "revision": "bbbbbbb", "deployedAt": "2026-01-02T10:00:00Z"},
					{"id": 3, "revision": "ccccccc", "deployedAt": "2026-01-03T10:00:00Z"},
				},
			},
		})
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, Token: "test", Insecure: true}, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	app, err := c.GetApp(context.Background(), "guestbook")
	if err != nil {
		t.Fatalf("GetApp failed: %v", err)
	}
	if got := len(app.History); got != 3 {
		t.Fatalf("expected 3 history entries, got %d", got)
	}
	// History should be newest-first.
	if app.History[0].ID != 3 || app.History[2].ID != 1 {
		t.Errorf("history not in newest-first order; got IDs %d, %d, %d",
			app.History[0].ID, app.History[1].ID, app.History[2].ID)
	}
	if app.History[2].Source != "https://github.com/example/app · deploy" {
		t.Errorf("expected source to combine repoURL+path, got %q", app.History[2].Source)
	}
}

// TestListProjects verifies the projects endpoint maps to a flat name slice.
func TestListProjects(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]interface{}{
				{"metadata": map[string]interface{}{"name": "default"}},
				{"metadata": map[string]interface{}{"name": "platform"}},
				{"metadata": map[string]interface{}{"name": ""}}, // should be skipped
			},
		})
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, Token: "test", Insecure: true}, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	projects, err := c.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if got := len(projects); got != 2 {
		t.Fatalf("expected 2 projects, got %d (%v)", got, projects)
	}
	if projects[0] != "default" || projects[1] != "platform" {
		t.Errorf("unexpected project list: %v", projects)
	}
}

// TestRollbackApp verifies the rollback POST hits the right endpoint with the
// supplied revision id.
func TestRollbackApp(t *testing.T) {
	var gotPath, gotMethod, gotBody string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		buf := make([]byte, 64)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, Token: "test", Insecure: true}, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err := c.RollbackApp(context.Background(), "guestbook", 7); err != nil {
		t.Fatalf("RollbackApp failed: %v", err)
	}
	if gotMethod != "POST" || gotPath != "/api/v1/applications/guestbook/rollback" {
		t.Errorf("unexpected request: %s %s", gotMethod, gotPath)
	}
	if gotBody != `{"id":7}` {
		t.Errorf("unexpected body: %q", gotBody)
	}
}
