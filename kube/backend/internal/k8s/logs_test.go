package k8s

import (
	"strings"
	"testing"
	"time"
)

// --- inferLogLevel ----------------------------------------------------------

func TestInferLogLevel(t *testing.T) {
	cases := []struct {
		line, want string
	}{
		{"connection refused: ERROR opening socket", "error"},
		{"FATAL: out of memory", "error"},
		{"goroutine 1 [running]: panic in main", "error"},
		{"WARN: timeout exceeded", "warning"},
		{"DEBUG: enter handler", "debug"},
		{"TRACE: function entered", "debug"},
		{"OK serving on :8080", "info"},
		{"", "info"},
	}
	for _, c := range cases {
		if got := inferLogLevel(c.line); got != c.want {
			t.Errorf("inferLogLevel(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}

// --- parseTimestampedLine ---------------------------------------------------

func TestParseTimestampedLine_RFC3339(t *testing.T) {
	line := "2026-05-09T12:34:56.789012345Z hello world"
	ts, msg := parseTimestampedLine(line)
	if !strings.HasPrefix(ts, "2026-05-09T") {
		t.Errorf("unexpected timestamp: %q", ts)
	}
	if msg != "hello world" {
		t.Errorf("unexpected message: %q", msg)
	}
}

func TestParseTimestampedLine_NoTimestamp(t *testing.T) {
	ts, msg := parseTimestampedLine("just a plain line")
	// Falls back to "now" — assert the timestamp parses as RFC3339Nano.
	if _, err := time.Parse(time.RFC3339Nano, ts); err != nil {
		t.Errorf("fallback timestamp doesn't parse: %q (%v)", ts, err)
	}
	if msg != "just a plain line" {
		t.Errorf("unexpected message: %q", msg)
	}
}

func TestParseTimestampedLine_TooShort(t *testing.T) {
	ts, msg := parseTimestampedLine("short")
	if _, err := time.Parse(time.RFC3339Nano, ts); err != nil {
		t.Errorf("expected RFC3339 fallback, got %q", ts)
	}
	if msg != "short" {
		t.Errorf("expected message preserved, got %q", msg)
	}
}

// --- sortLogEntries ---------------------------------------------------------

func TestSortLogEntries_DescendingByTimestamp(t *testing.T) {
	entries := []LogEntry{
		{Timestamp: "2026-05-09T10:00:00Z", Message: "old"},
		{Timestamp: "2026-05-09T12:00:00Z", Message: "newer"},
		{Timestamp: "2026-05-09T11:00:00Z", Message: "middle"},
	}
	sortLogEntries(entries)
	if entries[0].Message != "newer" || entries[1].Message != "middle" || entries[2].Message != "old" {
		t.Errorf("unexpected order: %+v", entries)
	}
}

func TestSortLogEntries_Empty(t *testing.T) {
	var entries []LogEntry
	sortLogEntries(entries) // must not panic
}

// --- buildHistogram ---------------------------------------------------------

func TestBuildHistogram_EmptyEntries(t *testing.T) {
	h := buildHistogram(nil, 50)
	if len(h) != 50 {
		t.Errorf("expected 50 buckets, got %d", len(h))
	}
	for _, v := range h {
		if v != 0 {
			t.Errorf("expected zero buckets for empty input, got %v", h)
			break
		}
	}
}

func TestBuildHistogram_ParseableTimestamps(t *testing.T) {
	now := time.Now().UTC()
	entries := []LogEntry{
		// Newest first (the actual sortLogEntries order).
		{Timestamp: now.Format(time.RFC3339Nano)},
		{Timestamp: now.Add(-30 * time.Minute).Format(time.RFC3339Nano)},
		// Oldest is the LAST entry — buildHistogram uses entries[len-1] as the
		// span anchor.
		{Timestamp: now.Add(-1 * time.Hour).Format(time.RFC3339Nano)},
	}
	h := buildHistogram(entries, 10)
	if len(h) != 10 {
		t.Fatalf("expected 10 buckets, got %d", len(h))
	}
	total := 0
	for _, v := range h {
		total += v
	}
	if total != len(entries) {
		t.Errorf("histogram total = %d, want %d", total, len(entries))
	}
}

func TestBuildHistogram_BadTimestamps(t *testing.T) {
	entries := []LogEntry{
		{Timestamp: "not a date"},
		{Timestamp: "also bad"},
	}
	h := buildHistogram(entries, 5)
	if len(h) != 5 {
		t.Fatalf("expected 5 buckets, got %d", len(h))
	}
	for _, v := range h {
		if v != 0 {
			t.Errorf("expected zero buckets when timestamps unparseable, got %v", h)
			break
		}
	}
}

// --- parseJournalLine -------------------------------------------------------

func TestParseJournalLine_SystemdFormat(t *testing.T) {
	line := "May 01 14:32:01 host123 kubelet[1234]: pod /default/api created"
	e := parseJournalLine(line, "fallback-svc")
	if e.Service != "kubelet" {
		t.Errorf("expected kubelet, got %q", e.Service)
	}
	if e.Timestamp != "May 01 14:32:01" {
		t.Errorf("unexpected timestamp: %q", e.Timestamp)
	}
	if e.Message != "pod /default/api created" {
		t.Errorf("unexpected message: %q", e.Message)
	}
	if e.Level != "INFO" {
		t.Errorf("expected INFO for benign message, got %q", e.Level)
	}
}

func TestParseJournalLine_LevelInference(t *testing.T) {
	cases := []struct {
		msg, wantLevel string
	}{
		{"May 01 14:32:01 host kubelet[1]: failed to start container", "ERROR"},
		{"May 01 14:32:01 host kubelet[1]: connection timeout reached", "WARN"},
		{"May 01 14:32:01 host kubelet[1]: container started", "INFO"},
	}
	for _, c := range cases {
		if got := parseJournalLine(c.msg, "x").Level; got != c.wantLevel {
			t.Errorf("level for %q = %q, want %q", c.msg, got, c.wantLevel)
		}
	}
}

func TestParseJournalLine_FallsBackToRFC3339(t *testing.T) {
	line := "2026-05-09T12:34:56.0Z plain RFC3339-prefixed line"
	e := parseJournalLine(line, "kubelet")
	if e.Service != "kubelet" {
		t.Errorf("expected service from default, got %q", e.Service)
	}
	if !strings.HasPrefix(e.Timestamp, "2026-05-09T") {
		t.Errorf("expected RFC3339 timestamp, got %q", e.Timestamp)
	}
}

// --- parseNodeLogOutput -----------------------------------------------------

func TestParseNodeLogOutput_TailLimit(t *testing.T) {
	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, "May 01 14:32:0"+string(rune('0'+i))+" host kubelet[1]: line "+string(rune('0'+i)))
	}
	body := []byte(strings.Join(lines, "\n"))

	all, err := parseNodeLogOutput(body, "kubelet", 100)
	if err != nil {
		t.Fatalf("parseNodeLogOutput: %v", err)
	}
	if len(all) != 10 {
		t.Errorf("expected 10 entries, got %d", len(all))
	}

	// Tail to 3 → should keep the last 3 of the 10.
	last3, err := parseNodeLogOutput(body, "kubelet", 3)
	if err != nil {
		t.Fatalf("parseNodeLogOutput tail: %v", err)
	}
	if len(last3) != 3 {
		t.Fatalf("expected 3 tailed entries, got %d", len(last3))
	}
	if !strings.Contains(last3[2].Message, "9") {
		t.Errorf("expected last entry to contain '9', got %q", last3[2].Message)
	}
}

func TestParseNodeLogOutput_SkipsBlankLines(t *testing.T) {
	body := []byte("\nMay 01 14:32:01 host kubelet[1]: ok\n\n\nMay 01 14:32:02 host kubelet[1]: also ok\n\n")
	entries, err := parseNodeLogOutput(body, "kubelet", 100)
	if err != nil {
		t.Fatalf("parseNodeLogOutput: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries (blanks skipped), got %d (%+v)", len(entries), entries)
	}
}

func TestParseNodeLogOutput_DefaultsServiceName(t *testing.T) {
	body := []byte("plain line with no journal pattern\n")
	entries, err := parseNodeLogOutput(body, "containerd", 100)
	if err != nil {
		t.Fatalf("parseNodeLogOutput: %v", err)
	}
	if len(entries) != 1 || entries[0].Service != "containerd" {
		t.Errorf("expected fallback service=containerd, got %+v", entries)
	}
}
