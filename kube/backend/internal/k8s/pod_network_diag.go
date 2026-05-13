package k8s

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// --- Public types exposed to the frontend ---

type PodNetworkInfo struct {
	Name            string          `json:"name"`
	Namespace       string          `json:"namespace"`
	PodIP           string          `json:"podIP"`
	HostIP          string          `json:"hostIP"`
	Node            string          `json:"node"`
	HostNetwork     bool            `json:"hostNetwork"`
	Containers      []string        `json:"containers"`
	CNIAnnotation   string          `json:"cniAnnotation,omitempty"`
	NetworkPolicies []AppliedPolicy `json:"networkPolicies"`
}

type AppliedPolicy struct {
	Name        string `json:"name"`
	Direction   string `json:"direction"`
	PodSelector string `json:"podSelector"`
}

type PodDNSResult struct {
	Hostname  string `json:"hostname"`
	Resolved  bool   `json:"resolved"`
	Addresses string `json:"addresses,omitempty"`
	Method    string `json:"method"`
	Error     string `json:"error,omitempty"`
}

type PodConnectivityResult struct {
	Target     string `json:"target"`
	Port       int    `json:"port"`
	Reachable  bool   `json:"reachable"`
	DurationMs int64  `json:"durationMs,omitempty"`
	Error      string `json:"error,omitempty"`
}

type CNIDaemonSetStatus struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Desired   int32  `json:"desired"`
	Ready     int32  `json:"ready"`
}

type CNIStatusResult struct {
	Plugin     string              `json:"plugin"`
	Healthy    bool                `json:"healthy"`
	DaemonSets []CNIDaemonSetStatus `json:"daemonSets"`
	Error      string              `json:"error,omitempty"`
}

type PodDiagSummary struct {
	NetworkInfo  *PodNetworkInfo        `json:"networkInfo,omitempty"`
	DNSResult    *PodDNSResult          `json:"dnsResult,omitempty"`
	Connectivity *PodConnectivityResult `json:"connectivity,omitempty"`
	CNIStatus    *CNIStatusResult       `json:"cniStatus,omitempty"`
}

// --- execInPod runs a one-shot command inside a pod and returns stdout+stderr ---

// runExec is the in-pod command executor used by every diagnostic. It is
// a field rather than a hard call to (*Client).execInPodReal so tests
// can inject a fake that returns canned stdout — exec'ing into a pod
// from a unit test is not practical (needs a real kubelet) and was the
// reason this whole file had zero coverage before. NewClient wires the
// real implementation; the QA review specifically called this out as
// the testability bottleneck.
//
// The signature matches what every caller in this file uses; the
// container-resolution helper (when container == "") still lives on
// the real method so tests don't have to re-implement it.
type podExecFn func(ctx context.Context, namespace, podName, container string, cmd []string) (string, error)

func (c *Client) execInPod(ctx context.Context, namespace, podName, container string, cmd []string) (string, error) {
	if c.runExec != nil {
		return c.runExec(ctx, namespace, podName, container, cmd)
	}
	return c.execInPodReal(ctx, namespace, podName, container, cmd)
}

