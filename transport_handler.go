package modbuslabs

import (
	"context"

	"github.com/rwirdemann/modbuslabs/pkg/modbus"
)

type HandleMasterConnectionCallback func(ctx context.Context, conn modbus.Connection)

type TransportHandler interface {
	Start(ctx context.Context, cb HandleMasterConnectionCallback) (err error)
	Stop() error
}
