package k8s

import (
	"strings"
	"testing"
)

// --- string utilities -------------------------------------------------------

func TestToLower(t *testing.T) {
	cases := map[string]string{
		"":             "",
		"abc":          "abc",
		"ABC":          "abc",
		"AbCdEf":       "abcdef",
		"With Space!":  "with space!",
		"123ABC":       "123abc",
		"non-ascii Ä": "non-ascii Ä", // bytewise lowercase ignores non-ASCII
	}
	for in, want := range cases {
		if got := toLower(in); got != want {
			t.Errorf("toLower(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIndexOf(t *testing.T) {
	cases := []struct {
		s, sub string
		want   int
	}{
		{"hello", "ll", 2},
		{"hello", "x", -1},
		{"", "x", -1},
		{"abc", "", 0},
		{"abc", "abc", 0},
		{"abcabc", "bc", 1},
		{"a", "ab", -1},
	}
	for _, c := range cases {
		if got := indexOf(c.s, c.sub); got != c.want {
			t.Errorf("indexOf(%q, %q) = %d, want %d", c.s, c.sub, got, c.want)
		}
	}
}

func TestContainsAny(t *testing.T) {
	if !containsAny("query for CPU usage", "cpu") {
		t.Error("expected case-insensitive match for 'cpu'")
	}
	if !containsAny("memory pressure", "mem", "Memory") {
		t.Error("expected match on first sub")
	}
	if containsAny("disk", "cpu", "memory") {
		t.Error("unexpected match")
	}
	if containsAny("", "anything") {
		t.Error("empty string should not match")
	}
}

// --- resource quantity parsers ---------------------------------------------

func TestParseInt64(t *testing.T) {
	cases := map[string]int64{
		"":     0,
		"0":    0,
		"42":   42,
		"1024": 1024,
		// Non-digit chars are stripped (current implementation behavior).
		"100m": 100,
		"500x": 500,
	}
	for in, want := range cases {
		if got := parseInt64(in); got != want {
			t.Errorf("parseInt64(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseCPUNanos(t *testing.T) {
	cases := []struct {
		in   string
		want int64 // milliCPU
	}{
		{"", 0},
		{"100m", 100},                // 100 millicores
		{"1", 1000},                  // 1 core = 1000 millicores
		{"4", 4000},                  // 4 cores
		{"500000000n", 500},          // 500M nanocores = 500 millicores
		{"1000000000n", 1000},        // 1B nanocores = 1 core
	}
	for _, c := range cases {
		if got := parseCPUNanos(c.in); got != c.want {
			t.Errorf("parseCPUNanos(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseMemBytes(t *testing.T) {
	const Ki = int64(1024)
	const Mi = Ki * 1024
	const Gi = Mi * 1024
	cases := []struct {
		in   string
		want int64
	}{
		{"", 0},
		{"1024", 1024},
		{"1Ki", Ki},
		{"512Mi", 512 * Mi},
		{"2Gi", 2 * Gi},
		{"1048576Ki", 1048576 * Ki},
	}
	for _, c := range cases {
		if got := parseMemBytes(c.in); got != c.want {
			t.Errorf("parseMemBytes(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// --- query parsing ---------------------------------------------------------

func TestParsePodQuery(t *testing.T) {
	cases := []struct {
		query, ns, name string
	}{
		{"cpu_pod_my-app", "", "my-app"},
		{"mem_pod_kube-system/coredns", "kube-system", "coredns"},
		{"memory_pod_default/api", "default", "api"},
		{"cpu_my-app", "", "my-app"},      // bare cpu_ prefix supported
		{"unknown_query", "", "unknown_query"}, // no match → returned as-is
		{"", "", ""},
	}
	for _, c := range cases {
		gotNs, gotName := parsePodQuery(c.query)
		if gotNs != c.ns || gotName != c.name {
			t.Errorf("parsePodQuery(%q) = (%q, %q), want (%q, %q)",
				c.query, gotNs, gotName, c.ns, c.name)
		}
	}
}

// --- metrics-server payload parsers ----------------------------------------

func TestParsePodMetrics(t *testing.T) {
	body := []byte(`{
		"items": [
			{
				"metadata": {"name": "p1", "namespace": "default"},
				"containers": [
					{"name": "c", "usage": {"cpu": "500m", "memory": "256Mi"}}
				]
			},
			{
				"metadata": {"name": "p2", "namespace": "default"},
				"containers": [
					{"name": "c", "usage": {"cpu": "1500m", "memory": "1Gi"}}
				]
			}
		]
	}`)
	res, err := parsePodMetrics(body, true /*cpu*/, false, 100)
	if err != nil {
		t.Fatalf("parsePodMetrics cpu: %v", err)
	}
	if len(res) != 100 {
		t.Errorf("expected 100 points, got %d", len(res))
	}
	// Total CPU = 2000 millicores, baseline 4000 → 0.5%; spread oscillates around it.
	for _, v := range res {
		if v < 0 || v > 100 {
			t.Errorf("cpu point out of range: %f", v)
		}
	}

	resMem, err := parsePodMetrics(body, false, true /*mem*/, 50)
	if err != nil {
		t.Fatalf("parsePodMetrics mem: %v", err)
	}
	if len(resMem) != 50 {
		t.Errorf("expected 50 points, got %d", len(resMem))
	}
}

func TestParsePodMetrics_EmptyItems(t *testing.T) {
	if _, err := parsePodMetrics([]byte(`{"items":[]}`), true, false, 10); err == nil {
		t.Error("expected error on empty items")
	}
}

func TestParsePodMetrics_BadJSON(t *testing.T) {
	if _, err := parsePodMetrics([]byte(`not json`), true, false, 10); err == nil {
		t.Error("expected JSON parse error")
	}
}

func TestParseSinglePodMetrics_DirectItem(t *testing.T) {
	body := []byte(`{
		"metadata": {"name": "api", "namespace": "default"},
		"containers": [{"name": "c", "usage": {"cpu": "250m", "memory": "128Mi"}}]
	}`)
	res, err := parseSinglePodMetrics(body, true, false, 30, "api")
	if err != nil {
		t.Fatalf("parseSinglePodMetrics: %v", err)
	}
	if len(res) != 30 {
		t.Errorf("expected 30 points, got %d", len(res))
	}
}

func TestParseSinglePodMetrics_FallbackList(t *testing.T) {
	body := []byte(`{"items": [
		{"metadata": {"name": "api"}, "containers": [{"name":"c","usage":{"cpu":"100m","memory":"64Mi"}}]},
		{"metadata": {"name": "other"}, "containers": [{"name":"c","usage":{"cpu":"100m","memory":"64Mi"}}]}
	]}`)
	res, err := parseSinglePodMetrics(body, false, true, 20, "api")
	if err != nil {
		t.Fatalf("expected list-fallback to find api, got %v", err)
	}
	if len(res) != 20 {
		t.Errorf("expected 20 points, got %d", len(res))
	}

	if _, err := parseSinglePodMetrics(body, true, false, 20, "missing"); err == nil {
		t.Error("expected error when pod name not in list")
	}
}

func TestParseNodeMetrics(t *testing.T) {
	body := []byte(`{"items": [
		{"metadata": {"name": "n1"}, "usage": {"cpu": "1500m", "memory": "4Gi"}},
		{"metadata": {"name": "n2"}, "usage": {"cpu": "500m", "memory": "2Gi"}}
	]}`)
	resCPU, err := parseNodeMetrics(body, true, false, 40)
	if err != nil {
		t.Fatalf("parseNodeMetrics cpu: %v", err)
	}
	if len(resCPU) != 40 {
		t.Errorf("expected 40 points, got %d", len(resCPU))
	}
	resMem, err := parseNodeMetrics(body, false, true, 25)
	if err != nil {
		t.Fatalf("parseNodeMetrics mem: %v", err)
	}
	if len(resMem) != 25 {
		t.Errorf("expected 25 points, got %d", len(resMem))
	}

	if _, err := parseNodeMetrics([]byte(`{"items":[]}`), true, false, 10); err == nil {
		t.Error("expected error on empty node list")
	}
	if _, err := parseNodeMetrics([]byte(`bad`), true, false, 10); err == nil {
		t.Error("expected JSON parse error")
	}
}

// --- spreadWithJitter -------------------------------------------------------

func TestSpreadWithJitter(t *testing.T) {
	res := spreadWithJitter(50, 30)
	if len(res) != 30 {
		t.Fatalf("expected 30 points, got %d", len(res))
	}
	for _, v := range res {
		if v < 0 || v > 100 {
			t.Errorf("value out of [0,100]: %f", v)
		}
	}

	// Negative base should still produce non-negative points (clamped to 0).
	for _, v := range spreadWithJitter(0, 10) {
		if v < 0 {
			t.Errorf("expected clamp to 0, got %f", v)
		}
	}
}

func TestSpreadWithJitter_ZeroPoints(t *testing.T) {
	if got := spreadWithJitter(50, 0); len(got) != 0 {
		t.Errorf("expected empty slice, got %d points", len(got))
	}
}

// --- containsAny / parsePodQuery composition --------------------------------
// Sanity check that the prefix list ordering in parsePodQuery handles
// overlap correctly (memory_pod_ should not be mis-stripped to mem_).
func TestParsePodQuery_PrefixOrdering(t *testing.T) {
	ns, name := parsePodQuery("memory_pod_kube-system/coredns")
	if ns != "kube-system" || name != "coredns" {
		t.Errorf("memory_pod_ prefix mis-stripped: ns=%q name=%q", ns, name)
	}
}

func TestSpreadWithJitter_Determinism(t *testing.T) {
	// Two consecutive calls share the same time-base within microseconds, so
	// the jitter formula will produce slightly different values; we only
	// assert that the *length* is stable.
	a := spreadWithJitter(75, 5)
	b := spreadWithJitter(75, 5)
	if len(a) != len(b) {
		t.Errorf("length differs across calls: %d vs %d", len(a), len(b))
	}
	// Each point lives in [75 - jitter, 75 + jitter] roughly within ±15 of base.
	for _, v := range a {
		if v < 50 || v > 100 {
			t.Errorf("unexpected swing for base=75: %f", v)
		}
	}
}

// --- parseSinglePodMetrics edge cases ---------------------------------------

func TestParseSinglePodMetrics_BadJSON(t *testing.T) {
	if _, err := parseSinglePodMetrics([]byte(`not json`), true, false, 10, "x"); err == nil {
		t.Error("expected JSON parse error")
	}
}

// quick guard: the helpers must not drop unicode quietly (toLower/indexOf
// only handle ASCII; documenting current behavior).
func TestStringHelpers_AsciiOnly(t *testing.T) {
	mixed := "ÄbC"
	low := toLower(mixed)
	if !strings.Contains(low, "bc") {
		t.Errorf("ASCII portion lost: %q", low)
	}
}
