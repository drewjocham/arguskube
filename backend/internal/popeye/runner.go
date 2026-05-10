package popeye

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var (
	ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	// K9s-style banner: lines starting with decorative ASCII before JSON begins.
	// Popeye sometimes prints its ASCII logo banner before the JSON output.
	jsonStartRE = regexp.MustCompile(`(?s)^.*?(\{.*)$`)
)

type SeverityLevel int

const (
	SevOK   SeverityLevel = 0
	SevInfo SeverityLevel = 1
	SevWarn SeverityLevel = 2
	SevErr  SeverityLevel = 3
)

func (s SeverityLevel) String() string {
	switch s {
	case SevOK:
		return "ok"
	case SevInfo:
		return "info"
	case SevWarn:
		return "warning"
	case SevErr:
		return "error"
	}
	return "unknown"
}

func (s SeverityLevel) Color() string {
	switch s {
	case SevOK:
		return "green"
	case SevInfo:
		return "blue"
	case SevWarn:
		return "amber"
	case SevErr:
		return "red"
	}
	return "gray"
}

// Finding is a single Popeye finding with explanation.
type Finding struct {
	ID          string `json:"id"`
	Resource    string `json:"resource"`
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Severity    string `json:"severity"`
	SevLevel    int    `json:"sevLevel"`
	Message     string `json:"message"`
	Explanation string `json:"explanation"`
	Fix         string `json:"fix"`
	Command     string `json:"command"`
}

type Report struct {
	Timestamp   time.Time `json:"timestamp"`
	Score       int       `json:"score"`
	Grade       string    `json:"grade"`
	Findings    []Finding `json:"findings"`
	TotalOK     int       `json:"totalOk"`
	TotalInfo   int       `json:"totalInfo"`
	TotalWarn   int       `json:"totalWarn"`
	TotalError  int       `json:"totalError"`
	ScanTimeMs  int64     `json:"scanTimeMs"`
	ClusterName string    `json:"clusterName"`
}

type popeyeJSON struct {
	Popeye struct {
		Score    int    `json:"score"`
		Grade    string `json:"grade"`
		Sections []struct {
			Linter string `json:"linter"` // resource type: "pod", "svc", etc.
			Tally  struct {
				OK   int `json:"ok"`
				Info int `json:"info"`
				Warn int `json:"warning"`
				Err  int `json:"error"`
			} `json:"tally"`
			Issues map[string][]struct {
				Group   string `json:"group"`
				Level   int    `json:"level"`
				Message string `json:"message"`
			} `json:"issues"`
		} `json:"sections"`
	} `json:"popeye"`
}

type Runner struct {
	binary     string
	kubeconfig string
	context    string
	namespace  string
	logger     *slog.Logger
}

func NewRunner(binary, kubeconfig, kubecontext, namespace string, logger *slog.Logger) *Runner {
	return &Runner{
		binary:     binary,
		kubeconfig: kubeconfig,
		context:    kubecontext,
		namespace:  namespace,
		logger:     logger,
	}
}

func (r *Runner) Run(ctx context.Context) (*Report, error) {
	start := time.Now()

	// Try local binary first, fall back to Docker.
	output, err := r.execPopeye(ctx)
	if err != nil {
		return nil, fmt.Errorf("popeye exec: %w", err)
	}
	if len(output) == 0 {
		return nil, fmt.Errorf("popeye returned empty output — is the cluster reachable?")
	}

	elapsed := time.Since(start)

	var raw popeyeJSON
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("popeye json parse: %w (output: %s)", err, truncateBytes(output, 200))
	}

	report := &Report{
		Timestamp:  time.Now(),
		Score:      raw.Popeye.Score,
		Grade:      raw.Popeye.Grade,
		ScanTimeMs: elapsed.Milliseconds(),
	}

	findingID := 0
	for _, san := range raw.Popeye.Sections {
		report.TotalOK += san.Tally.OK
		report.TotalInfo += san.Tally.Info
		report.TotalWarn += san.Tally.Warn
		report.TotalError += san.Tally.Err

		for resourceName, issues := range san.Issues {
			ns, name := splitResource(resourceName)
			for _, issue := range issues {
				findingID++
				sev := SeverityLevel(issue.Level)
				f := Finding{
					ID:        fmt.Sprintf("pop-%d", findingID),
					Resource:  san.Linter,
					Name:      name,
					Namespace: ns,
					Severity:  sev.String(),
					SevLevel:  issue.Level,
					Message:   issue.Message,
				}
				f.Explanation = explainFinding(san.Linter, issue.Message, sev)
				f.Fix = suggestFix(san.Linter, issue.Message, ns, name)
				f.Command = suggestCommand(san.Linter, issue.Message, ns, name)
				report.Findings = append(report.Findings, f)
			}
		}
	}

	r.logger.InfoContext(ctx, "popeye scan complete",
		slog.Int("score", report.Score),
		slog.String("grade", report.Grade),
		slog.Int("findings", len(report.Findings)),
		slog.Int64("scanMs", report.ScanTimeMs),
	)

	return report, nil
}

