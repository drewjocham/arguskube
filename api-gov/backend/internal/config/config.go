package config

import (
	"os"
	"strconv"
	"time"
)

type APIGovConfig struct {
	// Server is the HTTP server configuration.
	Server ServerConfig
	// Database is the PostgreSQL/pgvector connection configuration.
	Database DatabaseConfig
	// Agent is the Python agent service configuration.
	Agent AgentConfig
	// LLM is the language model configuration.
	LLM LLMConfig
}

type ServerConfig struct {
	Port   string `env:"PORT" envDefault:"8080"`
	APIKey string `env:"API_GOV_API_KEY"`
}

type DatabaseConfig struct {
	URL string `env:"DATABASE_URL" envDefault:"postgres://api-gov:api-gov@localhost:5432/api-gov?sslmode=disable"`
}

type AgentConfig struct {
	ServiceURL  string `env:"AGENT_SERVICE_URL" envDefault:"http://localhost:8001"`
	ScanTimeout time.Duration
}

type LLMConfig struct {
	APIKey         string  `env:"LLM_API_KEY"`
	Model          string  `env:"LLM_MODEL" envDefault:"gemini-1.5-pro"`
	EmbeddingModel string  `env:"EMBEDDING_MODEL" envDefault:"text-embedding-004"`
	DriftThreshold float64 `env:"DRIFT_THRESHOLD" envDefault:"0.85"`
}

func loadEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func New() *APIGovConfig {
	return &APIGovConfig{
		Server: ServerConfig{
			Port:   loadEnv("PORT", "8080"),
			APIKey: loadEnv("API_GOV_API_KEY", ""),
		},
		Database: DatabaseConfig{
			URL: loadEnv("DATABASE_URL", "postgres://api-gov:api-gov@localhost:5432/api-gov?sslmode=disable"),
		},
		Agent: AgentConfig{
			ServiceURL:  loadEnv("AGENT_SERVICE_URL", "http://localhost:8001"),
			ScanTimeout: 5 * time.Minute,
		},
		LLM: LLMConfig{
			APIKey:         loadEnv("LLM_API_KEY", loadEnv("DEEPSEEK_API_KEY", "")),
			Model:          loadEnv("LLM_MODEL", "gemini-1.5-pro"),
			EmbeddingModel: loadEnv("EMBEDDING_MODEL", "text-embedding-004"),
			DriftThreshold: loadEnvFloat("DRIFT_THRESHOLD", 0.85),
		},
	}
}
