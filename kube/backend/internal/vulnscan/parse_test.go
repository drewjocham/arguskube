package vulnscan

// White-box tests for the JSON-parsing, severity-normalization, AI-optimization,
// and formatting helpers.  These live in the same package so they can reach
// unexported symbols (titleCase, fmtElapsed) directly.
// parseTrivyOutput was extracted from scanImage specifically to enable this
// coverage without execing trivy.

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// parseTrivyOutput — happy path
// ---------------------------------------------------------------------------

func TestParseTrivyOutput_HappyPath(t *testing.T) {
	const raw = `{
		"Results": [{
			"Target": "alpine:3.18 (alpine 3.18.2)",
			"Vulnerabilities": [
				{
					"VulnerabilityID": "CVE-2023-0001",
					"PkgName": "curl",
					"Severity": "CRITICAL",
					"Title": "Heap overflow in curl",
					"FixedVersion": "8.4.0"
				},
				{
					"VulnerabilityID": "CVE-2023-0002",
					"PkgName": "openssl",
					"Severity": "HIGH",
					"Title": "Timing side-channel",
					"FixedVersion": "3.0.13"
				},
				{
					"VulnerabilityID": "CVE-2023-0003",
					"PkgName": "zlib",
					"Severity": "MEDIUM",
					"Title": "Memory corruption",
					"FixedVersion": ""
				},
				{
					"VulnerabilityID": "CVE-2023-0004",
					"PkgName": "libpng",
					"Severity": "LOW",
					"Title": "Info disclosure",
					"FixedVersion": ""
				}
			]
		}]
	}`

	img, err := parseTrivyOutput("myimage:v1", "default", "5s ago", []byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if img.Name != "myimage:v1" {
		t.Errorf("Name = %q, want %q", img.Name, "myimage:v1")
	}
	if img.Namespace != "default" {
		t.Errorf("Namespace = %q, want %q", img.Namespace, "default")
	}
	if img.LastScan != "5s ago" {
		t.Errorf("LastScan = %q, want %q", img.LastScan, "5s ago")
	}
	if img.Critical != 1 {
		t.Errorf("Critical = %d, want 1", img.Critical)
	}
	if img.High != 1 {
		t.Errorf("High = %d, want 1", img.High)
	}
	if img.Medium != 1 {
		t.Errorf("Medium = %d, want 1", img.Medium)
	}
	if img.Low != 1 {
		t.Errorf("Low = %d, want 1", img.Low)
	}
	// Only Critical+High land in the CVE list.
	if len(img.CVEs) != 2 {
		t.Fatalf("CVEs len = %d, want 2", len(img.CVEs))
	}
	if img.CVEs[0].ID != "CVE-2023-0001" {
		t.Errorf("CVEs[0].ID = %q, want CVE-2023-0001", img.CVEs[0].ID)
	}
	if img.CVEs[0].Fix != "Upgrade curl to 8.4.0" {
		t.Errorf("CVEs[0].Fix = %q", img.CVEs[0].Fix)
	}
	if img.CVEs[1].Fix != "Upgrade openssl to 3.0.13" {
		t.Errorf("CVEs[1].Fix = %q", img.CVEs[1].Fix)
	}
	if img.Status != "Vulnerable" {
		t.Errorf("Status = %q, want Vulnerable", img.Status)
	}
}

// ---------------------------------------------------------------------------
// parseTrivyOutput — malformed JSON
// ---------------------------------------------------------------------------

func TestParseTrivyOutput_MalformedJSON(t *testing.T) {
	_, err := parseTrivyOutput("img", "ns", "now", []byte(`{not valid json`))
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "trivy json parse") {
		t.Errorf("error message %q does not contain 'trivy json parse'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// parseTrivyOutput — empty Results array
// ---------------------------------------------------------------------------

func TestParseTrivyOutput_EmptyResults(t *testing.T) {
	const raw = `{"Results": []}`

	img, err := parseTrivyOutput("clean-image:v1", "prod", "now", []byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if img.Critical != 0 || img.High != 0 || img.Medium != 0 || img.Low != 0 {
		t.Errorf("expected all zero counts, got C=%d H=%d M=%d L=%d",
			img.Critical, img.High, img.Medium, img.Low)
	}
	if len(img.CVEs) != 0 {
		t.Errorf("expected empty CVEs, got %d", len(img.CVEs))
	}
	if img.Status != "Clean" {
		t.Errorf("Status = %q, want Clean", img.Status)
	}
}

// ---------------------------------------------------------------------------
// parseTrivyOutput — null Vulnerabilities field (missing key)
// ---------------------------------------------------------------------------

func TestParseTrivyOutput_NullVulnerabilities(t *testing.T) {
	const raw = `{"Results": [{"Target": "scratch", "Vulnerabilities": null}]}`

	img, err := parseTrivyOutput("scratch", "kube-system", "now", []byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if img.Critical != 0 {
		t.Errorf("Critical = %d, want 0", img.Critical)
	}
	if img.Status != "Clean" {
		t.Errorf("Status = %q, want Clean", img.Status)
	}
}

// ---------------------------------------------------------------------------
// parseTrivyOutput — multiple Results blocks (multi-layer scan)
// ---------------------------------------------------------------------------

func TestParseTrivyOutput_MultipleResultsBlocks(t *testing.T) {
	const raw = `{
		"Results": [
			{
				"Target": "layer1",
				"Vulnerabilities": [
					{"VulnerabilityID":"CVE-A","PkgName":"pkg-a","Severity":"CRITICAL","Title":"A","FixedVersion":"1.0"}
				]
			},
			{
				"Target": "layer2",
				"Vulnerabilities": [
					{"VulnerabilityID":"CVE-B","PkgName":"pkg-b","Severity":"HIGH","Title":"B","FixedVersion":"2.0"}
				]
			}
		]
	}`

	img, err := parseTrivyOutput("multi:v1", "ns", "now", []byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if img.Critical != 1 {
		t.Errorf("Critical = %d, want 1", img.Critical)
	}
	if img.High != 1 {
		t.Errorf("High = %d, want 1", img.High)
	}
	if len(img.CVEs) != 2 {
		t.Errorf("CVEs len = %d, want 2", len(img.CVEs))
	}
}

// ---------------------------------------------------------------------------
// parseTrivyOutput — severity normalization (case variants)
// ---------------------------------------------------------------------------

func TestParseTrivyOutput_SeverityNormalization(t *testing.T) {
	cases := []struct {
		rawSeverity string
		wantCritical int
		wantHigh     int
		wantMedium   int
		wantLow      int
		inCVEList    bool
	}{
		{"CRITICAL", 1, 0, 0, 0, true},
		{"critical", 1, 0, 0, 0, true},
		{"Critical", 1, 0, 0, 0, true},
		{"HIGH", 0, 1, 0, 0, true},
		{"high", 0, 1, 0, 0, true},
		{"MEDIUM", 0, 0, 1, 0, false},
		{"medium", 0, 0, 1, 0, false},
		{"LOW", 0, 0, 0, 1, false},
		{"low", 0, 0, 0, 1, false},
		// Unknown severity: none of the counters increment, not in CVE list.
		{"UNKNOWN", 0, 0, 0, 0, false},
		{"", 0, 0, 0, 0, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("severity="+tc.rawSeverity, func(t *testing.T) {
			raw := `{"Results":[{"Target":"t","Vulnerabilities":[{` +
				`"VulnerabilityID":"CVE-X","PkgName":"pkg","Severity":"` + tc.rawSeverity + `",` +
				`"Title":"title","FixedVersion":"1.0"}]}]}`

			img, err := parseTrivyOutput("img", "ns", "now", []byte(raw))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if img.Critical != tc.wantCritical {
				t.Errorf("Critical = %d, want %d", img.Critical, tc.wantCritical)
			}
			if img.High != tc.wantHigh {
				t.Errorf("High = %d, want %d", img.High, tc.wantHigh)
			}
			if img.Medium != tc.wantMedium {
				t.Errorf("Medium = %d, want %d", img.Medium, tc.wantMedium)
			}
			if img.Low != tc.wantLow {
				t.Errorf("Low = %d, want %d", img.Low, tc.wantLow)
			}
			inList := len(img.CVEs) > 0
			if inList != tc.inCVEList {
				t.Errorf("in CVE list = %v, want %v", inList, tc.inCVEList)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseTrivyOutput — status determination
// ---------------------------------------------------------------------------

func TestParseTrivyOutput_StatusDetermination(t *testing.T) {
	cases := []struct {
		name       string
		json       string
		wantStatus string
	}{
		{
			name:       "critical → Vulnerable",
			json:       `{"Results":[{"Target":"t","Vulnerabilities":[{"VulnerabilityID":"C","PkgName":"p","Severity":"CRITICAL","Title":"t","FixedVersion":"1"}]}]}`,
			wantStatus: "Vulnerable",
		},
		{
			name:       "high only → Warning",
			json:       `{"Results":[{"Target":"t","Vulnerabilities":[{"VulnerabilityID":"H","PkgName":"p","Severity":"HIGH","Title":"t","FixedVersion":"1"}]}]}`,
			wantStatus: "Warning",
		},
		{
			name:       "medium only → Clean",
			json:       `{"Results":[{"Target":"t","Vulnerabilities":[{"VulnerabilityID":"M","PkgName":"p","Severity":"MEDIUM","Title":"t","FixedVersion":"1"}]}]}`,
			wantStatus: "Clean",
		},
		{
			name:       "no vulns → Clean",
			json:       `{"Results":[]}`,
			wantStatus: "Clean",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			img, err := parseTrivyOutput("img", "ns", "now", []byte(tc.json))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if img.Status != tc.wantStatus {
				t.Errorf("Status = %q, want %q", img.Status, tc.wantStatus)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseTrivyOutput — description truncation at 120 chars
// ---------------------------------------------------------------------------

func TestParseTrivyOutput_DescriptionTruncation(t *testing.T) {
	longTitle := strings.Repeat("x", 130)
	raw := `{"Results":[{"Target":"t","Vulnerabilities":[{` +
		`"VulnerabilityID":"CVE-T","PkgName":"pkg","Severity":"CRITICAL",` +
		`"Title":"` + longTitle + `","FixedVersion":"1.0"}]}]}`

	img, err := parseTrivyOutput("img", "ns", "now", []byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(img.CVEs) == 0 {
		t.Fatal("expected CVE in list")
	}
	desc := img.CVEs[0].Desc
	if len(desc) != 123 { // 120 chars + "..."
		t.Errorf("Desc len = %d, want 123 (120+...); value: %q", len(desc), desc)
	}
	if !strings.HasSuffix(desc, "...") {
		t.Errorf("Desc does not end with '...': %q", desc)
	}
}

// ---------------------------------------------------------------------------
// parseTrivyOutput — description falls back to Description when Title empty
// ---------------------------------------------------------------------------

func TestParseTrivyOutput_DescriptionFallback(t *testing.T) {
	const raw = `{"Results":[{"Target":"t","Vulnerabilities":[{
		"VulnerabilityID":"CVE-F","PkgName":"pkg","Severity":"CRITICAL",
		"Title":"","Description":"fallback description","FixedVersion":"2.0"}]}]}`

	img, err := parseTrivyOutput("img", "ns", "now", []byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(img.CVEs) == 0 {
		t.Fatal("expected CVE in list")
	}
	if img.CVEs[0].Desc != "fallback description" {
		t.Errorf("Desc = %q, want 'fallback description'", img.CVEs[0].Desc)
	}
}

// ---------------------------------------------------------------------------
// parseTrivyOutput — fix text when FixedVersion is empty
// ---------------------------------------------------------------------------

func TestParseTrivyOutput_FixNoVersion(t *testing.T) {
	const raw = `{"Results":[{"Target":"t","Vulnerabilities":[{
		"VulnerabilityID":"CVE-NF","PkgName":"pkg","Severity":"HIGH",
		"Title":"no fix","Description":"","FixedVersion":""}]}]}`

	img, err := parseTrivyOutput("img", "ns", "now", []byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(img.CVEs) == 0 {
		t.Fatal("expected CVE in list")
	}
	if img.CVEs[0].Fix != "No fix available yet" {
		t.Errorf("Fix = %q, want 'No fix available yet'", img.CVEs[0].Fix)
	}
}

// ---------------------------------------------------------------------------
// titleCase
// ---------------------------------------------------------------------------

func TestTitleCase(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"CRITICAL", "Critical"},
		{"critical", "Critical"},
		{"Critical", "Critical"},
		{"HIGH", "High"},
		{"high", "High"},
		{"MEDIUM", "Medium"},
		{"LOW", "Low"},
		{"", ""},
		{"X", "X"},
		{"xY", "Xy"},
	}

	for _, tc := range cases {
		got := titleCase(tc.in)
		if got != tc.want {
			t.Errorf("titleCase(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// fmtElapsed
// ---------------------------------------------------------------------------

func TestFmtElapsed(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{500 * time.Millisecond, "just now"},
		{999 * time.Millisecond, "just now"},
		{time.Second, "1s ago"},
		{30 * time.Second, "30s ago"},
		{59 * time.Second, "59s ago"},
		{time.Minute, "1m ago"},
		{5 * time.Minute, "5m ago"},
	}

	for _, tc := range cases {
		got := fmtElapsed(tc.d)
		if got != tc.want {
			t.Errorf("fmtElapsed(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// generateAIOptimization — all branches
// ---------------------------------------------------------------------------

func TestGenerateAIOptimization(t *testing.T) {
	cases := []struct {
		name      string
		imageName string
		result    *ScannedImage
		wantIssue string // substring that must appear in Issue
		wantFix   string // substring that must appear in Fix
	}{
		{
			name:      "all zero counts → optimal",
			imageName: "clean-image:v1",
			result:    &ScannedImage{},
			wantIssue: "None",
			wantFix:   "optimal",
		},
		{
			name:      "latest tag",
			imageName: "myapp:latest",
			result:    &ScannedImage{Critical: 1, High: 2},
			wantIssue: ":latest",
			wantFix:   "Pin",
		},
		{
			name:      "debian base",
			imageName: "debian:bullseye",
			result:    &ScannedImage{Critical: 3, High: 1},
			wantIssue: "Full OS",
			wantFix:   "distroless",
		},
		{
			name:      "ubuntu base",
			imageName: "ubuntu:22.04",
			result:    &ScannedImage{Critical: 1, High: 0},
			wantIssue: "Full OS",
			wantFix:   "distroless",
		},
		{
			name:      "bullseye keyword",
			imageName: "myapp:bullseye-slim",
			result:    &ScannedImage{Critical: 1, High: 0},
			wantIssue: "Full OS",
			wantFix:   "distroless",
		},
		{
			name:      "buster keyword",
			imageName: "myapp:buster",
			result:    &ScannedImage{Critical: 1, High: 0},
			wantIssue: "Full OS",
			wantFix:   "distroless",
		},
		{
			name:      "alpine with critical",
			imageName: "alpine:3.18",
			result:    &ScannedImage{Critical: 2, High: 0},
			wantIssue: "Alpine",
			wantFix:   "alpine base",
		},
		{
			name:      "alpine without critical",
			imageName: "alpine:3.18",
			result:    &ScannedImage{Critical: 0, High: 0, Medium: 3, Low: 5},
			wantIssue: "Alpine",
			wantFix:   "alpine patch",
		},
		{
			name:      "nginx image",
			imageName: "nginx:1.25",
			result:    &ScannedImage{Critical: 1, High: 2},
			wantIssue: "Ingress/proxy",
			wantFix:   "latest stable",
		},
		{
			name:      "ingress keyword",
			imageName: "registry.k8s.io/ingress-nginx:v1.9",
			result:    &ScannedImage{Critical: 0, High: 3},
			wantIssue: "Ingress/proxy",
			wantFix:   "latest stable",
		},
		{
			name:      "generic image with critical",
			imageName: "myapp:v1.0",
			result:    &ScannedImage{Critical: 5, High: 0},
			wantIssue: "critical",
			wantFix:   "Rebuild",
		},
		{
			name:      "generic image high+medium",
			imageName: "myapp:v1.0",
			result:    &ScannedImage{Critical: 0, High: 2, Medium: 4},
			wantIssue: "high",
			wantFix:   "rebuild",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			opt := generateAIOptimization(tc.imageName, tc.result)
			if !strings.Contains(strings.ToLower(opt.Issue), strings.ToLower(tc.wantIssue)) {
				t.Errorf("Issue = %q, want substring %q", opt.Issue, tc.wantIssue)
			}
			if !strings.Contains(strings.ToLower(opt.Fix), strings.ToLower(tc.wantFix)) {
				t.Errorf("Fix = %q, want substring %q", opt.Fix, tc.wantFix)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseTrivyOutput — AIOpt is populated
// ---------------------------------------------------------------------------

func TestParseTrivyOutput_AIOpt_IsPopulated(t *testing.T) {
	const raw = `{"Results":[]}`
	img, err := parseTrivyOutput("myimage:latest", "default", "now", []byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// latest tag with zero counts still triggers the "all zero" branch.
	if img.AIOpt.Issue == "" {
		t.Error("AIOpt.Issue should not be empty")
	}
	if img.AIOpt.Fix == "" {
		t.Error("AIOpt.Fix should not be empty")
	}
}
