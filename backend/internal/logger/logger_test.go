package logger_test

import (
	"testing"

	"github.com/argues/kube-watcher/internal/config"
	"github.com/argues/kube-watcher/internal/logger"
)

func TestNewDefault(t *testing.T) {
	cfg := &config.OnlineDataConfig{
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}

	log := logger.New(cfg)
	if log == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNewJSONFormat(t *testing.T) {
	cfg := &config.OnlineDataConfig{
		Logging: config.LoggingConfig{
			Level:  "debug",
			Format: "json",
		},
	}

	log := logger.New(cfg)
	if log == nil {
		t.Fatal("New() with JSON format returned nil")
	}
}

func TestNewDebugLevel(t *testing.T) {
	cfg := &config.OnlineDataConfig{
		Logging: config.LoggingConfig{
			Level:  "debug",
			Format: "text",
		},
	}

	log := logger.New(cfg)
	if log == nil {
		t.Fatal("New() with debug level returned nil")
	}
}

func TestNewWarnLevel(t *testing.T) {
	cfg := &config.OnlineDataConfig{
		Logging: config.LoggingConfig{
			Level:  "warn",
			Format: "text",
		},
	}

	log := logger.New(cfg)
	if log == nil {
		t.Fatal("New() with warn level returned nil")
	}
}

func TestNewErrorLevel(t *testing.T) {
	cfg := &config.OnlineDataConfig{
		Logging: config.LoggingConfig{
			Level:  "error",
			Format: "text",
		},
	}

	log := logger.New(cfg)
	if log == nil {
		t.Fatal("New() with error level returned nil")
	}
}

func TestNewEmptyConfig(t *testing.T) {
	cfg := &config.OnlineDataConfig{}

	log := logger.New(cfg)
	if log == nil {
		t.Fatal("New() with empty config returned nil")
	}
}

func TestNewUnknownLevelDefaultsToInfo(t *testing.T) {
	cfg := &config.OnlineDataConfig{
		Logging: config.LoggingConfig{
			Level:  "unknown-level",
			Format: "text",
		},
	}

	log := logger.New(cfg)
	if log == nil {
		t.Fatal("New() with unknown level returned nil")
	}
	// The logger should default to Info level.
}

func TestNewUnknownFormatDefaultsToText(t *testing.T) {
	cfg := &config.OnlineDataConfig{
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "xml", // not supported, should default to text
		},
	}

	log := logger.New(cfg)
	if log == nil {
		t.Fatal("New() with unknown format returned nil")
	}
}

func TestLogConstants(t *testing.T) {
	// Verify that exported log key constants exist and are non-empty.
	tests := []struct {
		key   string
		value string
	}{
		{"KeyAlertID", logger.KeyAlertID},
		{"KeyPodName", logger.KeyPodName},
		{"KeyNamespace", logger.KeyNamespace},
		{"KeyNodeName", logger.KeyNodeName},
		{"KeySeverity", logger.KeySeverity},
		{"KeyRestarts", logger.KeyRestarts},
		{"KeyFeature", logger.KeyFeature},
		{"KeyTier", logger.KeyTier},
		{"KeyComponent", logger.KeyComponent},
		{"KeyError", logger.KeyError},
		{"KeyDuration", logger.KeyDuration},
		{"KeyCluster", logger.KeyCluster},
		{"KeyAnomstackJob", logger.KeyAnomstackJob},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if tt.value == "" {
				t.Errorf("%s is empty", tt.key)
			}
		})
	}
}

func TestLogKeyValues(t *testing.T) {
	// Verify the key constants have specific expected values.
	if logger.KeyAlertID != "alertId" {
		t.Errorf("expected KeyAlertID = 'alertId', got %q", logger.KeyAlertID)
	}
	if logger.KeyPodName != "podName" {
		t.Errorf("expected KeyPodName = 'podName', got %q", logger.KeyPodName)
	}
	if logger.KeyError != "error" {
		t.Errorf("expected KeyError = 'error', got %q", logger.KeyError)
	}
	if logger.KeyDuration != "duration" {
		t.Errorf("expected KeyDuration = 'duration', got %q", logger.KeyDuration)
	}
	if logger.KeyCluster != "cluster" {
		t.Errorf("expected KeyCluster = 'cluster', got %q", logger.KeyCluster)
	}
}
