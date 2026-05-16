// Package pubsub is a placeholder palette for the GCP Pub/Sub shell.
// The follow-up PR wires List() to `gcloud pubsub topics list` etc.
// — for now the tabs land so the render layer shows the strip and
// the shell type registers in the lufis registry alongside k8s.
package pubsub

import (
	"context"

	"github.com/argus/terminal/internal/palette"
)

const (
	shellName        = "pubsub"
	tabTopics        = "topics"
	tabSubscriptions = "subscriptions"
	tabSchemas       = "schemas"
	tabSnapshots     = "snapshots"
)

type Palette struct{}

func New() *Palette { return &Palette{} }

func (p *Palette) Name() string { return shellName }

func (p *Palette) Tabs() []palette.Tab {
	return []palette.Tab{
		{ID: tabTopics, Label: "Topics"},
		{ID: tabSubscriptions, Label: "Subscriptions"},
		{ID: tabSchemas, Label: "Schemas"},
		{ID: tabSnapshots, Label: "Snapshots"},
	}
}

func (p *Palette) List(_ context.Context, tabID string) ([]palette.Group, error) {
	if _, ok := palette.FindTab(p.Tabs(), tabID); !ok {
		return nil, palette.ErrUnknownTab
	}
	return nil, nil
}

func (p *Palette) Command(_ string, _ palette.ActionID, _ palette.Resource) (string, error) {
	return "", palette.ErrUnsupportedAction
}
