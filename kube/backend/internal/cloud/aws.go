package cloud

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// AWSProvider implements Provider against AWS Secrets Manager + STS.
// The zero value is usable; the SDK's default credential chain
// (env → shared config → SSO cache → IMDS) discovers credentials
// lazily on each call so we don't pin them to a stale snapshot at
// Argus startup — important once the user runs `aws sso login` mid-
// session and expects the console to pick it up.
type AWSProvider struct{}

func (AWSProvider) Identity(ctx context.Context) Identity {
	id := Identity{Provider: ProviderAWS}

	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		id.Error = fmt.Sprintf("load default config: %s", err)
		return id
	}

	// Resolve credentials once so we can read .Source and .Expires
	// before making any network call. If the chain is empty we get
	// a typed error we can render cleanly.
	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		id.Error = humanizeAWSCredError(err)
		return id
	}
	id.Source = creds.Source
	if creds.CanExpire {
		id.ExpiresAt = creds.Expires
		id.Expired = !creds.Expires.IsZero() && time.Now().After(creds.Expires)
	}

	// GetCallerIdentity is the canonical "who am I" probe — costs
	// almost nothing and works against every endpoint.
	out, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		id.Error = fmt.Sprintf("sts get-caller-identity: %s", err)
		return id
	}
	id.Authenticated = !id.Expired
	id.Subject = awssdk.ToString(out.Arn)
	id.Account = awssdk.ToString(out.Account)
	return id
}

func (AWSProvider) ListSecrets(ctx context.Context, opts ListOpts) ([]SecretItem, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsRegionOpt(opts.Region)...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	cli := secretsmanager.NewFromConfig(cfg)

	items := []SecretItem{}
	var nextToken *string
	// Hard cap so a user with 10k secrets doesn't melt the UI on
	// first open — they can narrow by region or search to drill in.
	const maxPages = 10
	for page := 0; page < maxPages; page++ {
		out, err := cli.ListSecrets(ctx, &secretsmanager.ListSecretsInput{
			NextToken:  nextToken,
			MaxResults: awssdk.Int32(100),
		})
		if err != nil {
			return nil, fmt.Errorf("list secrets: %w", err)
		}
		for _, s := range out.SecretList {
			items = append(items, SecretItem{
				Name:        awssdk.ToString(s.ARN),
				DisplayName: shortAWSName(awssdk.ToString(s.Name), awssdk.ToString(s.ARN)),
				Region:      cfg.Region,
				Created:     deref(s.CreatedDate),
				Updated:     deref(s.LastChangedDate),
				Description: awssdk.ToString(s.Description),
				Labels:      awsTagsToMap(s.Tags),
			})
		}
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	sort.Slice(items, func(i, j int) bool { return items[i].DisplayName < items[j].DisplayName })
	return items, nil
}

func (AWSProvider) RevealSecret(ctx context.Context, name string) (SecretValue, error) {
	// name is the secret ARN or short name. Region is encoded in the
	// ARN; for short names we fall back to the SDK's default config.
	region := regionFromAWSARN(name)
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsRegionOpt(region)...)
	if err != nil {
		return SecretValue{}, fmt.Errorf("load aws config: %w", err)
	}
	out, err := secretsmanager.NewFromConfig(cfg).GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: awssdk.String(name),
	})
	if err != nil {
		return SecretValue{}, fmt.Errorf("get secret value: %w", err)
	}
	if out.SecretString != nil {
		return SecretValue{Name: name, Value: *out.SecretString}, nil
	}
	if out.SecretBinary != nil {
		return SecretValue{
			Name:     name,
			Value:    base64.StdEncoding.EncodeToString(out.SecretBinary),
			IsBinary: true,
		}, nil
	}
	return SecretValue{Name: name}, nil
}

// ── helpers ──────────────────────────────────────────────────────

func awsRegionOpt(region string) []func(*awsconfig.LoadOptions) error {
	if region == "" {
		return nil
	}
	return []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
}

func humanizeAWSCredError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "failed to refresh cached credentials"):
		return "AWS credentials expired — run `aws sso login` (or refresh your access keys)"
	case strings.Contains(msg, "no EC2 IMDS role found"):
		return "no AWS credentials available — set AWS_PROFILE, env keys, or run `aws sso login`"
	default:
		return "no AWS credentials: " + msg
	}
}

// shortAWSName trims the ARN prefix off for the table display while
// keeping the full ARN as the lookup key.
func shortAWSName(name, arn string) string {
	if name != "" {
		return name
	}
	// Fallback: derive the trailing segment from the ARN.
	if i := strings.LastIndex(arn, ":"); i >= 0 && i+1 < len(arn) {
		return arn[i+1:]
	}
	return arn
}

func regionFromAWSARN(arn string) string {
	// arn:aws:secretsmanager:<region>:<account>:secret:<name>
	parts := strings.Split(arn, ":")
	if len(parts) >= 4 {
		return parts[3]
	}
	return ""
}

func awsTagsToMap(tags []smtypes.Tag) map[string]string {
	if len(tags) == 0 {
		return nil
	}
	out := make(map[string]string, len(tags))
	for _, t := range tags {
		k := awssdk.ToString(t.Key)
		v := awssdk.ToString(t.Value)
		if k != "" {
			out[k] = v
		}
	}
	return out
}

func deref(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
