package modbuslabs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/rwirdemann/modbuslabs/pkg/modbus"
)

// Bus represents a Modbus bus.
type Bus struct {
	handler      TransportHandler
	protocolPort ProtocolPort
	registers    map[uint8]map[uint16]uint16 // map[unitID]map[registerAddr]value
}

// NewBus creates a new Modbus bus.
func NewBus(handler TransportHandler, protocolPort ProtocolPort) *Bus {
	return &Bus{
		handler:      handler,
		protocolPort: protocolPort,
		registers:    make(map[uint8]map[uint16]uint16),
	}
}

// Start starts the Modbus bus.
func (m *Bus) Start(ctx context.Context) error {
	return m.handler.Start(ctx, m.processPDU)
}

// Stop stops the Modbus bus.
func (m *Bus) Stop() error {
	return m.handler.Stop()
}

func (h *Bus) processPDU(pdu modbus.PDU) *modbus.PDU {
	addr := modbus.BytesToUint16(pdu.Payload[0:2])
	quantity := modbus.BytesToUint16(pdu.Payload[2:4])
	slog.Debug("processPDU", "regAddr", fmt.Sprintf("%X", addr), "quantitiy", quantity, "pdu", pdu)

	if pdu.FunctionCode == modbus.FC2ReadDiscreteInput {
		res := &modbus.PDU{
			UnitId:       pdu.UnitId,
			FunctionCode: pdu.FunctionCode,
			Payload:      []byte{0},
		}
		var values = make([]bool, quantity)

		// Read values from registers map
		for i := uint16(0); i < quantity; i++ {
			currentAddr := addr + i
			var value uint16

			// Check if unit has registers
			if unitRegs, exists := h.registers[pdu.UnitId]; exists {
				// Check if register exists
				if regValue, exists := unitRegs[currentAddr]; exists {
					value = regValue
					slog.Debug("FC2 reading from map", "unitID", pdu.UnitId, "addr", currentAddr, "value", value)
				} else {
					slog.Debug("no value for discrete input", "addr", currentAddr)
				}
			} else {
				slog.Debug("no registers for unit", "unitID", pdu.UnitId)
			}

			// Convert register value to boolean (0x0000 = false, anything else = true)
			// For coils written with FC5, 0xFF00 = true
			values[i] = value != 0x0000
		}

		resCount := len(values)

		// byte count (1 byte for 8 coils)
		res.Payload[0] = uint8(resCount / 8)
		if resCount%8 != 0 {
			res.Payload[0]++
		}

		// coil values
		res.Payload = append(res.Payload, modbus.EncodeBools(values)...)
		return res
	}

	if pdu.FunctionCode == modbus.FC4ReadInputRegisters {
		byteCount := uint8(quantity * 2)
		res := &modbus.PDU{
			UnitId:       pdu.UnitId,
			FunctionCode: pdu.FunctionCode,
			Payload:      make([]byte, 1+byteCount), // byte count + register values
		}
		res.Payload[0] = byteCount

		// Timesync hack - handle first
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
			return res
		}

		// Read values from registers map
		for i := uint16(0); i < quantity; i++ {
			currentAddr := addr + i
			var value uint16

			// Check if unit has registers
			if unitRegs, exists := h.registers[pdu.UnitId]; exists {
				// Check if register exists
				if regValue, exists := unitRegs[currentAddr]; exists {
					value = regValue
					slog.Debug("FC4 reading from map", "unitID", pdu.UnitId, "addr", currentAddr, "value", value)
				} else {
					slog.Debug("no value for register", "regValue", regValue)
				}
			} else {
				slog.Debug("no registers for unit", "unitID", pdu.UnitId)
			}

			// Append register value as 2 bytes (big endian)
			res.Payload = append(res.Payload, modbus.Uint16ToBytes(value)...)
		}

		return res
	}

	if pdu.FunctionCode == modbus.FC5WriteSingleCoil {
		// FC5 payload format: [coilAddr(2 bytes)][value(2 bytes)]
		// value is 0xFF00 for ON, 0x0000 for OFF
		value := modbus.BytesToUint16(pdu.Payload[2:4])

		// Initialize unit's register map if it doesn't exist
		if h.registers[pdu.UnitId] == nil {
			h.registers[pdu.UnitId] = make(map[uint16]uint16)
		}

		// Store the coil value (0xFF00 for true, 0x0000 for false)
		h.registers[pdu.UnitId][addr] = value
		slog.Debug("FC5 Write Single Coil", "unitID", pdu.UnitId, "addr", fmt.Sprintf("%X", addr), "value", fmt.Sprintf("%X", value))

		// FC5 response: echo back the request (coil address + value)
		res := &modbus.PDU{
			UnitId:       pdu.UnitId,
			FunctionCode: pdu.FunctionCode,
			Payload:      pdu.Payload[0:4], // Echo back address and value
		}
		return res
	}

	if pdu.FunctionCode == modbus.FC6WriteSingleRegister {
		// FC6 payload format: [regAddr(2 bytes)][value(2 bytes)]
		value := modbus.BytesToUint16(pdu.Payload[2:4])

		// Initialize unit's register map if it doesn't exist
		if h.registers[pdu.UnitId] == nil {
			h.registers[pdu.UnitId] = make(map[uint16]uint16)
		}

		// Store the value
		h.registers[pdu.UnitId][addr] = value
		slog.Debug("FC6 Write Single Register", "unitID", pdu.UnitId, "addr", addr, "value", value)

		// FC6 response: echo back the request (register address + value)
		res := &modbus.PDU{
			UnitId:       pdu.UnitId,
			FunctionCode: pdu.FunctionCode,
			Payload:      pdu.Payload[0:4], // Echo back address and value
		}
		return res
	}

	return nil
}

func (h *Bus) AddSlave(unitID uint8) {

}

func (h *Bus) ConnectSlave(unitID uint8) {

}

func (h *Bus) DisconnectSlave(unitID uint8) {

}
