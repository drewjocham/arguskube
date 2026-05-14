// Package amqp1 implements the broker.Publisher interface for
// AMQP 1.0 brokers (Solace, Azure Service Bus, Apache Artemis) using
// the Azure go-amqp library. It registers itself as broker.KindAMQP1
// via init().
//
// Testability note: the actual send operation is accessed through the
// amqpSender interface so tests can inject a fake without a live
// broker.
package amqp1

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	goamqp "github.com/Azure/go-amqp"

	broker "github.com/argues/argus/pkg/broker"
)

var _ broker.Publisher = (*Publisher)(nil)

// amqpSender abstracts the send operation so tests can inject a fake.
// We use a single Send call that blocks until the broker ack lands,
// measured by the implementation. The interface method returns the
// ack latency directly so the fake can control it without needing to
// construct the unexported goamqp.SendReceipt struct.
type amqpSender interface {
	// SendAndAck sends msg and blocks until the broker acknowledges it.
	// Returns the wall-clock ack latency and an error if the send fails
	// or the broker nacks the delivery.
	SendAndAck(ctx context.Context, msg *goamqp.Message) (time.Duration, error)
	Close(ctx context.Context) error
}

// AMQPSender is the exported alias used by tests.
type AMQPSender = amqpSender

// realSender wraps a *goamqp.Sender and implements amqpSender using
// SendWithReceipt so the ack latency covers the full broker round-trip.
type realSender struct {
	s *goamqp.Sender
}

func (r *realSender) SendAndAck(ctx context.Context, msg *goamqp.Message) (time.Duration, error) {
	start := time.Now()
	receipt, err := r.s.SendWithReceipt(ctx, msg, nil)
	if err != nil {
		return 0, err
	}
	if _, err := receipt.Wait(ctx); err != nil {
		return 0, fmt.Errorf("disposition wait: %w", err)
	}
	return time.Since(start), nil
}

func (r *realSender) Close(ctx context.Context) error {
	return r.s.Close(ctx)
}

// Publisher implements broker.Publisher for AMQP 1.0.
type Publisher struct {
	cfg    *broker.AMQP1Config
	logger *slog.Logger

	// senderFactory is injected by tests. When nil the production
	// connect path is used.
	senderFactory func(ctx context.Context) (AMQPSender, error)

	mu     sync.Mutex
	conn   *goamqp.Conn
	sess   *goamqp.Session
	sender amqpSender
}

func init() {
	broker.Register(broker.KindAMQP1, func(_ context.Context, cfg any, logger *slog.Logger) (broker.Publisher, error) {
		c, ok := cfg.(*broker.AMQP1Config)
		if !ok {
			return nil, fmt.Errorf("amqp1: factory received wrong config type %T", cfg)
		}
		return &Publisher{cfg: c, logger: logger}, nil
	})
}

// NewWithSender constructs a Publisher that uses the supplied
// senderFactory instead of dialing a real broker. Intended for unit
// tests — production code should use broker.New().
func NewWithSender(cfg *broker.AMQP1Config, logger *slog.Logger, factory func(ctx context.Context) (AMQPSender, error)) broker.Publisher {
	return &Publisher{
		cfg:           cfg,
		logger:        logger,
		senderFactory: factory,
	}
}

// Connect establishes the AMQP 1.0 connection, opens a session, and
// attaches a sender link. Idempotent.
func (p *Publisher) Connect(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sender != nil {
		return nil
	}

	if p.senderFactory != nil {
		s, err := p.senderFactory(ctx)
		if err != nil {
			return err
		}
		p.sender = s
		return nil
	}

	// Production dial path.
	connOpts, err := p.buildConnOptions()
	if err != nil {
		return err
	}

	conn, err := goamqp.Dial(ctx, p.cfg.URL, connOpts)
	if err != nil {
		return mapErr(err)
	}

	sess, err := conn.NewSession(ctx, nil)
	if err != nil {
		_ = conn.Close()
		return mapErr(err)
	}

	rawSender, err := sess.NewSender(ctx, p.cfg.SenderTarget, nil)
	if err != nil {
		_ = conn.Close()
		return mapErr(err)
	}

	p.conn = conn
	p.sess = sess
	p.sender = &realSender{s: rawSender}
	p.logger.Info("amqp1 connected", "url", p.cfg.URL, "target", p.cfg.SenderTarget)
	return nil
}

