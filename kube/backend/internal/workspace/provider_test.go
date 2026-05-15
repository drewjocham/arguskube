package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// TestProvider is a dev/test-only Provider that satisfies the OAuth
// flow without contacting any external service. Phase 1A ships it so
// the end-to-end Connect path can be exercised before the real Google
// providers land.
//
// Lives in a *_test.go file so production builds don't pick it up.
type TestProvider struct {
	svc Service
}

func NewTestProvider(svc Service) *TestProvider {
	return &TestProvider{svc: svc}
}

func (p *TestProvider) Service() Service { return p.svc }

func (p *TestProvider) Start(_ context.Context, _, redirectURL string) (AuthURL, error) {
	state := randHex(16)
	return AuthURL{
		URL:   redirectURL + "?test=1&state=" + state,
		State: state,
	}, nil
}

func (p *TestProvider) Complete(_ context.Context, state, code string) (CompleteResult, error) {
	if code == "fail" {
		return CompleteResult{}, errors.New("test provider: code=fail")
	}
	return CompleteResult{
		ExternalWorkspaceID: "test-workspace-" + state[:6],
		DisplayName:         "Test Workspace",
		Email:               "test@example.com",
		Token: Token{
			AccessToken:  "test-access-" + code,
			RefreshToken: "test-refresh-" + code,
			TokenType:    "bearer",
			ExpiresAt:    time.Now().Add(1 * time.Hour),
			Scope:        "test:read test:write",
		},
	}, nil
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
