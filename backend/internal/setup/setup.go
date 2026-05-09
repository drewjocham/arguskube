package setup

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// ansiSGRPattern matches ANSI CSI sequences (color/style codes).
var ansiSGRPattern = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// popeyeVersionPattern extracts the version value from popeye's banner output.
var popeyeVersionPattern = regexp.MustCompile(`(?m)^\s*Version:\s*([^\s]+)`)

// ToolStatus describes the install state of a tool.
type ToolStatus struct {
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Via       string `json:"via"` // "binary", "docker", "helm", ""
	Message   string `json:"message"`
}

// SetupResult is returned from an install/deploy action.
type SetupResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Output  string `json:"output"`
}

// Manager handles detection and setup of external tools.
type Manager struct {
	kubeconfig string
	context    string
	namespace  string
	logger     *slog.Logger
}

func NewManager(kubeconfig, kubecontext, namespace string, logger *slog.Logger) *Manager {
	return &Manager{
		kubeconfig: kubeconfig,
		context:    kubecontext,
		namespace:  namespace,
		logger:     logger,
	}
}

// CheckAllTools returns the status of all external tools.
func (m *Manager) CheckAllTools(ctx context.Context) []ToolStatus {
	return []ToolStatus{
		m.checkPopeye(ctx),
		m.checkDocker(ctx),
		m.checkHelm(ctx),
		m.checkAgent(ctx),
		m.checkKubectl(ctx),
	}
}

func (m *Manager) checkPopeye(ctx context.Context) ToolStatus {
	// Check local binary first.
	if path, err := exec.LookPath("popeye"); err == nil {
		ver := parsePopeyeVersion(runQuiet(ctx, path, "version"))
		return ToolStatus{Name: "popeye", Installed: true, Version: ver, Via: "binary", Message: "Popeye binary found"}
	}

	// Check if Docker image is available.
	if m.hasDockerImage(ctx, "quay.io/derailed/popeye") {
		return ToolStatus{Name: "popeye", Installed: true, Version: "docker", Via: "docker", Message: "Popeye available via Docker"}
	}

	return ToolStatus{Name: "popeye", Installed: false, Message: "Popeye not found. Install via binary or Docker."}
}

func (m *Manager) checkDocker(ctx context.Context) ToolStatus {
	if path, err := exec.LookPath("docker"); err == nil {
		ver := runQuiet(ctx, path, "version", "--format", "{{.Client.Version}}")
		return ToolStatus{Name: "docker", Installed: true, Version: ver, Via: "binary", Message: "Docker available"}
	}
	return ToolStatus{Name: "docker", Installed: false, Message: "Docker not found"}
}

func (m *Manager) checkHelm(ctx context.Context) ToolStatus {
	if path, err := exec.LookPath("helm"); err == nil {
		ver := runQuiet(ctx, path, "version", "--short")
		return ToolStatus{Name: "helm", Installed: true, Version: ver, Via: "binary", Message: "Helm available"}
	}
	return ToolStatus{Name: "helm", Installed: false, Message: "Helm not found"}
}

func (m *Manager) checkKubectl(ctx context.Context) ToolStatus {
	if path, err := exec.LookPath("kubectl"); err == nil {
		ver := runQuiet(ctx, path, "version", "--client", "--short")
		return ToolStatus{Name: "kubectl", Installed: true, Version: ver, Via: "binary", Message: "kubectl available"}
	}
	return ToolStatus{Name: "kubectl", Installed: false, Message: "kubectl not found"}
}

func (m *Manager) checkAgent(ctx context.Context) ToolStatus {
	// Check if the KubeWatcher agent DaemonSet is running in the cluster.
	args := m.kubectlArgs("get", "daemonset", "-l", "app.kubernetes.io/name=kubewatcher-agent", "--all-namespaces", "-o", "jsonpath={.items[0].status.numberReady}")
	out := runQuiet(ctx, "kubectl", args...)
	if out != "" && out != "0" {
		return ToolStatus{Name: "kubewatcher-agent", Installed: true, Version: out + " ready", Via: "helm", Message: "Agent running in cluster"}
	}
	return ToolStatus{Name: "kubewatcher-agent", Installed: false, Message: "Agent not deployed to cluster"}
}

// InstallPopeye pulls the Popeye Docker image (or installs via go install if Go is available).
func (m *Manager) InstallPopeye(ctx context.Context) *SetupResult {
	m.logger.InfoContext(ctx, "installing popeye")

	// Try go install first.
	if goPath, err := exec.LookPath("go"); err == nil {
		m.logger.InfoContext(ctx, "go found, attempting go install")
		cmd := exec.CommandContext(ctx, goPath, "install", "github.com/derailed/popeye@latest")
		out, err := cmd.CombinedOutput()
		if err == nil {
			return &SetupResult{Success: true, Message: "Popeye installed via go install", Output: string(out)}
		}
		m.logger.WarnContext(ctx, "go install failed, falling back to docker", slog.String("error", err.Error()))
	}

	// Fall back to Docker pull.
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return &SetupResult{Success: false, Message: "Neither Go nor Docker found. Install Go (go install github.com/derailed/popeye@latest) or Docker to proceed."}
	}

	cmd := exec.CommandContext(ctx, dockerPath, "pull", "quay.io/derailed/popeye")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &SetupResult{Success: false, Message: "Docker pull failed", Output: string(out)}
	}

	return &SetupResult{Success: true, Message: "Popeye Docker image pulled successfully", Output: string(out)}
}

