package tools

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argues/kube-watcher/mcp/monitoring/history"
	"github.com/argues/kube-watcher/mcp/monitoring/recommendation"
	"github.com/argues/kube-watcher/pkg/kube"
)

// --- NodeStatusTool ---

func TestNodeStatusTool_Execute(t *testing.T) {
	tests := []struct {
		name      string
		nodes     []kube.NodeInfo
		nodesErr  error
		args      map[string]any
		wantErr   bool
		checkFunc func(t *testing.T, res map[string]any)
	}{
		{
			name: "AllNodesHealthy",
			nodes: []kube.NodeInfo{
				{Name: "node-1", Status: "Ready", Conditions: []kube.NodeCondition{{Type: "Ready", Status: "True"}}, Allocatable: map[string]string{"cpu": "4"}},
				{Name: "node-2", Status: "Ready", Conditions: []kube.NodeCondition{{Type: "Ready", Status: "True"}}, Allocatable: map[string]string{"cpu": "4"}},
			},
			args: map[string]any{},
			checkFunc: func(t *testing.T, res map[string]any) {
				summary := res["summary"].(map[string]interface{})
				assert.Equal(t, 2, summary["ready"])
				analysis := res["analysis"].(map[string]interface{})
				assert.Equal(t, "healthy", analysis["status"])
			},
		},
		{
			name: "UnhealthyNode",
			nodes: []kube.NodeInfo{
				{Name: "node-1", Status: "Ready", Conditions: []kube.NodeCondition{{Type: "Ready", Status: "True"}}},
				{Name: "node-bad", Status: "NotReady", Conditions: []kube.NodeCondition{{Type: "Ready", Status: "False"}}},
			},
			args: map[string]any{},
			checkFunc: func(t *testing.T, res map[string]any) {
				summary := res["summary"].(map[string]interface{})
				assert.Equal(t, 1, summary["ready"])
				assert.Contains(t, summary["unhealthy"], "node-bad")
			},
		},
		{
			name: "SingleNodeByName",
			nodes: []kube.NodeInfo{
				{Name: "node-1", Status: "Ready", Conditions: []kube.NodeCondition{{Type: "Ready", Status: "True"}}},
			},
			args: map[string]any{"node_name": "node-1"},
			checkFunc: func(t *testing.T, res map[string]any) {
				nodes := res["nodes"].([]map[string]interface{})
				assert.Len(t, nodes, 1)
			},
		},
		{
			name: "TaintsOnly",
			nodes: []kube.NodeInfo{
				{Name: "clean", Status: "Ready", Conditions: []kube.NodeCondition{{Type: "Ready", Status: "True"}}},
				{Name: "tainted", Status: "Ready", Conditions: []kube.NodeCondition{{Type: "Ready", Status: "True"}}, Taints: []corev1.Taint{{Key: "test"}}},
			},
			args: map[string]any{"taints_only": true},
			checkFunc: func(t *testing.T, res map[string]any) {
				nodes := res["nodes"].([]map[string]interface{})
				assert.Len(t, nodes, 1)
			},
		},
		{
			name:     "ErrorFetchingNodes",
			nodesErr: errors.New("connection refused"),
			args:     map[string]any{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockK8sClient{nodes: tt.nodes, nodesErr: tt.nodesErr}
			tool := NewNodeStatusTool(client)

			res, err := tool.Execute(context.Background(), tt.args)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			tt.checkFunc(t, res)
		})
	}
}

func TestClusterAnalysisTool_Name(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "CurrentName", want: "analyze_cluster"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := NewClusterAnalysisTool(&mockK8sClient{})
			assert.Equal(t, tt.want, tool.Name())
		})
	}
}

// --- PodResourcesTool ---

