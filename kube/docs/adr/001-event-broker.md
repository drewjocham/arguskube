# ADR-001: Event broker for Argus

**Status:** Proposed
**Date:** 2026-05-13
**Deciders:** @drewjocham (eng lead), product, SRE on-call

## Context

Argus is approaching prod-readiness. Five subsystems already act as
producer/consumer pairs but talk point-to-point:

- `alert-ingress` → AI agent ([kube/alert-ingress/cmd/main.go](../../../kube/alert-ingress/cmd/main.go))
- in-cluster agents → backend hub ([kube/backend/api/pkg/hub.go](../../../kube/backend/api/pkg/hub.go))
- environment probes → setup checklist + status marquee (new, see plan §4)
- user-activity observers → suggestion agent (new, see plan §6)
- background subsystems (popeye, vulnscan, setup, k8s watch) → status marquee (new, §7)

[../../../.context.md](../../../.context.md) flags this as `Hub bottleneck —
map[string]*AgentConnection + sync.RWMutex; won't scale past ~50 agents` and
`No tenant isolation model — no message broker`.

Constraints shaping the decision:

| Constraint | Source |
|---|---|
| Must work for a single-user desktop install with **no extra processes** | "easy for the user" goal |
| Must work behind a **corporate egress proxy** (HTTPS-only) | enterprise users |
| Must support **multi-tenant SaaS** without leaking events across orgs | future cloud tier |
| Must keep **at-least-once + dedupe** for alerts and agent observations | data correctness |
| Must keep **best-effort fire-and-forget** for UI/status events | marquee can drop noise |
| Reusable from existing `pubsub.Publisher` interface ([kube/alert-ingress/internal/pubsub/publisher.go](../../../kube/alert-ingress/internal/pubsub/publisher.go)) | avoid rewrite churn |
| Single Go binary, no CGo (matches `modernc.org/sqlite` ADR) | release pipeline |
| Ship M1 in ≤2 weeks, no team has prior NATS or Kafka ops experience | timeline + skills |

Non-goals: exactly-once semantics, stream processing/SQL, replay across
months of history.

## Decision

Adopt a **pluggable `pkg/broker` interface** with three implementations,
chosen at runtime from config:

1. **`broker/inproc`** — Go channels. Default for the Wails desktop app.
2. **`broker/nats`** — NATS JetStream. Default for self-hosted SaaS and the
   "self-host one-click" deployment.
3. **`broker/gcp`** — Google Pub/Sub. Used by Argus Cloud (multi-tenant).

The existing `pubsub.Publisher` becomes an adapter over `broker.Broker`, so
`alert-ingress` keeps its current call-site.

Subject taxonomy is locked early (`argus.alert.*`, `argus.k8s.*`,
`argus.status.*`, `argus.envprobe.*`, `argus.user.activity.*`,
`argus.user.suggestion.*`, `argus.agent.<id>.*`, `argus.policy.*`) so
producers and consumers can be developed independently.

Payload format: **protobuf**, one `.proto` file in `proto/argus/v1/`,
gateway-projected to JSON for the Vue side.

## Options Considered

### Option A: NATS JetStream (chosen as the self-hosted/SaaS default)

| Dimension | Assessment |
|---|---|
| Complexity | Low — single binary, helm chart, no ZK/Kraft |
| Cost | Low — 3-pod HA fits a small node pool |
| Scalability | Sufficient — millions of msgs/s/subject, well past target |
| Team familiarity | None — but operational surface is small |
| Corp-network friendliness | Native TLS, can be fronted by HTTPS relay |
| At-least-once + dedupe | Yes (JetStream `Nats-Msg-Id`) |

**Pros**

- Pluggable storage (memory / file / S3) — matches the "no Kafka tax" goal.
- Helm chart ships with mTLS; reuses our [kube/backend/internal/tlsconfig/certs.go](../../../kube/backend/internal/tlsconfig/certs.go) CA story.
- Subjects + queue groups map cleanly to the per-tenant streams we need.
- Same wire works for the in-cluster agent so `hub.go` can be retired.

**Cons**

- New ops surface — but smaller than Kafka by an order of magnitude.
- JetStream tuning (retention, max-bytes) is a footgun if defaults aren't shipped.

### Option B: Apache Kafka (rejected)

| Dimension | Assessment |
|---|---|
| Complexity | High — KRaft or ZK, partition planning, schema registry |
| Cost | Med-High — 3+ brokers, JVM RAM |
| Scalability | Way more than needed |
| Team familiarity | None |
| Corp-network friendliness | Hard — many ports, TLS chain pain |
| At-least-once + dedupe | Yes (idempotent producer) but operationally heavy |

**Pros**

- Largest ecosystem; Flink already uses Kafka connectors (see [kube/flink/job/anomaly_detection.py](../../../kube/flink/job/anomaly_detection.py)).
- Battle-tested for retention/replay.

**Cons**

- Operationally hostile for a small team and for self-hosted users.
- JVM dependency violates the "single Go binary, no CGo" intent.
- Schema registry adds yet another infra component the user must install.
- Partition planning is a long-term tax we don't need today.

### Option C: Google Pub/Sub only (rejected as primary, kept as one of three)

