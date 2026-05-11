package audit

import (
	"context"
	"log/slog"
	"testing"
)

type testLogHandler struct {
	attrs map[string]string
}

func (h *testLogHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *testLogHandler) Handle(_ context.Context, _ slog.Record) error { return nil }
func (h *testLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	m := make(map[string]string, len(attrs))
	for _, a := range attrs {
		m[a.Key] = a.Value.String()
	}
	return &testLogHandler{attrs: m}
}
func (h *testLogHandler) WithGroup(string) slog.Handler { return h }

func TestNewSlogLogger(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "creates logger"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewSlogLogger(slog.Default())
			if l == nil {
				t.Fatal("expected non-nil logger")
			}
			if l.logger == nil {
				t.Error("expected slog.Logger to be set")
			}
		})
	}
}

func TestLog(t *testing.T) {
	tests := []struct {
		name      string
		action    string
		resource  string
		namespace string
		objName   string
	}{
		{name: "logs list nodes", action: "list", resource: "nodes", namespace: "", objName: ""},
		{name: "logs get pod with namespace", action: "get", resource: "pod", namespace: "default", objName: "my-pod"},
		{name: "logs delete resource", action: "delete", resource: "deployment", namespace: "prod", objName: "web-v2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &testLogHandler{}
			l := NewSlogLogger(slog.New(h))
			l.Log(tt.action, tt.resource, tt.namespace, tt.objName)
		})
	}
}
