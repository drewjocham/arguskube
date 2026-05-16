package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type Vault struct {
	mu   sync.Mutex
	path string
	gcm  cipher.AEAD
}

func NewVault(dataDir string) (*Vault, error) {
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".config", "argus-terminal")
	}
	_ = os.MkdirAll(dataDir, 0o700)

	keyPath := filepath.Join(dataDir, ".vault-key")
	key, err := loadOrCreateKey(keyPath)
	if err != nil {
		return nil, fmt.Errorf("key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}

	return &Vault{
		path: filepath.Join(dataDir, "vault.enc"),
		gcm:  gcm,
	}, nil
}

func (v *Vault) Encrypt(plaintext []byte) (string, error) {
	nonce := make([]byte, v.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	ciphertext := v.gcm.Seal(nil, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

func (v *Vault) Decrypt(encoded string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	nonceSize := v.gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("too short")
	}
	return v.gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
}

func (v *Vault) Store(key string, value []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	enc, err := v.Encrypt(value)
	if err != nil {
		return err
	}
	store := make(map[string]string)
	if data, err := os.ReadFile(v.path); err == nil {
		_ = json.Unmarshal(data, &store)
	}
	store[key] = enc
	data, _ := json.Marshal(store)
	return os.WriteFile(v.path, data, 0o600)
}

func (v *Vault) Retrieve(key string) ([]byte, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	store := make(map[string]string)
	data, err := os.ReadFile(v.path)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	enc, ok := store[key]
	if !ok {
		return nil, fmt.Errorf("key %s not found", key)
	}
	return v.Decrypt(enc)
}

func loadOrCreateKey(path string) ([]byte, error) {
	if data, err := os.ReadFile(path); err == nil {
		return base64.StdEncoding.DecodeString(string(data))
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}
	enc := base64.StdEncoding.EncodeToString(key)
	if err := os.WriteFile(path, []byte(enc), 0o600); err != nil {
		return nil, fmt.Errorf("write key: %w", err)
	}
	return key, nil
}
