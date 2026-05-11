package tunnel

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/argues/argus/agent/internal/cd"
	"github.com/gorilla/websocket"
)

// Client represents the outbound WebSocket connection to the SaaS backend.
type Client struct {
	serverURL string
	agentID   string
	namespace string
	logger    *slog.Logger
	conn      *websocket.Conn
	send      chan []byte
	tlsCfg    *tls.Config // nil = no mTLS (insecure, dev only)
}

// NewClient creates a new Tunnel client.
func NewClient(serverURL, agentID, namespace string, logger *slog.Logger) *Client {
	return &Client{
		serverURL: serverURL,
		agentID:   agentID,
		namespace: namespace,
		logger:    logger,
		send:      make(chan []byte, 256),
	}
}

// WithTLS configures mTLS for the tunnel connection. The tls.Config should be
// created via tlsconfig.AgentTLSConfig() with the CA cert and agent cert/key.
func (c *Client) WithTLS(cfg *tls.Config) *Client {
	c.tlsCfg = cfg
	return c
}

// Start connects to the SaaS backend and maintains the connection.
func (c *Client) Start(ctx context.Context) error {
	u, err := url.Parse(c.serverURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	// Change schema to ws/wss
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else if u.Scheme == "http" {
		u.Scheme = "ws"
	}

	q := u.Query()
	q.Set("agent_id", c.agentID)
	q.Set("namespace", c.namespace)
	u.RawQuery = q.Encode()

	c.logger.Info("Starting SaaS tunnel connection", slog.String("url", u.String()))

	// Connection loop with exponential backoff
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Shutting down tunnel client")
			return nil
		default:
			err := c.connectAndRun(ctx, u.String())
			if err != nil {
				c.logger.Error("Tunnel disconnected, reconnecting...", slog.String("error", err.Error()), slog.Duration("backoff", backoff))
				time.Sleep(backoff)
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			} else {
				// Reset backoff on successful run that exited cleanly
				backoff = time.Second
			}
		}
	}
}

func (c *Client) connectAndRun(ctx context.Context, u string) error {
	dialer := *websocket.DefaultDialer
	if c.tlsCfg != nil {
		dialer.TLSClientConfig = c.tlsCfg
		dialer.HandshakeTimeout = 15 * time.Second
	}

	headers := http.Header{}
	headers.Set("X-Agent-ID", c.agentID)

	conn, _, err := dialer.DialContext(ctx, u, headers)
	if err != nil {
		return err
	}
	c.conn = conn
	defer c.conn.Close()

	c.logger.Info("Successfully connected to SaaS backend")

	// Create error channels for read/write loops
	errChan := make(chan error, 2)
	
	// Create CD applier
	applier := cd.NewApplier(c.logger.With("component", "cd"))

	go func() {
		defer c.logger.Debug("Read loop exited")
		for {
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
			
			// Dispatch incoming RPC commands from SaaS (e.g., ArgusCD apply)
			c.logger.Debug("Received command from SaaS", slog.Int("bytes", len(message)))
			
			// Define a temporary struct to match TunnelMessage
			var msg struct {
				Type    string          `json:"type"`
				Payload json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(message, &msg); err == nil {
				if msg.Type == "ApplyManifest" {
					var payload struct {
						YAML string `json:"yaml"`
					}
					if err := json.Unmarshal(msg.Payload, &payload); err == nil {
						// Run apply in a goroutine to not block the read loop
						go applier.ApplyManifest(ctx, []byte(payload.YAML))
					}
				}
			}
		}
	}()

	go func() {
		defer c.logger.Debug("Write loop exited")
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			case msg := <-c.send:
				if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					errChan <- err
					return
				}
			case <-ticker.C:
				if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					errChan <- err
					return
				}
			}
		}
	}()

	// Wait for an error from either loop, or context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}

// Send broadcasts a message to the SaaS backend.
func (c *Client) Send(data []byte) {
	select {
	case c.send <- data:
	default:
		c.logger.Warn("Tunnel send buffer full, dropping message")
	}
}
