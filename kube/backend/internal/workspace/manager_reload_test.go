package workspace

import (
	"log/slog"
	"testing"

	"github.com/argues/argus/internal/secretstore"
)

// TestReregisterProviders_SwapsProviderSet verifies that re-registering
// replaces the provider set wholesale: services in the new list become
// reachable, services not in the new list disappear. This is the
// hot-reload contract the Settings UI relies on.
func TestReregisterProviders_SwapsProviderSet(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(testDiscard{}, nil))
	store := NewStore(openTestDB(t), NewCrypto(secretstore.NewMemoryStore()))
	m := NewManager(store, logger)

	// Start with Slack only.
	m.ReregisterProviders([]Provider{NewTestProvider(ServiceSlack)})
	if !m.HasProvider(ServiceSlack) {
		t.Fatal("expected Slack provider after first reregister")
	}
	if m.HasProvider(ServiceGoogle) {
		t.Fatal("expected Google to be absent in slack-only set")
	}

	// Swap to Google only — Slack should drop, Google should appear.
	m.ReregisterProviders([]Provider{NewTestProvider(ServiceGoogle)})
	if m.HasProvider(ServiceSlack) {
		t.Error("expected Slack to be dropped after swap")
	}
	if !m.HasProvider(ServiceGoogle) {
		t.Error("expected Google after swap")
	}

	// Empty swap clears everything — represents "user cleared all creds".
	m.ReregisterProviders(nil)
	if len(m.AvailableServices()) != 0 {
		t.Errorf("expected empty provider set, got %v", m.AvailableServices())
	}
}

// TestReregisterProviders_DoesNotPanicOnRepeatedRegister guards against a
// regression where Register would panic on a duplicate service, but the
// hot-reload path must accept the same service being re-registered with
// new creds.
func TestReregisterProviders_AllowsRepeatedSameService(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(testDiscard{}, nil))
	store := NewStore(openTestDB(t), NewCrypto(secretstore.NewMemoryStore()))
	m := NewManager(store, logger)

	m.ReregisterProviders([]Provider{NewTestProvider(ServiceSlack)})
	m.ReregisterProviders([]Provider{NewTestProvider(ServiceSlack)}) // would panic via Register()
	if !m.HasProvider(ServiceSlack) {
		t.Error("expected Slack present after repeated reregister")
	}
}
