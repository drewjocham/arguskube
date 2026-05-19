package cloud

import (
	"encoding/base64"
	"strings"
	"testing"
)

// Unit tests focus on the pure helpers — anything that touches AWS
// SDK or GCP SDK is integration-tested in app_cloud_secrets_test.go
// at the App layer where we mock the Provider interface.

func TestShortAWSName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, arn, want string
	}{
		{"prod/db/password", "arn:aws:secretsmanager:eu-west-1:111:secret:prod/db/password-AbCdEf", "prod/db/password"},
		{"", "arn:aws:secretsmanager:eu-west-1:111:secret:lonely-XYZ", "lonely-XYZ"},
	}
	for _, tc := range cases {
		t.Run(tc.arn, func(t *testing.T) {
			t.Parallel()
			if got := shortAWSName(tc.name, tc.arn); got != tc.want {
				t.Errorf("shortAWSName(%q, %q) = %q, want %q", tc.name, tc.arn, got, tc.want)
			}
		})
	}
}

func TestRegionFromAWSARN(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"arn:aws:secretsmanager:eu-west-1:111:secret:foo": "eu-west-1",
		"arn:aws:secretsmanager:us-east-2:222:secret:bar": "us-east-2",
		"short-name-no-arn": "",
		"":                  "",
	}
	for arn, want := range cases {
		t.Run(arn, func(t *testing.T) {
			t.Parallel()
			if got := regionFromAWSARN(arn); got != want {
				t.Errorf("regionFromAWSARN(%q) = %q, want %q", arn, got, want)
			}
		})
	}
}

func TestHumanizeAWSCredError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, mustContain string
	}{
		{"failed to refresh cached credentials, EOF", "aws sso login"},
		{"no EC2 IMDS role found", "AWS_PROFILE"},
		{"generic boom", "no AWS credentials"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got := humanizeAWSCredError(stubErr(tc.in))
			if !strings.Contains(got, tc.mustContain) {
				t.Errorf("humanizeAWSCredError(%q) = %q, want it to contain %q", tc.in, got, tc.mustContain)
			}
		})
	}
}

func TestHumanizeGCPCredError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, mustContain string
	}{
		{"could not find default credentials", "gcloud auth"},
		{"oauth2: cannot fetch token: invalid_grant", "gcloud auth"},
		{"random other thing", "no GCP credentials"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got := humanizeGCPCredError(stubErr(tc.in))
			if !strings.Contains(got, tc.mustContain) {
				t.Errorf("humanizeGCPCredError(%q) = %q, want it to contain %q", tc.in, got, tc.mustContain)
			}
		})
	}
}

func TestInferGCPSource(t *testing.T) {
	t.Parallel()
	cases := []struct {
		json, want string
	}{
		{`{"type":"authorized_user","client_id":"x"}`, "gcloud user (ADC)"},
		{`{"type":"service_account","client_email":"a@b"}`, "service account key file"},
		{`{"type":"external_account"}`, "external account (workload identity)"},
		{`{"type":"impersonated_service_account"}`, "impersonated service account"},
		{``, "GCE/GKE metadata server"},
		{`{}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.json, func(t *testing.T) {
			t.Parallel()
			if got := inferGCPSource([]byte(tc.json)); got != tc.want {
				t.Errorf("inferGCPSource(%q) = %q, want %q", tc.json, got, tc.want)
			}
		})
	}
}

func TestSubjectHintFromCredsJSON(t *testing.T) {
	t.Parallel()
	cases := []struct {
		json, want string
	}{
		{`{"client_email":"sa@proj.iam.gserviceaccount.com"}`, "sa@proj.iam.gserviceaccount.com"},
		{`{"account":"jane@example.com"}`, "jane@example.com"},
		{`{}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.json, func(t *testing.T) {
			t.Parallel()
			if got := subjectHintFromCredsJSON([]byte(tc.json)); got != tc.want {
				t.Errorf("subjectHintFromCredsJSON(%q) = %q, want %q", tc.json, got, tc.want)
			}
		})
	}
}

func TestShortGCPName(t *testing.T) {
	t.Parallel()
	if got := shortGCPName("projects/123/secrets/my-secret"); got != "my-secret" {
		t.Errorf("shortGCPName = %q", got)
	}
	if got := shortGCPName("no-slashes"); got != "no-slashes" {
		t.Errorf("shortGCPName = %q", got)
	}
}

// ── Vault helpers ─────────────────────────────────────────────────

