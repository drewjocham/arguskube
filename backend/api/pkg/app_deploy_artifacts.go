package pkg

import (
	"fmt"
	"strings"
)

// DeployArtifact is the deployable shape for one tool, in one flavor.
// The frontend uses it both for "show me the command" (CommandText) and
// "download the file" (FileText + FileName).
type DeployArtifact struct {
	Tool        string `json:"tool"`   // matches setup tool keys: kubectl, helm, popeye, kubewatcher-agent…
	Flavor      string `json:"flavor"` // "helm" | "docker" | "compose"
	Description string `json:"description"`
	CommandText string `json:"commandText"` // a copy-pasteable shell line
	FileText    string `json:"fileText"`    // the file body (helm values, docker-compose.yaml, etc.)
	FileName    string `json:"fileName"`    // suggested download name; "" if no file
	// EnvVars is the list of variables this artifact expects. The
	// frontend can highlight ones that are missing or invalid based
	// on the user's uploaded env file.
	EnvVars []EnvVarSpec `json:"envVars"`
}

// EnvVarSpec describes one env variable: name, whether it's required,
// and a brief hint for the user.
type EnvVarSpec struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Hint     string `json:"hint"`
	Default  string `json:"default,omitempty"`
}

// GetDeployArtifacts returns every supported (tool, flavor) tuple. The
// frontend filters by tool when rendering. Static today; could be
// templated against the user's cluster context later.
func (a *App) GetDeployArtifacts() []DeployArtifact {
	return deployArtifacts()
}

// GetDeployArtifact returns a single artifact by tool + flavor for
// targeted UIs. Returns ErrUnknownArtifact when the combination isn't
// supported.
func (a *App) GetDeployArtifact(tool, flavor string) (DeployArtifact, error) {
	for _, art := range deployArtifacts() {
		if art.Tool == tool && art.Flavor == flavor {
			return art, nil
		}
	}
	return DeployArtifact{}, fmt.Errorf("no artifact for tool=%q flavor=%q", tool, flavor)
}

// ValidateEnvFile parses an uploaded `.env`-formatted body and returns
// the names of variables that are missing or empty for the requested
// (tool, flavor). The frontend uses this to prompt the user before
// it kicks off the install.
//
// Format: one KEY=VALUE per line, # comments allowed, blank lines OK.
// Quote handling is intentionally minimal — this is local-dev tooling,
// not a shell parser.
func (a *App) ValidateEnvFile(tool, flavor, body string) (EnvValidationResult, error) {
	art, err := a.GetDeployArtifact(tool, flavor)
	if err != nil {
		return EnvValidationResult{}, err
	}
	provided := parseDotenv(body)

	var missing []string
	var present []string
	for _, spec := range art.EnvVars {
		if !spec.Required {
			continue
		}
		v, ok := provided[spec.Name]
		if !ok || strings.TrimSpace(v) == "" {
			missing = append(missing, spec.Name)
			continue
		}
		present = append(present, spec.Name)
	}
	return EnvValidationResult{
		Tool:    tool,
		Flavor:  flavor,
		Missing: missing,
		Present: present,
		Vars:    provided,
	}, nil
}

// EnvValidationResult is what ValidateEnvFile returns.
type EnvValidationResult struct {
	Tool    string            `json:"tool"`
	Flavor  string            `json:"flavor"`
	Missing []string          `json:"missing"`
	Present []string          `json:"present"`
	Vars    map[string]string `json:"vars"`
}

// parseDotenv is the bare-minimum .env parser this UI needs. Strips
// surrounding double-quotes and trailing whitespace; ignores `export `
// prefix; ignores comments.
func parseDotenv(body string) map[string]string {
	out := make(map[string]string)
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:eq])
		v := strings.TrimSpace(line[eq+1:])
		v = strings.TrimSuffix(strings.TrimPrefix(v, `"`), `"`)
		v = strings.TrimSuffix(strings.TrimPrefix(v, `'`), `'`)
		out[k] = v
	}
	return out
}

