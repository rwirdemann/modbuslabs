package rtu

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"time"

	"github.com/goburrow/serial"
	"github.com/rwirdemann/modbuslabs"
	"github.com/rwirdemann/modbuslabs/pkg/modbus"
)

// Start starts the RTU handler.
type Handler struct {
	serialPort   serial.Port
	url          string
	protocolPort modbuslabs.ProtocolPort
}

// NewHandler creates a new RTU handler.
func NewHandler(url string, protocolPort modbuslabs.ProtocolPort) *Handler {
	return &Handler{url: url, protocolPort: protocolPort}
}

func (h *Handler) Start(ctx context.Context, processPDU modbuslabs.ProcessPDUCallback) (err error) {
	config := &serial.Config{
		Address:  h.url,
		BaudRate: 9600,
		DataBits: 8,
		Parity:   "N",
		StopBits: 1,
		Timeout:  5 * time.Second,
	}

	h.serialPort, err = serial.Open(config)
	if err != nil {
		return fmt.Errorf("failed to open serial port: %w", err)
	}

	go h.startRequestCycle(ctx, processPDU)
	slog.Debug("RTU listener started", "url", h.url)
	return nil
}

func (h *Handler) Description() string {
	return h.url
}

func (h *Handler) startRequestCycle(ctx context.Context, processPDU modbuslabs.ProcessPDUCallback) {
	buffer := make([]byte, 256)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := h.serialPort.Read(buffer)
			if err != nil {
				if err.Error() != "EOF" && err.Error() != "serial: timeout" {
					slog.Error("Error reading from serial port", "err", err)
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}
			if n > 0 {
				pdu := &modbus.PDU{}
				data := buffer[:n]
				slog.Debug("Received data from serial port", "n", n, "data", fmt.Sprintf("% X", data))
				if len(data) < 4 {
					slog.Error("Received bytes < 4")
					continue
				}
				pdu.UnitId = data[0]
				pdu.FunctionCode = data[1]
				pdu.Payload = data[2:n]

				h.protocolPort.Separator()
				h.protocolPort.Info(fmt.Sprintf("Incomming request on /virtual/com0 => %d", pdu.UnitId))

				// Verify CRC
				receivedCRC := binary.LittleEndian.Uint16(data[len(data)-2:])
				calculatedCRC := calculateCRC(data[:len(data)-2])
				if receivedCRC != calculatedCRC {
					slog.Error("crc's not equal")
					continue
				}

				h.protocolPort.Info(fmt.Sprintf("TX % X", data))
				res := processPDU(*pdu)

				// Echo back the request as response
				if res != nil {
					// Build complete RTU frame: UnitId + FunctionCode + Payload + CRC
					response := make([]byte, 0, 2+len(res.Payload))
					response = append(response, res.UnitId)
					response = append(response, res.FunctionCode)
					response = append(response, res.Payload...)

					// Calculate and append CRC
					crc := calculateCRC(response)
					response = append(response, byte(crc&0xFF), byte(crc>>8))

					h.serialPort.Write(response)
					h.protocolPort.Info(fmt.Sprintf("RX % X", response))
				}
			}
		}
	}
}

// Stop stops the handler.
func (h *Handler) Stop() error {
	slog.Debug("Closing serial port")
	if h.serialPort != nil {
		h.serialPort.Close()
	}
	return nil
}

func calculateCRC(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b)
		for range 8 {
			if crc&0x0001 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc = crc >> 1
			}
		}
	}
	return crc
}
