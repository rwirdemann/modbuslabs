package modbuslabs

import (
	"context"
	"io"
	"log/slog"

	"github.com/rwirdemann/modbuslabs/pkg/modbus"
)

type Bus struct {
	handler TransportHandler
}

func NewBus(handler TransportHandler) *Bus {
	return &Bus{handler: handler}
}

func (m *Bus) Start(ctx context.Context) error {
	return m.handler.Start(ctx, m.handleMasterConnection)
}

func (m *Bus) Stop() error {
	return m.handler.Stop()
}

func (h *Bus) handleMasterConnection(ctx context.Context, r modbus.Reader) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			pdu, txid, err := modbus.ReadMBAPFrame(r, modbus.MaxFrameLength)
			if err != nil {
				if err == io.EOF {
					slog.Info("client disconnected", "remote addr", r.Name())
					r.Close()
					return
				}
				slog.Error("failed to read MBAP header", "error", err)
			} else {
				slog.Info("MBAP header received", "pdu", pdu, "txid", txid)
			}
		}
	}
}
