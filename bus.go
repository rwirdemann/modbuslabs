package modbuslabs

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/rwirdemann/modbuslabs/pkg/modbus"
)

type Bus struct {
	handler      TransportHandler
	protocolPort ProtocolPort
}

func NewBus(handler TransportHandler, protocolPort ProtocolPort) *Bus {
	return &Bus{handler: handler, protocolPort: protocolPort}
}

func (m *Bus) Start(ctx context.Context) error {
	return m.handler.Start(ctx, m.handleMasterConnection, m.processPDU)
}

func (m *Bus) Stop() error {
	return m.handler.Stop()
}

func (h *Bus) processPDU(registerAddress uint16, pdu modbus.PDU) {
	slog.Debug("processPDU", "regAddr", registerAddress, "pdu", pdu)
}
