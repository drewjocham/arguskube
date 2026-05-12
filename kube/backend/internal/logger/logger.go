package logger

import (
	"log/slog"
	"os"

	"github.com/argues/argus/internal/config"
)

// Log attribute keys — no magic strings.
const (
	KeyAlertID      = "alertId"
	KeyPodName      = "podName"
	KeyNamespace    = "namespace"
	KeyNodeName     = "nodeName"
	KeySeverity     = "severity"
	KeyRestarts     = "restarts"
	KeyFeature      = "feature"
	KeyTier         = "tier"
	KeyComponent    = "component"
	KeyError        = "error"
	KeyDuration     = "duration"
	KeyCluster      = "cluster"
	KeyAnomstackJob = "anomstackJob"
)

// New creates the single shared slog.Logger from config.
func New(cfg *config.OnlineDataConfig) *slog.Logger {
	level := parseLevel(cfg.Logging.Level)

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level, AddSource: true}

	if cfg.Logging.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
