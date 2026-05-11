package api

import "net/http"

func (a *API) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	// recommendations from current alert history via the recommendation engine
	// The engine lives on the server; expose it here.
	recs := a.server.Recommendations()
	a.respond(w, r, http.StatusOK, map[string]any{"recommendations": recs})
}
