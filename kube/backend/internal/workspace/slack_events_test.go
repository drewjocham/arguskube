package workspace

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

// (silentLogger is now defined in refresh_worker.go; tests share it.)

func signBody(t *testing.T, secret string, ts int64, body []byte) string {
	t.Helper()
	base := "v0:" + strconv.FormatInt(ts, 10) + ":" + string(body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(base))
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}

func TestEventBus_VerifyHappyPath(t *testing.T) {
	b := NewEventBus("secret", silentLogger{})
	body := []byte(`{"type":"event_callback"}`)
	ts := time.Now().Unix()
	sig := signBody(t, "secret", ts, body)
	if err := b.Verify(sig, strconv.FormatInt(ts, 10), body); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestEventBus_RejectsBadSignature(t *testing.T) {
	b := NewEventBus("secret", silentLogger{})
	body := []byte(`{}`)
	ts := time.Now().Unix()
	sig := signBody(t, "wrong-secret", ts, body)
	if err := b.Verify(sig, strconv.FormatInt(ts, 10), body); err == nil {
		t.Fatal("expected signature mismatch")
	}
}

func TestEventBus_RejectsReplay(t *testing.T) {
	b := NewEventBus("secret", silentLogger{})
	body := []byte(`{}`)
	// 10 minutes ago — beyond default 5m window.
	ts := time.Now().Add(-10 * time.Minute).Unix()
	sig := signBody(t, "secret", ts, body)
	err := b.Verify(sig, strconv.FormatInt(ts, 10), body)
	if err == nil || !strings.Contains(err.Error(), "out of window") {
		t.Fatalf("expected replay rejection, got %v", err)
	}
}

func TestEventBus_RejectsEmptySecret(t *testing.T) {
	b := NewEventBus("", silentLogger{})
	if err := b.Verify("v0=abc", "0", []byte("")); err == nil {
		t.Fatal("expected error when signing secret missing")
	}
}

func TestEventBus_HandleEvent_URLVerification(t *testing.T) {
	b := NewEventBus("s", silentLogger{})
	body := []byte(`{"type":"url_verification","challenge":"abc123"}`)
	out, err := b.HandleEvent(body)
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if string(out.Body) != "abc123" {
		t.Errorf("challenge not echoed: %q", out.Body)
	}
	if !strings.Contains(out.ContentType, "text/plain") {
		t.Errorf("want text/plain, got %q", out.ContentType)
	}
}

func TestEventBus_HandleEvent_RecordsCallback(t *testing.T) {
	b := NewEventBus("s", silentLogger{})
	body := []byte(`{
		"type":"event_callback",
		"team_id":"T1",
		"event":{"type":"app_mention","user":"U1","channel":"C1","text":"hey argus"}
	}`)
	if _, err := b.HandleEvent(body); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	rec := b.RecentEvents()
	if len(rec) != 1 {
		t.Fatalf("expected 1 recent, got %d", len(rec))
	}
	e := rec[0]
	if e.Kind != "event" || e.Subtype != "app_mention" || e.UserID != "U1" || e.Channel != "C1" {
		t.Fatalf("event projection wrong: %+v", e)
	}
}

func TestEventBus_RingBufferBound(t *testing.T) {
	b := NewEventBus("s", silentLogger{})
	body := []byte(`{"type":"event_callback","team_id":"T","event":{"type":"x"}}`)
	for i := 0; i < 60; i++ {
		_, _ = b.HandleEvent(body)
	}
	if got := len(b.RecentEvents()); got != 50 {
		t.Fatalf("buffer not capped at 50: got %d", got)
	}
}

func TestEventBus_SlashCommand_UnknownReturnsHint(t *testing.T) {
	b := NewEventBus("s", silentLogger{})
	form := url.Values{
		"command": []string{"/argus"},
		"text":    []string{"alerts list"},
		"user_id": []string{"U1"},
		"team_id": []string{"T1"},
	}
	out, err := b.HandleSlashCommand(form)
	if err != nil {
		t.Fatalf("HandleSlashCommand: %v", err)
	}
	var reply map[string]string
	if err := json.Unmarshal(out.Body, &reply); err != nil {
		t.Fatalf("reply not JSON: %v", err)
	}
	if !strings.Contains(reply["text"], "not configured") {
		t.Errorf("expected 'not configured' hint, got %q", reply["text"])
	}
	if reply["response_type"] != "ephemeral" {
		t.Errorf("unknown commands should be ephemeral, got %q", reply["response_type"])
	}
}

func TestEventBus_SlashCommand_RoutesToHandler(t *testing.T) {
	b := NewEventBus("s", silentLogger{})
	b.RegisterCommand("/argus", func(c SlashCommand) string {
		return "pong " + c.UserName
	})
	form := url.Values{
		"command":   []string{"/argus"},
		"text":      []string{"ping"},
		"user_name": []string{"alice"},
	}
	out, _ := b.HandleSlashCommand(form)
	var reply map[string]string
	_ = json.Unmarshal(out.Body, &reply)
	if reply["text"] != "pong alice" {
		t.Fatalf("handler not invoked: %q", reply["text"])
	}
}

func TestEventBus_RegisterCommand_PanicsOnDuplicate(t *testing.T) {
	b := NewEventBus("s", silentLogger{})
	b.RegisterCommand("/argus", func(SlashCommand) string { return "" })
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate command")
		}
	}()
	b.RegisterCommand("/argus", func(SlashCommand) string { return "" })
}

func TestEventBus_RegisterCommand_PanicsOnUnslashed(t *testing.T) {
	b := NewEventBus("s", silentLogger{})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on missing slash prefix")
		}
	}()
	b.RegisterCommand("argus", func(SlashCommand) string { return "" })
}

func TestEventBus_RawCapPreservesShortPayloads(t *testing.T) {
	b := NewEventBus("s", silentLogger{})
	short := []byte(`{"type":"event_callback","team_id":"T","event":{"type":"x"}}`)
	_, _ = b.HandleEvent(short)
	rec := b.RecentEvents()[0]
	if len(rec.Raw) >= len(short) && string(rec.Raw) == `{"truncated":true}` {
		t.Fatalf("short payload should not be truncated")
	}
}
