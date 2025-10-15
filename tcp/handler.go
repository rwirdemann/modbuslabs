package tcp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"net"
	"strings"

	"github.com/rwirdemann/modbuslabs"
	"github.com/rwirdemann/modbuslabs/pkg/modbus"
)

type Connection struct {
	conn net.Conn
}

func NewConnection(c net.Conn) *Connection {
	return &Connection{conn: c}
}

func (r Connection) Read(p []byte) (n int, err error) {
	return r.conn.Read(p)
}

func (r Connection) Write(b []byte) (n int, err error) {
	return r.conn.Write(b)
}

func (r Connection) Close() {
	r.conn.Close()
}

func (r Connection) Name() string {
	return r.conn.RemoteAddr().String()
}

type Handler struct {
	url          string
	listener     net.Listener
	protocolPort modbuslabs.ProtocolPort
}

func NewHandler(url string, protocolPort modbuslabs.ProtocolPort) (*Handler, error) {
	splitURL := strings.SplitN(url, "://", 2)
	if len(splitURL) == 2 {
		return &Handler{url: splitURL[1], protocolPort: protocolPort}, nil
	}
	return nil, fmt.Errorf("invalid url format %s", url)
}

func (h *Handler) Start(ctx context.Context, processPDU modbuslabs.ProcessPDUCallback) (err error) {
	h.listener, err = net.Listen("tcp", h.url)
	if err != nil {
		return fmt.Errorf("failed to start TCP listener: %w", err)
	}
	go h.startRequestCycle(ctx, processPDU)
	slog.Debug("TCP listener started", "url", h.url)
	return nil
}

func (h *Handler) Stop() error {
	if h.listener != nil {
		slog.Debug("Stopping TCP listener", "url", h.url)
		return h.listener.Close()
	}
	return nil
}

func (h *Handler) startRequestCycle(ctx context.Context, processPDU modbuslabs.ProcessPDUCallback) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			slog.Debug("listening...")
			conn, err := h.listener.Accept()
			if err == nil {
				slog.Debug("client connected", "remote addr", conn.RemoteAddr())
				for {
					if err := h.processRequest(conn, processPDU); err != nil {
						break
					}
				}
			}
		}
	}
}

func (h *Handler) processRequest(conn net.Conn, processPDU modbuslabs.ProcessPDUCallback) error {
	header, pdu, txnId, err := modbus.ReadMBAPFrame(conn)
	if err != nil {
		if err == io.EOF {
			slog.Debug("client disconnected", "remote addr", conn.RemoteAddr())
			conn.Close()
		}
		return err
	}
	slog.Debug("MBAP header received", "pdu", pdu, "txid", txnId)
	h.protocolPort.Info(fmt.Sprintf("TX % X % X % X", header, pdu.FunctionCode, pdu.Payload))

	addr := modbus.BytesToUint16(pdu.Payload[0:2])
	processPDU(addr, *pdu)

	var res *modbus.PDU
	if pdu.FunctionCode == modbus.FC2ReadDiscreteInput {
		res = &modbus.PDU{
			UnitId:       pdu.UnitId,
			FunctionCode: pdu.FunctionCode,
			Payload:      []byte{0},
		}
		quantity := modbus.BytesToUint16(pdu.Payload[2:4])
		var values = make([]bool, quantity)
		for i := range int(quantity) {
			values[i] = rand.Intn(2) == 1
		}
		resCount := len(values)
		// byte count (1 byte for 8 coils)
		res.Payload[0] = uint8(resCount / 8)
		if resCount%8 != 0 {
			res.Payload[0]++
		}
		// coil values
		res.Payload = append(res.Payload, modbus.EncodeBools(values)...)
	}

	if pdu.FunctionCode == modbus.FC4ReadInputRegisters {
		quantity := modbus.BytesToUint16(pdu.Payload[2:4])
		slog.Info("FC4 Request", "txnId", txnId, "addr", addr, "quantity", quantity)

		res = &modbus.PDU{
			UnitId:       pdu.UnitId,
			FunctionCode: pdu.FunctionCode,
			Payload:      []byte{uint8(quantity * 2)}, // byte count (2 bytes per register)
		}

		// Generate float32 values (2 registers per float)
		// Each float needs 2 consecutive registers (4 bytes)
		numFloats := int(quantity) / 2
		for i := 0; i < numFloats; i++ {
			// Generate a random float between 0 and 100
			floatValue := rand.Float32() * 100.0

			// Convert float32 to bytes (big endian)
			bits := math.Float32bits(floatValue)
			res.Payload = append(res.Payload,
				byte(bits>>24),
				byte(bits>>16),
				byte(bits>>8),
				byte(bits))
		}

		// Handle odd number of registers (if quantity is odd)
		if int(quantity)%2 != 0 {
			// Add one more register with a random uint16 value
			value := uint16(rand.Intn(65536))
			res.Payload = append(res.Payload, modbus.Uint16ToBytes(value)...)
		}

		// Timesync hack
		timeregAddr := []byte{0x8F, 0xFC}
		if addr == modbus.BytesToUint16(timeregAddr) {
			var syncTime uint64 = 2815470101985099801 // 2025-08-14 15:36

			// Split into 4 words (16-bit each, big endian)
			word0 := uint16((syncTime >> 48) & 0xFFFF)
			word1 := uint16((syncTime >> 32) & 0xFFFF)
			word2 := uint16((syncTime >> 16) & 0xFFFF)
			word3 := uint16(syncTime & 0xFFFF)

			// Copy the 4 words into the first 8 bytes of res.payload
			copy(res.Payload[1:3], modbus.Uint16ToBytes(word0))
			copy(res.Payload[3:5], modbus.Uint16ToBytes(word1))
			copy(res.Payload[5:7], modbus.Uint16ToBytes(word2))
			copy(res.Payload[7:9], modbus.Uint16ToBytes(word3))
		}
	}

	slog.Info("Handling", "fc", pdu.FunctionCode)

	if res != nil {
		payload := modbus.AssembleMBAPFrame(txnId, res)
		if _, err := conn.Write(payload); err != nil {
			return err
		}
		slog.Debug(fmt.Sprintf("MBAP response written: % X", payload))
		h.protocolPort.Info(fmt.Sprintf("RX % X", payload))
	}
	return nil
}
