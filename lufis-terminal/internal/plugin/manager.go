package plugin

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/argus/terminal/plugin/api"
)

type Manager struct {
	mu      sync.RWMutex
	plugins map[string]api.Plugin
	logger  *slog.Logger
	host    *hostImpl
}

type hostImpl struct {
	logger *slog.Logger
	cmds   map[string]api.Command
	panels map[string]api.Panel
	hooks  map[api.HookID][]func(interface{}) error
}

func NewManager(logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	h := &hostImpl{
		logger: logger,
		cmds:   make(map[string]api.Command),
		panels: make(map[string]api.Panel),
		hooks:  make(map[api.HookID][]func(interface{}) error),
	}
	return &Manager{
		plugins: make(map[string]api.Plugin),
		logger:  logger,
		host:    h,
	}
}

func (m *Manager) Register(p api.Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := p.Name()
	if _, exists := m.plugins[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}

	if err := p.Init(m.host); err != nil {
		return fmt.Errorf("init %s: %w", name, err)
	}

	m.plugins[name] = p
	m.logger.Info("plugin registered", "name", name, "version", p.Version())
	return nil
}

func (m *Manager) Unload(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %s not found", name)
	}

	if err := p.Shutdown(); err != nil {
		m.logger.Error("plugin shutdown", "name", name, "error", err)
	}
	delete(m.plugins, name)
	m.logger.Info("plugin unloaded", "name", name)
	return nil
}

func (m *Manager) Plugins() []api.Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]api.Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		list = append(list, p)
	}
	return list
}

func (m *Manager) Commands() []api.Command {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cmds := make([]api.Command, 0, len(m.host.cmds))
	for _, c := range m.host.cmds {
		cmds = append(cmds, c)
	}
	return cmds
}

func (m *Manager) Panels() []api.Panel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	panels := make([]api.Panel, 0, len(m.host.panels))
	for _, p := range m.host.panels {
		panels = append(panels, p)
	}
	return panels
}

func (m *Manager) FireHook(id api.HookID, args interface{}) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if hooks, ok := m.host.hooks[id]; ok {
		for _, fn := range hooks {
			if err := fn(args); err != nil {
				m.logger.Error("hook failed", "hook", id, "error", err)
			}
		}
	}
}

func (h *hostImpl) Logger() api.Logger { return h.logger }

func (h *hostImpl) RegisterCommand(name string, fn func(args []string) error) {
	h.cmds[name] = api.Command{Name: name, Execute: fn}
}

func (h *hostImpl) RegisterSidebarPanel(name string, panel api.Panel) {
	h.panels[name] = panel
}

func (h *hostImpl) RegisterHook(id api.HookID, fn func(args interface{}) error) {
	h.hooks[id] = append(h.hooks[id], fn)
}
