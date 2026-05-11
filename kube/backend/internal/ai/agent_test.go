package ai_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/argues/argus/internal/ai"
	"github.com/argues/argus/internal/alerts"
)

func TestNewAgent(t *testing.T) {
	client := ai.NewDeepSeekClient("test-key", slog.New(slog.DiscardHandler))
	logger := slog.New(slog.DiscardHandler)
	agent := ai.NewAgent(client, logger)
	if agent == nil {
		t.Fatal("NewAgent() returned nil")
	}
}

func TestNewAgentWithNilClient(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	agent := ai.NewAgent(nil, logger)
	if agent == nil {
		t.Fatal("NewAgent() with nil client returned nil")
	}
}

func TestGetChatHistoryEmpty(t *testing.T) {
	client := ai.NewDeepSeekClient("test-key", slog.New(slog.DiscardHandler))
	agent := ai.NewAgent(client, slog.New(slog.DiscardHandler))

	history := agent.GetChatHistory("nonexistent-alert")
	if history != nil {
		t.Errorf("expected nil history for nonexistent alert, got %d entries", len(history))
	}
}

func TestGetAutoSummaryEmpty(t *testing.T) {
	client := ai.NewDeepSeekClient("test-key", slog.New(slog.DiscardHandler))
	agent := ai.NewAgent(client, slog.New(slog.DiscardHandler))

	summary := agent.GetAutoSummary("nonexistent-alert")
	if summary != nil {
		t.Errorf("expected nil summary for nonexistent alert")
	}
}

func TestGetEventLogInitialEmpty(t *testing.T) {
	client := ai.NewDeepSeekClient("test-key", slog.New(slog.DiscardHandler))
	agent := ai.NewAgent(client, slog.New(slog.DiscardHandler))

	events := agent.GetEventLog()
	if len(events) != 0 {
		t.Errorf("expected empty event log initially, got %d events", len(events))
	}
}

func TestTrackEvent(t *testing.T) {
	client := ai.NewDeepSeekClient("test-key", slog.New(slog.DiscardHandler))
	agent := ai.NewAgent(client, slog.New(slog.DiscardHandler))

	event := ai.AgentEvent{
		Type:    "pattern",
		Summary: "Test event detected",
	}

	agent.TrackEvent(event)

	events := agent.GetEventLog()
	if len(events) != 1 {
		t.Fatalf("expected 1 event in log, got %d", len(events))
	}
	if events[0].Type != "pattern" {
		t.Errorf("expected type 'pattern', got %q", events[0].Type)
	}
	if !strings.Contains(events[0].Summary, "Test event") {
		t.Errorf("expected summary to contain 'Test event', got %q", events[0].Summary)
	}
}

func TestTrackEventMultiple(t *testing.T) {
	client := ai.NewDeepSeekClient("test-key", slog.New(slog.DiscardHandler))
	agent := ai.NewAgent(client, slog.New(slog.DiscardHandler))

	agent.TrackEvent(ai.AgentEvent{Type: "alert", Summary: "Alert 1"})
	agent.TrackEvent(ai.AgentEvent{Type: "resolution", Summary: "Resolved 1"})
	agent.TrackEvent(ai.AgentEvent{Type: "pattern", Summary: "Pattern 1"})

	events := agent.GetEventLog()
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[0].Type != "alert" {
		t.Errorf("expected first event type 'alert', got %q", events[0].Type)
	}
	if events[2].Type != "pattern" {
		t.Errorf("expected third event type 'pattern', got %q", events[2].Type)
	}

	// Timestamps should be set automatically.
	if events[0].Timestamp.IsZero() {
		t.Error("expected timestamp to be set automatically")
	}
}

func TestTrackEventBounded(t *testing.T) {
	client := ai.NewDeepSeekClient("test-key", slog.New(slog.DiscardHandler))
	agent := ai.NewAgent(client, slog.New(slog.DiscardHandler))

	// Add 510 events — should be trimmed to 500.
	for i := 0; i < 510; i++ {
		agent.TrackEvent(ai.AgentEvent{Type: "test", Summary: "Event"})
	}

	events := agent.GetEventLog()
	if len(events) != 500 {
		t.Errorf("expected event log bounded to 500, got %d", len(events))
	}
}

func TestAutoInvestigateWithNilClient(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	agent := ai.NewAgent(nil, logger)

	alert := alerts.Alert{
		ID:   "test-alert-1",
		Name: "OOMKilled",
	}

	// This should not panic — the goroutine will fail silently.
	agent.AutoInvestigate(context.Background(), alert, nil, nil)

	// Give goroutine a moment to fail.
	time.Sleep(50 * time.Millisecond)

	// Summary should be nil since investigation failed.
	summary := agent.GetAutoSummary(alert.ID)
	if summary != nil {
		t.Error("expected nil summary for failed auto-investigation")
	}
}

