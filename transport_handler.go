package modbuslabs

import (
	"context"
)

type ProcessPDUCallback func(pdu PDU) *PDU

type TransportHandler interface {
	Start(ctx context.Context, processPDU ProcessPDUCallback) (err error)
	Stop() error
	Description() string
}
