package popeye

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		{"po", "pod"},
		{"deployment", "deploy"},
		{"deploy", "deploy"},
		{"service", "svc"},
		{"svc", "svc"},
		{"daemonset", "ds"},
		{"ds", "ds"},
		{"statefulset", "sts"},
		{"sts", "sts"},
		{"configmap", "cm"},
		{"cm", "cm"},
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

// TestExecBinaryEmptyOutputReturnsHelpfulError exercises the case where the popeye
// binary exits cleanly but writes only a banner (no JSON) to stdout. This is what
// happens with newer popeye versions when a kubeconfig issue keeps it from reaching
// the API server while --force-exit-zero hides the failure.
func TestExecBinaryEmptyOutputReturnsHelpfulError(t *testing.T) {
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("/bin/sh not available")
	}
	tmp := t.TempDir()
	stub := filepath.Join(tmp, "popeye-stub")
	script := "#!/bin/sh\nprintf ' ___ ___ banner only\\n' >&1\nprintf 'connection refused: 127.0.0.1:6443\\n' >&2\nexit 0\n"
	if err := os.WriteFile(stub, []byte(script), 0755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := NewRunner(stub, "", "", "", logger)

	_, err := r.execBinary(context.Background())
	if err == nil {
		t.Fatal("execBinary() returned nil err for banner-only output, want error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "no parseable output") {
		t.Errorf("err = %q, want to contain 'no parseable output'", msg)
	}
	if !strings.Contains(msg, "kubectl") && !strings.Contains(msg, "connectivity") {
		t.Errorf("err = %q, want a connectivity hint (kubectl/connectivity)", msg)
	}
	if !strings.Contains(msg, "stderr") || !strings.Contains(msg, "connection refused") {
		t.Errorf("err = %q, want stderr context to surface 'connection refused'", msg)
	}
}

// TestExecBinaryDoesNotPassNoColor regression-tests for popeye 0.22+ which
// rejects the --no-color flag with "unknown flag" before producing any output.
// The stub records its args to a file; the test asserts --no-color is absent.
func TestExecBinaryDoesNotPassNoColor(t *testing.T) {
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("/bin/sh not available")
	}
	tmp := t.TempDir()
	stub := filepath.Join(tmp, "popeye-stub")
	argsFile := filepath.Join(tmp, "args.txt")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsFile + "\nprintf '{\"popeye\":{\"score\":80}}\\n'\nexit 0\n"
	if err := os.WriteFile(stub, []byte(script), 0755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := NewRunner(stub, "", "", "", logger)

	if _, err := r.execBinary(context.Background()); err != nil {
		t.Fatalf("execBinary() error: %v", err)
	}
	recorded, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read recorded args: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(recorded)), "\n") {
		if line == "--no-color" {
			t.Errorf("execBinary should not pass --no-color (popeye 0.22+ rejects it); recorded args:\n%s", string(recorded))
		}
	}
	if !strings.Contains(string(recorded), "--out") || !strings.Contains(string(recorded), "json") {
		t.Errorf("expected --out json in args, got:\n%s", string(recorded))
	}
}

// TestExecBinaryBannerThenJSONIsParsedCleanly verifies banner-stripping still
// works: when popeye prints a banner before the JSON, only the JSON portion is
// returned.
func TestExecBinaryBannerThenJSONIsParsedCleanly(t *testing.T) {
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("/bin/sh not available")
	}
	tmp := t.TempDir()
	stub := filepath.Join(tmp, "popeye-stub")
	script := "#!/bin/sh\nprintf ' ___ ___ banner\\n{\"popeye\":{\"score\":80}}\\n'\nexit 0\n"
	if err := os.WriteFile(stub, []byte(script), 0755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := NewRunner(stub, "", "", "", logger)

	out, err := r.execBinary(context.Background())
	if err != nil {
		t.Fatalf("execBinary() error: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(out)), "{") {
		t.Errorf("output = %q, expected to start with '{' after banner strip", string(out))
	}
}
