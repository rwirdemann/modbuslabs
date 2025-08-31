package modbuslabs

import (
	"context"

	"github.com/rwirdemann/modbuslabs/pkg/modbus"
)

type HandleMasterCallback func(ctx context.Context, r modbus.Reader)

type TransportHandler interface {
	Start(ctx context.Context, cb HandleMasterCallback) (err error)
	Stop() error
}
