package pkg

import (
	"io"
	"net/http"

	"github.com/argues/argus/internal/workspace"
)

// workspace_slack_events.go — HTTP entry points for the Slack Events
// API + slash commands. SaaS-only: the routes only register when
// ARGUS_SLACK_SIGNING_SECRET is set + the EventBus is wired in main.go.
//
// Both handlers follow the same shape: read the body once (the HMAC
// covers the raw bytes, so any rewinding would invalidate the
// signature), verify, then dispatch to the bus.

// slackEventsMaxBody caps inbound payloads. Slack's own docs say
// events stay under 100 KB in practice; 1 MB is room for a chatty
// channel without inviting a DoS.
const slackEventsMaxBody = 1 << 20

func (a *App) handleSlackEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, ok := readSlackBody(w, r)
	if !ok {
		return
	}
	if err := a.slackEvents.Verify(
		r.Header.Get("X-Slack-Signature"),
		r.Header.Get("X-Slack-Request-Timestamp"),
		body,
	); err != nil {
		// Generic 401 — the bus's error messages are deliberately
		// bland so we just echo "unauthorized" here.
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	out, err := a.slackEvents.HandleEvent(body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	writeSlackOutcome(w, out)
}

func (a *App) handleSlackCommands(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, ok := readSlackBody(w, r)
	if !ok {
		return
	}
	if err := a.slackEvents.Verify(
		r.Header.Get("X-Slack-Signature"),
		r.Header.Get("X-Slack-Request-Timestamp"),
		body,
	); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	// Slash commands are form-encoded; ParseForm consumes r.Body so
	// we re-parse via PostForm by setting body on the request first.
	// Simpler: parse the bytes ourselves.
	values, err := parseFormBody(body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	out, err := a.slackEvents.HandleSlashCommand(values)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	writeSlackOutcome(w, out)
}

func readSlackBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, slackEventsMaxBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "body too large or unreadable", http.StatusBadRequest)
		return nil, false
	}
	return body, true
}

func writeSlackOutcome(w http.ResponseWriter, out workspace.Outcome) {
	ct := out.ContentType
	if ct == "" {
		ct = "application/json"
	}
	w.Header().Set("Content-Type", ct)
	status := out.StatusCode
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(out.Body)
}

// parseFormBody is the form-encoded parse we use instead of
// r.ParseForm so we can verify the signature against the EXACT bytes
// Slack sent — ParseForm would have already consumed and rewritten
// them. Standard-library url.ParseQuery is identical to the
// non-multipart form decode.
func parseFormBody(body []byte) (map[string][]string, error) {
	// net/url's ParseQuery returns the same shape we need (Values).
	v, err := parseQueryBytes(body)
	if err != nil {
		return nil, err
	}
	return v, nil
}
