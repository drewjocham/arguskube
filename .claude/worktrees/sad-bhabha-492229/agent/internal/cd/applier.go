package cd

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
)

// Applier handles the application of Kubernetes manifests natively within the cluster.
type Applier struct {
	logger *slog.Logger
}

func NewApplier(logger *slog.Logger) *Applier {
	return &Applier{logger: logger}
}

// ApplyManifest uses Server-Side Apply (via kubectl wrapper for now) to apply the YAML.
// In a full production implementation, this would use client-go's dynamic client.
func (a *Applier) ApplyManifest(ctx context.Context, yamlContent []byte) error {
	a.logger.Info("Applying manifest via ArgusCD", slog.Int("bytes", len(yamlContent)))
	
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	cmd.Stdin = bytes.NewReader(yamlContent)
	
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	
	err := cmd.Run()
	if err != nil {
		a.logger.Error("Failed to apply manifest", slog.String("stderr", stderr.String()))
		return fmt.Errorf("apply failed: %s", stderr.String())
	}
	
	a.logger.Info("Manifest applied successfully", slog.String("stdout", out.String()))
	return nil
}
