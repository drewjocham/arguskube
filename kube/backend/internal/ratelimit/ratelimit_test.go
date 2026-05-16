package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestAllowUnderLimit(t *testing.T) {
	t.Parallel()
	p := New(1000, 5, time.Minute)
	for i := 0; i < 5; i++ {
		if !p.Allow("1.2.3.4") {
			t.Fatalf("Allow should not refuse request %d within burst", i)
		}
	}
}

func TestAllowBlocksOverBurst(t *testing.T) {
	t.Parallel()
	// Tiny rate so the burst is the only credit we get within the
	// test budget.
	p := New(0.1, 2, time.Minute)
	if !p.Allow("9.9.9.9") {
		t.Fatal("first request should be allowed")
	}
	if !p.Allow("9.9.9.9") {
		t.Fatal("second request (within burst) should be allowed")
	}
	if p.Allow("9.9.9.9") {
		t.Error("third request must be refused — burst exhausted")
	}
}

func TestAllowIsPerIP(t *testing.T) {
	t.Parallel()
	p := New(0.1, 1, time.Minute)
	if !p.Allow("a.a.a.a") {
		t.Fatal("first request from a.a.a.a should pass")
	}
	if !p.Allow("b.b.b.b") {
		t.Fatal("first request from b.b.b.b should pass — different IP, own bucket")
	}
	if p.Allow("a.a.a.a") {
		t.Error("second request from a.a.a.a within the same instant must be refused")
	}
}

func TestCleanupEvictsIdleVisitors(t *testing.T) {
	t.Parallel()
	p := New(100, 5, 50*time.Millisecond)
	p.Allow("transient")
	if p.Size() != 1 {
		t.Fatalf("expected 1 visitor after Allow; got %d", p.Size())
	}
	time.Sleep(70 * time.Millisecond)
	if evicted := p.Cleanup(); evicted != 1 {
		t.Errorf("expected 1 eviction; got %d", evicted)
	}
	if p.Size() != 0 {
		t.Errorf("visitor map should be empty after cleanup; size=%d", p.Size())
	}
}

func TestMiddlewareReturns429WithRetryAfter(t *testing.T) {
	t.Parallel()
	p := New(0.1, 1, time.Minute)
	called := 0
	h := Middleware(p)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	}))

	// Two requests from the same non-loopback IP: first allowed,
	// second refused with 429.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = "203.0.113.7:5000"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		switch i {
		case 0:
			if rec.Code != http.StatusOK {
				t.Errorf("first request: got %d, want 200", rec.Code)
			}
		case 1:
			if rec.Code != http.StatusTooManyRequests {
				t.Errorf("second request: got %d, want 429", rec.Code)
			}
			if ra := rec.Header().Get("Retry-After"); ra == "" {
				t.Error("429 must set Retry-After")
			} else if _, err := strconv.Atoi(ra); err != nil {
				t.Errorf("Retry-After must be numeric seconds; got %q", ra)
			}
		}
	}
	if called != 1 {
		t.Errorf("inner handler called %d times; expected 1 (one pass-through)", called)
	}
}

func TestMiddlewareBypassesLoopback(t *testing.T) {
	t.Parallel()
	// Use a stingy limiter — loopback should sail through anyway.
	p := New(0.0001, 1, time.Minute)
	called := 0
	h := Middleware(p)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	}))

	for _, ip := range []string{"127.0.0.1:9", "[::1]:9"} {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = ip
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("loopback %s: got %d, want 200", ip, rec.Code)
		}
	}
	if called != 2 {
		t.Errorf("loopback bypass: handler called %d times, want 2", called)
	}
}

func TestMiddlewarePrefersFirstXForwardedFor(t *testing.T) {
	t.Parallel()
	// Burst of 1 — second request from the SAME extracted IP must 429.
	p := New(0.1, 1, time.Minute)
	h := Middleware(p)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = "203.0.113.99:5000" // distinct from XFF
		req.Header.Set("X-Forwarded-For", "203.0.113.50, 10.0.0.1")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		want := http.StatusOK
		if i == 1 {
			want = http.StatusTooManyRequests
		}
		if rec.Code != want {
			t.Errorf("req %d: got %d, want %d (XFF should pin the bucket to 203.0.113.50)", i, rec.Code, want)
		}
	}
}
