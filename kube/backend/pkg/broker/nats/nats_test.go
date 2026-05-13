package nats_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	natsclient "github.com/nats-io/nats.go"
	natsserver "github.com/nats-io/nats-server/v2/server"
	natstest "github.com/nats-io/nats-server/v2/test"

	broker "github.com/argues/argus/pkg/broker"
	_ "github.com/argues/argus/pkg/broker/nats" // registers factory

	"log/slog"
)

// startCoreServer starts an in-process NATS server on a random port.
func startCoreServer(t *testing.T) *natsserver.Server {
	t.Helper()
	opts := &natsserver.Options{
		Host:                  "127.0.0.1",
		Port:                  -1, // auto-assign
		NoLog:                 true,
		NoSigs:                true,
		MaxControlLine:        4096,
		DisableShortFirstPing: true,
	}
	srv := natstest.RunServer(opts)
	t.Cleanup(func() { srv.Shutdown() })
	return srv
}

// startJetStreamServer starts an in-process NATS server with JetStream.
func startJetStreamServer(t *testing.T) *natsserver.Server {
	t.Helper()
	opts := &natsserver.Options{
		Host:                  "127.0.0.1",
		Port:                  -1,
		NoLog:                 true,
		NoSigs:                true,
		MaxControlLine:        4096,
		DisableShortFirstPing: true,
		JetStream:             true,
		StoreDir:              t.TempDir(),
	}
	srv := natstest.RunServer(opts)
	t.Cleanup(func() { srv.Shutdown() })
	return srv
}

func serverURL(srv *natsserver.Server) string {
	return fmt.Sprintf("nats://%s", srv.Addr())
}

// provisionJetStreamStream connects a raw nats client to url and
// creates a stream named "ARGUS" with subject "ARGUS.*". Used in tests
// to pre-provision the stream before our Publisher publishes.
func provisionJetStreamStream(t *testing.T, url string) {
	t.Helper()
	nc, err := natsclient.Connect(url)
	if err != nil {
		t.Fatalf("admin nats connect: %v", err)
	}
	t.Cleanup(func() { nc.Close() })

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("admin jetstream: %v", err)
	}
	_, err = js.AddStream(&natsclient.StreamConfig{
		Name:     "ARGUS",
		Subjects: []string{"ARGUS.*"},
	})
	if err != nil {
		t.Fatalf("add stream: %v", err)
	}
}

func newCorePublisher(t *testing.T) (broker.Publisher, *natsserver.Server) {
	t.Helper()
	srv := startCoreServer(t)
	pub, err := broker.New(context.Background(), broker.Config{
		Kind: broker.KindNATS,
		NATS: &broker.NATSConfig{
			Servers:      serverURL(srv),
			UseJetStream: false,
			AuthMode:     "none",
		},
	}, slog.Default())
	if err != nil {
		t.Fatalf("broker.New: %v", err)
	}
	t.Cleanup(func() { _ = pub.Close() })
	return pub, srv
}

func TestNATSCore_ConnectAndPublish(t *testing.T) {
	pub, _ := newCorePublisher(t)
	ctx := context.Background()

	if err := pub.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	r, err := pub.Publish(ctx, broker.Message{
		Destination: "argus.test",
		Key:         "k1",
		Headers:     map[string]string{"x-trace": "123"},
		Payload:     []byte("hello"),
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
}

func TestNATSCore_ConnectIdempotent(t *testing.T) {
	pub, _ := newCorePublisher(t)
	ctx := context.Background()

	if err := pub.Connect(ctx); err != nil {
		t.Fatalf("first Connect: %v", err)
	}
	if err := pub.Connect(ctx); err != nil {
		t.Fatalf("second Connect should be idempotent: %v", err)
	}
}

func TestNATSCore_PublishNotConnected(t *testing.T) {
	srv := startCoreServer(t)
	pub, err := broker.New(context.Background(), broker.Config{
		Kind: broker.KindNATS,
		NATS: &broker.NATSConfig{
			Servers:  serverURL(srv),
			AuthMode: "none",
		},
	}, slog.Default())
	if err != nil {
		t.Fatalf("broker.New: %v", err)
	}

	_, err = pub.Publish(context.Background(), broker.Message{
		Destination: "x",
		Payload:     []byte("hi"),
	})
	if !errors.Is(err, broker.ErrNotConnected) {
		t.Errorf("expected ErrNotConnected, got %v", err)
	}
}

func TestNATSCore_CloseSafe(t *testing.T) {
	pub, _ := newCorePublisher(t)
	ctx := context.Background()
	_ = pub.Connect(ctx)

	if err := pub.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := pub.Close(); err != nil {
		t.Errorf("second Close should be idempotent: %v", err)
	}
}

func TestNATSCore_Kind(t *testing.T) {
	pub, _ := newCorePublisher(t)
	if pub.Kind() != broker.KindNATS {
		t.Errorf("Kind = %v, want %v", pub.Kind(), broker.KindNATS)
	}
}

func TestNATSJetStream_ConnectAndPublish(t *testing.T) {
	srv := startJetStreamServer(t)
	url := serverURL(srv)

	// Pre-provision the stream so our Publisher can publish into it.
	provisionJetStreamStream(t, url)

	pub, err := broker.New(context.Background(), broker.Config{
		Kind: broker.KindNATS,
		NATS: &broker.NATSConfig{
			Servers:      url,
			UseJetStream: true,
			AuthMode:     "none",
		},
	}, slog.Default())
	if err != nil {
		t.Fatalf("broker.New: %v", err)
	}
	defer pub.Close()

	ctx := context.Background()
	if err := pub.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	r, err := pub.Publish(ctx, broker.Message{
		Destination: "ARGUS.events",
		Payload:     []byte("js-payload"),
	})
	if err != nil {
		t.Fatalf("Publish JetStream: %v", err)
	}
	if r.MessageID == "" {
		t.Error("expected non-empty MessageID (stream.sequence) for JetStream")
	}
	if r.AckLatency <= 0 {
		t.Errorf("AckLatency = %v, want > 0", r.AckLatency)
	}
}

func TestNATSJetStream_AckLatencyMeasured(t *testing.T) {
	srv := startJetStreamServer(t)
	url := serverURL(srv)
	provisionJetStreamStream(t, url)

	pub, err := broker.New(context.Background(), broker.Config{
		Kind: broker.KindNATS,
		NATS: &broker.NATSConfig{
			Servers:      url,
			UseJetStream: true,
			AuthMode:     "none",
		},
	}, slog.Default())
	if err != nil {
		t.Fatalf("broker.New: %v", err)
	}
	defer pub.Close()

	ctx := context.Background()
	if err := pub.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	r, err := pub.Publish(ctx, broker.Message{
		Destination: "ARGUS.events",
		Payload:     []byte("probe"),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if r.AckLatency <= 0 || r.AckLatency > 5*time.Second {
		t.Errorf("AckLatency = %v out of expected range", r.AckLatency)
	}
}
