package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type prometheusProvider struct {
	baseURL string
	client  *http.Client
	logger  *slog.Logger
	typ     MetricsType
	healthy bool
}

func newPrometheusProvider(baseURL string, logger *slog.Logger) *prometheusProvider {
	typ := MetricsTypePrometheus
	if strings.Contains(baseURL, "victoria") || strings.Contains(baseURL, ":8428") {
		typ = MetricsTypeVictoriaMetrics
	}
	return &prometheusProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 15 * time.Second},
		logger:  logger.With(slog.String("metrics_provider", string(typ))),
		typ:     typ,
	}
}

type promQueryRangeResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Values [][2]float64 `json:"values"`
		} `json:"result"`
	} `json:"data"`
	Error string `json:"error,omitempty"`
}

type promInstantResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Value [2]interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
	Error string `json:"error,omitempty"`
}

func (p *prometheusProvider) QueryPromQL(ctx context.Context, promql string, start, end time.Time, step time.Duration) ([]float64, error) {
	u, _ := url.Parse(p.baseURL + "/api/v1/query_range")
	q := u.Query()
	q.Set("query", promql)
	q.Set("start", formatTime(start))
	q.Set("end", formatTime(end))
	q.Set("step", formatDuration(step))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("prometheus request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		p.healthy = false
		return nil, fmt.Errorf("prometheus query: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var pr promQueryRangeResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if pr.Status != "success" {
		return nil, fmt.Errorf("prometheus error: %s", pr.Error)
	}

	if len(pr.Data.Result) == 0 || len(pr.Data.Result[0].Values) == 0 {
		return nil, nil
	}

	// Average across all series, return values.
	var values []float64
	for _, series := range pr.Data.Result {
		for _, v := range series.Values {
			if !math.IsNaN(v[1]) && !math.IsInf(v[1], 0) {
				values = append(values, v[1])
			}
		}
	}

	if len(values) == 0 {
		return nil, nil
	}

	p.healthy = true
	return values, nil
}

func (p *prometheusProvider) QueryInstant(ctx context.Context, promql string) (float64, error) {
	u, _ := url.Parse(p.baseURL + "/api/v1/query")
	q := u.Query()
	q.Set("query", promql)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("prometheus request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		p.healthy = false
		return 0, fmt.Errorf("prometheus query: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read response: %w", err)
	}

	var pr promInstantResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return 0, fmt.Errorf("parse response: %w", err)
	}

	if pr.Status != "success" || len(pr.Data.Result) == 0 {
		return 0, nil
	}

	val, ok := pr.Data.Result[0].Value[1].(string)
	if !ok {
		return 0, nil
	}

	var f float64
	fmt.Sscanf(val, "%f", &f)
	p.healthy = true
	return f, nil
}

func (p *prometheusProvider) QueryTimeSeries(ctx context.Context, query string, points int) ([]float64, error) {
	return p.QueryPromQL(ctx, query, time.Now().Add(-1*time.Hour), time.Now(), time.Duration(3600/points)*time.Second)
}

func (p *prometheusProvider) Healthy(ctx context.Context) bool {
	if !p.healthy {
		_, err := p.QueryInstant(ctx, "up")
		return err == nil
	}
	return true
}

func (p *prometheusProvider) Type() MetricsType {
	return p.typ
}

func formatTime(t time.Time) string {
	return t.Format(time.RFC3339)
}

func formatDuration(d time.Duration) string {
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

var _ MetricsProvider = (*prometheusProvider)(nil)
var _ = sort.Strings
