package pkg

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// upgrader gates which origins can establish a WebSocket tunnel. The
// previous implementation accepted any origin, which would have let any
// website JS open a tunnel to a misconfigured server. We now defer to the
// same allowlist used by the REST API: localhost / loopback always, plus
// anything in ARGUS_API_ALLOWED_ORIGINS.
//
// Agents that connect machine-to-machine over mTLS don't carry an Origin
// header (it's a browser concept) and pass through cleanly.
// hubUpgrader is built by HandleTunnel rather than declared as a package
// var because the empty-Origin gate depends on the receiver's TLS state.
// Browsers always send Origin, so empty-Origin means "non-browser
// client" — fine for our in-cluster mTLS agent, but a free pass for any
// other process on the box if we ignore Origin unconditionally. Allow
// empty Origin only when (a) mTLS is configured for this hub (the agent
// path) or (b) the request came from loopback (the embedded-app path).
// Otherwise defer to the existing allowlist.
func (h *Hub) buildUpgrader() websocket.Upgrader {
	return websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return h.tlsCfg != nil || remoteIsLocal(r)
			}
			return originAllowed(origin, allowedOrigins())
		},
	}
}

// TunnelMessage represents a payload sent over the WebSocket.
type TunnelMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// AgentConnection represents a single connected agent.
type AgentConnection struct {
	ID        string
	Namespace string
	Conn      *websocket.Conn
	Send      chan []byte
}

// Hub manages active agent connections and routes messages.
type Hub struct {
	clients    map[string]*AgentConnection
	register   chan *AgentConnection
	unregister chan *AgentConnection
	logger     *slog.Logger
	mu         sync.RWMutex
	tlsCfg     *tls.Config // When set, hub requires mTLS for agent connections.

	// done is closed when Run() exits. Pump goroutines watch this so the
	// `h.unregister <- client` send in their defer can never deadlock if
	// the Run loop has already returned — previously, a pump that hit
	// its read/write deadline after shutdown would block forever on that
	// send and leak the goroutine plus the underlying socket.
	done chan struct{}
}

// NewHub creates a new WebSocket Hub.
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients:    make(map[string]*AgentConnection),
		register:   make(chan *AgentConnection),
		unregister: make(chan *AgentConnection),
		logger:     logger,
		done:       make(chan struct{}),
	}
}

// WithTLS configures mTLS for the hub. The tls.Config should be created via
// tlsconfig.ServerTLSConfig() with the CA cert and server cert/key.
func (h *Hub) WithTLS(cfg *tls.Config) *Hub {
	h.tlsCfg = cfg
	return h
}

// TLSConfig returns the hub's TLS config, or nil if mTLS is not configured.
func (h *Hub) TLSConfig() *tls.Config {
	return h.tlsCfg
}

// Run starts the hub's main event loop. Closes h.done when ctx fires
// so any in-flight pump goroutines can break their unregister send
// instead of blocking forever on a channel nothing is reading from.
func (h *Hub) Run(ctx context.Context) {
	defer close(h.done)
	for {
		select {
		case <-ctx.Done():
			// Close every still-open conn so read/writePump exit their
			// loops promptly instead of waiting on the 60s read deadline.
			h.mu.Lock()
			for _, c := range h.clients {
				_ = c.Conn.Close()
			}
			h.mu.Unlock()
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.mu.Unlock()
			h.logger.Info("Agent registered", slog.String("agent_id", client.ID), slog.String("namespace", client.Namespace))
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				close(client.Send)
				h.logger.Info("Agent unregistered", slog.String("agent_id", client.ID))
			}
			h.mu.Unlock()
		}
	}
}

// HandleTunnel upgrades the HTTP connection and handles agent communication.
// When mTLS is configured, the agent's certificate CN must match the agent_id.
func (h *Hub) HandleTunnel(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent_id")
	namespace := r.URL.Query().Get("namespace")
	if agentID == "" {
		http.Error(w, "Missing agent_id", http.StatusBadRequest)
		return
	}

	// Validate mTLS client certificate if TLS is configured.
	if h.tlsCfg != nil && r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
		clientCN := r.TLS.PeerCertificates[0].Subject.CommonName
		if clientCN != agentID {
			h.logger.Warn("mTLS agent ID mismatch",
				slog.String("cert_cn", clientCN),
				slog.String("claimed_id", agentID),
			)
			http.Error(w, "Certificate CN does not match agent_id", http.StatusForbidden)
			return
		}
		h.logger.Debug("mTLS agent verified", slog.String("agent_id", agentID))
	} else if h.tlsCfg != nil {
		// mTLS configured but no client cert presented — reject.
		h.logger.Warn("Agent connection rejected: no client certificate", slog.String("agent_id", agentID))
		http.Error(w, "Client certificate required", http.StatusUnauthorized)
		return
	}

	upgrader := h.buildUpgrader()
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade websocket", slog.String("error", err.Error()))
		return
	}

	client := &AgentConnection{
		ID:        agentID,
		Namespace: namespace,
		Conn:      conn,
		Send:      make(chan []byte, 256),
	}

	h.register <- client

	// Start read/write pumps.
	go h.writePump(client)
	go h.readPump(client)
}

// unregisterClient is the safe send used by pump defers. Once Run() has
// exited, h.done is closed and the unregister channel has no receiver;
// blocking on it would leak this goroutine. The select bails out in
// that case and just closes the conn locally.
func (h *Hub) unregisterClient(client *AgentConnection) {
	select {
	case h.unregister <- client:
	case <-h.done:
	}
}

func (h *Hub) readPump(client *AgentConnection) {
	defer func() {
		h.unregisterClient(client)
		client.Conn.Close()
	}()

	client.Conn.SetReadLimit(512 * 1024) // 512KB limit
	_ = client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		_ = client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Error("Websocket close error", slog.String("error", err.Error()))
			}
			break
		}

		// Here we would route the incoming telemetry/anomalies from the agent to the incident store or memory cache.
		h.logger.Debug("Received message from agent", slog.String("agent_id", client.ID), slog.Int("bytes", len(message)))
	}
}

func (h *Hub) writePump(client *AgentConnection) {
	ticker := time.NewTicker(54 * time.Second) // Ping ticker
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			_ = client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := client.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// SendCommand pushes a command (like ArgusCD deployments) down to a specific agent tunnel.
func (h *Hub) SendCommand(agentID string, cmdType string, payload []byte) error {
	h.mu.RLock()
	client, ok := h.clients[agentID]
	h.mu.RUnlock()

	if !ok {
		return fmt.Errorf("agent %s is not currently connected", agentID)
	}

	msg := TunnelMessage{
		Type:    cmdType,
		Payload: payload,
	}

	bytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case client.Send <- bytes:
		return nil
	default:
		return fmt.Errorf("agent %s tunnel is blocked/full", agentID)
	}
}