// deployArtifacts is the curated catalog. Adding a new tool /flavor =
// append to this list. The text bodies are intentionally inline so
// the user can copy them into version control unchanged.
func deployArtifacts() []DeployArtifact {
	return []DeployArtifact{
		// ── KubeWatcher Agent — Helm ───────────────────────────────
		{
			Tool:        "kubewatcher-agent",
			Flavor:      "helm",
			Description: "Install the KubeWatcher in-cluster agent via the bundled Helm chart.",
			CommandText: `helm upgrade --install kubewatcher-agent ./deploy/helm/kubewatcher-agent \
  --namespace kubewatcher --create-namespace \
  --set image.repository=ghcr.io/drewjocham/kubewatcher-agent \
  --set image.tag=v1.0.0`,
			FileName: "kubewatcher-agent.values.yaml",
			FileText: `# kubewatcher-agent values overrides
image:
  repository: ghcr.io/drewjocham/kubewatcher-agent
  tag: v1.0.0
env:
  saasServerURL: http://kubewatcher-backend:8080
  # Set saasToken via 'helm install --set' or a Secret to keep it out of source control.
resources:
  requests: { cpu: 50m, memory: 64Mi }
  limits:   { cpu: 200m, memory: 256Mi }
`,
			EnvVars: []EnvVarSpec{
				{Name: "KUBEWATCHER_AGENT_TOKEN", Required: true, Hint: "Bearer token the agent uses to call the SaaS server."},
				{Name: "KUBEWATCHER_NAMESPACE", Required: false, Hint: "Namespace the agent deploys into.", Default: "kubewatcher"},
			},
		},
		// ── KubeWatcher Agent — Docker run ─────────────────────────
		{
			Tool:        "kubewatcher-agent",
			Flavor:      "docker",
			Description: "Run the agent locally as a one-shot docker container (development only).",
			CommandText: `docker run --rm \
  -e KUBEWATCHER_AGENT_TOKEN=$KUBEWATCHER_AGENT_TOKEN \
  -e SAAS_SERVER_URL=http://host.docker.internal:8080 \
  -v $HOME/.kube/config:/root/.kube/config:ro \
  ghcr.io/drewjocham/kubewatcher-agent:v1.0.0`,
			EnvVars: []EnvVarSpec{
				{Name: "KUBEWATCHER_AGENT_TOKEN", Required: true, Hint: "Bearer token for the SaaS server."},
				{Name: "SAAS_SERVER_URL", Required: false, Hint: "URL the agent connects to.", Default: "http://host.docker.internal:8080"},
			},
		},
		// ── Argus Scan (Popeye) — Docker run ───────────────────────
		{
			Tool:        "popeye",
			Flavor:      "docker",
			Description: "Run Argus Scan against your current kubeconfig.",
			CommandText: `docker run --rm \
  -v $HOME/.kube/config:/root/.kube/config:ro \
  derailed/popeye:v0.22.0 -A`,
			EnvVars: []EnvVarSpec{},
		},
		// ── Monitoring stack — Helm ────────────────────────────────
		{
			Tool:        "monitoring",
			Flavor:      "helm",
			Description: "Install kube-prometheus-stack + Grafana via the bundled Helm chart.",
			CommandText: `helm upgrade --install kubewatcher-monitoring ./deploy/helm/kubewatcher-monitoring \
  --namespace kubewatcher --create-namespace \
  --set kube-prometheus-stack.grafana.adminPassword=$GF_SECURITY_ADMIN_PASSWORD`,
			FileName: "kubewatcher-monitoring.values.yaml",
			FileText: `# Monitoring stack overrides
kube-prometheus-stack:
  prometheus:
    prometheusSpec:
      retention: 15d
  grafana:
    adminPassword: ""    # Set via --set or sealed-secret; never commit.
    persistence:
      enabled: true
      size: 10Gi
`,
			EnvVars: []EnvVarSpec{
				{Name: "GF_SECURITY_ADMIN_PASSWORD", Required: true, Hint: "Initial Grafana admin password. Rotate after first login."},
			},
		},
		// ── docker-compose stack ───────────────────────────────────
		{
			Tool:        "kubewatcher-stack",
			Flavor:      "compose",
			Description: "Local docker-compose stack: backend + frontend + Prometheus + Grafana.",
			CommandText: `docker compose -f docker-compose.kubewatcher.yaml up -d`,
			FileName:    "docker-compose.kubewatcher.yaml",
			FileText: `# Local docker-compose stack for KubeWatcher.
# Pre-flight:
#   1. cp .env.example .env  and fill in the secrets
#   2. docker compose -f docker-compose.kubewatcher.yaml up -d
services:
  kubewatcher-backend:
    image: ghcr.io/drewjocham/kubewatcher-backend:v1.0.0
    ports: ["8080:8080"]
    environment:
      KUBEWATCHER_TIER: ${KUBEWATCHER_TIER:-pro}
      DEEPSEEK_API_KEY: ${DEEPSEEK_API_KEY}
      KUBEWATCHER_API_BIND: 0.0.0.0
      KUBEWATCHER_API_TOKEN: ${KUBEWATCHER_API_TOKEN}
    volumes:
      - ${HOME}/.kube/config:/root/.kube/config:ro
  kubewatcher-frontend:
    image: ghcr.io/drewjocham/kubewatcher-frontend:v1.0.0
    ports: ["8081:8080"]
    environment:
      BACKEND_URL: http://kubewatcher-backend:8080
    depends_on: [kubewatcher-backend]
  prometheus:
    image: prom/prometheus:v2.55.0
    ports: ["9090:9090"]
  grafana:
    image: grafana/grafana:11.2.0
    ports: ["3000:3000"]
    environment:
      GF_SECURITY_ADMIN_PASSWORD: ${GF_SECURITY_ADMIN_PASSWORD}
`,
			EnvVars: []EnvVarSpec{
				{Name: "DEEPSEEK_API_KEY", Required: true, Hint: "API key for the AI features. Leave blank to disable."},
				{Name: "KUBEWATCHER_API_TOKEN", Required: true, Hint: "Service token so the frontend can hit the API."},
				{Name: "GF_SECURITY_ADMIN_PASSWORD", Required: true, Hint: "Initial Grafana admin password."},
				{Name: "KUBEWATCHER_TIER", Required: false, Hint: "free | pro", Default: "pro"},
			},
		},
	}
}
