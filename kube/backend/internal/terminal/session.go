package terminal

import (
	"fmt"
	"log/slog"
	"sync"
)

type Domain string

const (
	DomainDefault Domain = "default"
	DomainK8s     Domain = "k8s"
	DomainKafka   Domain = "kafka"
	DomainCloud   Domain = "cloud"
)

type Session struct {
	ID       string
	Domain   Domain
	Label    string
	Terminal *Terminal
}

type SessionInfo struct {
	ID     string `json:"id"`
	Domain string `json:"domain"`
	Label  string `json:"label"`
	Alive  bool   `json:"alive"`
}

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	logger   *slog.Logger
}

func NewSessionManager(logger *slog.Logger) *SessionManager {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &SessionManager{
		sessions: make(map[string]*Session),
		logger:   logger,
	}
}

func (sm *SessionManager) NewSession(id string, domain Domain, label string, rows, cols uint16, extraEnv []string) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.sessions[id]; exists {
		return nil, fmt.Errorf("session %q already exists", id)
	}

	term := New(sm.logger.With("session", id, "domain", string(domain)))

	env := []string{
		"ARGUS_SESSION_ID=" + id,
		"ARGUS_SESSION_DOMAIN=" + string(domain),
	}
	env = append(env, kwDomainEnv(domain)...)
	env = append(env, extraEnv...)

	if err := term.StartWithEnv("", rows, cols, env); err != nil {
		return nil, err
	}

	sess := &Session{
		ID:       id,
		Domain:   domain,
		Label:    label,
		Terminal: term,
	}
	sm.sessions[id] = sess

	sm.logger.Info("session started",
		slog.String("session_id", id),
		slog.String("domain", string(domain)),
		slog.String("label", label),
	)

	return sess, nil
}

func (sm *SessionManager) GetSession(id string) *Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sessions[id]
}

func (sm *SessionManager) CloseSession(id string) error {
	sm.mu.Lock()
	sess, ok := sm.sessions[id]
	if !ok {
		sm.mu.Unlock()
		return fmt.Errorf("session %q not found", id)
	}
	delete(sm.sessions, id)
	sm.mu.Unlock()

	err := sess.Terminal.Close()
	sm.logger.Info("session closed",
		slog.String("session_id", id),
		slog.String("domain", string(sess.Domain)),
	)
	return err
}

func (sm *SessionManager) CloseAll() {
	sm.mu.Lock()
	ids := make([]string, 0, len(sm.sessions))
	for id := range sm.sessions {
		ids = append(ids, id)
	}
	sm.mu.Unlock()

	for _, id := range ids {
		_ = sm.CloseSession(id)
	}
}

func (sm *SessionManager) ListSessions() []SessionInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	infos := make([]SessionInfo, 0, len(sm.sessions))
	for id, sess := range sm.sessions {
		infos = append(infos, SessionInfo{
			ID:     id,
			Domain: string(sess.Domain),
			Label:  sess.Label,
			Alive:  sess.Terminal.IsRunning(),
		})
	}
	return infos
}
