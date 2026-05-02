package anomaly

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"sync"
)

// Settings holds the user's anomaly detection configuration.
type Settings struct {
	Sensitivity    int    `json:"sensitivity"`     // 0-100 slider
	BaselineWindow int    `json:"baselineWindow"`  // days
	MetricType     string `json:"metricType"`      // "cpu", "mem"
	Algorithm      string `json:"algorithm"`       // "smart", "fixed"
	Threshold      int    `json:"threshold"`       // 50-99 confidence %
	TargetScope    string `json:"targetScope"`     // namespace name or "all"
}

// Rule represents a single anomaly detection rule.
type Rule struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	Severity string `json:"severity"` // "critical", "warning", "info"
}

// SettingsStore persists anomaly settings and rules to SQLite.
type SettingsStore struct {
	db     *sql.DB
	logger *slog.Logger
	mu     sync.RWMutex
}

// NewSettingsStore creates a store and ensures the schema exists.
func NewSettingsStore(db *sql.DB, logger *slog.Logger) (*SettingsStore, error) {
	s := &SettingsStore{db: db, logger: logger}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *SettingsStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS anomaly_settings (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS anomaly_rules (
			id       TEXT PRIMARY KEY,
			name     TEXT NOT NULL,
			enabled  INTEGER NOT NULL DEFAULT 1,
			severity TEXT NOT NULL DEFAULT 'warning'
		);
	`)
	if err != nil {
		return err
	}

	// Seed default rules if table is empty.
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM anomaly_rules").Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		defaults := []Rule{
			{ID: "rule-1", Name: "Sudden Memory Spike", Enabled: true, Severity: "critical"},
			{ID: "rule-2", Name: "Anomalous Network Traffic", Enabled: true, Severity: "warning"},
			{ID: "rule-3", Name: "CrashLoop Frequency Deviation", Enabled: false, Severity: "warning"},
			{ID: "rule-4", Name: "High Error Rate on Ingress", Enabled: true, Severity: "critical"},
		}
		for _, r := range defaults {
			_, err := s.db.Exec(
				"INSERT INTO anomaly_rules (id, name, enabled, severity) VALUES (?, ?, ?, ?)",
				r.ID, r.Name, boolToInt(r.Enabled), r.Severity,
			)
			if err != nil {
				return err
			}
		}
		s.logger.Info("anomaly: seeded default rules", slog.Int("count", len(defaults)))
	}

	return nil
}

// GetSettings returns the saved anomaly settings, or defaults if none saved.
func (s *SettingsStore) GetSettings() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var raw string
	err := s.db.QueryRow("SELECT value FROM anomaly_settings WHERE key = 'config'").Scan(&raw)
	if err != nil {
		// Return defaults.
		return Settings{
			Sensitivity:    30,
			BaselineWindow: 7,
			MetricType:     "cpu",
			Algorithm:      "smart",
			Threshold:      85,
			TargetScope:    "all",
		}
	}
	var settings Settings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		s.logger.Warn("anomaly: bad settings JSON", slog.String("error", err.Error()))
		return Settings{Sensitivity: 30, BaselineWindow: 7, MetricType: "cpu", Algorithm: "smart", Threshold: 85, TargetScope: "all"}
	}
	return settings
}

// SaveSettings persists anomaly settings.
func (s *SettingsStore) SaveSettings(settings Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		"INSERT OR REPLACE INTO anomaly_settings (key, value) VALUES ('config', ?)",
		string(raw),
	)
	if err != nil {
		return err
	}
	s.logger.Info("anomaly: settings saved",
		slog.Int("sensitivity", settings.Sensitivity),
		slog.Int("threshold", settings.Threshold),
	)
	return nil
}

// ListRules returns all anomaly detection rules.
func (s *SettingsStore) ListRules() []Rule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT id, name, enabled, severity FROM anomaly_rules ORDER BY id")
	if err != nil {
		s.logger.Error("anomaly: list rules failed", slog.String("error", err.Error()))
		return nil
	}
	defer rows.Close()

	var rules []Rule
	for rows.Next() {
		var r Rule
		var enabled int
		if err := rows.Scan(&r.ID, &r.Name, &enabled, &r.Severity); err != nil {
			s.logger.Warn("anomaly: scan rule failed", slog.String("error", err.Error()))
			continue
		}
		r.Enabled = enabled != 0
		rules = append(rules, r)
	}
	return rules
}

// SaveRule creates or updates an anomaly rule.
func (s *SettingsStore) SaveRule(rule Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO anomaly_rules (id, name, enabled, severity) VALUES (?, ?, ?, ?)",
		rule.ID, rule.Name, boolToInt(rule.Enabled), rule.Severity,
	)
	return err
}

// ToggleRule flips the enabled state of a rule by ID and returns the new state.
func (s *SettingsStore) ToggleRule(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("UPDATE anomaly_rules SET enabled = 1 - enabled WHERE id = ?", id)
	if err != nil {
		return false, err
	}
	var enabled int
	if err := s.db.QueryRow("SELECT enabled FROM anomaly_rules WHERE id = ?", id).Scan(&enabled); err != nil {
		return false, err
	}
	return enabled != 0, nil
}

// DeleteRule removes a rule by ID.
func (s *SettingsStore) DeleteRule(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM anomaly_rules WHERE id = ?", id)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
