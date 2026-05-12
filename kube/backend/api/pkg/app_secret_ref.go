package pkg

import (
	"context"

	"github.com/argues/argus/internal/secretref"
)

// app_secret_ref.go — Wails methods that bridge the frontend
// SecretRefInput component to the Go-side resolver.
//
// The frontend stores raw "kind:value[#key]" strings in config and only
// needs to know two things at the UI layer:
//   1. What kind of source does this label parse as? (for the pill +
//      validation hint)
//   2. Does the resolver have a Source registered for that kind? (so
//      we can disable the "Save" button if not)
//
// The actual resolution (reading a file, calling AWS, etc.) happens
// server-side and never returns the raw value to the frontend — the
// resolved bytes are used to populate outbound requests (e.g. building
// an Authorization header). Exposing the unresolved label to the UI is
// fine because the label is not the secret.

// SecretRefInfo is the JSON shape returned by DescribeSecretRef. The
// frontend uses it to render the source pill + show / hide the #key
// subfield without re-parsing client-side.
type SecretRefInfo struct {
	Kind        string `json:"kind"`        // "inline" | "env" | "file" | "volume" | "aws-secret" | ...
	Value       string `json:"value"`       // body after the prefix
	Key         string `json:"key"`         // optional #key suffix
	Resolvable  bool   `json:"resolvable"`  // false for inline
	Supported   bool   `json:"supported"`   // resolver has a Source for this kind
	Description string `json:"description"` // human-readable summary
}

// DescribeSecretRef parses a "kind:value[#key]" reference and returns
// metadata about the source. Used by the UI to decide whether the
// "Save" button is enabled and what badge to display.
//
// This method does NOT resolve the value — it never touches the
// filesystem, network, or vault. Safe to call as often as the UI wants.
func (a *App) DescribeSecretRef(raw string) SecretRefInfo {
	ref := secretref.Parse(raw)
	info := SecretRefInfo{
		Kind:       string(ref.Kind),
		Value:      ref.Value,
		Key:        ref.Key,
		Resolvable: ref.IsResolvable(),
	}
	if a.secretRefResolver != nil {
		info.Supported = a.secretRefResolver.Has(ref.Kind)
	} else {
		// No resolver wired up — only inline is supported.
		info.Supported = ref.Kind == secretref.KindInline
	}
	info.Description = describeSecretRefHuman(ref)
	return info
}

// ResolveSecretRef resolves a reference and returns the value as a
// string. This is intentionally NOT in httpExposedMethods — it must
// only be reachable from the in-process Wails binding, never the
// HTTP API surface. Exposing it via HTTP would let any browser script
// dereference whatever the operator's filesystem / vault holds.
//
// Returns an error string (empty on success) alongside the value so
// the frontend can render the failure inline without unwrapping a
// thrown exception from the Wails bridge.
func (a *App) ResolveSecretRef(raw string) (string, string) {
	if a.secretRefResolver == nil {
		return "", "secret reference resolver is not configured"
	}
	val, err := a.secretRefResolver.ResolveString(context.Background(), raw)
	if err != nil {
		return "", err.Error()
	}
	return val, ""
}

// describeSecretRefHuman builds the same tooltip string the frontend
// would, but on the server so the two stay in sync via the test suite
// rather than divergent string-formatting in two languages.
func describeSecretRefHuman(r secretref.Ref) string {
	if r.Kind == "" || r.Kind == secretref.KindInline {
		if r.Value == "" {
			return "Empty inline value"
		}
		return "Inline value"
	}
	label := secretRefKindLabel(r.Kind)
	if r.Key != "" {
		return label + " · " + r.Value + " (" + r.Key + ")"
	}
	if r.Value == "" {
		return label
	}
	return label + " · " + r.Value
}

func secretRefKindLabel(k secretref.Kind) string {
	switch k {
	case secretref.KindEnv:
		return "Env var"
	case secretref.KindFile:
		return "File"
	case secretref.KindVolume:
		return "Volume"
	case secretref.KindAWSSecret:
		return "AWS Secrets Mgr"
	case secretref.KindGCPSecret:
		return "GCP Secret Mgr"
	case secretref.KindAzureVault:
		return "Azure Key Vault"
	case secretref.KindArgusVault:
		return "Argus Vault"
	default:
		return string(k)
	}
}
