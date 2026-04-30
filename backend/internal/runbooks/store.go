package runbooks

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/djocham/kube-watcher/internal/notebooks"
)

// Runbook represents a runbook with metadata and content.
type Runbook struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Trigger  string    `json:"trigger"`
	Status   string    `json:"status"` // "ready" or "draft"
	Steps    int       `json:"steps"`
	LastRun  string    `json:"lastRun"`
	Modified time.Time `json:"modified"`
	Path     string    `json:"path"`
}

// Store manages runbook files on disk (and optionally synced via the notebooks S3 store).
type Store struct {
	logger    *slog.Logger
	dir       string
	notebooks *notebooks.Store
}

// New creates a runbooks store. If a notebooks.Store is provided, runbooks can
// also be synced to S3 under the "runbooks/" prefix.
func New(nbStore *notebooks.Store, logger *slog.Logger) (*Store, error) {
	dir := filepath.Join(os.ExpandEnv("$HOME"), ".kubewatcher", "runbooks")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create runbooks directory: %w", err)
	}

	return &Store{
		logger:    logger,
		dir:       dir,
		notebooks: nbStore,
	}, nil
}

// List returns all runbooks sorted by name.
func (st *Store) List(ctx context.Context) ([]Runbook, error) {
	entries, err := os.ReadDir(st.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read runbooks directory: %w", err)
	}

	var runbooks []Runbook
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		path := filepath.Join(st.dir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			st.logger.Warn("failed to read runbook", slog.String("path", path), slog.String("error", err.Error()))
			continue
		}

		rb := parseRunbook(entry.Name(), string(content), info.ModTime())
		runbooks = append(runbooks, rb)
	}

	sort.Slice(runbooks, func(i, j int) bool {
		return runbooks[i].Name < runbooks[j].Name
	})

	return runbooks, nil
}

// Get retrieves a single runbook's full content.
func (st *Store) Get(ctx context.Context, id string) (string, error) {
	path := st.idToPath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("runbook not found: %s", id)
	}
	return string(data), nil
}

// Save creates or updates a runbook file. The content should be markdown,
// optionally with YAML frontmatter.
func (st *Store) Save(ctx context.Context, id, content string) error {
	path := st.idToPath(id)
	if err := os.WriteFile(path, []byte(content), 0640); err != nil {
		return fmt.Errorf("failed to write runbook: %w", err)
	}

	// Sync to S3 via notebooks store if configured.
	if st.notebooks != nil && st.notebooks.IsConfigured() {
		s3Path := "runbooks/" + id + ".md"
		if err := st.notebooks.SaveFile(ctx, s3Path, content); err != nil {
			st.logger.Warn("failed to sync runbook to S3",
				slog.String("id", id),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

// Delete removes a runbook.
func (st *Store) Delete(ctx context.Context, id string) error {
	path := st.idToPath(id)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete runbook: %w", err)
	}

	// Delete from S3 if synced.
	if st.notebooks != nil && st.notebooks.IsConfigured() {
		s3Path := "runbooks/" + id + ".md"
		if err := st.notebooks.DeleteFile(ctx, s3Path); err != nil {
			st.logger.Warn("failed to delete runbook from S3",
				slog.String("id", id),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

// Create initializes a new runbook with default frontmatter.
func (st *Store) Create(ctx context.Context, name, trigger string) (Runbook, error) {
	id := nameToID(name)
	path := st.idToPath(id)

	// Check for duplicate.
	if _, err := os.Stat(path); err == nil {
		return Runbook{}, fmt.Errorf("runbook already exists: %s", name)
	}

	content := fmt.Sprintf(`---
name: %s
trigger: %s
status: draft
---

# %s

## Trigger
%s

## Steps

1. **Assess** — Verify the alert is legitimate.
2. **Investigate** — Check logs and metrics for root cause.
3. **Remediate** — Apply the fix.
4. **Verify** — Confirm the issue is resolved.

## Notes

_Add your notes here._
`, name, trigger, name, trigger)

	if err := os.WriteFile(path, []byte(content), 0640); err != nil {
		return Runbook{}, fmt.Errorf("failed to create runbook: %w", err)
	}

	rb := parseRunbook(id+".md", content, time.Now())
	return rb, nil
}

// idToPath converts a runbook ID to a file path.
func (st *Store) idToPath(id string) string {
	// Strip .md if caller included it.
	id = strings.TrimSuffix(id, ".md")
	return filepath.Join(st.dir, id+".md")
}

// nameToID converts a human-readable name to a filesystem-safe ID.
func nameToID(name string) string {
	id := strings.ToLower(name)
	id = strings.ReplaceAll(id, " ", "-")
	// Remove anything that's not alphanumeric, dash, or underscore.
	var clean strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			clean.WriteRune(r)
		}
	}
	return clean.String()
}

// parseRunbook extracts metadata from frontmatter and content.
func parseRunbook(filename, content string, modified time.Time) Runbook {
	id := strings.TrimSuffix(filename, ".md")

	rb := Runbook{
		ID:       id,
		Name:     id,
		Trigger:  "",
		Status:   "draft",
		Steps:    0,
		LastRun:  "Never",
		Modified: modified,
		Path:     filename,
	}

	// Parse YAML frontmatter if present.
	if strings.HasPrefix(content, "---\n") {
		end := strings.Index(content[4:], "\n---")
		if end >= 0 {
			frontmatter := content[4 : 4+end]
			body := content[4+end+4:]

			for _, line := range strings.Split(frontmatter, "\n") {
				line = strings.TrimSpace(line)
				if k, v, ok := strings.Cut(line, ":"); ok {
					k = strings.TrimSpace(k)
					v = strings.TrimSpace(v)
					switch k {
					case "name":
						rb.Name = v
					case "trigger":
						rb.Trigger = v
					case "status":
						rb.Status = v
					case "lastRun":
						rb.LastRun = v
					}
				}
			}

			// Count numbered steps in the body (lines starting with a digit and period).
			rb.Steps = countSteps(body)
		}
	} else {
		rb.Steps = countSteps(content)
	}

	if rb.Steps == 0 {
		rb.Steps = 1
	}

	return rb
}

// countSteps counts numbered list items in markdown content.
func countSteps(content string) int {
	count := 0
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 1 && trimmed[0] >= '1' && trimmed[0] <= '9' && (trimmed[1] == '.' || (len(trimmed) > 2 && trimmed[1] >= '0' && trimmed[1] <= '9' && trimmed[2] == '.')) {
			count++
		}
	}
	return count
}
