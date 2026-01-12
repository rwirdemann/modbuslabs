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
	rules map[uint16][]config.Rule // map[register]rules
}

// NewEngine creates a new rule engine from configuration rules
func NewEngine(configRules []config.Rule) *Engine {
	e := &Engine{
		rules: make(map[uint16][]config.Rule),
	}

	// Index rules by register for faster lookup
	for _, rule := range configRules {
		e.rules[rule.Register] = append(e.rules[rule.Register], rule)
	}

	return e
}

func (e *Engine) ApplyReadRules(register uint16, currentValue uint16) (uint16, bool) {
	rules, exists := e.rules[register]
	if !exists {
		return 0, false
	}
	for _, rule := range rules {
		if !e.shouldTrigger(rule.Trigger, TriggerOnRead) {
			continue
		}
		slog.Debug("Rule executed", "register", fmt.Sprintf("0x%04X", register), "trigger", rule.Trigger, "action", rule.Action, "oldValue", fmt.Sprintf("0x%04X", currentValue), "newValue", fmt.Sprintf("0x%04X", *rule.Value))
		return *rule.Value, true
	}

	return 0, false
}

func (e *Engine) ApplyWriteRules(register uint16, currentValue uint16, registers map[uint16]uint16) (uint16, uint16, bool) {
	rules, exists := e.rules[register]
	if !exists {
		return 0, 0, false
	}

	for _, rule := range rules {
		if !e.shouldTrigger(rule.Trigger, TriggerOnWrite) {
			continue
		}

		if rule.Value != nil && *rule.Value == currentValue {
			return *rule.WriteRegister, *rule.WriteValue, true
		}
	}

	return 0, 0, false
}

func (e *Engine) Status() string {
	if len(e.rules) == 0 {
		return ""
	}
	s := "\n      Rules:"
	for register, rules := range e.rules {
		for i, r := range rules {
			s = fmt.Sprintf("%s\n      - R%d: 0x%04X => %s %s", s, i+1, register, r.Trigger, r.Action)
		}
	}
	return s
}

func (e *Engine) shouldTrigger(ruleTrigger string, triggerType TriggerType) bool {
	return ruleTrigger == string(triggerType)
}
