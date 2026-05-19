package pkg

import (
	"fmt"

	"github.com/argues/argus/internal/cloud"
)

// Cloud secrets handlers (Phase 1):
//   - CloudIdentities      : who am I in AWS / GCP (uses ambient creds)
//   - CloudListSecrets     : list secrets in one provider
//   - CloudRevealSecret    : fetch a single secret's value
//
// Login flow and service-account impersonation land in follow-up PRs.
// All three handlers are safe to expose over the SaaS HTTP shim
// (added to httpExposedMethods in server.go): every call resolves
// credentials from the local process environment, so an HTTP caller
// can only ever surface the secrets the running Argus process can
// already see.

func (a *App) cloudProvider(name string) (cloud.Provider, error) {
	switch name {
	case cloud.ProviderAWS:
		return cloud.AWSProvider{}, nil
	case cloud.ProviderGCP:
		return cloud.GCPProvider{}, nil
	case cloud.ProviderVault:
		return cloud.VaultProvider{}, nil
	case cloud.ProviderAzure:
		return cloud.AzureProvider{}, nil
	default:
		return nil, fmt.Errorf("unknown cloud provider %q (want aws|gcp|vault|azure)", name)
	}
}

// CloudIdentities reports the current authentication state for every
// supported provider. Always returns one entry per provider; failure
// to authenticate is encoded in the Identity.Error field rather than
// raised as a Go error so the frontend can render all cards.
func (a *App) CloudIdentities() []cloud.Identity {
	providers := []cloud.Provider{
		cloud.AWSProvider{},
		cloud.GCPProvider{},
		cloud.VaultProvider{},
		cloud.AzureProvider{},
	}
	out := make([]cloud.Identity, 0, len(providers))
	for _, p := range providers {
		out = append(out, p.Identity(a.ctx))
	}
	return out
}

// CloudListSecrets lists secrets in the named provider. opts narrows
// the lookup (AWS region / GCP project). Returns an empty slice with
// no error when the provider has no secrets in scope.
func (a *App) CloudListSecrets(provider string, opts cloud.ListOpts) ([]cloud.SecretItem, error) {
	p, err := a.cloudProvider(provider)
	if err != nil {
		return nil, err
	}
	return p.ListSecrets(a.ctx, opts)
}

// CloudRevealSecret returns the resolved plaintext (or base64 for
// binary payloads) of a single secret. The frontend keeps the value
// obfuscated until the user clicks Reveal, then decodes locally —
// same UX as kube-secret reveal.
func (a *App) CloudRevealSecret(provider, name string) (cloud.SecretValue, error) {
	p, err := a.cloudProvider(provider)
	if err != nil {
		return cloud.SecretValue{}, err
	}
	if name == "" {
		return cloud.SecretValue{}, fmt.Errorf("secret name required")
	}
	return p.RevealSecret(a.ctx, name)
}
