package broker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// Factory builds a Publisher from a per-kind config block. Each
// adapter registers its Factory in its package init() via Register.
// The load-test engine never imports adapter packages directly — it
// gets a Publisher through New() so future adapters can be added
// without touching the engine.
type Factory func(ctx context.Context, cfg any, logger *slog.Logger) (Publisher, error)

var (
	regMu      sync.RWMutex
	factories  = map[Kind]Factory{}
)

// Register wires a Factory for a Kind. Adapters call this from init().
// Re-registering the same kind panics — that catches duplicate-init
// bugs (two adapters claiming the same Kind) at process startup
// rather than at first-use, when the panic would be confusing.
func Register(kind Kind, f Factory) {
	regMu.Lock()
	defer regMu.Unlock()
	if _, exists := factories[kind]; exists {
		panic(fmt.Sprintf("broker: factory for kind %q already registered", kind))
	}
	factories[kind] = f
}

// New constructs a Publisher for the configured Kind. The returned
// Publisher is NOT yet Connected — callers explicitly call Connect so
// the load-test engine can measure connect-time as a separate metric
// from per-message ack latency.
func New(ctx context.Context, cfg Config, logger *slog.Logger) (Publisher, error) {
	block, err := cfg.Resolve()
	if err != nil {
		return nil, err
	}
	regMu.RLock()
	f, ok := factories[cfg.Kind]
	regMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("broker: no factory registered for kind %q (import the adapter package)", cfg.Kind)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return f(ctx, block, logger)
}

// Registered returns the kinds that currently have a registered
// factory. Useful for tests + frontend feature-flagging.
func Registered() []Kind {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]Kind, 0, len(factories))
	for k := range factories {
		out = append(out, k)
	}
	return out
}

// reset clears the registry. Test-only — package-internal so production
// code can't accidentally use it.
func reset() {
	regMu.Lock()
	defer regMu.Unlock()
	factories = map[Kind]Factory{}
}
