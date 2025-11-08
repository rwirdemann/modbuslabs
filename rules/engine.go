package rules

import (
	"fmt"
	"log/slog"

	"github.com/rwirdemann/modbuslabs/config"
)

// TriggerType defines when a rule should be executed
type TriggerType string

const (
	TriggerOnRead      TriggerType = "on_read"
	TriggerOnWrite     TriggerType = "on_write"
	TriggerOnReadWrite TriggerType = "on_read_write"
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

// ApplyRead applies all read-triggered rules for the given register
// Returns the modified value and true if any rules were applied
func (e *Engine) ApplyRead(register uint16, currentValue uint16) (uint16, bool) {
	return e.apply(register, currentValue, TriggerOnRead)
}

// ApplyWrite applies all write-triggered rules for the given register
// Returns the modified value and true if any rules were applied
func (e *Engine) ApplyWrite(register uint16, currentValue uint16) (uint16, bool) {
	return e.apply(register, currentValue, TriggerOnWrite)
}

func (e *Engine) apply(register uint16, currentValue uint16, triggerType TriggerType) (uint16, bool) {
	rules, exists := e.rules[register]
	if !exists {
		return currentValue, false
	}

	modified := false
	value := currentValue

	for _, rule := range rules {
		// Check if this rule should be triggered
		if !e.shouldTrigger(rule.Trigger, triggerType) {
			continue
		}

		oldValue := value
		value = e.executeAction(rule, value)
		modified = true

		slog.Debug("Rule executed",
			"register", fmt.Sprintf("0x%04X", register),
			"trigger", rule.Trigger,
			"action", rule.Action,
			"oldValue", fmt.Sprintf("0x%04X", oldValue),
			"newValue", fmt.Sprintf("0x%04X", value))
	}

	return value, modified
}

func (e *Engine) Status() string {
	if len(e.rules) == 0 {
		return ""
	}
	s := "\n      Rules:"
	for register, rules := range e.rules {
		for i, r := range rules {
			s += fmt.Sprintf("\n      - R%d: 0x%04X => %s %s", i+1, register, r.Trigger, r.Action)
		}
	}
	return s
}

func (e *Engine) shouldTrigger(ruleTrigger string, triggerType TriggerType) bool {
	if ruleTrigger == string(TriggerOnReadWrite) {
		return true
	}
	return ruleTrigger == string(triggerType)
}

func (e *Engine) executeAction(rule config.Rule, currentValue uint16) uint16 {
	switch rule.Action {
	case "set_value":
		return rule.Value

	case "increment":
		return currentValue + 1

	case "decrement":
		if currentValue > 0 {
			return currentValue - 1
		}
		return 0

	case "toggle":
		// For boolean values (0x0000 or 0xFF00)
		if currentValue == 0x0000 {
			return 0xFF00
		}
		return 0x0000

	default:
		slog.Warn("Unknown action", "action", rule.Action)
		return currentValue
	}
}

// HasRulesForRegister checks if there are any rules for the given register
func (e *Engine) HasRulesForRegister(register uint16) bool {
	_, exists := e.rules[register]
	return exists
}
