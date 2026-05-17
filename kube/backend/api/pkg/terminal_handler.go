package pkg

import (
	"log/slog"

	"github.com/argues/argus/internal/terminal"
)

// SessionService is the subset of *terminal.SessionManager the handler needs.
// Defining it on the consumer side keeps the handler unit-testable with a fake.
type SessionService interface {
	NewSession(id string, domain terminal.Domain, label string, rows, cols uint16, extraEnv []string) (*terminal.Session, error)
	CloseSession(id string) error
	ListSessions() []terminal.SessionInfo
}

type TerminalHandler struct {
	sessions SessionService
	logger   *slog.Logger
}

func NewTerminalHandler(app *App) *TerminalHandler {
	return &TerminalHandler{
		sessions: app.sessions,
		logger:   app.logger,
	}
}

func (h *TerminalHandler) StartTerminalSession(sessionID, domain, label string, rows, cols int) error {
	h.logger.Info("starting terminal session",
		"session_id", sessionID, "domain", domain, "label", label)
	_, err := h.sessions.NewSession(sessionID, terminal.Domain(domain), label, uint16(rows), uint16(cols), nil)
	return err
}

func (h *TerminalHandler) CloseTerminalSession(sessionID string) error {
	return h.sessions.CloseSession(sessionID)
}

func (h *TerminalHandler) ListTerminalSessions() []terminal.SessionInfo {
	return h.sessions.ListSessions()
}