// Publish sends a single message and blocks until the broker
// disposition (Accepted) is received via SendWithReceipt.Wait, which
// is what gives us the ack latency.
func (p *Publisher) Publish(ctx context.Context, msg broker.Message) (broker.Receipt, error) {
	p.mu.Lock()
	sender := p.sender
	p.mu.Unlock()
	if sender == nil {
		return broker.Receipt{}, fmt.Errorf("amqp1 publish: %w", broker.ErrNotConnected)
	}

	appProps := make(map[string]any, len(msg.Headers))
	for k, v := range msg.Headers {
		appProps[k] = v
	}

	amqpMsg := &goamqp.Message{
		Data:                  [][]byte{msg.Payload},
		ApplicationProperties: appProps,
	}
	if msg.Key != "" {
		amqpMsg.Properties = &goamqp.MessageProperties{
			CorrelationID: msg.Key,
		}
	}

	start := time.Now()
	lat, err := sender.SendAndAck(ctx, amqpMsg)
	if err != nil {
		if strings.Contains(err.Error(), "disposition wait") {
			return broker.Receipt{}, fmt.Errorf("amqp1: %w: %v", broker.ErrTimeout, err)
		}
		return broker.Receipt{}, mapErr(err)
	}

	return broker.Receipt{
		PublishedAt: start,
		AckLatency:  lat,
		// AMQP 1.0 does not return a broker-assigned message ID in the
		// basic send path — the MessageID field in the published message
		// is set by the producer, not the broker.
	}, nil
}

// Close detaches the sender link and closes the connection. Safe to
// call multiple times.
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sender == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sErr := p.sender.Close(ctx)
	p.sender = nil
	p.sess = nil
	if p.conn != nil {
		cErr := p.conn.Close()
		p.conn = nil
		if sErr != nil {
			return sErr
		}
		return cErr
	}
	return sErr
}

// Kind returns broker.KindAMQP1.
func (p *Publisher) Kind() broker.Kind { return broker.KindAMQP1 }

// ---- helpers ----

func (p *Publisher) buildConnOptions() (*goamqp.ConnOptions, error) {
	opts := &goamqp.ConnOptions{}

	tlsCfg, err := p.buildTLS()
	if err != nil {
		return nil, err
	}
	if tlsCfg != nil {
		opts.TLSConfig = tlsCfg
	}

	switch p.cfg.AuthMode {
	case "none", "":
		opts.SASLType = goamqp.SASLTypeAnonymous()
	case "plain":
		opts.SASLType = goamqp.SASLTypePlain(p.cfg.Username, p.cfg.Password)
	case "external":
		opts.SASLType = goamqp.SASLTypeExternal("")
	case "bearer":
		if p.cfg.BearerToken == "" {
			return nil, fmt.Errorf("amqp1: authMode=bearer requires bearerToken: %w", broker.ErrAuth)
		}
		opts.SASLType = goamqp.SASLTypeXOAUTH2("", p.cfg.BearerToken, 0)
	default:
		return nil, fmt.Errorf("amqp1: unknown authMode %q: %w", p.cfg.AuthMode, broker.ErrAuth)
	}

	return opts, nil
}

func (p *Publisher) buildTLS() (*tls.Config, error) {
	if p.cfg.TLSCACert == "" && p.cfg.TLSClientCert == "" && !p.cfg.InsecureSkipVerify {
		return nil, nil
	}
	cfg := &tls.Config{InsecureSkipVerify: p.cfg.InsecureSkipVerify} //nolint:gosec
	if p.cfg.TLSCACert != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(p.cfg.TLSCACert)) {
			return nil, fmt.Errorf("amqp1: failed to parse CA certificate PEM")
		}
		cfg.RootCAs = pool
	}
	if p.cfg.TLSClientCert != "" && p.cfg.TLSClientKey != "" {
		cert, err := tls.X509KeyPair([]byte(p.cfg.TLSClientCert), []byte(p.cfg.TLSClientKey))
		if err != nil {
			return nil, fmt.Errorf("amqp1: client cert/key: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return cfg, nil
}

// mapErr converts go-amqp errors to broker sentinel errors.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "sasl") || strings.Contains(msg, "SASL") ||
		strings.Contains(msg, "authentication") || strings.Contains(msg, "unauthorized"):
		return fmt.Errorf("amqp1: %w: %v", broker.ErrAuth, err)
	case strings.Contains(msg, "unauthorized-access") || strings.Contains(msg, "not-allowed"):
		return fmt.Errorf("amqp1: %w: %v", broker.ErrPermission, err)
	case strings.Contains(msg, "not-found"):
		return fmt.Errorf("amqp1: %w: %v", broker.ErrDestination, err)
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "context deadline exceeded"):
		return fmt.Errorf("amqp1: %w: %v", broker.ErrTimeout, err)
	case strings.Contains(msg, "message-size-exceeded"):
		return fmt.Errorf("amqp1: %w: %v", broker.ErrPayloadTooLarge, err)
	default:
		return fmt.Errorf("amqp1: %w", err)
	}
}
