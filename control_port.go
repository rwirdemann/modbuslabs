package modbuslabs

type ControlPort interface {
	ConnectSlave(unitID uint8, url string) error
	DisconnectSlave(unitID uint8)
	Status() string
}
