package ai_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/argues/argus/internal/ai"
)

// fastClient builds a DeepSeek client pointed at the test server with retries
// disabled, so the suite stays under a second total.
func fastClient(t *testing.T, baseURL string) *ai.DeepSeekClient {
	t.Helper()
	c := ai.NewDeepSeekClient("sk-test-key", slog.New(slog.DiscardHandler))
	c.SetBaseURL(baseURL)
	c.SetRetryConfig(0, 1*time.Millisecond, 5*time.Millisecond)
	return c
}

// retryingClient builds a client with a small retry budget for the few
// tests that explicitly want to exercise the retry path.
func retryingClient(t *testing.T, baseURL string, max int) *ai.DeepSeekClient {
	t.Helper()
	c := ai.NewDeepSeekClient("sk-test-key", slog.New(slog.DiscardHandler))
	c.SetBaseURL(baseURL)
	c.SetRetryConfig(max, 1*time.Millisecond, 5*time.Millisecond)
	return c
}

func TestNewDeepSeekClient(t *testing.T) {
	t.Parallel()
	if ai.NewDeepSeekClient("sk-test-key", slog.New(slog.DiscardHandler)) == nil {
		t.Fatal("NewDeepSeekClient() returned nil")
	}
}

func TestNewDeepSeekClientEmptyKey(t *testing.T) {
	t.Parallel()
	if ai.NewDeepSeekClient("", slog.New(slog.DiscardHandler)) == nil {
		t.Fatal("NewDeepSeekClient() with empty key returned nil")
	}
}

func TestNewDeepSeekClientNilLogger(t *testing.T) {
	t.Parallel()
	// Production never passes nil; this just guards the constructor against
	// a regression where the field were dereferenced eagerly.
	if ai.NewDeepSeekClient("test-key", nil) == nil {
		t.Fatal("NewDeepSeekClient() with nil logger returned nil")
	}
}

func TestChatSuccess(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/chat/completions") {
			t.Errorf("expected /chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test-key" {
			t.Errorf("expected Bearer token, got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "chat-1",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "Hello, I am DeepSeek!"}, "finish_reason": "stop"}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
		}`))
	}))
	defer server.Close()

	c := fastClient(t, server.URL)
	got, err := c.Chat(context.Background(), []ai.Message{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if got != "Hello, I am DeepSeek!" {
		t.Errorf("unexpected reply: %q", got)
	}
}

func TestChatRecordsUsage(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chat-2",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "ok"}, "finish_reason": "stop"}],
			"usage": {"prompt_tokens": 7, "completion_tokens": 11, "total_tokens": 18}
		}`))
	}))
	defer server.Close()

	c := fastClient(t, server.URL)
	var model string
	var prompt, completion int
	c.SetUsageRecorder(func(m string, p, comp int) {
		model = m
		prompt = p
		completion = comp
	})
	if _, err := c.Chat(context.Background(), []ai.Message{{Role: "user", Content: "x"}}); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if prompt != 7 || completion != 11 {
		t.Errorf("usage recorder got prompt=%d completion=%d, want 7/11", prompt, completion)
	}
	if model == "" {
		t.Error("usage recorder got empty model name")
	}
}

func TestChatHandlesServerError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()

	c := fastClient(t, server.URL)
	_, err := c.Chat(context.Background(), []ai.Message{{Role: "user", Content: "x"}})
	if err == nil {
		t.Fatal("expected an error on 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention 500; got %v", err)
	}
}