func explainFinding(resource, message string, sev SeverityLevel) string {
	lower := strings.ToLower(message)

	switch {
	case strings.Contains(lower, "no probes"):
		return "This container has no liveness or readiness probes configured. " +
			"Without probes, Kubernetes cannot detect if your application is healthy or ready to serve traffic. " +
			"This means unhealthy pods won't be restarted, and traffic may be routed to pods that can't handle it. " +
			"In production, this can lead to silent failures and degraded user experience."

	case strings.Contains(lower, "no resources"):
		return "This container has no CPU or memory resource requests/limits set. " +
			"Without resource definitions, the scheduler cannot make informed placement decisions, " +
			"and the container can consume unbounded resources — starving other workloads. " +
			"This also prevents HPA from functioning and makes capacity planning impossible."

	case strings.Contains(lower, "cpu limit"):
		return "The CPU limit for this container is either missing or misconfigured. " +
			"Without CPU limits, a single pod can monopolize node CPU, causing throttling for colocated workloads. " +
			"Setting appropriate limits ensures fair scheduling and prevents noisy-neighbor problems."

	case strings.Contains(lower, "memory limit"):
		return "The memory limit for this container is either missing or misconfigured. " +
			"Without memory limits, a memory leak can consume all node memory, triggering OOMKills on other pods. " +
			"Always set memory limits slightly above your expected peak usage."

	case strings.Contains(lower, "image tag") || strings.Contains(lower, ":latest"):
		return "This container uses the ':latest' tag or no specific image tag. " +
			"Using ':latest' makes deployments non-reproducible — you can't tell which version is running, " +
			"rollbacks become unreliable, and different nodes may pull different versions. " +
			"Always pin to a specific tag or digest."

	case strings.Contains(lower, "service account"):
		return "This pod is using the default service account, which may have broader permissions than needed. " +
			"Create a dedicated service account with minimal RBAC permissions for this workload " +
			"to follow the principle of least privilege."

	case strings.Contains(lower, "security context") || strings.Contains(lower, "root"):
		return "This container is running without a restricted security context or is running as root. " +
			"Running as root inside a container increases the blast radius of a container escape. " +
			"Set runAsNonRoot: true and drop unnecessary capabilities."

	case strings.Contains(lower, "replicas") || strings.Contains(lower, "single replica"):
		return "This deployment is running with only one replica, creating a single point of failure. " +
			"If this pod crashes or the node goes down, there will be downtime until a new pod starts. " +
			"Run at least 2 replicas for any production workload."

	case strings.Contains(lower, "unused") || strings.Contains(lower, "not used"):
		return "This resource exists in the cluster but is not referenced by any workload. " +
			"Unused resources add clutter, make auditing harder, and may hold secrets or permissions " +
			"that could be exploited. Clean up resources that are no longer needed."

	case strings.Contains(lower, "immutable"):
		return "This ConfigMap or Secret is not marked as immutable. " +
			"Immutable resources are protected from accidental changes and are more efficiently handled by the API server. " +
			"Mark stable configs as immutable to prevent accidental modifications in production."

	default:
		if sev >= SevWarn {
			return fmt.Sprintf("Popeye detected an issue with this %s resource. "+
				"Review the finding and assess whether it impacts reliability, security, or performance in your environment.", resource)
		}
		return fmt.Sprintf("This %s resource passed the check or has a minor informational note.", resource)
	}
}

