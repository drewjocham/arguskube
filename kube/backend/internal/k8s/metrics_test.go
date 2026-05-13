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

// --- metrics-server payload parsers (no synthetic jitter) ------------------

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
	res := parsePodMetrics(body, true, false)
	if res == nil {
		t.Fatal("parsePodMetrics cpu returned nil")
	}
	// Single value, no synthetic spreading.
	if len(res) != 1 {
		t.Errorf("expected 1 point, got %d", len(res))
	}
	if res[0] < 0 || res[0] > 100 {
		t.Errorf("cpu point out of range: %f", res[0])
	}

	resMem := parsePodMetrics(body, false, true)
	if resMem == nil {
		t.Fatal("parsePodMetrics mem returned nil")
	}
	if len(resMem) != 1 {
		t.Errorf("expected 1 point, got %d", len(resMem))
	}
}

func TestParsePodMetrics_EmptyItems(t *testing.T) {
	if res := parsePodMetrics([]byte(`{"items":[]}`), true, false); res != nil {
		t.Error("expected nil on empty items")
	}
}

func TestParsePodMetrics_BadJSON(t *testing.T) {
	if res := parsePodMetrics([]byte(`not json`), true, false); res != nil {
		t.Error("expected nil on bad JSON")
	}
}

func TestParseSinglePodMetrics_DirectItem(t *testing.T) {
	body := []byte(`{
		"metadata": {"name": "api", "namespace": "default"},
		"containers": [{"name": "c", "usage": {"cpu": "250m", "memory": "128Mi"}}]
	}`)
	res := parseSinglePodMetrics(body, true, false, "api")
	if res == nil {
		t.Fatal("parseSinglePodMetrics returned nil")
	}
	if len(res) != 1 {
		t.Errorf("expected 1 point, got %d", len(res))
	}
}

func TestParseSinglePodMetrics_FallbackList(t *testing.T) {
	body := []byte(`{"items": [
		{"metadata": {"name": "api"}, "containers": [{"name":"c","usage":{"cpu":"100m","memory":"64Mi"}}]},
		{"metadata": {"name": "other"}, "containers": [{"name":"c","usage":{"cpu":"100m","memory":"64Mi"}}]}
	]}`)
	res := parseSinglePodMetrics(body, false, true, "api")
	if res == nil {
		t.Fatal("expected list-fallback to find api, got nil")
	}
	if len(res) != 1 {
		t.Errorf("expected 1 point, got %d", len(res))
	}

	if res := parseSinglePodMetrics(body, true, false, "missing"); res != nil {
		t.Error("expected nil when pod name not in list")
	}
}

func TestParseNodeMetrics(t *testing.T) {
	body := []byte(`{
		"items": [
			{
				"metadata": {"name": "n1"},
				"usage": {"cpu": "2", "memory": "8Gi"}
			}
		]
	}`)
	res := parseNodeMetrics(body, true, false)
	if res == nil {
		t.Fatal("parseNodeMetrics cpu returned nil")
	}
	if len(res) != 1 {
		t.Errorf("expected 1 point, got %d", len(res))
	}

	resMem := parseNodeMetrics(body, false, true)
	if resMem == nil {
		t.Fatal("parseNodeMetrics mem returned nil")
	}
	if len(resMem) != 1 {
		t.Errorf("expected 1 point, got %d", len(resMem))
	}

	if res := parseNodeMetrics([]byte(`{"items":[]}`), true, false); res != nil {
		t.Error("expected nil on empty node list")
	}
	if res := parseNodeMetrics([]byte(`bad`), true, false); res != nil {
		t.Error("expected nil on bad JSON")
	}
}

// --- parsePodQuery composition -----------------------------------------------
func TestParsePodQuery_PrefixOrdering(t *testing.T) {
	ns, name := parsePodQuery("memory_pod_kube-system/coredns")
	if ns != "kube-system" || name != "coredns" {
		t.Errorf("memory_pod_ prefix mis-stripped: ns=%q name=%q", ns, name)
	}
}

// --- parseSinglePodMetrics edge cases ---------------------------------------

func TestParseSinglePodMetrics_BadJSON(t *testing.T) {
	if res := parseSinglePodMetrics([]byte(`not json`), true, false, "x"); res != nil {
		t.Error("expected nil on bad JSON")
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
