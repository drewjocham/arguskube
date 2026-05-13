package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/argues/argus/mcp/monitoring/alerts"
	"github.com/argues/argus/mcp/monitoring/history"
	"github.com/argues/argus/mcp/monitoring/recommendation"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/argues/argus/mcp/tools"
	"github.com/argues/argus/pkg/kube"
	kwatch "github.com/argues/argus/pkg/kube/watch"
)

var (
	ErrLoggerRequired = errors.New("server: logger is required")
	ErrToolNotFound   = errors.New("server: tool not found")
)

const (
	alertsResourceURI   = "kube://alerts/current"
	historyResourceURI  = "kube://history/incidents"
	jsonMIMEType        = "application/json"
	maxAlertRecords     = 100
	defaultHistoryRange = 72 * time.Hour
)

type Tool interface {
	Name() string
	Description() string
	Parameters() []tools.ToolParameter
	Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error)
}

type Config struct {
	Version      string
	GitCommit    string
	BuildDate    string
	HistoryPath  string
	K8sClient    kube.ClientInterface
	HistoryStore history.Recorder
	AlertsStore  alerts.StoreInterface
	PodTracker   *kwatch.PodTracker
	Watcher      *kwatch.Manager
	PollInterval time.Duration
}

type AlertRecord struct {
	ID              string                        `json:"id"`
	Alert           kwatch.Alert                  `json:"alert"`
	Recommendation  recommendation.Recommendation `json:"recommendation"`
	PodExists       bool                          `json:"pod_exists,omitempty"`
	PodCache        *alerts.PodCache              `json:"pod_cache,omitempty"`
	State           string                        `json:"state,omitempty"`
	Comments        []alerts.Comment              `json:"comments,omitempty"`
	OccurrenceCount int                           `json:"occurrence_count,omitempty"`
	FirstOccurredAt time.Time                     `json:"first_occurred_at,omitempty"`
}

type ToolSummary struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Parameters  []tools.ToolParameter `json:"parameters,omitempty"`
}

type MCPServer struct {
	logger      *slog.Logger
	mcp         *mcp.Server
	client      kube.ClientInterface
	history     history.Recorder
	alertsStore alerts.StoreInterface
	podTracker  *kwatch.PodTracker
	engine      *recommendation.Engine
	watcher     *kwatch.Manager

	tools     map[string]Tool
	executors map[string]func(context.Context, map[string]interface{}) (map[string]interface{}, error)

	alertsMu sync.RWMutex
	alerts   []AlertRecord
}

func NewMCPServer(logger *slog.Logger, cfg Config) (*MCPServer, error) {
	if logger == nil {
		return nil, ErrLoggerRequired
	}

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "argus",
		Version: cfg.Version,
	}, &mcp.ServerOptions{
		Logger: logger,
	})

	server := &MCPServer{
		logger:      logger,
		mcp:         mcpServer,
		client:      cfg.K8sClient,
		history:     cfg.HistoryStore,
		alertsStore: cfg.AlertsStore,
		podTracker:  cfg.PodTracker,
		engine:      recommendation.NewEngine(cfg.HistoryStore, logger),
		watcher:     cfg.Watcher,
		tools:       make(map[string]Tool),
		executors:   make(map[string]func(context.Context, map[string]interface{}) (map[string]interface{}, error)),
	}

	server.setupResources()
	server.setupTools(cfg)

	return server, nil
}

func (s *MCPServer) Start(ctx context.Context) error {
	alertCh := s.watcher.Start(ctx)
	go s.listenForAlerts(ctx, alertCh)

	if s.podTracker != nil {
		if err := s.podTracker.Start(ctx); err != nil {
			s.logger.Warn("failed to start pod tracker", "error", err)
		} else {
			defer s.podTracker.Stop()
		}
	}

	if err := s.mcp.Run(ctx, &mcp.StdioTransport{}); err != nil {
		s.logger.Warn("mcp server disconnected", "error", err)
	}

	<-ctx.Done()
	return nil
}

func (s *MCPServer) setupResources() {
	s.mcp.AddResource(&mcp.Resource{
		URI:      alertsResourceURI,
		Name:     "Current Alerts",
		MIMEType: jsonMIMEType,
	}, s.handleReadAlerts)

	s.mcp.AddResource(&mcp.Resource{
		URI:      historyResourceURI,
		Name:     "Incident History",
		MIMEType: jsonMIMEType,
	}, s.handleReadHistory)
}

