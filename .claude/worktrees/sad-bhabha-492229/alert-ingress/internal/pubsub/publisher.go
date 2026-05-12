package pubsub

import (
	"context"
	"encoding/json"

	"github.com/argues/kube-watcher/alert-ingress/internal/models"
)

type Publisher interface {
	PublishAlert(ctx context.Context, alert models.ArgusAlert) error
	Close() error
}

type nopPublisher struct{}

func NewNop() Publisher {
	return &nopPublisher{}
}

func (n *nopPublisher) PublishAlert(_ context.Context, alert models.ArgusAlert) error {
	_ = json.NewEncoder(noopWriter{}).Encode(alert)
	return nil
}

func (n *nopPublisher) Close() error { return nil }

type noopWriter struct{}

func (noopWriter) Write(p []byte) (int, error) { return len(p), nil }
