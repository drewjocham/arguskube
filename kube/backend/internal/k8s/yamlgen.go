package k8s

import (
	"fmt"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	sigyaml "sigs.k8s.io/yaml"
)

type PortSpec struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort,omitempty"`
	Protocol   string `json:"protocol"`
}

type EnvVarSpec struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ResourceProfile struct {
	CPURequest    string `json:"cpuRequest,omitempty"`
	MemoryRequest string `json:"memoryRequest,omitempty"`
	CPULimit      string `json:"cpuLimit,omitempty"`
	MemoryLimit   string `json:"memoryLimit,omitempty"`
}

type ProbeSpec struct {
	Type                string `json:"type"`
	HTTPPath            string `json:"httpPath,omitempty"`
	HTTPPort            int32  `json:"httpPort,omitempty"`
	TCPSocketPort       int32  `json:"tcpSocketPort,omitempty"`
	Command             string `json:"command,omitempty"`
	InitialDelaySeconds int32  `json:"initialDelaySeconds"`
	PeriodSeconds       int32  `json:"periodSeconds"`
	TimeoutSeconds      int32  `json:"timeoutSeconds"`
	FailureThreshold    int32  `json:"failureThreshold"`
}

type WorkloadSpec struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Image       string            `json:"image"`
	Replicas    int32             `json:"replicas"`
	Labels      map[string]string `json:"labels,omitempty"`
	Ports       []PortSpec        `json:"ports,omitempty"`
	EnvVars     []EnvVarSpec      `json:"envVars,omitempty"`
	Resources   ResourceProfile   `json:"resources,omitempty"`
	Liveness    *ProbeSpec        `json:"liveness,omitempty"`
	Readiness   *ProbeSpec        `json:"readiness,omitempty"`
	Startup     *ProbeSpec        `json:"startup,omitempty"`
	GenerateSvc bool              `json:"generateSvc"`
}

type WorkloadYAML struct {
	Deployment string `json:"deployment"`
	Service    string `json:"service,omitempty"`
}

func buildProbe(p *ProbeSpec) *corev1.Probe {
	if p == nil {
		return nil
	}
	probe := &corev1.Probe{
		InitialDelaySeconds: p.InitialDelaySeconds,
		PeriodSeconds:       p.PeriodSeconds,
		TimeoutSeconds:      p.TimeoutSeconds,
		FailureThreshold:    p.FailureThreshold,
	}
	switch p.Type {
	case "http":
		path := p.HTTPPath
		if path == "" {
			path = "/"
		}
		port := p.HTTPPort
		if port == 0 {
			port = 8080
		}
		probe.ProbeHandler = corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: path,
				Port: intstr.FromInt32(port),
			},
		}
	case "tcp":
		port := p.TCPSocketPort
		if port == 0 {
			port = 8080
		}
		probe.ProbeHandler = corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt32(port),
			},
		}
	case "command":
		probe.ProbeHandler = corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{"sh", "-c", p.Command},
			},
		}
	}
	return probe
}

func parseResourceQuantity(s string) (resource.Quantity, error) {
	if s == "" {
		return resource.Quantity{}, nil
	}
	return resource.ParseQuantity(s)
}