func suggestFix(_, message, _, _ string) string {
	lower := strings.ToLower(message)

	switch {
	case strings.Contains(lower, "no probes"):
		return "Add liveness and readiness probes to your container spec. " +
			"Use httpGet for HTTP services, tcpSocket for TCP services, or exec for custom checks. " +
			"Start with generous timeouts (initialDelaySeconds: 15, periodSeconds: 10) and tune from there."

	case strings.Contains(lower, "no resources"):
		return "Add resource requests and limits to your container spec. " +
			"Start with requests based on observed usage (kubectl top) and set limits at 1.5-2x requests. " +
			"Use a VPA recommendation or load test to fine-tune."

	case strings.Contains(lower, ":latest"):
		return "Pin the image to a specific version tag or SHA digest. " +
			"Update your CI/CD pipeline to tag images with the git commit SHA or semantic version."

	case strings.Contains(lower, "security context") || strings.Contains(lower, "root"):
		return "Add a securityContext to your pod/container spec with: " +
			"runAsNonRoot: true, readOnlyRootFilesystem: true, and allowPrivilegeEscalation: false. " +
			"Drop all capabilities and add only what's needed."

	case strings.Contains(lower, "single replica") || strings.Contains(lower, "replicas"):
		return "Increase the replica count to at least 2 for production workloads. " +
			"Configure a PodDisruptionBudget to ensure availability during node maintenance."

	default:
		return "Review the Popeye documentation for detailed remediation guidance for this finding type."
	}
}

func suggestCommand(resource, message, ns, name string) string {
	lower := strings.ToLower(message)

	if ns == "" {
		ns = "default"
	}

	switch {
	case strings.Contains(lower, "no probes") || strings.Contains(lower, "no resources"):
		return fmt.Sprintf("kubectl edit %s %s -n %s", singularResource(resource), name, ns)

	case strings.Contains(lower, ":latest"):
		return fmt.Sprintf("kubectl get %s %s -n %s -o jsonpath='{.spec.template.spec.containers[*].image}'", singularResource(resource), name, ns)

	case strings.Contains(lower, "unused") || strings.Contains(lower, "not used"):
		return fmt.Sprintf("kubectl delete %s %s -n %s", singularResource(resource), name, ns)

	case strings.Contains(lower, "security context") || strings.Contains(lower, "root"):
		return fmt.Sprintf("kubectl get %s %s -n %s -o jsonpath='{.spec.template.spec.securityContext}'", singularResource(resource), name, ns)

	default:
		return fmt.Sprintf("kubectl describe %s %s -n %s", singularResource(resource), name, ns)
	}
}

func (r *Runner) execPopeye(ctx context.Context) ([]byte, error) {
	binaryAvailable := false
	if _, err := exec.LookPath(r.binary); err == nil {
		binaryAvailable = true
		output, err := r.execBinary(ctx)
		if err == nil && len(output) > 0 {
			return output, nil
		}
		// Binary failed or returned no output — try docker before giving up.
		r.logger.Warn("popeye binary attempt did not yield output, falling back to docker",
			slog.String("binary", r.binary),
			slog.Any("error", err),
		)
		if dockerPath, derr := exec.LookPath("docker"); derr == nil {
			out, dockerErr := r.execDocker(ctx, dockerPath)
			if dockerErr == nil && len(out) > 0 {
				return out, nil
			}
			// Both binary and docker failed — return the most informative error we have.
			if err != nil {
				return nil, fmt.Errorf("popeye binary failed (%w); docker fallback also failed: %v", err, dockerErr)
			}
			return nil, fmt.Errorf("popeye produced no output; docker fallback also failed: %w", dockerErr)
		}
		// No docker; return the binary error if any, else a clear empty-output error.
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("popeye produced no output (binary: %s) — verify the cluster is reachable: kubectl get ns", r.binary)
	}

	if dockerPath, err := exec.LookPath("docker"); err == nil {
		return r.execDocker(ctx, dockerPath)
	}

	if binaryAvailable {
		return nil, fmt.Errorf("popeye binary present but failed; install Docker as a fallback or verify cluster connectivity")
	}
	return nil, fmt.Errorf("popeye not found: install the binary (go install github.com/derailed/popeye@latest) or Docker")
}

