package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	vaultapi "github.com/hashicorp/vault/api"
)

// VaultProvider implements Provider against HashiCorp Vault. Phase 1
// supports the most common auth: token via VAULT_TOKEN env var or the
// ~/.vault-token file written by `vault login`. VAULT_ADDR points at
// the server.
//
// Listing is intentionally scoped to a single KV mount + prefix
// rather than walking the entire backend — Vault has no global
// "list everything" primitive, and a recursive sweep against a
// real-world install would melt the UI and probably the audit log.
// The frontend exposes mount + path as inputs.
type VaultProvider struct{}

func (VaultProvider) Identity(ctx context.Context) Identity {
	id := Identity{Provider: ProviderVault}

	addr := os.Getenv("VAULT_ADDR")
	if addr == "" {
		id.Error = "no VAULT_ADDR set — point Argus at a Vault server (e.g. https://vault.internal:8200)"
		return id
	}
	cli, err := vaultClient()
	if err != nil {
		id.Error = "vault client: " + err.Error()
		return id
	}
	if cli.Token() == "" {
		id.Error = "no Vault token — set VAULT_TOKEN or run `vault login`"
		return id
	}

	// token/lookup-self returns the calling token's metadata. Cheap
	// and works against every Vault version.
	sec, err := cli.Auth().Token().LookupSelfWithContext(ctx)
	if err != nil {
		id.Error = humanizeVaultErr(err)
		return id
	}
	if sec == nil || sec.Data == nil {
		id.Error = "vault returned empty token-lookup response"
		return id
	}
	id.Source = "VAULT_TOKEN / ~/.vault-token"
	id.Subject = strFromVaultData(sec.Data, "display_name", "id")
	id.Account = addr
	if ttlAny, ok := sec.Data["ttl"]; ok {
		if ttl := vaultTTLSeconds(ttlAny); ttl > 0 {
			id.ExpiresAt = time.Now().Add(time.Duration(ttl) * time.Second)
		}
	}
	id.Expired = !id.ExpiresAt.IsZero() && time.Now().After(id.ExpiresAt)
	id.Authenticated = !id.Expired
	return id
}

func (VaultProvider) ListSecrets(ctx context.Context, opts ListOpts) ([]SecretItem, error) {
	cli, err := vaultClient()
	if err != nil {
		return nil, err
	}
	mount := strings.TrimSpace(opts.VaultMount)
	if mount == "" {
		return nil, fmt.Errorf("vault mount required (e.g. \"secret\")")
	}
	prefix := strings.Trim(opts.VaultPath, "/")

	// KV v2 listing path: <mount>/metadata/<prefix>
	listPath := mount + "/metadata"
	if prefix != "" {
		listPath += "/" + prefix
	}
	sec, err := cli.Logical().ListWithContext(ctx, listPath)
	if err != nil {
		// Fall back to KV v1 layout if v2 metadata isn't there.
		v1, v1err := cli.Logical().ListWithContext(ctx, mount+"/"+prefix)
		if v1err != nil {
			return nil, fmt.Errorf("list %s: %w", listPath, err)
		}
		sec = v1
	}
	if sec == nil || sec.Data == nil {
		return []SecretItem{}, nil
	}
	keysAny, ok := sec.Data["keys"]
	if !ok {
		return []SecretItem{}, nil
	}
	keys, ok := keysAny.([]interface{})
	if !ok {
		return []SecretItem{}, nil
	}
	items := make([]SecretItem, 0, len(keys))
	for _, k := range keys {
		name, _ := k.(string)
		if name == "" {
			continue
		}
		full := name
		if prefix != "" {
			full = prefix + "/" + name
		}
		// Skip directory entries — frontend doesn't recurse yet; the
		// user types the deeper prefix into the path input.
		if strings.HasSuffix(name, "/") {
			continue
		}
		items = append(items, SecretItem{
			Name:        mount + "/" + full,
			DisplayName: full,
			Region:      mount,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].DisplayName < items[j].DisplayName })
	return items, nil
}

