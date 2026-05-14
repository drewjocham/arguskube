package rest_test

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	broker "github.com/argues/argus/pkg/broker"
	_ "github.com/argues/argus/pkg/broker/rest" // registers factory
)

func newPublisher(t *testing.T, cfg *broker.RESTConfig) broker.Publisher {
	t.Helper()
	pub, err := broker.New(context.Background(), broker.Config{
		Kind: broker.KindREST,
		REST: cfg,
	}, slog.Default())
	if err != nil {
		t.Fatalf("broker.New: %v", err)
	}
	if err := pub.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = pub.Close() })
	return pub
}

func TestREST_FactoryRegistered(t *testing.T) {
	pub, err := broker.New(context.Background(), broker.Config{
		Kind: broker.KindREST,
		REST: &broker.RESTConfig{BaseURL: "http://example.invalid"},
	}, slog.Default())
	if err != nil {
		t.Fatalf("broker.New: %v", err)
	}
	if pub.Kind() != broker.KindREST {
		t.Errorf("Kind = %v, want %v", pub.Kind(), broker.KindREST)
	}
	_ = pub.Close()
}

func TestREST_PublishBasic(t *testing.T) {
	var gotMethod, gotCT, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		if r.URL.Path != "/events" {
			t.Errorf("path = %q, want /events", r.URL.Path)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	pub := newPublisher(t, &broker.RESTConfig{
		BaseURL: srv.URL,
		Method:  "POST",
	})

	r, err := pub.Publish(context.Background(), broker.Message{
		Destination: "/events",
		Payload:     []byte(`{"k":"v"}`),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotCT != "application/json" {
		t.Errorf("content-type = %q, want application/json", gotCT)
	}
	if gotBody != `{"k":"v"}` {
		t.Errorf("body = %q, want {\"k\":\"v\"}", gotBody)
	}
	if r.AckLatency <= 0 {
		t.Errorf("AckLatency = %v, want > 0", r.AckLatency)
	}
	if r.Bytes == 0 {
		t.Error("Bytes = 0, want response body length")
	}
}

func TestREST_BearerAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pub := newPublisher(t, &broker.RESTConfig{
		BaseURL:     srv.URL,
		BearerToken: "abc123",
	})
	if _, err := pub.Publish(context.Background(), broker.Message{Payload: []byte("x")}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if gotAuth != "Bearer abc123" {
		t.Errorf("Authorization = %q, want Bearer abc123", gotAuth)
	}
}

func TestREST_BasicAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pub := newPublisher(t, &broker.RESTConfig{
		BaseURL:           srv.URL,
		BasicAuthUser:     "alice",
		BasicAuthPassword: "s3cret",
	})
	if _, err := pub.Publish(context.Background(), broker.Message{Payload: []byte("x")}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:s3cret"))
	if gotAuth != want {
		t.Errorf("Authorization = %q, want %q", gotAuth, want)
	}
}

func TestREST_NonSuccessReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("upstream exploded: db unreachable"))
	}))
	defer srv.Close()

	pub := newPublisher(t, &broker.RESTConfig{BaseURL: srv.URL})

	_, err := pub.Publish(context.Background(), broker.Message{Payload: []byte("x")})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error %q missing status code 500", err)
	}
	if !strings.Contains(err.Error(), "upstream exploded") {
		t.Errorf("error %q missing body snippet", err)
	}
}

func TestREST_NonSuccessBodyTruncated(t *testing.T) {
	huge := strings.Repeat("A", 5000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(huge))
	}))
	defer srv.Close()

	pub := newPublisher(t, &broker.RESTConfig{BaseURL: srv.URL})
	_, err := pub.Publish(context.Background(), broker.Message{Payload: []byte("x")})
	if err == nil {
		t.Fatal("expected error")
	}
	// 200-byte excerpt + some prefix text; total under 500.
	if len(err.Error()) > 500 {
		t.Errorf("error length %d, want truncated", len(err.Error()))
	}
}

func TestREST_SuccessCodesOverride(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot) // 418
		_, _ = w.Write([]byte(`brewing`))
	}))
	defer srv.Close()

	pub := newPublisher(t, &broker.RESTConfig{
		BaseURL:      srv.URL,
		SuccessCodes: []int{418},
	})
	r, err := pub.Publish(context.Background(), broker.Message{Payload: []byte("x")})
	if err != nil {
		t.Fatalf("Publish with SuccessCodes=[418]: %v", err)
	}
	if r.AckLatency <= 0 {
		t.Errorf("AckLatency = %v, want > 0", r.AckLatency)
	}
}

