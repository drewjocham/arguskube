package kafka_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kfake"

	broker "github.com/argues/argus/pkg/broker"
	_ "github.com/argues/argus/pkg/broker/kafka" // registers factory

	"log/slog"
)

// newFakeCluster starts an in-process kfake cluster with the given topics
// pre-seeded (1 partition each). Topics must be seeded because kfake does
// not auto-create topics by default.
func newFakeCluster(t *testing.T, numBrokers int, topics ...string) *kfake.Cluster {
	t.Helper()
	opts := []kfake.Opt{kfake.NumBrokers(numBrokers)}
	for _, topic := range topics {
		opts = append(opts, kfake.SeedTopics(1, topic))
	}
	cl, err := kfake.NewCluster(opts...)
	if err != nil {
		t.Fatalf("kfake.NewCluster: %v", err)
	}
	t.Cleanup(cl.Close)
	return cl
}

func newPublisher(t *testing.T, cl *kfake.Cluster, extraOpts ...func(*broker.KafkaConfig)) broker.Publisher {
	t.Helper()
	addrs := cl.ListenAddrs()
	cfg := &broker.KafkaConfig{
		BootstrapServers: strings.Join(addrs, ","),
		AuthMode:         "none",
		Acks:             "all",
	}
	for _, o := range extraOpts {
		o(cfg)
	}
	pub, err := broker.New(context.Background(), broker.Config{
		Kind:  broker.KindKafka,
		Kafka: cfg,
	}, slog.Default())
	if err != nil {
		t.Fatalf("broker.New: %v", err)
	}
	t.Cleanup(func() { _ = pub.Close() })
	return pub
}

func TestKafka_ConnectAndPublish(t *testing.T) {
	cl := newFakeCluster(t, 1, "argus-events")
	pub := newPublisher(t, cl)

	ctx := context.Background()
	if err := pub.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	r, err := pub.Publish(ctx, broker.Message{
		Destination: "argus-events",
		Key:         "key-1",
		Headers:     map[string]string{"x-trace": "abc"},
		Payload:     []byte(`{"event":"test"}`),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if r.MessageID == "" {
		t.Error("expected non-empty MessageID (topic/partition/offset)")
	}
	if r.AckLatency <= 0 {
		t.Errorf("AckLatency = %v, want > 0", r.AckLatency)
	}
	if r.PublishedAt.IsZero() {
		t.Error("PublishedAt is zero")
	}
}

func TestKafka_ConnectIdempotent(t *testing.T) {
	cl := newFakeCluster(t, 1)
	pub := newPublisher(t, cl) // no topic needed — just tests connect
	ctx := context.Background()

	if err := pub.Connect(ctx); err != nil {
		t.Fatalf("first Connect: %v", err)
	}
	if err := pub.Connect(ctx); err != nil {
		t.Fatalf("second Connect should be idempotent: %v", err)
	}
}

func TestKafka_PublishNotConnected(t *testing.T) {
	cl := newFakeCluster(t, 1)
	pub := newPublisher(t, cl) // not calling Connect

	_, err := pub.Publish(context.Background(), broker.Message{
		Destination: "x",
		Payload:     []byte("hi"),
	})
	if !errors.Is(err, broker.ErrNotConnected) {
		t.Errorf("expected ErrNotConnected, got %v", err)
	}
}

func TestKafka_CloseSafe(t *testing.T) {
	cl := newFakeCluster(t, 1)
	pub := newPublisher(t, cl) // connect then close twice
	ctx := context.Background()
	_ = pub.Connect(ctx)

	if err := pub.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := pub.Close(); err != nil {
		t.Errorf("second Close should be idempotent: %v", err)
	}
}

func TestKafka_Kind(t *testing.T) {
	cl := newFakeCluster(t, 1)
	pub := newPublisher(t, cl) // no connect needed
	if pub.Kind() != broker.KindKafka {
		t.Errorf("Kind = %v, want %v", pub.Kind(), broker.KindKafka)
	}
}

func TestKafka_AckLatencyMeasured(t *testing.T) {
	cl := newFakeCluster(t, 1, "latency-topic")
	pub := newPublisher(t, cl)
	ctx := context.Background()

	if err := pub.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	r, err := pub.Publish(ctx, broker.Message{
		Destination: "latency-topic",
		Payload:     []byte("probe"),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if r.AckLatency <= 0 || r.AckLatency > 5*time.Second {
		t.Errorf("AckLatency = %v out of expected range", r.AckLatency)
	}
}

func TestKafka_MultipleMessages(t *testing.T) {
	cl := newFakeCluster(t, 1, "argus-events")
	pub := newPublisher(t, cl)
	ctx := context.Background()

	if err := pub.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	for i := 0; i < 5; i++ {
		_, err := pub.Publish(ctx, broker.Message{
			Destination: "argus-events",
			Payload:     []byte("msg"),
		})
		if err != nil {
			t.Fatalf("Publish[%d]: %v", i, err)
		}
	}
}

func TestKafka_AcksLeader(t *testing.T) {
	cl := newFakeCluster(t, 1, "leader-topic")
	pub := newPublisher(t, cl, func(cfg *broker.KafkaConfig) {
		cfg.Acks = "leader"
	})
	ctx := context.Background()

	if err := pub.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	r, err := pub.Publish(ctx, broker.Message{
		Destination: "leader-topic",
		Payload:     []byte("leader-ack"),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if r.AckLatency <= 0 {
		t.Errorf("AckLatency = %v, want > 0", r.AckLatency)
	}
}
