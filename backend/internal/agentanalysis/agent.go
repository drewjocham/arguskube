package agentanalysis

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/argues/kube-watcher/internal/config"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type Agent struct {
	logger *slog.Logger
	cfg    *config.OnlineDataConfig
	appCtx context.Context
}

func NewAgent(logger *slog.Logger, cfg *config.OnlineDataConfig, appCtx context.Context) *Agent {
	return &Agent{
		logger: logger.With("component", "agentanalysis"),
		cfg:    cfg,
		appCtx: appCtx,
	}
}

func (a *Agent) StartLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.RunAnalysis()
		}
	}
}

// TriggerAnalysis manually fires the analysis (useful for testing/UI trigger)
func (a *Agent) TriggerAnalysis() {
	go a.RunAnalysis()
}

func (a *Agent) RunAnalysis() {
	a.logger.Info("Starting periodic agent analysis")

	// Emit start event with context of what is being looked at
	if a.appCtx != nil {
		runtime.EventsEmit(a.appCtx, "agent:analysis:start", map[string]interface{}{
			"lookingAt": "Cluster Health, Pending Alerts, Network Patterns",
		})
	}

	// Simulate AI analysis delay (in real scenario, we'd query DeepSeek or k8s)
	time.Sleep(3 * time.Second)

	// Combine instructions and simulated results
	resultText := fmt.Sprintf("Based on the instructions: '%s'\n\nEverything looks stable. 2 warnings detected in the past hour. Pod restarts are within acceptable limits.", a.cfg.AI.AgentInstructions)

	if a.appCtx != nil {
		runtime.EventsEmit(a.appCtx, "agent:analysis:complete", map[string]interface{}{
			"result": resultText,
		})
	}
}
