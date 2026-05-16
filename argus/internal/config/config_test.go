package config

import (
	"context"
	"testing"
)

func TestNewDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("SAAS_TOKEN", "")
	t.Setenv("SAAS_SERVER_URL", "")

	cfg, err := New(context.Background())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if cfg.ServerPort != "8080" {
		t.Errorf("default ServerPort = %q, want 8080", cfg.ServerPort)
	}
	if cfg.SaaSToken != "" {
		t.Errorf("default SaaSToken should be empty, got %q", cfg.SaaSToken)
	}
	if cfg.SaaSServerURL != "ws://localhost:8080/tunnel" {
		t.Errorf("default SaaSServerURL = %q, want ws://localhost:8080/tunnel", cfg.SaaSServerURL)
	}
}

func TestNewHonorsExplicitPort(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("SAAS_TOKEN", "")
	t.Setenv("SAAS_SERVER_URL", "")

	cfg, err := New(context.Background())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if cfg.ServerPort != "9090" {
		t.Errorf("ServerPort = %q, want 9090", cfg.ServerPort)
	}
}

func TestNewPropagatesSaaSToken(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("SAAS_TOKEN", "secret-abc")
	t.Setenv("SAAS_SERVER_URL", "")

	cfg, err := New(context.Background())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if cfg.SaaSToken != "secret-abc" {
		t.Errorf("SaaSToken = %q, want secret-abc", cfg.SaaSToken)
	}
}

func TestNewHonorsExplicitServerURL(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("SAAS_TOKEN", "")
	t.Setenv("SAAS_SERVER_URL", "wss://prod.argus.example/tunnel")

	cfg, err := New(context.Background())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if cfg.SaaSServerURL != "wss://prod.argus.example/tunnel" {
		t.Errorf("SaaSServerURL = %q", cfg.SaaSServerURL)
	}
}
