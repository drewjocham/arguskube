package k8s

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LogEntry is a structured log line for the log explorer.
type LogEntry struct {
	Timestamp string `json:"time"`
	Message   string `json:"message"`
	Pod       string `json:"pod"`
	Namespace string `json:"namespace"`
	Container string `json:"container"`
	Node      string `json:"node"`
}

// LogQueryResult is the response from QueryLogs.
type LogQueryResult struct {
	Entries   []LogEntry    `json:"entries"`
	Total     int           `json:"total"`
	Fields    []string      `json:"fields"`
	Histogram []int         `json:"histogram"` // 50-bucket hit counts
}

// QueryLogs searches pod logs across the cluster with optional text filter.
func (c *Client) QueryLogs(ctx context.Context, query, namespace string, limit int) (*LogQueryResult, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	ns := namespace
	if ns == "" {
		ns = c.cfg.Kubernetes.Namespace
	}

	// List pods in the target namespace.
	pods, err := c.cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods for logs: %w", err)
	}

	var allEntries []LogEntry
	tailLines := int64(50) // per pod

	for i := range pods.Items {
		p := &pods.Items[i]
		if p.Status.Phase != corev1.PodRunning && p.Status.Phase != corev1.PodSucceeded {
			continue
		}

		nodeName := p.Spec.NodeName

		for _, cs := range p.Status.ContainerStatuses {
			if !cs.Ready && p.Status.Phase == corev1.PodRunning {
				continue
			}

			entries, err := c.fetchContainerLogs(ctx, p.Namespace, p.Name, cs.Name, nodeName, tailLines, query)
			if err != nil {
				c.logger.Debug("skipping container logs",
					"pod", p.Name, "container", cs.Name, "error", err,
				)
				continue
			}
			allEntries = append(allEntries, entries...)
		}

		if len(allEntries) >= limit*2 {
			break
		}
	}

	sortLogEntries(allEntries)

	if len(allEntries) > limit {
		allEntries = allEntries[:limit]
	}

	// Build histogram (50 buckets).
	histogram := buildHistogram(allEntries, 50)

	// Collect unique field names.
	fieldSet := map[string]bool{
		"kubernetes.pod_name":       true,
		"kubernetes.pod_namespace":  true,
		"kubernetes.container_name": true,
		"kubernetes.node_name":      true,
	}
	var fields []string
	for f := range fieldSet {
		fields = append(fields, f)
	}

	return &LogQueryResult{
		Entries:   allEntries,
		Total:     len(allEntries),
		Fields:    fields,
		Histogram: histogram,
	}, nil
}

// fetchContainerLogs retrieves and parses logs from a single container.
func (c *Client) fetchContainerLogs(ctx context.Context, namespace, podName, containerName, nodeName string, tailLines int64, query string) ([]LogEntry, error) {
	opts := &corev1.PodLogOptions{
		Container:  containerName,
		TailLines:  &tailLines,
		Timestamps: true,
	}

	req := c.cs.CoreV1().Pods(namespace).GetLogs(podName, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	var entries []LogEntry
	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 64*1024), 256*1024)

	lowerQuery := strings.ToLower(query)
	isWildcard := query == "" || query == "*"

	for scanner.Scan() {
		line := scanner.Text()

		// Kubernetes log format with --timestamps: "2024-01-15T10:30:00.123456789Z <message>"
		ts, msg := parseTimestampedLine(line)

		if !isWildcard && !strings.Contains(strings.ToLower(msg), lowerQuery) {
			continue
		}

		entries = append(entries, LogEntry{
			Timestamp: ts,
			Message:   msg,
			Pod:       podName,
			Namespace: namespace,
			Container: containerName,
			Node:      nodeName,
		})
	}

	return entries, scanner.Err()
}

// parseTimestampedLine splits a "timestamp message" line from Kubernetes logs.
func parseTimestampedLine(line string) (string, string) {
	// Try to find the RFC3339 timestamp at the beginning.
	if len(line) > 30 && line[4] == '-' && line[10] == 'T' {
		spaceIdx := strings.IndexByte(line, ' ')
		if spaceIdx > 20 && spaceIdx < 40 {
			return line[:spaceIdx], line[spaceIdx+1:]
		}
	}
	// Fallback: no parsable timestamp.
	return time.Now().UTC().Format(time.RFC3339Nano), line
}

// sortLogEntries sorts entries by timestamp descending.
func sortLogEntries(entries []LogEntry) {
	// Simple insertion sort (typically <500 entries).
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].Timestamp > entries[j-1].Timestamp; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
}

// buildHistogram distributes log entries into N time buckets.
func buildHistogram(entries []LogEntry, buckets int) []int {
	hist := make([]int, buckets)
	if len(entries) == 0 {
		return hist
	}

	// Parse first and last timestamps to determine range.
	now := time.Now()
	earliest := now.Add(-1 * time.Hour) // default 1h window

	if len(entries) > 0 {
		if t, err := time.Parse(time.RFC3339Nano, entries[len(entries)-1].Timestamp); err == nil {
			earliest = t
		}
	}

	span := now.Sub(earliest)
	if span <= 0 {
		span = time.Hour
	}
	bucketDur := span / time.Duration(buckets)

	for _, e := range entries {
		t, err := time.Parse(time.RFC3339Nano, e.Timestamp)
		if err != nil {
			continue
		}
		idx := int(t.Sub(earliest) / bucketDur)
		if idx < 0 {
			idx = 0
		}
		if idx >= buckets {
			idx = buckets - 1
		}
		hist[idx]++
	}

	return hist
}

