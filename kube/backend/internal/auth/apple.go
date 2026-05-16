package auth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Sign in with Apple deviates from generic OIDC in three places that
// would pollute the standard OIDCManager:
//
//  1. client_secret is a dynamic ES256-signed JWT, not a static string.
//     Built from the .p8 key + team/services/key IDs and rotated < 6mo.
//  2. The callback is HTTP POST (response_mode=form_post), not the GET
//     query-string redirect every other OIDC provider uses.
//  3. The email claim is returned ONLY on the first authorization; on
//     subsequent sign-ins we get just the opaque `sub`. We must persist
//     the (sub → email) mapping ourselves.
//
// Hence this file is a parallel manager rather than another
// ProviderConfig hung off OIDCManager.

const (
	appleIssuerURL = "https://appleid.apple.com"
	appleAuthURL   = "https://appleid.apple.com/auth/authorize"
	appleTokenURL  = "https://appleid.apple.com/auth/token"
	appleJWKsURL   = "https://appleid.apple.com/auth/keys"
	appleAudience  = "https://appleid.apple.com"
	appleSecretTTL = 50 * time.Minute // refresh well before the 6-month upper bound
	appleJWKsTTL   = 24 * time.Hour
	appleScopes    = "name email"
)

// AppleConfig is the operator-supplied identity material for Sign in
// with Apple. Everything except DisplayName is required.
type AppleConfig struct {
	ServicesID    string // Apple "Services ID" registered in the Developer portal — acts as client_id (e.g. "com.argus.signin")
	TeamID        string // 10-char team ID from the Developer portal
	KeyID         string // 10-char key ID matching the .p8 private key
	PrivateKeyPEM string // PEM-encoded ECDSA P-256 private key (.p8 contents)
	DisplayName   string // Optional; defaults to "Apple"
	RedirectURL   string // Where Apple POSTs the callback
}

// AppleManager owns Apple-specific state: the parsed signing key, the
// cached client_secret JWT, the cached JWKs used to verify id_tokens,
// and the in-flight (state → flow) map. We could persist pending flows
// in oauth_pending like the OIDC path; instead we mirror them to that
// table so the existing /auth/oauth/poll endpoint just works.
type AppleManager struct {
	cfg    AppleConfig
	store  *Store
	logger loggerLike
	key    *ecdsa.PrivateKey
	http   *http.Client
	now    func() time.Time // override-able for tests

	secretMu     sync.Mutex
	cachedSecret string
	cachedUntil  time.Time

	jwksMu    sync.Mutex
	jwksByKid map[string]*rsa.PublicKey
	jwksUntil time.Time

	// JWKsURL/TokenURL/AuthURL are pluggable so tests can point the
	// manager at an httptest.Server. Real builds leave them empty and
	// get the canonical Apple endpoints.
	jwksURL, tokenURL, authURL string
}

// NewAppleManager parses the PEM, validates fields, and returns a
// ready-to-use manager. Returns an error if any required field is
// missing or the key won't parse — startup should treat that as
// "Apple sign-in not configured" rather than crashing.
func NewAppleManager(store *Store, logger loggerLike, cfg AppleConfig) (*AppleManager, error) {
	if cfg.ServicesID == "" {
		return nil, errors.New("apple: ServicesID required")
	}
	if cfg.TeamID == "" {
		return nil, errors.New("apple: TeamID required")
	}
	if cfg.KeyID == "" {
		return nil, errors.New("apple: KeyID required")
	}
	if cfg.PrivateKeyPEM == "" {
		return nil, errors.New("apple: PrivateKeyPEM required")
	}
	if cfg.RedirectURL == "" {
		return nil, errors.New("apple: RedirectURL required")
	}
	if cfg.DisplayName == "" {
		cfg.DisplayName = "Apple"
	}
	key, err := parseApplePrivateKey(cfg.PrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("apple: parse private key: %w", err)
	}
	return &AppleManager{
		cfg:       cfg,
		store:     store,
		logger:    logger,
		key:       key,
		http:      &http.Client{Timeout: 10 * time.Second},
		now:       time.Now,
		jwksByKid: map[string]*rsa.PublicKey{},
	}, nil
}

