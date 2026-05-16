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
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// testECKeyPEM produces a fresh P-256 PKCS8 PEM blob, mirroring the
// shape of a real Apple .p8 file.
func testECKeyPEM(t *testing.T) (string, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), cryptorand.Reader)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	out := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	return string(out), key
}

func newTestAppleManager(t *testing.T) (*AppleManager, *ecdsa.PrivateKey) {
	t.Helper()
	pemStr, key := testECKeyPEM(t)
	m, err := NewAppleManager(newTestStore(t), discardLogger{}, AppleConfig{
		ServicesID:    "com.example.signin",
		TeamID:        "TEAM000000",
		KeyID:         "KEY0000000",
		PrivateKeyPEM: pemStr,
		RedirectURL:   "https://example.test/auth/apple/callback",
	})
	if err != nil {
		t.Fatalf("NewAppleManager: %v", err)
	}
	return m, key
}

type discardLogger struct{}

func (discardLogger) Warn(string, ...any)  {}
func (discardLogger) Info(string, ...any)  {}
func (discardLogger) Error(string, ...any) {}

func TestApple_NewAppleManager_RejectsBadConfig(t *testing.T) {
	cases := map[string]AppleConfig{
		"missing services": {TeamID: "T", KeyID: "K", PrivateKeyPEM: "x", RedirectURL: "r"},
		"missing team":     {ServicesID: "S", KeyID: "K", PrivateKeyPEM: "x", RedirectURL: "r"},
		"bad PEM":          {ServicesID: "S", TeamID: "T", KeyID: "K", PrivateKeyPEM: "not pem", RedirectURL: "r"},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewAppleManager(newTestStore(t), discardLogger{}, cfg); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestApple_ClientSecret_CachedAndRotated(t *testing.T) {
	m, _ := newTestAppleManager(t)
	base := time.Now()
	m.now = func() time.Time { return base }

	a, err := m.clientSecret()
	if err != nil {
		t.Fatalf("clientSecret: %v", err)
	}
	b, _ := m.clientSecret()
	if a != b {
		t.Errorf("expected cache hit; got two different secrets")
	}

	// Verify header has alg=ES256 + kid + the right claims.
	parts := strings.Split(a, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWS parts, got %d", len(parts))
	}
	hb, _ := base64.RawURLEncoding.DecodeString(parts[0])
	var hdr map[string]string
	_ = json.Unmarshal(hb, &hdr)
	if hdr["alg"] != "ES256" || hdr["kid"] != "KEY0000000" || hdr["typ"] != "JWT" {
		t.Errorf("header wrong: %v", hdr)
	}
	cb, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims map[string]any
	_ = json.Unmarshal(cb, &claims)
	if claims["iss"] != "TEAM000000" || claims["sub"] != "com.example.signin" || claims["aud"] != "https://appleid.apple.com" {
		t.Errorf("claims wrong: %v", claims)
	}

	// Advance past the cache TTL → second call should mint a new one.
	m.now = func() time.Time { return base.Add(60 * time.Minute) }
	c, _ := m.clientSecret()
	if c == a {
		t.Errorf("expected fresh secret after TTL")
	}
}

// jwksMint signs a fake Apple id_token with a test RSA key and serves
// the matching JWKs via an httptest.Server.
type jwksRig struct {
	key    *rsa.PrivateKey
	kid    string
	server *httptest.Server
}

func newJWKsRig(t *testing.T) *jwksRig {
	t.Helper()
	rk, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa gen: %v", err)
	}
	r := &jwksRig{key: rk, kid: "test-kid"}
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nBytes := r.key.N.Bytes()
		eBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(eBuf, uint32(r.key.E))
		// Trim leading zeros from e for canonical JWK form.
		for len(eBuf) > 1 && eBuf[0] == 0 {
			eBuf = eBuf[1:]
		}
		body := fmt.Sprintf(`{"keys":[{"kid":%q,"kty":"RSA","n":%q,"e":%q}]}`,
			r.kid,
			base64.RawURLEncoding.EncodeToString(nBytes),
			base64.RawURLEncoding.EncodeToString(eBuf),
		)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(r.server.Close)
	return r
}

func (r *jwksRig) signIDToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	hdr := map[string]string{"alg": "RS256", "kid": r.kid, "typ": "JWT"}
	hb, _ := json.Marshal(hdr)
	cb, _ := json.Marshal(claims)
	signing := base64.RawURLEncoding.EncodeToString(hb) + "." + base64.RawURLEncoding.EncodeToString(cb)
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(cryptorand.Reader, r.key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestApple_VerifyIDToken_HappyPathAndTamper(t *testing.T) {
	m, _ := newTestAppleManager(t)
	rig := newJWKsRig(t)
	m.jwksURL = rig.server.URL
	m.now = func() time.Time { return time.Unix(1_700_000_000, 0) }

	tok := rig.signIDToken(t, map[string]any{
		"iss":   appleIssuerURL,
		"aud":   m.cfg.ServicesID,
		"sub":   "001234.deadbeef.0000",
		"email": "user@privaterelay.appleid.com",
		"exp":   m.now().Add(5 * time.Minute).Unix(),
		"iat":   m.now().Unix(),
	})
	claims, err := m.verifyIDToken(context.Background(), tok)
	if err != nil {
		t.Fatalf("verifyIDToken: %v", err)
	}
	if claims.Sub != "001234.deadbeef.0000" {
		t.Errorf("sub = %q", claims.Sub)
	}

	// Tamper a single byte of the signature → must fail.
	parts := strings.Split(tok, ".")
	sigBytes, _ := base64.RawURLEncoding.DecodeString(parts[2])
	sigBytes[0] ^= 0xFF
	bad := parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(sigBytes)
	if _, err := m.verifyIDToken(context.Background(), bad); err == nil {
		t.Error("expected tampered token to fail verification")
	}
}

func TestApple_StartComplete_RoundTripAndEmailPersistence(t *testing.T) {
	m, _ := newTestAppleManager(t)
	rig := newJWKsRig(t)
	m.jwksURL = rig.server.URL
	m.now = func() time.Time { return time.Unix(1_700_000_000, 0) }

	// Mock token endpoint — returns a signed id_token.
	var firstSeen atomic.Bool
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostForm.Get("client_id") != m.cfg.ServicesID {
			t.Errorf("bad client_id in token request")
		}
		if r.PostForm.Get("client_secret") == "" {
			t.Errorf("missing client_secret JWT")
		}
		claims := map[string]any{
			"iss": appleIssuerURL,
			"aud": m.cfg.ServicesID,
			"sub": "001234.test.0000",
			"exp": m.now().Add(5 * time.Minute).Unix(),
			"iat": m.now().Unix(),
		}
		if !firstSeen.Load() {
			// First auth: include email like Apple does.
			claims["email"] = "first@example.com"
			firstSeen.Store(true)
		}
		body := map[string]string{"id_token": rig.signIDToken(t, claims)}
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(tokenSrv.Close)
	m.tokenURL = tokenSrv.URL

	// First sign-in → carries email + name.
	_, state, err := m.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	userJSON := `{"name":{"firstName":"Ada","lastName":"Lovelace"},"email":"first@example.com"}`
	sess, err := m.Complete(context.Background(), state, "fakecode", userJSON)
	if err != nil {
		t.Fatalf("Complete (first): %v", err)
	}
	if sess == nil || sess.Token == "" {
		t.Fatal("first Complete returned no session")
	}
	// Verify the user row has email + name.
	row := m.store.db.QueryRow(`SELECT email, name FROM users WHERE provider = ? AND provider_subject = ?`,
		string(ProviderApple), "001234.test.0000")
	var gotEmail, gotName string
	if err := row.Scan(&gotEmail, &gotName); err != nil {
		t.Fatalf("read user: %v", err)
	}
	if gotEmail != "first@example.com" || gotName != "Ada Lovelace" {
		t.Errorf("first auth user row: email=%q name=%q", gotEmail, gotName)
	}

	// Second sign-in → no email, no user blob (the typical case).
	_, state2, err := m.Start()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Complete(context.Background(), state2, "fakecode2", ""); err != nil {
		t.Fatalf("Complete (second): %v", err)
	}
	// Email must still be present from the first auth — this is the
	// whole point of the (sub → email) persistence requirement.
	row = m.store.db.QueryRow(`SELECT email FROM users WHERE provider = ? AND provider_subject = ?`,
		string(ProviderApple), "001234.test.0000")
	if err := row.Scan(&gotEmail); err != nil {
		t.Fatal(err)
	}
	if gotEmail != "first@example.com" {
		t.Errorf("email lost on second auth: %q", gotEmail)
	}
}

func TestApple_VerifyIDToken_RejectsWrongAudOrIssuer(t *testing.T) {
	m, _ := newTestAppleManager(t)
	rig := newJWKsRig(t)
	m.jwksURL = rig.server.URL
	m.now = func() time.Time { return time.Unix(1_700_000_000, 0) }

	// Wrong aud.
	tok := rig.signIDToken(t, map[string]any{
		"iss": appleIssuerURL,
		"aud": "com.someone.else",
		"sub": "x",
		"exp": m.now().Add(time.Minute).Unix(),
	})
	if _, err := m.verifyIDToken(context.Background(), tok); err == nil {
		t.Error("expected aud mismatch to fail")
	}

	// Wrong iss.
	tok = rig.signIDToken(t, map[string]any{
		"iss": "https://evil.example",
		"aud": m.cfg.ServicesID,
		"sub": "x",
		"exp": m.now().Add(time.Minute).Unix(),
	})
	if _, err := m.verifyIDToken(context.Background(), tok); err == nil {
		t.Error("expected iss mismatch to fail")
	}
}

// silence the unused big.Int / rsa imports when the file changes.
var _ = big.NewInt
