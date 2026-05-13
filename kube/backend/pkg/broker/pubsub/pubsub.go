// Package pubsub implements the broker.Publisher interface for Google
// Cloud Pub/Sub. It registers itself as broker.KindPubSub via init()
// so callers only need a blank import.
package pubsub

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/option"

	broker "github.com/argues/argus/pkg/broker"
)

var _ broker.Publisher = (*Publisher)(nil)

// Publisher wraps a Pub/Sub client and implements broker.Publisher.
// One Publisher corresponds to one Google Cloud project. Individual
// Publish calls resolve the topic by name (msg.Destination) — topics
// are cached so we don't hammer the metadata API.
type Publisher struct {
	cfg    *broker.PubSubConfig
	logger *slog.Logger

	// extraOpts are merged with the config-derived options at Connect
	// time. Tests use this to inject option.WithGRPCConn so that
	// the client talks to a pstest.Server instead of googleapis.com.
	extraOpts []option.ClientOption

	mu     sync.Mutex
	client *pubsub.Client
	topics map[string]*pubsub.Topic // destination → topic handle
}

func init() {
	broker.Register(broker.KindPubSub, func(_ context.Context, cfg any, logger *slog.Logger) (broker.Publisher, error) {
		c, ok := cfg.(*broker.PubSubConfig)
		if !ok {
			return nil, fmt.Errorf("pubsub: factory received wrong config type %T", cfg)
		}
		return &Publisher{
			cfg:    c,
			logger: logger,
			topics: make(map[string]*pubsub.Topic),
		}, nil
	})
}

// NewWithOptions constructs a Publisher with extra google-api option
// overrides. Used by tests to inject option.WithGRPCConn so the
// client talks to a pstest.Server instead of googleapis.com.
// Production code should use broker.New().
func NewWithOptions(cfg *broker.PubSubConfig, logger *slog.Logger, opts ...option.ClientOption) broker.Publisher {
	return &Publisher{
		cfg:       cfg,
		logger:    logger,
		topics:    make(map[string]*pubsub.Topic),
		extraOpts: opts,
	}
}

// Connect establishes the Pub/Sub client. It is idempotent — a second
// call while already connected returns nil immediately.
func (p *Publisher) Connect(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client != nil {
		return nil
	}

	baseOpts, err := p.clientOptions()
	if err != nil {
		return err
	}
	opts := append(baseOpts, p.extraOpts...)

	client, err := pubsub.NewClient(ctx, p.cfg.ProjectID, opts...)
	if err != nil {
		return mapErr(err)
	}
	p.client = client
	p.logger.Info("pubsub connected", "project", p.cfg.ProjectID, "authMode", p.cfg.AuthMode)
	return nil
}

// Publish sends msg.Payload to the topic named by msg.Destination and
// blocks until Pub/Sub acknowledges the message, returning the ack
// latency.
func (p *Publisher) Publish(ctx context.Context, msg broker.Message) (broker.Receipt, error) {
	p.mu.Lock()
	client := p.client
	p.mu.Unlock()
	if client == nil {
		return broker.Receipt{}, fmt.Errorf("pubsub publish: %w", broker.ErrNotConnected)
	}

	topic, err := p.topic(ctx, client, msg.Destination)
	if err != nil {
		return broker.Receipt{}, err
	}

	attrs := make(map[string]string, len(msg.Headers)+1)
	for k, v := range msg.Headers {
		attrs[k] = v
	}
	if msg.Key != "" {
		attrs["key"] = msg.Key
	}

	start := time.Now()
	res := topic.Publish(ctx, &pubsub.Message{
		Data:       msg.Payload,
		Attributes: attrs,
	})
	msgID, err := res.Get(ctx) // blocks until broker ack
	if err != nil {
		return broker.Receipt{}, mapErr(err)
	}
	lat := time.Since(start)

	return broker.Receipt{
		PublishedAt: start,
		AckLatency:  lat,
		MessageID:   msgID,
	}, nil
}

// Close stops all cached topic handles and closes the underlying client.
// Safe to call multiple times.
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == nil {
		return nil
	}
	for _, t := range p.topics {
		t.Stop()
	}
	err := p.client.Close()
	p.client = nil
	p.topics = make(map[string]*pubsub.Topic)
	return err
}

// Kind returns broker.KindPubSub.
func (p *Publisher) Kind() broker.Kind { return broker.KindPubSub }

// ---- helpers ----

// topic returns a cached *pubsub.Topic handle for the given name. If
// not cached it fetches metadata to confirm the topic exists.
func (p *Publisher) topic(ctx context.Context, client *pubsub.Client, name string) (*pubsub.Topic, error) {
	p.mu.Lock()
	t, ok := p.topics[name]
	p.mu.Unlock()
	if ok {
		return t, nil
	}

	t = client.Topic(name)
	exists, err := t.Exists(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	if !exists {
		return nil, fmt.Errorf("pubsub: topic %q: %w", name, broker.ErrDestination)
	}

	p.mu.Lock()
	p.topics[name] = t
	p.mu.Unlock()
	return t, nil
}

// clientOptions builds the google.golang.org/api/option list from the
// auth mode config.
func (p *Publisher) clientOptions() ([]option.ClientOption, error) {
	var opts []option.ClientOption

	if p.cfg.Endpoint != "" {
		opts = append(opts, option.WithEndpoint(p.cfg.Endpoint))
	}

	switch p.cfg.AuthMode {
	case "adc", "workload_identity", "":
		// Both ADC and workload-identity use the default credential
		// chain — in GKE the workload-identity token is injected there.
	case "service_account_json":
		if p.cfg.ServiceAccountJSON == "" {
			return nil, fmt.Errorf("pubsub: authMode=service_account_json requires serviceAccountJson: %w", broker.ErrAuth)
		}
		opts = append(opts, option.WithCredentialsJSON([]byte(p.cfg.ServiceAccountJSON)))
	default:
		return nil, fmt.Errorf("pubsub: unknown authMode %q: %w", p.cfg.AuthMode, broker.ErrAuth)
	}
	return opts, nil
}

// mapErr converts Pub/Sub SDK errors to broker sentinel errors.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "PermissionDenied") || strings.Contains(msg, "403"):
		return fmt.Errorf("pubsub: %w: %v", broker.ErrPermission, err)
	case strings.Contains(msg, "Unauthenticated") || strings.Contains(msg, "401"):
		return fmt.Errorf("pubsub: %w: %v", broker.ErrAuth, err)
	case strings.Contains(msg, "NotFound") || strings.Contains(msg, "404"):
		return fmt.Errorf("pubsub: %w: %v", broker.ErrDestination, err)
	case strings.Contains(msg, "DeadlineExceeded") || strings.Contains(msg, "context deadline exceeded"):
		return fmt.Errorf("pubsub: %w: %v", broker.ErrTimeout, err)
	case strings.Contains(msg, "MessageSizeExceeded") || strings.Contains(msg, "too large"):
		return fmt.Errorf("pubsub: %w: %v", broker.ErrPayloadTooLarge, err)
	default:
		return fmt.Errorf("pubsub: %w: %v", err, err)
	}
}
