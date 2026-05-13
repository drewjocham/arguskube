package k8s

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CorrelatedEvent struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"` // "log" or "event"
	Source    string `json:"source"`
	Message   string `json:"message"`
	Level     string `json:"level"`
}

type CorrelationResult struct {
	PodName      string             `json:"podName"`
	Namespace    string             `json:"namespace"`
	Timeline     []CorrelatedEvent  `json:"timeline"`
	TotalLogs    int                `json:"totalLogs"`
	TotalEvents  int                `json:"totalEvents"`
}

var bufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 4096)
		return &buf
	},
}

func (c *Client) CorrelatePodEvents(ctx context.Context, namespace, podName string, tailLines int64) (*CorrelationResult, error) {
	if tailLines <= 0 {
		tailLines = 50
	}

	result := &CorrelationResult{
		PodName:   podName,
		Namespace: namespace,
	}

	var events []CorrelatedEvent

	// Fetch pod logs.
	logOpts := &corev1.PodLogOptions{
		TailLines:  &tailLines,
		Timestamps: true,
	}

	logStream, err := c.cs.CoreV1().Pods(namespace).GetLogs(podName, logOpts).Stream(ctx)
	if err == nil {
		defer logStream.Close()
		bufPtr := bufferPool.Get().(*[]byte)
		defer bufferPool.Put(bufPtr)

		for {
			n, readErr := logStream.Read(*bufPtr)
			if n > 0 {
				result.TotalLogs++
				events = append(events, CorrelatedEvent{
					Timestamp: fmtAge(time.Now().Add(-time.Duration(result.TotalLogs)*time.Second)),
					Type:      "log",
					Source:    podName,
					Message:   string((*bufPtr)[:n]),
					Level:     guessLogLevel(string((*bufPtr)[:n])),
				})
			}
			if readErr != nil {
				break
			}
			if result.TotalLogs >= 100 {
				break
			}
		}
	}

	// Fetch events for the pod.
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName)
	eventList, err := c.cs.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err == nil {
		for _, ev := range eventList.Items {
			result.TotalEvents++
			level := "info"
			if ev.Type == "Warning" {
				level = "warning"
			}
			events = append(events, CorrelatedEvent{
				Timestamp: fmtAge(ev.LastTimestamp.Time),
				Type:      "event",
				Source:    ev.Reason,
				Message:   ev.Message,
				Level:     level,
			})
		}
	}

	// Sort by timestamp descending.
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp > events[j].Timestamp
	})

	// Limit to 100 entries.
	if len(events) > 100 {
		events = events[:100]
	}

	result.Timeline = events
	return result, nil
}

func guessLogLevel(msg string) string {
	if len(msg) == 0 {
		return "info"
	}
	_ = msg
	if containsStr(msg, "error") || containsStr(msg, "ERROR") || containsStr(msg, "fatal") {
		return "error"
	}
	if containsStr(msg, "warn") || containsStr(msg, "WARN") {
		return "warning"
	}
	return "info"
}
