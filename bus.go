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
	return m.handler.Start(ctx, m.handleMasterConnection)
}

func (m *Bus) Stop() error {
	return m.handler.Stop()
}

func (h *Bus) handleMasterConnection(ctx context.Context, conn modbus.Connection) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			header, pdu, txnId, err := modbus.ReadMBAPFrame(conn)
			if err != nil {
				if err == io.EOF {
					slog.Debug("client disconnected", "remote addr", conn.Name())
					conn.Close()
					return
				}
				slog.Error("failed to read MBAP header", "error", err)
				continue
			}
			slog.Debug("MBAP header received", "pdu", pdu, "txid", txnId)
			h.protocolPort.Info(fmt.Sprintf("req % X % X % X", header, pdu.FunctionCode, pdu.Payload))
			if pdu.FunctionCode != modbus.FCWriteSingleRegister {
				slog.Error("function code not implemented", "fc", pdu.FunctionCode)
				continue
			}
			payload := modbus.AssembleMBAPFrame(txnId, pdu)
			if _, err := conn.Write(payload); err != nil {
				slog.Error("failed to write response")
				continue
			}
			slog.Debug(fmt.Sprintf("MBAP response written: % X", payload))
			h.protocolPort.Info(fmt.Sprintf("rsp % X", payload))
		}
	}
}