func TestChatHandlesBadRequestNoRetry(t *testing.T) {
	t.Parallel()
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "invalid request"}`))
	}))
	defer server.Close()

	// Give it a retry budget the test would observe if 400 were retried.
	c := retryingClient(t, server.URL, 3)
	_, err := c.Chat(context.Background(), []ai.Message{{Role: "user", Content: "x"}})
	if err == nil {
		t.Fatal("expected an error on 400, got nil")
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Errorf("400 must not retry; got %d attempts", got)
	}
}

func TestChatCancelledContext(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow the server so the cancelled context wins before the response.
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`{"choices": [{"message": {"content": "late"}}]}`))
	}))
	defer server.Close()

	c := fastClient(t, server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Chat(ctx, []ai.Message{{Role: "user", Content: "x"}})
	if err == nil {
		t.Fatal("expected an error from a cancelled context, got nil")
	}
}

func TestChatResponseParsing(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chat-test",
			"choices": [
				{"index": 0, "message": {"role": "assistant", "content": "Test response"}, "finish_reason": "stop"}
			],
			"usage": {"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8}
		}`))
	}))
	defer server.Close()

	c := fastClient(t, server.URL)
	got, err := c.Chat(context.Background(), []ai.Message{{Role: "user", Content: "x"}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if got != "Test response" {
		t.Errorf("expected 'Test response', got %q", got)
	}
}

func TestChatTimeout(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		_, _ = w.Write([]byte(`{"choices": [{"message": {"content": "slow"}}], "usage": {}}`))
	}))
	defer server.Close()

	c := fastClient(t, server.URL)
	c.SetHTTPTimeout(30 * time.Millisecond)

	_, err := c.Chat(context.Background(), []ai.Message{{Role: "user", Content: "x"}})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestRetryOn429ThenSuccess(t *testing.T) {
	t.Parallel()
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error": "rate limited"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "ok",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "finally"}, "finish_reason": "stop"}],
			"usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
		}`))
	}))
	defer server.Close()

	c := retryingClient(t, server.URL, 3)
	got, err := c.Chat(context.Background(), []ai.Message{{Role: "user", Content: "x"}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if got != "finally" {
		t.Errorf("expected 'finally', got %q", got)
	}
	if n := atomic.LoadInt32(&attempts); n != 3 {
		t.Errorf("expected 3 attempts (2 retries then success); got %d", n)
	}
}

func TestRetryOn429Exhausted(t *testing.T) {
	t.Parallel()
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error": "still rate limited"}`))
	}))
	defer server.Close()

	c := retryingClient(t, server.URL, 2)
	_, err := c.Chat(context.Background(), []ai.Message{{Role: "user", Content: "x"}})
	if err == nil {
		t.Fatal("expected an error after exhausting retries, got nil")
	}
	if n := atomic.LoadInt32(&attempts); n != 3 {
		t.Errorf("expected 3 attempts (initial + 2 retries); got %d", n)
	}
}

func TestChatHandlesEmptyChoices(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": "empty", "choices": [], "usage": {}}`))
	}))
	defer server.Close()

	c := fastClient(t, server.URL)
	_, err := c.Chat(context.Background(), []ai.Message{{Role: "user", Content: "x"}})
	if err == nil {
		t.Fatal("expected an error when choices is empty, got nil")
	}
	if !strings.Contains(err.Error(), "no choices") {
		t.Errorf("error should mention 'no choices'; got %v", err)
	}
}

func TestChatHandlesMalformedJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	c := fastClient(t, server.URL)
	_, err := c.Chat(context.Background(), []ai.Message{{Role: "user", Content: "x"}})
	if err == nil {
		t.Fatal("expected a decode error on malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("error should mention decode; got %v", err)
	}
}

func TestIsRetryableViaPublicSurface(t *testing.T) {
	// Drive isRetryable indirectly: 502 should retry, 401 should not.
	t.Parallel()

	for _, tc := range []struct {
		name        string
		status      int
		wantRetried bool
	}{
		{"502 retried", http.StatusBadGateway, true},
		{"503 retried", http.StatusServiceUnavailable, true},
		{"504 retried", http.StatusGatewayTimeout, true},
		{"401 not retried", http.StatusUnauthorized, false},
		{"403 not retried", http.StatusForbidden, false},
		{"404 not retried", http.StatusNotFound, false},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var attempts int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&attempts, 1)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"error": "x"}`))
			}))
			defer server.Close()

			c := retryingClient(t, server.URL, 2)
			_, err := c.Chat(context.Background(), []ai.Message{{Role: "user", Content: "x"}})
			if err == nil {
				t.Fatal("expected error")
			}
			got := atomic.LoadInt32(&attempts)
			if tc.wantRetried && got == 1 {
				t.Errorf("status %d should retry; got %d attempts", tc.status, got)
			}
			if !tc.wantRetried && got != 1 {
				t.Errorf("status %d should NOT retry; got %d attempts", tc.status, got)
			}
		})
	}
}

func TestMessageTypeStructure(t *testing.T) {
	t.Parallel()
	msg := ai.Message{Role: "user", Content: "What's happening?"}
	if msg.Role != "user" {
		t.Errorf("expected Role 'user', got %q", msg.Role)
	}
	if msg.Content != "What's happening?" {
		t.Errorf("expected Content, got %q", msg.Content)
	}
	sysMsg := ai.Message{Role: "system", Content: "You are an SRE."}
	if sysMsg.Role != "system" {
		t.Errorf("expected Role 'system', got %q", sysMsg.Role)
	}
}

func TestMultipleMessages(t *testing.T) {
	t.Parallel()
	messages := []ai.Message{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "User message 1"},
		{Role: "assistant", Content: "Assistant response 1"},
		{Role: "user", Content: "User message 2"},
	}
	if len(messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(messages))
	}
	if messages[0].Role != "system" {
		t.Errorf("expected first message role 'system', got %q", messages[0].Role)
	}
}
