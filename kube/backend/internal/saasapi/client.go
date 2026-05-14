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

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
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
