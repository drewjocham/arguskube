package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"cloud.google.com/go/pubsub"

	"github.com/argues/argus/alert-ingress/internal/models"
)

type GCPPublisher struct {
	client *pubsub.Client
	topic  *pubsub.Topic
	logger *slog.Logger
}

func NewGCP(ctx context.Context, logger *slog.Logger, projectID, topicID string) (*GCPPublisher, error) {
	if projectID == "" {
		projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}
	if topicID == "" {
		topicID = os.Getenv("PUBSUB_TOPIC")
		if topicID == "" {
			topicID = "argus-alerts"
		}
	}

	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("pubsub client: %w", err)
	}

	topic := client.Topic(topicID)
	exists, err := topic.Exists(ctx)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("topic check: %w", err)
	}
	if !exists {
		topic, err = client.CreateTopic(ctx, topicID)
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("create topic: %w", err)
		}
		logger.Info("created pubsub topic", "topic", topicID)
	}

	logger.Info("pubsub publisher ready", "project", projectID, "topic", topicID)
	return &GCPPublisher{client: client, topic: topic, logger: logger}, nil
}

func (g *GCPPublisher) PublishAlert(ctx context.Context, alert models.ArgusAlert) error {
	data, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("marshal alert: %w", err)
	}

	result := g.topic.Publish(ctx, &pubsub.Message{
		Data: data,
	})

	id, err := result.Get(ctx)
	if err != nil {
		return fmt.Errorf("pubsub publish: %w", err)
	}

	g.logger.Info("published alert", "alertID", alert.ID, "messageID", id)
	return nil
}

func (g *GCPPublisher) Close() error {
	g.topic.Stop()
	return g.client.Close()
}
