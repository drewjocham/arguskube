package pubsub_test

import (
	"context"
	"errors"
	"testing"
	"time"

	gcppubsub "cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	broker "github.com/argues/argus/pkg/broker"
	_ "github.com/argues/argus/pkg/broker/pubsub" // side-effect: registers factory
	psadapter "github.com/argues/argus/pkg/broker/pubsub"

	"log/slog"
)

// newFakeEnv starts an in-process pstest.Server, pre-creates topics,
// and returns a grpc.ClientConn pointing at it. The conn and server are
// cleaned up via t.Cleanup.
func newFakeEnv(t *testing.T, projectID string, topics ...string) *grpc.ClientConn {
	t.Helper()
	srv := pstest.NewServer()
	t.Cleanup(func() { _ = srv.Close() })

	conn, err := grpc.Dial(srv.Addr, grpc.WithTransportCredentials(insecure.NewCredentials())) //nolint:staticcheck
	if err != nil {
		t.Fatalf("grpc dial fake: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx := context.Background()
	adminClient, err := gcppubsub.NewClient(ctx, projectID,
		option.WithGRPCConn(conn),
		option.WithoutAuthentication(),
	)
	if err != nil {
		t.Fatalf("admin pubsub client: %v", err)
	}
	t.Cleanup(func() { _ = adminClient.Close() })

	for _, topicID := range topics {
		if _, err := adminClient.CreateTopic(ctx, topicID); err != nil {
			t.Fatalf("create topic %q: %v", topicID, err)
		}
	}
	return conn
}

// newFakePublisher returns a Publisher wired to the fake gRPC conn.
func newFakePublisher(t *testing.T, conn *grpc.ClientConn, projectID string) broker.Publisher {
	t.Helper()
	pub := psadapter.NewWithOptions(
		&broker.PubSubConfig{
			ProjectID: projectID,
			AuthMode:  "adc",
		},
		slog.Default(),
		option.WithGRPCConn(conn),
		option.WithoutAuthentication(),
	)
	t.Cleanup(func() { _ = pub.Close() })
	return pub
}

func TestPubSubPublisher_ConnectAndPublish(t *testing.T) {
	const projectID = "test-project"
	conn := newFakeEnv(t, projectID, "test-topic")
	pub := newFakePublisher(t, conn, projectID)
	ctx := context.Background()

	if err := pub.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	r, err := pub.Publish(ctx, broker.Message{
		Destination: "test-topic",
		Key:         "k1",
		Headers:     map[string]string{"trace": "abc"},
		Payload:     []byte(`{"hello":"world"}`),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if r.MessageID == "" {
		t.Error("expected non-empty MessageID from Pub/Sub fake")
	}
	if r.AckLatency <= 0 {
		t.Errorf("AckLatency = %v, want > 0", r.AckLatency)
	}
	if r.PublishedAt.IsZero() {
		t.Error("PublishedAt is zero")
	}
}

func TestPubSubPublisher_ConnectIdempotent(t *testing.T) {
	conn := newFakeEnv(t, "proj", "tpc")
	pub := newFakePublisher(t, conn, "proj")
	ctx := context.Background()

	if err := pub.Connect(ctx); err != nil {
		t.Fatalf("first Connect: %v", err)
	}
	if err := pub.Connect(ctx); err != nil {
		t.Fatalf("second Connect should be idempotent: %v", err)
	}
}

func TestPubSubPublisher_PublishNotConnected(t *testing.T) {
	pub, err := broker.New(context.Background(), broker.Config{
		Kind: broker.KindPubSub,
		PubSub: &broker.PubSubConfig{
			ProjectID: "p",
			AuthMode:  "adc",
		},
	}, slog.Default())
	if err != nil {
		t.Fatalf("broker.New: %v", err)
	}
	_, err = pub.Publish(context.Background(), broker.Message{Destination: "x", Payload: []byte("hi")})
	if !errors.Is(err, broker.ErrNotConnected) {
		t.Errorf("expected ErrNotConnected, got %v", err)
	}
}

func TestPubSubPublisher_UnknownTopicReturnsErrDestination(t *testing.T) {
	conn := newFakeEnv(t, "proj", "known-topic")
	pub := newFakePublisher(t, conn, "proj")
	ctx := context.Background()

	if err := pub.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	_, err := pub.Publish(ctx, broker.Message{
		Destination: "does-not-exist",
		Payload:     []byte("x"),
	})
	if !errors.Is(err, broker.ErrDestination) {
		t.Errorf("expected ErrDestination, got %v", err)
	}
}

func TestPubSubPublisher_Kind(t *testing.T) {
	pub, _ := broker.New(context.Background(), broker.Config{
		Kind:   broker.KindPubSub,
		PubSub: &broker.PubSubConfig{ProjectID: "p", AuthMode: "adc"},
	}, slog.Default())
	if pub.Kind() != broker.KindPubSub {
		t.Errorf("Kind = %v, want %v", pub.Kind(), broker.KindPubSub)
	}
}

func TestPubSubPublisher_CloseSafe(t *testing.T) {
	conn := newFakeEnv(t, "proj", "tpc")
	pub := newFakePublisher(t, conn, "proj")
	ctx := context.Background()
	_ = pub.Connect(ctx)

	if err := pub.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := pub.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestPubSubPublisher_AckLatencyMeasured(t *testing.T) {
	conn := newFakeEnv(t, "proj", "latency-topic")
	pub := newFakePublisher(t, conn, "proj")
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
