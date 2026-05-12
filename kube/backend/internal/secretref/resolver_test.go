package secretref

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in       string
		wantKind Kind
		wantVal  string
		wantKey  string
	}{
		{"", KindInline, "", ""},
		{"plain", KindInline, "plain", ""},
		{"env:DATABASE_URL", KindEnv, "DATABASE_URL", ""},
		{"file:/etc/x", KindFile, "/etc/x", ""},
		{"volume:my-vol/inner", KindVolume, "my-vol/inner", ""},
		{"aws-secret:prod/db#user", KindAWSSecret, "prod/db", "user"},
		{"gcp-secret:projects/p/secrets/s", KindGCPSecret, "projects/p/secrets/s", ""},
		{"vault:gh#token", KindArgusVault, "gh", "token"},
		{"inline:literal value", KindInline, "literal value", ""},
		// Unknown prefixes fall through to inline preserving the colon.
		{"mysql://user:pass@host", KindInline, "mysql://user:pass@host", ""},
		// Trailing # without anything after is allowed as empty key.
		{"aws-secret:foo#", KindAWSSecret, "foo", ""},
		// Leading-space tolerance.
		{"vault: gh-pat", KindArgusVault, "gh-pat", ""},
		// Uppercase prefix is normalised.
		{"ENV:HOST", KindEnv, "HOST", ""},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := Parse(c.in)
			if got.Kind != c.wantKind {
				t.Errorf("kind: got %q, want %q", got.Kind, c.wantKind)
			}
			if got.Value != c.wantVal {
				t.Errorf("value: got %q, want %q", got.Value, c.wantVal)
			}
			if got.Key != c.wantKey {
				t.Errorf("key: got %q, want %q", got.Key, c.wantKey)
			}
		})
	}
}

func TestRef_String_RoundTrip(t *testing.T) {
	inputs := []string{
		"plain",
		"env:HOST",
		"file:/etc/x",
		"volume:my-vol/foo",
		"aws-secret:prod/db#user",
		"vault:gh",
	}
	for _, in := range inputs {
		if got := Parse(in).String(); got != in {
			t.Errorf("round-trip %q → %q", in, got)
		}
	}
	// Inline coerces away from any "inline:" prefix.
	if got := Parse("inline:foo").String(); got != "foo" {
		t.Errorf("inline:foo round-trip → %q (want foo)", got)
	}
}

func TestRef_IsResolvable(t *testing.T) {
	if Parse("").IsResolvable() {
		t.Error("empty should be inline + not resolvable")
	}
	if Parse("plain").IsResolvable() {
		t.Error("plain should be inline + not resolvable")
	}
	if !Parse("env:HOST").IsResolvable() {
		t.Error("env should be resolvable")
	}
}

func TestResolver_Inline(t *testing.T) {
	r := NewResolver()
	got, err := r.Resolve(context.Background(), "plain value")
	if err != nil {
		t.Fatalf("inline resolve err: %v", err)
	}
	if string(got) != "plain value" {
		t.Errorf("inline: got %q", got)
	}
}

func TestResolver_Env(t *testing.T) {
	t.Setenv("TEST_SECRETREF_FOO", "bar-value")
	r := NewResolver()
	got, err := r.ResolveString(context.Background(), "env:TEST_SECRETREF_FOO")
	if err != nil {
		t.Fatalf("env resolve err: %v", err)
	}
	if got != "bar-value" {
		t.Errorf("env: got %q", got)
	}
}

func TestResolver_Env_Missing(t *testing.T) {
	r := NewResolver()
	_, err := r.Resolve(context.Background(), "env:DEFINITELY_NOT_SET_X9")
	if err == nil {
		t.Fatal("expected error for missing env var")
	}
}

func TestResolver_Env_Empty(t *testing.T) {
	r := NewResolver()
	_, err := r.Resolve(context.Background(), "env:")
	if !errors.Is(err, ErrEmptyValue) {
		t.Errorf("want ErrEmptyValue, got %v", err)
	}
}

