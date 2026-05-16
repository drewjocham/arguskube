package pkg

import (
	"github.com/argues/argus/internal/terminal"
)

type TerminalHandler struct {
	app *App
}

func NewTerminalHandler(app *App) *TerminalHandler {
	return &TerminalHandler{app: app}
}

func (h *TerminalHandler) StartTerminalSession(sessionID, domain, label string, rows, cols int) error {
	h.app.logger.Info("starting terminal session",
		"session_id", sessionID, "domain", domain, "label", label)
	_, err := h.app.sessions.NewSession(sessionID, terminal.Domain(domain), label, uint16(rows), uint16(cols), nil)
	return err
}

func (h *TerminalHandler) CloseTerminalSession(sessionID string) error {
	return h.app.sessions.CloseSession(sessionID)
}

func (h *TerminalHandler) ListTerminalSessions() []terminal.SessionInfo {
	return h.app.sessions.ListSessions()
}
