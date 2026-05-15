package workspace

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

// Workspace tokens are encrypted with AES-256-GCM, master key in
// secretstore. Same envelope dbconfig uses for DB passwords. Two
// separate accounts (workspace.master_key vs dbconfig.master_key) keep
// the blast radius of a future key-rotation bug bounded to one feature.

const masterKeyKey = "workspace.master_key"

// Crypto handles per-token encryption. Lazily fetches/creates the
// master key on first use so an Argus run that never connects a
// workspace never touches secretstore.
type Crypto struct {
	secrets secretstore.Store
	mu      sync.Mutex
	gcm     cipher.AEAD
}

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
		return nil, fmt.Errorf("workspace: aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("workspace: gcm: %w", err)
	}
	c.gcm = gcm
	return gcm, nil
}

func (c *Crypto) loadOrCreateKey(ctx context.Context) ([]byte, error) {
	enc, ok, err := c.secrets.Get(ctx, masterKeyKey)
	if err != nil {
		return nil, fmt.Errorf("workspace: read master key: %w", err)
	}
	if ok {
		key, err := base64.StdEncoding.DecodeString(enc)
		if err != nil {
			return nil, fmt.Errorf("workspace: decode master key: %w", err)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("workspace: master key has wrong length (%d)", len(key))
		}
		return key, nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("workspace: gen master key: %w", err)
	}
	if err := c.secrets.Set(ctx, masterKeyKey, base64.StdEncoding.EncodeToString(key)); err != nil {
		return nil, fmt.Errorf("workspace: persist master key: %w", err)
	}
	return key, nil
}

// Encrypt returns base64(nonce||ciphertext||tag). Empty input → empty
// output so a refresh-tokenless OAuth round-trips cleanly.
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
		return "", fmt.Errorf("workspace: nonce: %w", err)
	}
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

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
		return "", fmt.Errorf("workspace: decode ciphertext: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("workspace: ciphertext too short")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("workspace: decrypt: %w", err)
	}
	return string(pt), nil
}