// Configured reports whether the manager has usable creds. Always true
// after construction (NewAppleManager rejects half-configured input),
// but the field is queried via a nil-safe helper for callers that
// store a *AppleManager that may be nil.
func (m *AppleManager) Configured() bool { return m != nil && m.key != nil }

// DisplayName surfaces the operator-chosen label for the login button.
func (m *AppleManager) DisplayName() string {
	if m == nil {
		return "Apple"
	}
	return m.cfg.DisplayName
}

// Start mints state + nonce, persists them in oauth_pending so the
// existing /auth/oauth/poll endpoint can resolve them, and returns the
// URL the user's browser should visit.
//
// response_mode=form_post is the magic bit — without it Apple redirects
// via GET on the *first* sign-in (when there's no `user` payload) and
// POSTs on subsequent ones, which is unusable.
func (m *AppleManager) Start() (authURL, state string, err error) {
	if !m.Configured() {
		return "", "", ErrOAuthDisabled
	}
	state, err = randomToken(24)
	if err != nil {
		return "", "", err
	}
	// We reuse the oauth_pending table to keep the poll API identical
	// to the OIDC path. pkce_verifier is empty — Apple doesn't accept
	// PKCE on the web flow (it's a confidential client).
	if _, err := m.store.db.Exec(`INSERT INTO oauth_pending (state, pkce_verifier, provider, created_at)
		VALUES (?, '', ?, ?)`, state, string(ProviderApple), m.now().Unix()); err != nil {
		return "", "", fmt.Errorf("apple: persist state: %w", err)
	}
	base := m.authURL
	if base == "" {
		base = appleAuthURL
	}
	q := url.Values{}
	q.Set("response_type", "code id_token")
	q.Set("response_mode", "form_post")
	q.Set("client_id", m.cfg.ServicesID)
	q.Set("redirect_uri", m.cfg.RedirectURL)
	q.Set("scope", appleScopes)
	q.Set("state", state)
	return base + "?" + q.Encode(), state, nil
}

// appleUserPayload is the form-encoded JSON blob Apple sends on the
// FIRST authorization only. We capture the display name here so we
// don't have to re-prompt the user later.
type appleUserPayload struct {
	Name struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
	} `json:"name"`
	Email string `json:"email"`
}

