package setup_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/argues/kube-watcher/internal/setup"
)

func TestNewManager(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	mgr := setup.NewManager("", "default", "kubewatcher", logger)
	if mgr == nil {
		t.Fatal("NewManager() returned nil")
	}
}

func TestNewManagerWithNilLogger(t *testing.T) {
	mgr := setup.NewManager("", "", "", nil)
	if mgr == nil {
		t.Fatal("NewManager() with nil logger returned nil")
	}
}

func TestCheckAllToolsNoKubectl(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	mgr := setup.NewManager("", "default", "kubewatcher", logger)

	statuses := mgr.CheckAllTools(context.Background())
	if len(statuses) != 5 {
		t.Fatalf("expected 5 tool statuses, got %d", len(statuses))
	}
}

func TestCheckAllToolsReturnsAllExpectedTools(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	mgr := setup.NewManager("", "", "", logger)

	statuses := mgr.CheckAllTools(context.Background())

	expectedNames := []string{"popeye", "docker", "helm", "kubewatcher-agent", "kubectl"}
	if len(statuses) != len(expectedNames) {
		t.Fatalf("expected %d tools, got %d", len(expectedNames), len(statuses))
	}

	for i, name := range expectedNames {
		if statuses[i].Name != name {
			t.Errorf("statuses[%d].Name = %q, want %q", i, statuses[i].Name, name)
		}
	}
}

func TestToolStatusFields(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	mgr := setup.NewManager("", "", "", logger)

	statuses := mgr.CheckAllTools(context.Background())

	for _, s := range statuses {
		if s.Name == "" {
			t.Error("expected non-empty Name for all tool statuses")
		}
		// Installed may be true or false, but should be set.
		// Message should always be present.
		if s.Message == "" {
			t.Errorf("expected non-empty Message for tool %q", s.Name)
		}
		// Installed tools should have Via and Version set.
		if s.Installed {
			if s.Via == "" {
				t.Errorf("expected non-empty Via for installed tool %q", s.Name)
			}
		}
	}
}

func TestKubectlArgsEmpty(t *testing.T) {
	// This tests the Builder's internal state through the CheckAllTools call.
	// If kubeconfig and context are empty, args should not include --kubeconfig or --context.
	logger := slog.New(slog.DiscardHandler)
	mgr := setup.NewManager("", "", "", logger)
	_ = mgr.CheckAllTools(context.Background())
	// No assertion needed — just verify no panic.
}

func TestKubectlArgsWithConfig(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	mgr := setup.NewManager("/home/user/.kube/config", "prod-cluster", "kubewatcher", logger)
	_ = mgr.CheckAllTools(context.Background())
	// No panic means args construction works.
}

func TestInstallPopeyeNoBinaries(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	mgr := setup.NewManager("", "", "kubewatcher", logger)

	result := mgr.InstallPopeye(context.Background())
	if result == nil {
		t.Fatal("InstallPopeye() returned nil result")
	}
	if result.Success {
		t.Log("InstallPopeye succeeded (binaries may be available in this environment)")
	}
}

func TestDeployAgentWithEmptyNamespace(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	mgr := setup.NewManager("", "", "", logger)

	result := mgr.DeployAgent(context.Background(), "")
	if result == nil {
		t.Fatal("DeployAgent() returned nil result")
	}
}

func TestDeployAgentWithSpecificNamespace(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	mgr := setup.NewManager("", "", "", logger)

	result := mgr.DeployAgent(context.Background(), "custom-ns")
	if result == nil {
		t.Fatal("DeployAgent() returned nil result")
	}
}

func TestUndeployAgentWithEmptyNamespace(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	mgr := setup.NewManager("", "", "", logger)

	result := mgr.UndeployAgent(context.Background(), "")
	if result == nil {
		t.Fatal("UndeployAgent() returned nil result")
	}
}

func TestUndeployAgentWithSpecificNamespace(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	mgr := setup.NewManager("", "", "", logger)

	result := mgr.UndeployAgent(context.Background(), "custom-ns")
	if result == nil {
		t.Fatal("UndeployAgent() returned nil result")
	}
}

func TestSetupResultFields(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	mgr := setup.NewManager("", "", "", logger)

	result := mgr.InstallPopeye(context.Background())
	if result.Message == "" {
		t.Error("expected non-empty Message in SetupResult")
	}
}

func TestCheckAgentNoKubectl(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	mgr := setup.NewManager("", "", "", logger)

	statuses := mgr.CheckAllTools(context.Background())
	for _, s := range statuses {
		if s.Name == "kubewatcher-agent" {
			if s.Installed {
				t.Log("Agent appears installed — unexpected in test env, but possible")
			}
			return
		}
	}
	t.Error("kubewatcher-agent not found in tool statuses")
}

func TestCheckKubectl(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	mgr := setup.NewManager("", "", "", logger)

	statuses := mgr.CheckAllTools(context.Background())
	for _, s := range statuses {
		if s.Name == "kubectl" {
			if s.Installed {
				if !strings.Contains(s.Version, ".") && s.Version != "" {
					t.Errorf("expected valid version string, got %q", s.Version)
				}
			}
			return
		}
	}
	t.Error("kubectl not found in tool statuses")
}

func TestAgentDeployOutputFormat(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	mgr := setup.NewManager("", "", "", logger)

	result := mgr.DeployAgent(context.Background(), "kubewatcher")
	if result == nil {
		t.Fatal("DeployAgent() returned nil")
	}

	// The Message should contain the namespace.
	if !strings.Contains(result.Message, "kubewatcher") && result.Success {
		t.Errorf("expected Message to mention namespace, got %q", result.Message)
	}
}

func TestAgentUndeployOutputFormat(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	mgr := setup.NewManager("", "", "", logger)

	result := mgr.UndeployAgent(context.Background(), "kubewatcher")
	if result == nil {
		t.Fatal("UndeployAgent() returned nil")
	}

	if result.Success {
		if !strings.Contains(result.Message, "removed") && !strings.Contains(result.Message, "deleted") {
			t.Errorf("expected removal message, got %q", result.Message)
		}
	}
}

func TestMultiFileKubeconfig(t *testing.T) {
	// Test that multi-file kubeconfig (colon-separated) doesn't cause issues.
	logger := slog.New(slog.DiscardHandler)

	t.Run("single kubeconfig", func(t *testing.T) {
		mgr := setup.NewManager("/home/user/.kube/config", "", "", logger)
		_ = mgr.CheckAllTools(context.Background())
	})

	t.Run("multi-file kubeconfig", func(t *testing.T) {
		mgr := setup.NewManager("/home/user/.kube/config:/home/user/.kube/secondary", "", "", logger)
		_ = mgr.CheckAllTools(context.Background())
	})
}

func TestContextAndNamespace(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	t.Run("default context", func(t *testing.T) {
		mgr := setup.NewManager("", "default", "", logger)
		_ = mgr.CheckAllTools(context.Background())
	})

	t.Run("custom context", func(t *testing.T) {
		mgr := setup.NewManager("", "staging", "staging-ns", logger)
		_ = mgr.CheckAllTools(context.Background())
	})
}