// DeployAgent deploys the KubeWatcher agent to the cluster using kubectl apply with inline manifests.
func (m *Manager) DeployAgent(ctx context.Context, namespace string) *SetupResult {
	m.logger.InfoContext(ctx, "deploying kubewatcher agent", slog.String("namespace", namespace))

	if namespace == "" {
		namespace = "kubewatcher"
	}

	// Create namespace if it doesn't exist.
	nsArgs := m.kubectlArgs("create", "namespace", namespace, "--dry-run=client", "-o", "yaml")
	nsYaml := runQuiet(ctx, "kubectl", nsArgs...)

	applyNsArgs := m.kubectlArgs("apply", "-f", "-")
	applyNs := exec.CommandContext(ctx, "kubectl", applyNsArgs...)
	applyNs.Stdin = strings.NewReader(nsYaml)
	if out, err := applyNs.CombinedOutput(); err != nil {
		m.logger.WarnContext(ctx, "namespace create warning", slog.String("output", string(out)))
	}

	// Apply all manifests.
	manifests := m.buildAgentManifests(namespace)
	applyArgs := m.kubectlArgs("apply", "-f", "-")
	cmd := exec.CommandContext(ctx, "kubectl", applyArgs...)
	cmd.Stdin = strings.NewReader(manifests)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &SetupResult{Success: false, Message: "Agent deployment failed", Output: string(out)}
	}

	return &SetupResult{Success: true, Message: fmt.Sprintf("Agent deployed to namespace %q", namespace), Output: string(out)}
}

// UndeployAgent removes the agent from the cluster.
func (m *Manager) UndeployAgent(ctx context.Context, namespace string) *SetupResult {
	if namespace == "" {
		namespace = "kubewatcher"
	}

	manifests := m.buildAgentManifests(namespace)
	args := m.kubectlArgs("delete", "-f", "-", "--ignore-not-found")
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Stdin = strings.NewReader(manifests)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &SetupResult{Success: false, Message: "Agent removal failed", Output: string(out)}
	}

	return &SetupResult{Success: true, Message: "Agent removed", Output: string(out)}
}

func (m *Manager) kubectlArgs(args ...string) []string {
	var out []string
	if m.kubeconfig != "" {
		out = append(out, "--kubeconfig", m.kubeconfig)
	}
	if m.context != "" && m.context != "default" {
		out = append(out, "--context", m.context)
	}
	out = append(out, args...)
	return out
}

func (m *Manager) hasDockerImage(ctx context.Context, image string) bool {
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return false
	}
	cmd := exec.CommandContext(ctx, dockerPath, "image", "inspect", image)
	return cmd.Run() == nil
}

func (m *Manager) buildAgentManifests(namespace string) string {
	return fmt.Sprintf(`---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kubewatcher-agent
  namespace: %[1]s
  labels:
    app.kubernetes.io/name: kubewatcher-agent
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubewatcher-agent
  labels:
    app.kubernetes.io/name: kubewatcher-agent
rules:
- apiGroups: [""]
  resources: ["pods", "nodes", "services", "events", "namespaces", "configmaps", "secrets", "persistentvolumeclaims", "persistentvolumes"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["deployments", "statefulsets", "daemonsets", "replicasets"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["batch"]
  resources: ["jobs", "cronjobs"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["networking.k8s.io"]
  resources: ["ingresses", "networkpolicies"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["autoscaling"]
  resources: ["horizontalpodautoscalers"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["metrics.k8s.io"]
  resources: ["pods", "nodes"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kubewatcher-agent
  labels:
    app.kubernetes.io/name: kubewatcher-agent
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kubewatcher-agent
subjects:
- kind: ServiceAccount
  name: kubewatcher-agent
  namespace: %[1]s
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kubewatcher-agent
  namespace: %[1]s
  labels:
    app.kubernetes.io/name: kubewatcher-agent
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: kubewatcher-agent
  template:
    metadata:
      labels:
        app.kubernetes.io/name: kubewatcher-agent
    spec:
      serviceAccountName: kubewatcher-agent
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
      containers:
      - name: agent
        image: ghcr.io/drewjocham/kubewatcher-agent:latest
        imagePullPolicy: IfNotPresent
        ports:
        - name: http
          containerPort: 8080
          protocol: TCP
        env:
        - name: PORT
          value: "8080"
        resources:
          limits:
            cpu: 500m
            memory: 256Mi
          requests:
            cpu: 100m
            memory: 64Mi
        securityContext:
          readOnlyRootFilesystem: true
          allowPrivilegeEscalation: false
          capabilities:
            drop: ["ALL"]
        livenessProbe:
          httpGet:
            path: /health
            port: http
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: http
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: kubewatcher-agent
  namespace: %[1]s
  labels:
    app.kubernetes.io/name: kubewatcher-agent
spec:
  type: ClusterIP
  ports:
  - port: 8080
    targetPort: http
    protocol: TCP
    name: http
  selector:
    app.kubernetes.io/name: kubewatcher-agent
`, namespace)
}

func runQuiet(ctx context.Context, name string, args ...string) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// parsePopeyeVersion extracts the version from popeye's banner output.
// `popeye version` prints an ANSI-colored ASCII banner with a "Version: X.Y.Z"
// line; without parsing, the entire banner ends up in the UI's Version field.
func parsePopeyeVersion(raw string) string {
	if raw == "" {
		return ""
	}
	cleaned := ansiSGRPattern.ReplaceAllString(raw, "")
	if m := popeyeVersionPattern.FindStringSubmatch(cleaned); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	// Fall back to first non-empty line if the pattern doesn't match,
	// avoiding the multi-line banner that motivated this helper.
	for _, line := range strings.Split(cleaned, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			return s
		}
	}
	return ""
}
