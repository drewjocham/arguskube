// Package secretref resolves "label-prefixed" config values into their
// real values at use time. Anywhere a user can supply an env var, token,
// or password, the value may be either a literal or a label-prefixed
// reference like:
//
//	env:DATABASE_URL
//	file:/etc/x/secret
//	volume:my-vol/secret-key
//	aws-secret:prod/db-creds#username
//	gcp-secret:projects/p/secrets/s#versions/latest
//	azure-vault:my-vault/my-secret
//	vault:gh-pat#token             (Argus local credential vault)
//	inline:literal-value           (or no prefix at all)
//
// The grammar matches the frontend lib/secretRef.js parser one-for-one so
// the two stay in sync.
//
// Resolution is intentionally pull-based: nothing is fetched until the
// caller asks for it, and each Source is a small interface that tests
// can substitute. Real cloud-vault clients (AWS Secrets Manager, etc.)
// are wired in by the app layer; this package stays dependency-free so
// it builds + tests in any environment.
package secretref

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Kind enumerates every recognised reference source.
type Kind string

const (
	KindInline      Kind = "inline"
	KindEnv         Kind = "env"
	KindFile        Kind = "file"
	KindVolume      Kind = "volume"
	KindAWSSecret   Kind = "aws-secret"
	KindGCPSecret   Kind = "gcp-secret"
	KindAzureVault  Kind = "azure-vault"
	KindArgusVault  Kind = "vault"
)

var knownKinds = map[Kind]struct{}{
	KindInline:     {},
	KindEnv:        {},
	KindFile:       {},
	KindVolume:     {},
	KindAWSSecret:  {},
	KindGCPSecret:  {},
	KindAzureVault: {},
	KindArgusVault: {},
}

// Ref is the parsed form of a reference string.
type Ref struct {
	Kind  Kind   // resolution source
	Value string // the body — meaning depends on Kind
	Key   string // optional "#key" suffix
	Raw   string // the original string for diagnostics
}

// String reformats a Ref back to its canonical "kind:value[#key]" form.
// For KindInline (or empty kind) this returns just the value with no
// prefix, matching the frontend formatter.
func (r Ref) String() string {
	if r.Kind == "" || r.Kind == KindInline {
		return r.Value
	}
	if r.Key != "" {
		return fmt.Sprintf("%s:%s#%s", r.Kind, r.Value, r.Key)
	}
	return fmt.Sprintf("%s:%s", r.Kind, r.Value)
}

// IsResolvable reports whether the ref needs an external lookup. Inline
// values pass through unchanged; everything else routes through a Source.
func (r Ref) IsResolvable() bool {
	return r.Kind != "" && r.Kind != KindInline
}

// Parse takes the user-typed string and returns a Ref. Whitespace + case
// for the kind is normalised; an unknown prefix is treated as inline so
// values like "mysql://user:pass@host" survive unscathed.
func Parse(input string) Ref {
	raw := input
	if raw == "" {
		return Ref{Kind: KindInline, Value: "", Raw: ""}
	}
	colon := strings.Index(raw, ":")
	if colon < 0 {
		return Ref{Kind: KindInline, Value: raw, Raw: raw}
	}
	candidate := Kind(strings.ToLower(strings.TrimSpace(raw[:colon])))
	if _, ok := knownKinds[candidate]; !ok {
		// Unknown prefix — treat as inline (preserve the colon).
		return Ref{Kind: KindInline, Value: raw, Raw: raw}
	}
	if candidate == KindInline {
		// Explicit "inline:foo" — strip the prefix.
		return Ref{Kind: KindInline, Value: raw[colon+1:], Raw: raw}
	}
	body := raw[colon+1:]
	body = strings.TrimPrefix(body, " ")
	value := body
	key := ""
	if hash := strings.LastIndex(body, "#"); hash >= 0 {
		value = body[:hash]
		key = body[hash+1:]
	}
	return Ref{Kind: candidate, Value: value, Key: key, Raw: raw}
}

// Source is the per-kind adapter the resolver delegates to. Each source
// returns the raw bytes (so binary values like TLS keys are supported)
// or an error.
//
// Implementations should be safe to call concurrently — Resolver does
// not serialise calls across sources.
type Source interface {
	// Kind returns the Kind this source handles.
	Kind() Kind
	// Fetch resolves a single reference. The Resolver guarantees ref.Kind
	// matches Kind() before calling.
	Fetch(ctx context.Context, ref Ref) ([]byte, error)
}

