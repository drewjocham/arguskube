package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"k8s.io/client-go/rest"
)

type metricsServerProvider struct {
	client  *http.Client
	baseURL string
	cs      *Client
	logger  *slog.Logger
	cfg     providerConfig
}

type providerConfig struct {
	defaultNamespace string
}

func newMetricsServerProvider(c *Client, logger *slog.Logger) *metricsServerProvider {
	httpClient, err := restHTTPClient(c.restCfg)
	if err != nil {
		logger.Warn("failed to create metrics-server http client", slog.String("err", err.Error()))
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	ns := ""
	if c.cfg != nil {
		ns = c.cfg.Kubernetes.Namespace
	}

	return &metricsServerProvider{
		client:  httpClient,
		baseURL: c.restCfg.Host,
		cs:      c,
		logger:  logger.With(slog.String("metrics_provider", "metrics-server")),
		cfg:     providerConfig{defaultNamespace: ns},
	}
}

func (p *metricsServerProvider) QueryPromQL(ctx context.Context, promql string, start, end time.Time, step time.Duration) ([]float64, error) {
	return p.QueryTimeSeries(ctx, promql, 100)
}

func (p *metricsServerProvider) QueryTimeSeries(ctx context.Context, query string, points int) ([]float64, error) {
	isCPU := containsAny(query, "cpu", "CPU")
	isMem := containsAny(query, "memory", "Memory", "mem")
	isNodeQuery := containsAny(query, "node")
	isPodQuery := containsAny(query, "_pod_")

	path := p.resolvePath(query, isNodeQuery, isPodQuery)
	if path == "" {
		return p.deriveMetrics(ctx, query, points)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, p.baseURL+path, nil)
	if err != nil {
		return p.deriveMetrics(ctx, query, points)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return p.deriveMetrics(ctx, query, points)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return p.deriveMetrics(ctx, query, points)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return p.deriveMetrics(ctx, query, points)
	}

	var values []float64
	if isNodeQuery {
		values = parseNodeMetrics(body, isCPU, isMem)
	} else if isPodQuery {
		_, podName := parsePodQuery(query)
		values = parseSinglePodMetrics(body, isCPU, isMem, podName)
	} else {
		values = parsePodMetrics(body, isCPU, isMem)
	}

	if len(values) == 0 {
		return p.deriveMetrics(ctx, query, points)
	}

	return values, nil
}

func (p *metricsServerProvider) resolvePath(query string, isNode, isPod bool) string {
	if isNode {
		return "/apis/metrics.k8s.io/v1beta1/nodes"
	}
	if isPod {
		ns, podName := parsePodQuery(query)
		if ns == "" {
			ns = p.cfg.defaultNamespace
		}
		if ns != "" && podName != "" {
			return fmt.Sprintf("/apis/metrics.k8s.io/v1beta1/namespaces/%s/pods/%s", ns, podName)
		}
		if ns != "" {
			return fmt.Sprintf("/apis/metrics.k8s.io/v1beta1/namespaces/%s/pods", ns)
		}
		return "/apis/metrics.k8s.io/v1beta1/pods"
	}
	ns := p.cfg.defaultNamespace
	if ns != "" {
		return fmt.Sprintf("/apis/metrics.k8s.io/v1beta1/namespaces/%s/pods", ns)
	}
	return "/apis/metrics.k8s.io/v1beta1/pods"
}

func (p *metricsServerProvider) deriveMetrics(ctx context.Context, query string, points int) ([]float64, error) {
	if p.cs == nil {
		return nil, fmt.Errorf("no kubernetes client available for derived metrics")
	}
	return p.cs.deriveCoreMetrics(ctx, query, points)
}

func (p *metricsServerProvider) Healthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/apis/metrics.k8s.io/v1beta1/pods", nil)
	if err != nil {
		return false
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (p *metricsServerProvider) Type() MetricsType {
	return MetricsTypeMetricsServer
}

// --- Parsing helpers (single value, no synthetic jitter) ---

func parsePodMetrics(data []byte, isCPU, isMem bool) []float64 {
	var list podMetricsList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil
	}
	if len(list.Items) == 0 {
		return nil
	}
	var total int64
	for _, pod := range list.Items {
		for _, c := range pod.Containers {
			if isCPU {
				total += parseCPUNanos(c.Usage["cpu"])
			} else if isMem {
				total += parseMemBytes(c.Usage["memory"])
			}
		}
	}
	return singlePct(float64(total), isCPU, isMem)
}

func parseNodeMetrics(data []byte, isCPU, isMem bool) []float64 {
	var list nodeMetricsList
	if err := json.Unmarshal(data, &list); err != nil || len(list.Items) == 0 {
		return nil
	}
	var total int64
	for _, node := range list.Items {
		if isCPU {
			total += parseCPUNanos(node.Usage["cpu"])
		} else if isMem {
			total += parseMemBytes(node.Usage["memory"])
		}
	}
	var pct float64
	if isCPU {
		pct = float64(total) / (float64(len(list.Items)) * 2000.0) * 100
	} else {
		pct = float64(total) / (float64(len(list.Items)) * 8 * 1024 * 1024 * 1024) * 100
	}
	if pct > 100 {
		pct = 100
	}
	return []float64{pct}
}

func parseSinglePodMetrics(data []byte, isCPU, isMem bool, podName string) []float64 {
	var single podMetricsItem
	if err := json.Unmarshal(data, &single); err == nil && single.Metadata.Name != "" {
		var total int64
		for _, c := range single.Containers {
			if isCPU {
				total += parseCPUNanos(c.Usage["cpu"])
			} else if isMem {
				total += parseMemBytes(c.Usage["memory"])
			}
		}
		return singlePct(float64(total), isCPU, isMem)
	}
	var list podMetricsList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil
	}
	for _, pod := range list.Items {
		if pod.Metadata.Name != podName {
			continue
		}
		var total int64
		for _, c := range pod.Containers {
			if isCPU {
				total += parseCPUNanos(c.Usage["cpu"])
			} else if isMem {
				total += parseMemBytes(c.Usage["memory"])
			}
		}
		return singlePct(float64(total), isCPU, isMem)
	}
	return nil
}

func singlePct(v float64, isCPU, isMem bool) []float64 {
	var pct float64
	if isCPU {
		pct = v / 1000.0 * 100
	} else if isMem {
		pct = v / (512 * 1024 * 1024) * 100
	}
	if pct > 100 {
		pct = 100
	}
	return []float64{pct}
}

func restHTTPClient(cfg *rest.Config) (*http.Client, error) {
	transport, err := rest.TransportFor(cfg)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}, nil
}

var _ MetricsProvider = (*metricsServerProvider)(nil)
