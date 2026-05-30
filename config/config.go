package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Config represents the slavesim configuration
type Config struct {
	Transports []Transport `toml:"transport"`
	Slaves     []Slave     `toml:"slave"`
}

// Transport defines a transport handler (TCP or RTU)
type Transport struct {
	Type        string `toml:"type"`         // "tcp" or "rtu"
	Address     string `toml:"address"`      // For TCP: "localhost:502", for RTU: "/tmp/virtualcom0"
	PeerAddress string `toml:"peer_address"` // RTU only: client-side TTY, e.g. "/tmp/ttyV1"
}

// Slave defines a slave configuration
type Slave struct {
	ID      uint8  `toml:"id"`      // Slave ID (e.g., 101)
	Address string `toml:"address"` // Reference to transport address
	Rules   []Rule `toml:"rule"`    // Behavioral rules for this slave
}

// RuleValue holds a uint16 register value or the wildcard "any".
type RuleValue struct {
	V   uint16
	Any bool
}

// UnmarshalTOML implements toml.Unmarshaler. It accepts an integer or
// the string "any".
func (rv *RuleValue) UnmarshalTOML(data any) error {
	switch v := data.(type) {
	case string:
		if v == "any" {
			rv.Any = true
			return nil
		}
		return fmt.Errorf(
			"invalid value %q, expected a number or \"any\"", v,
		)
	case int64:
		rv.V = uint16(v)
		return nil
	default:
		return fmt.Errorf("unexpected type %T for rule value", data)
	}
}

// Rule defines a behavior rule for a slave.
type Rule struct {
	Trigger       string     `toml:"trigger"`
	Register      uint16     `toml:"register"`
	Action        string     `toml:"action"`
	Plugin        string     `toml:"plugin"`
	Value         *RuleValue `toml:"value"`
	WriteRegister *uint16    `toml:"write_register"`
	WriteValue    *uint16    `toml:"write_value"`
}

// Load reads and parses a TOML configuration file
func Load(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse TOML: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if len(c.Transports) == 0 {
		return fmt.Errorf("at least one transport must be defined")
	}

	// Check that all transports have valid types
	transportAddresses := make(map[string]bool)
	for i, t := range c.Transports {
		if t.Type != "tcp" && t.Type != "rtu" {
			return fmt.Errorf("transport[%d]: invalid type %q, must be 'tcp' or 'rtu'", i, t.Type)
		}
		if t.Address == "" {
			return fmt.Errorf("transport[%d]: address is required", i)
		}
		if t.Type == "rtu" && t.PeerAddress == "" {
			return fmt.Errorf(
				"transport[%d]: peer_address required for rtu transport",
				i,
			)
		}
		transportAddresses[t.Address] = true
	}

	// Check that all slaves reference valid transports
	for i, s := range c.Slaves {
		if s.ID <= 0 {
			return fmt.Errorf("slave[%d]: invalid ID %d, must be between 1 and 255", i, s.ID)
		}
		if !transportAddresses[s.Address] {
			return fmt.Errorf("slave[%d]: address %q does not match any transport", i, s.Address)
		}

		// Validate rules
		for j, rule := range s.Rules {
			if err := rule.Validate(); err != nil {
				return fmt.Errorf("slave[%d].rule[%d]: %w", i, j, err)
			}
		}
	}

	return nil
}

// GetTransportByAddress returns the transport configuration for a given address
func (c *Config) GetTransportByAddress(address string) *Transport {
	for _, t := range c.Transports {
		if t.Address == address {
			return &t
		}
	}
	return nil
}

// Validate checks if a rule is valid.
func (r *Rule) Validate() error {
	validTriggers := map[string]bool{
		"on_read":       true,
		"on_write":      true,
		"on_read_write": true,
	}
	if !validTriggers[r.Trigger] {
		return fmt.Errorf(
			"invalid trigger %q, must be one of: "+
				"on_read, on_write, on_read_write",
			r.Trigger,
		)
	}

	if r.Action != "" && r.Plugin != "" {
		return fmt.Errorf(
			"rule cannot specify both 'action' and 'plugin'",
		)
	}
	if r.Action == "" && r.Plugin == "" {
		return fmt.Errorf(
			"rule must specify either 'action' or 'plugin'",
		)
	}

	if r.Plugin != "" {
		return nil
	}

	validActions := map[string]bool{
		"set_value":      true,
		"increment":      true,
		"decrement":      true,
		"toggle":         true,
		"write_register": true,
	}
	if !validActions[r.Action] {
		return fmt.Errorf(
			"invalid action %q, must be one of: "+
				"set_value, increment, decrement, toggle, write_register",
			r.Action,
		)
	}

	if r.Action == "set_value" && r.Value == nil {
		return fmt.Errorf("set_value action requires 'value' field")
	}

	if r.Action == "write_register" {
		if r.WriteRegister == nil {
			return fmt.Errorf(
				"write_register action requires 'write_register' field",
			)
		}
		if r.WriteValue == nil {
			return fmt.Errorf(
				"write_register action requires 'write_value' field",
			)
		}
	}

	return nil
}
