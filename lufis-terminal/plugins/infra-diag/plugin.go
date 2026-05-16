package infradiag

import (
	"github.com/argus/terminal/plugin/api"
)

type Plugin struct{}

func (p *Plugin) Name() string    { return "infra-diag" }
func (p *Plugin) Version() string { return "0.1.0" }

func (p *Plugin) Init(host api.HostAPI) error {
	log := host.Logger()

	host.RegisterCommand("dns-check", func(args []string) error {
		log.Info("checking DNS resolution")
		return nil
	})

	host.RegisterCommand("tls-check", func(args []string) error {
		log.Info("checking TLS certificate")
		return nil
	})

	host.RegisterCommand("network-latency", func(args []string) error {
		log.Info("measuring network latency")
		return nil
	})

	host.RegisterSidebarPanel("Infra Diagnostics", api.Panel{
		Title:    "Infra Diagnostics",
		Priority: 40,
		Content: func() string {
			return "Infrastructure:\n  DNS: not checked\n  TLS: not checked\n  Network: not checked"
		},
	})

	return nil
}

func (p *Plugin) Shutdown() error { return nil }
