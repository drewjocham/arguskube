package runner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/argues/argus/internal/saasapi"
)

// logWriter is an io.Writer that feeds each line to a slog.Logger.
type logWriter struct {
	logger *slog.Logger
	key    string
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.logger.Info(string(p), w.key, "")
	return len(p), nil
}

// tofuModuleDirName is the directory name of the OpenTofu module inside
// the deployed runner image.
const tofuModuleDirName = "runner-region"

// tofuApply provisions infra for one region via OpenTofu. Returns the
// in-cluster broker endpoint the engine connects to.
func (r *Runner) tofuApply(ctx context.Context, reg saasapi.RegionSpec, cs *cleanupState) (string, error) {
	workDir := filepath.Join(r.workspace, r.spec.RunID, reg.Region)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", fmt.Errorf("mkdir workspace: %w", err)
	}
	cs.workDir = workDir

	// Copy the terraform module into the working directory.
	if err := copyDir(r.modulePath, workDir); err != nil {
		return "", fmt.Errorf("copy module: %w", err)
	}

	// Write terraform.tfvars.
	tfvars := fmt.Sprintf(`project_id   = "%s"
run_id       = "%s"
region       = "%s"
node_count   = %d
machine_type = "%s"
broker_kind  = "%s"
tags         = { argus-managed = "true", argus-run-id = "%s" }
`, r.spec.RunID, r.spec.RunID, reg.Region, reg.Count, reg.InstanceType, r.spec.Broker, r.spec.RunID)

	if err := os.WriteFile(filepath.Join(workDir, "terraform.tfvars"), []byte(tfvars), 0644); err != nil {
		return "", fmt.Errorf("write tfvars: %w", err)
	}

	// tofu init.
	if err := r.runTofu(ctx, workDir, "init", "-input=false"); err != nil {
		return "", fmt.Errorf("tofu init: %w", err)
	}

	// Mark provisioned before invoking apply, not after. A partial apply
	// (e.g. VPC and subnets created but GKE cluster creation fails)
	// leaves real cloud resources behind that must be torn down via
	// `tofu destroy`. Gating the destroy on a flag set only after a
	// successful apply would leak those resources for ~2h until the
	// spot quota auto-reclaims them.
	cs.provisioned = true

	// tofu apply.
	if err := r.runTofu(ctx, workDir, "apply", "-auto-approve", "-input=false", "-no-color"); err != nil {
		return "", fmt.Errorf("tofu apply: %w", err)
	}

	// Read the broker endpoint from tofu output.
	return r.readTofuOutput(workDir, "broker_endpoint")
}

// tofuDestroy tears down infra for one region.
func (r *Runner) tofuDestroy(ctx context.Context, workDir string) error {
	if workDir == "" {
		return nil
	}
	if err := r.runTofu(ctx, workDir, "destroy", "-auto-approve", "-input=false", "-no-color"); err != nil {
		return fmt.Errorf("tofu destroy: %w", err)
	}
	return os.RemoveAll(workDir)
}

// runTofu executes a tofu command in the given working directory.
func (r *Runner) runTofu(ctx context.Context, workDir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "tofu", args...)
	cmd.Dir = workDir
	lw := &logWriter{logger: r.logger, key: "tofu"}
	cmd.Stdout = lw
	cmd.Stderr = lw
	return cmd.Run()
}

// readTofuOutput reads a terraform output value. Simple implementation
// that parses `tofu output -raw <name>`.
func (r *Runner) readTofuOutput(workDir, name string) (string, error) {
	cmd := exec.Command("tofu", "output", "-raw", name)
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tofu output %s: %w", name, err)
	}
	return string(out), nil
}

// tofuDestroyAll tears down ALL regions for this run. Called on
// cancellation or the final deferred cleanup.
func (r *Runner) tofuDestroyAll(ctx context.Context) {
	for _, reg := range r.spec.Regions {
		workDir := filepath.Join(r.workspace, r.spec.RunID, reg.Region)
		if err := r.tofuDestroy(ctx, workDir); err != nil {
			r.logger.Warn("tofu destroy failed",
				"region", reg.Region, "error", err)
		}
	}
}

// copyDir copies a directory recursively. Simple implementation;
// production would use a proper file copy.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