func (c *Client) execInPodReal(ctx context.Context, namespace, podName, container string, cmd []string) (string, error) {
	if container == "" {
		pod, err := c.cs.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("get pod: %w", err)
		}
		if len(pod.Spec.Containers) > 0 {
			container = pod.Spec.Containers[0].Name
		}
	}

	req := c.cs.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   cmd,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.restCfg, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("create exec: %w", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		combined := strings.TrimSpace(stderr.String())
		if combined == "" {
			combined = err.Error()
		}
		return strings.TrimSpace(stdout.String()), fmt.Errorf("%s", combined)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// --- Pod Network Info (K8s API, no exec) ---

func (c *Client) GetPodNetworkInfo(ctx context.Context, namespace, podName string) (*PodNetworkInfo, error) {
	pod, err := c.cs.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get pod: %w", err)
	}

	info := &PodNetworkInfo{
		Name:        pod.Name,
		Namespace:   pod.Namespace,
		PodIP:       pod.Status.PodIP,
		HostIP:      pod.Status.HostIP,
		Node:        pod.Spec.NodeName,
		HostNetwork: pod.Spec.HostNetwork,
	}

	for _, ctr := range pod.Spec.Containers {
		info.Containers = append(info.Containers, ctr.Name)
	}

	// Detect CNI from pod annotations.
	if a, ok := pod.Annotations["cni.projectcalico.org/podIP"]; ok {
		info.CNIAnnotation = fmt.Sprintf("Calico: %s", a)
	} else if a, ok := pod.Annotations["k8s.v1.cni.cncf.io/networks"]; ok {
		info.CNIAnnotation = fmt.Sprintf("Multus: %s", a)
	} else if a, ok := pod.Annotations["kube-router.io/pod.name"]; ok {
		info.CNIAnnotation = fmt.Sprintf("Kube-Router: %s", a)
	}

	policies, err := c.getAppliedPolicies(ctx, namespace, pod.Labels)
	if err != nil {
		c.logger.WarnContext(ctx, "failed to resolve network policies", "pod", podName, "error", err)
	} else {
		info.NetworkPolicies = policies
	}

	return info, nil
}

// getAppliedPolicies finds all NetworkPolicies whose podSelector matches the given labels.
func (c *Client) getAppliedPolicies(ctx context.Context, namespace string, podLabels map[string]string) ([]AppliedPolicy, error) {
	list, err := c.cs.NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []AppliedPolicy
	for i := range list.Items {
		np := &list.Items[i]
		sel, err := metav1.LabelSelectorAsSelector(&np.Spec.PodSelector)
		if err != nil {
			continue
		}
		if !sel.Matches(labels.Set(podLabels)) {
			continue
		}

		dir := "ingress"
		var types []string
		for _, pt := range np.Spec.PolicyTypes {
			types = append(types, strings.ToLower(string(pt)))
		}
		if len(types) > 1 {
			dir = strings.Join(types, "+")
		} else if len(types) == 1 {
			dir = types[0]
		}

		selectorStr := ""
		if len(np.Spec.PodSelector.MatchLabels) > 0 {
			parts := make([]string, 0, len(np.Spec.PodSelector.MatchLabels))
			for k, v := range np.Spec.PodSelector.MatchLabels {
				parts = append(parts, fmt.Sprintf("%s=%s", k, v))
			}
			selectorStr = strings.Join(parts, ", ")
		} else {
			selectorStr = "<all pods>"
		}

		result = append(result, AppliedPolicy{
			Name:        np.Name,
			Direction:   dir,
			PodSelector: selectorStr,
		})
	}
	return result, nil
}

// --- DNS Check (exec-based) ---

// dnsToolProbe is the script we run first to discover which DNS tool the
// target container actually has. Many production images (distroless,
// busybox-minimal, scratch+app) ship none of getent/nslookup/host/dig;
// the previous version chained them with `||` and reported a confusing
// "no output" message when they all failed silently. Now we detect
// availability up-front and surface a precise "no DNS tool available"
// when nothing is on PATH. The script prints exactly the name of the
// first tool it finds, or `none`.
const dnsToolProbe = `if command -v getent >/dev/null 2>&1; then echo getent; ` +
	`elif command -v nslookup >/dev/null 2>&1; then echo nslookup; ` +
	`elif command -v host >/dev/null 2>&1; then echo host; ` +
	`elif command -v dig >/dev/null 2>&1; then echo dig; ` +
	`else echo none; fi`

