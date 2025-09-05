package rtu

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"time"

	"github.com/goburrow/serial"
	"github.com/rwirdemann/modbuslabs"
)

type Handler struct {
	serialPort serial.Port
	url        string
}

func NewHandler(url string) *Handler {
	return &Handler{url: url}
}

func (h *Handler) Start(ctx context.Context, cb modbuslabs.HandleMasterConnectionCallback) (err error) {
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

	go h.handleRequest(ctx, cb)
	slog.Debug("RTU listener started", "url", h.url)
	return nil
}

func (h *Handler) handleRequest(ctx context.Context, cb modbuslabs.HandleMasterConnectionCallback) {
	buffer := make([]byte, 256)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := h.serialPort.Read(buffer)
			if err != nil {
				if err.Error() != "EOF" {
					slog.Error("Error reading from serial port", "err", err)
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}
			if n > 0 {
				data := buffer[:n]
				slog.Debug("Received data from serial port", "n", n, "data", data)
				if len(data) < 4 {
					slog.Error("Received bytes < 4")
					continue
				}

				slaveID := data[0]
				functionCode := data[1]
				switch functionCode {
				case 0x06:

					if len(data) < 8 {
						slog.Error("Received bytes < 8")
						continue
					}

					// Verify CRC
					receivedCRC := binary.LittleEndian.Uint16(data[len(data)-2:])
					calculatedCRC := calculateCRC(data[:len(data)-2])
					if receivedCRC != calculatedCRC {
						slog.Error("crc's not equal")
						continue
					}

					regAddr := binary.BigEndian.Uint16(data[2:4])
					value := binary.BigEndian.Uint16(data[4:6])

					slog.Debug("received data", "slaveID", slaveID, "addr", regAddr, "value", value)

					// Echo back the request as response
					h.serialPort.Write(data)
				}
			}
		}
	}
}

func (h *Handler) Stop() error {
	return nil
}

func calculateCRC(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&0x0001 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc = crc >> 1
			}
		}
	}
	return crc
}
