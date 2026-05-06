package popeye

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

func TestNewRunner(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := NewRunner("popeye", "/tmp/kubeconfig", "test-ctx", "default", logger)

	if runner == nil {
		t.Fatal("NewRunner() returned nil")
	}
}

func TestNewRunnerEmpty(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := NewRunner("", "", "", "", logger)

	if runner == nil {
		t.Fatal("NewRunner() returned nil with empty params")
	}
}

func TestRunnerRunNoBinary(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := NewRunner("nonexistent-binary-xyz", "", "", "", logger)

	_, err := runner.Run(context.Background())
	if err == nil {
		t.Fatal("Run() should fail when popeye binary is not found")
	}
}

func TestSeverityLevels(t *testing.T) {
	tests := []struct {
		level SeverityLevel
		want  string
	}{
		{SevOK, "ok"},
		{SevInfo, "info"},
		{SevWarn, "warning"},
		{SevErr, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSeverityColors(t *testing.T) {
	tests := []struct {
		level SeverityLevel
		want  string
	}{
		{SevOK, "green"},
		{SevInfo, "blue"},
		{SevWarn, "amber"},
		{SevErr, "red"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.level.Color(); got != tt.want {
				t.Errorf("Color() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSingularResource(t *testing.T) {
	tests := []struct {
		plural string
		want   string
	}{
		{"pod", "pod"},
		{"pods", "pod"},
		{"deployment", "deploy"},
		{"deployments", "deploy"},
		{"service", "svc"},
		{"daemonset", "ds"},
		{"statefulset", "sts"},
		{"configmap", "cm"},
		{"secret", "secret"},
		{"node", "node"},
		{"ingress", "ing"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.plural, func(t *testing.T) {
			got := singularResource(tt.plural)
			if got != tt.want {
				t.Errorf("singularResource(%q) = %q, want %q", tt.plural, got, tt.want)
			}
		})
	}
}
