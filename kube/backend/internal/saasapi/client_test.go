package saasapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// TestClient_IsConfigured pins the audit bug-1 fix: an empty API key
// means "not signed in", not "wrong credentials". The previous code
// constructed the client unconditionally and emitted Authorization
// headers iff apiKey != ""; methods returned a generic 401 ErrUnauthorized
// instead of a precise "not configured" signal. IsConfigured + the new
// short-circuit in do() together close that gap.
func TestClient_IsConfigured(t *testing.T) {
	cases := []struct {
		name   string
		apiKey string
		want   bool
	}{
		{"empty", "", false},
		{"set", "sk-test-abc", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cli := NewClient("https://saas.example", c.apiKey, testLogger())
			if got := cli.IsConfigured(); got != c.want {
				t.Errorf("IsConfigured = %v, want %v", got, c.want)
			}
		})
	}
}

func TestClient_IsConfigured_NilReceiver(t *testing.T) {
	// A nil *Client must report as not-configured rather than panic.
	// The Wails binding tests pass nil for "no SaaS configured" path.
	var cli *Client
	if cli.IsConfigured() {
		t.Error("nil client should not report as configured")
	}
}

// TestClient_Do_ShortCircuitsOnEmptyKey is the regression for the
// fail-fast path. Without it, do() would attempt the HTTP call with
// no Authorization header and surface a 401 — confusing because it's
// indistinguishable from "you typed the wrong key". With it, the
// caller gets ErrNotConfigured before any network I/O.
func TestClient_Do_ShortCircuitsOnEmptyKey(t *testing.T) {
	// We point the client at a deliberately wrong URL — the test
	// only passes if do() returns BEFORE making the request.
	cli := NewClient("http://127.0.0.1:1", "", testLogger())
	err := cli.do(context.Background(), http.MethodGet, "/whatever", nil, nil)
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
}

func TestClient_Do_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization header = %q, want 'Bearer test-key'", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	cli := NewClient(srv.URL, "test-key", testLogger())
	var out struct {
		OK bool `json:"ok"`
	}
	if err := cli.do(context.Background(), http.MethodGet, "/v1/x", nil, &out); err != nil {
		t.Fatalf("do: %v", err)
	}
	if !out.OK {
		t.Errorf("decoded body wrong: %+v", out)
	}
}

func TestClient_Do_ErrorMapping(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   error
	}{
		{"401 → ErrUnauthorized", http.StatusUnauthorized, ErrUnauthorized},
		{"402 → ErrInsufficientCredits", http.StatusPaymentRequired, ErrInsufficientCredits},
		{"404 → ErrNotFound", http.StatusNotFound, ErrNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(c.status)
			}))
			t.Cleanup(srv.Close)
			cli := NewClient(srv.URL, "k", testLogger())
			err := cli.do(context.Background(), http.MethodGet, "/x", nil, nil)
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}

func TestClient_Do_500_IncludesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("upstream exploded"))
	}))
	t.Cleanup(srv.Close)
	cli := NewClient(srv.URL, "k", testLogger())
	err := cli.do(context.Background(), http.MethodGet, "/x", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "upstream exploded") {
		t.Errorf("err = %q, want it to include the body", err.Error())
	}
}

func TestClient_Do_NetworkError_WrappedUnreachable(t *testing.T) {
	// Loopback port 1 is not listening; the dial fails. The
	// frontend uses this to render "platform unreachable" instead
	// of a raw Go error. Note: with backoff retries enabled, this
	// now takes ~3.5s (3 retries with jitter) — acceptable cost
	// for "this is genuinely down" feedback.
	cli := NewClient("http://127.0.0.1:1", "k", testLogger())
	err := cli.do(context.Background(), http.MethodGet, "/x", nil, nil)
	if !errors.Is(err, ErrUnreachable) {
		t.Errorf("err = %v, want wrapping ErrUnreachable", err)
	}
}

// --- Retry + circuit-breaker tests -----------------------------------------

// 5xx is retryable. We use a counter to simulate "second attempt
// succeeds" — the client should NOT propagate the first 503 to the
// caller.
func TestClient_Do_RetriesOn5xx(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	cli := NewClient(srv.URL, "k", testLogger())
	if err := cli.do(context.Background(), http.MethodGet, "/x", nil, nil); err != nil {
		t.Fatalf("expected eventual success, got %v", err)
	}
	if attempts < 2 {
		t.Errorf("attempts = %d, want >=2 (one retry)", attempts)
	}
}

// 429 is retryable.
func TestClient_Do_RetriesOn429(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cli := NewClient(srv.URL, "k", testLogger())
	if err := cli.do(context.Background(), http.MethodGet, "/x", nil, nil); err != nil {
		t.Fatalf("expected eventual success, got %v", err)
	}
	if attempts < 2 {
		t.Errorf("attempts = %d, want >=2", attempts)
	}
}

// 401 is permanent — must NOT retry. Auth errors don't fix themselves
// in a few hundred milliseconds, and retrying would burn time on every
// startup before the user has set their key.
func TestClient_Do_NoRetryOnPermanent4xx(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	cli := NewClient(srv.URL, "k", testLogger())
	err := cli.do(context.Background(), http.MethodGet, "/x", nil, nil)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("err = %v, want ErrUnauthorized", err)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (no retry on 401)", attempts)
	}
}

// After 5 consecutive failures, the breaker opens and subsequent
// calls fail fast with ErrCircuitOpen — no longer hitting the server.
func TestClient_Do_BreakerOpensAfterConsecutiveFailures(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	cli := NewClient(srv.URL, "k", testLogger())
	// Each do() retries 3 times internally, so 5 do() calls is 5
	// "logical" failures from the breaker's perspective (each
	// retry chain returns one final failure to Execute()).
	for i := 0; i < 5; i++ {
		_ = cli.do(context.Background(), http.MethodGet, "/x", nil, nil)
	}
	// 6th call should fail fast.
	beforeAttempts := attempts
	err := cli.do(context.Background(), http.MethodGet, "/x", nil, nil)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("err = %v, want ErrCircuitOpen", err)
	}
	if attempts != beforeAttempts {
		t.Errorf("server received %d attempts after breaker open; should be %d (no new requests)", attempts, beforeAttempts)
	}
}