// --- Node-level log streaming (kubelet, containerd) ---

// NodeLogEntry is a structured log line from a node's system services.
type NodeLogEntry struct {
	Timestamp string `json:"time"`
	Level     string `json:"level"`   // INFO, WARN, ERROR
	Service   string `json:"service"` // kubelet, containerd, kube-proxy, etc.
	Message   string `json:"message"`
}

// journalRe matches systemd journal lines: "May 01 14:32:01 hostname kubelet[1234]: message"
var journalRe = regexp.MustCompile(`^(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+\S+\s+(\S+?)(?:\[\d+\])?:\s+(.*)$`)

// GetNodeLogs fetches system service logs from a node via the kubelet proxy API.
// Services: kubelet, containerd, kube-proxy. Returns the most recent tailLines entries.
func (c *Client) GetNodeLogs(ctx context.Context, nodeName string, services []string, tailLines int) ([]NodeLogEntry, error) {
	if tailLines <= 0 {
		tailLines = 100
	}
	if len(services) == 0 {
		services = []string{"kubelet", "containerd", "kube-proxy"}
	}

	var allEntries []NodeLogEntry

	for _, svc := range services {
		entries, err := c.fetchNodeServiceLogs(ctx, nodeName, svc, tailLines)
		if err != nil {
			c.logger.WarnContext(ctx, "failed to fetch node service logs",
				"node", nodeName, "service", svc, "error", err)
			continue // Partial results are fine.
		}
		allEntries = append(allEntries, entries...)
	}

	// Sort by timestamp descending (newest first).
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Timestamp > allEntries[j].Timestamp
	})

	// Trim to tailLines.
	if len(allEntries) > tailLines {
		allEntries = allEntries[:tailLines]
	}

	return allEntries, nil
}

// fetchNodeServiceLogs reads logs for a single service from a node's journal via
// the kubelet proxy endpoint: /api/v1/nodes/{name}/proxy/logs/journal
// with query parameter ?unit={service}.
func (c *Client) fetchNodeServiceLogs(ctx context.Context, nodeName, service string, tailLines int) ([]NodeLogEntry, error) {
	// Use the kubelet /logs/journal endpoint with systemd unit filter.
	// Fallback: /logs/{service} for plain-file log systems.
	body, err := c.cs.CoreV1().RESTClient().Get().
		Resource("nodes").
		Name(nodeName).
		SubResource("proxy", "logs", "journal").
		Param("unit", service).
		Param("boot", "0"). 
		Do(ctx).
		Raw()

	if err != nil {
		body, err = c.cs.CoreV1().RESTClient().Get().
			Resource("nodes").
			Name(nodeName).
			SubResource("proxy", "logs", service+".log").
			Do(ctx).
			Raw()
		if err != nil {
			return nil, fmt.Errorf("node proxy logs for %s/%s: %w", nodeName, service, err)
		}
	}

	return parseNodeLogOutput(body, service, tailLines)
}

// parseNodeLogOutput parses raw log output from the kubelet proxy into structured entries.
func parseNodeLogOutput(data []byte, service string, tailLines int) ([]NodeLogEntry, error) {
	reader := strings.NewReader(string(data))
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	var entries []NodeLogEntry

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		entry := parseJournalLine(line, service)
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return entries, fmt.Errorf("scan node logs: %w", err)
	}

	// Take only the last tailLines entries.
	if len(entries) > tailLines {
		entries = entries[len(entries)-tailLines:]
	}

	return entries, nil
}

// parseJournalLine parses a single journal/syslog line into a NodeLogEntry.
func parseJournalLine(line, defaultService string) NodeLogEntry {
	entry := NodeLogEntry{
		Level:   "INFO",
		Service: defaultService,
	}

	// Try systemd journal format: "May 01 14:32:01 hostname kubelet[1234]: message"
	if m := journalRe.FindStringSubmatch(line); m != nil {
		entry.Timestamp = m[1]
		entry.Service = m[2]
		entry.Message = m[3]
	} else {
		// Try RFC3339 prefixed format.
		ts, msg := parseTimestampedLine(line)
		entry.Timestamp = ts
		entry.Message = msg
	}

	// Infer log level from message content.
	lower := strings.ToLower(entry.Message)
	switch {
	case strings.Contains(lower, "error") || strings.Contains(lower, "fatal") || strings.Contains(lower, "failed"):
		entry.Level = "ERROR"
	case strings.Contains(lower, "warn") || strings.Contains(lower, "timeout") || strings.Contains(lower, "delayed"):
		entry.Level = "WARN"
	}

	return entry
}

