// Package cloud surfaces AWS and GCP credentials + Secrets Manager
// data inside the Argus console. Phase 1 (this file set) covers
// identity status + secret list/reveal using *ambient* credentials —
// whatever's already on disk via the cloud CLIs or env vars. Login
// flows and service-account impersonation land in follow-up changes.
//
// All operations are READ-ONLY against the cloud APIs.
package cloud

import (
	"context"
	"time"
)

// Provider names — kept as constants so the wire payload, frontend
// switches, and Go switches all use the same strings.
const (
	ProviderAWS = "aws"
	ProviderGCP = "gcp"
)

// Identity describes who the console is currently authenticated as
// for a given cloud provider. The shape is intentionally cross-cloud
// so the frontend can render a uniform identity card.
type Identity struct {
	// Provider is "aws" or "gcp".
	Provider string `json:"provider"`

	// Authenticated is true when the provider has usable credentials.
	// When false, all other fields except Provider and Error are zero.
	Authenticated bool `json:"authenticated"`

	// Subject is the human-friendly identity string:
	//   AWS:  STS caller ARN, e.g. "arn:aws:iam::123456789012:user/jane"
	//   GCP:  Active account email, e.g. "jane@example.com"
	Subject string `json:"subject,omitempty"`

	// Account is the tenant identifier:
	//   AWS: 12-digit account ID
	//   GCP: project ID (best-effort — depends on ADC source)
	Account string `json:"account,omitempty"`

	// Source labels where the credentials came from. Helps users
	// understand "why am I logged in as this" — env var vs config
	// file vs metadata server.
	Source string `json:"source,omitempty"`

	// ExpiresAt is non-zero when the underlying credentials have a
	// known expiry (STS, OIDC, OAuth2). Static IAM keys have no expiry
	// so the frontend should render "—" when zero.
	ExpiresAt time.Time `json:"expiresAt,omitempty"`

	// Expired is a convenience derived from ExpiresAt vs. wall clock.
	Expired bool `json:"expired,omitempty"`

	// Error captures the reason Authenticated == false (e.g. "no
	// credentials found in default chain"). Surfaced verbatim in the
	// identity card's hint line.
	Error string `json:"error,omitempty"`
}

// SecretItem is the row-level shape for the secrets list. All clouds
// project into this — fields that don't apply stay empty.
type SecretItem struct {
	// Name is the resource path used to reveal:
	//   AWS: full ARN or short name (we always pass the full ARN
	//        between frontend and backend so reveal is unambiguous)
	//   GCP: "projects/<num>/secrets/<id>/versions/<ver>"
	Name string `json:"name"`

	// DisplayName is what the row renders by default. The backend
	// shortens long ARNs so the table stays readable.
	DisplayName string `json:"displayName"`

	// Region (AWS) or Location (GCP). Empty for global secrets.
	Region string `json:"region,omitempty"`

	// Created is the secret's creation timestamp. Zero if the API
	// didn't return it.
	Created time.Time `json:"created,omitempty"`

	// Updated is the most recent modification time (last rotation /
	// metadata change for AWS, latest enabled version for GCP).
	Updated time.Time `json:"updated,omitempty"`

	// Description carries the cloud-side description if present —
	// helpful for picking a secret out of a long list.
	Description string `json:"description,omitempty"`

	// Tags / Labels — both clouds support free-form key/value
	// metadata. Surfaced as a tag pill list in the row.
	Labels map[string]string `json:"labels,omitempty"`
}

// SecretValue carries the resolved plaintext of a single secret. The
// frontend treats this exactly like a Kubernetes Secret data value
// (obfuscated until the user clicks Reveal). We deliberately do NOT
// expose binary-only secrets as base64 here — when IsBinary is true
// the value is base64-encoded just like a K8s secret data value so
// the existing decodeBase64 path works.
type SecretValue struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	IsBinary bool   `json:"isBinary,omitempty"`
}

// Provider is the cloud-agnostic surface every cloud backend exposes.
// Implementations live in aws.go and gcp.go.
type Provider interface {
	Identity(ctx context.Context) Identity
	ListSecrets(ctx context.Context, opts ListOpts) ([]SecretItem, error)
	RevealSecret(ctx context.Context, name string) (SecretValue, error)
}

// ListOpts narrows ListSecrets. Right now both clouds use Region or
// Project as the only meaningful filter; we may grow this later
// (tag/label match, name prefix) without changing the Provider
// signature.
type ListOpts struct {
	// Region is the AWS region. Empty falls back to the SDK default
	// (env / config file).
	Region string `json:"region,omitempty"`
	// Project is the GCP project ID. Empty falls back to ADC's
	// inferred project.
	Project string `json:"project,omitempty"`
}
