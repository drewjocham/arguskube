package cloud

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azsecrets"
)

// AzureProvider implements Provider against Azure Key Vault. Phase 1
// uses DefaultAzureCredential, which walks the standard chain:
//   1. Env vars (AZURE_TENANT_ID / AZURE_CLIENT_ID / AZURE_CLIENT_SECRET)
//   2. Workload identity (federated tokens in AKS / GH Actions OIDC)
//   3. Managed identity (on Azure VMs, App Service, etc.)
//   4. Azure CLI cache (`az login`)
//   5. Azure Developer CLI (`azd auth login`)
// This matches what `az keyvault secret list` would discover, so an
// SRE already authenticated via `az login` sees their secrets here
// with zero further setup.
type AzureProvider struct{}

func (AzureProvider) Identity(ctx context.Context) Identity {
	id := Identity{Provider: ProviderAzure}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		id.Error = humanizeAzureCredErr(err)
		return id
	}

	// GetToken probes the chain with the Key Vault audience. Success
	// implies *some* credential in the chain answered; the returned
	// token carries an expiry we can surface verbatim.
	tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://vault.azure.net/.default"},
	})
	if err != nil {
		id.Error = humanizeAzureCredErr(err)
		return id
	}
	id.Source = inferAzureSource()
	id.ExpiresAt = tok.ExpiresOn
	id.Expired = time.Now().After(tok.ExpiresOn)
	// Subject = upn / oid claim parsed out of the JWT, best-effort.
	id.Subject = azureClaim(tok.Token, "upn", "preferred_username", "appid", "oid")
	id.Account = azureClaim(tok.Token, "tid") // tenant id
	id.Authenticated = !id.Expired
	return id
}

func (AzureProvider) ListSecrets(ctx context.Context, opts ListOpts) ([]SecretItem, error) {
	vaultURL := normalizeAzureVaultURL(opts.AzureVaultURL)
	if vaultURL == "" {
		return nil, errors.New("Azure vault URL required (e.g. https://my-vault.vault.azure.net)")
	}
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("azure credential: %w", err)
	}
	cli, err := azsecrets.NewClient(vaultURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("keyvault client: %w", err)
	}
	pager := cli.NewListSecretsPager(nil)
	items := []SecretItem{}
	const maxPages = 20 // guard against multi-thousand-secret vaults
	for page := 0; pager.More() && page < maxPages; page++ {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list secrets: %w", err)
		}
		for _, s := range resp.Value {
			if s == nil || s.ID == nil {
				continue
			}
			full := string(*s.ID)
			displayName := full
			// ID shape: https://<vault>.vault.azure.net/secrets/<name>/<version?>
			if i := strings.Index(full, "/secrets/"); i > 0 {
				rest := full[i+len("/secrets/"):]
				if slash := strings.Index(rest, "/"); slash > 0 {
					displayName = rest[:slash]
				} else {
					displayName = rest
				}
			}
			item := SecretItem{
				Name:        full,
				DisplayName: displayName,
				Region:      vaultURL,
			}
			if s.Attributes != nil {
				if s.Attributes.Created != nil {
					item.Created = *s.Attributes.Created
				}
				if s.Attributes.Updated != nil {
					item.Updated = *s.Attributes.Updated
				}
			}
			if s.Tags != nil {
				labels := make(map[string]string, len(s.Tags))
				for k, v := range s.Tags {
					if v != nil {
						labels[k] = *v
					}
				}
				if len(labels) > 0 {
					item.Labels = labels
				}
			}
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].DisplayName < items[j].DisplayName })
	return items, nil
}

func (AzureProvider) RevealSecret(ctx context.Context, name string) (SecretValue, error) {
	// name is the full Key Vault secret ID (URL). Split into vault +
	// secret name + version so we can use the typed SDK call.
	vaultURL, secretName, version := splitAzureSecretID(name)
	if vaultURL == "" || secretName == "" {
		return SecretValue{}, fmt.Errorf("invalid Azure secret id %q", name)
	}
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return SecretValue{}, fmt.Errorf("azure credential: %w", err)
	}
	cli, err := azsecrets.NewClient(vaultURL, cred, nil)
	if err != nil {
		return SecretValue{}, fmt.Errorf("keyvault client: %w", err)
	}
	out, err := cli.GetSecret(ctx, secretName, version, nil)
	if err != nil {
		return SecretValue{}, fmt.Errorf("get secret: %w", err)
	}
	if out.Value == nil {
		return SecretValue{Name: name}, nil
	}
	if utf8.ValidString(*out.Value) {
		return SecretValue{Name: name, Value: *out.Value}, nil
	}
	return SecretValue{
		Name:     name,
		Value:    base64.StdEncoding.EncodeToString([]byte(*out.Value)),
		IsBinary: true,
	}, nil
}

// ── helpers ──────────────────────────────────────────────────────

func humanizeAzureCredErr(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "DefaultAzureCredential: failed to acquire a token"):
		return "no Azure credentials available — run `az login` or set AZURE_TENANT_ID/CLIENT_ID/CLIENT_SECRET"
	case strings.Contains(msg, "AADSTS70043"), strings.Contains(msg, "expired"):
		return "Azure token expired — run `az login` to refresh"
	case strings.Contains(msg, "AADSTS50034"):
		return "Azure user does not exist in this tenant — wrong --tenant on `az login`?"
	default:
		return "azure: " + msg
	}
}

// inferAzureSource is a best-effort hint based on env presence —
// DefaultAzureCredential doesn't expose which child credential won.
func inferAzureSource() string {
	switch {
	case has("AZURE_CLIENT_SECRET"):
		return "service principal (env)"
	case has("AZURE_FEDERATED_TOKEN_FILE"):
		return "workload identity (federated)"
	case has("MSI_ENDPOINT"), has("IDENTITY_ENDPOINT"):
		return "managed identity"
	default:
		return "az CLI / azd CLI cache"
	}
}

// azureClaim pulls a single claim out of a JWT access token without
// pulling in a JWT library. We deliberately don't validate the
// signature — this is a hint for the identity card, not an
// authorization decision.
func azureClaim(token string, keys ...string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Some tokens use padded base64.
		payload, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	s := string(payload)
	for _, k := range keys {
		marker := `"` + k + `":"`
		i := strings.Index(s, marker)
		if i < 0 {
			continue
		}
		rest := s[i+len(marker):]
		if j := strings.Index(rest, `"`); j > 0 {
			return rest[:j]
		}
	}
	return ""
}

func normalizeAzureVaultURL(in string) string {
	in = strings.TrimSpace(in)
	if in == "" {
		return ""
	}
	if !strings.HasPrefix(in, "https://") {
		in = "https://" + in
	}
	return strings.TrimRight(in, "/")
}

// splitAzureSecretID splits an Azure Key Vault secret URL into the
// vault base URL, secret name, and version (version may be empty).
//   https://my-vault.vault.azure.net/secrets/db-password/abc123
//   -> "https://my-vault.vault.azure.net", "db-password", "abc123"
func splitAzureSecretID(id string) (vault, name, version string) {
	const marker = "/secrets/"
	i := strings.Index(id, marker)
	if i < 0 {
		return "", "", ""
	}
	vault = id[:i]
	rest := id[i+len(marker):]
	if j := strings.Index(rest, "/"); j > 0 {
		return vault, rest[:j], rest[j+1:]
	}
	return vault, rest, ""
}

func has(env string) bool {
	v, ok := lookupEnv(env)
	return ok && v != ""
}
