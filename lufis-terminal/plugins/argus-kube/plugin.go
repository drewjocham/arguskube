package arguskube

import (
	"log/slog"

	"github.com/argus/terminal/plugin/api"
)

type Plugin struct{}

func (p *Plugin) Name() string    { return "argus-kube" }
func (p *Plugin) Version() string { return "0.1.0" }

func (p *Plugin) Init(host api.HostAPI) error {
	log := host.Logger()
	log.Info("argus-kube plugin initializing")

	host.RegisterCommand("cluster-status", func(args []string) error {
		log.Info("cluster status requested")
		return nil
	})

	host.RegisterCommand("pod-list", func(args []string) error {
		log.Info("pod list requested", "args", args)
		return nil
	})

	host.RegisterSidebarPanel("Cluster", api.Panel{
		Title:    "Cluster",
		Priority: 10,
		Content: func() string {
			return "Cluster status: connected\nNamespace: default"
		},
	})

	host.RegisterSidebarPanel("Alerts", api.Panel{
		Title:    "Alerts",
		Priority: 20,
		Content: func() string {
			return "No active alerts"
		},
	})

	host.RegisterHook(api.HookCommandExecuted, func(args interface{}) error {
		log.Debug("command executed hook fired")
		return nil
	})

	return nil
}

func (p *Plugin) Shutdown() error {
	slog.Info("argus-kube plugin shutting down")
	return nil
}