func TestResolver_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if err := os.WriteFile(path, []byte("file-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := NewResolver()
	got, err := r.ResolveString(context.Background(), "file:"+path)
	if err != nil {
		t.Fatalf("file resolve err: %v", err)
	}
	if got != "file-value" {
		t.Errorf("file: got %q", got)
	}
}

func TestResolver_File_Rejects_PathTraversal(t *testing.T) {
	r := NewResolver()
	r.FileRoot = t.TempDir()
	_, err := r.Resolve(context.Background(), "file:/../../etc/passwd")
	if !errors.Is(err, ErrUnsafePath) {
		t.Errorf("want ErrUnsafePath, got %v", err)
	}
}

func TestResolver_File_Empty(t *testing.T) {
	r := NewResolver()
	_, err := r.Resolve(context.Background(), "file:")
	if !errors.Is(err, ErrEmptyValue) {
		t.Errorf("want ErrEmptyValue, got %v", err)
	}
}

func TestResolver_File_FileRoot_Sandboxes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := NewResolver()
	r.FileRoot = dir
	got, err := r.ResolveString(context.Background(), "file:/x")
	if err != nil {
		t.Fatalf("file resolve err: %v", err)
	}
	if got != "hello" {
		t.Errorf("file: got %q", got)
	}
}

