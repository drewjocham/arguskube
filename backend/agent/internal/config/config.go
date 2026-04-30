package config

import (
	"context"
	"os"
)

type Config struct {
	ServerPort    string
	SaaSToken     string
	SaaSServerURL string
}

func New(ctx context.Context) (*Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	token := os.Getenv("SAAS_TOKEN")
	serverURL := os.Getenv("SAAS_SERVER_URL")
	if serverURL == "" {
		serverURL = "ws://localhost:8080/tunnel"
	}

	return &Config{
		ServerPort:    port,
		SaaSToken:     token,
		SaaSServerURL: serverURL,
	}, nil
}
