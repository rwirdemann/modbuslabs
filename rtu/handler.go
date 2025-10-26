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

			// Sample FC16 Request to write float32:
			//
			// 65 10 90 02 00 02 04 42 F6 E9 79 7C 86
			//
			// | 0x65 | 101 | Slave-Adresse (dezimal 101) |
			// | 0x10 | 16 | Function Code (FC16 = Write Multiple Registers) |
			// | 0x90 0x02 | 36866 | Startadresse (Register 0x9002) |
			// | 0x00 0x02 | 2 | Anzahl Register (Float32 = 2 Register) |
			// | 0x04 | 4 | Byte Count (2 Register × 2 Bytes = 4 Bytes) |
			// | 0x42 0xF6 | | Float32 High Word (Bytes 1-2) |
			// | 0xE9 0x79 | | Float32 Low Word (Bytes 3-4) |
			// | 0x7C 0x86 | | CRC-16 (Low Byte, High Byte) |
			//
			// Float32-Conversion:
			//
			// Der Wert **123.456** als IEEE 754 Float32:
			// - **Hexadezimal:** 0x42F6E979
			// - **Register 0x9002:** 0x42F6
			// - **Register 0x9003:** 0xE979
			//
			// The Response
			// 65 10 90 02 00 02 01 47
			//
			// | 0x65 | 101 | Slave-Adresse (Echo vom Request) |
			// | 0x10 | 16 | Function Code (Echo vom Request) |
			// | 0x90 0x02 | 36866 | Startadresse (Echo: Register 0x9002) |
			// | 0x00 0x02 | 2 | Anzahl geschriebener Register (Echo) |
			// | 0x01 0x47 | | CRC-16 (Low Byte, High Byte) |
			//
			// ## Wichtige Punkte:
			// 1. **Bei FC16 (Write Multiple Registers)** gibt der Slave die **gleichen Informationen zurück** wie im Request (ohne die Daten selbst)
			// 2. Die Response ist **deutlich kürzer** als der Request (nur 8 Bytes statt 13 Bytes)
			// 3. Der Slave bestätigt damit: "Ich habe 2 Register ab Adresse 0x9002 erfolgreich geschrieben"
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
				h.protocolPort.Info(fmt.Sprintf("TX % X", data))

				// Verify CRC
				receivedCRC := binary.LittleEndian.Uint16(data[len(data)-2:])
				calculatedCRC := calculateCRC(data[:len(data)-2])
				if receivedCRC != calculatedCRC {
					h.protocolPort.Info("crc's are not equal")
					continue
				}

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
			h.protocolPort.Separator()
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