func (c *Client) RunPodDNSCheck(ctx context.Context, namespace, podName, hostname string) (*PodDNSResult, error) {
	if hostname == "" {
		hostname = "kubernetes.default.svc.cluster.local"
	}

	res := &PodDNSResult{Hostname: hostname}

	// Step 1 — discover which DNS tool exists.
	tool, err := c.execInPod(ctx, namespace, podName, "", []string{"sh", "-c", dnsToolProbe})
	if err != nil {
		res.Error = err.Error()
		return res, nil
	}
	tool = strings.TrimSpace(tool)
	if tool == "" || tool == "none" {
		res.Error = "no DNS resolution tool available in pod (getent/nslookup/host/dig all missing)"
		return res, nil
	}
	res.Method = tool

	// Step 2 — run the actual lookup with the discovered tool. The
	// commands are escaped via printf %q-style single-quoting since
	// hostname is user-controlled.
	q := shellQuote(hostname)
	var lookupCmd string
	switch tool {
	case "getent":
		lookupCmd = "getent hosts " + q
	case "nslookup":
		lookupCmd = "nslookup " + q + " 2>&1 | tail -10"
	case "host":
		lookupCmd = "host " + q + " 2>&1"
	case "dig":
		lookupCmd = "dig +short " + q + " 2>&1"
	default:
		res.Error = "unrecognized DNS tool: " + tool
		return res, nil
	}

	out, err := c.execInPod(ctx, namespace, podName, "", []string{"sh", "-c", lookupCmd})
	if err != nil {
		res.Error = err.Error()
		return res, nil
	}

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if res.Addresses == "" {
			res.Addresses = line
		} else {
			res.Addresses += "\n" + line
		}
	}

	res.Resolved = !strings.Contains(out, "NXDOMAIN") &&
		!strings.Contains(out, "not found") &&
		!strings.Contains(out, "SERVFAIL") &&
		strings.TrimSpace(res.Addresses) != ""

	return res, nil
}

// shellQuote single-quotes a value for safe inclusion in a `sh -c '...'`
// command. Escapes embedded single quotes via the standard `'\''`
// trick. The previous code used fmt.Sprintf with bare `'%s'` which
// breaks the moment hostname contains a quote.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// --- Connectivity Check (exec-based) ---

// connectivityProbe is the in-pod TCP-reachability script. Three changes
// from the previous version that the QA review called out:
//
//  1. `/dev/tcp/host/port` was tried FIRST. That's a bash-builtin — on
//     Alpine and most distroless/minimal images the default shell is
//     ash/busybox, which doesn't implement it. The expansion failed
//     non-fatally and we fell through, but the resulting stderr was
//     confusing ("/dev/tcp/host/port: No such file or directory") and
//     looked like a real network failure in logs. Now `nc` is tried
//     first (busybox ships it on every image we've seen), `/dev/tcp`
//     is the LAST-resort fallback gated on bash being available.
//  2. The bare nc invocation now uses `-w 3` (connect timeout) +
//     `-z` (no data, just probe) consistently across both branches.
//  3. The script prints a single deterministic token (`OK` / `UNREACHABLE`)
//     so we don't have to grep for vendor-specific connection messages
//     across BSD-nc / GNU-nc / busybox-nc.
const connectivityProbe = `set -e
target="$1"; port="$2"
if command -v nc >/dev/null 2>&1; then
  if nc -z -w 3 "$target" "$port" 2>/dev/null; then echo OK; exit 0; fi
elif command -v bash >/dev/null 2>&1; then
  if bash -c "timeout 5 bash -c '</dev/tcp/$target/$port'" 2>/dev/null; then echo OK; exit 0; fi
fi
echo UNREACHABLE`

func (c *Client) RunPodConnectivityCheck(ctx context.Context, namespace, podName, target string, port int) (*PodConnectivityResult, error) {
	res := &PodConnectivityResult{
		Target: target,
		Port:   port,
	}

	// Port validation — the previous code passed unchecked `int` into
	// shell interpolation. A negative or out-of-range value would
	// either crash nc or, worse, produce a nonsense command line that
	// silently failed.
	if port < 1 || port > 65535 {
		res.Error = fmt.Sprintf("invalid port %d (must be 1..65535)", port)
		return res, nil
	}
	if strings.TrimSpace(target) == "" {
		res.Error = "target host required"
		return res, nil
	}

	start := time.Now()

	// `sh -c <script> sh "$target" "$port"` form: the "sh" after the
	// script becomes $0 inside the script; target and port land as
	// $1/$2. Avoids any interpolation of attacker-controlled bytes
	// into the script body itself.
	out, err := c.execInPod(ctx, namespace, podName, "", []string{
		"sh", "-c", connectivityProbe, "sh", target, fmt.Sprintf("%d", port),
	})
	if err != nil {
		res.Error = err.Error()
		return res, nil
	}

	res.DurationMs = time.Since(start).Milliseconds()
	res.Reachable = strings.Contains(out, "OK") &&
		!strings.Contains(out, "UNREACHABLE")

	if !res.Reachable {
		res.Error = strings.TrimSpace(out)
	}

	return res, nil
}