// Complete exchanges the code, verifies the id_token, upserts the user,
// and creates a session. userJSON is the optional first-auth "user"
// form field — pass it through verbatim and we'll extract the name.
func (m *AppleManager) Complete(ctx context.Context, state, code, userJSON string) (*Session, error) {
	if !m.Configured() {
		return nil, ErrOAuthDisabled
	}
	// Validate state and guard against replay — same pattern as OIDC.
	row := m.store.db.QueryRow(`SELECT provider, completed_at FROM oauth_pending WHERE state = ?`, state)
	var prov string
	var completedAt int64
	if err := row.Scan(&prov, &completedAt); err != nil {
		return nil, ErrOAuthState
	}
	if prov != string(ProviderApple) || completedAt > 0 {
		return nil, ErrOAuthState
	}

	secret, err := m.clientSecret()
	if err != nil {
		return nil, fmt.Errorf("apple: client_secret: %w", err)
	}
	tokenURL := m.tokenURL
	if tokenURL == "" {
		tokenURL = appleTokenURL
	}
	body := url.Values{}
	body.Set("client_id", m.cfg.ServicesID)
	body.Set("client_secret", secret)
	body.Set("code", code)
	body.Set("grant_type", "authorization_code")
	body.Set("redirect_uri", m.cfg.RedirectURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := m.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("apple: token exchange: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple: token exchange HTTP %d: %s", resp.StatusCode, string(raw))
	}
	var tokResp struct {
		IDToken string `json:"id_token"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(raw, &tokResp); err != nil {
		return nil, fmt.Errorf("apple: parse token response: %w", err)
	}
	if tokResp.Error != "" {
		return nil, fmt.Errorf("apple: %s", tokResp.Error)
	}
	if tokResp.IDToken == "" {
		return nil, errors.New("apple: token response missing id_token")
	}

	claims, err := m.verifyIDToken(ctx, tokResp.IDToken)
	if err != nil {
		return nil, err
	}
	if claims.Sub == "" {
		return nil, errors.New("apple: id_token missing sub")
	}

	// Display name comes from the form-posted `user` blob (first auth
	// only). For repeat sign-ins we have nothing, which is fine —
	// UpsertOAuthUser preserves whatever name is on the row.
	name := ""
	if userJSON != "" {
		var u appleUserPayload
		if err := json.Unmarshal([]byte(userJSON), &u); err == nil {
			name = strings.TrimSpace(u.Name.FirstName + " " + u.Name.LastName)
		}
	}
	if name == "" {
		name = claims.Email
	}

	user, err := m.store.UpsertOAuthUser(ProviderApple, claims.Sub, claims.Email, name)
	if err != nil {
		return nil, err
	}
	sess, err := m.store.CreateSession(user.ID)
	if err != nil {
		return nil, err
	}
	// Same poll-resolution dance as the OIDC path.
	if _, err := m.store.db.Exec(`UPDATE oauth_pending SET session_token = ?, completed_at = ? WHERE state = ?`,
		sess.Token, m.now().Unix(), state); err != nil {
		return nil, fmt.Errorf("apple: mark pending: %w", err)
	}
	return sess, nil
}

// clientSecret returns the cached JWT or mints a fresh one. We hold
// the mutex only across the cache check / store — the actual signing
// happens on a copy of the inputs so concurrent first-time callers
// don't serialize behind the ECDSA op.
func (m *AppleManager) clientSecret() (string, error) {
	m.secretMu.Lock()
	if m.cachedSecret != "" && m.now().Before(m.cachedUntil) {
		s := m.cachedSecret
		m.secretMu.Unlock()
		return s, nil
	}
	m.secretMu.Unlock()

	now := m.now()
	header := map[string]string{
		"alg": "ES256",
		"kid": m.cfg.KeyID,
		"typ": "JWT",
	}
	claims := map[string]any{
		"iss": m.cfg.TeamID,
		"iat": now.Unix(),
		"exp": now.Add(appleSecretTTL + 5*time.Minute).Unix(), // small slack > cache TTL
		"aud": appleAudience,
		"sub": m.cfg.ServicesID,
	}
	tok, err := signES256(header, claims, m.key)
	if err != nil {
		return "", err
	}
	m.secretMu.Lock()
	m.cachedSecret = tok
	m.cachedUntil = now.Add(appleSecretTTL)
	m.secretMu.Unlock()
	return tok, nil
}

// idTokenClaims captures the fields we consume. Extra claims are
// silently dropped; we don't need them.
type idTokenClaims struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"` // Apple sends "true"/"false" strings sometimes
	Iss           string `json:"iss"`
	Aud           string `json:"aud"`
	Exp           int64  `json:"exp"`
	Iat           int64  `json:"iat"`
}

// verifyIDToken parses the JWS, looks up the Apple JWK by kid, verifies
// the RS256 signature, and returns the parsed claims. We also do the
// standard iss/aud/exp checks — without them a swapped-in token for a
// different Apple client would still verify cryptographically.
func (m *AppleManager) verifyIDToken(ctx context.Context, raw string) (*idTokenClaims, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, errors.New("apple: id_token not a JWS")
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("apple: decode header: %w", err)
	}
	var hdr struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerJSON, &hdr); err != nil {
		return nil, fmt.Errorf("apple: parse header: %w", err)
	}
	if hdr.Alg != "RS256" {
		return nil, fmt.Errorf("apple: unsupported alg %q", hdr.Alg)
	}
	key, err := m.lookupJWK(ctx, hdr.Kid)
	if err != nil {
		return nil, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("apple: decode signature: %w", err)
	}
	signed := parts[0] + "." + parts[1]
	sum := sha256.Sum256([]byte(signed))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig); err != nil {
		return nil, fmt.Errorf("apple: signature invalid: %w", err)
	}
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("apple: decode claims: %w", err)
	}
	var c idTokenClaims
	if err := json.Unmarshal(claimsJSON, &c); err != nil {
		return nil, fmt.Errorf("apple: parse claims: %w", err)
	}
	if c.Iss != appleIssuerURL {
		return nil, fmt.Errorf("apple: unexpected iss %q", c.Iss)
	}
	if c.Aud != m.cfg.ServicesID {
		return nil, fmt.Errorf("apple: aud %q does not match ServicesID", c.Aud)
	}
	if c.Exp > 0 && m.now().Unix() > c.Exp {
		return nil, errors.New("apple: id_token expired")
	}
	return &c, nil
}

