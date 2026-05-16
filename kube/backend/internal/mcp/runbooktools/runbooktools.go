// Package runbooktools exposes the runbooks store via the MCP Tool
// interface so the AI agent can read user-authored markdown runbooks
// and propose actions from them. Three tools land in this first
// cut — list, get, and plan. Plan is a dry-run: it parses the steps
// and returns the proposed actions, but does NOT execute anything.
//
// Why dry-run only:
//   Letting an LLM execute K8s API calls or shell commands directly
//   from a user-authored runbook is an unbounded blast-radius. The
//   pattern this package establishes — agent proposes, human (or a
//   separate approval policy) confirms, then a downstream tool
//   applies — keeps the failure mode at "AI told me wrong" not "AI
//   deleted prod". The execute path will land as runbook_apply
//   alongside an approval flow; that's a separate review.
//
// MCP Tool interface alignment: matches the existing dbtools.Tool
// shape (Name / Description / Parameters / Execute) so the future MCP
// server can register both packages through the same registry.
package runbooktools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/argues/argus/internal/runbooks"
)

// ToolParameter mirrors dbtools.ToolParameter — duplicated to avoid
// the runbook tools depending on dbtools and vice versa. Once the
// MCP server in kube/mcp/ comes back online both packages will alias
// these to a shared definition.
type ToolParameter struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
}

// Tool is the MCP-server contract.
type Tool interface {
	Name() string
	Description() string
	Parameters() []ToolParameter
	Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error)
}

// listTool returns all runbooks the user has authored. The agent
// calls this first to discover what's available before issuing get
// or plan calls.
type listTool struct {
	store *runbooks.Store
}

func NewListTool(store *runbooks.Store) Tool { return &listTool{store: store} }

func (t *listTool) Name() string { return "runbook_list" }
func (t *listTool) Description() string {
	return "List all runbooks. Returns id, name, trigger, status, step count, and last-modified time."
}
func (t *listTool) Parameters() []ToolParameter { return nil }

func (t *listTool) Execute(ctx context.Context, _ map[string]interface{}) (map[string]interface{}, error) {
	if t.store == nil {
		return nil, errors.New("runbook_list: store not configured")
	}
	list, err := t.store.List(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"runbooks": list}, nil
}

// getTool returns the raw markdown body of one runbook. The agent
// reads this to reason about what to do — separate from `plan` so
// the agent can choose between "read the prose" (chat context) and
// "extract structured steps" (action).
type getTool struct {
	store *runbooks.Store
}

func NewGetTool(store *runbooks.Store) Tool { return &getTool{store: store} }

func (t *getTool) Name() string { return "runbook_get" }
func (t *getTool) Description() string {
	return "Fetch the markdown body of one runbook by ID. Pair with runbook_list to discover IDs."
}
func (t *getTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "id", Type: "string", Required: true,
			Description: "Runbook ID — as returned by runbook_list."},
	}
}

func (t *getTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	if t.store == nil {
		return nil, errors.New("runbook_get: store not configured")
	}
	id, _ := args["id"].(string)
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("runbook_get: id is required")
	}
	body, err := t.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":   id,
		"body": body,
	}, nil
}

// PlanStep is the parsed shape of one runbook step. Steps live in
// markdown today (numbered list items under "## Steps"); future
// runbooks may grow YAML/JSON frontmatter for richer typing.
type PlanStep struct {
	Index int    `json:"index"`
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
}

// planTool extracts the step plan from a runbook. Dry-run only —
// returns the steps to the caller; does NOT apply anything. This is
// the safe seam between "AI reads runbook" and "operator confirms +
// system applies".
type planTool struct {
	store *runbooks.Store
}

func NewPlanTool(store *runbooks.Store) Tool { return &planTool{store: store} }

func (t *planTool) Name() string { return "runbook_plan" }
func (t *planTool) Description() string {
	return "Parse a runbook's Steps section and return the plan WITHOUT executing it. " +
		"Always run this before any apply step so the operator can audit the plan."
}
func (t *planTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "id", Type: "string", Required: true,
			Description: "Runbook ID — as returned by runbook_list."},
	}
}

func (t *planTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	if t.store == nil {
		return nil, errors.New("runbook_plan: store not configured")
	}
	id, _ := args["id"].(string)
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("runbook_plan: id is required")
	}
	body, err := t.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	steps := ParseSteps(body)
	return map[string]interface{}{
		"id":    id,
		"steps": steps,
		"count": len(steps),
		"note":  "This is a dry-run plan. No actions have been executed.",
	}, nil
}

// ParseSteps extracts numbered list items under the first "## Steps"
// (case-insensitive) heading in the runbook markdown. Exported so the
// Wails frontend can use the same parser for its preview UI.
func ParseSteps(md string) []PlanStep {
	lines := strings.Split(md, "\n")

	// Find the Steps heading.
	inSection := false
	stepLines := []string{}
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "## ") {
			heading := strings.ToLower(strings.TrimPrefix(trim, "## "))
			if heading == "steps" {
				inSection = true
				continue
			}
			if inSection {
				// Hit the next H2 — Steps section ended.
				break
			}
		}
		if inSection {
			stepLines = append(stepLines, line)
		}
	}

	// Parse numbered list items: "1. text", "2. text", ...
	steps := []PlanStep{}
	var current *PlanStep
	for _, line := range stepLines {
		trim := strings.TrimSpace(line)
		if num, rest, ok := splitNumberedItem(trim); ok {
			if current != nil {
				steps = append(steps, *current)
			}
			current = &PlanStep{Index: num, Title: rest}
			continue
		}
		if current != nil && trim != "" {
			// Continuation line — append to body.
			if current.Body == "" {
				current.Body = trim
			} else {
				current.Body += "\n" + trim
			}
		}
	}
	if current != nil {
		steps = append(steps, *current)
	}
	return steps
}

// splitNumberedItem returns (index, rest, true) for a markdown
// "N. text" line. Tolerates one or two-digit indices.
func splitNumberedItem(s string) (int, string, bool) {
	if len(s) < 2 {
		return 0, "", false
	}
	dot := strings.IndexByte(s, '.')
	if dot <= 0 || dot > 3 {
		return 0, "", false
	}
	prefix := s[:dot]
	for _, c := range prefix {
		if c < '0' || c > '9' {
			return 0, "", false
		}
	}
	n := 0
	for _, c := range prefix {
		n = n*10 + int(c-'0')
	}
	rest := strings.TrimSpace(s[dot+1:])
	if rest == "" {
		return 0, "", false
	}
	return n, rest, true
}

// String renders a PlanStep as the agent will see it in chat. Kept
// simple — the chat side does its own formatting on top of this.
func (p PlanStep) String() string {
	if p.Body == "" {
		return fmt.Sprintf("%d. %s", p.Index, p.Title)
	}
	return fmt.Sprintf("%d. %s\n   %s", p.Index, p.Title, p.Body)
}
