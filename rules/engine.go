package rules

import (
	"fmt"
	"log/slog"

	"github.com/rwirdemann/modbuslabs/config"
)

// TriggerType defines when a rule should be executed
type TriggerType string

const (
	TriggerOnRead  TriggerType = "on_read"
	TriggerOnWrite TriggerType = "on_write"
)

// Engine manages and executes rules for register operations
type Engine struct {
	rules    map[uint16][]config.Rule
	registry Registry
}

// NewEngine creates a new rule engine from configuration rules. A nil
// registry is safe when no plugins are used.
func NewEngine(configRules []config.Rule, registry Registry) *Engine {
	e := &Engine{
		rules:    make(map[uint16][]config.Rule),
		registry: registry,
	}
	for _, rule := range configRules {
		e.rules[rule.Register] = append(e.rules[rule.Register], rule)
	}
	return e
}

// ApplyReadRules returns the target register, the new value, and whether a
// rule fired. For set_value the target is the same register; for
// write_register the target is the register named in the rule.
func (e *Engine) ApplyReadRules(
	register uint16,
	currentValue uint16,
) (uint16, uint16, bool) {
	rules, exists := e.rules[register]
	if !exists {
		return 0, 0, false
	}
	for _, rule := range rules {
		if !e.shouldTrigger(rule.Trigger, TriggerOnRead) {
			continue
		}
		if rule.Action == "write_register" {
			slog.Debug(
				"Rule executed",
				"register", fmt.Sprintf("0x%04X", register),
				"trigger", rule.Trigger,
				"action", rule.Action,
				"writeRegister",
				fmt.Sprintf("0x%04X", *rule.WriteRegister),
				"writeValue",
				fmt.Sprintf("0x%04X", *rule.WriteValue),
			)
			return *rule.WriteRegister, *rule.WriteValue, true
		}
		slog.Debug(
			"Rule executed",
			"register", fmt.Sprintf("0x%04X", register),
			"trigger", rule.Trigger,
			"action", rule.Action,
			"oldValue", fmt.Sprintf("0x%04X", currentValue),
			"newValue", fmt.Sprintf("0x%04X", rule.Value.V),
		)
		return register, rule.Value.V, true
	}

	return 0, 0, false
}

func (e *Engine) ApplyWriteRules(
	register uint16,
	currentValue uint16,
	registers map[uint16]uint16,
	payload []byte,
) (uint16, uint16, bool) {
	rules, exists := e.rules[register]
	if !exists {
		return 0, 0, false
	}

	for _, rule := range rules {
		if !e.shouldTrigger(rule.Trigger, TriggerOnWrite) {
			continue
		}
		if rule.Value != nil && !rule.Value.Any &&
			rule.Value.V != currentValue {
			continue
		}
		if rule.Plugin != "" {
			e.callPlugin(
				rule.Plugin, register, currentValue, registers, payload,
			)
			return 0, 0, false
		}
		return *rule.WriteRegister, *rule.WriteValue, true
	}

	return 0, 0, false
}

func (e *Engine) callPlugin(
	name string,
	register, value uint16,
	registers map[uint16]uint16,
	payload []byte,
) {
	if e.registry == nil {
		slog.Error("no plugin registry configured", "plugin", name)
		return
	}
	p, ok := e.registry[name]
	if !ok {
		slog.Error("plugin not found", "plugin", name)
		return
	}
	if err := p.Execute(register, value, registers, payload); err != nil {
		slog.Error(
			"plugin error",
			"plugin", name,
			"register", fmt.Sprintf("0x%04X", register),
			"error", err,
		)
		return
	}
	slog.Debug(
		"plugin executed",
		"plugin", name,
		"register", fmt.Sprintf("0x%04X", register),
		"value", fmt.Sprintf("0x%04X", value),
	)
}

func (e *Engine) Status() string {
	if len(e.rules) == 0 {
		return ""
	}
	s := "\n    Rules:"
	for register, rules := range e.rules {
		for i, r := range rules {
			s = fmt.Sprintf(
				"%s\n    - R%d: 0x%04X => %s %s",
				s, i+1, register, r.Trigger, r.ActionString(),
			)
		}
	}
	return s
}

func (e *Engine) shouldTrigger(ruleTrigger string, triggerType TriggerType) bool {
	return ruleTrigger == string(triggerType)
}
