package workspace

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Slack Events API + slash commands — SaaS-only inbound.
//
// What this file owns:
//   - Signature verification (HMAC-SHA256 over `v0:ts:body` with the
//     app's Signing Secret) so an arbitrary HTTP client can't forge
//     event deliveries.
//   - URL-verification handshake (Slack's first POST during app setup
//     contains `type:url_verification` + a challenge string we echo).
//   - A small router that fans event payloads to registered handlers
//     plus a bounded in-memory ring buffer the UI can poll to render
//     a "recent events" feed.
//
// What this file deliberately doesn't own:
//   - Long-running event-handler logic. Slack's 3-second response
//     budget means we ack immediately and hand the payload to a
//     goroutine via go EventBus.Dispatch — handlers must be cheap.
//   - Slash-command business logic (e.g. "/argus alert list"). The
//     command dispatch is built; what each command DOES lives in the
//     consumer (api/pkg).

// EventBus is the receiving side of the Slack Events API. One bus per
// process. Initialized in main.go alongside the SlackProvider when
// ARGUS_SLACK_SIGNING_SECRET is set.
type EventBus struct {
	signingSecret string
	logger        loggerLike
	maxAge        time.Duration

	// Recent events buffer — ring of the last N inbound events, used
	// by the desktop UI to render an Inbound tab. Bounded so a chatty
	// channel doesn't bloat memory.
	mu     sync.Mutex
	recent []RecentEvent
	cap    int

	// Slash-command handlers keyed by command (e.g. "/argus"). When
	// nil/unset, slash commands respond with a generic "command not
	// configured" message.
	commands map[string]CommandHandler
}

// loggerLike narrows *slog.Logger to just InfoContext for unit
// testing. We don't want to import slog here for the package's tests.
type loggerLike interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
}

// RecentEvent is the projection of an inbound event the UI sees. We
// keep enough to render a one-liner per row plus the raw payload for
// debugging; tokens / signing material never enter this struct.
type RecentEvent struct {
	ReceivedAt time.Time `json:"received_at"`
	Kind       string    `json:"kind"`   // "event" | "slash_command" | "interactive"
	Subtype    string    `json:"subtype,omitempty"`
	TeamID     string    `json:"team_id,omitempty"`
	UserID     string    `json:"user_id,omitempty"`
	Channel    string    `json:"channel,omitempty"`
	Text       string    `json:"text,omitempty"`
	// Raw is the full payload as JSON-encoded bytes. Capped at 8 KB so
	// a large message_attachments blob doesn't pin memory.
	Raw json.RawMessage `json:"raw,omitempty"`
}

// CommandHandler returns the message body Slack should display to the
// invoking user. Implementations have ~3 seconds — anything slower
// should respond synchronously with "working on it" and use the
// response_url field of the inbound payload for the async reply.
type CommandHandler func(cmd SlashCommand) string

// SlashCommand is the form-decoded payload of `/your-command` invokes.
// We surface only the fields a handler typically needs.
type SlashCommand struct {
	TeamID      string `json:"team_id"`
	TeamDomain  string `json:"team_domain"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`
	Command     string `json:"command"`
	Text        string `json:"text"`
	ResponseURL string `json:"response_url"`
	TriggerID   string `json:"trigger_id"`
}

// NewEventBus constructs an EventBus. signingSecret is what Slack's
// app config calls the "Signing Secret" — without it Verify always
// fails closed.
func NewEventBus(signingSecret string, logger loggerLike) *EventBus {
	return &EventBus{
		signingSecret: signingSecret,
		logger:        logger,
		maxAge:        5 * time.Minute,
		cap:           50,
		commands:      map[string]CommandHandler{},
	}
}

// RegisterCommand installs a handler for a slash command. Command is
// the slash-prefixed name, e.g. "/argus". Re-registering panics so
// duplicate-init shows up at boot, not at first user invoke.
func (b *EventBus) RegisterCommand(command string, fn CommandHandler) {
	if !strings.HasPrefix(command, "/") {
		panic("workspace: slash commands must start with '/'")
	}
	if _, ok := b.commands[command]; ok {
		panic(fmt.Sprintf("workspace: command %q already registered", command))
	}
	b.commands[command] = fn
}

// Verify validates the X-Slack-Signature header against the body.
// Caller is responsible for reading the body BEFORE calling Verify —
// the body bytes feed the HMAC. Timestamps older than maxAge are
// rejected to thwart replay attacks (Slack's own guidance).
//
// Returns nil on success; a wrapped error otherwise. The error
// messages are deliberately bland because they end up in HTTP 401
// responses — leaking which check failed helps attackers.
func (b *EventBus) Verify(signature, timestamp string, body []byte) error {
	if b.signingSecret == "" {
		return errors.New("slack events: signing secret not configured")
	}
	if signature == "" || timestamp == "" {
		return errors.New("slack events: missing signature headers")
	}

	tsInt, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return errors.New("slack events: bad timestamp")
	}
	age := time.Since(time.Unix(tsInt, 0))
	if age < 0 {
		age = -age
	}
	if age > b.maxAge {
		return errors.New("slack events: signature timestamp out of window")
	}

	// Slack's signature format: v0=<hex>. We compute the same.
	basestring := "v0:" + timestamp + ":" + string(body)
	mac := hmac.New(sha256.New, []byte(b.signingSecret))
	mac.Write([]byte(basestring))
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return errors.New("slack events: signature mismatch")
	}
	return nil
}

// challengeRequest is the URL-verification shape Slack sends once per
// app config to confirm the endpoint is alive. We echo `challenge`
// back as plain text.
type challengeRequest struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge"`
}

