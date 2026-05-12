package models

import "time"

type DriftFilter struct {
	Resolved *bool
	Severity string
	Category string
	Limit    int
	Page     int
}

func (f *DriftFilter) Offset() int {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}
	return (f.Page - 1) * f.Limit
}

type DriftReport struct {
	ID         string        `json:"id"`
	SpecID     string        `json:"spec_id"`
	EndpointID string        `json:"endpoint_id,omitempty"`
	Severity   string        `json:"severity"`
	Category   string        `json:"category"`
	Score      float64       `json:"score"`
	Source     string        `json:"source"`
	Observed   *ObservedData `json:"observed,omitempty"`
	Expected   any           `json:"expected,omitempty"`
	Actual     any           `json:"actual,omitempty"`
	Suggestion string        `json:"suggestion,omitempty"`
	Resolved   bool          `json:"resolved"`
	CreatedAt  time.Time     `json:"created_at"`
	ResolvedAt *time.Time    `json:"resolved_at,omitempty"`
}

type DriftSummary struct {
	SpecID        string         `json:"spec_id"`
	SpecName      string         `json:"spec_name,omitempty"`
	TotalDrifts   int            `json:"total_drifts"`
	CriticalCount int            `json:"critical_count"`
	HighCount     int            `json:"high_count"`
	AvgScore      float64        `json:"avg_score"`
	LastDetected  time.Time      `json:"last_detected"`
	ByCategory    map[string]int `json:"by_category"`
}

type ObservedData struct {
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	StatusCode int               `json:"status_code"`
	Request    map[string]any    `json:"request,omitempty"`
	Response   map[string]any    `json:"response,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Timestamp  time.Time         `json:"timestamp"`
}