// Resolver dispatches Refs to the right Source. Construct with
// NewResolver and register Sources via Use. Resolver is safe to share
// across goroutines once configured.
type Resolver struct {
	sources map[Kind]Source

	// FileRoot, if non-empty, is prepended to all file:/volume: refs.
	// Tests use this to point the resolver at a sandbox directory and
	// avoid touching the real filesystem.
	FileRoot string

	// VolumeRoots, if non-empty, maps volume name → host path on disk.
	// Falls back to FileRoot+"/"+volume when no mapping is set.
	VolumeRoots map[string]string
}

// NewResolver builds an empty resolver with the inline + env + file +
// volume sources pre-registered. Caller adds cloud sources via Use().
func NewResolver() *Resolver {
	r := &Resolver{sources: map[Kind]Source{}}
	r.Use(inlineSource{})
	r.Use(envSource{})
	r.Use(&fileSource{resolver: r})
	r.Use(&volumeSource{resolver: r})
	return r
}

// Use registers a Source, overwriting any previous registration for the
// same Kind. Returns the resolver for chaining.
func (r *Resolver) Use(s Source) *Resolver {
	if s == nil {
		return r
	}
	r.sources[s.Kind()] = s
	return r
}

// Has reports whether a Source is registered for the given Kind.
func (r *Resolver) Has(k Kind) bool {
	_, ok := r.sources[k]
	return ok
}

// Resolve takes a raw reference string, parses it, and returns the
// resolved bytes. Inline values pass through verbatim. Unknown kinds
// return an ErrUnknownKind so callers can detect "configuration says
// aws-secret but no AWS source is wired up".
func (r *Resolver) Resolve(ctx context.Context, raw string) ([]byte, error) {
	ref := Parse(raw)
	return r.ResolveRef(ctx, ref)
}

// ResolveRef is the parsed-input variant of Resolve.
func (r *Resolver) ResolveRef(ctx context.Context, ref Ref) ([]byte, error) {
	if !ref.IsResolvable() {
		return []byte(ref.Value), nil
	}
	src, ok := r.sources[ref.Kind]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownKind, ref.Kind)
	}
	return src.Fetch(ctx, ref)
}

// ResolveString is the same as Resolve but returns a string. Trailing
// newlines are stripped (a common foot-gun when reading files where
// editors append a final \n).
func (r *Resolver) ResolveString(ctx context.Context, raw string) (string, error) {
	b, err := r.Resolve(ctx, raw)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(b), "\r\n"), nil
}

// Errors.
var (
	ErrUnknownKind = errors.New("secretref: no source registered for kind")
	ErrEmptyValue  = errors.New("secretref: empty reference value")
	ErrUnsafePath  = errors.New("secretref: file path traversal rejected")
)

// ---- Built-in sources --------------------------------------------------

type inlineSource struct{}

func (inlineSource) Kind() Kind { return KindInline }
func (inlineSource) Fetch(_ context.Context, r Ref) ([]byte, error) {
	return []byte(r.Value), nil
}

type envSource struct{}

func (envSource) Kind() Kind { return KindEnv }
func (envSource) Fetch(_ context.Context, r Ref) ([]byte, error) {
	if r.Value == "" {
		return nil, ErrEmptyValue
	}
	val, ok := os.LookupEnv(r.Value)
	if !ok {
		return nil, fmt.Errorf("env var %q not set", r.Value)
	}
	return []byte(val), nil
}

type fileSource struct {
	resolver *Resolver
}

