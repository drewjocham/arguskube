// Package kafka implements the broker.Publisher interface for Apache
// Kafka using the franz-go client. It registers itself as
// broker.KindKafka via init().
package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/oauth"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"

	broker "github.com/argues/argus/pkg/broker"
)

var _ broker.Publisher = (*Publisher)(nil)

// Publisher wraps a franz-go Client and implements broker.Publisher.
type Publisher struct {
	cfg    *broker.KafkaConfig
	logger *slog.Logger

	mu     sync.Mutex
	client *kgo.Client
}

func init() {
	broker.Register(broker.KindKafka, func(_ context.Context, cfg any, logger *slog.Logger) (broker.Publisher, error) {
		c, ok := cfg.(*broker.KafkaConfig)
		if !ok {
			return nil, fmt.Errorf("kafka: factory received wrong config type %T", cfg)
		}
		return &Publisher{cfg: c, logger: logger}, nil
	})
}

// Connect builds the kafka client and pings the cluster metadata to
// confirm connectivity. Idempotent.
func (p *Publisher) Connect(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client != nil {
		return nil
	}

	opts, err := p.buildOpts()
	if err != nil {
		return err
	}

	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return mapErr(err)
	}

	// Ping: issue a metadata request to detect auth/connectivity failures
	// at connect time rather than first-publish time.
	if err := cl.Ping(ctx); err != nil {
		cl.Close()
		return mapErr(err)
	}

	p.client = cl
	p.logger.Info("kafka connected", "brokers", p.cfg.BootstrapServers, "authMode", p.cfg.AuthMode)
	return nil
}

// Publish sends a single record and blocks until the broker has
// acknowledged it (per the configured Acks level). Ack latency is
// measured from call entry to ProduceSync return.
func (p *Publisher) Publish(ctx context.Context, msg broker.Message) (broker.Receipt, error) {
	p.mu.Lock()
	cl := p.client
	p.mu.Unlock()
	if cl == nil {
		return broker.Receipt{}, fmt.Errorf("kafka publish: %w", broker.ErrNotConnected)
	}

	var headers []kgo.RecordHeader
	for k, v := range msg.Headers {
		headers = append(headers, kgo.RecordHeader{Key: k, Value: []byte(v)})
	}

	rec := &kgo.Record{
		Topic:   msg.Destination,
		Value:   msg.Payload,
		Headers: headers,
	}
	if msg.Key != "" {
		rec.Key = []byte(msg.Key)
	}

	start := time.Now()
	results := cl.ProduceSync(ctx, rec)
	if err := results.FirstErr(); err != nil {
		return broker.Receipt{}, mapErr(err)
	}
	lat := time.Since(start)

	r, _ := results.First()
	msgID := ""
	if r != nil {
		msgID = fmt.Sprintf("%s/%d/%d", r.Topic, r.Partition, r.Offset)
	}

	return broker.Receipt{
		PublishedAt: start,
		AckLatency:  lat,
		MessageID:   msgID,
	}, nil
}

// Close shuts down the franz-go client. Safe to call multiple times.
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == nil {
		return nil
	}
	p.client.Close()
	p.client = nil
	return nil
}

// Kind returns broker.KindKafka.
func (p *Publisher) Kind() broker.Kind { return broker.KindKafka }

// ---- helpers ----

func (p *Publisher) buildOpts() ([]kgo.Opt, error) {
	seeds := splitSeeds(p.cfg.BootstrapServers)
	opts := []kgo.Opt{
		kgo.SeedBrokers(seeds...),
	}

	if p.cfg.ClientID != "" {
		opts = append(opts, kgo.ClientID(p.cfg.ClientID))
	} else {
		opts = append(opts, kgo.ClientID("argus-loadtest"))
	}

	// Acks. Idempotent writes (the default) require acks=all.
	// For leader or none, we must disable idempotency explicitly.
	switch strings.ToLower(p.cfg.Acks) {
	case "", "all":
		opts = append(opts, kgo.RequiredAcks(kgo.AllISRAcks()))
	case "leader":
		opts = append(opts,
			kgo.RequiredAcks(kgo.LeaderAck()),
			kgo.DisableIdempotentWrite(),
		)
	case "none":
		opts = append(opts,
			kgo.RequiredAcks(kgo.NoAck()),
			kgo.DisableIdempotentWrite(),
		)
	default:
		return nil, fmt.Errorf("kafka: unknown acks value %q", p.cfg.Acks)
	}

	// Auth / TLS
	saslOpts, tlsCfg, err := p.buildAuth()
	if err != nil {
		return nil, err
	}
	if len(saslOpts) > 0 {
		opts = append(opts, saslOpts...)
	}
	if tlsCfg != nil {
		opts = append(opts, kgo.DialTLSConfig(tlsCfg))
	}

	return opts, nil
}

