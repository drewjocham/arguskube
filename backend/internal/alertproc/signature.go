package alertproc

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/argues/kube-watcher/internal/alerts"
)

// Signature is a stable hash that collapses near-duplicate alerts
// into one logical group. Two alerts with the same Signature are
// "the same problem firing again." The hash deliberately ignores
// fields that drift between firings (timestamps, restart count,
// random pod-suffix uuids) and keeps the parts that identify the
// underlying issue.
//
// The signature design matters because:
//   - Dedupe correctness — two CrashLoops on the same Deployment
//     should NOT count as two separate alerts.
//   - Fatigue tracking — we count silences against the signature,
//     not the alert ID, so the user sees noise warnings at the
//     deployment level rather than per-pod.
type Signature string

// SignatureOf computes the alert signature. The included fields are
// the ones an SRE would call out as "this is the same issue":
//
//   name + severity + namespace + pod-name-base + node + image-tag
//
// We strip the random suffix from pod names (`payments-api-7c9d-abc12`
// → `payments-api`) so a Deployment's churn doesn't create new groups
// every time the ReplicaSet recycles a pod.
func SignatureOf(a alerts.Alert) Signature {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%s|%s|%s",
		a.Name,
		a.Severity,
		a.Namespace,
		stripPodSuffix(a.PodName),
		a.NodeName,
		a.ImageTag,
	)
	return Signature(hex.EncodeToString(h.Sum(nil))[:16])
}

// stripPodSuffix turns "payments-api-7c9d4b6f8d-abc12" into
// "payments-api". The Kubernetes ReplicaSet suffix is two segments
// of hex-ish characters at the end, so we trim them when present.
// If the input doesn't look like a Deployment-managed pod (e.g. a
// StatefulSet member with a numeric index), we leave it alone.
func stripPodSuffix(podName string) string {
	if podName == "" {
		return ""
	}
	// Walk backwards through the dash-separated segments; drop the
	// last two only if both look like generated suffixes.
	segments := splitDashes(podName)
	if len(segments) < 3 {
		return podName
	}
	last := segments[len(segments)-1]
	prev := segments[len(segments)-2]
	if isPodSuffix(last) && isPodSuffix(prev) {
		return joinDashes(segments[:len(segments)-2])
	}
	return podName
}

func splitDashes(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == '-' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	out = append(out, cur)
	return out
}

func joinDashes(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "-"
		}
		out += p
	}
	return out
}

// isPodSuffix returns true for short alphanumeric strings that look
// like the random tail Kubernetes appends to pods. Specifically:
// 5-10 chars, lowercase alphanumeric, NOT a stable word like "primary".
func isPodSuffix(s string) bool {
	if len(s) < 5 || len(s) > 10 {
		return false
	}
	hasDigit := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			return false
		}
	}
	// The ReplicaSet hash always contains digits. "primary" doesn't,
	// "abc12" does — that's the discriminator.
	return hasDigit
}
