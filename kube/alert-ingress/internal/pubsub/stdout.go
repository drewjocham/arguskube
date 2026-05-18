package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/argues/argus/alert-ingress/internal/models"
)

type StdoutPublisher struct {
	logger *slog.Logger
}

func NewStdout(logger *slog.Logger) Publisher {
	return &StdoutPublisher{logger: logger}
}

func (s *StdoutPublisher) PublishAlert(_ context.Context, alert models.ArgusAlert) error {
	b, _ := json.MarshalIndent(alert, "", "  ")
	s.logger.Info("alert published", "alert", string(b))
	return nil
}

func (s *StdoutPublisher) Close() error { return nil }

var _ Publisher = (*StdoutPublisher)(nil)

var _ fmt.Stringer = (*StdoutPublisher)(nil)

func (s *StdoutPublisher) String() string { return "stdout" }
