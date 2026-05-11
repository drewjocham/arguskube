package pkg

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/argues/argus/internal/secretref"
)

// These tests construct a minimal *App with only secretRefResolver
// populated — the methods under test don't touch any other field.

func TestApp_DescribeSecretRef_Inline(t *testing.T) {
	a := &App{secretRefResolver: secretref.NewResolver()}
	info := a.DescribeSecretRef("plain value")
	if info.Kind != "inline" {
		t.Errorf("kind = %q", info.Kind)
	}
	if info.Value != "plain value" {
		t.Errorf("value = %q", info.Value)
	}
	if info.Resolvable {
		t.Error("inline should not be Resolvable")
	}
	if !info.Supported {
		t.Error("inline should be Supported")
	}
	if info.Description != "Inline value" {
		t.Errorf("description = %q", info.Description)
	}
}

func TestApp_DescribeSecretRef_EmptyInline(t *testing.T) {
	a := &App{secretRefResolver: secretref.NewResolver()}
	info := a.DescribeSecretRef("")
	if info.Description != "Empty inline value" {
		t.Errorf("description = %q", info.Description)
	}
}

func TestApp_DescribeSecretRef_Env(t *testing.T) {
	a := &App{secretRefResolver: secretref.NewResolver()}
	info := a.DescribeSecretRef("env:HOST")
	if info.Kind != "env" || info.Value != "HOST" {
		t.Errorf("got %+v", info)
	}
	if !info.Resolvable {
		t.Error("env should be Resolvable")
	}
	if !info.Supported {
		t.Error("env should be Supported (registered by NewResolver)")
	}
	if info.Description != "Env var · HOST" {
		t.Errorf("description = %q", info.Description)
	}
}

func TestApp_DescribeSecretRef_AwsSecret_NotSupported_WhenNotRegistered(t *testing.T) {
	a := &App{secretRefResolver: secretref.NewResolver()}
	info := a.DescribeSecretRef("aws-secret:prod/db#user")
	if info.Kind != "aws-secret" {
		t.Errorf("kind = %q", info.Kind)
	}
	if !info.Resolvable {
		t.Error("aws-secret should be Resolvable")
	}
	if info.Supported {
		t.Error("aws-secret should not be Supported without registration")
	}
	if info.Description != "AWS Secrets Mgr · prod/db (user)" {
		t.Errorf("description = %q", info.Description)
	}
}

func TestApp_DescribeSecretRef_AwsSecret_SupportedAfterRegistration(t *testing.T) {
	r := secretref.NewResolver()
	r.Use(secretref.NewCloudSource(secretref.KindAWSSecret, func(context.Context, string, string) ([]byte, error) {
		return []byte("v"), nil
	}))
	a := &App{secretRefResolver: r}
	info := a.DescribeSecretRef("aws-secret:foo")
	if !info.Supported {
		t.Error("aws-secret should be Supported after Use()")
	}
}

func TestApp_DescribeSecretRef_NilResolver_OnlyInlineSupported(t *testing.T) {
	a := &App{}
	if !a.DescribeSecretRef("plain").Supported {
		t.Error("inline must always be Supported")
	}
	if a.DescribeSecretRef("env:HOST").Supported {
		t.Error("env should not be Supported with nil resolver")
	}
}

func TestApp_DescribeSecretRef_Vault(t *testing.T) {
	a := &App{secretRefResolver: secretref.NewResolver()}
	info := a.DescribeSecretRef("vault:gh-pat#token")
	if info.Description != "Argus Vault · gh-pat (token)" {
		t.Errorf("description = %q", info.Description)
	}
}

func TestApp_ResolveSecretRef_InlinePassesThrough(t *testing.T) {
	a := &App{secretRefResolver: secretref.NewResolver()}
	val, errMsg := a.ResolveSecretRef("hello world")
	if val != "hello world" {
		t.Errorf("val = %q", val)
	}
	if errMsg != "" {
		t.Errorf("errMsg = %q", errMsg)
	}
}

func TestApp_ResolveSecretRef_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if err := os.WriteFile(path, []byte("file-value"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &App{secretRefResolver: secretref.NewResolver()}
	val, errMsg := a.ResolveSecretRef("file:" + path)
	if errMsg != "" {
		t.Errorf("errMsg = %q", errMsg)
	}
	if val != "file-value" {
		t.Errorf("val = %q", val)
	}
}

func TestApp_ResolveSecretRef_PropagatesError(t *testing.T) {
	a := &App{secretRefResolver: secretref.NewResolver()}
	val, errMsg := a.ResolveSecretRef("file:/does/not/exist")
	if errMsg == "" {
		t.Error("expected non-empty errMsg")
	}
	if val != "" {
		t.Errorf("val on error = %q", val)
	}
}

func TestApp_ResolveSecretRef_NilResolver(t *testing.T) {
	a := &App{}
	val, errMsg := a.ResolveSecretRef("anything")
	if errMsg == "" {
		t.Error("expected error with nil resolver")
	}
	if val != "" {
		t.Errorf("val should be empty on error, got %q", val)
	}
}

func TestApp_ResolveSecretRef_CustomCloudSource(t *testing.T) {
	r := secretref.NewResolver()
	r.Use(secretref.NewCloudSource(secretref.KindAWSSecret, func(_ context.Context, val, key string) ([]byte, error) {
		if val != "prod/db" || key != "user" {
			return nil, errors.New("unexpected ref")
		}
		return []byte("admin"), nil
	}))
	a := &App{secretRefResolver: r}
	val, errMsg := a.ResolveSecretRef("aws-secret:prod/db#user")
	if errMsg != "" || val != "admin" {
		t.Errorf("val=%q err=%q", val, errMsg)
	}
}

func TestSecretRefKindLabel_Coverage(t *testing.T) {
	cases := map[secretref.Kind]string{
		secretref.KindEnv:        "Env var",
		secretref.KindFile:       "File",
		secretref.KindVolume:     "Volume",
		secretref.KindAWSSecret:  "AWS Secrets Mgr",
		secretref.KindGCPSecret:  "GCP Secret Mgr",
		secretref.KindAzureVault: "Azure Key Vault",
		secretref.KindArgusVault: "Argus Vault",
	}
	for kind, want := range cases {
		if got := secretRefKindLabel(kind); got != want {
			t.Errorf("kindLabel(%s) = %q, want %q", kind, got, want)
		}
	}
	// Unknown kind falls back to the string form.
	if got := secretRefKindLabel(secretref.Kind("mystery")); got != "mystery" {
		t.Errorf("unknown kind = %q", got)
	}
}

func TestApp_DescribeSecretRef_TrailingWhitespaceInRefSurvives(t *testing.T) {
	// Operators may paste config with stray spaces; we don't trim
	// arbitrary whitespace from the body because passwords can contain
	// trailing spaces. Only the kind separator's leading space is
	// special-cased.
	a := &App{secretRefResolver: secretref.NewResolver()}
	info := a.DescribeSecretRef("vault:  gh-pat")
	if !strings.Contains(info.Description, " gh-pat") {
		t.Errorf("description = %q", info.Description)
	}
}
