package modbuslabs

type ControlPort interface {
	ConnectSlave(unitID uint8, url string)
	DisconnectSlave(unitID uint8)
	Status() string
}
