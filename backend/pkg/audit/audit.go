// Package audit provides audit logging for Kubernetes API operations.
package audit

import "log/slog"

// Logger records audit events for Kubernetes API calls.
type Logger interface {
	Log(action, resource, namespace, name string)
}

// SlogLogger implements Logger using a *slog.Logger.
type SlogLogger struct {
	logger *slog.Logger
}

// NewSlogLogger wraps an slog.Logger as an audit Logger.
func NewSlogLogger(l *slog.Logger) *SlogLogger {
	return &SlogLogger{logger: l}
}

// Log records a single audit event.
func (s *SlogLogger) Log(action, resource, namespace, name string) {
	s.logger.Info("k8s audit",
		slog.String("action", action),
		slog.String("resource", resource),
		slog.String("namespace", namespace),
		slog.String("name", name),
	)
}
