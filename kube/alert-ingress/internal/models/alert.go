package models

import "time"

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

type AnomstackWebhook struct {
	Title          string         `json:"title"`
	Message        string         `json:"message"`
	MetricName     string         `json:"metric_name"`
	Threshold      float64        `json:"threshold"`
	AnomalyDetails map[string]any `json:"anomaly_details,omitempty"`
}

type ArgusAlert struct {
	ID          string            `json:"id"`
	Source      string            `json:"source"`
	Severity    Severity          `json:"severity"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	MetricName  string            `json:"metric_name"`
	MetricValue float64           `json:"metric_value,omitempty"`
	Score       float64           `json:"score"`
	Threshold   float64           `json:"threshold"`
	Labels      map[string]string `json:"labels,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
}
