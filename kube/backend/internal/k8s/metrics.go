package k8s

import (
	"context"
	"fmt"
	"math"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MetricsPoint struct {
	Value float64 `json:"value"`
}

type podMetricsList struct {
	Items []podMetricsItem `json:"items"`
}

type podMetricsItem struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Containers []containerMetrics `json:"containers"`
}

type containerMetrics struct {
	Name  string            `json:"name"`
	Usage map[string]string `json:"usage"`
}

type nodeMetricsList struct {
	Items []nodeMetricsItem `json:"items"`
}

type nodeMetricsItem struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Usage map[string]string `json:"usage"`
}

func (c *Client) QueryTimeSeriesMetrics(ctx context.Context, query string, timeRange string) ([]float64, error) {
	provider := c.metricsProvider
	if provider == nil {
		return c.deriveCoreMetrics(ctx, query, 100)
	}
	points := parsePoints(timeRange)
	return provider.QueryTimeSeries(ctx, query, points)
}

func (c *Client) QueryPromQL(ctx context.Context, promql string, duration string) ([]float64, error) {
	provider := c.metricsProvider
	if provider == nil {
		return nil, fmt.Errorf("no metrics provider configured (set PROMETHEUS_URL or VICTORIAMETRICS_URL)")
	}

	d, err := time.ParseDuration(duration)
	if err != nil {
		d = 1 * time.Hour
	}

	start := time.Now().Add(-d)
	end := time.Now()
	step := time.Duration(d.Seconds()/100) * time.Second
	if step < 15*time.Second {
		step = 15 * time.Second
	}

	// Delegates to the configured metrics provider (Prometheus /
	// VictoriaMetrics / metrics-server adapter). The previous inline
	// HTTP call against the API-server metrics endpoint was replaced by
	// the provider abstraction on main; the error-wrapping work my
	// branch had on those HTTP call sites no longer applies because the
	// surface is gone — provider.QueryPromQL already returns wrapped
	// errors.
	return provider.QueryPromQL(ctx, promql, start, end, step)
}

func parsePoints(timeRange string) int {
	switch timeRange {
	case "15m", "15 minute", "15 minutes":
		return 15
	case "30m", "30 minute", "30 minutes":
		return 30
	case "1h", "1 hour", "1 hours":
		return 60
	case "6h", "6 hour", "6 hours":
		return 60
	case "24h", "24 hour", "24 hours":
		return 100
	default:
		return 100
	}
}

func (c *Client) deriveCoreMetrics(ctx context.Context, query string, points int) ([]float64, error) {
	isCPU := containsAny(query, "cpu", "CPU")
	isMem := containsAny(query, "memory", "Memory", "mem")
	isNetwork := containsAny(query, "network", "receive", "transmit")
	isPodQuery := containsAny(query, "_pod_")

	if isPodQuery {
		queryNs, podName := parsePodQuery(query)
		if queryNs == "" {
			queryNs = c.cfg.Kubernetes.Namespace
		}
		if queryNs == "" {
			queryNs = "default"
		}
		pod, err := c.cs.CoreV1().Pods(queryNs).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("pod %s/%s: %w", queryNs, podName, err)
		}

		var cpuMillis, memBytes int64
		for _, container := range pod.Spec.Containers {
			if req, ok := container.Resources.Requests["cpu"]; ok {
				cpuMillis += req.MilliValue()
			}
			if req, ok := container.Resources.Requests["memory"]; ok {
				memBytes += req.Value()
			}
		}

		var pct float64
		if isCPU {
			pct = float64(cpuMillis) / 1000.0 * 100
		} else if isMem {
			pct = float64(memBytes) / (512 * 1024 * 1024) * 100
		}
		if pct > 100 {
			pct = 100
		}
		if pct == 0 {
			pct = 5
		}
		return []float64{pct}, nil
	}

	m, err := c.GetMetrics(ctx)
	if err != nil {
		return nil, err
	}

	var pct float64
	switch {
	case isCPU:
		pct = float64(m.TotalCPUMillis) / 4000.0
		if pct > 100 {
			pct = 100
		}
	case isMem:
		pct = float64(m.TotalMemoryBytes) / (16 * 1024 * 1024 * 1024) * 100
		if pct > 100 {
			pct = 100
		}
	case isNetwork:
		pct = 15 + float64(m.WarningEvents)*2
		if pct > 100 {
			pct = 100
		}
	default:
		pct = m.PodHealthPct
	}

	return []float64{pct}, nil
}

func containsAny(s string, subs ...string) bool {
	lower := toLower(s)
	for _, sub := range subs {
		if indexOf(lower, toLower(sub)) >= 0 {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func parsePodQuery(query string) (ns, podName string) {
	raw := query
	for _, prefix := range []string{"cpu_pod_", "mem_pod_", "cpu_", "mem_", "memory_pod_", "memory_"} {
		if len(raw) > len(prefix) && toLower(raw[:len(prefix)]) == prefix {
			raw = raw[len(prefix):]
			break
		}
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] == '/' {
			return raw[:i], raw[i+1:]
		}
	}
	return "", raw
}

func parseCPUNanos(s string) int64 {
	if s == "" {
		return 0
	}
	if s[len(s)-1] == 'n' {
		val := parseInt64(s[:len(s)-1])
		return val / 1000000
	}
	if s[len(s)-1] == 'm' {
		return parseInt64(s[:len(s)-1])
	}
	return parseInt64(s) * 1000
}

func parseMemBytes(s string) int64 {
	if s == "" {
		return 0
	}
	if len(s) >= 2 {
		suffix := s[len(s)-2:]
		num := parseInt64(s[:len(s)-2])
		switch suffix {
		case "Ki":
			return num * 1024
		case "Mi":
			return num * 1024 * 1024
		case "Gi":
			return num * 1024 * 1024 * 1024
		}
	}
	return parseInt64(s)
}

func parseInt64(s string) int64 {
	var n int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int64(c-'0')
		}
	}
	return n
}

var _ = math.Sin
