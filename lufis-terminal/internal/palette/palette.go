// Package palette is the lufis-terminal "command palette" backend:
// the pure-Go logic for the shell-type tabs (k8s / solace / pubsub
// / rabbitmq), their command groups (pods / services / queues / …),
// the resource enumeration each group lists, and the shell-command
// strings the popup buttons produce.
//
// The render layer (GLFW + popup chrome) lives in a separate package;
// it consumes the ShellPalette interface here and stays unaware of
// the specifics of any one shell type. Splitting the two means new
// shells land without touching render code and the heuristics can be
// unit-tested in isolation.
//
// See docs/lufis-command-palette.md for the full design + the spec
// driving the current shape.
package palette

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

// ShellPalette is what the render layer asks for: "what commands does
// this shell offer, and what do their resource lists look like right
// now?" One implementation per shell type (k8s, solace, …).
type ShellPalette interface {
	// Name is the user-facing label of the shell-type tab at the top
	// of the terminal window ("k8s", "solace", …). The render layer
	// uses it for the tab strip; callers can also key the registry
	// by it.
	Name() string

	// Tabs are the command groups within this shell ("pods",
	// "services", …). Stable for the lifetime of the palette — the
	// render layer caches them once per shell instance.
	Tabs() []Tab

	// List enumerates resources for one tab. Implementations apply
	// the collapsing rule themselves so they can choose a sensible
	// min-collapse threshold per data shape (k8s pods collapse
	// aggressively; solace queues less so).
	List(ctx context.Context, tabID string) ([]Group, error)

	// Command turns "user picked this Group, wants to do X" into a
	// shell-executable command. The render layer's Copy / Run /
	// Run-in-new buttons call this and feed the result to the
	// clipboard or PTY. Returns ErrUnsupportedAction when the
	// palette doesn't implement the given action for that tab.
	Command(tab string, action ActionID, picked Resource) (string, error)
}

// Tab is a command group within a shell type.
type Tab struct {
	ID    string // stable id used by Command / List ("pods")
	Label string // user-facing ("Pods")
}

// Resource is one concrete item the user can act on. The same struct
// flows from List → render-layer → click → Command, so the popup
// button handlers don't have to ferry per-shell state around.
type Resource struct {
	Name      string            // canonical name (no wildcard)
	Namespace string            // when applicable
	Display   string            // optional override; defaults to Name
	Meta      map[string]string // shell-specific bag (broker URL, partition #, …)
}

// String returns Display when set, otherwise Name. Useful for the
// render layer that just wants "what to draw on the row."
func (r Resource) String() string {
	if r.Display != "" {
		return r.Display
	}
	return r.Name
}

// Group is one row in the resource list. Members is one entry for an
// un-collapsed row and N for a wildcard row ("nginx-deployment-*").
type Group struct {
	Display string     // "nginx-deployment-*" or "redis-master-0"
	Members []Resource // 1 element for un-collapsed; N for wildcard rows
}

// Collapsed reports whether this Group represents multiple resources.
func (g Group) Collapsed() bool { return len(g.Members) > 1 }

// ActionID names the popup buttons. Each shell can implement a
// subset — Command returns ErrUnsupportedAction for the ones it
// doesn't support. The render layer asks the palette which actions
// apply per Group via the Actions method below.
type ActionID string

const (
	// ActionCopy: copy the rendered command to the clipboard.
	ActionCopy ActionID = "copy"
	// ActionRunCurrent: pipe the command into the currently-active
	// PTY (the main shell).
	ActionRunCurrent ActionID = "run-current"
	// ActionRunNewShell: spawn a fresh shell tab in lufis and run
	// the command there. The hotkey is the render layer's concern.
	ActionRunNewShell ActionID = "run-new-shell"
)

// ErrUnsupportedAction is returned by ShellPalette.Command when the
// (tab, action) combination is not implemented. Render layer treats
// it as "hide this button"; not an error to surface.
var ErrUnsupportedAction = errors.New("palette: action not supported for this tab")

// ErrUnknownTab is returned by List / Command when the tabID
// doesn't match any of the palette's Tabs(). Programmer error.
var ErrUnknownTab = errors.New("palette: unknown tab")

// ─── Registry ─────────────────────────────────────────────────────────

// Registry is the lookup the render layer uses to enumerate available
// shell types and resolve a palette by name. Goroutine-safe; intended
// to be populated at process startup (the lufis main wires every
// compiled-in shell type once).
type Registry struct {
	mu       sync.RWMutex
	palettes map[string]ShellPalette
	order    []string
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{palettes: map[string]ShellPalette{}}
}

// Register adds a palette under its Name(). Panics on duplicate
// names — wiring is supposed to be deterministic and a collision is
// always a build-time mistake worth failing loudly.
func (r *Registry) Register(p ShellPalette) {
	if p == nil {
		panic("palette: Register(nil)")
	}
	name := p.Name()
	if name == "" {
		panic("palette: Register with empty Name()")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.palettes[name]; exists {
		panic(fmt.Sprintf("palette: duplicate Register for %q", name))
	}
	r.palettes[name] = p
	r.order = append(r.order, name)
}

// Get returns the palette registered under name, or (nil, false).
func (r *Registry) Get(name string) (ShellPalette, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.palettes[name]
	return p, ok
}

// Names returns the registered palette names in registration order.
// Render layer uses this for the shell-tab strip at the top.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// Len returns how many palettes are registered. Convenience for tests.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.order)
}

// All returns palettes in registration order. Stable copy.
func (r *Registry) All() []ShellPalette {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ShellPalette, 0, len(r.order))
	for _, n := range r.order {
		out = append(out, r.palettes[n])
	}
	return out
}

// ─── Tab helpers ──────────────────────────────────────────────────────

// FindTab returns the Tab whose ID matches id, or (Tab{}, false).
// Palettes use it to validate tab IDs before doing real work.
func FindTab(tabs []Tab, id string) (Tab, bool) {
	for _, t := range tabs {
		if t.ID == id {
			return t, true
		}
	}
	return Tab{}, false
}

// SortGroupsByDisplay sorts groups alphabetically by Display. The
// render layer wants stable order so the rows don't reshuffle every
// refresh; palettes apply this before returning.
func SortGroupsByDisplay(groups []Group) {
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Display < groups[j].Display
	})
}
