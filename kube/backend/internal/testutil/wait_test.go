package testutil

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestWaitForReturnsWhenConditionTrue(t *testing.T) {
	t.Parallel()
	var hit atomic.Bool
	go func() {
		time.Sleep(10 * time.Millisecond)
		hit.Store(true)
	}()
	start := time.Now()
	WaitFor(t, time.Second, 5*time.Millisecond, hit.Load, "hit never flipped")
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Errorf("WaitFor took %s — should be ~10ms", elapsed)
	}
}

func TestWaitForReturnsImmediatelyOnAlreadyTrue(t *testing.T) {
	t.Parallel()
	start := time.Now()
	WaitFor(t, time.Second, time.Hour, func() bool { return true }, "already true")
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Errorf("WaitFor took %s on an already-true condition; should be ~0", elapsed)
	}
}
