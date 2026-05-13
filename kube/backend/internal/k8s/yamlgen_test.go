package k8s

import (
	"strings"
	"testing"
)

func TestGenerateWorkloadYAML(t *testing.T) {
	tests := []struct {
		name    string
		spec    WorkloadSpec
		wantErr string
		wantDep bool
		wantSvc bool
		checks  []string
	}{
		{
			name:    "requires name",
			spec:    WorkloadSpec{Image: "nginx:latest"},
			wantErr: "workload name is required",
		},
		{
			name:    "requires image",
			spec:    WorkloadSpec{Name: "my-app"},
			wantErr: "container image is required",
		},
		{
			name: "minimal deployment",
			spec: WorkloadSpec{
				Name:  "my-app",
				Image: "nginx:latest",
			},
			wantDep: true,
			checks: []string{
				"kind: Deployment",
				"name: my-app",
				"image: nginx:latest",
				"replicas: 1",
				"apps/v1",
			},
		},
		{
			name: "deployment with service",
			spec: WorkloadSpec{
				Name:        "web",
				Image:       "nginx:1.25",
				Namespace:   "staging",
				Replicas:    3,
				GenerateSvc: true,
			},
			wantDep: true,
			wantSvc: true,
			checks: []string{
				"kind: Deployment",
				"kind: Service",
				"name: web",
				"namespace: staging",
				"replicas: 3",
				"nginx:1.25",
			},
		},
		{
			name: "with ports and env vars",
			spec: WorkloadSpec{
				Name:  "api",
				Image: "myapi:v2",
				Ports: []PortSpec{
					{Name: "http", Port: 8080, TargetPort: 8080, Protocol: "TCP"},
					{Name: "metrics", Port: 9090, TargetPort: 9090, Protocol: "TCP"},
				},
				EnvVars: []EnvVarSpec{
					{Name: "ENV", Value: "production"},
					{Name: "LOG_LEVEL", Value: "info"},
				},
				Resources: ResourceProfile{
					CPURequest:    "250m",
					MemoryRequest: "512Mi",
					CPULimit:      "1",
					MemoryLimit:   "1Gi",
				},
				GenerateSvc: true,
			},
			wantDep: true,
			wantSvc: true,
			checks: []string{
				"containerPort: 8080",
				"containerPort: 9090",
				"name: ENV",
				"value: production",
				"name: LOG_LEVEL",
				"value: info",
				"cpu: 250m",
				"memory: 512Mi",
			},
		},
		{
			name: "with probes",
			spec: WorkloadSpec{
				Name:  "probed-app",
				Image: "app:latest",
				Liveness: &ProbeSpec{
					Type:          "http",
					HTTPPath:      "/healthz",
					HTTPPort:      8080,
					PeriodSeconds: 10,
				},
				Readiness: &ProbeSpec{
					Type:          "tcp",
					TCPSocketPort: 8080,
					PeriodSeconds: 5,
				},
			},
			wantDep: true,
			checks: []string{
				"livenessProbe",
				"readinessProbe",
				"httpGet",
				"path: /healthz",
				"tcpSocket",
				"periodSeconds: 10",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GenerateWorkloadYAML(&tt.spec)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("result is nil")
			}
			if tt.wantDep && result.Deployment == "" {
				t.Error("expected deployment YAML, got empty")
			}
			if tt.wantSvc && result.Service == "" {
				t.Error("expected service YAML, got empty")
			}
			if !tt.wantSvc && result.Service != "" {
				t.Error("expected no service YAML, got one")
			}
			for _, check := range tt.checks {
				if !strings.Contains(result.Deployment, check) && !strings.Contains(result.Service, check) {
					t.Errorf("expected output to contain %q", check)
				}
			}
		})
	}
}

func TestTShirtSizes(t *testing.T) {
	tests := []struct {
		name     string
		profile  string
		wantOK   bool
		wantCPU  string
		wantMem  string
	}{
		{name: "small-web", profile: "small-web", wantOK: true, wantCPU: "100m", wantMem: "128Mi"},
		{name: "medium-api", profile: "medium-api", wantOK: true, wantCPU: "250m", wantMem: "512Mi"},
		{name: "heavy-java", profile: "heavy-java", wantOK: true, wantCPU: "1", wantMem: "2Gi"},
		{name: "data-pipeline", profile: "data-pipeline", wantOK: true, wantCPU: "500m", wantMem: "1Gi"},
		{name: "unknown", profile: "unknown", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, ok := GetTShirtSize(tt.profile)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantOK {
				if p.CPURequest != tt.wantCPU {
					t.Errorf("CPURequest = %q, want %q", p.CPURequest, tt.wantCPU)
				}
				if p.MemoryRequest != tt.wantMem {
					t.Errorf("MemoryRequest = %q, want %q", p.MemoryRequest, tt.wantMem)
				}
			}
		})
	}
}

func TestBuildProbe(t *testing.T) {
	tests := []struct {
		name   string
		spec   *ProbeSpec
		check  string
		isHTTP bool
		isTCP  bool
		isExec bool
	}{
		{name: "nil probe", spec: nil, isHTTP: false, isTCP: false, isExec: false},
		{name: "http probe", spec: &ProbeSpec{Type: "http", HTTPPath: "/health", HTTPPort: 8080}, isHTTP: true, check: "/health"},
		{name: "tcp probe", spec: &ProbeSpec{Type: "tcp", TCPSocketPort: 3306}, isTCP: true, check: "3306"},
		{name: "command probe", spec: &ProbeSpec{Type: "command", Command: "pg_isready"}, isExec: true, check: "pg_isready"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probe := buildProbe(tt.spec)
			if tt.spec == nil {
				if probe != nil {
					t.Error("expected nil probe")
				}
				return
			}
			if probe == nil {
				t.Fatal("expected non-nil probe")
			}
			if tt.isHTTP && probe.HTTPGet == nil {
				t.Error("expected HTTPGet handler")
			}
			if tt.isTCP && probe.TCPSocket == nil {
				t.Error("expected TCPSocket handler")
			}
			if tt.isExec && probe.Exec == nil {
				t.Error("expected Exec handler")
			}
		})
	}
}
