package pkg

import "context"

func (a *App) QueryPromQL(ctx context.Context, promql, duration string) ([]float64, error) {
	return a.k8s.QueryPromQL(ctx, promql, duration)
}
