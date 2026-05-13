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