func TestPodResourcesTool_Execute(t *testing.T) {
	tests := []struct {
		name      string
		pods      []kube.PodInfo
		args      map[string]any
		checkFunc func(t *testing.T, res map[string]any)
	}{
		{
			name: "HealthyPods",
			pods: []kube.PodInfo{
				{Name: "api", Namespace: "default", Phase: "Running", Status: "Running", Containers: []kube.ContainerInfo{{Name: "app", Ready: true, State: "Running"}}},
			},
			args: map[string]any{},
			checkFunc: func(t *testing.T, res map[string]any) {
				summary := res["summary"].(map[string]interface{})
				assert.Equal(t, 1, summary["total"])
			},
		},
		{
			name: "HighRestartPod",
			pods: []kube.PodInfo{
				{Name: "crasher", Namespace: "default", Phase: "Running", Status: "Running", RestartCount: 10},
			},
			args: map[string]any{"high_restart_threshold": 5.0},
			checkFunc: func(t *testing.T, res map[string]any) {
				summary := res["summary"].(map[string]interface{})
				assert.Len(t, summary["high_restarts"], 1)
			},
		},
		{
			name: "ProblematicOnly",
			pods: []kube.PodInfo{
				{Name: "good", Namespace: "default", Phase: "Running", Status: "Running"},
				{Name: "bad", Namespace: "default", Phase: "Pending", Status: "Pending", NodeName: ""},
			},
			args: map[string]any{"problematic_only": true},
			checkFunc: func(t *testing.T, res map[string]any) {
				summary := res["summary"].(map[string]interface{})
				assert.Equal(t, 1, summary["total"])
			},
		},
		{
			name: "FilterByNamespace",
			pods: []kube.PodInfo{
				{Name: "api", Namespace: "prod", Phase: "Running", Status: "Running"},
			},
			args: map[string]any{"namespace": "prod"},
			checkFunc: func(t *testing.T, res map[string]any) {
				summary := res["summary"].(map[string]interface{})
				assert.Equal(t, 1, summary["total"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockK8sClient{pods: tt.pods}
			tool := NewPodResourcesTool(client)
			res, err := tool.Execute(context.Background(), tt.args)
			require.NoError(t, err)
			tt.checkFunc(t, res)
		})
	}
}

// --- ClusterAnalysisTool ---

func TestClusterAnalysisTool_Execute(t *testing.T) {
	tests := []struct {
		name      string
		cluster   *kube.ClusterInfo
		nodes     []kube.NodeInfo
		args      map[string]any
		wantErr   bool
		checkFunc func(t *testing.T, res map[string]any)
	}{
		{
			name:    "HealthyCluster",
			cluster: &kube.ClusterInfo{Version: "v1.28", NodeCount: 2},
			nodes: []kube.NodeInfo{
				{Name: "n1", Status: "Ready"},
				{Name: "n2", Status: "Ready"},
			},
			args: map[string]any{"include_events": false, "include_pods": false},
			checkFunc: func(t *testing.T, res map[string]any) {
				health := res["health"].(map[string]any)
				assert.Equal(t, "healthy", health["status"])
				assert.Equal(t, "2/2", health["ready_nodes"])
			},
		},
		{
			name:    "DegradedCluster",
			cluster: &kube.ClusterInfo{Version: "v1.28"},
			nodes: []kube.NodeInfo{
				{Name: "n1", Status: "Ready"},
				{Name: "n2", Status: "NotReady"},
			},
			args: map[string]any{"include_events": false, "include_pods": false},
			checkFunc: func(t *testing.T, res map[string]any) {
				health := res["health"].(map[string]any)
				assert.Equal(t, "degraded", health["status"])
			},
		},
		{
			name:    "SkipEventsAndServices",
			cluster: &kube.ClusterInfo{Version: "v1.28"},
			nodes: []kube.NodeInfo{
				{Name: "n1", Status: "Ready"},
			},
			args: map[string]any{
				"include_events":   false,
				"include_services": false,
				"include_pods":     true,
			},
			checkFunc: func(t *testing.T, res map[string]any) {
				assert.NotContains(t, res, "events")
				assert.NotContains(t, res, "services")
				assert.Contains(t, res, "pods")
			},
		},
		{
			name:    "ClusterInfoError",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockK8sClient{
				cluster: tt.cluster,
				clusterErr: func() error {
					if tt.cluster == nil {
						return errors.New("cluster unavailable")
					}
					return nil
				}(),
				nodes: tt.nodes,
			}
			tool := NewClusterAnalysisTool(client)
			res, err := tool.Execute(context.Background(), tt.args)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			tt.checkFunc(t, res)
		})
	}
}

// --- ClusterEventsTool ---

func TestClusterEventsTool_Execute(t *testing.T) {
	now := time.Now()
	logger := slog.Default()

	tests := []struct {
		name      string
		events    []kube.EventInfo
		eventsErr error
		args      map[string]any
		wantErr   bool
		checkFunc func(t *testing.T, res map[string]any)
	}{
		{
			name: "ReturnsFilteredEvents",
			events: []kube.EventInfo{
				{Type: "Warning", Reason: "BackOff", ObjectKind: "Pod", ObjectName: "api", LastTimestamp: now.Add(-1 * time.Hour), Message: "crash"},
				{Type: "Normal", Reason: "Pulled", ObjectKind: "Pod", ObjectName: "api", LastTimestamp: now.Add(-2 * time.Hour)},
			},
			args: map[string]any{"event_type": "Warning"},
			checkFunc: func(t *testing.T, res map[string]any) {
				assert.Equal(t, 1, res["total_found"])
			},
		},
		{
			name: "AllEvents",
			events: []kube.EventInfo{
				{Type: "Warning", Reason: "BackOff", LastTimestamp: now.Add(-1 * time.Hour)},
				{Type: "Normal", Reason: "Pulled", LastTimestamp: now.Add(-2 * time.Hour)},
			},
			args: map[string]any{},
			checkFunc: func(t *testing.T, res map[string]any) {
				assert.Equal(t, 2, res["total_found"])
			},
		},
		{
			name:      "FetchError",
			eventsErr: errors.New("api error"),
			args:      map[string]any{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockK8sClient{events: tt.events, eventsErr: tt.eventsErr}
			tool := NewClusterEventsTool(client, logger)
			res, err := tool.Execute(context.Background(), tt.args)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			tt.checkFunc(t, res)
		})
	}
}

// --- NamespaceListTool ---

func TestNamespaceListTool_Execute(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name      string
		ns        []string
		pods      []kube.PodInfo
		args      map[string]any
		wantErr   bool
		checkFunc func(t *testing.T, res map[string]any)
	}{
		{
			name: "BasicNamespaces",
			ns:   []string{"default", "prod"},
			pods: []kube.PodInfo{{Name: "api", Namespace: "prod", Phase: "Running"}},
			args: map[string]any{"include_system": true},
			checkFunc: func(t *testing.T, res map[string]any) {
				summary := res["summary"].(map[string]any)
				assert.Equal(t, 2, summary["total_namespaces"])
			},
		},
		{
			name: "ExcludeSystem",
			ns:   []string{"kube-system", "prod"},
			args: map[string]any{},
			checkFunc: func(t *testing.T, res map[string]any) {
				nsList := res["namespaces"].([]map[string]any)
				assert.Len(t, nsList, 1)
			},
		},
		{
			name:    "NamespaceError",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockK8sClient{
				namespaces: tt.ns,
				pods:       tt.pods,
				nsErr: func() error {
					if tt.wantErr {
						return errors.New("forbidden")
					}
					return nil
				}(),
			}
			tool := NewNamespaceListTool(client, logger)
			res, err := tool.Execute(context.Background(), tt.args)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			tt.checkFunc(t, res)
		})
	}
}

