package pkg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// app_external_secrets — first-class support for the secrets-encryption
// tooling teams typically pair with Kubernetes:
//
//   * Bitnami Sealed Secrets (kubeseal CLI)
//   * SOPS
//   * GPG / PGP for SOPS / age workflows
//   * External Secrets Operator (cluster CRDs from external-secrets.io)
//
// The frontend uses these to:
//   1. Auto-detect whether the local tooling binaries are installed (so a
//      reconfigure flow knows what's already on PATH and what isn't).
//   2. Surface possible volume-mount sources (Secret / SealedSecret /
//      ExternalSecret) per namespace so the user can see at a glance
//      which encrypted-secret pipelines are in play in their cluster.
//
// Nothing here mutates cluster state — both methods are read-only and
// safe over the SaaS HTTP API.

// --- Tool detection --------------------------------------------------------

// SecretsToolStatus reports whether a local CLI is installed, plus the
// detected version and absolute path. Empty Version with Found=false means
// the binary isn't on PATH (or couldn't be exec'd).
type SecretsToolStatus struct {
	Tool    string `json:"tool"`
	Found   bool   `json:"found"`
	Version string `json:"version,omitempty"`
	Path    string `json:"path,omitempty"`
	Error   string `json:"error,omitempty"`
}

// secretsToolSpecs lists the binaries we know how to probe. Adding a new
// tool only requires extending this table — every new entry inherits the
// same exec timeout, version-line extraction, and frontend rendering.
type secretsToolSpec struct {
	tool       string
	binary     string
	versionArg []string
	// versionFilter, when set, picks the line out of stdout/stderr that
	// carries the version string. Tools with single-line output use the
	// default (whole output trimmed).
	versionFilter func(string) string
}

var secretsToolSpecsByName = map[string]secretsToolSpec{
	"kubeseal": {
		tool: "kubeseal", binary: "kubeseal", versionArg: []string{"--version"},
	},
	"sops": {
		tool: "sops", binary: "sops", versionArg: []string{"--version"},
		// `sops --version` prints e.g. "sops 3.8.1 (latest) …" — we only
		// want the first line because some builds also append banner copy.
		versionFilter: firstLine,
	},
	"gpg": {
		tool: "gpg", binary: "gpg", versionArg: []string{"--version"},
		// `gpg --version` is multi-line; the first line carries the version.
		versionFilter: firstLine,
	},
	"age": {
		tool: "age", binary: "age", versionArg: []string{"--version"},
	},
}

// TestSecretsTool runs `<binary> --version` and parses the result.
// Returns a populated SecretsToolStatus even when the binary is missing —
// the frontend always wants something to render in the row.
func (a *App) TestSecretsTool(name string) (SecretsToolStatus, error) {
	spec, ok := secretsToolSpecsByName[name]
	if !ok {
		return SecretsToolStatus{
			Tool:  name,
			Error: fmt.Sprintf("unknown tool %q", name),
		}, nil
	}
	out := SecretsToolStatus{Tool: spec.tool}

	path, err := exec.LookPath(spec.binary)
	if err != nil {
		out.Error = "not found on PATH"
		return out, nil
	}
	out.Path = path
	out.Found = true

	ctx, cancel := context.WithTimeout(a.ctx, 4*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, spec.binary, spec.versionArg...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Even on non-zero exit, --version output is sometimes still
		// useful (e.g. older sops returns 1 on --version). Surface what
		// we got so the row reads "found, version unknown" instead of
		// silently missing.
		out.Error = err.Error()
	}

	combined := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
	if spec.versionFilter != nil {
		combined = spec.versionFilter(combined)
	}
	out.Version = combined
	return out, nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

// --- Cluster-side secret-source enumeration --------------------------------

// SecretSource is one possible volume-mount source in a namespace. Kind is
// the Kubernetes-level resource type ("Secret", "SealedSecret",
// "ExternalSecret", "ConfigMap"); Type is the inner type/discriminator
// (Opaque, kubernetes.io/tls, sealed-secrets.bitnami.com/v1alpha1 etc.).
type SecretSource struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"`
	Type      string `json:"type,omitempty"`
	// Encrypted indicates the source is *not* a plain Kubernetes Secret —
	// either it's a SealedSecret / ExternalSecret pulling from a real
	// secret backend, or it carries SOPS-style annotations marking it as
	// pre-encrypted at rest.
	Encrypted bool `json:"encrypted"`
	// Hint is a short human-readable note (the secret type, the ESO
	// secretStoreRef name, etc.) the UI can render alongside the row.
	Hint string `json:"hint,omitempty"`
}

