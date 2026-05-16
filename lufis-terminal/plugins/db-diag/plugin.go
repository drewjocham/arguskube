package dbdiag

import (
	"github.com/argus/terminal/plugin/api"
)

type Plugin struct{}

func (p *Plugin) Name() string    { return "db-diag" }
func (p *Plugin) Version() string { return "0.1.0" }

func (p *Plugin) Init(host api.HostAPI) error {
	log := host.Logger()

	host.RegisterCommand("db-slow-queries", func(args []string) error {
		log.Info("analyzing slow queries")
		return nil
	})

	host.RegisterCommand("db-index-analysis", func(args []string) error {
		log.Info("analyzing indexes")
		return nil
	})

	host.RegisterSidebarPanel("DB Diagnostics", api.Panel{
		Title:    "DB Diagnostics",
		Priority: 30,
		Content: func() string {
			return "Databases:\n  No databases configured"
		},
	})

	return nil
}

func (p *Plugin) Shutdown() error { return nil }