// --- PodLogsTool ---

func TestPodLogsTool_Execute(t *testing.T) {
	tests := []struct {
		name      string
		client    *mockK8sClient
		args      map[string]any
		wantErr   bool
		checkFunc func(t *testing.T, res map[string]any)
	}{
		{
			name: "FetchesLogsWithDefaults",
			client: &mockK8sClient{
				podLogs: "line1\nline2",
			},
			args: map[string]any{
				"namespace": "prod",
				"pod_name":  "api-123",
			},
			checkFunc: func(t *testing.T, res map[string]any) {
				assert.Equal(t, "prod", res["namespace"])
				assert.Equal(t, "api-123", res["pod_name"])
				assert.Equal(t, "line1\nline2", res["logs"])
				assert.EqualValues(t, 200, res["tail_lines"])
			},
		},
		{
			name: "FetchesLogsWithOptionalArgs",
			client: &mockK8sClient{
				podLogs: "previous logs",
			},
			args: map[string]any{
				"namespace":     "prod",
				"pod_name":      "worker-0",
				"container":     "job",
				"tail_lines":    50.0,
				"since_seconds": 600.0,
				"previous":      true,
			},
			checkFunc: func(t *testing.T, res map[string]any) {
				assert.Equal(t, "job", res["container"])
				assert.EqualValues(t, 50, res["tail_lines"])
				assert.EqualValues(t, 600, res["since_seconds"])
				assert.Equal(t, true, res["previous"])
			},
		},
		{
			name:   "MissingNamespace",
			client: &mockK8sClient{},
			args: map[string]any{
				"pod_name": "api-123",
			},
			wantErr: true,
		},
		{
			name:   "MissingPodName",
			client: &mockK8sClient{},
			args: map[string]any{
				"namespace": "prod",
			},
			wantErr: true,
		},
		{
			name: "ClientError",
			client: &mockK8sClient{
				podLogsErr: errors.New("forbidden"),
			},
			args: map[string]any{
				"namespace": "prod",
				"pod_name":  "api-123",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := NewPodLogsTool(tt.client)
			res, err := tool.Execute(context.Background(), tt.args)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			tt.checkFunc(t, res)
		})
	}
}