// lookupJWK returns the RSA public key for a kid, fetching + caching
// the JWK set on a miss. We refresh on miss even within the TTL because
// Apple can rotate keys at any time.
func (m *AppleManager) lookupJWK(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	m.jwksMu.Lock()
	if m.now().Before(m.jwksUntil) {
		if k, ok := m.jwksByKid[kid]; ok {
			m.jwksMu.Unlock()
			return k, nil
		}
	}
	m.jwksMu.Unlock()

	jurl := m.jwksURL
	if jurl == "" {
		jurl = appleJWKsURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jurl, nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("apple: fetch JWKs: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple: JWKs HTTP %d", resp.StatusCode)
	}
	var set struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return nil, fmt.Errorf("apple: parse JWKs: %w", err)
	}
	fresh := map[string]*rsa.PublicKey{}
	for _, k := range set.Keys {
		if k.Kty != "RSA" {
			continue
		}
		nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}
		// e is a big-endian unsigned integer. Most JWKs encode it as
		// 3 bytes ("AQAB"); pad to a 4-byte read for binary.BigEndian.
		var eBuf [4]byte
		copy(eBuf[4-len(eBytes):], eBytes)
		e := int(binary.BigEndian.Uint32(eBuf[:]))
		fresh[k.Kid] = &rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: e,
		}
	}
	m.jwksMu.Lock()
	m.jwksByKid = fresh
	m.jwksUntil = m.now().Add(appleJWKsTTL)
	m.jwksMu.Unlock()
	if k, ok := fresh[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("apple: no JWK with kid %q", kid)
}

// parseApplePrivateKey accepts the PEM contents of a .p8 file from the
// Apple Developer portal. The file is PKCS8-encoded with a "PRIVATE KEY"
// label; we parse and assert it's an ECDSA key.
func parseApplePrivateKey(pemStr string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("no PEM block")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	ec, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("not an ECDSA key")
	}
	if ec.Curve != elliptic.P256() {
		return nil, errors.New("key must be on P-256 curve")
	}
	return ec, nil
}

// signES256 produces a compact JWS: base64url(header).base64url(claims).base64url(r||s).
// Hand-rolled because pulling github.com/golang-jwt/jwt for one fixed
// signing scheme adds a non-trivial dependency.
func signES256(header map[string]string, claims map[string]any, key *ecdsa.PrivateKey) (string, error) {
	hb, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	cb, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signing := base64.RawURLEncoding.EncodeToString(hb) + "." + base64.RawURLEncoding.EncodeToString(cb)
	sum := sha256.Sum256([]byte(signing))
	r, s, err := ecdsa.Sign(randReader(), key, sum[:])
	if err != nil {
		return "", err
	}
	// JWS-style ECDSA encoding is r||s padded to the curve byte length,
	// NOT the ASN.1 wrapping ecdsa.Sign returns indirectly. P-256 →
	// 32-byte halves.
	const half = 32
	sig := make([]byte, 2*half)
	rb := r.Bytes()
	sb := s.Bytes()
	copy(sig[half-len(rb):half], rb)
	copy(sig[2*half-len(sb):], sb)
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// randReader is a tiny indirection so tests can swap in a deterministic
// reader if a non-flaky golden test is ever needed. Default is crypto/rand.
var randReader = func() io.Reader { return cryptorand.Reader }