// --- CNI Status (K8s API, no exec) ---

func (c *Client) GetCNIStatus(ctx context.Context) (*CNIStatusResult, error) {
	res := &CNIStatusResult{}

	cniDaemonSets := []struct {
		namespace string
		name      string
		plugin    string
	}{
		{"kube-system", "calico-node", "Calico"},
		{"kube-system", "cilium", "Cilium"},
		{"kube-system", "kube-flannel-ds", "Flannel"},
		{"kube-system", "kube-router", "Kube-Router"},
		{"kube-system", "weave-net", "Weave"},
		{"kube-system", "antrea-agent", "Antrea"},
		{"kube-system", "kube-ovn-cni", "Kube-OVN"},
		{"kube-system", "aws-node", "AWS VPC CNI"},
		{"kube-system", "azure-npm", "Azure NPM"},
	}

	// Health rule: ALL discovered CNI DaemonSets must be fully ready.
	// The previous version set healthy=true on the first match and
	// never reset, so a cluster running e.g. Calico+Multus with one
	// degraded showed up as "healthy" — masking the real problem.
	// `foundAny` distinguishes "no CNI DS exists" (unknown) from
	// "we found some and they're all up" (healthy true).
	healthy := true
	foundAny := false
	for _, ds := range cniDaemonSets {
		daemon, err := c.cs.AppsV1().DaemonSets(ds.namespace).Get(ctx, ds.name, metav1.GetOptions{})
		if err != nil {
			continue
		}
		foundAny = true
		status := CNIDaemonSetStatus{
			Name:      ds.name,
			Namespace: ds.namespace,
			Desired:   daemon.Status.DesiredNumberScheduled,
			Ready:     daemon.Status.NumberReady,
		}
		res.DaemonSets = append(res.DaemonSets, status)
		if res.Plugin == "" {
			res.Plugin = ds.plugin
		}
		if status.Desired == 0 || status.Ready != status.Desired {
			healthy = false
		}
	}

	res.Healthy = foundAny && healthy
	if !foundAny {
		res.Plugin = "unknown"
		res.Error = "no known CNI DaemonSet found"
	}

	return res, nil
}

// --- Run all diagnostics at once ---

func (c *Client) RunPodNetworkDiagnostics(ctx context.Context, namespace, podName, targetHost string, targetPort int) (*PodDiagSummary, error) {
	summary := &PodDiagSummary{}

	info, err := c.GetPodNetworkInfo(ctx, namespace, podName)
	if err != nil {
		c.logger.WarnContext(ctx, "pod network info failed", "pod", podName, "error", err)
	} else {
		summary.NetworkInfo = info
	}

	dns, err := c.RunPodDNSCheck(ctx, namespace, podName, targetHost)
	if err != nil {
		c.logger.WarnContext(ctx, "dns check failed", "pod", podName, "error", err)
	} else {
		summary.DNSResult = dns
	}

	if targetHost != "" && targetPort > 0 {
		conn, err := c.RunPodConnectivityCheck(ctx, namespace, podName, targetHost, targetPort)
		if err != nil {
			c.logger.WarnContext(ctx, "connectivity check failed", "pod", podName, "error", err)
		} else {
			summary.Connectivity = conn
		}
	}

	cni, err := c.GetCNIStatus(ctx)
	if err != nil {
		c.logger.WarnContext(ctx, "cni status check failed", "error", err)
	} else {
		summary.CNIStatus = cni
	}

	return summary, nil
}
