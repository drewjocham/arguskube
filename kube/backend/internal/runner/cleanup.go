package runner

import (
	"context"
	"encoding/json"
	"os/exec"
	"time"
)

// StaleThreshold is how old a run's infra must be before the cleanup
// loop considers it orphaned and destroys it. Default 2 hours.
var StaleThreshold = 2 * time.Hour

// CleanupLoop runs periodically to find and destroy orphaned GKE
// clusters that were left behind by crashed runner instances.
// Call this in a background goroutine.
func (r *Runner) CleanupLoop(ctx context.Context, projectID string) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.destroyOrphans(ctx, projectID)
		}
	}
}

// destroyOrphans lists all GKE clusters tagged with argus-managed=true,
// checks if their run-id is older than StaleThreshold, and destroys
// any that are.
func (r *Runner) destroyOrphans(ctx context.Context, projectID string) {
	// List clusters with the argus-managed label.
	cmd := exec.CommandContext(ctx,
		"gcloud", "container", "clusters", "list",
		"--project="+projectID,
		"--filter=labels.argus-managed=true",
		"--format=json(name,location,labels.argus-run-id,createTime)",
	)
	out, err := cmd.Output()
	if err != nil {
		r.logger.Warn("cleanup: failed to list clusters", "error", err)
		return
	}

	type cluster struct {
		Name       string `json:"name"`
		Location   string `json:"location"`
		RunID      string `json:"labels.argus-run-id"`
		CreateTime string `json:"createTime"`
	}
	var clusters []cluster
	if err := json.Unmarshal(out, &clusters); err != nil {
		r.logger.Warn("cleanup: failed to parse cluster list", "error", err)
		return
	}

	for _, c := range clusters {
		createTime, err := time.Parse(time.RFC3339, c.CreateTime)
		if err != nil {
			continue
		}
		if time.Since(createTime) < StaleThreshold {
			continue
		}

		r.logger.Info("cleanup: destroying orphaned cluster",
			"cluster", c.Name, "region", c.Location, "runId", c.RunID,
			"age", time.Since(createTime).Round(time.Minute))

		del := exec.CommandContext(ctx,
			"gcloud", "container", "clusters", "delete", c.Name,
			"--region="+c.Location,
			"--project="+projectID,
			"--quiet",
		)
		if err := del.Run(); err != nil {
			r.logger.Warn("cleanup: failed to delete cluster",
				"cluster", c.Name, "error", err)
		}
	}
}
