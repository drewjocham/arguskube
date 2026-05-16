package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	gochi "github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/argues/argus/agent/internal/k8s"
)

// Agent HTTP routes. Constants per the pim-agl style so the desktop
// app and the agent share a single source of truth.
const (
	agentPathHealth         = "/health"
	agentPathPods           = "/api/v1/pods"
	agentPathNodes          = "/api/v1/nodes"
	agentPathAnomalies      = "/api/v1/anomalies"
	agentPathEvents         = "/api/v1/events"
	agentPathDeployments    = "/api/v1/deployments"
	agentPathServices       = "/api/v1/services"
	agentPathTopology       = "/api/v1/topology"
	agentPathStreamMetrics  = "/stream/metrics"
)

type AnomalyScore struct {
	Timestamp time.Time `json:"timestamp"`
	Score     float64   `json:"score"`
	Target    string    `json:"target"`
	Rule      string    `json:"rule"`
}

type TopologyNode struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
}

type TopologyEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type TopologyGraph struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

type MetricsFrame struct {
	Timestamp string        `json:"timestamp"`
	Nodes     []NodeMetrics `json:"nodes"`
	Pods      []PodMetrics  `json:"pods"`
}

type NodeMetrics struct {
	Name     string `json:"name"`
	CPUUsage string `json:"cpu_usage"`
	MemUsage string `json:"mem_usage"`
}

type PodMetrics struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	CPUUsage  string `json:"cpu_usage"`
	MemUsage  string `json:"mem_usage"`
}

type Server struct {
	httpServer *http.Server
	k8sClient  *k8s.Client
	logger     *slog.Logger
}

func New(port string, k8sClient *k8s.Client, logger *slog.Logger) *Server {
	s := &Server{
		k8sClient: k8sClient,
		logger:    logger,
	}
	s.httpServer = &http.Server{
		Addr:        ":" + port,
		Handler:     s.Routes(),
		ReadTimeout: 30 * time.Second,
		// No WriteTimeout — streaming endpoints need long-lived connections.
	}
	return s
}

// Routes builds the agent's HTTP router and returns it as a
// http.Handler — matching the pim-agl Routes() http.Handler pattern.
func (s *Server) Routes() http.Handler {
	r := gochi.NewRouter()

	r.Use(middleware.CleanPath)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get(agentPathHealth, s.handleHealth)
	r.Get(agentPathPods, s.handleGetPods)
	r.Get(agentPathNodes, s.handleGetNodes)
	r.Get(agentPathAnomalies, s.handleGetAnomalies)
	r.Get(agentPathEvents, s.handleGetEvents)
	r.Get(agentPathDeployments, s.handleGetDeployments)
	r.Get(agentPathServices, s.handleGetServices)
	r.Get(agentPathTopology, s.handleGetTopology)
	r.Get(agentPathStreamMetrics, s.handleStreamMetrics)

	return r
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func (s *Server) handleGetPods(w http.ResponseWriter, r *http.Request) {
	pods := s.k8sClient.GetPods()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(pods)
}

func (s *Server) handleGetNodes(w http.ResponseWriter, r *http.Request) {
	nodes := s.k8sClient.GetNodes()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(nodes)
}

func (s *Server) handleGetAnomalies(w http.ResponseWriter, r *http.Request) {
	anomalies := []AnomalyScore{
		{Timestamp: time.Now().Add(-2 * time.Minute), Score: 94.5, Target: "aws-prod-db", Rule: "Sudden Memory Spike"},
		{Timestamp: time.Now().Add(-1 * time.Hour), Score: 88.2, Target: "ingress/traefik", Rule: "High Error Rate"},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(anomalies)
}

func (s *Server) handleGetEvents(w http.ResponseWriter, r *http.Request) {
	events := s.k8sClient.GetEvents()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(events)
}

func (s *Server) handleGetDeployments(w http.ResponseWriter, r *http.Request) {
	deployments := s.k8sClient.GetDeployments()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(deployments)
}

func (s *Server) handleGetServices(w http.ResponseWriter, r *http.Request) {
	services := s.k8sClient.GetServices()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(services)
}

func (s *Server) handleGetTopology(w http.ResponseWriter, r *http.Request) {
	pods := s.k8sClient.GetPods()
	nodes := s.k8sClient.GetNodes()
	services := s.k8sClient.GetServices()

	graph := TopologyGraph{
		Nodes: []TopologyNode{},
		Edges: []TopologyEdge{},
	}

	// Add cluster nodes
	for _, node := range nodes {
		nodeStatus := "NotReady"
		for _, cond := range node.Status.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				nodeStatus = "Ready"
				break
			}
		}
		graph.Nodes = append(graph.Nodes, TopologyNode{
			ID:        node.Name,
			Kind:      "Node",
			Name:      node.Name,
			Namespace: "",
			Status:    nodeStatus,
		})
	}

	// Add pod nodes and edges from services
	for _, pod := range pods {
		podStatus := string(pod.Status.Phase)
		if podStatus == "" {
			podStatus = "Pending"
		}
		podID := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
		graph.Nodes = append(graph.Nodes, TopologyNode{
			ID:        podID,
			Kind:      "Pod",
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    podStatus,
		})

		// Add edge from node to pod
		if pod.Spec.NodeName != "" {
			graph.Edges = append(graph.Edges, TopologyEdge{
				Source: pod.Spec.NodeName,
				Target: podID,
			})
		}
	}

	// Add service nodes and connect to matching pods
	for _, svc := range services {
		svcID := fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)
		graph.Nodes = append(graph.Nodes, TopologyNode{
			ID:        svcID,
			Kind:      "Service",
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Status:    "Active",
		})

		// Find pods matching service selector
		selector := labels.SelectorFromSet(svc.Spec.Selector)
		for _, pod := range pods {
			if pod.Namespace != svc.Namespace {
				continue
			}
			if selector.Matches(labels.Set(pod.Labels)) {
				podID := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
				graph.Edges = append(graph.Edges, TopologyEdge{
					Source: svcID,
					Target: podID,
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(graph)
}

func (s *Server) handleStreamMetrics(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// SSE-style streaming: newline-delimited JSON frames every 5s.
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	// Send an initial frame immediately.
	frame := s.buildMetricsFrame()
	if err := json.NewEncoder(w).Encode(frame); err != nil {
		return
	}
	flusher.Flush()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			frame := s.buildMetricsFrame()
			if err := json.NewEncoder(w).Encode(frame); err != nil {
				s.logger.Error("failed to write metrics frame", "error", err)
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) buildMetricsFrame() MetricsFrame {
	nodes := s.k8sClient.GetNodes()
	pods := s.k8sClient.GetPods()

	nodeMetrics := []NodeMetrics{}
	for _, node := range nodes {
		nodeMetrics = append(nodeMetrics, NodeMetrics{
			Name:     node.Name,
			CPUUsage: "0m",
			MemUsage: "0Mi",
		})
	}

	podMetrics := []PodMetrics{}
	for _, pod := range pods {
		podMetrics = append(podMetrics, PodMetrics{
			Namespace: pod.Namespace,
			Name:      pod.Name,
			CPUUsage:  "0m",
			MemUsage:  "0Mi",
		})
	}

	return MetricsFrame{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Nodes:     nodeMetrics,
		Pods:      podMetrics,
	}
}

func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		s.logger.Info("starting http server", "addr", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("shutting down http server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
