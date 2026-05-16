package automate

import (
	"log/slog"
	"strings"
	"sync"
	"time"
)

type TriggerKind string

const (
	TriggerCron       TriggerKind = "cron"
	TriggerCommand    TriggerKind = "terminal.command"
	TriggerOutput     TriggerKind = "terminal.output"
	TriggerExitCode   TriggerKind = "terminal.exit_code"
	TriggerBattery    TriggerKind = "system.battery"
	TriggerFileChange TriggerKind = "system.file_change"
	TriggerClipboard  TriggerKind = "system.clipboard"
	TriggerIdle       TriggerKind = "system.idle"
)

type ActionKind string

const (
	ActionExec     ActionKind = "terminal.exec"
	ActionNotify   ActionKind = "terminal.notify"
	ActionOpenPane ActionKind = "terminal.open_pane"
	ActionHTTP     ActionKind = "http.post"
	ActionSSH      ActionKind = "ssh.exec"
)

type Rule struct {
	ID         string
	Name       string
	Enabled    bool
	Trigger    TriggerDef
	Conditions []Condition
	Actions    []ActionDef
}

type TriggerDef struct {
	Kind   TriggerKind
	Params map[string]string
}

type Condition struct {
	Field string
	Op    string
	Value string
}

type ActionDef struct {
	Kind   ActionKind
	Params map[string]string
}

type Event struct {
	Kind    TriggerKind
	Payload map[string]string
	Time    time.Time
}

type Engine struct {
	mu     sync.RWMutex
	rules  []Rule
	logger *slog.Logger
	events chan Event
	done   chan struct{}
}

func New(logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Engine{
		logger: logger,
		events: make(chan Event, 100),
		done:   make(chan struct{}),
	}
}

func (e *Engine) Start() {
	go e.loop()
}

func (e *Engine) Stop() {
	close(e.done)
}

func (e *Engine) AddRule(r Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = append(e.rules, r)
}

func (e *Engine) RemoveRule(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, r := range e.rules {
		if r.ID == id {
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			return
		}
	}
}

func (e *Engine) Rules() []Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Rule, len(e.rules))
	copy(out, e.rules)
	return out
}

func (e *Engine) Emit(evt Event) {
	select {
	case e.events <- evt:
	default:
		e.logger.Debug("event dropped", "kind", evt.Kind)
	}
}

func (e *Engine) loop() {
	for {
		select {
		case <-e.done:
			return
		case evt := <-e.events:
			e.process(evt)
		}
	}
}

func (e *Engine) process(evt Event) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}
		if rule.Trigger.Kind != evt.Kind {
			continue
		}
		if !e.matchConditions(rule.Conditions, evt) {
			continue
		}
		e.execute(rule.Actions, evt)
	}
}

func (e *Engine) matchConditions(conds []Condition, evt Event) bool {
	for _, c := range conds {
		val, ok := evt.Payload[c.Field]
		if !ok {
			return false
		}
		switch c.Op {
		case "eq":
			if val != c.Value {
				return false
			}
		case "neq":
			if val == c.Value {
				return false
			}
		case "contains":
			if !contains(val, c.Value) {
				return false
			}
		default:
			if val != c.Value {
				return false
			}
		}
	}
	return true
}

func (e *Engine) execute(actions []ActionDef, evt Event) {
	for _, a := range actions {
		switch a.Kind {
		case ActionNotify:
			msg := interpolate(a.Params["message"], evt)
			e.logger.Info("automation notify", "message", msg)
		case ActionExec:
			cmd := interpolate(a.Params["command"], evt)
			e.logger.Info("automation exec", "command", cmd)
		default:
			e.logger.Debug("unhandled action", "kind", a.Kind)
		}
	}
}

func interpolate(tmpl string, evt Event) string {
	return tmpl
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