// --- RecommendationTool ---

func TestRecommendationTool_Execute(t *testing.T) {
	store, err := history.NewStore(t.TempDir())
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	logger := slog.Default()
	engine := recommendation.NewEngine(store, logger)

	tests := []struct {
		name      string
		args      map[string]any
		wantErr   bool
		checkFunc func(t *testing.T, res map[string]any)
	}{
		{
			name: "NodeAlert",
			args: map[string]any{
				"kind":     "node",
				"name":     "worker-1",
				"severity": "critical",
				"message":  "Node not ready",
			},
			checkFunc: func(t *testing.T, res map[string]any) {
				rec := res["recommendation"].(map[string]interface{})
				assert.NotEmpty(t, rec["title"])
				steps := rec["remediation_steps"].([]string)
				assert.True(t, len(steps) > 0)
			},
		},
		{
			name: "PodAlert",
			args: map[string]any{
				"kind":    "pod",
				"name":    "api",
				"reason":  "CrashLoopBackOff",
				"message": "container crashing",
			},
			checkFunc: func(t *testing.T, res map[string]any) {
				rec := res["recommendation"].(map[string]interface{})
				assert.NotEmpty(t, rec["remediation_steps"])
			},
		},
		{
			name:    "MissingRequiredFields",
			args:    map[string]any{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockK8sClient{}
			tool := NewRecommendationTool(client, engine)
			res, err := tool.Execute(context.Background(), tt.args)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			tt.checkFunc(t, res)
		})
	}
}

// --- VersionTool ---

func TestVersionTool_Execute(t *testing.T) {
	tests := []struct {
		name      string
		args      map[string]any
		checkFunc func(t *testing.T, res map[string]any)
	}{
		{
			name: "DefaultWithBuildMeta",
			args: map[string]any{},
			checkFunc: func(t *testing.T, res map[string]any) {
				assert.Equal(t, "1.0.0", res["version"])
				assert.Equal(t, "abc123", res["git_commit"])
			},
		},
		{
			name: "WithoutBuildMeta",
			args: map[string]any{"include_build_metadata": false},
			checkFunc: func(t *testing.T, res map[string]any) {
				assert.Equal(t, "1.0.0", res["version"])
				assert.Nil(t, res["git_commit"])
			},
		},
		{
			name: "WithGoVersion",
			args: map[string]any{"include_go_version": true},
			checkFunc: func(t *testing.T, res map[string]any) {
				assert.NotEmpty(t, res["go_version"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := NewVersionTool("1.0.0", "abc123", "2024-01-01")
			res, err := tool.Execute(context.Background(), tt.args)
			require.NoError(t, err)
			tt.checkFunc(t, res)
		})
	}
}
