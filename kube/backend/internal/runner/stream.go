package runner

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/argues/argus/internal/saasapi"
)

// EventStream is a fan-out broadcaster for SSE events. The runner
// emits events as regions progress; the HTTP handler attaches
// listeners that receive every event.
type EventStream struct {
	runID    string
	mu       sync.RWMutex
	channels map[chan saasapi.RunnerEvent]struct{}
	closed   bool
	logger   *slog.Logger
}

// NewEventStream creates a new event stream for a run.
func NewEventStream(runID string) *EventStream {
	return &EventStream{
		runID:    runID,
		channels: make(map[chan saasapi.RunnerEvent]struct{}),
		logger:   slog.Default().With("component", "eventstream", "runId", runID),
	}
}

// Subscribe returns a channel that receives all subsequent events.
// The caller MUST call Unsubscribe when done to avoid goroutine leaks.
// After Close(), Subscribe returns an already-closed channel — the
// subscriber will see it as closed immediately.
func (s *EventStream) Subscribe() chan saasapi.RunnerEvent {
	ch := make(chan saasapi.RunnerEvent, 64)
	s.mu.Lock()
	if s.channels == nil {
		// Stream is closed. Return a closed channel so the subscriber
		// sees immediate close rather than a channel that blocks forever.
		s.mu.Unlock()
		close(ch)
		return ch
	}
	s.channels[ch] = struct{}{}
	s.mu.Unlock()
	return ch
}

// Unsubscribe removes a channel from the fan-out. Idempotent — safe
// to call multiple times for the same channel.
func (s *EventStream) Unsubscribe(ch chan saasapi.RunnerEvent) {
	s.mu.Lock()
	if _, ok := s.channels[ch]; !ok {
		s.mu.Unlock()
		return
	}
	delete(s.channels, ch)
	s.mu.Unlock()
	close(ch)
}

// Emit sends an event to all active subscribers. Non-blocking: if a
// subscriber's buffer is full, the event is dropped for that subscriber.
//
// Uses a full Lock (not RLock) to prevent a race with Close/Unsubscribe:
// those methods mutate the channels map AND close channels under Lock.
// An RLock-protected iteration could hold a reference to a channel that
// is then closed by a concurrent Close/Unsubscribe, causing a panic on
// send to closed channel.
func (s *EventStream) Emit(evt saasapi.RunnerEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	for ch := range s.channels {
		select {
		case ch <- evt:
		default:
			s.logger.Warn("dropping event for slow subscriber",
				"type", evt.Type, "region", evt.Region)
		}
	}
}

// Close shuts down the stream, closing all subscriber channels.
func (s *EventStream) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	for ch := range s.channels {
		close(ch)
	}
	s.channels = nil
}

// SSEEvent formats an event as an SSE data frame.
func SSEEvent(evt saasapi.RunnerEvent) (string, error) {
	data, err := json.Marshal(evt)
	if err != nil {
		return "", fmt.Errorf("marshal event: %w", err)
	}
	return fmt.Sprintf("event: %s\ndata: %s\n\n", evt.Type, string(data)), nil
}
