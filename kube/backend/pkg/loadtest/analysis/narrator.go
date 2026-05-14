// Package analysis turns a load-test RunRecord into a Markdown
// narrative an operator can read in 30 seconds.
//
// The narrator does NOT replace the deterministic Markdown writer
// from PR-C — instead, it wraps it. PR-C's renderer still produces
// the structured Summary / Errors / Scale-timeline / Raw-JSON
// sections. The narrator prepends a "Narrative" section above those,
// produced by an LLM call. If no LLM client is configured (or the
// call fails), the file is written without the narrative section —
// the operator gets the deterministic data and a one-line note that
// the narrative was unavailable, instead of a half-broken render.
//
// File shape, before-and-after:
//
//	+ frontmatter (YAML)              ← unchanged
//	+ # Load test report              ← unchanged
//	+ ## Narrative                    ← NEW (added by this package)
//	+ <agent prose>                   ← NEW
//	+ ## Summary                      ← unchanged
//	+ ...                             ← unchanged
//
// PR-D's "View report" link consumes the file via GetFile and
// renders it as-is; this layering means PR-D doesn't need to know
// whether a narrative is present.
package analysis

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/argues/argus/internal/ai"
	"github.com/argues/argus/pkg/loadtest"
)

// Chatter is the small surface the narrator needs from the LLM
// client. ai.DeepSeekClient already satisfies it; tests substitute a
// fake so the package compiles + tests pass without an API key.
type Chatter interface {
	Chat(ctx context.Context, messages []ai.Message) (string, error)
}

// Narrator turns RunRecords into narratives. Holds the Chatter so a
// caller wires the client once and reuses it across runs.
type Narrator struct {
	chat Chatter
}

// New returns a Narrator wrapping the given Chatter. A nil chat is
// allowed — Narrate() then returns an explicit "no client" error
// the caller can treat as "skip the narrative section".
func New(chat Chatter) *Narrator {
	return &Narrator{chat: chat}
}

// ErrNoClient is returned by Narrate when the Narrator has no
// Chatter wired. Callers compare via errors.Is so the export path
// can degrade gracefully without parsing the error string.
var ErrNoClient = errors.New("analysis: no LLM client configured")

// Narrate produces the prose body. Plain text (no Markdown fences),
// 3–6 short paragraphs, written for an SRE who's about to look at
// the structured Summary below it. Examples of what the narrator
// covers:
//
//   - "Backlog peaked at 47k messages while the consumer was at 0
//     replicas. Scale-up to 2 replicas took 38s; first replica
//     drained 220 msg/s, second hit steady state at 410 msg/s.
//     Full clear in 4m12s after scale-up."
//   - "Error rate 0.3% (mostly publish timeouts at the peak of the
//     ramp). P95 ack latency 86ms, P99 720ms — the tail looks
//     coordinated with the burst at t+2m."
//
// Returns the prose (no surrounding section header — the caller
// places it) or an error if the LLM call fails. The error path
// includes context so the operator log shows e.g. "narrate: rate
// limit" rather than a bare LLM error.
func (n *Narrator) Narrate(ctx context.Context, rec *loadtest.RunRecord) (string, error) {
	if n == nil || n.chat == nil {
		return "", ErrNoClient
	}
	if rec == nil {
		return "", errors.New("narrate: nil record")
	}
	msgs := []ai.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: buildUserPrompt(rec)},
	}
	out, err := n.chat.Chat(ctx, msgs)
	if err != nil {
		return "", fmt.Errorf("narrate: %w", err)
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", errors.New("narrate: empty response from LLM")
	}
	return out, nil
}

const systemPrompt = `You are an SRE writing a 3–6 paragraph
narrative summary of a Kubernetes consumer load test. The user will
read it once, then look at the structured tables that follow.

Rules:
1. Plain prose. No Markdown headers, no bullet lists, no code fences.
2. Cite specific numbers from the data (e.g. "P95 of 86ms", "scaled
   up at t+38s"). Round latency to the nearest millisecond and
   timestamps to the nearest second.
3. Highlight the most interesting finding first. If there's a
   problem (high error rate, blown P99, slow scale-up), lead with
   that. If everything looks fine, say so clearly.
4. Avoid generic SRE platitudes ("performance was acceptable",
   "recommend further investigation"). Either there's a specific
   actionable observation or there isn't.
5. End with one sentence on what to do next, if anything.
6. Length: ~150 words. Under 250 words always.`

// buildUserPrompt assembles the data the LLM needs into a compact
// JSON-shaped block. We send only the aggregated summary, the scale
// log, and the error breakdown — never the raw Sample stream, which
// at the 1M-message ceiling would blow well past any model's input
// budget. The summary is enough; the agent has nothing to add by
// looking at each individual ack.
func buildUserPrompt(rec *loadtest.RunRecord) string {
	var b strings.Builder
	b.WriteString("Load test record. Summarize in 3–6 paragraphs of prose.\n\n")
	fmt.Fprintf(&b, "Broker: %s\n", rec.BrokerKind)
	if rec.Spec.Name != "" {
		fmt.Fprintf(&b, "Test name: %s\n", rec.Spec.Name)
	}
	fmt.Fprintf(&b, "Ramp kind: %s\n", rec.Spec.Ramp.Kind)
	if rec.Spec.Scale.Deployment != "" {
		fmt.Fprintf(&b, "Target deployment: %s/%s (pre-scale-to-zero=%v, post min replicas=%d)\n",
			rec.Spec.Scale.Namespace, rec.Spec.Scale.Deployment,
			rec.Spec.Scale.PreScaleToZero, rec.Spec.Scale.MinReplicas)
	}
	fmt.Fprintf(&b, "Wall-clock duration: %s\n\n", rec.Summary.Duration)

	b.WriteString("Summary:\n")
	fmt.Fprintf(&b, "  sent=%d acked=%d errors=%d\n", rec.Summary.Sent, rec.Summary.Acked, rec.Summary.Errors)
	fmt.Fprintf(&b, "  throughput=%.1f msg/s\n", rec.Summary.Throughput)
	fmt.Fprintf(&b, "  p50=%s p95=%s p99=%s max=%s\n\n",
		rec.Summary.P50AckLatency,
		rec.Summary.P95AckLatency,
		rec.Summary.P99AckLatency,
		rec.Summary.MaxAckLatency)

	if len(rec.Summary.ErrorBreakdown) > 0 {
		b.WriteString("Errors by kind:\n")
		for k, v := range rec.Summary.ErrorBreakdown {
			fmt.Fprintf(&b, "  %s: %d\n", k, v)
		}
		b.WriteString("\n")
	}

	if len(rec.ScaleLog) > 0 {
		b.WriteString("Scale timeline (phase, spec replicas, ready replicas, time-since-start):\n")
		base := rec.Started
		for _, ev := range rec.ScaleLog {
			fmt.Fprintf(&b, "  %s ts+%s spec=%d ready=%d\n",
				ev.Phase, ev.At.Sub(base).Round(0), ev.Replicas, ev.Ready)
		}
		b.WriteString("\n")
	}

	if rec.FinalError != "" {
		fmt.Fprintf(&b, "Final error (aborted): %s\n", rec.FinalError)
	}
	return b.String()
}
