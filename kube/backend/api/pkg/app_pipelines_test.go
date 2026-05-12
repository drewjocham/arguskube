package pkg

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func newPipelinesTestApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	return &App{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestGetPRGuidelines_NoFile(t *testing.T) {
	a := newPipelinesTestApp(t)
	got, err := a.GetPRGuidelines("github")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string for missing file, got %q", got)
	}
}

func TestSavePRGuidelines_RoundTrip(t *testing.T) {
	a := newPipelinesTestApp(t)
	want := "# Review rules\n\n- Be kind\n- Check tests\n"
	if err := a.SavePRGuidelines("github", want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := a.GetPRGuidelines("github")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != want {
		t.Errorf("round trip mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestSavePRGuidelines_EmptyClears(t *testing.T) {
	a := newPipelinesTestApp(t)
	if err := a.SavePRGuidelines("gitlab", "draft content"); err != nil {
		t.Fatalf("seed save: %v", err)
	}
	if err := a.SavePRGuidelines("gitlab", ""); err != nil {
		t.Fatalf("clear: %v", err)
	}
	got, err := a.GetPRGuidelines("gitlab")
	if err != nil {
		t.Fatalf("get after clear: %v", err)
	}
	if got != "" {
		t.Errorf("expected cleared content, got %q", got)
	}
}

func TestSavePRGuidelines_PerProvider(t *testing.T) {
	a := newPipelinesTestApp(t)
	if err := a.SavePRGuidelines("github", "github rules"); err != nil {
		t.Fatalf("github save: %v", err)
	}
	if err := a.SavePRGuidelines("azure", "azure rules"); err != nil {
		t.Fatalf("azure save: %v", err)
	}
	gh, err := a.GetPRGuidelines("github")
	if err != nil || gh != "github rules" {
		t.Errorf("github get: %q err=%v", gh, err)
	}
	az, err := a.GetPRGuidelines("azure")
	if err != nil || az != "azure rules" {
		t.Errorf("azure get: %q err=%v", az, err)
	}
}

func TestSavePRGuidelines_RejectsInvalidProvider(t *testing.T) {
	a := newPipelinesTestApp(t)
	bad := []string{"", "../escape", "Has Space", "weird/slash", "Upper"}
	for _, p := range bad {
		if err := a.SavePRGuidelines(p, "x"); err == nil {
			t.Errorf("expected error for provider %q, got nil", p)
		}
		if _, err := a.GetPRGuidelines(p); err == nil {
			t.Errorf("expected error reading provider %q, got nil", p)
		}
	}
}

func TestSavePRGuidelines_RejectsOversize(t *testing.T) {
	a := newPipelinesTestApp(t)
	huge := strings.Repeat("x", maxPRGuidelinesBytes+1)
	err := a.SavePRGuidelines("github", huge)
	if err == nil {
		t.Fatalf("expected oversize error, got nil")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("expected 'too large' in error, got %q", err)
	}
}

func TestCodeReviewReport_RoundTrip(t *testing.T) {
	a := newPipelinesTestApp(t)
	body := "## Findings\n\n- Looks good\n"
	rep, err := a.CreateCodeReviewReport("github", "PR #42 review", "github:acme/repo#42", body)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if rep.ID == "" || rep.Title != "PR #42 review" || rep.PRRef != "github:acme/repo#42" {
		t.Errorf("unexpected metadata: %+v", rep)
	}
	got, err := a.GetCodeReviewReport("github", rep.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != body {
		t.Errorf("body round-trip:\n got: %q\nwant: %q", got, body)
	}
}

func TestListCodeReviewReports_Sorted(t *testing.T) {
	a := newPipelinesTestApp(t)
	if _, err := a.CreateCodeReviewReport("github", "first", "", "a"); err != nil {
		t.Fatalf("create first: %v", err)
	}
	// Different second so the unix timestamp differs.
	// On fast machines two calls may share the same second; force separation.
	originalNow := time.Now()
	for time.Now().Unix() == originalNow.Unix() {
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := a.CreateCodeReviewReport("github", "second", "", "b"); err != nil {
		t.Fatalf("create second: %v", err)
	}
	reps, err := a.ListCodeReviewReports("github")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(reps) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(reps))
	}
	if reps[0].Title != "second" {
		t.Errorf("expected 'second' first (newest), got %q", reps[0].Title)
	}
}

func TestListCodeReviewReports_EmptyOnFreshProvider(t *testing.T) {
	a := newPipelinesTestApp(t)
	reps, err := a.ListCodeReviewReports("gitlab")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(reps) != 0 {
		t.Errorf("expected empty list, got %d", len(reps))
	}
}

func TestDeleteCodeReviewReport(t *testing.T) {
	a := newPipelinesTestApp(t)
	rep, err := a.CreateCodeReviewReport("github", "to delete", "", "body")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := a.DeleteCodeReviewReport("github", rep.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := a.GetCodeReviewReport("github", rep.ID); err == nil {
		t.Errorf("expected get-after-delete to fail")
	}
	// Idempotent: deleting again is fine.
	if err := a.DeleteCodeReviewReport("github", rep.ID); err != nil {
		t.Errorf("expected delete to be idempotent, got %v", err)
	}
}

func TestCodeReviewReport_RejectsBadInputs(t *testing.T) {
	a := newPipelinesTestApp(t)
	// Empty title.
	if _, err := a.CreateCodeReviewReport("github", "  ", "", "body"); err == nil {
		t.Errorf("expected error for empty title")
	}
	// Bad provider.
	if _, err := a.CreateCodeReviewReport("../escape", "t", "", "b"); err == nil {
		t.Errorf("expected error for bad provider")
	}
	// Bad id on get/delete.
	if _, err := a.GetCodeReviewReport("github", "../escape"); err == nil {
		t.Errorf("expected error for bad id (get)")
	}
	if err := a.DeleteCodeReviewReport("github", "../escape"); err == nil {
		t.Errorf("expected error for bad id (delete)")
	}
	// Oversize.
	huge := strings.Repeat("x", maxCodeReviewBytes+1)
	if _, err := a.CreateCodeReviewReport("github", "t", "", huge); err == nil {
		t.Errorf("expected oversize error")
	}
}
