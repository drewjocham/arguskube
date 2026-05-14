package saasapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/sony/gobreaker"
)

var (
	ErrUnauthorized       = errors.New("saas: unauthorized — check your API key")
	ErrInsufficientCredits = errors.New("saas: insufficient credits")
	ErrNotFound           = errors.New("saas: resource not found")
	ErrUnreachable        = errors.New("saas: platform unreachable")
	// ErrNotConfigured is returned by every Client method when the
	// API key is empty. Previously the client was constructed
	// unconditionally with apiKey="" and every call resulted in a
	// confusing 401 from the SaaS endpoint. Now callers can
	// distinguish "you haven't connected your account yet" from
	// "your key is wrong".
	ErrNotConfigured = errors.New("saas: client not configured — set the SaaS API key in Settings")
	// ErrCircuitOpen is returned when the breaker has tripped from
	// repeated failures. The frontend renders this as a "platform
	// degraded, give it a moment" toast — it's not the same as
	// ErrUnreachable (a single failed dial) because it means we
	// observed multiple failures and stopped trying.
	ErrCircuitOpen = errors.New("saas: circuit breaker open — recent failures, retry shortly")
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
	// breaker trips when too many consecutive failures hit the SaaS
	// endpoint. Default tuning: 5 consecutive failures open it, 30s
	// half-open probe, 60s full-reset. This stops a degraded backend
	// from getting hammered with retries while also recovering on
	// its own without operator intervention.
	breaker *gobreaker.CircuitBreaker
}

func NewClient(baseURL, apiKey string, logger *slog.Logger) *Client {
	cbLogger := logger.With("component", "saasapi.breaker")
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "saasapi",
		MaxRequests: 1, // one probe in half-open before deciding
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Trip after 5 consecutive failures. Total-count
			// heuristics like "failure ratio" would be confused by
			// the desktop app's bursty traffic pattern (long idle
			// + sudden batch); consecutive-count is the right shape.
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(_ string, from, to gobreaker.State) {
			cbLogger.Info("circuit breaker state changed",
				slog.String("from", from.String()),
				slog.String("to", to.String()),
			)
		},
	})
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:  logger.With("component", "saasapi"),
		breaker: cb,
	}
}

// IsConfigured reports whether the client has an API key. Callers
// should check this before calling any method — every method
// short-circuits to ErrNotConfigured otherwise, but the typed-error
// path is for catching mis-wiring; the surface for the frontend is
// "is the user signed in yet?" which is exactly this.
func (c *Client) IsConfigured() bool {
	return c != nil && c.apiKey != ""
}

// do is the public entrypoint. It wraps the actual HTTP call in two
// resilience layers:
//
//   1. Circuit breaker (outer): when the SaaS endpoint is repeatedly
//      failing, stop trying entirely for the breaker's timeout window.
//      Returns ErrCircuitOpen so the frontend can show "platform
//      degraded" instead of accumulating timeouts.
//   2. Exponential backoff (inner): one logical RPC retries 3 times
//      with jittered backoff (250ms → 500ms → 1s) for transient
//      errors (dial failure, 5xx, 429). Permanent errors (4xx other
//      than 429, decode errors, auth) fail fast without retrying.
//
// Total worst-case wall time per do() call: ~3.5s of retries +
// however long each attempt takes. The outer ctx still bounds
// everything; if the caller cancels, both layers exit.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	if c == nil || c.apiKey == "" {
		return ErrNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	expo := backoff.NewExponentialBackOff()
	expo.InitialInterval = 250 * time.Millisecond
	expo.MaxInterval = 2 * time.Second
	expo.MaxElapsedTime = 0 // bounded by retry count below
	expo.Multiplier = 2
	expo.RandomizationFactor = 0.3

	bo := backoff.WithContext(backoff.WithMaxRetries(expo, 3), ctx)

	attempt := func() error {
		return c.doOnce(ctx, method, path, body, out)
	}

	cbResult, cbErr := c.breaker.Execute(func() (any, error) {
		err := backoff.Retry(attempt, bo)
		if err == nil {
			return nil, nil
		}
		return nil, err
	})
	_ = cbResult
	if cbErr == nil {
		return nil
	}
	if errors.Is(cbErr, gobreaker.ErrOpenState) {
		return ErrCircuitOpen
	}
	if errors.Is(cbErr, gobreaker.ErrTooManyRequests) {
		// Half-open admitted only the probe; subsequent calls bounce.
		return ErrCircuitOpen
	}
	return cbErr
}

// doOnce is one HTTP round-trip. Returns a plain error for transient
// failures (network blip, 5xx, 429) — backoff.Retry will retry on
// those. Returns backoff.Permanent(err) for permanent failures (4xx
// other than 429, decode failure, marshal failure) — backoff.Retry
// stops immediately on those.
func (c *Client) doOnce(ctx context.Context, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return backoff.Permanent(fmt.Errorf("saas: marshal request: %w", err))
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return backoff.Permanent(fmt.Errorf("saas: create request: %w", err))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Network-layer failure — almost always transient. Retry.
		return fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		if out == nil {
			return nil
		}
		limited := io.LimitReader(resp.Body, 10<<20)
		if err := json.NewDecoder(limited).Decode(out); err != nil {
			return backoff.Permanent(fmt.Errorf("saas: decode response: %w", err))
		}
		return nil
	case http.StatusNoContent:
		return nil
	case http.StatusUnauthorized:
		return backoff.Permanent(ErrUnauthorized)
	case http.StatusPaymentRequired:
		return backoff.Permanent(ErrInsufficientCredits)
	case http.StatusNotFound:
		return backoff.Permanent(ErrNotFound)
	case http.StatusTooManyRequests:
		// Rate-limited — transient. Honor-Retry-After is a follow-up.
		return fmt.Errorf("saas: rate limited (HTTP 429)")
	case http.StatusUnprocessableEntity:
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return backoff.Permanent(fmt.Errorf("saas: invalid request (422): %s", strings.TrimSpace(string(bodyBytes))))
	}

	if resp.StatusCode >= 500 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		msg := strings.TrimSpace(string(bodyBytes))
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf("saas: %s (HTTP %d)", msg, resp.StatusCode)
	}

	// Other 4xx — permanent.
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	msg := strings.TrimSpace(string(bodyBytes))
	if msg == "" {
		msg = http.StatusText(resp.StatusCode)
	}
	return backoff.Permanent(fmt.Errorf("saas: %s (HTTP %d)", msg, resp.StatusCode))
}