// Outcome encodes what the HTTP handler should do with the response.
// It exists because Slack's endpoint takes three shapes (challenge
// echo, JSON body, plain text), and the handler keeps the HTTP
// shape decisions outside the bus.
type Outcome struct {
	// ContentType + Body — when set, write these directly to the
	// response. ContentType "" means use "application/json".
	Body        []byte
	ContentType string
	// StatusCode — defaults to 200 when zero.
	StatusCode int
}

// HandleEvent processes an Events-API JSON payload (NOT a slash
// command — those are form-encoded). The caller has already verified
// signature + freshness via Verify.
func (b *EventBus) HandleEvent(body []byte) (Outcome, error) {
	// First, parse just enough to discriminate the message type.
	var head struct {
		Type      string          `json:"type"`
		Challenge string          `json:"challenge"`
		TeamID    string          `json:"team_id"`
		Event     json.RawMessage `json:"event"`
	}
	if err := json.Unmarshal(body, &head); err != nil {
		return Outcome{}, fmt.Errorf("parse event envelope: %w", err)
	}

	switch head.Type {
	case "url_verification":
		// Echo the challenge as text/plain. Slack accepts JSON too but
		// the docs example is text and our infra is simpler this way.
		return Outcome{
			Body:        []byte(head.Challenge),
			ContentType: "text/plain; charset=utf-8",
		}, nil
	case "event_callback":
		b.recordEvent(head.TeamID, head.Event)
		// Ack immediately — handlers are free to be async.
		return Outcome{Body: []byte("{}")}, nil
	}
	// Unknown top-level type: ack anyway so Slack doesn't keep
	// retrying; log it so we know to add support.
	if b.logger != nil {
		b.logger.Warn("slack events: unknown envelope type", "type", head.Type)
	}
	return Outcome{Body: []byte("{}")}, nil
}

// HandleSlashCommand processes a `/command` invocation. Body is the
// raw form-encoded payload Slack POSTed; the caller passes it through
// pre-verify. Returns an Outcome whose Body is the response text the
// invoking user sees.
func (b *EventBus) HandleSlashCommand(form map[string][]string) (Outcome, error) {
	cmd := SlashCommand{
		TeamID:      first(form, "team_id"),
		TeamDomain:  first(form, "team_domain"),
		ChannelID:   first(form, "channel_id"),
		ChannelName: first(form, "channel_name"),
		UserID:      first(form, "user_id"),
		UserName:    first(form, "user_name"),
		Command:     first(form, "command"),
		Text:        first(form, "text"),
		ResponseURL: first(form, "response_url"),
		TriggerID:   first(form, "trigger_id"),
	}
	b.recordCommand(cmd)

	handler, ok := b.commands[cmd.Command]
	if !ok {
		// "ephemeral" makes the response visible only to the invoking
		// user — quieter than blasting "command not configured" into
		// the channel.
		body := map[string]string{
			"response_type": "ephemeral",
			"text":          fmt.Sprintf("Command %q is not configured on this Argus deployment.", cmd.Command),
		}
		raw, _ := json.Marshal(body)
		return Outcome{Body: raw}, nil
	}
	text := handler(cmd)
	body := map[string]string{
		"response_type": "ephemeral",
		"text":          text,
	}
	raw, _ := json.Marshal(body)
	return Outcome{Body: raw}, nil
}

// RecentEvents returns a snapshot of the buffer for the UI to render.
// Newest first. The caller can't mutate the underlying slice — we
// copy on the way out.
func (b *EventBus) RecentEvents() []RecentEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]RecentEvent, len(b.recent))
	for i, e := range b.recent {
		out[len(b.recent)-1-i] = e // newest first
	}
	return out
}

func (b *EventBus) recordEvent(teamID string, raw json.RawMessage) {
	// Extract the per-event shape Slack puts inside `event`. We care
	// about subtype + channel + user + text for the row; the rest
	// stays in Raw for the debug expander.
	var inner struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
		User    string `json:"user"`
		Channel string `json:"channel"`
		Text    string `json:"text"`
	}
	_ = json.Unmarshal(raw, &inner)

	b.appendRecent(RecentEvent{
		ReceivedAt: time.Now(),
		Kind:       "event",
		Subtype:    inner.Type,
		TeamID:     teamID,
		UserID:     inner.User,
		Channel:    inner.Channel,
		Text:       truncRaw(inner.Text, 200),
		Raw:        capRaw(raw, 8192),
	})
}

func (b *EventBus) recordCommand(cmd SlashCommand) {
	raw, _ := json.Marshal(cmd)
	b.appendRecent(RecentEvent{
		ReceivedAt: time.Now(),
		Kind:       "slash_command",
		Subtype:    cmd.Command,
		TeamID:     cmd.TeamID,
		UserID:     cmd.UserID,
		Channel:    cmd.ChannelID,
		Text:       truncRaw(cmd.Text, 200),
		Raw:        capRaw(raw, 8192),
	})
}

func (b *EventBus) appendRecent(e RecentEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.recent = append(b.recent, e)
	if len(b.recent) > b.cap {
		b.recent = b.recent[len(b.recent)-b.cap:]
	}
}

func first(m map[string][]string, key string) string {
	v, ok := m[key]
	if !ok || len(v) == 0 {
		return ""
	}
	return v[0]
}

func truncRaw(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func capRaw(raw json.RawMessage, n int) json.RawMessage {
	if len(raw) <= n {
		return raw
	}
	// Trim mid-JSON would produce invalid JSON; we just store a
	// stub. The full payload is only useful for the inline debugger;
	// truncated rows still render correctly because the row fields
	// above carry the human-readable bits.
	return json.RawMessage(`{"truncated":true}`)
}
