package rules

// Plugin handles register writes with stateful behavior.
type Plugin interface {
	Execute(register, value uint16, registers map[uint16]uint16) error
}

// Registry maps plugin names to Plugin implementations.
type Registry map[string]Plugin