func TestREST_InsecureSkipTLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pub := newPublisher(t, &broker.RESTConfig{
		BaseURL:         srv.URL,
		InsecureSkipTLS: true,
	})
	if _, err := pub.Publish(context.Background(), broker.Message{Payload: []byte("x")}); err != nil {
		t.Fatalf("Publish over self-signed TLS with InsecureSkipTLS: %v", err)
	}
}

func TestREST_InsecureSkipTLSFalseRejectsSelfSigned(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pub := newPublisher(t, &broker.RESTConfig{
		BaseURL:         srv.URL,
		InsecureSkipTLS: false,
	})
	_, err := pub.Publish(context.Background(), broker.Message{Payload: []byte("x")})
	if err == nil {
		t.Fatal("expected TLS verification error, got nil")
	}
}

func TestREST_HeadersMerged(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pub := newPublisher(t, &broker.RESTConfig{
		BaseURL: srv.URL,
		Headers: map[string]string{"X-Static": "yes"},
	})
	_, err := pub.Publish(context.Background(), broker.Message{
		Headers: map[string]string{"X-Per-Message": "true"},
		Payload: []byte("x"),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if got.Get("X-Static") != "yes" {
		t.Error("static header missing")
	}
	if got.Get("X-Per-Message") != "true" {
		t.Error("per-message header missing")
	}
}

func TestREST_PublishNotConnected(t *testing.T) {
	pub, err := broker.New(context.Background(), broker.Config{
		Kind: broker.KindREST,
		REST: &broker.RESTConfig{BaseURL: "http://example.invalid"},
	}, slog.Default())
	if err != nil {
		t.Fatalf("broker.New: %v", err)
	}
	_, err = pub.Publish(context.Background(), broker.Message{Payload: []byte("x")})
	if !errors.Is(err, broker.ErrNotConnected) {
		t.Errorf("expected ErrNotConnected, got %v", err)
	}
}

func TestREST_ConnectIdempotent(t *testing.T) {
	pub, err := broker.New(context.Background(), broker.Config{
		Kind: broker.KindREST,
		REST: &broker.RESTConfig{BaseURL: "http://example.invalid"},
	}, slog.Default())
	if err != nil {
		t.Fatalf("broker.New: %v", err)
	}
	defer func() { _ = pub.Close() }()
	if err := pub.Connect(context.Background()); err != nil {
		t.Fatalf("first Connect: %v", err)
	}
	if err := pub.Connect(context.Background()); err != nil {
		t.Fatalf("second Connect should be idempotent: %v", err)
	}
}

func TestREST_CloseSafe(t *testing.T) {
	pub, err := broker.New(context.Background(), broker.Config{
		Kind: broker.KindREST,
		REST: &broker.RESTConfig{BaseURL: "http://example.invalid"},
	}, slog.Default())
	if err != nil {
		t.Fatalf("broker.New: %v", err)
	}
	_ = pub.Connect(context.Background())
	if err := pub.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := pub.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestREST_ConcurrentPublishShareTransport(t *testing.T) {
	var n int64
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pub := newPublisher(t, &broker.RESTConfig{BaseURL: srv.URL})

	const workers = 16
	const each = 8
	var wg sync.WaitGroup
	errs := make(chan error, workers*each)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < each; j++ {
				_, err := pub.Publish(context.Background(), broker.Message{Payload: []byte("x")})
				if err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Errorf("concurrent Publish: %v", e)
	}
	if n != workers*each {
		t.Errorf("server saw %d requests, want %d", n, workers*each)
	}
}

func TestREST_MethodDefaultsToPOST(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pub := newPublisher(t, &broker.RESTConfig{BaseURL: srv.URL})
	if _, err := pub.Publish(context.Background(), broker.Message{Payload: []byte("x")}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if gotMethod != "POST" {
		t.Errorf("default method = %q, want POST", gotMethod)
	}
}

func TestREST_PathFallback(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pub := newPublisher(t, &broker.RESTConfig{
		BaseURL: srv.URL,
		Path:    "/default",
	})
	// No Destination → falls back to cfg.Path.
	if _, err := pub.Publish(context.Background(), broker.Message{Payload: []byte("x")}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if gotPath != "/default" {
		t.Errorf("path = %q, want /default", gotPath)
	}
}
