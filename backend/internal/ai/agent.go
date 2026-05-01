package ai

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/argues/kube-watcher/internal/alerts"
)

// AgentEvent is a tracked observation the agent logs for pattern recognition.
type AgentEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // "alert", "resolution", "pattern", "investigation"
	Summary   string    `json:"summary"`
	AlertID   string    `json:"alertId,omitempty"`
	Namespace string    `json:"namespace,omitempty"`
	Severity  string    `json:"severity,omitempty"`
}

// ChatEntry is one message in a conversation thread.
type ChatEntry struct {
	Role      string    `json:"role"` // "user", "assistant", "system"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// AutoSummary is the agent's automatic investigation result for an alert.
type AutoSummary struct {
	AlertID    string    `json:"alertId"`
	Summary    string    `json:"summary"`
	Severity   string    `json:"severity"`
	Timestamp  time.Time `json:"timestamp"`
	Confidence float64   `json:"confidence"`
}

// Agent manages conversations, auto-investigation, and pattern tracking.
type Agent struct {
	client *DeepSeekClient
	logger *slog.Logger

	mu            sync.RWMutex
	conversations map[string][]ChatEntry  // keyed by alertID or "global"
	autoSummaries map[string]*AutoSummary // keyed by alertID
	eventLog      []AgentEvent            // rolling event log for pattern tracking
}

// NewAgent creates a new AI agent.
func NewAgent(client *DeepSeekClient, logger *slog.Logger) *Agent {
	return &Agent{
		client:        client,
		logger:        logger,
		conversations: make(map[string][]ChatEntry),
		autoSummaries: make(map[string]*AutoSummary),
	}
}

// AutoInvestigate is called when a new alert arrives. It builds context and
// sends the alert to DeepSeek for automatic analysis. Non-blocking — runs
// in a goroutine and stores the result.
func (a *Agent) AutoInvestigate(ctx context.Context, alert alerts.Alert, metrics *alerts.ClusterMetrics, relatedAlerts []alerts.Alert) {
	go func() {
		summary, err := a.investigate(ctx, alert, metrics, relatedAlerts)
		if err != nil {
			a.logger.WarnContext(ctx, "auto-investigation failed",
				slog.String("alertId", alert.ID),
				slog.String("error", err.Error()),
			)
			return
		}

		a.mu.Lock()
		a.autoSummaries[alert.ID] = summary
		a.eventLog = append(a.eventLog, AgentEvent{
			Timestamp: time.Now(),
			Type:      "investigation",
			Summary:   fmt.Sprintf("Auto-investigated %s: %s", alert.Name, truncate(summary.Summary, 120)),
			AlertID:   alert.ID,
			Namespace: alert.Namespace,
			Severity:  string(alert.Severity),
		})
		// Keep event log bounded.
		if len(a.eventLog) > 500 {
			a.eventLog = a.eventLog[len(a.eventLog)-500:]
		}
		a.mu.Unlock()

		a.logger.InfoContext(ctx, "auto-investigation complete",
			slog.String("alertId", alert.ID),
			slog.Float64("confidence", summary.Confidence),
		)
	}()
}

func (a *Agent) investigate(ctx context.Context, alert alerts.Alert, metrics *alerts.ClusterMetrics, related []alerts.Alert) (*AutoSummary, error) {
	systemPrompt := a.buildSystemPrompt(metrics)

	alertContext := formatAlertForAgent(alert)
	if len(related) > 0 {
		alertContext += "\n\nRelated alerts:\n"
		for _, r := range related {
			alertContext += fmt.Sprintf("- %s (%s) in %s\n", r.Name, r.Severity, r.Namespace)
		}
	}

	// Include recent patterns from event log.
	a.mu.RLock()
	recentPatterns := a.getRecentPatterns(alert.Namespace)
	a.mu.RUnlock()
	if recentPatterns != "" {
		alertContext += "\n\nRecent patterns observed:\n" + recentPatterns
	}

	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: fmt.Sprintf(
			"A new alert has fired. Investigate and provide a concise summary of the likely root cause, "+
				"impact, and recommended immediate action.\n\n%s", alertContext,
		)},
	}

	response, err := a.client.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	// Seed the conversation history with the auto-investigation.
	a.mu.Lock()
	a.conversations[alert.ID] = []ChatEntry{
		{Role: "system", Content: systemPrompt, Timestamp: time.Now()},
		{Role: "assistant", Content: response, Timestamp: time.Now()},
	}
	a.mu.Unlock()

	return &AutoSummary{
		AlertID:    alert.ID,
		Summary:    response,
		Severity:   string(alert.Severity),
		Timestamp:  time.Now(),
		Confidence: 0.8,
	}, nil
}

