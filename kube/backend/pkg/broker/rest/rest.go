// Package rest implements the broker.Publisher interface against
// arbitrary HTTP/REST endpoints. The load-tester treats the HTTP
// response (status + headers received) as the ack, so AckLatency
// reflects end-to-end request → response time. Registered as
// broker.KindREST via init().
package rest

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	broker "github.com/argues/argus/pkg/broker"
)

var _ broker.Publisher = (*Publisher)(nil)

// defaultTimeout caps a single request — HTTP servers can hang forever
// otherwise and the load-test engine would lose its goroutine pool.
const defaultTimeout = 30 * time.Second

// bodyExcerptLen bounds the body snippet we splice into non-success
// errors so a giant HTML 500 page doesn't blow up the run report.
const bodyExcerptLen = 200

// Publisher drives REST endpoints. The *http.Client is built lazily on
// Connect so the transport-level TLS config + InsecureSkipTLS toggle
// only affect one publisher instance — sharing http.DefaultClient would
// leak that into the rest of the process.
type Publisher struct {
	cfg    *broker.RESTConfig
	logger *slog.Logger

	mu        sync.Mutex
	client    *http.Client
	transport *http.Transport
}

func init() {
	broker.Register(broker.KindREST, func(_ context.Context, cfg any, logger *slog.Logger) (broker.Publisher, error) {
		c, ok := cfg.(*broker.RESTConfig)
		if !ok {
			return nil, fmt.Errorf("rest: factory received wrong config type %T", cfg)
		}
		return &Publisher{cfg: c, logger: logger}, nil
	})
}

// Connect builds the *http.Client. There's no TCP handshake to do here
// — HTTP is per-request — so we just construct a reusable transport
// with sensible pool limits. Idempotent: subsequent calls are no-ops.
//
// We deliberately skip the optional HEAD probe described in the task:
// it's confusing (false-fails on endpoints that only accept POST) and
// adds latency to the test setup phase.
func (p *Publisher) Connect(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client != nil {
		return nil
	}

	tr := &http.Transport{
		// Pool tuning: load tests fan out many concurrent goroutines,
		// so the default of 2 idle conns per host bottlenecks.
		MaxIdleConns:        256,
		MaxIdleConnsPerHost: 64,
		IdleConnTimeout:     90 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	if p.cfg.InsecureSkipTLS {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}

	timeout := defaultTimeout
	if p.cfg.TimeoutSeconds > 0 {
		timeout = time.Duration(p.cfg.TimeoutSeconds) * time.Second
	}

	p.transport = tr
	p.client = &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}

	p.logger.Info("rest connected", "baseURL", p.cfg.BaseURL, "timeout", timeout)
	return nil
}

// Publish executes a single HTTP request. Body is msg.Payload; URL is
// BaseURL + (msg.Destination or cfg.Path); method falls back to POST.
// AckLatency spans request start to response headers received (we read
// the body inside the window so total latency includes the response
// payload — the load tester cares about wall-time, not just TTFB).
func (p *Publisher) Publish(ctx context.Context, msg broker.Message) (broker.Receipt, error) {
	p.mu.Lock()
	client := p.client
	p.mu.Unlock()
	if client == nil {
		return broker.Receipt{}, fmt.Errorf("rest publish: %w", broker.ErrNotConnected)
	}

	method := strings.ToUpper(strings.TrimSpace(p.cfg.Method))
	if method == "" {
		method = http.MethodPost
	}

	// Destination overrides config Path so a single Publisher instance
	// can hit multiple URLs in the same run (e.g. /orders, /alerts).
	path := msg.Destination
	if path == "" {
		path = p.cfg.Path
	}
	url := buildURL(p.cfg.BaseURL, path)

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(msg.Payload))
	if err != nil {
		return broker.Receipt{}, fmt.Errorf("rest: build request: %w", err)
	}

	// Content-Type defaults to JSON since that's the dominant REST
	// payload format; operator can override per-publisher.
	ct := p.cfg.ContentType
	if ct == "" {
		ct = "application/json"
	}
	req.Header.Set("Content-Type", ct)

	for k, v := range p.cfg.Headers {
		req.Header.Set(k, v)
	}
	for k, v := range msg.Headers {
		req.Header.Set(k, v)
	}

	// Auth precedence: bearer wins over basic if both set — explicit
	// Authorization header from cfg.Headers wins over both because the
	// header map was applied above.
	if _, ok := req.Header["Authorization"]; !ok {
		switch {
		case p.cfg.BearerToken != "":
			req.Header.Set("Authorization", "Bearer "+p.cfg.BearerToken)
		case p.cfg.BasicAuthUser != "" || p.cfg.BasicAuthPassword != "":
			cred := p.cfg.BasicAuthUser + ":" + p.cfg.BasicAuthPassword
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(cred)))
		}
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return broker.Receipt{}, mapErr(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read body fully so the connection can be reused from the idle
	// pool — leaving it half-read disables keep-alive.
	body, readErr := io.ReadAll(resp.Body)
	lat := time.Since(start)

	if !p.isSuccess(resp.StatusCode) {
		excerpt := body
		if len(excerpt) > bodyExcerptLen {
			excerpt = excerpt[:bodyExcerptLen]
		}
		return broker.Receipt{
				PublishedAt: start,
				AckLatency:  lat,
				Bytes:       len(body),
			}, fmt.Errorf("rest: non-success status %d: %s",
				resp.StatusCode, strings.TrimSpace(string(excerpt)))
	}
	if readErr != nil {
		// Status was success but body read failed — still surface so
		// the load tester counts it as an error rather than silently
		// counting a partial response as a pass.
		return broker.Receipt{
			PublishedAt: start,
			AckLatency:  lat,
			Bytes:       len(body),
		}, fmt.Errorf("rest: read response body: %w", readErr)
	}

	return broker.Receipt{
		PublishedAt: start,
		AckLatency:  lat,
		Bytes:       len(body),
	}, nil
}

// Close closes idle transport connections so the goroutine pool inside
// http.Transport doesn't outlive the load run. Safe to call repeatedly.
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.transport != nil {
		p.transport.CloseIdleConnections()
	}
	p.client = nil
	p.transport = nil
	return nil
}

// Kind returns broker.KindREST.
func (p *Publisher) Kind() broker.Kind { return broker.KindREST }

// ---- helpers ----

func (p *Publisher) isSuccess(code int) bool {
	if len(p.cfg.SuccessCodes) > 0 {
		for _, c := range p.cfg.SuccessCodes {
			if c == code {
				return true
			}
		}
		return false
	}
	return code >= 200 && code < 300
}

// buildURL joins base + path tolerating "" / leading slash variations.
// Avoids net/url's full parse cost on the hot path.
func buildURL(base, path string) string {
	if path == "" {
		return base
	}
	if base == "" {
		return path
	}
	// If path is already absolute (starts with scheme), use as-is.
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	b := strings.TrimRight(base, "/")
	p := strings.TrimLeft(path, "/")
	return b + "/" + p
}

// mapErr translates net/http errors into broker sentinels where the
// mapping is unambiguous. Timeout vs. cancel: context.DeadlineExceeded
// is mapped to ErrTimeout; everything else is wrapped.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "context deadline exceeded"),
		strings.Contains(msg, "Client.Timeout"):
		return fmt.Errorf("rest: %w: %v", broker.ErrTimeout, err)
	default:
		return fmt.Errorf("rest: %w", err)
	}
}
