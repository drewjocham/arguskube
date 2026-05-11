package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/argues/argus/alert-ingress/internal/models"
)

type StdoutPublisher struct{}

func NewStdout() Publisher {
	return &StdoutPublisher{}
}

func (s *StdoutPublisher) PublishAlert(_ context.Context, alert models.ArgusAlert) error {
	b, _ := json.MarshalIndent(alert, "", "  ")
	log.Printf("[alert-ingress] %s\n", string(b))
	return nil
}

func (s *StdoutPublisher) Close() error { return nil }

var _ Publisher = (*StdoutPublisher)(nil)

var _ fmt.Stringer = (*StdoutPublisher)(nil)

func (s *StdoutPublisher) String() string { return "stdout" }
