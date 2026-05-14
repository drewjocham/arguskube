package broker

import (
	"encoding/json"
	"fmt"
)

// Config is the polymorphic top-level config the frontend builds and
// posts to StartLoadTest. The Kind discriminator picks which embedded
// struct is meaningful. We deliberately don't use Go's interface-based
// polymorphism here because the value crosses the Wails JSON boundary
// where interfaces lose their type information.
//
// On the Go side, fetch the typed config with Config.Resolve() — it
// returns the embedded struct that matches Kind and errors if the
// caller picked a Kind whose config block is missing.
type Config struct {
	Kind     Kind            `json:"kind"`
	PubSub   *PubSubConfig   `json:"pubsub,omitempty"`
	NATS     *NATSConfig     `json:"nats,omitempty"`
	Kafka    *KafkaConfig    `json:"kafka,omitempty"`
	RabbitMQ *RabbitMQConfig `json:"rabbitmq,omitempty"`
	AMQP1    *AMQP1Config    `json:"amqp1,omitempty"`
}

// Resolve returns the per-kind config block. Returns an error if the
// caller picked a Kind but didn't populate the matching block — a
// common shape of frontend bug where a dropdown selection was changed
// but the form fields under it weren't.
func (c Config) Resolve() (any, error) {
	switch c.Kind {
	case KindPubSub:
		if c.PubSub == nil {
			return nil, fmt.Errorf("broker config: kind=%q but pubsub block missing", c.Kind)
		}
		return c.PubSub, nil
	case KindNATS:
		if c.NATS == nil {
			return nil, fmt.Errorf("broker config: kind=%q but nats block missing", c.Kind)
		}
		return c.NATS, nil
	case KindKafka:
		if c.Kafka == nil {
			return nil, fmt.Errorf("broker config: kind=%q but kafka block missing", c.Kind)
		}
		return c.Kafka, nil
	case KindRabbitMQ:
		if c.RabbitMQ == nil {
			return nil, fmt.Errorf("broker config: kind=%q but rabbitmq block missing", c.Kind)
		}
		return c.RabbitMQ, nil
	case KindAMQP1:
		if c.AMQP1 == nil {
			return nil, fmt.Errorf("broker config: kind=%q but amqp1 block missing", c.Kind)
		}
		return c.AMQP1, nil
	default:
		return nil, fmt.Errorf("broker config: unknown kind %q", c.Kind)
	}
}

// MarshalJSON enforces that exactly one config block is populated. The
// previous shape (Resolve at use time) caught it eventually but the
// error message was harder to trace back to the source field.
func (c Config) MarshalJSON() ([]byte, error) {
	type alias Config
	return json.Marshal(alias(c))
}

// PubSubConfig — Google Cloud Pub/Sub.
//
// AuthMode picks how we authenticate. The two paths the desktop app
// realistically uses are "adc" (Application Default Credentials —
// gcloud login on the operator's machine) and "service_account_json"
// (a pasted JSON key). Workload-identity is added so the same code
// works inside GKE pods where credentials are injected automatically.
type PubSubConfig struct {
	ProjectID string `json:"projectId"`
	// AuthMode: "adc" | "service_account_json" | "workload_identity"
	AuthMode string `json:"authMode"`
	// ServiceAccountJSON is the raw JSON of a service-account key
	// when AuthMode=="service_account_json". Sensitive — masked at
	// the frontend after submission.
	ServiceAccountJSON string `json:"serviceAccountJson,omitempty"`
	// Endpoint overrides the default googleapis.com URL — for the
	// Pub/Sub emulator (PUBSUB_EMULATOR_HOST) or private VPC
	// endpoints. Optional.
	Endpoint string `json:"endpoint,omitempty"`
}

