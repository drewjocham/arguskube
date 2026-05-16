package palette

import (
	"sort"
	"strings"
	"testing"
)

func TestCollapseDeploymentPods(t *testing.T) {
	t.Parallel()
	// Classic deployment shape: <deployment>-<replicaset-hash>-<random5>.
	// Two pods with the same prefix should collapse to one row.
	names := []string{
		"nginx-deployment-7c8d4-abc12",
		"nginx-deployment-7c8d4-def34",
		"nginx-deployment-7c8d4-ghi56",
	}
	groups := Collapse(names, 2)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group; got %d: %v", len(groups), summarize(groups))
	}
	if !strings.HasSuffix(groups[0].Display, "-*") {
		t.Errorf("collapsed row should end in -*; got %q", groups[0].Display)
	}
	if len(groups[0].Members) != 3 {
		t.Errorf("expected 3 members; got %d", len(groups[0].Members))
	}
}

func TestCollapseStatefulSetMembers(t *testing.T) {
	t.Parallel()
	// StatefulSet members carry numeric ordinals (kafka-0, kafka-1, …).
	// They SHOULD collapse — same prefix + numeric shape.
	names := []string{"kafka-0", "kafka-1", "kafka-2"}
	groups := Collapse(names, 2)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group; got %d: %v", len(groups), summarize(groups))
	}
	if groups[0].Display != "kafka-*" {
		t.Errorf("display = %q, want kafka-*", groups[0].Display)
	}
}

func TestCollapseDoesNotMergeAcrossShape(t *testing.T) {
	t.Parallel()
	// kafka-0 (numeric suffix) and kafka-broker-abc12 (alpha suffix)
	// must not collapse together — different shapes.
	names := []string{
		"kafka-0", "kafka-1", "kafka-2",
		"kafka-broker-abc12", "kafka-broker-def34",
	}
	groups := Collapse(names, 2)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups (kafka-* and kafka-broker-*); got %d: %v", len(groups), summarize(groups))
	}
	displays := []string{groups[0].Display, groups[1].Display}
	sort.Strings(displays)
	if displays[0] != "kafka-*" || displays[1] != "kafka-broker-*" {
		t.Errorf("groups = %v, want [kafka-*, kafka-broker-*]", displays)
	}
}

func TestCollapseHonorsMinThreshold(t *testing.T) {
	t.Parallel()
	// Single pod must not collapse — Display == Name.
	names := []string{"singleton-abc12"}
	groups := Collapse(names, 2)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group; got %d", len(groups))
	}
	if groups[0].Display != "singleton-abc12" {
		t.Errorf("solo row should display its raw name; got %q", groups[0].Display)
	}
	if len(groups[0].Members) != 1 {
		t.Errorf("expected 1 member; got %d", len(groups[0].Members))
	}

	// With min=5 and only 3 matching pods, they DON'T collapse.
	multi := []string{
		"foo-bar-abc12", "foo-bar-def34", "foo-bar-ghi56",
	}
	groups = Collapse(multi, 5)
	if len(groups) != 3 {
		t.Errorf("min=5, 3 members: expected 3 rows (no collapse); got %d", len(groups))
	}
}

func TestCollapsePreservesUngroupableNames(t *testing.T) {
	t.Parallel()
	// Names that don't fit the dash-short-token pattern emit as-is.
	// "redis" has no dash; "argus-agent-something-very-long" has a
	// final segment >10 chars; both should stand alone.
	names := []string{
		"redis",
		"argus-agent-something-very-long",
		"orders-queue-1", "orders-queue-2",
	}
	groups := Collapse(names, 2)
	if len(groups) != 3 {
		t.Fatalf("expected 3 rows (2 standalone + 1 orders-queue-*); got %d: %v",
			len(groups), summarize(groups))
	}
	hasCollapsed := false
	for _, g := range groups {
		if strings.HasSuffix(g.Display, "-*") {
			hasCollapsed = true
			if g.Display != "orders-queue-*" {
				t.Errorf("only orders-queue should collapse; got %q", g.Display)
			}
		}
	}
	if !hasCollapsed {
		t.Error("expected at least one collapsed row")
	}
}

func TestCollapseSortsOutputStable(t *testing.T) {
	t.Parallel()
	// Order of input shouldn't affect output — same input produces
	// same display order so the render layer doesn't jitter on
	// refresh.
	in := []string{
		"zebra-0", "zebra-1",
		"alpha-1", "alpha-2",
		"middle-x",
	}
	groups := Collapse(in, 2)
	gotOrder := make([]string, len(groups))
	for i, g := range groups {
		gotOrder[i] = g.Display
	}
	wantOrder := []string{"alpha-*", "middle-x", "zebra-*"}
	if !equal(gotOrder, wantOrder) {
		t.Errorf("display order = %v, want %v", gotOrder, wantOrder)
	}
}

func TestCollapseSkipsEmptyInput(t *testing.T) {
	t.Parallel()
	if got := Collapse(nil, 2); len(got) != 0 {
		t.Errorf("nil input: expected 0 groups; got %d", len(got))
	}
	if got := Collapse([]string{"", "   ", ""}, 2); len(got) != 0 {
		t.Errorf("whitespace-only input: expected 0 groups; got %d", len(got))
	}
}

func TestCollapseRejectsLongSuffix(t *testing.T) {
	t.Parallel()
	// A trailing segment >10 chars means the last dash is part of a
	// human-meaningful name, not a hash. Don't split it.
	names := []string{
		"service-frontend",
		"service-backend",
	}
	groups := Collapse(names, 2)
	// Both are standalone — neither splits the last token.
	if len(groups) != 2 {
		t.Errorf("expected 2 standalone rows (no collapse); got %d: %v",
			len(groups), summarize(groups))
	}
}

func TestCollapseMinClampedToTwo(t *testing.T) {
	t.Parallel()
	// min < 2 doesn't make sense — Collapse should clamp to 2.
	names := []string{"foo-abc12", "foo-def34"}
	g1 := Collapse(names, 0)
	g2 := Collapse(names, 1)
	g3 := Collapse(names, 2)
	if len(g1) != 1 || len(g2) != 1 || len(g3) != 1 {
		t.Errorf("all three should collapse to 1 row; got %d / %d / %d", len(g1), len(g2), len(g3))
	}
}

// ─── helpers ─────────────────────────────────────────────────────────

func summarize(gs []Group) []string {
	out := make([]string, len(gs))
	for i, g := range gs {
		out[i] = g.Display
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
