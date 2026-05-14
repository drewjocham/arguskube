package dbconfig

import (
	"context"
	"testing"

	"github.com/argues/argus/internal/secretstore"
)

func TestCrypto_RoundTrip(t *testing.T) {
	c := NewCrypto(secretstore.NewMemoryStore())
	ctx := context.Background()

	enc, err := c.Encrypt(ctx, "hunter2")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if enc == "hunter2" || enc == "" {
		t.Fatalf("ciphertext not encrypted: %q", enc)
	}
	got, err := c.Decrypt(ctx, enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != "hunter2" {
		t.Fatalf("decrypt got %q want hunter2", got)
	}
}

func TestCrypto_EmptyPassthrough(t *testing.T) {
	c := NewCrypto(secretstore.NewMemoryStore())
	ctx := context.Background()
	if enc, err := c.Encrypt(ctx, ""); err != nil || enc != "" {
		t.Fatalf("empty encrypt: %q %v", enc, err)
	}
	if dec, err := c.Decrypt(ctx, ""); err != nil || dec != "" {
		t.Fatalf("empty decrypt: %q %v", dec, err)
	}
}

func TestCrypto_NonceVariesPerCall(t *testing.T) {
	c := NewCrypto(secretstore.NewMemoryStore())
	ctx := context.Background()
	a, _ := c.Encrypt(ctx, "same")
	b, _ := c.Encrypt(ctx, "same")
	if a == b {
		t.Fatalf("nonce reused: identical ciphertext for same plaintext")
	}
}

func TestCrypto_TamperedCiphertextFails(t *testing.T) {
	c := NewCrypto(secretstore.NewMemoryStore())
	ctx := context.Background()
	enc, _ := c.Encrypt(ctx, "secret")
	// Flip a byte at the end (covers the GCM tag).
	tampered := enc[:len(enc)-2] + "AA"
	if _, err := c.Decrypt(ctx, tampered); err == nil {
		t.Fatalf("decrypt of tampered ciphertext should fail")
	}
}

func TestCrypto_MasterKeyPersists(t *testing.T) {
	secrets := secretstore.NewMemoryStore()
	ctx := context.Background()
	enc, err := NewCrypto(secrets).Encrypt(ctx, "abc")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	// A fresh Crypto using the same secretstore must decrypt cleanly,
	// proving the key survives a restart so existing rows stay readable.
	got, err := NewCrypto(secrets).Decrypt(ctx, enc)
	if err != nil || got != "abc" {
		t.Fatalf("decrypt after restart: %q %v", got, err)
	}
}
