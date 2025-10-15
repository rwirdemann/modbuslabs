package modbuslabs

import (
	"context"
	"log/slog"

	"github.com/rwirdemann/modbuslabs/pkg/modbus"
)

// Bus represents a Modbus bus.
type Bus struct {
	handler      TransportHandler
	protocolPort ProtocolPort
}

// NewBus creates a new Modbus bus.
func NewBus(handler TransportHandler, protocolPort ProtocolPort) *Bus {
	return &Bus{handler: handler, protocolPort: protocolPort}
}

// Start starts the Modbus bus.
func (m *Bus) Start(ctx context.Context) error {
	return m.handler.Start(ctx, m.processPDU)
}

// Stop stops the Modbus bus.
func (m *Bus) Stop() error {
	return m.handler.Stop()
}

func (h *Bus) processPDU(registerAddress uint16, pdu modbus.PDU) {
	slog.Debug("processPDU", "regAddr", registerAddress, "pdu", pdu)
}

func (h *Bus) AddSlave(unitID uint8) {

}

func (h *Bus) ConnectSlave(unitID uint8) {

}

func (h *Bus) DisconnectSlave(unitID uint8) {

}
