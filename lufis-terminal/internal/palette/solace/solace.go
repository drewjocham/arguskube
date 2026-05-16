// Package solace is a placeholder palette for Solace Event Broker
// shells. The next PR wires Tabs() → SEMP queries via the management
// plane; for now we ship the tab metadata so the render layer can
// already show the "solace" tab in the lufis shell strip and the
// command palette below it.
package solace

import (
	"context"

	"github.com/argus/terminal/internal/palette"
)

const (
	shellName     = "solace"
	tabQueues     = "queues"
	tabTopics     = "topics"
	tabClients    = "clients"
	tabMessageVPN = "message-vpns"
)

// Palette is the placeholder Solace palette. It returns its tabs and
// metadata but has no resource enumeration wired up yet (List returns
// an empty Group slice). Stash a SEMP client here in the follow-up PR.
type Palette struct{}

func New() *Palette { return &Palette{} }

func (p *Palette) Name() string { return shellName }

func (p *Palette) Tabs() []palette.Tab {
	return []palette.Tab{
		{ID: tabQueues, Label: "Queues"},
		{ID: tabTopics, Label: "Topics"},
		{ID: tabClients, Label: "Clients"},
		{ID: tabMessageVPN, Label: "Message VPNs"},
	}
}

// List is a no-op until the SEMP integration lands. Returning an
// empty slice is intentional: the render layer treats it as "no
// resources to show" and renders an "(not yet wired)" hint.
func (p *Palette) List(_ context.Context, tabID string) ([]palette.Group, error) {
	if _, ok := palette.FindTab(p.Tabs(), tabID); !ok {
		return nil, palette.ErrUnknownTab
	}
	return nil, nil
}

// Command is also a stub. Returning ErrUnsupportedAction tells the
// render layer to hide the Copy / Run buttons until the next PR.
func (p *Palette) Command(_ string, _ palette.ActionID, _ palette.Resource) (string, error) {
	return "", palette.ErrUnsupportedAction
}
