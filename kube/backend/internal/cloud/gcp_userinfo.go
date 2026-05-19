package cloud

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// fetchGCPEmail returns the authenticated user's email by calling the
// public v3 userinfo endpoint. Failure returns "" — the identity card
// will fall back to the credential JSON's client_email hint or just
// show the project. This keeps the dependency footprint tiny (no
// google.golang.org/api/oauth2/v2 just for one field).
func fetchGCPEmail(ctx context.Context, accessToken string) string {
	if accessToken == "" {
		return ""
	}
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet,
		"https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return ""
	}
	return body.Email
}
