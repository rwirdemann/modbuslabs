package config

import (
	"testing"

	"github.com/BurntSushi/toml"
)

func TestRuleValidate_PluginOnly(t *testing.T) {
	r := Rule{
		Trigger:  "on_write",
		Register: 0x1000,
		Plugin:   "my_plugin",
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestRuleValidate_BothActionAndPlugin(t *testing.T) {
	r := Rule{
		Trigger:  "on_write",
		Register: 0x1000,
		Action:   "toggle",
		Plugin:   "my_plugin",
	}
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for both action and plugin")
	}
}

func TestRuleValidate_NeitherActionNorPlugin(t *testing.T) {
	r := Rule{
		Trigger:  "on_write",
		Register: 0x1000,
	}
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for neither action nor plugin")
	}
}

func TestRuleValue_ParseAny(t *testing.T) {
	type wrapper struct {
		Value *RuleValue `toml:"value"`
	}
	var w wrapper
	if _, err := toml.Decode(`value = "any"`, &w); err != nil {
		t.Fatalf("TOML decode: %v", err)
	}
	if w.Value == nil {
		t.Fatal("expected non-nil RuleValue")
	}
	if !w.Value.Any {
		t.Errorf("expected Any=true, got false")
	}
	if w.Value.V != 0 {
		t.Errorf("expected V=0, got %d", w.Value.V)
	}
}

func TestRuleValue_ParseUint16(t *testing.T) {
	type wrapper struct {
		Value *RuleValue `toml:"value"`
	}
	var w wrapper
	if _, err := toml.Decode(`value = 42`, &w); err != nil {
		t.Fatalf("TOML decode: %v", err)
	}
	if w.Value == nil {
		t.Fatal("expected non-nil RuleValue")
	}
	if w.Value.Any {
		t.Errorf("expected Any=false")
	}
	if w.Value.V != 42 {
		t.Errorf("expected V=42, got %d", w.Value.V)
	}
}