// SendMessage sends a user message in the context of an alert and returns the response.
func (a *Agent) SendMessage(ctx context.Context, alertID string, userMessage string, alert *alerts.Alert, metrics *alerts.ClusterMetrics) (string, error) {
	a.mu.Lock()
	history, ok := a.conversations[alertID]
	if !ok {
		// Start a new conversation.
		systemPrompt := a.buildSystemPrompt(metrics)
		history = []ChatEntry{
			{Role: "system", Content: systemPrompt, Timestamp: time.Now()},
		}
		if alert != nil {
			history = append(history, ChatEntry{
				Role:      "user",
				Content:   "Context for this conversation:\n" + formatAlertForAgent(*alert),
				Timestamp: time.Now(),
			})
		}
	}

	// Append user message.
	history = append(history, ChatEntry{
		Role:      "user",
		Content:   userMessage,
		Timestamp: time.Now(),
	})
	a.conversations[alertID] = history
	a.mu.Unlock()

	// Build messages for the API (keep last 20 messages to stay within token limits).
	messages := historyToMessages(history, 20)

	response, err := a.client.Chat(ctx, messages)
	if err != nil {
		return "", err
	}

	// Append assistant response.
	a.mu.Lock()
	a.conversations[alertID] = append(a.conversations[alertID], ChatEntry{
		Role:      "assistant",
		Content:   response,
		Timestamp: time.Now(),
	})
	a.mu.Unlock()

	return response, nil
}

// GetChatHistory returns the conversation history for an alert.
func (a *Agent) GetChatHistory(alertID string) []ChatEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.conversations[alertID]
}

// GetAutoSummary returns the auto-investigation summary for an alert.
func (a *Agent) GetAutoSummary(alertID string) *AutoSummary {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.autoSummaries[alertID]
}

// GetEventLog returns the agent's event log.
func (a *Agent) GetEventLog() []AgentEvent {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]AgentEvent, len(a.eventLog))
	copy(result, a.eventLog)
	return result
}

// TrackEvent logs an observation for pattern recognition.
func (a *Agent) TrackEvent(event AgentEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()
	event.Timestamp = time.Now()
	a.eventLog = append(a.eventLog, event)
	if len(a.eventLog) > 500 {
		a.eventLog = a.eventLog[len(a.eventLog)-500:]
	}
}

// --- internal helpers ---

