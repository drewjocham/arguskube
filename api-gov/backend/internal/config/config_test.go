package config

import (
	"os"
	"testing"
)

func TestNewDefaults(t *testing.T) {
	cfg := New()

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "port default", got: cfg.Server.Port, want: "8080"},
		{name: "database default", got: cfg.Database.URL, want: "postgres://api-gov:api-gov@localhost:5432/api-gov?sslmode=disable"},
		{name: "agent service default", got: cfg.Agent.ServiceURL, want: "http://localhost:8001"},
		{name: "llm model default", got: cfg.LLM.Model, want: "gemini-1.5-pro"},
		{name: "embedding model default", got: cfg.LLM.EmbeddingModel, want: "text-embedding-004"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("default %s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestNewFromEnv(t *testing.T) {
	os.Setenv("PORT", "9090")
	os.Setenv("DATABASE_URL", "postgres://custom:pass@host:5432/db")
	os.Setenv("AGENT_SERVICE_URL", "http://agent:8001")
	os.Setenv("LLM_MODEL", "gpt-4")
	os.Setenv("DRIFT_THRESHOLD", "0.9")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("AGENT_SERVICE_URL")
		os.Unsetenv("LLM_MODEL")
		os.Unsetenv("DRIFT_THRESHOLD")
	}()

	cfg := New()

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "port from env", got: cfg.Server.Port, want: "9090"},
		{name: "database from env", got: cfg.Database.URL, want: "postgres://custom:pass@host:5432/db"},
		{name: "agent url from env", got: cfg.Agent.ServiceURL, want: "http://agent:8001"},
		{name: "llm model from env", got: cfg.LLM.Model, want: "gpt-4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}

	if cfg.LLM.DriftThreshold != 0.9 {
		t.Errorf("drift threshold = %f, want 0.9", cfg.LLM.DriftThreshold)
	}
}