| Dimension | Assessment |
|---|---|
| Complexity | Very low — fully managed |
| Cost | Pay-per-message — fine for SaaS, wrong shape for desktop |
| Scalability | Effectively unbounded |
| Team familiarity | Already used in `alert-ingress` |
| Corp-network friendliness | Outbound HTTPS only — excellent |
| At-least-once + dedupe | Yes |

**Pros**

- Already wired ([kube/alert-ingress/internal/pubsub/gcp.go](../../../kube/alert-ingress/internal/pubsub/gcp.go)).
- Zero ops cost on our side.

**Cons**

- Vendor-locked; on-prem users cannot run it.
- Per-message billing punishes the chatty status/marquee topics.
- Cross-region latency >100 ms is wrong for the in-cluster agent path.

**Decision:** keep as the Argus-Cloud backend for tenants that opt in.

### Option D: Redis Streams (rejected)

| Dimension | Assessment |
|---|---|
| Complexity | Low — single binary we already might want for caching |
| Cost | Low |
| Scalability | OK to ~100K msgs/s on a single node |
| Team familiarity | High |
| Corp-network friendliness | Native TLS, single port |
| At-least-once + dedupe | Consumer-groups give at-least-once; dedupe is DIY |

**Pros**

- Operationally trivial.
- Sub-millisecond latencies.

**Cons**

- Persistence story (AOF/RDB) is not what users expect from a "broker".
- No native fan-out to thousands of subjects without naming gymnastics.
- Forces a second piece of infra later anyway for >1 node.
- Memory-first storage is wrong for replayable audit-grade events.

### Option E: HTTP-only fan-out (rejected)

Status quo, basically. Rejected because it can't satisfy multi-tenant
isolation, the agent-hub bottleneck flagged in `.context.md`, or replay.

## Trade-off Analysis

The core trade-off is **operational footprint vs. ceiling**. Kafka has the
highest ceiling but the highest floor; NATS JetStream has a high enough
ceiling for any realistic Argus deployment with a floor low enough that a
single SRE can run it. Pub/Sub has no floor but doesn't work on-prem.

The pluggable interface neutralises the choice for the desktop user —
they get the in-process channel and never see the broker. The SaaS
tenant gets Pub/Sub. The self-hosted enterprise gets NATS. **The same
event subjects and protobuf payloads flow through all three**, so
producers and consumers don't know which backend they're on.

Cost framing: the marginal cost of supporting three backends is the
adapter layer (~300 LoC each); the marginal cost of *not* supporting
all three is losing either the desktop story, the on-prem story, or
the SaaS story.

## Consequences

**Easier**

- The status marquee, setup checklist, env-probe, and suggester agents
  all become consumers of one stream instead of bespoke wiring.
- `hub.go` agent fan-out can be retired in favour of broker queue groups.
- New event sources (e.g. a future cost-explorer) only need a `Publish`
  call; no UI plumbing per source.
- OpenTelemetry traces follow events end-to-end via `Event.Headers`.

**Harder**

- Three implementations to maintain. We mitigate with a contract test
  suite (`broker/conformance_test.go`) that every implementation must pass.
- Schema evolution becomes a real concern. Mitigated by protobuf +
  reserved field numbers + an `Argus-Schema-Version` header.
- Backpressure semantics differ subtly across backends. The publisher
  API hides this with a SQLite outbox fallback ([kube/backend/internal/sqlitedb/db.go](../../../kube/backend/internal/sqlitedb/db.go)).

**Revisit if**

- A single tenant exceeds ~10K events/s sustained (NATS tuning).
- Replay >7 days becomes a real product requirement (consider Kafka
  with tiered storage, or JetStream with object-store backing).
- A non-Go consumer needs to subscribe (we'd want a stable, language-
  neutral schema gateway).

## Action Items

1. [ ] Define `pkg/broker/broker.go` interface + `Event` struct + `Subscription`.
2. [ ] Implement `broker/inproc` with buffered channels and a SQLite-backed outbox.
3. [ ] Implement `broker/nats` against JetStream; ship a `helm/argus-nats` chart with mTLS defaults reusing [kube/backend/internal/tlsconfig/certs.go](../../../kube/backend/internal/tlsconfig/certs.go).
4. [ ] Implement `broker/gcp` as a thin re-export of the existing [kube/alert-ingress/internal/pubsub/gcp.go](../../../kube/alert-ingress/internal/pubsub/gcp.go) adapted to the new interface.
5. [ ] Write `broker/conformance_test.go` — pub/sub round-trip, dedupe, slow-consumer behaviour, reconnect — and run it against all three.
6. [ ] Migrate `alert-ingress` and `hub.go` to the broker; keep the old call-sites green by adapting `pubsub.Publisher`.
7. [ ] Define `proto/argus/v1/events.proto` covering the locked subject taxonomy; generate Go + a JSON gateway for the Vue side.
8. [ ] Add Prometheus metrics (`argus_broker_publish_total`, `argus_broker_handler_duration_seconds`) on the `KUBEWATCHER_METRICS_PORT` (`9090`) listener.
9. [ ] Add OpenTelemetry trace propagation through `Event.Headers`.
10. [ ] Update [../../../.context.md](../../../.context.md) and [../../../DECISION_LOG.md](../../../DECISION_LOG.md) when M1 lands.
