package opencode

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
	maxRetries     = 3
	initialBackoff = 500 * time.Millisecond
	maxBackoff     = 10 * time.Second
	requestTimeout = 120 * time.Second
)

type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
	logger     *slog.Logger
}

func NewClient(cfg ModelConfig, logger *slog.Logger) *Client {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		switch cfg.Provider {
		case ProviderOpenAI:
			baseURL = "https://api.openai.com/v1"
		case ProviderAnthropic:
			baseURL = "https://api.anthropic.com/v1"
		case ProviderDeepSeek:
			baseURL = "https://api.deepseek.com/v1"
		case ProviderOllama:
			baseURL = "http://localhost:11434/v1"
		default:
			baseURL = "https://api.openai.com/v1"
		}
	}

	model := cfg.Model
	if model == "" {
		switch cfg.Provider {
		case ProviderDeepSeek:
			model = "deepseek-chat"
		case ProviderOllama:
			model = "llama3"
		default:
			model = "gpt-4o-mini"
		}
	}

	return &Client{
		baseURL:    baseURL,
		apiKey:     cfg.APIKey,
		model:      model,
		httpClient: &http.Client{Timeout: requestTimeout},
		logger:     logger,
	}
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) Chat(ctx context.Context, messages []Message) (string, error) {
	req := chatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: 0.3,
		MaxTokens:   4096,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := initialBackoff * (1 << (attempt - 1))
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		resp, err := c.doRequest(ctx, body)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		if isRetryable(err) {
			continue
		}
		break
	}

	return "", fmt.Errorf("chat failed after %d retries: %w", maxRetries, lastErr)
}

func (c *Client) doRequest(ctx context.Context, body []byte) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", &httpError{code: resp.StatusCode, body: string(respBody)}
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshal: %w", err)
	}

	if chatResp.Error.Message != "" {
		return "", fmt.Errorf("api: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}

func isRetryable(err error) bool {
	var httpErr *httpError
	if !asHttpError(err, &httpErr) {
		return false
	}
	return httpErr.code == 429 || httpErr.code == 502 ||
		httpErr.code == 503 || httpErr.code == 504
}

type httpError struct {
	code int
	body string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("http %d: %s", e.code, e.body)
}

func asHttpError(err error, target **httpError) bool {
	if err == nil || target == nil {
		return false
	}
	for e := err; e != nil; {
		if he, ok := e.(*httpError); ok {
			*target = he
			return true
		}
		if unwrap, ok := e.(interface{ Unwrap() error }); ok {
			e = unwrap.Unwrap()
		} else {
			break
		}
	}
	return false
}