func buildDeployment(spec *WorkloadSpec) (*appsv1.Deployment, error) {
	labels := map[string]string{"app": spec.Name}
	for k, v := range spec.Labels {
		labels[k] = v
	}

	requests := corev1.ResourceList{}
	limits := corev1.ResourceList{}

	if spec.Resources.CPURequest != "" {
		q, err := parseResourceQuantity(spec.Resources.CPURequest)
		if err != nil {
			return nil, fmt.Errorf("invalid cpu request %q: %w", spec.Resources.CPURequest, err)
		}
		requests[corev1.ResourceCPU] = q
	}
	if spec.Resources.MemoryRequest != "" {
		q, err := parseResourceQuantity(spec.Resources.MemoryRequest)
		if err != nil {
			return nil, fmt.Errorf("invalid memory request %q: %w", spec.Resources.MemoryRequest, err)
		}
		requests[corev1.ResourceMemory] = q
	}
	if spec.Resources.CPULimit != "" {
		q, err := parseResourceQuantity(spec.Resources.CPULimit)
		if err != nil {
			return nil, fmt.Errorf("invalid cpu limit %q: %w", spec.Resources.CPULimit, err)
		}
		limits[corev1.ResourceCPU] = q
	}
	if spec.Resources.MemoryLimit != "" {
		q, err := parseResourceQuantity(spec.Resources.MemoryLimit)
		if err != nil {
			return nil, fmt.Errorf("invalid memory limit %q: %w", spec.Resources.MemoryLimit, err)
		}
		limits[corev1.ResourceMemory] = q
	}

	container := corev1.Container{
		Name:            spec.Name,
		Image:           spec.Image,
		ImagePullPolicy: corev1.PullAlways,
		Resources: corev1.ResourceRequirements{
			Requests: requests,
			Limits:   limits,
		},
	}

	for _, p := range spec.Ports {
		container.Ports = append(container.Ports, corev1.ContainerPort{
			Name:          p.Name,
			ContainerPort: p.Port,
			Protocol:      corev1.Protocol(p.Protocol),
		})
	}

	for _, e := range spec.EnvVars {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  e.Name,
			Value: e.Value,
		})
	}

	if spec.Liveness != nil {
		container.LivenessProbe = buildProbe(spec.Liveness)
	}
	if spec.Readiness != nil {
		container.ReadinessProbe = buildProbe(spec.Readiness)
	}
	if spec.Startup != nil {
		container.StartupProbe = buildProbe(spec.Startup)
	}

	replicas := spec.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": spec.Name},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
					MaxSurge:       &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{container},
				},
			},
		},
	}, nil
}

func buildService(spec *WorkloadSpec) *corev1.Service {
	labels := map[string]string{"app": spec.Name}
	for k, v := range spec.Labels {
		labels[k] = v
	}

	svcPorts := make([]corev1.ServicePort, 0, len(spec.Ports))
	for _, p := range spec.Ports {
		target := p.TargetPort
		if target == 0 {
			target = p.Port
		}
		svcPorts = append(svcPorts, corev1.ServicePort{
			Name:       p.Name,
			Port:       p.Port,
			TargetPort: intstr.FromInt32(target),
			Protocol:   corev1.Protocol(p.Protocol),
		})
	}
	if len(svcPorts) == 0 {
		svcPorts = append(svcPorts, corev1.ServicePort{
			Name:       "http",
			Port:       80,
			TargetPort: intstr.FromInt32(8080),
			Protocol:   corev1.ProtocolTCP,
		})
	}

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"app": spec.Name},
			Ports:    svcPorts,
		},
	}
}

var tShirtSizes = map[string]ResourceProfile{
	"small-web":     {CPURequest: "100m", MemoryRequest: "128Mi", CPULimit: "500m", MemoryLimit: "256Mi"},
	"medium-api":    {CPURequest: "250m", MemoryRequest: "512Mi", CPULimit: "1", MemoryLimit: "1Gi"},
	"heavy-java":    {CPURequest: "1", MemoryRequest: "2Gi", CPULimit: "2", MemoryLimit: "4Gi"},
	"data-pipeline": {CPURequest: "500m", MemoryRequest: "1Gi", CPULimit: "2", MemoryLimit: "2Gi"},
}

func GenerateWorkloadYAML(spec *WorkloadSpec) (*WorkloadYAML, error) {
	if spec.Name == "" {
		return nil, fmt.Errorf("workload name is required")
	}
	if spec.Image == "" {
		return nil, fmt.Errorf("container image is required")
	}
	if spec.Namespace == "" {
		spec.Namespace = "default"
	}

	dep, err := buildDeployment(spec)
	if err != nil {
		return nil, err
	}
	depYAML, err := sigyaml.Marshal(dep)
	if err != nil {
		return nil, fmt.Errorf("marshal deployment: %w", err)
	}

	out := &WorkloadYAML{
		Deployment: string(depYAML),
	}

	if spec.GenerateSvc {
		svc := buildService(spec)
		svcYAML, err := sigyaml.Marshal(svc)
		if err != nil {
			return nil, fmt.Errorf("marshal service: %w", err)
		}
		out.Service = string(svcYAML)
	}

	return out, nil
}

func TShirtSizes() map[string]ResourceProfile {
	out := make(map[string]ResourceProfile, len(tShirtSizes))
	for k, v := range tShirtSizes {
		out[k] = v
	}
	return out
}

func GetTShirtSize(name string) (ResourceProfile, bool) {
	p, ok := tShirtSizes[name]
	return p, ok
}

func sortedMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