// ListEncryptedSecretSources enumerates every Secret / SealedSecret /
// ExternalSecret in the namespace that could be referenced as a volume
// mount source. SealedSecrets and ExternalSecrets require their CRDs to
// be installed; they're polled via the dynamic client and silently
// skipped when absent.
func (a *App) ListEncryptedSecretSources(namespace string) ([]SecretSource, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	if strings.TrimSpace(namespace) == "" {
		return nil, fmt.Errorf("namespace is required")
	}

	out := []SecretSource{}

	// Native Secrets — always present, never errors out cleanly.
	cs := a.k8s.GetClientset()
	if cs != nil {
		ctx, cancel := context.WithTimeout(a.ctx, 6*time.Second)
		defer cancel()
		secs, err := cs.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
		if err == nil {
			for _, s := range secs.Items {
				src := SecretSource{
					Name:      s.Name,
					Namespace: s.Namespace,
					Kind:      "Secret",
					Type:      string(s.Type),
				}
				// SOPS / sealed-secret annotations on a regular Secret
				// indicate the source-of-truth is encrypted on disk
				// somewhere (helm-secrets, ArgoCD with sops plugin, etc.).
				switch {
				case s.Annotations["sops"] == "true",
					hasAnyAnnotation(s.Annotations, "sops/", "argocd-vault-plugin/"):
					src.Encrypted = true
					src.Hint = "SOPS-encrypted source"
				case s.Annotations["secrets.bitnami.com/managed"] == "true":
					src.Encrypted = true
					src.Hint = "managed by SealedSecret"
				}
				out = append(out, src)
			}
		}
	}

	// SealedSecrets + ExternalSecrets via the dynamic client. Either CRD
	// being absent is normal — most clusters won't have both.
	if rest := a.k8s.GetRestConfig(); rest != nil {
		if dyn, err := dynamic.NewForConfig(rest); err == nil {
			out = append(out, listDynamicSecretSources(a.ctx, dyn, namespace,
				schema.GroupVersionResource{
					Group:    "bitnami.com",
					Version:  "v1alpha1",
					Resource: "sealedsecrets",
				},
				"SealedSecret",
				func(item map[string]any) string {
					// SealedSecret.spec.template.metadata.name is the
					// resulting Secret name; surface that as the hint
					// so users can correlate with mount references.
					if spec, _ := item["spec"].(map[string]any); spec != nil {
						if tpl, _ := spec["template"].(map[string]any); tpl != nil {
							if md, _ := tpl["metadata"].(map[string]any); md != nil {
								if n, _ := md["name"].(string); n != "" {
									return "→ Secret/" + n
								}
							}
						}
					}
					return "decrypted by sealed-secrets controller"
				},
			)...)

			out = append(out, listDynamicSecretSources(a.ctx, dyn, namespace,
				schema.GroupVersionResource{
					Group:    "external-secrets.io",
					Version:  "v1beta1",
					Resource: "externalsecrets",
				},
				"ExternalSecret",
				func(item map[string]any) string {
					if spec, _ := item["spec"].(map[string]any); spec != nil {
						// secretStoreRef.{name,kind} → tells the user
						// which provider this pulls from (AWS, GCP, etc.).
						if ref, _ := spec["secretStoreRef"].(map[string]any); ref != nil {
							name, _ := ref["name"].(string)
							kind, _ := ref["kind"].(string)
							if kind == "" {
								kind = "SecretStore"
							}
							if name != "" {
								return kind + "/" + name
							}
						}
					}
					return "external-secrets-operator"
				},
			)...)
		}
	}

	return out, nil
}

// listDynamicSecretSources is a generic helper that lists items of a CRD
// kind via the dynamic client and converts each into a SecretSource. A
// missing CRD (NoMatchKind) returns an empty slice so the caller can keep
// composing without conditionals everywhere.
func listDynamicSecretSources(
	ctx context.Context,
	dyn dynamic.Interface,
	namespace string,
	gvr schema.GroupVersionResource,
	kindLabel string,
	hintFn func(map[string]any) string,
) []SecretSource {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	list, err := dyn.Resource(gvr).Namespace(namespace).List(cctx, metav1.ListOptions{})
	if err != nil {
		// Distinguish "CRD not installed" (NoKindMatchError / NotFound)
		// from a real failure. We swallow the former and surface nothing
		// — the row just doesn't show — but log the latter.
		if isCRDAbsent(err) {
			return nil
		}
		// Best effort: still return nothing, caller doesn't need to
		// error out the whole call because one optional CRD glitched.
		return nil
	}
	out := make([]SecretSource, 0, len(list.Items))
	for _, item := range list.Items {
		hint := ""
		if hintFn != nil {
			hint = hintFn(item.Object)
		}
		out = append(out, SecretSource{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			Kind:      kindLabel,
			Encrypted: true,
			Hint:      hint,
		})
	}
	return out
}

func isCRDAbsent(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "no matches for kind"),
		strings.Contains(msg, "the server could not find the requested resource"),
		strings.Contains(msg, "could not find the requested resource"),
		errors.Is(err, context.DeadlineExceeded):
		return true
	}
	return false
}

func hasAnyAnnotation(m map[string]string, prefixes ...string) bool {
	for k := range m {
		for _, p := range prefixes {
			if strings.HasPrefix(k, p) {
				return true
			}
		}
	}
	return false
}
