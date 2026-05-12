package pkg

import (
	"fmt"
	"log/slog"

	"github.com/argues/kube-watcher/internal/incidents"
	"github.com/argues/kube-watcher/internal/notebooks"
	"github.com/argues/kube-watcher/internal/runbooks"
	"github.com/argues/kube-watcher/internal/setup"
	"github.com/argues/kube-watcher/internal/workflows"
)

// --- Runbooks bindings ---

// ListRunbooks returns all runbooks.
func (a *App) ListRunbooks() ([]runbooks.Runbook, error) {
	if a.runbooks == nil {
		return []runbooks.Runbook{}, nil
	}
	return a.runbooks.List(a.ctx)
}

// GetRunbook retrieves a runbook's full markdown content.
func (a *App) GetRunbook(id string) (string, error) {
	if a.runbooks == nil {
		return "", fmt.Errorf("runbooks not configured")
	}
	return a.runbooks.Get(a.ctx, id)
}

// SaveRunbook saves a runbook's content.
func (a *App) SaveRunbook(id, content string) error {
	if a.runbooks == nil {
		return fmt.Errorf("runbooks not configured")
	}
	return a.runbooks.Save(a.ctx, id, content)
}

// DeleteRunbook removes a runbook.
func (a *App) DeleteRunbook(id string) error {
	if a.runbooks == nil {
		return fmt.Errorf("runbooks not configured")
	}
	return a.runbooks.Delete(a.ctx, id)
}

// CreateRunbook creates a new runbook with the given name and trigger.
func (a *App) CreateRunbook(name, trigger string) (runbooks.Runbook, error) {
	if a.runbooks == nil {
		return runbooks.Runbook{}, fmt.Errorf("runbooks not configured")
	}
	return a.runbooks.Create(a.ctx, name, trigger)
}

// --- Incident CRUD bindings ---

// ListIncidents returns all incidents, newest first.
func (a *App) ListIncidents() []incidents.Incident {
	if a.incidents == nil {
		return nil
	}
	return a.incidents.List(a.ctx)
}

// CreateIncident creates a new incident.
func (a *App) CreateIncident(title, severity, incType, description, namespace string) (incidents.Incident, error) {
	if a.incidents == nil {
		return incidents.Incident{}, fmt.Errorf("incident store not initialized")
	}
	return a.incidents.Create(a.ctx, title, severity, incType, description, namespace)
}

// UpdateIncident updates an existing incident's status or description.
func (a *App) UpdateIncident(id, status, description string) (*incidents.Incident, error) {
	if a.incidents == nil {
		return nil, fmt.Errorf("incident store not initialized")
	}
	return a.incidents.Update(a.ctx, id, status, description)
}

// DeleteIncident removes an incident.
func (a *App) DeleteIncident(id string) error {
	if a.incidents == nil {
		return fmt.Errorf("incident store not initialized")
	}
	return a.incidents.Delete(a.ctx, id)
}

// --- Notebooks bindings ---

// ListNotebooks returns the tree structure of all notebooks.
func (a *App) ListNotebooks() ([]notebooks.FileEntry, error) {
	if a.notebooks == nil {
		return []notebooks.FileEntry{}, nil
	}
	return a.notebooks.ListFiles(a.ctx)
}

// GetNotebook retrieves the content of a specific notebook file.
func (a *App) GetNotebook(path string) (string, error) {
	if a.notebooks == nil {
		return "", fmt.Errorf("notebooks not configured")
	}
	return a.notebooks.GetFile(a.ctx, path)
}

// SaveNotebook saves notebook content and syncs to S3.
func (a *App) SaveNotebook(path, content string) error {
	if a.notebooks == nil {
		return fmt.Errorf("notebooks not configured")
	}
	return a.notebooks.SaveFile(a.ctx, path, content)
}

// DeleteNotebook removes a notebook file.
func (a *App) DeleteNotebook(path string) error {
	if a.notebooks == nil {
		return fmt.Errorf("notebooks not configured")
	}
	return a.notebooks.DeleteFile(a.ctx, path)
}

// CreateNotebookFolder creates a new folder in the notebooks hierarchy.
func (a *App) CreateNotebookFolder(path string) error {
	if a.notebooks == nil {
		return fmt.Errorf("notebooks not configured")
	}
	return a.notebooks.CreateFolder(a.ctx, path)
}

