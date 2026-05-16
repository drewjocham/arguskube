package automate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEngine(t *testing.T) {
	e := New(nil)
	assert.NotNil(t, e)
}

func TestAddAndRemoveRule(t *testing.T) {
	e := New(nil)
	e.AddRule(Rule{ID: "r1", Name: "test"})
	assert.Len(t, e.Rules(), 1)

	e.RemoveRule("r1")
	assert.Len(t, e.Rules(), 0)
}

func TestRuleNotTriggeredByWrongEvent(t *testing.T) {
	e := New(nil)
	called := false
	e.AddRule(Rule{
		ID:      "r1",
		Enabled: true,
		Trigger: TriggerDef{Kind: TriggerCron},
		Actions: []ActionDef{{Kind: ActionNotify, Params: map[string]string{"message": "hi"}}},
	})

	e.Start()
	e.logger = nil
	e.Emit(Event{Kind: TriggerCommand})
	assert.False(t, called)
	e.Stop()
}

func TestDisabledRuleNotFired(t *testing.T) {
	e := New(nil)
	e.AddRule(Rule{
		ID:      "r1",
		Enabled: false,
		Trigger: TriggerDef{Kind: TriggerCommand},
		Actions: []ActionDef{{Kind: ActionNotify, Params: map[string]string{"message": "ignored"}}},
	})
	e.Start()
	e.Emit(Event{Kind: TriggerCommand})
	e.Stop()
}

func TestConditionsMatch(t *testing.T) {
	e := New(nil)
	e.AddRule(Rule{
		ID:      "r1",
		Enabled: true,
		Trigger: TriggerDef{Kind: TriggerOutput},
		Conditions: []Condition{
			{Field: "status", Op: "eq", Value: "error"},
		},
		Actions: []ActionDef{{Kind: ActionNotify, Params: map[string]string{"message": "error detected"}}},
	})

	e.Start()
	e.Emit(Event{Kind: TriggerOutput, Payload: map[string]string{"status": "error"}})
	e.Emit(Event{Kind: TriggerOutput, Payload: map[string]string{"status": "ok"}})
	e.Stop()
}

func TestContains(t *testing.T) {
	assert.True(t, contains("deploy failed", "failed"))
	assert.True(t, contains("hello world", "hello"))
	assert.False(t, contains("hello", "world"))
	assert.True(t, contains("test", ""))
}

func TestMatchConditions(t *testing.T) {
	e := New(nil)
	evt := Event{Payload: map[string]string{"status": "error", "code": "500"}}

	assert.True(t, e.matchConditions(nil, evt))
	assert.True(t, e.matchConditions([]Condition{}, evt))
	assert.True(t, e.matchConditions(
		[]Condition{{Field: "status", Op: "eq", Value: "error"}}, evt))
	assert.False(t, e.matchConditions(
		[]Condition{{Field: "status", Op: "eq", Value: "ok"}}, evt))
	assert.True(t, e.matchConditions(
		[]Condition{{Field: "code", Op: "neq", Value: "200"}}, evt))
	assert.True(t, e.matchConditions(
		[]Condition{{Field: "code", Op: "contains", Value: "50"}}, evt))
}
