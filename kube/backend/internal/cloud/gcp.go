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

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	smpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
)

// GCPProvider implements Provider against Google Secret Manager and
// Google Application Default Credentials. Same lazy-credential
// posture as AWSProvider — every call re-reads ADC so a fresh
// `gcloud auth login` is picked up without a restart.
type GCPProvider struct{}

// The full set of OAuth scopes we ever request. cloud-platform covers
// Secret Manager + STS + userinfo — narrower scopes would require
// re-running the auth dance per surface.
var gcpScopes = []string{"https://www.googleapis.com/auth/cloud-platform"}

func (GCPProvider) Identity(ctx context.Context) Identity {
	id := Identity{Provider: ProviderGCP}

	creds, err := google.FindDefaultCredentials(ctx, gcpScopes...)
	if err != nil {
		id.Error = humanizeGCPCredError(err)
		return id
	}

	// JSON tells us whether ADC came from a SA key file, an end-user
	// gcloud login, or the metadata server.
	id.Source = inferGCPSource(creds.JSON)
	id.Account = creds.ProjectID

	tok, err := creds.TokenSource.Token()
	if err != nil {
		id.Error = humanizeGCPCredError(err)
		return id
	}
	if !tok.Expiry.IsZero() {
		id.ExpiresAt = tok.Expiry
		id.Expired = time.Now().After(tok.Expiry)
	}

	// Try userinfo for a friendly subject. Best-effort — failing this
	// shouldn't blank the identity card; we already know the token is
	// good from above.
	id.Subject = lookupGCPUserinfo(ctx, tok.AccessToken)
	if id.Subject == "" {
		// Fall back to credential-file-derived hint.
		id.Subject = subjectHintFromCredsJSON(creds.JSON)
	}
	id.Authenticated = !id.Expired
	return id
}

func (GCPProvider) ListSecrets(ctx context.Context, opts ListOpts) ([]SecretItem, error) {
	project, err := resolveGCPProject(ctx, opts.Project)
	if err != nil {
		return nil, err
	}
	cli, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("secretmanager client: %w", err)
	}
	defer cli.Close()

	items := []SecretItem{}
	it := cli.ListSecrets(ctx, &smpb.ListSecretsRequest{
		Parent: "projects/" + project,
	})
	// Same hard cap as AWS — 1000 secrets is plenty for a UI list,
	// power users can filter later.
	const maxItems = 1000
	for i := 0; i < maxItems; i++ {
		s, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterate secrets: %w", err)
		}
		items = append(items, SecretItem{
			Name:        s.Name + "/versions/latest",
			DisplayName: shortGCPName(s.Name),
			Region:      gcpLocationOf(s),
			Created:     timeFromPB(s.CreateTime),
			Updated:     timeFromPB(s.CreateTime),
			Labels:      s.Labels,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].DisplayName < items[j].DisplayName })
	return items, nil
}

func (GCPProvider) RevealSecret(ctx context.Context, name string) (SecretValue, error) {
	cli, err := secretmanager.NewClient(ctx)
	if err != nil {
		return SecretValue{}, fmt.Errorf("secretmanager client: %w", err)
	}
	defer cli.Close()
	out, err := cli.AccessSecretVersion(ctx, &smpb.AccessSecretVersionRequest{Name: name})
	if err != nil {
		return SecretValue{}, fmt.Errorf("access secret version: %w", err)
	}
	data := out.Payload.GetData()
	// Mirror AWS behavior: if the payload is not valid UTF-8 we treat
	// it as binary and send base64 so the existing frontend decode
	// path handles it.
	if utf8.Valid(data) {
		return SecretValue{Name: name, Value: string(data)}, nil
	}
	return SecretValue{
		Name:     name,
		Value:    base64.StdEncoding.EncodeToString(data),
		IsBinary: true,
	}, nil
}

// ── helpers ──────────────────────────────────────────────────────

func humanizeGCPCredError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "could not find default credentials"):
		return "no GCP credentials available — run `gcloud auth application-default login`"
	case strings.Contains(msg, "invalid_grant"):
		return "GCP credentials expired — run `gcloud auth application-default login`"
	default:
		return "no GCP credentials: " + msg
	}
}

// inferGCPSource peeks at the ADC JSON to label where the credential
// came from. Best-effort — we just need a hint for the identity card.
func inferGCPSource(adcJSON []byte) string {
	s := string(adcJSON)
	switch {
	case strings.Contains(s, `"authorized_user"`):
		return "gcloud user (ADC)"
	case strings.Contains(s, `"service_account"`):
		return "service account key file"
	case strings.Contains(s, `"external_account"`):
		return "external account (workload identity)"
	case strings.Contains(s, `"impersonated_service_account"`):
		return "impersonated service account"
	default:
		// Metadata server credentials return no JSON.
		if len(adcJSON) == 0 {
			return "GCE/GKE metadata server"
		}
		return ""
	}
}

func subjectHintFromCredsJSON(adcJSON []byte) string {
	s := string(adcJSON)
	for _, key := range []string{`"client_email"`, `"account"`} {
		i := strings.Index(s, key)
		if i < 0 {
			continue
		}
		rest := s[i+len(key):]
		if j := strings.Index(rest, `"`); j >= 0 {
			rest = rest[j+1:]
			if k := strings.Index(rest, `"`); k > 0 {
				return rest[:k]
			}
		}
	}
	return ""
}

// resolveGCPProject prefers the user-supplied opts.Project; if absent,
// falls back to whatever ADC reports.
func resolveGCPProject(ctx context.Context, project string) (string, error) {
	if project != "" {
		return project, nil
	}
	creds, err := google.FindDefaultCredentials(ctx, gcpScopes...)
	if err != nil {
		return "", fmt.Errorf("no project supplied and no default credentials: %w", err)
	}
	if creds.ProjectID == "" {
		return "", errors.New("no project supplied and ADC carries no project — pass project explicitly")
	}
	return creds.ProjectID, nil
}

func shortGCPName(full string) string {
	// "projects/123/secrets/my-secret"
	if i := strings.LastIndex(full, "/"); i >= 0 && i+1 < len(full) {
		return full[i+1:]
	}
	return full
}

func gcpLocationOf(s *smpb.Secret) string {
	if s == nil || s.Replication == nil {
		return ""
	}
	switch r := s.Replication.Replication.(type) {
	case *smpb.Replication_Automatic_:
		return "automatic"
	case *smpb.Replication_UserManaged_:
		if r.UserManaged != nil && len(r.UserManaged.Replicas) > 0 {
			locs := make([]string, 0, len(r.UserManaged.Replicas))
			for _, rep := range r.UserManaged.Replicas {
				locs = append(locs, rep.Location)
			}
			return strings.Join(locs, ",")
		}
	}
	return ""
}

func timeFromPB(t interface{ AsTime() time.Time }) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.AsTime()
}

// lookupGCPUserinfo hits the v1 userinfo endpoint with the access
// token. Failures are non-fatal — callers fall back to inferring from
// the JSON credential.
func lookupGCPUserinfo(ctx context.Context, accessToken string) string {
	// Implemented as an HTTP call (not the userinfo library) because
	// google.golang.org/api/oauth2/v2 pulls a big dep tree for a
	// single field. See gcp_userinfo.go for the tiny client.
	return fetchGCPEmail(ctx, accessToken)
}
