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
	Type    string `toml:"type"`    // "tcp" or "rtu"
	Address string `toml:"address"` // For TCP: "localhost:502", for RTU: "/tmp/virtualcom0"
}

// Slave defines a slave configuration
type Slave struct {
	ID      uint8  `toml:"id"`      // Slave ID (e.g., 101)
	Address string `toml:"address"` // Reference to transport address
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
		transportAddresses[t.Address] = true
	}

	// Check that all slaves reference valid transports
	for i, s := range c.Slaves {
		if s.ID <= 0 || s.ID > 255 {
			return fmt.Errorf("slave[%d]: invalid ID %d, must be between 1 and 255", i, s.ID)
		}
		if !transportAddresses[s.Address] {
			return fmt.Errorf("slave[%d]: address %q does not match any transport", i, s.Address)
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