func (a *Agent) buildSystemPrompt(metrics *alerts.ClusterMetrics) string {
	var sb strings.Builder
	sb.WriteString(`You are the KubeWatcher SRE AI Agent — an expert Kubernetes diagnostician embedded in a desktop SRE console. You have deep knowledge of Kubernetes internals, pod lifecycle, resource management, networking, and common failure modes.

Your responsibilities:
1. Investigate alerts automatically when they fire — identify root cause, assess blast radius, and recommend actions.
2. Answer follow-up questions from SREs about alerts, cluster state, and remediation.
3. Track patterns across alerts to identify systemic issues (e.g., recurring OOMs in the same namespace, cascading failures).
4. Provide ready-to-paste kubectl commands when suggesting actions.
5. Reference the DECISION_LOG when past decisions are relevant.

Be concise but thorough. Use structured output with clear sections. Severity-appropriate urgency: critical alerts get immediate actionable steps, warnings get monitoring guidance.
`)

	if metrics != nil {
		sb.WriteString(fmt.Sprintf(`
Current cluster state:
- Pod health: %.1f%% (%d/%d running, %d pending, %d failed)
- Error rate: %.2f%%
- Restart count: %d (top: %s)
- Warning events (30m): %d
- SLO status: %s
`,
			metrics.PodHealthPct, metrics.PodsRunning, metrics.PodsTotal,
			metrics.PodsPending, metrics.PodsFailed,
			metrics.ErrorRate, metrics.RestartCount, metrics.RestartTop,
			metrics.WarningEvents, metrics.SLOStatus,
		))
	}

	return sb.String()
}

func (a *Agent) getRecentPatterns(namespace string) string {
	var patterns []string
	cutoff := time.Now().Add(-2 * time.Hour)

	for _, event := range a.eventLog {
		if event.Timestamp.Before(cutoff) {
			continue
		}
		if namespace != "" && event.Namespace != "" && event.Namespace != namespace {
			continue
		}
		patterns = append(patterns, fmt.Sprintf("[%s] %s: %s",
			event.Timestamp.Format("15:04"), event.Type, event.Summary,
		))
	}

	if len(patterns) > 10 {
		patterns = patterns[len(patterns)-10:]
	}
	return strings.Join(patterns, "\n")
}

func formatAlertForAgent(a alerts.Alert) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Alert: %s\n", a.Name))
	sb.WriteString(fmt.Sprintf("Severity: %s\n", a.Severity))
	sb.WriteString(fmt.Sprintf("Namespace: %s\n", a.Namespace))
	sb.WriteString(fmt.Sprintf("Description: %s\n", a.Description))

	if a.PodName != "" {
		sb.WriteString(fmt.Sprintf("Pod: %s (phase: %s, restarts: %d)\n", a.PodName, a.PodPhase, a.RestartCount))
	}
	if a.MemoryLimit != "" {
		sb.WriteString(fmt.Sprintf("Memory: limit=%s request=%s\n", a.MemoryLimit, a.MemoryRequest))
	}
	if a.CPULimit != "" {
		sb.WriteString(fmt.Sprintf("CPU: limit=%s request=%s throttle=%.0f%%\n", a.CPULimit, a.CPURequest, a.CPUThrottle))
	}
	if a.NodeName != "" {
		sb.WriteString(fmt.Sprintf("Node: %s\n", a.NodeName))
	}
	if a.ImageTag != "" {
		sb.WriteString(fmt.Sprintf("Image: %s\n", a.ImageTag))
	}
	if !a.DeployTime.IsZero() {
		sb.WriteString(fmt.Sprintf("Last deploy: %s\n", a.DeployTime.Format(time.RFC3339)))
	}
	if len(a.Tags) > 0 {
		var tags []string
		for _, t := range a.Tags {
			tags = append(tags, t.Label)
		}
		sb.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(tags, ", ")))
	}
	return sb.String()
}

func historyToMessages(history []ChatEntry, maxMessages int) []Message {
	start := 0
	if len(history) > maxMessages {
		// Always keep the system message (first entry).
		start = len(history) - maxMessages
		if history[0].Role == "system" {
			messages := []Message{{Role: history[0].Role, Content: history[0].Content}}
			for _, entry := range history[start:] {
				if entry.Role == "system" {
					continue
				}
				messages = append(messages, Message{Role: entry.Role, Content: entry.Content})
			}
			return messages
		}
	}

	messages := make([]Message, 0, len(history)-start)
	for _, entry := range history[start:] {
		messages = append(messages, Message{Role: entry.Role, Content: entry.Content})
	}
	return messages
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
