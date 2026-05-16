// Package rabbitmq is a placeholder palette for the RabbitMQ shell.
// The follow-up PR wires List() to the RabbitMQ management HTTP API.
package rabbitmq

import (
	"context"

	"github.com/argus/terminal/internal/palette"
)

const (
	shellName    = "rabbitmq"
	tabExchanges = "exchanges"
	tabQueues    = "queues"
	tabBindings  = "bindings"
	tabUsers     = "users"
)

type Palette struct{}

func New() *Palette { return &Palette{} }

func (p *Palette) Name() string { return shellName }

func (p *Palette) Tabs() []palette.Tab {
	return []palette.Tab{
		{ID: tabExchanges, Label: "Exchanges"},
		{ID: tabQueues, Label: "Queues"},
		{ID: tabBindings, Label: "Bindings"},
		{ID: tabUsers, Label: "Users"},
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