func (s *fileSource) Kind() Kind { return KindFile }
func (s *fileSource) Fetch(_ context.Context, r Ref) ([]byte, error) {
	if r.Value == "" {
		return nil, ErrEmptyValue
	}
	// Reject obvious traversal attempts on the raw input BEFORE joining.
	// filepath.Clean would resolve "/foo/../etc" → "/etc" and silently
	// bypass our check, so we look at the user's typed value first.
	if hasTraversalSegment(r.Value) {
		return nil, fmt.Errorf("%w: %s", ErrUnsafePath, r.Value)
	}
	path := r.Value
	root := ""
	if s.resolver != nil && s.resolver.FileRoot != "" {
		root = s.resolver.FileRoot
		// FileRoot is for sandboxing tests. We treat the ref as a path
		// rooted at FileRoot if it's absolute (which is the documented
		// form) — Join handles the leading slash safely.
		path = filepath.Join(root, path)
	}
	if err := assertWithinRoot(root, path); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

type volumeSource struct {
	resolver *Resolver
}

func (s *volumeSource) Kind() Kind { return KindVolume }
func (s *volumeSource) Fetch(_ context.Context, r Ref) ([]byte, error) {
	// volume:<name>/<path-inside-volume>
	slash := strings.IndexByte(r.Value, '/')
	if slash <= 0 || slash == len(r.Value)-1 {
		return nil, fmt.Errorf("volume ref must be <name>/<path>: %q", r.Value)
	}
	name, inner := r.Value[:slash], r.Value[slash+1:]
	if hasTraversalSegment(inner) {
		return nil, fmt.Errorf("%w: %s", ErrUnsafePath, r.Value)
	}
	var root string
	if s.resolver != nil {
		if v, ok := s.resolver.VolumeRoots[name]; ok {
			root = v
		} else if s.resolver.FileRoot != "" {
			root = filepath.Join(s.resolver.FileRoot, name)
		}
	}
	if root == "" {
		return nil, fmt.Errorf("no host path configured for volume %q", name)
	}
	path := filepath.Join(root, inner)
	if err := assertWithinRoot(root, path); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

// hasTraversalSegment inspects a path BEFORE filepath.Clean and reports
// whether any ".." segment is present. We can't trust the post-Clean
// form because Clean resolves traversal away — "/foo/../etc" becomes
// "/etc" and our check would never trigger. Instead we look at the raw
// segments as the user typed them.
func hasTraversalSegment(p string) bool {
	for _, part := range strings.Split(filepath.ToSlash(p), "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

// assertWithinRoot rejects a joined path that resolves outside the
// configured root. When root is empty (no FileRoot configured) it's a
// pass-through — the caller is fully trusted in that mode and only the
// hasTraversalSegment check above protects against rogue labels.
func assertWithinRoot(root, path string) error {
	if root == "" {
		return nil
	}
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return fmt.Errorf("resolve root path: %w", err)
	}
	pathAbs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("resolve secret path: %w", err)
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return fmt.Errorf("compute relative path: %w", err)
	}
	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("%w: %s", ErrUnsafePath, path)
	}
	return nil
}

// ---- Cloud source contract --------------------------------------------

// CloudSecretFetcher is the small dependency the AWS / GCP / Azure
// sources require. Apps wire in their preferred SDK by writing a thin
// adapter; tests pass a stub. The Fetcher receives the value AND the
// key (so "aws-secret:db-creds#username" → Fetch("db-creds", "username")).
type CloudSecretFetcher func(ctx context.Context, value, key string) ([]byte, error)

// NewCloudSource turns a CloudSecretFetcher into a Source bound to a
// specific Kind. Caller-friendly so the same shape works for AWS, GCP,
// and Azure without duplicating wiring.
func NewCloudSource(kind Kind, fetch CloudSecretFetcher) Source {
	return &cloudSource{kind: kind, fetch: fetch}
}

type cloudSource struct {
	kind  Kind
	fetch CloudSecretFetcher
}

func (c *cloudSource) Kind() Kind { return c.kind }
func (c *cloudSource) Fetch(ctx context.Context, r Ref) ([]byte, error) {
	if r.Value == "" {
		return nil, ErrEmptyValue
	}
	if c.fetch == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownKind, c.kind)
	}
	return c.fetch(ctx, r.Value, r.Key)
}

// LocalVaultLookup is the in-process lookup the ArgusVault source uses.
// Apps inject one closure that reads the local credential vault.
type LocalVaultLookup func(ctx context.Context, entryID, key string) ([]byte, error)

// NewArgusVaultSource returns a Source backed by a local lookup.
func NewArgusVaultSource(lookup LocalVaultLookup) Source {
	return &argusVaultSource{lookup: lookup}
}

type argusVaultSource struct {
	lookup LocalVaultLookup
}

func (a *argusVaultSource) Kind() Kind { return KindArgusVault }
func (a *argusVaultSource) Fetch(ctx context.Context, r Ref) ([]byte, error) {
	if r.Value == "" {
		return nil, ErrEmptyValue
	}
	if a.lookup == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownKind, KindArgusVault)
	}
	return a.lookup(ctx, r.Value, r.Key)
}
