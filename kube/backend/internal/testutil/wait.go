// Package testutil holds small helpers shared across the test suites
// that don't have a natural home in any single package.
package testutil

import (
	"testing"
	"time"
)

// WaitFor polls cond every interval until it returns true or the
// timeout elapses, in which case the test fails with msg. Use this
// instead of `time.Sleep(N)` followed by an assertion — that pattern
// is fragile under load (slow CI, race detector overhead) and waits
// the full N when the condition might be satisfied in milliseconds.
//
// Example:
//
//	agent.AutoInvestigate(ctx, alert, ...)
//	testutil.WaitFor(t, time.Second, 5*time.Millisecond,
//	    func() bool { return inv.Calls() > 0 },
//	    "investigator was not invoked")
//
// timeout is an upper bound — the function returns as soon as cond is
// true. interval should be short enough that the polling latency is a
// small fraction of timeout (typically 10–50ms vs 1–5s).
func WaitFor(t *testing.T, timeout, interval time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("WaitFor: %s (timeout %s)", msg, timeout)
		}
		time.Sleep(interval)
	}
}
