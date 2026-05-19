package cloud

import (
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
		"": "",
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

type stubError string

func (s stubError) Error() string { return string(s) }

func stubErr(s string) error { return stubError(s) }
