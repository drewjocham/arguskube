// Package rabbitmq implements the broker.Publisher interface for
// RabbitMQ using AMQP 0.9.1. It registers itself as
// broker.KindRabbitMQ via init().
//
// Testability note: the AMQP channel operations are accessed through
// the amqpChannel interface, allowing tests to supply a fake channel
// without needing a live broker process.
package rabbitmq

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	broker "github.com/argues/argus/pkg/broker"
)

var _ broker.Publisher = (*Publisher)(nil)

// AMQPChannel abstracts the subset of *amqp.Channel methods that
// Publisher uses. Exported so tests can supply a fake implementation
// via NewWithChannel without importing the amqp package transitively.
type AMQPChannel interface {
	ExchangeDeclarePassive(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	Confirm(noWait bool) error
	PublishWithDeferredConfirmWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) (*amqp.DeferredConfirmation, error)
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	Close() error
}

// Publisher implements broker.Publisher for RabbitMQ.
type Publisher struct {
	cfg    *broker.RabbitMQConfig
	logger *slog.Logger

	// chanFactory is normally nil and the production dial path is used.
	// Tests inject a factory that returns a fake AMQPChannel, bypassing
	// the real AMQP dial.
	chanFactory func() (AMQPChannel, error)

	mu   sync.Mutex
	conn *amqp.Connection
	ch   AMQPChannel
}

func init() {
	broker.Register(broker.KindRabbitMQ, func(_ context.Context, cfg any, logger *slog.Logger) (broker.Publisher, error) {
		c, ok := cfg.(*broker.RabbitMQConfig)
		if !ok {
			return nil, fmt.Errorf("rabbitmq: factory received wrong config type %T", cfg)
		}
		return &Publisher{cfg: c, logger: logger}, nil
	})
}

// NewWithChannel constructs a Publisher that uses the supplied
// chanFactory instead of dialing a real broker. Intended for unit
// tests — production code should use broker.New().
func NewWithChannel(cfg *broker.RabbitMQConfig, logger *slog.Logger, factory func() (AMQPChannel, error)) broker.Publisher {
	return &Publisher{
		cfg:         cfg,
		logger:      logger,
		chanFactory: factory,
	}
}

// Connect dials the broker, opens a channel, declares the exchange
// (passive — assumes it exists), and enables publisher confirms if
// configured. Idempotent.
func (p *Publisher) Connect(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ch != nil {
		return nil
	}

	if p.chanFactory != nil {
		// Test path: use injected factory.
		ch, err := p.chanFactory()
		if err != nil {
			return err
		}
		p.ch = ch
		return nil
	}

	// Production path.
	tlsCfg, err := p.buildTLS()
	if err != nil {
		return err
	}

	var conn *amqp.Connection
	if tlsCfg != nil {
		conn, err = amqp.DialTLS(p.cfg.URL, tlsCfg)
	} else {
		conn, err = amqp.Dial(p.cfg.URL)
	}
	if err != nil {
		return mapErr(err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return mapErr(err)
	}

	exchangeType := p.cfg.ExchangeType
	if exchangeType == "" {
		exchangeType = "topic"
	}
	if err := ch.ExchangeDeclarePassive(
		p.cfg.Exchange, exchangeType, true, false, false, false, nil,
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("rabbitmq: exchange %q: %w: %v", p.cfg.Exchange, broker.ErrDestination, err)
	}

	if p.cfg.PublisherConfirms {
		if err := ch.Confirm(false); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return mapErr(err)
		}
	}

	p.conn = conn
	p.ch = ch
	p.logger.Info("rabbitmq connected",
		"url", redactURL(p.cfg.URL),
		"exchange", p.cfg.Exchange,
		"confirms", p.cfg.PublisherConfirms,
	)
	return nil
}