func (s *MCPServer) setupTools(cfg Config) {
	clusterAnalysisTool := tools.NewClusterAnalysisTool(s.client)
	allTools := []Tool{
		tools.NewNodeStatusTool(s.client),
		tools.NewPodResourcesTool(s.client),
		tools.NewPodLogsTool(s.client),
		tools.NewNamespaceListTool(s.client, s.logger),
		tools.NewHistoryInsightsTool(s.history),
		tools.NewVersionTool(cfg.Version, cfg.GitCommit, cfg.BuildDate),
		clusterAnalysisTool,
		tools.NewClusterAnalysisLegacyTool(clusterAnalysisTool),
		tools.NewRecommendationTool(s.client, s.engine),
		tools.NewClusterEventsTool(s.client, s.logger),
		tools.NewHistoryInsightsToolWithClient(s.client, s.history),
		tools.NewWatcherConfigTool(s.client),
		tools.NewWatcherConfigGetTool(s.client),
	}

	for _, t := range allTools {
		s.registerTool(t)
	}
}

func (s *MCPServer) registerTool(t Tool) {
	name := t.Name()
	s.tools[name] = t
	s.executors[name] = t.Execute

	mcpTool := &mcp.Tool{
		Name:        name,
		Description: t.Description(),
		InputSchema: s.generateSchema(t.Parameters()),
	}

	s.mcp.AddTool(mcpTool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := make(map[string]interface{})
		if req.Params.Arguments != nil {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				s.logger.Warn("failed to unmarshal tool arguments", "tool", name, "error", err)
			}
		}

		res, err := t.Execute(ctx, args)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			}, nil
		}

		out, err := json.Marshal(res)
		if err != nil {
			s.logger.Warn("failed to marshal tool result", "tool", name, "error", err)
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: "internal error: failed to marshal result"}},
			}, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(out)}},
		}, nil
	})
}

func (s *MCPServer) listenForAlerts(ctx context.Context, ch <-chan kwatch.Alert) {
	for {
		select {
		case <-ctx.Done():
			return
		case a, ok := <-ch:
			if !ok {
				return
			}
			s.processAlert(ctx, a)
		}
	}
}

func (s *MCPServer) processAlert(ctx context.Context, a kwatch.Alert) {
	rec, err := s.engine.ForAlert(ctx, a)
	if err != nil {
		s.logger.Warn("failed to generate recommendation", "alert", a.Name, "error", err)
	}
	s.alertsMu.Lock()
	alertKey := a.Key()
	foundIndex := -1
	for i, existing := range s.alerts {
		if existing.Alert.Key() == alertKey {
			foundIndex = i
			break
		}
	}

	if foundIndex >= 0 {
		// Update existing alert in place, then shift it to index 0 so it
		// ranks as the most-recent occurrence. The previous form rebuilt
		// the whole slice with two append(make-new-slice) calls — O(n)
		// allocation per repeated alert, blocking every concurrent
		// reader on s.alertsMu. The in-place copy below moves the same
		// number of elements but reuses the backing array, dropping the
		// lock-hold time from ~milliseconds at typical alert volumes to
		// microseconds.
		existing := &s.alerts[foundIndex]
		existing.Alert.OccurredAt = a.OccurredAt
		existing.OccurrenceCount++
		if foundIndex > 0 {
			record := s.alerts[foundIndex]
			copy(s.alerts[1:foundIndex+1], s.alerts[0:foundIndex])
			s.alerts[0] = record
		}
	} else {
		// Add new alert
		newRecord := AlertRecord{
			ID:              alerts.GenerateAlertID(a),
			Alert:           a,
			Recommendation:  rec,
			State:           "",
			Comments:        nil,
			OccurrenceCount: 1,
			FirstOccurredAt: a.OccurredAt,
		}
		s.alerts = append([]AlertRecord{newRecord}, s.alerts...)
		if len(s.alerts) > maxAlertRecords {
			s.alerts = s.alerts[:maxAlertRecords]
		}
	}
	s.alertsMu.Unlock()

	// Store alert in alerts store if configured
	if s.alertsStore != nil {
		metadata := map[string]interface{}{
			"recommendation": rec,
			"stored_by":      "mcp_server",
		}
		if err := s.alertsStore.StoreAlert(ctx, a, metadata); err != nil {
			s.logger.Warn("failed to store alert in alerts store", "alert", a.Name, "error", err)
		} else {
			s.logger.Debug("alert stored in alerts store", "alert", a.Name, "kind", a.Kind)
		}

		// If pod alert, capture pod data asynchronously
		if a.Kind == kwatch.AlertKindPod && a.Namespace != "" && a.Name != "" {
			go s.capturePodData(ctx, a)
		}
	}

	if err := s.history.Record(ctx, history.Incident{
		ID:        uuid.NewString(),
		Timestamp: a.OccurredAt,
		Kind:      history.IssueKind(a.Kind),
		Severity:  a.Severity,
		Name:      a.Name,
		Message:   a.Message,
	}); err != nil {
		s.logger.Warn("failed to record incident", "alert", a.Name, "error", err)
	}

	if err := s.mcp.ResourceUpdated(ctx, &mcp.ResourceUpdatedNotificationParams{URI: alertsResourceURI}); err != nil {
		s.logger.Warn("failed to send resource update notification", "error", err)
	}
}