func TestHumanizeVaultErr(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"permission denied on auth/token/lookup-self", "permission denied"},
		{"missing client token", "VAULT_TOKEN"},
		{"Get https://vault.local:8200/v1/auth/token/lookup-self: connection refused", "VAULT_ADDR"},
		{"tls: failed to verify certificate: x509: cert", "VAULT_CACERT"},
		{"some unrelated thing", "vault:"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got := humanizeVaultErr(stubErr(tc.in))
			if !strings.Contains(got, tc.want) {
				t.Errorf("humanizeVaultErr(%q) = %q, want it to contain %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFormatVaultSecret(t *testing.T) {
	t.Parallel()
	// Single-key map returns the bare string value.
	got := formatVaultSecret("secret/db", map[string]interface{}{"password": "hunter2"})
	if got.Value != "hunter2" {
		t.Errorf("single-key value = %q, want %q", got.Value, "hunter2")
	}
	// Multi-key map renders as indented JSON.
	got = formatVaultSecret("secret/api", map[string]interface{}{"user": "admin", "pw": "x"})
	if !strings.Contains(got.Value, `"user"`) || !strings.Contains(got.Value, `"pw"`) {
		t.Errorf("multi-key value = %q, want JSON with both keys", got.Value)
	}
	// Empty map gives empty SecretValue.
	got = formatVaultSecret("secret/empty", nil)
	if got.Value != "" {
		t.Errorf("empty map value = %q, want \"\"", got.Value)
	}
}

func TestVaultTTLSeconds(t *testing.T) {
	t.Parallel()
	if vaultTTLSeconds(float64(3600)) != 3600 {
		t.Error("float64")
	}
	if vaultTTLSeconds(int(7200)) != 7200 {
		t.Error("int")
	}
	if vaultTTLSeconds(int64(86400)) != 86400 {
		t.Error("int64")
	}
	if vaultTTLSeconds("not a number") != 0 {
		t.Error("string fallback")
	}
}

// ── Azure helpers ─────────────────────────────────────────────────

func TestNormalizeAzureVaultURL(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":                                  "",
		"  my-vault.vault.azure.net  ":      "https://my-vault.vault.azure.net",
		"my-vault.vault.azure.net/":         "https://my-vault.vault.azure.net",
		"https://my-vault.vault.azure.net":  "https://my-vault.vault.azure.net",
		"https://my-vault.vault.azure.net/": "https://my-vault.vault.azure.net",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			if got := normalizeAzureVaultURL(in); got != want {
				t.Errorf("normalizeAzureVaultURL(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

func TestSplitAzureSecretID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id, vault, name, version string
	}{
		{
			"https://my-vault.vault.azure.net/secrets/db-password/abc123",
			"https://my-vault.vault.azure.net", "db-password", "abc123",
		},
		{
			"https://my-vault.vault.azure.net/secrets/db-password",
			"https://my-vault.vault.azure.net", "db-password", "",
		},
		{"not-a-key-vault-url", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			t.Parallel()
			v, n, ver := splitAzureSecretID(tc.id)
			if v != tc.vault || n != tc.name || ver != tc.version {
				t.Errorf("splitAzureSecretID(%q) = (%q, %q, %q), want (%q, %q, %q)",
					tc.id, v, n, ver, tc.vault, tc.name, tc.version)
			}
		})
	}
}

func TestHumanizeAzureCredErr(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"DefaultAzureCredential: failed to acquire a token", "az login"},
		{"AADSTS70043 The refresh token has expired", "az login"},
		{"AADSTS50034 user does not exist in this tenant", "wrong --tenant"},
		{"random other", "azure:"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got := humanizeAzureCredErr(stubErr(tc.in))
			if !strings.Contains(got, tc.want) {
				t.Errorf("humanizeAzureCredErr(%q) = %q, want it to contain %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestInferAzureSource(t *testing.T) {
	t.Parallel()
	cases := []struct {
		env, want string
	}{
		{"AZURE_CLIENT_SECRET", "service principal (env)"},
		{"AZURE_FEDERATED_TOKEN_FILE", "workload identity (federated)"},
		{"MSI_ENDPOINT", "managed identity"},
		{"", "az CLI / azd CLI cache"},
	}
	for _, tc := range cases {
		t.Run(tc.env, func(t *testing.T) {
			// Cannot run in parallel — mutates the package-level
			// lookupEnv stub.
			original := lookupEnv
			t.Cleanup(func() { lookupEnv = original })
			lookupEnv = func(k string) (string, bool) {
				if tc.env != "" && k == tc.env {
					return "x", true
				}
				return "", false
			}
			if got := inferAzureSource(); got != tc.want {
				t.Errorf("inferAzureSource() with %q = %q, want %q", tc.env, got, tc.want)
			}
		})
	}
}

// azureClaim parses an unsigned-from-our-perspective JWT.
func TestAzureClaim(t *testing.T) {
	t.Parallel()
	// header.payload.signature — only payload matters here.
	// payload: {"upn":"jane@contoso.com","tid":"tenant-id"}
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"upn":"jane@contoso.com","tid":"tenant-id"}`))
	tok := "x." + payload + ".y"
	if got := azureClaim(tok, "upn"); got != "jane@contoso.com" {
		t.Errorf("upn = %q", got)
	}
	if got := azureClaim(tok, "tid"); got != "tenant-id" {
		t.Errorf("tid = %q", got)
	}
	if got := azureClaim(tok, "missing"); got != "" {
		t.Errorf("missing = %q, want \"\"", got)
	}
	if got := azureClaim("not.a.jwt", "upn"); got != "" {
		t.Errorf("bad payload = %q", got)
	}
}

type stubError string

func (s stubError) Error() string { return string(s) }

func stubErr(s string) error { return stubError(s) }
