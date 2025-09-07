package modbuslabs

import (
	"context"

	"github.com/rwirdemann/modbuslabs/pkg/modbus"
)

type ProcessPDUCallback func(registerAddress uint16, pdu modbus.PDU)

type TransportHandler interface {
	Start(ctx context.Context, processPDU ProcessPDUCallback) (err error)
	Stop() error
}
