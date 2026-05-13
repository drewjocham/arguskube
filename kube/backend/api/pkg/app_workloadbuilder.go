package pkg

import (
	"github.com/argues/argus/internal/k8s"
)

// See app_authjuggler.go: ctx is dropped from Wails bindings; use a.appCtx().

func (a *App) GenerateWorkloadYAML(spec k8s.WorkloadSpec) (*k8s.WorkloadYAML, error) {
	return k8s.GenerateWorkloadYAML(&spec)
}

func (a *App) ListRegistryTags(image string) ([]k8s.RegistryTag, error) {
	client := k8s.NewRegistryClient(image, a.logger)
	return client.ListTags(a.appCtx(), image)
}

func (a *App) ListNamespaces() ([]string, error) {
	return a.k8s.ListAllNamespaces(a.appCtx())
}

func (a *App) DuplicateWorkload(namespace, name, targetNamespace string) (*k8s.DuplicateResult, error) {
	return a.k8s.DuplicateDeployment(a.appCtx(), namespace, name, targetNamespace)
}

func (a *App) RunCronJob(namespace, name string) (string, error) {
	job, err := a.k8s.CreateJobFromCronJob(a.appCtx(), namespace, name)
	if err != nil {
		return "", err
	}
	return job.Name, nil
}

func (a *App) GetCrashLogs(namespace, podName, containerName string) (*k8s.CrashInfo, error) {
	return a.k8s.GetCrashLogs(a.appCtx(), namespace, podName, containerName)
}

func (a *App) GetResourceSuggestion(profile string) (*k8s.ResourceSuggestion, error) {
	return a.k8s.GetResourceSuggestion(a.appCtx(), profile)
}

func (a *App) GetTShirtSizes() map[string]k8s.ResourceProfile {
	return k8s.TShirtSizes()
}

func (a *App) GetNodeCapacities() ([]k8s.NodeCapacity, error) {
	return a.k8s.SuggestResources(a.appCtx())
}
