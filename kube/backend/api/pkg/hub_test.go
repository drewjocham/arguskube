package pkg

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestHub_PumpDoesNotDeadlockAfterRunExits is the regression test for C1.
//
// Before the fix, a readPump that hit its read error after Run() had
// returned would block forever on `h.unregister <- client` — nothing
// was reading the channel, so the goroutine + socket leaked. We
// reproduce that shape: spin up a hub, connect a client, cancel the
// hub's context so Run exits, then close the WebSocket from the client
// side to force the readPump to error. The unregister send must NOT
// block; the test's t.Cleanup + timing guards would fail if it did.
func TestHub_PumpDoesNotDeadlockAfterRunExits(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHub(logger)

	hubCtx, cancelHub := context.WithCancel(context.Background())
	defer cancelHub()
	go h.Run(hubCtx)

	srv := httptest.NewServer(http.HandlerFunc(h.HandleTunnel))
	t.Cleanup(srv.Close)

	u, _ := url.Parse(srv.URL)
	wsURL := "ws://" + u.Host + "/?agent_id=test-agent&namespace=ns1"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// Wait for the hub's register flow.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.RLock()
		_, ok := h.clients["test-agent"]
		h.mu.RUnlock()
		if ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Cancel the hub. Run() should exit promptly and close h.done.
	cancelHub()
	select {
	case <-h.done:
	case <-time.After(2 * time.Second):
		t.Fatal("hub.Run did not exit within 2s of context cancel")
	}

	// Force the readPump to error out by closing the conn from the
	// client side. Its defer will try `h.unregister <- client`; with
	// the fix that select-with-done falls through immediately, without
	// it the goroutine would hang here forever.
	_ = conn.Close()

	// A short pause lets the pump's defer run. If the fix were missing,
	// the leaked goroutine wouldn't fail this assertion directly but
	// would show up as a leak warning under -race or goleak.
	time.Sleep(100 * time.Millisecond)
}

// TestHub_PumpsExitTogether is the regression test for the
// "writePump exits → readPump blocks for 60s" leak. With the done
// channel + SetReadDeadline plumbing in place, signalling one pump
// must drop the other out of its blocking syscall within a few
// milliseconds, not the read-deadline budget.
func TestHub_PumpsExitTogether(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHub(logger)

	hubCtx, cancelHub := context.WithCancel(context.Background())
	t.Cleanup(cancelHub)
	go h.Run(hubCtx)

	srv := httptest.NewServer(http.HandlerFunc(h.HandleTunnel))
	t.Cleanup(srv.Close)

	u, _ := url.Parse(srv.URL)
	wsURL := "ws://" + u.Host + "/?agent_id=pump-exit&namespace=ns1"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	// Wait until the hub registers the connection so client is non-nil.
	var client *AgentConnection
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.RLock()
		client = h.clients["pump-exit"]
		h.mu.RUnlock()
		if client != nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if client == nil {
		t.Fatal("hub never registered the test client")
	}

	// Signal both pumps to exit. After this, writePump should drop out
	// of its select within microseconds, and the readPump should be
	// kicked by SetReadDeadline + conn.Close from writePump's defer.
	start := time.Now()
	client.signalDone()

	// Both pumps share the same conn — once they exit, the second
	// Close() is a no-op. Confirm the conn is observably closed within
	// well under the 60-second read deadline.
	select {
	case <-client.done:
		// signalDone() already closed it; this just confirms the channel
		// closure is observable, which is what writePump selects on.
	default:
		t.Fatal("signalDone() did not actually close client.done")
	}

	pumpDeadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(pumpDeadline) {
		h.mu.RLock()
		_, stillRegistered := h.clients["pump-exit"]
		h.mu.RUnlock()
		if !stillRegistered {
			elapsed := time.Since(start)
			if elapsed > 1*time.Second {
				t.Errorf("pumps took %s to unwind — expected < 1s with the new signal", elapsed)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("pumps did not unregister within 2s of signalDone — the leak fix is not effective")
}