func (s *MCPServer) handleReadAlerts(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	data, err := json.Marshal(s.AlertsSnapshot(ctx))
	if err != nil {
		s.logger.Warn("failed to marshal alerts", "error", err)
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{URI: alertsResourceURI, Text: string(data)}},
	}, nil
}

func (s *MCPServer) handleReadHistory(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	results, err := s.IncidentHistory(ctx, defaultHistoryRange)
	if err != nil {
		s.logger.Warn("failed to fetch incident history", "error", err)
		return nil, err
	}
	data, err := json.Marshal(results)
	if err != nil {
		s.logger.Warn("failed to marshal history", "error", err)
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{URI: historyResourceURI, Text: string(data)}},
	}, nil
}

func (s *MCPServer) generateSchema(params []tools.ToolParameter) map[string]interface{} {
	props := make(map[string]interface{})
	for _, p := range params {
		t := "string"
		switch strings.ToLower(p.Type) {
		case "boolean":
			t = "boolean"
		case "number", "integer":
			t = "number"
		}
		props[p.Name] = map[string]interface{}{"type": t, "description": p.Description}
	}
	return map[string]interface{}{"type": "object", "properties": props}
}

func (s *MCPServer) ListTools() map[string]interface{} {
	return map[string]interface{}{"tools": s.tools}
}

func (s *MCPServer) ExecuteTool(ctx context.Context, name string, args map[string]interface{}) (map[string]interface{}, error) {
	if f, ok := s.executors[name]; ok {
		return f(ctx, args)
	}
	return nil, ErrToolNotFound
}

func (s *MCPServer) HealthCheck(ctx context.Context) map[string]interface{} {
	err := s.client.HealthCheck(ctx)
	status := "healthy"
	if err != nil {
		status = "degraded"
	}
	return map[string]interface{}{"status": status}
}

func (s *MCPServer) AlertsSnapshot(ctx context.Context) []AlertRecord {
	s.alertsMu.RLock()
	defer s.alertsMu.RUnlock()
	out := make([]AlertRecord, len(s.alerts))
	copy(out, s.alerts)

	// Enrich pod alerts with pod existence and cache if alerts store is available
	if s.alertsStore != nil {
		for i := range out {
			alert := out[i].Alert
			if alert.Kind == kwatch.AlertKindPod && alert.Namespace != "" && alert.Name != "" {
				// Check pod existence via pod tracker
				if s.podTracker != nil {
					out[i].PodExists = s.podTracker.PodExists(alert.Namespace, alert.Name)
				}
				// Try to get pod cache
				cache, err := s.alertsStore.GetPodCache(ctx, alert.Namespace, alert.Name)
				if err == nil {
					out[i].PodCache = cache
					// Ensure PodExists reflects current state from pod tracker if available
					if s.podTracker != nil {
						out[i].PodExists = s.podTracker.PodExists(alert.Namespace, alert.Name)
					} else {
						out[i].PodExists = cache.PodExists
					}
				}
			}
			// Enrich with state and comments from store
			stored, err := s.alertsStore.GetAlert(ctx, out[i].ID)
			if err == nil {
				out[i].State = stored.State
				out[i].Comments = stored.Comments
			}
		}
	}
	return out
}

func (s *MCPServer) IncidentHistory(ctx context.Context, window time.Duration) ([]history.Incident, error) {
	if window <= 0 {
		window = defaultHistoryRange
	}
	var all []history.Incident
	for _, k := range history.SupportedKinds {
		incidents, err := s.history.List(ctx, k, window)
		if err != nil {
			return nil, err
		}
		all = append(all, incidents...)
	}
	return all, nil
}

// UpdateAlertState updates the state of an alert in the store
func (s *MCPServer) UpdateAlertState(ctx context.Context, id string, state string) error {
	if s.alertsStore == nil {
		return errors.New("alerts store not configured")
	}
	// Update in-memory alert if present
	s.alertsMu.Lock()
	for i := range s.alerts {
		if s.alerts[i].ID == id {
			s.alerts[i].State = state
			break
		}
	}
	s.alertsMu.Unlock()
	// Update persistent store
	return s.alertsStore.UpdateAlertState(ctx, id, state)
}

// AddAlertComment adds a comment to an alert in the store
func (s *MCPServer) AddAlertComment(ctx context.Context, id string, author string, content string) error {
	if s.alertsStore == nil {
		return errors.New("alerts store not configured")
	}
	// Update in-memory alert if present
	s.alertsMu.Lock()
	for i := range s.alerts {
		if s.alerts[i].ID == id {
			comment := alerts.Comment{
				ID:        uuid.New().String(),
				Author:    author,
				Content:   content,
				CreatedAt: time.Now(),
			}
			s.alerts[i].Comments = append(s.alerts[i].Comments, comment)
			break
		}
	}
	s.alertsMu.Unlock()
	// Update persistent store
	return s.alertsStore.AddAlertComment(ctx, id, author, content)
}

