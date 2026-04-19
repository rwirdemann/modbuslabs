package modbuslabs

type ControlPort interface {
	ConnectSlave(unitID uint8, url string) error
	DisconnectSlave(unitID uint8)
	Status() string

	// WriteRegister writes one or more uint16 values to consecutive
	// registers on the slave identified by unitID, starting at addr.
	WriteRegister(unitID uint8, addr uint16, values []uint16) error
}
