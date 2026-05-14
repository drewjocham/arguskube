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
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
}

func NewClient(baseURL, apiKey string, logger *slog.Logger) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger.With("component", "saasapi"),
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

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	if c == nil || c.apiKey == "" {
		// Fail fast with a precise error rather than letting the
		// request go out with no Authorization header and getting
		// a generic 401 back. The frontend uses this to decide
		// whether to show "Connect your account" vs "Your key is
		// wrong".
		return ErrNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("saas: marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("saas: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
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
			return fmt.Errorf("saas: decode response: %w", err)
		}
		return nil
	case http.StatusNoContent:
		return nil
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusPaymentRequired:
		return ErrInsufficientCredits
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusUnprocessableEntity:
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("saas: invalid request (422): %s", strings.TrimSpace(string(bodyBytes)))
	default:
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		msg := strings.TrimSpace(string(bodyBytes))
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf("saas: %s (HTTP %d)", msg, resp.StatusCode)
	}
}
