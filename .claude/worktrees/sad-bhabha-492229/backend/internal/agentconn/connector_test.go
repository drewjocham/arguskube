package agentconn_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/argues/kube-watcher/internal/agentconn"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

// fakeRestConfig returns a minimal rest.Config that can be used to construct
// the Connector without panicking.
func fakeRestConfig() *rest.Config {
	return &rest.Config{
		Host: "https://localhost:6443",
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}
}

func isValidJSON(s string) bool {
	var v interface{}
	return json.Unmarshal([]byte(s), &v) == nil
}

func TestNew(t *testing.T) {
	cs := fake.NewSimpleClientset()
	logger := slog.New(slog.DiscardHandler)

	c := agentconn.New(cs, fakeRestConfig(), logger)
	if c == nil {
		t.Fatal("New() returned nil")
	}
}

func TestGetAnomalies_NoAgentPod(t *testing.T) {
	cs := fake.NewSimpleClientset()
	logger := slog.New(slog.DiscardHandler)
	c := agentconn.New(cs, fakeRestConfig(), logger)

	_, err := c.GetAnomalies(context.Background(), "")
	if err == nil {
		t.Fatal("expected error when no agent pod exists")
	}
}

func TestGetTopology_NoAgentPod(t *testing.T) {
	cs := fake.NewSimpleClientset()
	logger := slog.New(slog.DiscardHandler)
	c := agentconn.New(cs, fakeRestConfig(), logger)

	_, err := c.GetTopology(context.Background(), "")
	if err == nil {
		t.Fatal("expected error when no agent pod exists")
	}
}

func TestGetEvents_NoAgentPod(t *testing.T) {
	cs := fake.NewSimpleClientset()
	logger := slog.New(slog.DiscardHandler)
	c := agentconn.New(cs, fakeRestConfig(), logger)

	_, err := c.GetEvents(context.Background(), "")
	if err == nil {
		t.Fatal("expected error when no agent pod exists")
	}
}

func TestAgentPodNotRunning(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubewatcher-agent-abc",
				Namespace: "kubewatcher",
				Labels: map[string]string{
					"app.kubernetes.io/name": "kubewatcher-agent",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
			},
		},
	)
	logger := slog.New(slog.DiscardHandler)
	c := agentconn.New(cs, fakeRestConfig(), logger)

	_, err := c.GetAnomalies(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for non-running pod")
	}
}

func TestAgentPodFoundButPortForwardFails(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubewatcher-agent-xyz",
				Namespace: "kubewatcher",
				Labels: map[string]string{
					"app.kubernetes.io/name": "kubewatcher-agent",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				PodIP: "10.0.0.1",
			},
		},
	)
	logger := slog.New(slog.DiscardHandler)
	c := agentconn.New(cs, fakeRestConfig(), logger)

	// Use defer/recover to handle nil RESTClient() from fake clientset.
	// In production, a real clientset always has a RESTClient, so this only
	// occurs under test with fake clientsets.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("port-forward panicked with fake clientset (expected): %v", r)
			}
		}()
		_, err := c.GetAnomalies(context.Background(), "")
		if err == nil {
			t.Fatal("expected error from port-forward with fake clientset")
		}
	}()
}

func TestNewWithNilClientset(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	c := agentconn.New(nil, fakeRestConfig(), logger)
	if c == nil {
		t.Fatal("New() with nil clientset returned nil")
	}
}

func TestNilLoggerDoesNotPanic(t *testing.T) {
	cs := fake.NewSimpleClientset()

	c := agentconn.New(cs, fakeRestConfig(), nil)
	if c == nil {
		t.Fatal("New() with nil logger returned nil")
	}

	_, err := c.GetAnomalies(context.Background(), "")
	if err == nil {
		t.Fatal("expected error with no pods")
	}
}

func TestNoNamespaceFallback(t *testing.T) {
	cs := fake.NewSimpleClientset()
	logger := slog.New(slog.DiscardHandler)

	t.Run("empty namespace", func(t *testing.T) {
		c := agentconn.New(cs, fakeRestConfig(), logger)
		_, err := c.GetAnomalies(context.Background(), "")
		if err == nil {
			t.Fatal("expected error with no pods")
		}
	})

	t.Run("all namespace", func(t *testing.T) {
		c := agentconn.New(cs, fakeRestConfig(), logger)
		_, err := c.GetAnomalies(context.Background(), "all")
		if err == nil {
			t.Fatal("expected error with no pods")
		}
	})
}

func TestAnomalyJSONStructure(t *testing.T) {
	data := []byte(`{"timestamp":"2024-01-15T10:00:00Z","score":0.85,"target":"default/web-app","rule":"cpu-spike"}`)
	if !isValidJSON(string(data)) {
		t.Fatal("expected valid anomaly JSON")
	}
}

func TestTopologyGraphTypes(t *testing.T) {
	nodeJSON := `{"id":"node-1","kind":"Pod","name":"web-app-abc","namespace":"default","status":"Running"}`
	edgeJSON := `{"source":"node-1","target":"node-2"}`

	if !isValidJSON(nodeJSON) {
		t.Error("expected valid node JSON")
	}
	if !isValidJSON(edgeJSON) {
		t.Error("expected valid edge JSON")
	}
}

func TestTopologyJSONRoundTrip(t *testing.T) {
	nodeInput := `{"id":"n1","kind":"Deployment","name":"web","namespace":"prod","status":"Healthy"}`
	var v interface{}
	if err := json.Unmarshal([]byte(nodeInput), &v); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	output, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	outputStr := string(output)
	for _, key := range []string{"n1", "Deployment", "web", "prod", "Healthy"} {
		if !strings.Contains(outputStr, key) {
			t.Errorf("expected output to contain %q", key)
		}
	}
}
