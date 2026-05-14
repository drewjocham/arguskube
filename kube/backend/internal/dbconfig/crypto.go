package dbconfig

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"

	"github.com/argues/argus/internal/secretstore"
)

// masterKeyKey is the secretstore account name under which we keep the
// AES-256 master key. It's namespaced so a future "dbconfig.signing_key"
// can sit next to it without collision.
const masterKeyKey = "dbconfig.master_key"

// Crypto encrypts and decrypts DB credentials. The master key lives in
// secretstore (macOS Keychain on darwin, in-memory elsewhere) and is
// generated lazily on first use. We use AES-256-GCM: authenticated,
// nonce-misuse-resistant enough for our threat model (local user, no
// network), and present in the stdlib so we don't add a dep.
type Crypto struct {
	secrets secretstore.Store
	mu      sync.Mutex
	gcm     cipher.AEAD
}

// NewCrypto wires Crypto onto an existing secretstore. The key is not
// fetched here — Encrypt/Decrypt do that lazily so an Argus run that
// never registers a DB never touches the Keychain.
func NewCrypto(secrets secretstore.Store) *Crypto {
	return &Crypto{secrets: secrets}
}

func (c *Crypto) aead(ctx context.Context) (cipher.AEAD, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.gcm != nil {
		return c.gcm, nil
	}
	key, err := c.loadOrCreateKey(ctx)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("dbconfig: aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("dbconfig: gcm: %w", err)
	}
	c.gcm = gcm
	return gcm, nil
}

// loadOrCreateKey returns the 32-byte master key, generating one on
// first use. The key never leaves this process except as a base64
// string in secretstore.
func (c *Crypto) loadOrCreateKey(ctx context.Context) ([]byte, error) {
	enc, ok, err := c.secrets.Get(ctx, masterKeyKey)
	if err != nil {
		return nil, fmt.Errorf("dbconfig: read master key: %w", err)
	}
	if ok {
		key, err := base64.StdEncoding.DecodeString(enc)
		if err != nil {
			return nil, fmt.Errorf("dbconfig: decode master key: %w", err)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("dbconfig: master key has wrong length (%d)", len(key))
		}
		return key, nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("dbconfig: gen master key: %w", err)
	}
	if err := c.secrets.Set(ctx, masterKeyKey, base64.StdEncoding.EncodeToString(key)); err != nil {
		return nil, fmt.Errorf("dbconfig: persist master key: %w", err)
	}
	return key, nil
}

// Encrypt returns base64(nonce||ciphertext||tag). Empty input returns
// empty output so a connection without a password round-trips cleanly.
func (c *Crypto) Encrypt(ctx context.Context, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	gcm, err := c.aead(ctx)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("dbconfig: nonce: %w", err)
	}
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt reverses Encrypt. An empty input is allowed and yields "".
func (c *Crypto) Decrypt(ctx context.Context, encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	gcm, err := c.aead(ctx)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("dbconfig: decode ciphertext: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("dbconfig: ciphertext too short")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("dbconfig: decrypt: %w", err)
	}
	return string(pt), nil
}
