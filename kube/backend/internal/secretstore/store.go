// Package secretstore persists small per-user secrets (the Argus
// session token, future device JWTs, OAuth refresh tokens) in OS-native
// secure storage. It's a thin abstraction over the platform Keychain
// equivalents so the rest of the app doesn't care whether it's running
// on macOS (Keychain), Linux (libsecret/inmemory today), Windows (DPAPI
// future), SaaS (in-memory), or a test (in-memory).
//
// Design constraints:
//
//   - No CGO. macOS uses the bundled `security` command; Linux falls
//     back to in-memory for now (we ship desktop Argus on macOS first;
//     when the Linux build arrives a libsecret D-Bus impl slots in).
//   - Key prefix is enforced by the Wails binding layer, NOT here.
//     This package treats every key as opaque so unit tests and
//     non-Wails callers can use it freely.
//   - Get on a missing key returns ("", false, nil) — not an error.
//     Callers commonly want "do we have a session?" semantics, not "is
//     the Keychain working?", so the boolean is the answer for the
//     common case and err is reserved for backend failure.
package secretstore

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

// Store is the small surface the rest of the app codes against. All
// methods are safe for concurrent use; implementations must serialize
// internally if their backend isn't already.
type Store interface {
	Set(ctx context.Context, key, value string) error
	Get(ctx context.Context, key string) (string, bool, error)
	Delete(ctx context.Context, key string) error
	// Backend identifies the implementation for logs + the settings UI
	// ("Session stored in macOS Keychain", "in-memory only", …).
	Backend() string
}

// New returns the best-available Store for the current platform. The
// caller passes a service name — the macOS Keychain entry is keyed by
// (service, account); we use service="Argus" and key as the account.
func New(service string) Store {
	if service == "" {
		service = "Argus"
	}
	if runtime.GOOS == "darwin" {
		if ok := keychainAvailable(); ok {
			return &macKeychain{service: service}
		}
	}
	return NewMemoryStore()
}

// ErrNotSupported is returned by an in-memory Store from operations that
// would be silently broken (clears across process restart, for example)
// when callers explicitly ask "is this persistent?". Reserved — today
// no impl returns it directly. Kept for API stability.
var ErrNotSupported = errors.New("operation not supported by this secret store")

// --------- macOS Keychain implementation -----------------------------

type macKeychain struct {
	service string
	mu      sync.Mutex
}

func (m *macKeychain) Backend() string { return "macOS Keychain" }

// Set writes (or updates) an entry in the user's login Keychain. We
// shell out to `security` because the only alternative is CGO via
// Security.framework, which would break the single-binary, no-CGO build
// rule from a prior ADR. The Keychain dialog only appears the *first*
// time an app accesses an entry — subsequent accesses are silent.
func (m *macKeychain) Set(ctx context.Context, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// `security add-generic-password -U` updates if present, creates if
	// not. -a account, -s service, -w password value, -U upsert.
	cmd := exec.CommandContext(ctx, "security", "add-generic-password",
		"-U",
		"-a", key,
		"-s", m.service,
		"-w", value,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("security add-generic-password: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (m *macKeychain) Get(ctx context.Context, key string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cmd := exec.CommandContext(ctx, "security", "find-generic-password",
		"-a", key,
		"-s", m.service,
		"-w", // print the password to stdout (and only that)
	)
	out, err := cmd.Output()
	if err != nil {
		// `security` exits 44 ("specified item could not be found in the
		// keychain") for a clean miss; we treat that as not-present, not
		// as an error.
		if exitErr := new(exec.ExitError); errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 44 {
				return "", false, nil
			}
		}
		return "", false, fmt.Errorf("security find-generic-password: %w", err)
	}
	return strings.TrimRight(string(out), "\n"), true, nil
}

func (m *macKeychain) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cmd := exec.CommandContext(ctx, "security", "delete-generic-password",
		"-a", key,
		"-s", m.service,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Missing → success, idempotent delete.
		if exitErr := new(exec.ExitError); errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 44 {
				return nil
			}
		}
		return fmt.Errorf("security delete-generic-password: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func keychainAvailable() bool {
	// `security -h` exits 0 if the binary is installed (every macOS has
	// it under /usr/bin/security but Wails dev on a fresh machine may
	// have a PATH oddity; we check before declaring keychain available).
	_, err := exec.LookPath("security")
	return err == nil
}

// --------- in-memory fallback ----------------------------------------

// MemoryStore is the fallback for non-macOS platforms, SaaS mode, and
// every unit test that doesn't want to write to the user's keychain.
// Exported so tests can construct one without going through New().
type MemoryStore struct {
	mu sync.RWMutex
	m  map[string]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{m: map[string]string{}}
}

func (m *MemoryStore) Backend() string { return "in-memory only" }

func (m *MemoryStore) Set(_ context.Context, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m[key] = value
	return nil
}

func (m *MemoryStore) Get(_ context.Context, key string) (string, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.m[key]
	return v, ok, nil
}

func (m *MemoryStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.m, key)
	return nil
}