func TestResolver_Volume(t *testing.T) {
	dir := t.TempDir()
	volDir := filepath.Join(dir, "my-vol")
	if err := os.MkdirAll(volDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(volDir, "creds"), []byte("v-val"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := NewResolver()
	r.VolumeRoots = map[string]string{"my-vol": volDir}
	got, err := r.ResolveString(context.Background(), "volume:my-vol/creds")
	if err != nil {
		t.Fatalf("volume resolve err: %v", err)
	}
	if got != "v-val" {
		t.Errorf("volume: got %q", got)
	}
}

func TestResolver_Volume_FallsBackToFileRoot(t *testing.T) {
	dir := t.TempDir()
	volDir := filepath.Join(dir, "fallback-vol")
	if err := os.MkdirAll(volDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(volDir, "creds"), []byte("fb-val"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := NewResolver()
	r.FileRoot = dir // VolumeRoots not set
	got, err := r.ResolveString(context.Background(), "volume:fallback-vol/creds")
	if err != nil {
		t.Fatalf("volume fallback err: %v", err)
	}
	if got != "fb-val" {
		t.Errorf("volume fallback: got %q", got)
	}
}

func TestResolver_Volume_NoHostPath_Errors(t *testing.T) {
	r := NewResolver()
	_, err := r.Resolve(context.Background(), "volume:mystery/x")
	if err == nil {
		t.Fatal("expected error for unmapped volume")
	}
}

func TestResolver_Volume_BadFormat(t *testing.T) {
	r := NewResolver()
	r.VolumeRoots = map[string]string{"vol": "/tmp"}
	for _, bad := range []string{"volume:just-name", "volume:/no-name", "volume:vol/"} {
		if _, err := r.Resolve(context.Background(), bad); err == nil {
			t.Errorf("expected error for malformed %q", bad)
		}
	}
}

func TestResolver_UnknownKind_ReturnsErr(t *testing.T) {
	r := NewResolver()
	// aws-secret isn't registered by default.
	_, err := r.Resolve(context.Background(), "aws-secret:foo")
	if !errors.Is(err, ErrUnknownKind) {
		t.Errorf("want ErrUnknownKind, got %v", err)
	}
}

func TestResolver_CloudSource(t *testing.T) {
	calls := 0
	fetcher := func(_ context.Context, val, key string) ([]byte, error) {
		calls++
		if val != "prod/db" || key != "username" {
			t.Errorf("unexpected call: val=%q key=%q", val, key)
		}
		return []byte("admin"), nil
	}
	r := NewResolver().Use(NewCloudSource(KindAWSSecret, fetcher))
	got, err := r.ResolveString(context.Background(), "aws-secret:prod/db#username")
	if err != nil {
		t.Fatalf("aws-secret resolve err: %v", err)
	}
	if got != "admin" {
		t.Errorf("got %q", got)
	}
	if calls != 1 {
		t.Errorf("fetcher called %d times, want 1", calls)
	}
}

func TestResolver_CloudSource_PropagatesError(t *testing.T) {
	myErr := errors.New("aws unreachable")
	fetcher := func(_ context.Context, _, _ string) ([]byte, error) {
		return nil, myErr
	}
	r := NewResolver().Use(NewCloudSource(KindAWSSecret, fetcher))
	_, err := r.Resolve(context.Background(), "aws-secret:foo")
	if !errors.Is(err, myErr) {
		t.Errorf("want %v, got %v", myErr, err)
	}
}

func TestResolver_CloudSource_EmptyValue(t *testing.T) {
	r := NewResolver().Use(NewCloudSource(KindAWSSecret, func(context.Context, string, string) ([]byte, error) {
		return []byte("never"), nil
	}))
	_, err := r.Resolve(context.Background(), "aws-secret:")
	if !errors.Is(err, ErrEmptyValue) {
		t.Errorf("want ErrEmptyValue, got %v", err)
	}
}

func TestResolver_ArgusVaultSource(t *testing.T) {
	lookup := func(_ context.Context, entry, key string) ([]byte, error) {
		if entry != "gh-pat" || key != "token" {
			t.Errorf("unexpected lookup: %s/%s", entry, key)
		}
		return []byte("ghs_xxxxxx"), nil
	}
	r := NewResolver().Use(NewArgusVaultSource(lookup))
	got, err := r.ResolveString(context.Background(), "vault:gh-pat#token")
	if err != nil {
		t.Fatalf("vault resolve err: %v", err)
	}
	if got != "ghs_xxxxxx" {
		t.Errorf("got %q", got)
	}
}

func TestResolver_Has(t *testing.T) {
	r := NewResolver()
	if !r.Has(KindInline) || !r.Has(KindEnv) || !r.Has(KindFile) || !r.Has(KindVolume) {
		t.Error("default resolver should have inline/env/file/volume")
	}
	if r.Has(KindAWSSecret) {
		t.Error("default resolver should NOT have aws-secret")
	}
	r.Use(NewCloudSource(KindAWSSecret, nil))
	if !r.Has(KindAWSSecret) {
		t.Error("after Use, aws-secret should be registered")
	}
}

func TestResolver_Use_Nil_NoOp(t *testing.T) {
	r := NewResolver()
	r.Use(nil) // must not panic
	if !r.Has(KindInline) {
		t.Error("Use(nil) should not disturb existing sources")
	}
}

func TestResolveString_TrimsTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if err := os.WriteFile(path, []byte("value\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := NewResolver()
	got, err := r.ResolveString(context.Background(), "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "value" {
		t.Errorf("expected trailing CRLF trimmed, got %q", got)
	}
}

func TestParse_PreservesValueWithEmbeddedHash(t *testing.T) {
	// The LAST # is the key separator — preceding # stay in the value.
	r := Parse("aws-secret:weird#name#real-key")
	if r.Value != "weird#name" {
		t.Errorf("value: got %q", r.Value)
	}
	if r.Key != "real-key" {
		t.Errorf("key: got %q", r.Key)
	}
}

func TestResolver_FileRoot_PathTraversal_Blocked(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok"), []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := NewResolver()
	r.FileRoot = dir
	_, err := r.Resolve(context.Background(), "file:/../etc/passwd")
	if !errors.Is(err, ErrUnsafePath) {
		t.Errorf("want ErrUnsafePath, got %v", err)
	}
}
