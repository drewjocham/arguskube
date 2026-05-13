// Package broker is the publish-side abstraction the Argus load-tester
// drives. Each supported broker (Google Pub/Sub, NATS JetStream, Apache
// Kafka, RabbitMQ AMQP-0.9, Solace / AMQP-1.0) ships an adapter that
// implements this interface. The load-test engine in pkg/loadtest only
// ever sees the interface, so the same engine drives every broker.
//
// Why this exists separately from the existing alert-ingress publisher:
//
//	alert-ingress already has a publisher abstraction
//	(internal/pubsub/publisher.go), but it's tightly coupled to the
//	ArgusAlert payload type and only ever calls PublishAlert(...). We
//	need (a) raw-bytes publish, (b) ack-latency measurement per call,
//	and (c) per-adapter connection lifecycle so the load tester can
//	tear down cleanly between runs. Rather than retrofit those onto a
//	package whose only consumer is the alert pipeline, we add a fresh
//	package for the broader use case. The two will probably converge
//	once ADR-001 lands its `pkg/broker` interface; this is the
//	publish-side half of that.
//
// Adapters MUST NOT log to a global logger — each adapter takes a
// *slog.Logger so a load run shows up in its own request scope.
package broker

import (
	"context"
	"errors"
	"time"
)

// Kind names a broker family. Kept as a small enum (string for JSON
// transport friendliness) rather than an int so config files and Wails
// JSON args don't have to know magic numbers.
type Kind string

const (
	KindPubSub   Kind = "pubsub"   // Google Cloud Pub/Sub
	KindNATS     Kind = "nats"     // NATS / JetStream
	KindKafka    Kind = "kafka"    // Apache Kafka (Confluent / MSK / Strimzi)
	KindRabbitMQ Kind = "rabbitmq" // RabbitMQ AMQP-0.9
	KindAMQP1    Kind = "amqp1"    // Solace, Azure Service Bus, anything AMQP-1.0
)

// Knowns is the closed set of broker kinds. Frontend dropdowns iterate
// this so adding a kind here automatically exposes it in the UI list.
var Knowns = []Kind{KindPubSub, KindNATS, KindKafka, KindRabbitMQ, KindAMQP1}

// Publisher is the publish-side contract every adapter implements.
//
// Lifecycle: Connect → many Publish → Close. Re-Connect after Close is
// not supported — callers should construct a fresh Publisher per run so
// each load test starts with cold connection pools.
type Publisher interface {
	// Connect establishes whatever the underlying broker needs (TCP
	// + TLS + SASL handshake + topic discovery). Idempotent: calling
	// Connect twice returns the first call's outcome without re-doing
	// the handshake. Returns ErrAlreadyConnected only if the caller
	// passes an explicit "must be fresh" flag — see (re)connect docs
	// in each adapter; the default is forgiving so retries don't
	// pile up.
	Connect(ctx context.Context) error

	// Publish sends a single Message. The returned Receipt carries
	// the wire-observed ack latency (time from "request handed to
	// adapter" to "broker acknowledged"). For brokers that ack
	// asynchronously (Pub/Sub, Kafka with acks=1, NATS Core), the
	// adapter MUST wait for the ack before returning so the latency
	// number is meaningful — the load-test engine relies on this for
	// its P50/P95/P99 numbers.
	//
	// An error here is per-message: the caller decides whether to
	// retry or surface to the run report. The Publisher remains
	// usable for further Publish calls unless ctx is canceled.
	Publish(ctx context.Context, msg Message) (Receipt, error)

	// Close releases all resources. Safe to call multiple times.
	// In-flight Publish calls observe ctx.Done() in their own ctx
	// argument; Close does NOT cancel those — it just stops future
	// ones. Tests want this separation so they can assert the in-
	// flight goroutine returned a real error vs. shutdown.
	Close() error

	// Kind returns the broker family. Useful for adapter-agnostic
	// reporters that want to label charts with the broker name.
	Kind() Kind
}

// Message is the publish-side payload. Headers map onto each broker's
// native concept (Kafka headers, AMQP application-properties, NATS
// headers, Pub/Sub attributes). Key is the partition or routing key —
// the adapter chooses how to apply it; some brokers (RabbitMQ direct
// exchange) treat Key as the routing key, others (Kafka) as the
// partition selector.
//
// Destination is the topic / subject / queue / exchange name. Each
// adapter has slightly different semantics for what this means; the
// per-adapter config lets the operator predeclare the destination so
// Destination here is the leaf name only ("orders.created" rather than
// "amqp://host/exchanges/orders/created").
type Message struct {
	Destination string            `json:"destination"`
	Key         string            `json:"key,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Payload     []byte            `json:"payload"`
}

// Receipt is what Publish returns on success. AckLatency is the wall
// clock between "Publish called" and "broker acknowledged" — adapters
// MUST measure this themselves rather than letting the caller wrap the
// call, so the measurement excludes call-site overhead (channel sends,
// goroutine scheduling) that isn't really broker latency.
type Receipt struct {
	PublishedAt time.Time     `json:"publishedAt"`
	AckLatency  time.Duration `json:"ackLatencyNs"`
	// MessageID is the broker-assigned identifier when one is
	// available (Pub/Sub message ID, Kafka offset, JetStream sequence).
	// Empty for brokers that don't return one (RabbitMQ AMQP-0.9 in
	// non-confirm mode, AMQP-1 fire-and-forget).
	MessageID string `json:"messageId,omitempty"`
}

// Errors callers should be able to distinguish at the call site. Most
// broker SDKs return rich error trees; adapters MUST map their library-
// specific errors onto one of these (or wrap with %w) so the load-test
// engine can categorize without importing every SDK's errors package.
var (
	ErrNotConnected     = errors.New("broker: publisher not connected")
	ErrAlreadyConnected = errors.New("broker: publisher already connected")
	ErrAuth             = errors.New("broker: authentication failed")
	ErrPermission       = errors.New("broker: insufficient permissions")
	ErrDestination      = errors.New("broker: destination does not exist or is not writable")
	ErrTimeout          = errors.New("broker: publish ack timed out")
	ErrPayloadTooLarge  = errors.New("broker: payload exceeds broker max size")
)