func (p *Publisher) buildAuth() ([]kgo.Opt, *tls.Config, error) {
	var saslOpts []kgo.Opt
	var tlsCfg *tls.Config

	switch p.cfg.AuthMode {
	case "none", "":
		// no auth

	case "plain":
		saslOpts = append(saslOpts, kgo.SASL(
			plain.Auth{User: p.cfg.Username, Pass: p.cfg.Password}.AsMechanism(),
		))

	case "scram_sha256":
		saslOpts = append(saslOpts, kgo.SASL(
			scram.Auth{User: p.cfg.Username, Pass: p.cfg.Password}.AsSha256Mechanism(),
		))

	case "scram_sha512":
		saslOpts = append(saslOpts, kgo.SASL(
			scram.Auth{User: p.cfg.Username, Pass: p.cfg.Password}.AsSha512Mechanism(),
		))

	case "oauthbearer":
		if p.cfg.OAuthBearerToken == "" {
			return nil, nil, fmt.Errorf("kafka: authMode=oauthbearer requires oauthBearerToken: %w", broker.ErrAuth)
		}
		token := p.cfg.OAuthBearerToken
		saslOpts = append(saslOpts, kgo.SASL(oauth.Auth{Token: token}.AsMechanism()))

	case "mtls":
		tc, err := buildTLS(p.cfg.TLSCACert, p.cfg.TLSClientCert, p.cfg.TLSClientKey, p.cfg.InsecureSkipVerify)
		if err != nil {
			return nil, nil, fmt.Errorf("kafka: mtls: %w: %v", broker.ErrAuth, err)
		}
		tlsCfg = tc

	default:
		return nil, nil, fmt.Errorf("kafka: unknown authMode %q: %w", p.cfg.AuthMode, broker.ErrAuth)
	}

	// If we have extra TLS material (CA/client cert) but are not in
	// mtls mode, still apply it for transport security.
	if p.cfg.AuthMode != "mtls" && (p.cfg.TLSCACert != "" || p.cfg.InsecureSkipVerify) {
		tc, err := buildTLS(p.cfg.TLSCACert, "", "", p.cfg.InsecureSkipVerify)
		if err != nil {
			return nil, nil, fmt.Errorf("kafka: tls: %w", err)
		}
		if tlsCfg == nil {
			tlsCfg = tc
		}
	}

	return saslOpts, tlsCfg, nil
}

func buildTLS(caCert, clientCert, clientKey string, insecure bool) (*tls.Config, error) {
	cfg := &tls.Config{InsecureSkipVerify: insecure} //nolint:gosec

	if caCert != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(caCert)) {
			return nil, fmt.Errorf("failed to parse CA certificate PEM")
		}
		cfg.RootCAs = pool
	}

	if clientCert != "" && clientKey != "" {
		cert, err := tls.X509KeyPair([]byte(clientCert), []byte(clientKey))
		if err != nil {
			return nil, fmt.Errorf("client cert/key: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}

	return cfg, nil
}

func splitSeeds(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// mapErr converts franz-go errors to broker sentinel errors.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "SASL") || strings.Contains(msg, "authentication") ||
		strings.Contains(msg, "Authentication"):
		return fmt.Errorf("kafka: %w: %v", broker.ErrAuth, err)
	case strings.Contains(msg, "TOPIC_AUTHORIZATION_FAILED") || strings.Contains(msg, "authorization"):
		return fmt.Errorf("kafka: %w: %v", broker.ErrPermission, err)
	case strings.Contains(msg, "UNKNOWN_TOPIC") || strings.Contains(msg, "TOPIC_DOES_NOT_EXIST") ||
		strings.Contains(msg, "LEADER_NOT_AVAILABLE"):
		return fmt.Errorf("kafka: %w: %v", broker.ErrDestination, err)
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "context deadline exceeded"):
		return fmt.Errorf("kafka: %w: %v", broker.ErrTimeout, err)
	case strings.Contains(msg, "MESSAGE_TOO_LARGE") || strings.Contains(msg, "too large"):
		return fmt.Errorf("kafka: %w: %v", broker.ErrPayloadTooLarge, err)
	default:
		return fmt.Errorf("kafka: %w", err)
	}
}
