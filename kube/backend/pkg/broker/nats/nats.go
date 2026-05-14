// Package nats implements the broker.Publisher interface for NATS Core
// and JetStream. It registers itself as broker.KindNATS via init().
package nats

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	natsclient "github.com/nats-io/nats.go"

	broker "github.com/argues/argus/pkg/broker"
)

var _ broker.Publisher = (*Publisher)(nil)

// Publisher wraps a NATS connection (core or JetStream).
type Publisher struct {
	cfg    *broker.NATSConfig
	logger *slog.Logger

	mu sync.Mutex
	nc *natsclient.Conn
	js natsclient.JetStreamContext // non-nil when UseJetStream==true
}

func init() {
	broker.Register(broker.KindNATS, func(_ context.Context, cfg any, logger *slog.Logger) (broker.Publisher, error) {
		c, ok := cfg.(*broker.NATSConfig)
		if !ok {
			return nil, fmt.Errorf("nats: factory received wrong config type %T", cfg)
		}
		return &Publisher{cfg: c, logger: logger}, nil
	})
}

// Connect dials the NATS server(s) and, if UseJetStream is true,
// obtains a JetStream context. Idempotent.
func (p *Publisher) Connect(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.nc != nil {
		return nil
	}

	opts, err := p.buildOpts(ctx)
	if err != nil {
		return err
	}

	servers := p.cfg.Servers
	nc, err := natsclient.Connect(servers, opts...)
	if err != nil {
		return mapErr(err)
	}

	if p.cfg.UseJetStream {
		js, err := nc.JetStream()
		if err != nil {
			_ = nc.Drain()
			return fmt.Errorf("nats: jetstream: %w", err)
		}
		p.js = js
	}

	p.nc = nc
	p.logger.Info("nats connected", "servers", servers, "jetstream", p.cfg.UseJetStream)
	return nil
}

// Publish sends a single message and blocks until an ack is received.
//
// JetStream mode: calls js.Publish which returns a PubAck — ack
// latency is measured around that call.
//
// Core mode: calls nc.Publish then nc.FlushWithDeadline which
// forces a round-trip through the server, giving us a meaningful
// latency number (without Flush the frame might sit in the socket
// buffer for an unbounded time).
func (p *Publisher) Publish(ctx context.Context, msg broker.Message) (broker.Receipt, error) {
	p.mu.Lock()
	nc := p.nc
	js := p.js
	p.mu.Unlock()
	if nc == nil {
		return broker.Receipt{}, fmt.Errorf("nats publish: %w", broker.ErrNotConnected)
	}

	headers := natsclient.Header{}
	for k, v := range msg.Headers {
		headers.Set(k, v)
	}
	if msg.Key != "" {
		headers.Set("Nats-Msg-Key", msg.Key)
	}

	natsMsg := &natsclient.Msg{
		Subject: msg.Destination,
		Header:  headers,
		Data:    msg.Payload,
	}

	start := time.Now()

	if js != nil {
		// JetStream path: js.PublishMsg provides a PubAck.
		opts := []natsclient.PubOpt{}
		if d, ok := ctx.Deadline(); ok {
			opts = append(opts, natsclient.AckWait(time.Until(d)))
		}
		ack, err := js.PublishMsg(natsMsg, opts...)
		if err != nil {
			return broker.Receipt{}, mapErr(err)
		}
		lat := time.Since(start)
		return broker.Receipt{
			PublishedAt: start,
			AckLatency:  lat,
			MessageID:   fmt.Sprintf("%s.%d", ack.Stream, ack.Sequence),
		}, nil
	}

	// Core NATS path: Publish + FlushWithDeadline.
	if err := nc.PublishMsg(natsMsg); err != nil {
		return broker.Receipt{}, mapErr(err)
	}
	deadline := 5 * time.Second
	if d, ok := ctx.Deadline(); ok {
		if rem := time.Until(d); rem < deadline {
			deadline = rem
		}
	}
	if err := nc.FlushTimeout(deadline); err != nil {
		return broker.Receipt{}, mapErr(err)
	}
	lat := time.Since(start)
	return broker.Receipt{
		PublishedAt: start,
		AckLatency:  lat,
	}, nil
}

// Close drains the connection and shuts it down. Safe to call multiple
// times.
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.nc == nil {
		return nil
	}
	// Drain flushes in-flight publishes, then closes.
	err := p.nc.Drain()
	p.nc = nil
	p.js = nil
	return err
}

// Kind returns broker.KindNATS.
func (p *Publisher) Kind() broker.Kind { return broker.KindNATS }

// ---- helpers ----

func (p *Publisher) buildOpts(ctx context.Context) ([]natsclient.Option, error) {
	var opts []natsclient.Option

	opts = append(opts, natsclient.Name("argus-loadtest"))

	// Apply a connection timeout derived from the context if it has a deadline.
	if d, ok := ctx.Deadline(); ok {
		opts = append(opts, natsclient.Timeout(time.Until(d)))
	} else {
		opts = append(opts, natsclient.Timeout(10*time.Second))
	}

	if p.cfg.InsecureSkipVerify {
		opts = append(opts, natsclient.Secure(&tls.Config{InsecureSkipVerify: true})) //nolint:gosec
	}

	switch p.cfg.AuthMode {
	case "none", "":
		// no auth
	case "user_pass":
		opts = append(opts, natsclient.UserInfo(p.cfg.Username, p.cfg.Password))
	case "token":
		opts = append(opts, natsclient.Token(p.cfg.Token))
	case "nkey":
		if p.cfg.NKeySeed == "" {
			return nil, fmt.Errorf("nats: authMode=nkey requires nkeySeed: %w", broker.ErrAuth)
		}
		nkeyOpt, err := natsclient.NkeyOptionFromSeed(p.cfg.NKeySeed)
		if err != nil {
			return nil, fmt.Errorf("nats: nkey: %w: %v", broker.ErrAuth, err)
		}
		opts = append(opts, nkeyOpt)
	case "creds_file":
		if p.cfg.CredsFile == "" {
			return nil, fmt.Errorf("nats: authMode=creds_file requires credsFile: %w", broker.ErrAuth)
		}
		opts = append(opts, natsclient.UserCredentials(p.cfg.CredsFile))
	default:
		return nil, fmt.Errorf("nats: unknown authMode %q: %w", p.cfg.AuthMode, broker.ErrAuth)
	}

	return opts, nil
}

// mapErr converts NATS SDK errors to broker sentinel errors.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case err == natsclient.ErrAuthorization || strings.Contains(msg, "Authorization"):
		return fmt.Errorf("nats: %w: %v", broker.ErrAuth, err)
	case err == natsclient.ErrTimeout || err == natsclient.ErrSlowConsumer:
		return fmt.Errorf("nats: %w: %v", broker.ErrTimeout, err)
	case strings.Contains(msg, "maximum payload"):
		return fmt.Errorf("nats: %w: %v", broker.ErrPayloadTooLarge, err)
	case strings.Contains(msg, "permission"):
		return fmt.Errorf("nats: %w: %v", broker.ErrPermission, err)
	case strings.Contains(msg, "nats: no servers") || err == natsclient.ErrNoServers:
		return fmt.Errorf("nats: %w: %v", broker.ErrNotConnected, err)
	default:
		return fmt.Errorf("nats: %w", err)
	}
}
