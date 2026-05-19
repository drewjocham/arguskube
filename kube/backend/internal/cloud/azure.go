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

// azureSecretsPathMarker is the path segment Key Vault URIs use to
// delimit the secret name from the rest of the resource id, e.g.
// https://my-vault.vault.azure.net/secrets/<name>/<version>.
const azureSecretsPathMarker = "/secrets/"

func (AzureProvider) ListSecrets(ctx context.Context, opts ListOpts) ([]SecretItem, error) {
	vaultURL := normalizeAzureVaultURL(opts.AzureVaultURL)
	if vaultURL == "" {
		return nil, errors.New("Azure vault URL required (e.g. https://my-vault.vault.azure.net)")
	}
	cli, err := newAzureSecretsClient(vaultURL)
	if err != nil {
		return nil, err
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
			if item, ok := azureSecretItem(s, vaultURL); ok {
				items = append(items, item)
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].DisplayName < items[j].DisplayName })
	return items, nil
}

// newAzureSecretsClient wraps the credential + client construction so
// the public list/reveal calls don't each carry the same boilerplate.
func newAzureSecretsClient(vaultURL string) (*azsecrets.Client, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("azure credential: %w", err)
	}
	cli, err := azsecrets.NewClient(vaultURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("keyvault client: %w", err)
	}
	return cli, nil
}

// azureSecretItem projects a single SDK SecretItem into the cloud-
// agnostic SecretItem row. Returns ok=false for entries that have no
// id (no useful row content).
func azureSecretItem(s *azsecrets.SecretItem, vaultURL string) (SecretItem, bool) {
	if s == nil || s.ID == nil {
		return SecretItem{}, false
	}
	full := string(*s.ID)
	item := SecretItem{
		Name:        full,
		DisplayName: azureDisplayName(full),
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
	if labels := azureLabelsFromTags(s.Tags); len(labels) > 0 {
		item.Labels = labels
	}
	return item, true
}

func azureDisplayName(fullID string) string {
	i := strings.Index(fullID, azureSecretsPathMarker)
	if i < 0 {
		return fullID
	}
	rest := fullID[i+len(azureSecretsPathMarker):]
	if slash := strings.Index(rest, "/"); slash > 0 {
		return rest[:slash]
	}
	return rest
}

func azureLabelsFromTags(tags map[string]*string) map[string]string {
	if len(tags) == 0 {
		return nil
	}
	labels := make(map[string]string, len(tags))
	for k, v := range tags {
		if v != nil {
			labels[k] = *v
		}
	}
	return labels
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
	i := strings.Index(id, azureSecretsPathMarker)
	if i < 0 {
		return "", "", ""
	}
	vault = id[:i]
	rest := id[i+len(azureSecretsPathMarker):]
	if j := strings.Index(rest, "/"); j > 0 {
		return vault, rest[:j], rest[j+1:]
	}
	return vault, rest, ""
}

func has(env string) bool {
	v, ok := lookupEnv(env)
	return ok && v != ""
}
