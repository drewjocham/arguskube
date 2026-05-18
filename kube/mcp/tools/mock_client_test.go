package tools

import (
	"context"

	clientset "k8s.io/client-go/kubernetes"

	"github.com/argues/argus/pkg/kube"
)

type mockK8sClient struct {
	nodes      []kube.NodeInfo
	nodesErr   error
	node       *kube.NodeInfo
	nodeErr    error
	pods       []kube.PodInfo
	podsErr    error
	events     []kube.EventInfo
	eventsErr  error
	namespaces []string
	nsErr      error
	quotas     []kube.ResourceQuotaInfo
	cluster    *kube.ClusterInfo
	clusterErr error
	healthErr  error
	podLogs    string
	podLogsErr error
}

func (m *mockK8sClient) GetNodes(_ context.Context) ([]kube.NodeInfo, error) {
	return m.nodes, m.nodesErr
}

func (m *mockK8sClient) GetNode(_ context.Context, name string) (*kube.NodeInfo, error) {
	if m.node != nil {
		return m.node, m.nodeErr
	}
	for _, n := range m.nodes {
		if n.Name == name {
			return &n, nil
		}
	}
	return nil, m.nodeErr
}

func (m *mockK8sClient) GetPods(_ context.Context, _ string) ([]kube.PodInfo, error) {
	return m.pods, m.podsErr
}

func (m *mockK8sClient) GetPodsAllNamespaces(_ context.Context) ([]kube.PodInfo, error) {
	return m.pods, m.podsErr
}

func (m *mockK8sClient) GetPod(_ context.Context, _, _ string) (*kube.PodInfo, error) {
	return nil, nil
}

func (m *mockK8sClient) GetPodLogs(_ context.Context, _, _, _ string, _, _ int64, _ bool) (string, error) {
	return m.podLogs, m.podLogsErr
}

func (m *mockK8sClient) GetServices(_ context.Context, _ string) ([]kube.ServiceInfo, error) {
	return nil, nil
}

func (m *mockK8sClient) GetServicesAllNamespaces(_ context.Context) ([]kube.ServiceInfo, error) {
	return nil, nil
}

func (m *mockK8sClient) GetEvents(_ context.Context, _ string) ([]kube.EventInfo, error) {
	return m.events, m.eventsErr
}

func (m *mockK8sClient) GetEventsAllNamespaces(_ context.Context) ([]kube.EventInfo, error) {
	return m.events, m.eventsErr
}

func (m *mockK8sClient) GetNamespaces(_ context.Context) ([]string, error) {
	return m.namespaces, m.nsErr
}

func (m *mockK8sClient) GetResourceQuotas(_ context.Context, _ string) ([]kube.ResourceQuotaInfo, error) {
	return m.quotas, nil
}

func (m *mockK8sClient) GetClusterInfo(_ context.Context) (*kube.ClusterInfo, error) {
	return m.cluster, m.clusterErr
}

func (m *mockK8sClient) HealthCheck(_ context.Context) error {
	return m.healthErr
}

func (m *mockK8sClient) GetRawInterface() clientset.Interface {
	return nil
}
