package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const (
	defaultBaseURL = "https://api.deepseek.com/v1"
	defaultModel   = "deepseek-chat"
)

// Message is an OpenAI-compatible chat message.
type Message struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

// ChatRequest is the request body for the chat completions endpoint.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream"`
}

// ChatResponse is the response from the chat completions endpoint.
type ChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Index   int     `json:"index"`
		Message Message `json:"message"`
		Finish  string  `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Retry configuration for transient API failures.
const (
	maxRetries     = 3
	initialBackoff = 500 * time.Millisecond
	maxBackoff     = 10 * time.Second
)

// DeepSeekClient is an OpenAI-compatible HTTP client for DeepSeek.
type DeepSeekClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewDeepSeekClient creates a DeepSeek API client.
func NewDeepSeekClient(apiKey string, logger *slog.Logger) *DeepSeekClient {
	return &DeepSeekClient{
		baseURL: defaultBaseURL,
		apiKey:  apiKey,
		model:   defaultModel,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger,
	}
}

// isRetryable returns true for status codes that warrant a retry.
func isRetryable(code int) bool {
	return code == http.StatusTooManyRequests ||
		code == http.StatusBadGateway ||
		code == http.StatusServiceUnavailable ||
		code == http.StatusGatewayTimeout
}

// Chat sends a chat completion request and returns the assistant message.
func (c *DeepSeekClient) Chat(ctx context.Context, messages []Message) (string, error) {
	req := ChatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: 0.3,
		MaxTokens:   2048,
		Stream:      false,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	c.logger.DebugContext(ctx, "deepseek chat request",
		slog.Int("messages", len(messages)),
	)

	var chatResp ChatResponse
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			c.logger.WarnContext(ctx, "deepseek retrying",
				slog.Int("attempt", attempt),
				slog.Duration("backoff", backoff),
			)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}

			// Rebuild the request body (reader is consumed after first attempt).
			httpReq, err = http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
			if err != nil {
				return "", fmt.Errorf("create request: %w", err)
			}
			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		}

		resp, doErr := c.httpClient.Do(httpReq)
		if doErr != nil {
			if attempt < maxRetries {
				continue
			}
			return "", fmt.Errorf("deepseek request: %w", doErr)
		}

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if attempt < maxRetries && isRetryable(resp.StatusCode) {
				// Check for Retry-After header from rate limiter.
				if ra := resp.Header.Get("Retry-After"); ra != "" {
					if d, parseErr := time.ParseDuration(ra + "s"); parseErr == nil {
						backoff = d
					}
				}
				continue
			}
			return "", fmt.Errorf("deepseek returned %d: %s", resp.StatusCode, string(respBody))
		}

		if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
			resp.Body.Close()
			return "", fmt.Errorf("decode response: %w", err)
		}
		resp.Body.Close()
		break
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("deepseek returned no choices")
	}

	c.logger.DebugContext(ctx, "deepseek chat response",
		slog.Int("promptTokens", chatResp.Usage.PromptTokens),
		slog.Int("completionTokens", chatResp.Usage.CompletionTokens),
	)

	return chatResp.Choices[0].Message.Content, nil
}