// MoveNotebook moves a notebook from one path to another (copy + delete).
func (a *App) MoveNotebook(oldPath, newPath string) error {
	if a.notebooks == nil {
		return fmt.Errorf("notebooks not configured")
	}
	content, err := a.notebooks.GetFile(a.ctx, oldPath)
	if err != nil {
		return fmt.Errorf("failed to read source: %w", err)
	}
	if err := a.notebooks.SaveFile(a.ctx, newPath, content); err != nil {
		return fmt.Errorf("failed to write destination: %w", err)
	}
	return a.notebooks.DeleteFile(a.ctx, oldPath)
}

// TestS3Connection verifies S3 credentials and connectivity.
func (a *App) TestS3Connection() error {
	if a.notebooks == nil {
		return fmt.Errorf("notebooks not configured")
	}
	return a.notebooks.TestConnection(a.ctx)
}

// --- Setup bindings ---

// CheckToolStatus returns the install status of all required external tools.
func (a *App) CheckToolStatus() []setup.ToolStatus {
	if a.setup == nil {
		return []setup.ToolStatus{
			{Name: "popeye", Installed: false, Message: "Setup manager not initialized"},
			{Name: "kubewatcher-agent", Installed: false, Message: "Setup manager not initialized"},
		}
	}
	return a.setup.CheckAllTools(a.ctx)
}

// InstallArgusScan installs the scanner via go install or Docker pull.
func (a *App) InstallArgusScan() (*setup.SetupResult, error) {
	if a.setup == nil {
		return nil, fmt.Errorf("setup manager not initialized")
	}
	return a.setup.InstallPopeye(a.ctx), nil
}

// DeployAgent deploys the KubeWatcher agent to the connected cluster.
func (a *App) DeployAgent(namespace string) (*setup.SetupResult, error) {
	if a.setup == nil {
		return &setup.SetupResult{Success: false, Message: "Setup manager not initialized"}, nil
	}
	return a.setup.DeployAgent(a.ctx, namespace), nil
}

// UndeployAgent removes the KubeWatcher agent from the cluster.
func (a *App) UndeployAgent(namespace string) (*setup.SetupResult, error) {
	if a.setup == nil {
		return &setup.SetupResult{Success: false, Message: "Setup manager not initialized"}, nil
	}
	return a.setup.UndeployAgent(a.ctx, namespace), nil
}

// --- Workflow bindings ---

// ListWorkflows returns summaries of all saved workflows.
func (a *App) ListWorkflows() ([]workflows.WorkflowSummary, error) {
	if a.workflows == nil {
		return []workflows.WorkflowSummary{}, nil
	}
	return a.workflows.List()
}

// GetWorkflow returns a single workflow by ID.
func (a *App) GetWorkflow(id string) (*workflows.Workflow, error) {
	if a.workflows == nil {
		return nil, fmt.Errorf("workflow store not initialized")
	}
	return a.workflows.Get(id)
}

// SaveWorkflow creates or updates a workflow.
func (a *App) SaveWorkflow(wf workflows.Workflow) (*workflows.Workflow, error) {
	if a.workflows == nil {
		return nil, fmt.Errorf("workflow store not initialized")
	}
	return a.workflows.Save(&wf)
}

// DeleteWorkflow removes a workflow by ID.
func (a *App) DeleteWorkflow(id string) error {
	if a.workflows == nil {
		return fmt.Errorf("workflow store not initialized")
	}
	return a.workflows.Delete(id)
}

// --- Code Block bindings ---

// RunCodeSandbox executes code in a sandbox environment and returns the output.
func (a *App) RunCodeSandbox(code string, language string) (string, error) {
	// Mock implementation
	a.logger.InfoContext(a.ctx, "Running code sandbox", slog.String("language", language))
	return "> Execution started...\n> Loading dependencies...\n> Compilation successful.\n\nOutput:\nHello, KubeWatcher Sandbox Environment!\n\n> Exit code 0", nil
}

// GetCodeSuggestion returns an AI suggestion for the given code.
func (a *App) GetCodeSuggestion(code string, language string) (string, error) {
	// Mock implementation
	a.logger.InfoContext(a.ctx, "Getting code suggestion", slog.String("language", language))
	return "Consider adding error handling to this block. Extracting the hardcoded credentials into Kubernetes Secrets and loading them via environment variables would significantly improve security.", nil
}
