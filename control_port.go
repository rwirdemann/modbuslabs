package modbuslabs

type ControlPort interface {
	AddSlave(unitID uint8)
	ConnectSlave(unitID uint8)
	DisconnectSlave(unitID uint8)
}
