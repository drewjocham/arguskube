package broker

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

// fakePublisher is a hand-rolled stand-in used by tests in this
// package to exercise the registry + Config.Resolve plumbing without
// needing any real broker library. Each Publish records the message
// and returns a synthetic Receipt. Tests can preload an error to
// simulate failure paths.
type fakePublisher struct {
	kind    Kind
	connect error
	pub     error
	got     []Message
	closed  bool
}

func (f *fakePublisher) Connect(_ context.Context) error { return f.connect }
func (f *fakePublisher) Publish(_ context.Context, m Message) (Receipt, error) {
	if f.pub != nil {
		return Receipt{}, f.pub
	}
	f.got = append(f.got, m)
	return Receipt{
		PublishedAt: time.Unix(0, 0),
		AckLatency:  10 * time.Microsecond,
		MessageID:   "fake-id",
	}, nil
}
func (f *fakePublisher) Close() error { f.closed = true; return nil }
func (f *fakePublisher) Kind() Kind   { return f.kind }

func TestConfig_Resolve_HappyPaths(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		wantKind Kind
	}{
		{"pubsub", Config{Kind: KindPubSub, PubSub: &PubSubConfig{ProjectID: "p"}}, KindPubSub},
		{"nats", Config{Kind: KindNATS, NATS: &NATSConfig{Servers: "nats://x"}}, KindNATS},
		{"kafka", Config{Kind: KindKafka, Kafka: &KafkaConfig{BootstrapServers: "x:9092"}}, KindKafka},
		{"rabbitmq", Config{Kind: KindRabbitMQ, RabbitMQ: &RabbitMQConfig{URL: "amqp://x"}}, KindRabbitMQ},
		{"amqp1", Config{Kind: KindAMQP1, AMQP1: &AMQP1Config{URL: "amqp://x"}}, KindAMQP1},
	}
	for _, c := range cases {
		t.Run(string(c.name), func(t *testing.T) {
			got, err := c.cfg.Resolve()
			if err != nil {
				t.Fatalf("Resolve(%s): %v", c.name, err)
			}
			if got == nil {
				t.Fatalf("Resolve(%s) returned nil block", c.name)
			}
		})
	}
}

func TestConfig_Resolve_MissingBlock(t *testing.T) {
	cases := []Kind{KindPubSub, KindNATS, KindKafka, KindRabbitMQ, KindAMQP1}
	for _, k := range cases {
		t.Run(string(k), func(t *testing.T) {
			cfg := Config{Kind: k} // no block populated
			_, err := cfg.Resolve()
			if err == nil {
				t.Fatalf("expected error for kind=%s with no block", k)
			}
		})
	}
}

func TestConfig_Resolve_UnknownKind(t *testing.T) {
	cfg := Config{Kind: Kind("rabbithole")}
	_, err := cfg.Resolve()
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
}

func TestRegistry_New(t *testing.T) {
	defer reset()
	reset()
	Register(KindPubSub, func(_ context.Context, cfg any, _ *slog.Logger) (Publisher, error) {
		if _, ok := cfg.(*PubSubConfig); !ok {
			return nil, errors.New("factory: wrong config type")
		}
		return &fakePublisher{kind: KindPubSub}, nil
	})

	pub, err := New(context.Background(), Config{
		Kind:   KindPubSub,
		PubSub: &PubSubConfig{ProjectID: "p"},
	}, slog.Default())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if pub.Kind() != KindPubSub {
		t.Errorf("Kind = %v, want %v", pub.Kind(), KindPubSub)
	}
}

func TestRegistry_New_UnregisteredKind(t *testing.T) {
	defer reset()
	reset()
	_, err := New(context.Background(), Config{
		Kind:  KindNATS,
		NATS:  &NATSConfig{Servers: "nats://x"},
	}, slog.Default())
	if err == nil {
		t.Fatal("expected error for unregistered kind")
	}
}

func TestRegistry_DuplicateRegisterPanics(t *testing.T) {
	defer reset()
	reset()
	Register(KindKafka, func(context.Context, any, *slog.Logger) (Publisher, error) {
		return nil, nil
	})
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate Register")
		}
	}()
	Register(KindKafka, func(context.Context, any, *slog.Logger) (Publisher, error) {
		return nil, nil
	})
}

func TestRegistry_Registered(t *testing.T) {
	defer reset()
	reset()
	if len(Registered()) != 0 {
		t.Errorf("expected empty registry, got %v", Registered())
	}
	Register(KindPubSub, func(context.Context, any, *slog.Logger) (Publisher, error) {
		return nil, nil
	})
	Register(KindNATS, func(context.Context, any, *slog.Logger) (Publisher, error) {
		return nil, nil
	})
	got := Registered()
	if len(got) != 2 {
		t.Errorf("Registered len = %d, want 2 (got %v)", len(got), got)
	}
}

// TestFakePublisher_RoundTrip exercises the Publisher contract that
// adapters MUST satisfy: Connect succeeds, Publish records the msg
// and returns a non-zero AckLatency, Close marks the publisher closed.
// Adapters get their own integration tests; this test pins the
// contract so adapters can compile-check against it via the interface
// (all five adapters import broker and the *fakePublisher pattern
// would fail to compile if the interface drifted).
func TestFakePublisher_RoundTrip(t *testing.T) {
	p := &fakePublisher{kind: KindNATS}
	if err := p.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	r, err := p.Publish(context.Background(), Message{
		Destination: "x",
		Payload:     []byte("hello"),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if r.AckLatency == 0 {
		t.Error("expected non-zero AckLatency")
	}
	if len(p.got) != 1 {
		t.Errorf("got %d messages, want 1", len(p.got))
	}
	if err := p.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if !p.closed {
		t.Error("expected closed=true")
	}
}