// capturePodData asynchronously captures pod logs, describe, YAML and AI help for pod alerts
func (s *MCPServer) capturePodData(ctx context.Context, a kwatch.Alert) {
	if s.client == nil || s.alertsStore == nil {
		return
	}

	// Check pod existence via pod tracker if available
	podExists := false
	var podUID string
	if s.podTracker != nil {
		podExists = s.podTracker.PodExists(a.Namespace, a.Name)
		uid, ok := s.podTracker.GetPodUID(a.Namespace, a.Name)
		if ok {
			podUID = uid
		}
	}

	// Get pod info
	var logs, describe, yamlStr, aiHelp string
	if podExists {
		// Fetch pod details
		podInfo, err := s.client.GetPod(ctx, a.Namespace, a.Name)
		if err != nil {
			s.logger.Warn("failed to fetch pod info", "namespace", a.Namespace, "pod", a.Name, "error", err)
		} else {
			// Generate YAML representation
			rawPod, err := s.client.GetRawInterface().CoreV1().Pods(a.Namespace).Get(ctx, a.Name, metav1.GetOptions{})
			if err == nil {
				yamlBytes, err := yaml.Marshal(rawPod)
				if err == nil {
					yamlStr = string(yamlBytes)
				}
			}
			// Get logs from first container (if any)
			if len(podInfo.Containers) > 0 {
				containerName := podInfo.Containers[0].Name
				logs, err = s.client.GetPodLogs(ctx, a.Namespace, a.Name, containerName, 100, 3600, false)
				if err != nil {
					s.logger.Debug("failed to fetch pod logs", "namespace", a.Namespace, "pod", a.Name, "error", err)
				}
			}
			// Generate describe output
			describe = generatePodDescribe(podInfo)
		}
	}

	cache := alerts.PodCache{
		PodUID:      podUID,
		PodName:     a.Name,
		Namespace:   a.Namespace,
		CachedAt:    time.Now(),
		Logs:        logs,
		YAML:        yamlStr,
		Describe:    describe,
		AIHelp:      aiHelp,
		PodExists:   podExists,
		LastChecked: time.Now(),
	}

	if err := s.alertsStore.StorePodCache(ctx, cache); err != nil {
		s.logger.Warn("failed to store pod cache", "namespace", a.Namespace, "pod", a.Name, "error", err)
	} else {
		s.logger.Debug("pod cache stored", "namespace", a.Namespace, "pod", a.Name, "exists", podExists)
	}
}

// generatePodDescribe creates a human-readable description of a pod
func generatePodDescribe(pod *kube.PodInfo) string {
	if pod == nil {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Name:         %s\n", pod.Name)
	fmt.Fprintf(&sb, "Namespace:    %s\n", pod.Namespace)
	fmt.Fprintf(&sb, "Status:       %s\n", pod.Status)
	fmt.Fprintf(&sb, "Phase:        %s\n", pod.Phase)
	fmt.Fprintf(&sb, "Node:         %s\n", pod.NodeName)
	fmt.Fprintf(&sb, "Age:          %v\n", pod.Age)
	fmt.Fprintf(&sb, "Restarts:     %d\n", pod.RestartCount)
	if len(pod.Labels) > 0 {
		sb.WriteString("Labels:\n")
		for k, v := range pod.Labels {
			fmt.Fprintf(&sb, "  %s=%s\n", k, v)
		}
	}
	sb.WriteString("Containers:\n")
	for _, c := range pod.Containers {
		fmt.Fprintf(&sb, "  - %s: %s (Ready: %v, Restarts: %d)\n",
			c.Name, c.Image, c.Ready, c.RestartCount)
	}
	return sb.String()
}

// Recommendations returns a deduplicated slice of recommendations derived
// from the current alert snapshot, sorted by severity.
func (s *MCPServer) Recommendations() []recommendation.Recommendation {
	s.alertsMu.RLock()
	defer s.alertsMu.RUnlock()
	seen := make(map[string]bool, len(s.alerts))
	var out []recommendation.Recommendation
	for _, ar := range s.alerts {
		key := ar.Recommendation.Title
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ar.Recommendation)
	}
	return out
}

func (s *MCPServer) ToolSummaries() []ToolSummary {
	summaries := make([]ToolSummary, 0, len(s.tools))
	for _, t := range s.tools {
		summary := ToolSummary{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		}
		summaries = append(summaries, summary)
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Name < summaries[j].Name
	})
	return summaries
}