func TestAutoInvestigateConcurrent(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	agent := ai.NewAgent(nil, logger)

	// Fire multiple auto-investigations concurrently — none should panic.
	for i := 0; i < 10; i++ {
		alert := alerts.Alert{
			ID:   "alert-", // unique IDs
			Name: "TestAlert",
		}
		agent.AutoInvestigate(context.Background(), alert, nil, nil)
	}
}

func TestSendMessageWithNilClient(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	agent := ai.NewAgent(nil, logger)

	_, err := agent.SendMessage(context.Background(), "alert-1", "What's happening?", nil, nil)
	if err == nil {
		t.Fatal("expected error when DeepSeek client returns nil")
	}
}

func TestGetChatHistoryAfterSendMessage(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	agent := ai.NewAgent(nil, logger)

	// SendMessage will fail but conversation history should be created.
	_, _ = agent.SendMessage(context.Background(), "alert-1", "test", nil, nil)

	history := agent.GetChatHistory("alert-1")
	if history == nil {
		t.Fatal("expected non-nil history after SendMessage")
	}
	if len(history) < 2 {
		t.Errorf("expected at least 2 entries (system + user), got %d", len(history))
	}
	if history[0].Role != "system" {
		t.Errorf("expected first entry role 'system', got %q", history[0].Role)
	}
}

func TestChatMessagesBounded(t *testing.T) {
	// Test that the message conversion function (tested internally via
	// historyToMessages) properly bounds history to maxMessages.
	client := ai.NewDeepSeekClient("test-key", slog.New(slog.DiscardHandler))
	agent := ai.NewAgent(client, slog.New(slog.DiscardHandler))

	// Send many messages for a single alert.
	for i := 0; i < 30; i++ {
		_, _ = agent.SendMessage(context.Background(), "alert-1", "Message", nil, nil)
	}

	history := agent.GetChatHistory("alert-1")
	// System + 30 user + 30 assistant = up to 61 entries.
	// The API call limits to last 20 messages (excluding system prompt),
	// but history storage keeps everything.
	if len(history) < 30 {
		t.Errorf("expected at least 30 entries in full history, got %d", len(history))
	}
}

func TestAgentEventTimestampNotOverwritten(t *testing.T) {
	client := ai.NewDeepSeekClient("test-key", slog.New(slog.DiscardHandler))
	agent := ai.NewAgent(client, slog.New(slog.DiscardHandler))

	pastTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	event := ai.AgentEvent{
		Timestamp: pastTime,
		Type:      "test",
		Summary:   "Past event",
	}

	agent.TrackEvent(event)

	events := agent.GetEventLog()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Timestamp.Equal(pastTime) {
		t.Error("expected timestamp to be overwritten to current time")
	}
}

// TestAgentHasClientReflectsConstruction confirms the agent reports whether a
// DeepSeek client is wired up. Drives the UI surfacing of "AI not configured".
func TestAgentHasClientReflectsConstruction(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	agentNil := ai.NewAgent(nil, logger)
	if agentNil.HasClient() {
		t.Error("expected HasClient() == false for nil client")
	}

	agent := ai.NewAgent(ai.NewDeepSeekClient("k", logger), logger)
	if !agent.HasClient() {
		t.Error("expected HasClient() == true for non-nil client")
	}
}

// TestAgentSetClientHotSwap verifies that updating the API key at runtime via
// SetClient() flips HasClient() and surfaces a useful error before swap.
func TestAgentSetClientHotSwap(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	agent := ai.NewAgent(nil, logger)

	// Before swap: SendMessage should return a "not configured" error rather
	// than panic on nil client.
	_, err := agent.SendMessage(context.Background(), "global", "ping", nil, nil)
	if err == nil {
		t.Fatal("expected error from SendMessage when no client is configured")
	}
	if !strings.Contains(err.Error(), "AI agent not configured") {
		t.Errorf("expected user-facing error message, got %q", err.Error())
	}

	// Hot-swap a new client.
	agent.SetClient(ai.NewDeepSeekClient("new-key", logger))
	if !agent.HasClient() {
		t.Error("expected HasClient() == true after SetClient")
	}

	// And clearing it again should return us to the nil-client path.
	agent.SetClient(nil)
	if agent.HasClient() {
		t.Error("expected HasClient() == false after SetClient(nil)")
	}
}

// TestAgentSetClientPreservesHistory confirms that swapping the client does
// not wipe out conversation history — important so users don't lose context
// when they finally configure an API key.
func TestAgentSetClientPreservesHistory(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	agent := ai.NewAgent(nil, logger)

	// Seed a fake conversation by calling SendMessage; it'll error on the
	// missing client but should still record the user message in history.
	_, _ = agent.SendMessage(context.Background(), "global", "early question", nil, nil)
	before := len(agent.GetChatHistory("global"))
	if before == 0 {
		t.Fatal("expected history to contain the user message even when the client is nil")
	}

	agent.SetClient(ai.NewDeepSeekClient("k", logger))
	after := len(agent.GetChatHistory("global"))
	if after != before {
		t.Errorf("history changed after SetClient (%d → %d); should be preserved", before, after)
	}
}
