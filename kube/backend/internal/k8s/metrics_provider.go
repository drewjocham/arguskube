package k8s

import (
	"context"
	"time"
)

type MetricsType string

const (
	MetricsTypePrometheus      MetricsType = "prometheus"
	MetricsTypeVictoriaMetrics MetricsType = "victoriametrics"
	MetricsTypeMetricsServer   MetricsType = "metrics-server"
	MetricsTypeDerived         MetricsType = "derived"
)

type MetricsProvider interface {
	QueryPromQL(ctx context.Context, promql string, start, end time.Time, step time.Duration) ([]float64, error)
	QueryTimeSeries(ctx context.Context, query string, points int) ([]float64, error)
	Healthy(ctx context.Context) bool
	Type() MetricsType
}
