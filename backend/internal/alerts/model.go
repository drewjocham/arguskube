package alerts

import "time"

// Severity levels for alerts.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

// Alert is an enriched Kubernetes alert with full context for AI diagnostics.
type Alert struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Severity  Severity  `json:"severity"`
	Namespace string    `json:"namespace"`
	Timestamp time.Time `json:"timestamp"`

	// Pod context
	PodName      string `json:"podName,omitempty"`
	PodPhase     string `json:"podPhase,omitempty"`
	RestartCount int32  `json:"restartCount"`
	ContainerID  string `json:"containerID,omitempty"`

	// Resource context
	MemoryLimit   string  `json:"memoryLimit,omitempty"`
	MemoryRequest string  `json:"memoryRequest,omitempty"`
	CPULimit      string  `json:"cpuLimit,omitempty"`
	CPURequest    string  `json:"cpuRequest,omitempty"`
	CPUThrottle   float64 `json:"cpuThrottle,omitempty"`

	// Node context (for node-level alerts)
	NodeName     string   `json:"nodeName,omitempty"`
	DiskUsage    float64  `json:"diskUsage,omitempty"`
	DiskCapacity string   `json:"diskCapacity,omitempty"`
	EvictedPods  []string `json:"evictedPods,omitempty"`

	// Deploy context
	ImageTag      string    `json:"imageTag,omitempty"`
	PreviousImage string    `json:"previousImage,omitempty"`
	DeployTime    time.Time `json:"deployTime,omitempty"`

	// Description and tags
	Description string `json:"description"`
	Tags        []Tag  `json:"tags"`

	// Cascade: IDs of related alerts
	RelatedAlerts []string `json:"relatedAlerts,omitempty"`
}

// Tag is a colored label on an alert.
type Tag struct {
	Label string `json:"label"`
	Color string `json:"color"` // "red", "blue", "amber", "purple", "teal"
}

// Diagnosis is the AI-generated output for an alert.
type Diagnosis struct {
	AlertID          string        `json:"alertId"`
	Hypothesis       string        `json:"hypothesis"`
	Confidence       float64       `json:"confidence"` // 0.0 - 1.0
	Steps            []RunbookStep `json:"steps"`
	DecisionLogEntry string        `json:"decisionLogEntry,omitempty"`
	CascadeNote      string        `json:"cascadeNote,omitempty"`
}

// RunbookStep is one remediation action.
type RunbookStep struct {
	Number  int    `json:"number"`
	Text    string `json:"text"`
	Command string `json:"command,omitempty"` // kubectl command, ready to paste
}

// ClusterMetrics are the top-level health numbers.
// All fields derived from the core K8s API — no metrics-server required.
type ClusterMetrics struct {
	PodHealthPct     float64 `json:"podHealthPct"`
	PodsRunning      int     `json:"podsRunning"`
	PodsTotal        int     `json:"podsTotal"`
	PodsPending      int     `json:"podsPending"`
	PodsFailed       int     `json:"podsFailed"`
	ErrorRate        float64 `json:"errorRate"`     // unhealthy containers / total
	ErrorRatePrev    float64 `json:"errorRatePrev"` // previous poll for trend
	RestartCount     int32   `json:"restartCount"`
	RestartTop       string  `json:"restartTop"`       // "payments-api: 32"
	WarningEvents    int     `json:"warningEvents"`    // warning events in last 30m
	TotalCPUMillis   int64   `json:"totalCpuMillis"`   // aggregate CPU requests
	TotalMemoryBytes int64   `json:"totalMemoryBytes"` // aggregate memory requests
	P99Latency       string  `json:"p99Latency"`       // from Prometheus if available
	SLOStatus        string  `json:"sloStatus"`        // "ok" or "breach"
}

// LogLine is a structured log entry for the live stream.
type LogLine struct {
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`
	Level     string    `json:"level"` // "error", "warn", "info", "ok"
	Message   string    `json:"message"`
}

// ServiceNode represents a node in the service topology map.
type ServiceNode struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "ok", "warn", "crit"
}

// TopologyEdge connects two service nodes.
type TopologyEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}
