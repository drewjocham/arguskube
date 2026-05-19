package workspace

import (
	"context"
	"fmt"
)

// CustomProvider is a manual/fallback connection type for providers not
// in the built-in catalogue. No OAuth — the user enters a display name,
// optional notes, and an optional API key or token. The connection is
// tracked in the UI but no automated API calls are made.
type CustomProvider struct{}

func NewCustomProvider() *CustomProvider { return &CustomProvider{} }

func (p *CustomProvider) Service() Service { return ServiceCustom }

func (p *CustomProvider) Start(_ context.Context, _, _ string) (AuthURL, error) {
	return AuthURL{}, fmt.Errorf("custom: manual connection — use the Connect button to enter details")
}

func (p *CustomProvider) Complete(_ context.Context, _, _ string) (CompleteResult, error) {
	return CompleteResult{}, fmt.Errorf("custom: manual connection — use ConnectCustom from the UI")
}