// Publish sends a single message. When PublisherConfirms is true it
// waits for the broker ack via DeferredConfirmation.WaitContext —
// this is what makes the AckLatency number meaningful. Without
// confirms the ack-latency reflects only the time to write the frame
// to the socket, which is a valid choice when the caller accepts
// at-most-once semantics.
func (p *Publisher) Publish(ctx context.Context, msg broker.Message) (broker.Receipt, error) {
	p.mu.Lock()
	ch := p.ch
	confirms := p.cfg.PublisherConfirms
	p.mu.Unlock()
	if ch == nil {
		return broker.Receipt{}, fmt.Errorf("rabbitmq publish: %w", broker.ErrNotConnected)
	}

	headers := amqp.Table{}
	for k, v := range msg.Headers {
		headers[k] = v
	}

	publishing := amqp.Publishing{
		ContentType:  "application/octet-stream",
		Body:         msg.Payload,
		Headers:      headers,
		DeliveryMode: amqp.Persistent,
	}

	routingKey := msg.Key
	if routingKey == "" {
		routingKey = msg.Destination
	}

	start := time.Now()

	if confirms {
		dc, err := ch.PublishWithDeferredConfirmWithContext(
			ctx,
			p.cfg.Exchange,
			routingKey,
			false, // mandatory
			false, // immediate
			publishing,
		)
		if err != nil {
			return broker.Receipt{}, mapErr(err)
		}
		// Block until the broker sends the ack. ctx carries the caller's
		// deadline so we don't wait forever.
		acked, err := dc.WaitContext(ctx)
		if err != nil {
			return broker.Receipt{}, fmt.Errorf("rabbitmq: confirm wait: %w: %v", broker.ErrTimeout, err)
		}
		if !acked {
			return broker.Receipt{}, fmt.Errorf("rabbitmq: broker nacked message: %w", broker.ErrTimeout)
		}
		lat := time.Since(start)
		return broker.Receipt{
			PublishedAt: start,
			AckLatency:  lat,
		}, nil
	}

	// No confirms: fire-and-forget with socket-write as the ack signal.
	if err := ch.Publish(
		p.cfg.Exchange,
		routingKey,
		false,
		false,
		publishing,
	); err != nil {
		return broker.Receipt{}, mapErr(err)
	}
	lat := time.Since(start)
	return broker.Receipt{
		PublishedAt: start,
		AckLatency:  lat,
	}, nil
}

// Close shuts down the channel and connection. Safe to call multiple
// times.
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ch == nil {
		return nil
	}
	chErr := p.ch.Close()
	p.ch = nil
	if p.conn != nil {
		connErr := p.conn.Close()
		p.conn = nil
		if chErr != nil {
			return chErr
		}
		return connErr
	}
	return chErr
}

// Kind returns broker.KindRabbitMQ.
func (p *Publisher) Kind() broker.Kind { return broker.KindRabbitMQ }

// ---- helpers ----

func (p *Publisher) buildTLS() (*tls.Config, error) {
	if p.cfg.TLSCACert == "" && p.cfg.TLSClientCert == "" && !p.cfg.InsecureSkipVerify {
		return nil, nil
	}
	cfg := &tls.Config{InsecureSkipVerify: p.cfg.InsecureSkipVerify} //nolint:gosec
	if p.cfg.TLSCACert != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(p.cfg.TLSCACert)) {
			return nil, fmt.Errorf("rabbitmq: failed to parse CA certificate PEM")
		}
		cfg.RootCAs = pool
	}
	if p.cfg.TLSClientCert != "" && p.cfg.TLSClientKey != "" {
		cert, err := tls.X509KeyPair([]byte(p.cfg.TLSClientCert), []byte(p.cfg.TLSClientKey))
		if err != nil {
			return nil, fmt.Errorf("rabbitmq: client cert/key: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return cfg, nil
}

func redactURL(u string) string {
	// Replace password in amqp://user:pass@host with amqp://user:***@host
	if idx := strings.Index(u, "@"); idx != -1 {
		prefix := u[:idx]
		suffix := u[idx:]
		if ci := strings.LastIndex(prefix, ":"); ci != -1 {
			return prefix[:ci] + ":***" + suffix
		}
	}
	return u
}

// mapErr converts amqp091-go errors to broker sentinel errors.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	var amqpErr *amqp.Error
	if strings.Contains(err.Error(), "ACCESS_REFUSED") {
		return fmt.Errorf("rabbitmq: %w: %v", broker.ErrPermission, err)
	}
	if strings.Contains(err.Error(), "NOT_FOUND") {
		return fmt.Errorf("rabbitmq: %w: %v", broker.ErrDestination, err)
	}
	if strings.Contains(err.Error(), "FRAME_ERROR") || strings.Contains(err.Error(), "connection/channel is not open") {
		return fmt.Errorf("rabbitmq: %w: %v", broker.ErrNotConnected, err)
	}
	// amqp.Error carries a code field we can pattern-match.
	_ = amqpErr // used for type awareness; actual check is string above
	return fmt.Errorf("rabbitmq: %w", err)
}