// NATSConfig — NATS Core and JetStream both. JetStream is selected by
// UseJetStream=true; otherwise we publish on the core wire.
//
// Servers is the comma-list of URLs ("nats://1.2.3.4:4222,
// nats://5.6.7.8:4222"). TLS is on automatically when the URL scheme
// is `tls://` or `wss://`.
type NATSConfig struct {
	Servers      string `json:"servers"`
	UseJetStream bool   `json:"useJetStream"`
	// AuthMode: "none" | "user_pass" | "token" | "nkey" | "creds_file"
	AuthMode string `json:"authMode"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
	// NKeySeed is the seed string (starts with "SU..."). Stored as a
	// string for transport; converted to a key pair inside the
	// adapter using nats.go's nkeys helper.
	NKeySeed string `json:"nkeySeed,omitempty"`
	// CredsFile is a path to a NATS .creds file on the operator's
	// machine. The desktop app reads it; SaaS rejects this AuthMode
	// because the file isn't on the server.
	CredsFile string `json:"credsFile,omitempty"`
	// TLS settings — only consulted when the connection URL implies
	// TLS or InsecureSkipVerify is set.
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// KafkaConfig — Apache Kafka. Bootstrap is a comma-list of broker
// host:port pairs. SASL modes cover the realistic deployment surface
// (PLAIN for dev, SCRAM for prod, OAUTHBEARER for cloud-managed
// services like Confluent Cloud and MSK).
type KafkaConfig struct {
	BootstrapServers string `json:"bootstrapServers"`
	ClientID         string `json:"clientId,omitempty"`
	// AuthMode: "none" | "plain" | "scram_sha256" | "scram_sha512" | "oauthbearer" | "mtls"
	AuthMode string `json:"authMode"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	// OAuthBearerToken is the pre-acquired token when AuthMode=="oauthbearer".
	// Full OAuth dance is out of scope; the operator pastes a token
	// minted elsewhere. The adapter refuses to publish once the
	// token's `exp` has passed.
	OAuthBearerToken string `json:"oauthBearerToken,omitempty"`
	// mTLS material — PEM blocks pasted by the operator. Adapters
	// build the *tls.Config in-memory; no disk writes.
	TLSCACert     string `json:"tlsCaCert,omitempty"`
	TLSClientCert string `json:"tlsClientCert,omitempty"`
	TLSClientKey  string `json:"tlsClientKey,omitempty"`
	// Acks: "all" (default) | "leader" | "none". The load tester
	// surfaces ack latency, so "all" makes the latency number
	// meaningful end-to-end. "none" mode disables AckLatency
	// measurement — the receipt comes back at send-side queue,
	// not broker-confirmed.
	Acks               string `json:"acks,omitempty"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty"`
}

// RabbitMQConfig — RabbitMQ over AMQP 0.9.
//
// PublisherConfirms toggles confirmation mode — without it,
// ack-latency measurement is meaningless (the wire returns the moment
// the frame is on the socket, before the broker has accepted it). The
// load tester sets this to true by default and offers it as a toggle
// only because some pathological setups (no broker reachable, just
// want to fill a queue) need to disable it.
type RabbitMQConfig struct {
	URL                string `json:"url"` // amqp://user:pass@host:5672/vhost
	Exchange           string `json:"exchange"`
	ExchangeType       string `json:"exchangeType,omitempty"` // direct|topic|fanout|headers — defaults to topic
	PublisherConfirms  bool   `json:"publisherConfirms"`
	TLSCACert          string `json:"tlsCaCert,omitempty"`
	TLSClientCert      string `json:"tlsClientCert,omitempty"`
	TLSClientKey       string `json:"tlsClientKey,omitempty"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty"`
}

// AMQP1Config — Solace, Azure Service Bus, Artemis, anything AMQP 1.0.
// We use Azure's `go-amqp` because it's the only mature pure-Go
// AMQP-1.0 lib. SASL modes cover the realistic surface.
type AMQP1Config struct {
	URL string `json:"url"` // amqps://host:5671 or amqp://host:5672
	// AuthMode: "none" | "plain" | "external" | "bearer"
	AuthMode    string `json:"authMode"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	BearerToken string `json:"bearerToken,omitempty"`
	// SenderTarget is the AMQP "target address" — broker-specific:
	//   Solace:   "topic/orders.created"  or "queue/in.work"
	//   Azure SB: "myqueue" or "mytopic"
	//   Artemis:  the destination address
	SenderTarget       string `json:"senderTarget"`
	TLSCACert          string `json:"tlsCaCert,omitempty"`
	TLSClientCert      string `json:"tlsClientCert,omitempty"`
	TLSClientKey       string `json:"tlsClientKey,omitempty"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty"`
}