func (r *Runner) execBinary(ctx context.Context) ([]byte, error) {
	// Don't pass --no-color: popeye 0.22+ removed that flag and rejects it with
	// "unknown flag" before producing any output. stripANSI() below cleans any
	// escapes that leak through.
	args := []string{"--out", "json", "--force-exit-zero"}
	// Multi-file kubeconfig (colon-separated) can't be passed via --kubeconfig flag.
	// Set the KUBECONFIG env var instead and only use --kubeconfig for single files.
	var envKubeconfig string
	if r.kubeconfig != "" {
		if strings.Contains(r.kubeconfig, ":") {
			envKubeconfig = r.kubeconfig
		} else {
			args = append(args, "--kubeconfig", r.kubeconfig)
		}
	}
	if r.context != "" {
		args = append(args, "--context", r.context)
	}
	if r.namespace != "" {
		args = append(args, "--namespace", r.namespace)
	}

	r.logger.Info("running popeye scan via binary",
		slog.String("binary", r.binary),
		slog.String("namespace", r.namespace),
	)

	cmd := exec.CommandContext(ctx, r.binary, args...)
	if envKubeconfig != "" {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+envKubeconfig)
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	output, err := cmd.Output()
	stderr := stripANSI(stderrBuf.Bytes())
	if err != nil && len(output) == 0 {
		if len(stderr) > 0 {
			return nil, fmt.Errorf("popeye exec: %w (stderr: %s)", err, truncateBytes(stderr, 400))
		}
		return nil, fmt.Errorf("popeye exec: %w", err)
	}
	if len(stderr) > 0 {
		r.logger.Warn("popeye stderr output",
			slog.String("stderr", truncateBytes(stderr, 500)),
		)
	}
	// Strip any ANSI escape codes that leak through despite --no-color.
	output = stripANSI(output)
	// Strip any banner output before the JSON document (popeye 0.22+ prints
	// a colored ASCII logo even with --no-color in some environments).
	trimmed := bytes.TrimSpace(output)
	if len(trimmed) > 0 && trimmed[0] != '{' {
		if matches := jsonStartRE.FindSubmatch(output); len(matches) >= 2 {
			output = matches[1]
			trimmed = bytes.TrimSpace(output)
		}
	}
	if len(trimmed) == 0 || trimmed[0] != '{' {
		hint := "verify cluster connectivity (kubectl get ns) and that the kubecontext is valid"
		if len(stderr) > 0 {
			return nil, fmt.Errorf("popeye produced no parseable output (%s); stderr: %s", hint, truncateBytes(stderr, 400))
		}
		return nil, fmt.Errorf("popeye produced no parseable output (%s)", hint)
	}
	return output, nil
}

func (r *Runner) execDocker(ctx context.Context, dockerPath string) ([]byte, error) {
	kubeconfigPath := r.kubeconfig
	if kubeconfigPath == "" {
		kubeconfigPath = homeDir() + "/.kube/config"
	}

	// Build docker run args — mount kubeconfig and pass popeye flags.
	args := []string{
		"run", "--rm", "--network", "host",
		"-v", kubeconfigPath + ":/root/.kube/config:ro",
		"quay.io/derailed/popeye",
		"--out", "json", "--force-exit-zero",
	}
	if r.context != "" {
		args = append(args, "--context", r.context)
	}
	if r.namespace != "" {
		args = append(args, "--namespace", r.namespace)
	}

	r.logger.Info("running popeye scan via docker",
		slog.String("kubeconfig", kubeconfigPath),
		slog.String("namespace", r.namespace),
	)

	cmd := exec.CommandContext(ctx, dockerPath, args...)
	output, err := cmd.Output()
	if err != nil && len(output) == 0 {
		return nil, fmt.Errorf("popeye docker exec: %w", err)
	}
	return output, nil
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return "."
}

func splitResource(fullName string) (namespace, name string) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", parts[0]
}

func singularResource(plural string) string {
	mapping := map[string]string{
		"pod": "pod", "po": "pod",
		"svc": "svc", "service": "svc",
		"dp": "deploy", "deploy": "deploy", "deployment": "deploy",
		"ds": "ds", "daemonset": "ds",
		"sts": "sts", "statefulset": "sts",
		"cm": "cm", "configmap": "cm",
		"sec": "secret", "secret": "secret",
		"sa": "sa", "serviceaccount": "sa",
		"ns": "ns", "namespace": "ns",
		"no": "node", "node": "node",
		"pv": "pv", "pvc": "pvc",
		"rs": "rs", "replicaset": "rs",
		"ing": "ing", "ingress": "ing",
		"np": "netpol", "networkpolicy": "netpol",
		"cr": "clusterrole", "crb": "clusterrolebinding",
		"rb": "rolebinding", "ro": "role",
	}
	if s, ok := mapping[strings.ToLower(plural)]; ok {
		return s
	}
	return plural
}

func stripANSI(b []byte) []byte {
	return ansiRE.ReplaceAll(b, nil)
}

func truncateBytes(b []byte, max int) string {
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "..."
}
