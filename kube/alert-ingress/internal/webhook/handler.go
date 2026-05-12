package webhook

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/argues/argus/alert-ingress/internal/models"
	"github.com/argues/argus/alert-ingress/internal/pubsub"
)

type Handler struct {
	publisher pubsub.Publisher
}

func New(publisher pubsub.Publisher) *Handler {
	return &Handler{publisher: publisher}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var webhook models.AnomstackWebhook
	if err := json.NewDecoder(r.Body).Decode(&webhook); err != nil {
		log.Printf("bad webhook payload: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	severity := models.SeverityWarning
	var metricValue float64
	if details, ok := webhook.AnomalyDetails["metric_value"]; ok {
		if v, ok := details.(float64); ok {
			metricValue = v
		}
	}
	score, _ := webhook.AnomalyDetails["metric_score_smooth"].(float64)
	if score == 0 {
		score = webhook.Threshold
	}

	if score > 0.9 {
		severity = models.SeverityCritical
	}

	desc := webhook.Message
	if desc == "" {
		desc = webhook.Title
	}

	alert := models.ArgusAlert{
		ID:          uuid.New().String(),
		Source:      "anomstack",
		Severity:    severity,
		Title:       webhook.Title,
		Description: desc,
		MetricName:  webhook.MetricName,
		MetricValue: metricValue,
		Score:       score,
		Threshold:   webhook.Threshold,
		Timestamp:   time.Now().UTC(),
	}

	if err := h.publisher.PublishAlert(r.Context(), alert); err != nil {
		log.Printf("publish failed: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "alert_id": alert.ID})
}
