# Argus image-trust policies

The Argus container images (`argus-agent`, `argus-alert-ingress`) are signed
in CI using **cosign keyless** against the GitHub Actions OIDC issuer. The
files in this directory are drop-in policies that allow those signed images
on a cluster that enforces signed-image admission via Kyverno, OPA
Gatekeeper, or the Sigstore policy-controller.

| File | Engine | Apply with |
| --- | --- | --- |
| `kyverno.yaml` | [Kyverno](https://kyverno.io) | `kubectl apply -f kyverno.yaml` |
| `gatekeeper.yaml` | [OPA Gatekeeper](https://open-policy-agent.github.io/gatekeeper/) | `kubectl apply -f gatekeeper.yaml` |
| `policy-controller.yaml` | [Sigstore policy-controller](https://docs.sigstore.dev/policy-controller/overview/) | `kubectl label namespace argus policy.sigstore.dev/include=true && kubectl apply -f policy-controller.yaml` |

The Argus desktop app's *Settings → Get Argus ready* checklist surfaces a
**Cluster requires signed images** row when any of these engines is
detected and applies the matching file with a `--dry-run=server` diff
preview before committing — so no policy change ever happens silently.

## Identity pinned by each policy

| Field | Value |
| --- | --- |
| OIDC issuer | `https://token.actions.githubusercontent.com` |
| Subject pattern | `https://github.com/<owner>/<repo>/.github/workflows/release.yml@refs/tags/v*` |

The `<owner>/<repo>` placeholder is rewritten by the desktop app on Apply
so each user gets a policy pinned to *their* fork if they self-host. If
you're applying these files by hand, replace `__REPO__` first.

## Verifying the signature manually

```bash
cosign verify \
  --certificate-identity-regexp 'https://github.com/__REPO__/\.github/workflows/release\.yml@refs/tags/v.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/__REPO__/argus-agent:vX.Y.Z
```
