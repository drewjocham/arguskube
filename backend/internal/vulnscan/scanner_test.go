package vulnscan_test

import (
	"log/slog"
	"os"
	"testing"

	"github.com/argues/kube-watcher/internal/vulnscan"
)

func TestDemoResults(t *testing.T) {
	results := vulnscan.DemoResults()
	if len(results) == 0 {
		t.Fatal("DemoResults() returned empty slice")
	}

	// Verify structure of first result
	r := results[0]
	if r.ID == "" {
		t.Error("expected non-empty ID")
	}
	if r.Name == "" {
		t.Error("expected non-empty Name")
	}
	if r.LastScan == "" {
		t.Error("expected non-empty LastScan")
	}
	if r.Status != "Vulnerable" && r.Status != "Warning" && r.Status != "Clean" && r.Status != "Error" {
		t.Errorf("unexpected status: %s", r.Status)
	}
}

func TestDemoResultsCountBySeverity(t *testing.T) {
	results := vulnscan.DemoResults()

	for _, r := range results {
		// Verify CVE count matches severity fields
		var criticalCount, highCount int
		for _, cve := range r.CVEs {
			switch cve.Severity {
			case "Critical":
				criticalCount++
			case "High":
				highCount++
			}
		}
		if criticalCount != r.Critical {
			t.Errorf("%s: CVE critical count (%d) != field Critical (%d)", r.Name, criticalCount, r.Critical)
		}
		if highCount != r.High {
			t.Errorf("%s: CVE high count (%d) != field High (%d)", r.Name, highCount, r.High)
		}
	}
}

func TestDemoResultsStructure(t *testing.T) {
	results := vulnscan.DemoResults()

	for _, r := range results {
		if r.AIOpt.Issue == "" {
			t.Errorf("%s: missing AIOpt Issue", r.Name)
		}
		if r.AIOpt.Fix == "" {
			t.Errorf("%s: missing AIOpt Fix", r.Name)
		}
		if r.LastScan == "" {
			t.Errorf("%s: missing LastScan", r.Name)
		}
		if r.Namespace == "" {
			t.Errorf("%s: missing Namespace", r.Name)
		}
	}
}

func TestDemoResultsCleanImage(t *testing.T) {
	results := vulnscan.DemoResults()

	// Find the clean image (img-3)
	for _, r := range results {
		if r.Name == "payment:v2.0" {
			if r.Status != "Clean" {
				t.Errorf("payment:v2.0 status = %s, want Clean", r.Status)
			}
			if len(r.CVEs) != 0 {
				t.Errorf("payment:v2.0 should have 0 CVEs, got %d", len(r.CVEs))
			}
			if r.Critical != 0 || r.High != 0 || r.Medium != 0 || r.Low != 0 {
				t.Error("payment:v2.0 should have zero severity counts")
			}
		}
	}
}

func TestDemoResultsOrder(t *testing.T) {
	results := vulnscan.DemoResults()

	// Results should be ordered by severity (most critical first)
	for i := 1; i < len(results); i++ {
		prevScore := results[i-1].Critical*1000 + results[i-1].High*100 + results[i-1].Medium*10
		currScore := results[i].Critical*1000 + results[i].High*100 + results[i].Medium*10
		if currScore > prevScore {
			t.Errorf("results not sorted: index %d (score %d) > index %d (score %d)",
				i-1, prevScore, i, currScore)
		}
	}
}

func TestNewScannerNoClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("New() panicked: %v", r)
		}
	}()

	scanner := vulnscan.New(nil, logger)
	if scanner == nil {
		t.Fatal("New() returned nil")
	}
}

func TestScannerListReturnsDemoByDefault(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	scanner := vulnscan.New(nil, logger)

	results := scanner.List()
	if len(results) == 0 {
		t.Fatal("List() returned empty (should get demo results)")
	}

	// Should match DemoResults
	demo := vulnscan.DemoResults()
	if len(results) != len(demo) {
		t.Errorf("List() returned %d items, DemoResults() has %d", len(results), len(demo))
	}
}
