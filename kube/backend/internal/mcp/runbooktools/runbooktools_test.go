package runbooktools

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/argues/argus/internal/runbooks"
)

func newStore(t *testing.T) *runbooks.Store {
	t.Helper()
	// runbooks.New writes to $HOME/.argus/runbooks. Redirect HOME at
	// test temp so we don't pollute the developer's real one.
	t.Setenv("HOME", t.TempDir())
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	st, err := runbooks.New(nil, logger)
	if err != nil {
		t.Fatalf("runbooks.New: %v", err)
	}
	return st
}

func TestListToolMetadata(t *testing.T) {
	t.Parallel()
	tool := NewListTool(nil)
	if tool.Name() != "runbook_list" {
		t.Errorf("Name = %q, want runbook_list", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
	if len(tool.Parameters()) != 0 {
		t.Errorf("runbook_list has no inputs; got %d parameters", len(tool.Parameters()))
	}
}

func TestListToolReturnsEmpty(t *testing.T) {
	// No t.Parallel: newStore uses t.Setenv which is incompatible.
	store := newStore(t)
	tool := NewListTool(store)
	out, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	list, ok := out["runbooks"].([]runbooks.Runbook)
	if !ok {
		t.Fatalf("expected []runbooks.Runbook, got %T", out["runbooks"])
	}
	if len(list) != 0 {
		t.Errorf("fresh dir should be empty; got %d", len(list))
	}
}

func TestGetToolFetchesBody(t *testing.T) {
	// No t.Parallel: newStore uses t.Setenv which is incompatible.
	store := newStore(t)
	const body = "---\nname: noisy\n---\n# Noisy\n\n## Steps\n1. Look at the metrics.\n"
	if err := store.Save(context.Background(), "noisy", body); err != nil {
		t.Fatalf("Save: %v", err)
	}

	tool := NewGetTool(store)
	out, err := tool.Execute(context.Background(), map[string]any{"id": "noisy"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["id"] != "noisy" {
		t.Errorf("id mismatch: %v", out["id"])
	}
	if got, _ := out["body"].(string); !strings.Contains(got, "Look at the metrics") {
		t.Errorf("body did not roundtrip; got %q", got)
	}
}

func TestGetToolRequiresID(t *testing.T) {
	// No t.Parallel: newStore uses t.Setenv which is incompatible.
	store := newStore(t)
	tool := NewGetTool(store)
	if _, err := tool.Execute(context.Background(), map[string]any{}); err == nil {
		t.Fatal("Execute without id should error")
	}
	if _, err := tool.Execute(context.Background(), map[string]any{"id": "   "}); err == nil {
		t.Fatal("Execute with whitespace id should error")
	}
}

func TestPlanToolReturnsParsedSteps(t *testing.T) {
	// No t.Parallel: newStore uses t.Setenv which is incompatible.
	store := newStore(t)
	const body = `---
name: rolling-restart
---
# Rolling restart

## Trigger
On CrashLoopBackOff in prod

## Steps
1. Identify the offending deployment.
2. kubectl rollout restart deployment/<name>.
   Continuation line for step 2.
3. Verify pods come up Ready.

## Notes
Some prose here that is NOT a step.
`
	if err := store.Save(context.Background(), "rolling-restart", body); err != nil {
		t.Fatalf("Save: %v", err)
	}

	tool := NewPlanTool(store)
	out, err := tool.Execute(context.Background(), map[string]any{"id": "rolling-restart"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	steps, ok := out["steps"].([]PlanStep)
	if !ok {
		t.Fatalf("expected []PlanStep, got %T", out["steps"])
	}
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps; got %d (%v)", len(steps), steps)
	}
	if steps[0].Index != 1 || !strings.Contains(steps[0].Title, "offending deployment") {
		t.Errorf("step 1 mis-parsed: %+v", steps[0])
	}
	if steps[1].Index != 2 {
		t.Errorf("step 2 index = %d, want 2", steps[1].Index)
	}
	if !strings.Contains(steps[1].Body, "Continuation line") {
		t.Errorf("step 2 continuation not captured: %+v", steps[1])
	}
	if steps[2].Index != 3 {
		t.Errorf("step 3 index = %d, want 3", steps[2].Index)
	}

	if out["note"] != "This is a dry-run plan. No actions have been executed." {
		t.Errorf("plan must announce its dry-run nature; note = %q", out["note"])
	}
}

func TestParseStepsEmptyRunbook(t *testing.T) {
	t.Parallel()
	if got := ParseSteps(""); len(got) != 0 {
		t.Errorf("empty input should yield no steps; got %d", len(got))
	}
}

func TestParseStepsNoStepsSection(t *testing.T) {
	t.Parallel()
	md := "# Title\n\nNo Steps heading here, just prose.\n"
	if got := ParseSteps(md); len(got) != 0 {
		t.Errorf("runbook without ## Steps should yield no steps; got %d", len(got))
	}
}

func TestParseStepsHandlesDoubleDigit(t *testing.T) {
	t.Parallel()
	md := "## Steps\n10. tenth\n11. eleventh\n"
	steps := ParseSteps(md)
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps; got %d", len(steps))
	}
	if steps[0].Index != 10 || steps[1].Index != 11 {
		t.Errorf("indexes = %d, %d; want 10, 11", steps[0].Index, steps[1].Index)
	}
}

func TestPlanStepString(t *testing.T) {
	t.Parallel()
	if got := (PlanStep{Index: 1, Title: "Title"}).String(); got != "1. Title" {
		t.Errorf("String without body = %q, want %q", got, "1. Title")
	}
	if got := (PlanStep{Index: 2, Title: "T", Body: "B"}).String(); !strings.Contains(got, "2. T") || !strings.Contains(got, "B") {
		t.Errorf("String with body lost content: %q", got)
	}
}