func (VaultProvider) RevealSecret(ctx context.Context, name string) (SecretValue, error) {
	cli, err := vaultClient()
	if err != nil {
		return SecretValue{}, err
	}
	// name shape: "<mount>/<path>" (see ListSecrets). We try KV v2
	// first (<mount>/data/<path>) then fall back to KV v1.
	mount, path, ok := strings.Cut(name, "/")
	if !ok || mount == "" || path == "" {
		return SecretValue{}, fmt.Errorf("invalid vault secret name %q (want \"mount/path\")", name)
	}
	v2Path := mount + "/data/" + path
	sec, err := cli.Logical().ReadWithContext(ctx, v2Path)
	if err != nil {
		// Try v1.
		v1, v1err := cli.Logical().ReadWithContext(ctx, mount+"/"+path)
		if v1err != nil {
			return SecretValue{}, fmt.Errorf("read %s: %w", v2Path, err)
		}
		return formatVaultSecret(name, v1.Data), nil
	}
	if sec == nil || sec.Data == nil {
		return SecretValue{Name: name}, nil
	}
	// KV v2 nests the actual values under .data.data
	data := sec.Data
	if nested, ok := sec.Data["data"].(map[string]interface{}); ok {
		data = nested
	}
	return formatVaultSecret(name, data), nil
}

// ── helpers ──────────────────────────────────────────────────────

// vaultClient builds a Vault API client from ambient env. The Vault
// SDK already honors VAULT_ADDR/VAULT_TOKEN/VAULT_NAMESPACE — we
// just defer to its default config.
func vaultClient() (*vaultapi.Client, error) {
	cfg := vaultapi.DefaultConfig()
	if cfg.Error != nil {
		return nil, cfg.Error
	}
	c, err := vaultapi.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	// SDK auto-loads VAULT_TOKEN; if that's empty fall back to the
	// ~/.vault-token file (the CLI writes this on `vault login`).
	if c.Token() == "" {
		if home, err := os.UserHomeDir(); err == nil {
			b, err := os.ReadFile(home + "/.vault-token")
			if err == nil {
				c.SetToken(strings.TrimSpace(string(b)))
			}
		}
	}
	return c, nil
}

func strFromVaultData(data map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := data[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// vaultTTLSeconds normalizes a TTL value from Vault — it can be a
// json.Number, a float64, or an int depending on backend version.
func vaultTTLSeconds(v interface{}) int64 {
	switch n := v.(type) {
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return i
		}
	case float64:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	}
	return 0
}

// formatVaultSecret turns a KV map into a SecretValue. When the map
// has a single key the value is returned directly; multi-key maps
// render as compact JSON so the user can see every field. Binary
// payloads use the same isBinary+base64 dance as AWS / GCP.
func formatVaultSecret(name string, data map[string]interface{}) SecretValue {
	if len(data) == 0 {
		return SecretValue{Name: name}
	}
	if len(data) == 1 {
		for _, v := range data {
			if s, ok := v.(string); ok {
				if utf8.ValidString(s) {
					return SecretValue{Name: name, Value: s}
				}
			}
		}
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return SecretValue{Name: name, Value: fmt.Sprintf("%v", data)}
	}
	return SecretValue{Name: name, Value: string(b)}
}

func humanizeVaultErr(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "permission denied"):
		return "Vault returned permission denied — token may lack `auth/token/lookup-self` capability"
	case strings.Contains(msg, "missing client token"):
		return "no Vault token set — run `vault login` or export VAULT_TOKEN"
	case strings.Contains(msg, "connection refused"):
		return "could not reach VAULT_ADDR — is the Vault server up?"
	case strings.Contains(msg, "tls: failed to verify certificate"):
		return "Vault TLS verification failed — set VAULT_CACERT or VAULT_SKIP_VERIFY (dev only)"
	default:
		return "vault: " + msg
	}
}
