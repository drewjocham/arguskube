package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

// AnomalyScore represents an AI-generated anomaly metric.
type AnomalyScore struct {
	Timestamp time.Time `json:"timestamp"`
	Score     float64   `json:"score"`
	Target    string    `json:"target"`
	Rule      string    `json:"rule"`
}

func main() {
	log.Println("Starting KubeWatcher In-Cluster Agent...")

	token := os.Getenv("SAAS_TOKEN")
	if token == "" {
		log.Println("Warning: SAAS_TOKEN not set. Running in local-only mode.")
	} else {
		log.Println("SaaS Token detected. Will sync metadata to cloud.")
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	http.HandleFunc("/api/v1/anomalies", func(w http.ResponseWriter, r *http.Request) {
		// Mock data for the desktop client to consume via port-forward.
		anomalies := []AnomalyScore{
			{Timestamp: time.Now().Add(-2 * time.Minute), Score: 94.5, Target: "aws-prod-db", Rule: "Sudden Memory Spike"},
			{Timestamp: time.Now().Add(-1 * time.Hour), Score: 88.2, Target: "ingress/traefik", Rule: "High Error Rate"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(anomalies)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Agent listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
