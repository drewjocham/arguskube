package analysis

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/argues/argus/internal/ai"
	"github.com/argues/argus/pkg/broker"
	"github.com/argues/argus/pkg/loadtest"
)

// fakeChat captures the messages it received and returns a canned
// response (or error). Keeps tests deterministic — the real DeepSeek
// client would need a network round-trip + API key.
type fakeChat struct {
	got      []ai.Message
	response string
	err      error
}

func (f *fakeChat) Chat(_ context.Context, msgs []ai.Message) (string, error) {
	f.got = msgs
	return f.response, f.err
}

func sampleRecord() *loadtest.RunRecord {
	return &loadtest.RunRecord{
		Spec: loadtest.RunSpec{
			Name:        "soak-test",
			Destination: "orders.created",
			Payload:     loadtest.Payload{Kind: loadtest.PayloadKindPasted, Size: 42},
			Ramp:        loadtest.Ramp{Kind: loadtest.RampConstant, Rate: 100},
			Scale: loadtest.ScalePlan{
				Namespace:      "checkout",
				Deployment:     "orders-consumer",
				PreScaleToZero: true,
				MinReplicas:    2,
			},
		},
		BrokerKind: broker.KindKafka,
		Started:    time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC),
		Finished:   time.Date(2026, 5, 14, 10, 5, 0, 0, time.UTC),
		Summary: loadtest.Summary{
			Sent:           100_000,
			Acked:          99_700,
			Errors:         300,
			Duration:       5 * time.Minute,
			Throughput:     333.0,
			P50AckLatency:  10 * time.Millisecond,
			P95AckLatency:  86 * time.Millisecond,
			P99AckLatency:  720 * time.Millisecond,
			MaxAckLatency:  1500 * time.Millisecond,
			ErrorBreakdown: map[string]int{"publish timeout": 280, "topic full": 20},
		},
		ScaleLog: []loadtest.ScaleEvent{
			{At: time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC), Phase: "pre-scale", Replicas: 2, Ready: 2},
			{At: time.Date(2026, 5, 14, 10, 0, 10, 0, time.UTC), Phase: "publishing", Replicas: 0, Ready: 0},
			{At: time.Date(2026, 5, 14, 10, 4, 0, 0, time.UTC), Phase: "scaling-up", Replicas: 2, Ready: 0},
			{At: time.Date(2026, 5, 14, 10, 4, 38, 0, time.UTC), Phase: "draining", Replicas: 2, Ready: 2},
		},
	}
}

func TestNarrate_HappyPath_PassesRecordDetailsToLLM(t *testing.T) {
	chat := &fakeChat{response: "Cold-start drain completed cleanly. Backlog was 100k; both replicas were Ready 38 seconds after scale-up. P99 hit 720ms during the publish phase, driven by the publish-timeout errors (0.3% of sends). Throughput averaged 333 msg/s. Recommend keeping minReplicas at 2 for this workload."}
	n := New(chat)

	out, err := n.Narrate(context.Background(), sampleRecord())
	if err != nil {
		t.Fatalf("Narrate: %v", err)
	}
	if !strings.Contains(out, "Cold-start") {
		t.Errorf("output didn't include canned response: %q", out)
	}

	// The user prompt should include the key facts the LLM needs.
	if len(chat.got) != 2 {
		t.Fatalf("got %d messages, want 2 (system + user)", len(chat.got))
	}
	if chat.got[0].Role != "system" {
		t.Errorf("first message role = %q, want system", chat.got[0].Role)
	}
	user := chat.got[1].Content
	for _, want := range []string{
		"kafka",
		"soak-test",
		"checkout/orders-consumer",
		"sent=100000",
		"errors=300",
		"p95=86ms",
		"publish timeout: 280",
		"scaling-up",
	} {
		if !strings.Contains(user, want) {
			t.Errorf("user prompt missing %q\n--- prompt ---\n%s", want, user)
		}
	}
}

func TestNarrate_NoClient(t *testing.T) {
	n := New(nil)
	_, err := n.Narrate(context.Background(), sampleRecord())
	if !errors.Is(err, ErrNoClient) {
		t.Errorf("err = %v, want ErrNoClient", err)
	}
}

func TestNarrate_NilNarrator(t *testing.T) {
	var n *Narrator
	_, err := n.Narrate(context.Background(), sampleRecord())
	if !errors.Is(err, ErrNoClient) {
		t.Errorf("err = %v, want ErrNoClient", err)
	}
}

func TestNarrate_NilRecord(t *testing.T) {
	n := New(&fakeChat{response: "ok"})
	_, err := n.Narrate(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil record")
	}
}

func TestNarrate_ChatError_Wrapped(t *testing.T) {
	n := New(&fakeChat{err: errors.New("rate limit exceeded")})
	_, err := n.Narrate(context.Background(), sampleRecord())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "narrate") {
		t.Errorf("error not wrapped with narrate context: %v", err)
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("error didn't preserve LLM message: %v", err)
	}
}

func TestNarrate_EmptyResponse_Errored(t *testing.T) {
	n := New(&fakeChat{response: "   "}) // only whitespace
	_, err := n.Narrate(context.Background(), sampleRecord())
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestBuildUserPrompt_AbortedRunMentionsFinalError(t *testing.T) {
	rec := sampleRecord()
	rec.FinalError = "broker connect: dial tcp 10.0.0.1:9092: timeout"
	got := buildUserPrompt(rec)
	if !strings.Contains(got, "Final error (aborted)") {
		t.Error("aborted run should include Final error section")
	}
	if !strings.Contains(got, "dial tcp 10.0.0.1") {
		t.Error("aborted run should include the error text")
	}
}

func TestBuildUserPrompt_NoScaleLog_OmitsSection(t *testing.T) {
	rec := sampleRecord()
	rec.ScaleLog = nil
	got := buildUserPrompt(rec)
	if strings.Contains(got, "Scale timeline") {
		t.Error("no scale log should omit the timeline section")
	}
}

// Compile-check that DeepSeekClient satisfies our Chatter interface.
// If ai.DeepSeekClient ever drops Chat(ctx, []Message), this stops
// compiling and we know to update the narrator's expectations.
var _ Chatter = (*ai.DeepSeekClient)(nil)
