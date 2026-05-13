package amqp1_test

import (
	"context"
	"errors"
	"testing"
	"time"

	goamqp "github.com/Azure/go-amqp"

	broker "github.com/argues/argus/pkg/broker"
	amqp1pkg "github.com/argues/argus/pkg/broker/amqp1"
	_ "github.com/argues/argus/pkg/broker/amqp1" // registers factory

	"log/slog"
)

// fakeSender implements amqp1pkg.AMQPSender. It records sent messages
// and returns a synthetic ack latency.
type fakeSender struct {
	sendErr  error
	latency  time.Duration
	messages []*goamqp.Message
}

func (f *fakeSender) SendAndAck(_ context.Context, msg *goamqp.Message) (time.Duration, error) {
	if f.sendErr != nil {
		return 0, f.sendErr
	}
	f.messages = append(f.messages, msg)
	lat := f.latency
	if lat == 0 {
		lat = 500 * time.Microsecond
	}
	return lat, nil
}

func (f *fakeSender) Close(_ context.Context) error { return nil }

func newTestPublisher(t *testing.T, cfg *broker.AMQP1Config, sender *fakeSender) broker.Publisher {
	t.Helper()
	pub := amqp1pkg.NewWithSender(cfg, slog.Default(), func(_ context.Context) (amqp1pkg.AMQPSender, error) {
		return sender, nil
	})
	t.Cleanup(func() { _ = pub.Close() })
	return pub
}

func baseCfg() *broker.AMQP1Config {
	return &broker.AMQP1Config{
		URL:          "amqp://localhost:5672",
		AuthMode:     "none",
		SenderTarget: "argus-queue",
	}
}

func TestAMQP1_ConnectAndPublish(t *testing.T) {
	sender := &fakeSender{}
	pub := newTestPublisher(t, baseCfg(), sender)
	ctx := context.Background()

	if err := pub.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	r, err := pub.Publish(ctx, broker.Message{
		Destination: "argus-queue",
		Key:         "k1",
		Headers:     map[string]string{"x-trace": "t1"},
		Payload:     []byte(`{"event":"fired"}`),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if r.AckLatency <= 0 {
		t.Errorf("AckLatency = %v, want > 0", r.AckLatency)
	}
	if r.PublishedAt.IsZero() {
		t.Error("PublishedAt is zero")
	}
	if len(sender.messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(sender.messages))
	}
}

func TestAMQP1_ConnectIdempotent(t *testing.T) {
	sender := &fakeSender{}
	pub := newTestPublisher(t, baseCfg(), sender)
	ctx := context.Background()

	if err := pub.Connect(ctx); err != nil {
		t.Fatalf("first Connect: %v", err)
	}
	if err := pub.Connect(ctx); err != nil {
		t.Fatalf("second Connect should be idempotent: %v", err)
	}
}

func TestAMQP1_PublishNotConnected(t *testing.T) {
	pub, err := broker.New(context.Background(), broker.Config{
		Kind: broker.KindAMQP1,
		AMQP1: &broker.AMQP1Config{
			URL:          "amqp://localhost",
			AuthMode:     "none",
			SenderTarget: "q",
		},
	}, slog.Default())
	if err != nil {
		t.Fatalf("broker.New: %v", err)
	}

	_, err = pub.Publish(context.Background(), broker.Message{
		Destination: "q",
		Payload:     []byte("hi"),
	})
	if !errors.Is(err, broker.ErrNotConnected) {
		t.Errorf("expected ErrNotConnected, got %v", err)
	}
}

func TestAMQP1_CloseSafe(t *testing.T) {
	sender := &fakeSender{}
	pub := newTestPublisher(t, baseCfg(), sender)
	ctx := context.Background()
	_ = pub.Connect(ctx)

	if err := pub.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := pub.Close(); err != nil {
		t.Errorf("second Close should be idempotent: %v", err)
	}
}

func TestAMQP1_Kind(t *testing.T) {
	sender := &fakeSender{}
	pub := newTestPublisher(t, baseCfg(), sender)
	if pub.Kind() != broker.KindAMQP1 {
		t.Errorf("Kind = %v, want %v", pub.Kind(), broker.KindAMQP1)
	}
}

func TestAMQP1_PublishError(t *testing.T) {
	sender := &fakeSender{sendErr: errors.New("link detached")}
	pub := newTestPublisher(t, baseCfg(), sender)
	ctx := context.Background()
	_ = pub.Connect(ctx)

	_, err := pub.Publish(ctx, broker.Message{
		Destination: "argus-queue",
		Payload:     []byte("x"),
	})
	if err == nil {
		t.Fatal("expected error from failed send")
	}
}

func TestAMQP1_AckLatencyMeasured(t *testing.T) {
	expected := 2 * time.Millisecond
	sender := &fakeSender{latency: expected}
	pub := newTestPublisher(t, baseCfg(), sender)
	ctx := context.Background()
	_ = pub.Connect(ctx)

	r, err := pub.Publish(ctx, broker.Message{
		Destination: "argus-queue",
		Payload:     []byte("probe"),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	// The adapter uses the latency returned by SendAndAck directly.
	if r.AckLatency != expected {
		t.Errorf("AckLatency = %v, want %v", r.AckLatency, expected)
	}
}

func TestAMQP1_MultiplePublishes(t *testing.T) {
	sender := &fakeSender{}
	pub := newTestPublisher(t, baseCfg(), sender)
	ctx := context.Background()
	_ = pub.Connect(ctx)

	const n = 5
	for i := 0; i < n; i++ {
		_, err := pub.Publish(ctx, broker.Message{
			Destination: "argus-queue",
			Payload:     []byte("msg"),
		})
		if err != nil {
			t.Fatalf("Publish[%d]: %v", i, err)
		}
	}
	if len(sender.messages) != n {
		t.Errorf("expected %d messages, got %d", n, len(sender.messages))
	}
}

func TestAMQP1_Headers(t *testing.T) {
	sender := &fakeSender{}
	pub := newTestPublisher(t, baseCfg(), sender)
	ctx := context.Background()
	_ = pub.Connect(ctx)

	_, err := pub.Publish(ctx, broker.Message{
		Destination: "argus-queue",
		Key:         "routing-key",
		Headers:     map[string]string{"source": "argus", "version": "v1"},
		Payload:     []byte("payload"),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sender.messages))
	}
	msg := sender.messages[0]
	if msg.ApplicationProperties["source"] != "argus" {
		t.Errorf("header source = %v, want argus", msg.ApplicationProperties["source"])
	}
	if msg.Properties == nil || msg.Properties.CorrelationID != "routing-key" {
		t.Error("expected CorrelationID to be set to Key")
	}
}
