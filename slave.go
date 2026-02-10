package modbuslabs

import (
	"fmt"
	"log/slog"

	"github.com/rwirdemann/modbuslabs/encoding"
	"github.com/rwirdemann/modbuslabs/message"
	"github.com/rwirdemann/modbuslabs/rules"
)

type Slave struct {
	unitID       uint8
	registers    map[uint16]uint16
	connected    bool
	ruleEngine   *rules.Engine
	protocolPort ProtocolPort
}

func NewSlave(unitID uint8, connected bool, ruleEngine *rules.Engine, protocolPort ProtocolPort) *Slave {
	return &Slave{unitID: unitID, registers: make(map[uint16]uint16), connected: connected, ruleEngine: ruleEngine, protocolPort: protocolPort}
}

func (s *Slave) Process(pdu PDU) *PDU {
	switch pdu.FunctionCode {
	case FC2ReadDiscreteRegisters:
		return s.processFC2(pdu)
	case FC6WriteSingleRegister:
		return s.processFC6(pdu)
	case FC17ReadWriteMultipleRegisters:
		return s.processFC17(pdu)
	}
	return nil
}

// Response Payload:  [Byte Count] [Status Byte 1] [Status Byte 2] ... Each
// status byte contains up to 8 coils.
func (h *Slave) processFC2(pdu PDU) *PDU {
	startAddr := encoding.BytesToUint16(pdu.Payload[0:2])
	quantity := encoding.BytesToUint16(pdu.Payload[2:4])
	h.protocolPort.InfoX(message.NewEncoded(fmt.Sprintf("TX FC=%d UnitID=%d Address=0x%X Quantity=%d", pdu.FunctionCode, pdu.UnitId, startAddr, quantity)))
	var values = make([]bool, quantity)

	// Read values from registers map
	for i := range quantity {
		currentAddr := startAddr + i
		var value uint16

		if regValue, exists := h.registers[currentAddr]; exists {
			value = regValue
			slog.Debug("FC2 reading from map", "unitID", pdu.UnitId, "addr", currentAddr, "value", value)
		} else {
			slog.Debug("no value for discrete input", "addr", currentAddr)
		}

		// Apply read rules. The rule is applied after the register value has been read
		// from the store. The read value is the value that is going to be changed after
		// it has been returned to the master. The new value is update in the store.
		if newValue, modified := h.ruleEngine.ApplyReadRules(currentAddr, value); modified {
			h.registers[currentAddr] = newValue
			h.protocolPort.InfoX(message.NewEncoded(fmt.Sprintf("R1 FC=2 Rule=set_value UnitID=%d Address=0x%X NewValue(after read)=0x%X", pdu.UnitId, currentAddr, newValue)))
		}

		// Convert register value to boolean (0x0000 = false, anything else = true)
		// For coils written with FC5, 0xFF00 = true
		values[i] = value != 0x0000
	}

	resCount := len(values)
	res := &PDU{
		UnitId:       pdu.UnitId,
		FunctionCode: pdu.FunctionCode,
		Payload:      []byte{0},
	}

	// byte count (1 byte for 8 coils)
	res.Payload[0] = uint8(resCount / 8)
	if resCount%8 != 0 {
		res.Payload[0]++
	}
	h.protocolPort.InfoX(message.NewEncoded(fmt.Sprintf("RX FC=%d UnitID=%d Address=0x%X Quantity=%d Values=%v", pdu.FunctionCode, pdu.UnitId, startAddr, quantity, values)))

	// coil values
	res.Payload = append(res.Payload, encoding.EncodeBools(values)...)
	return res
}

// FC6 payload format: [regAddr(2 bytes)][value(2 bytes)]
func (s *Slave) processFC6(pdu PDU) *PDU {
	addr := encoding.BytesToUint16(pdu.Payload[0:2])
	value := encoding.BytesToUint16(pdu.Payload[2:4])

	s.protocolPort.InfoX(message.NewEncoded(fmt.Sprintf("TX FC=%d UnitID=%d Address=0x%X Value=0x%X", pdu.FunctionCode, pdu.UnitId, addr, value)))

	s.registers[addr] = value
	slog.Debug("FC6 Write Single Register", "unitID", pdu.UnitId, "addr", fmt.Sprintf("0x%04X", addr), "value", fmt.Sprintf("0x%04X", value))

	if targetRegister, targetValue, applied := s.ruleEngine.ApplyWriteRules(addr, value, s.registers); applied {
		s.registers[targetRegister] = targetValue
		s.protocolPort.InfoX(message.NewEncoded(fmt.Sprintf("R1 FC=6 Rule applied UnitID=%d WriteAddress=0x%X NewValue=0x%X", pdu.UnitId, targetRegister, targetValue)))
	}

	s.protocolPort.InfoX(message.NewEncoded(fmt.Sprintf("RX FC=%d UnitID=%d Address=0x%X Value=0x%X",
		pdu.FunctionCode, pdu.UnitId, addr, value)))

	// FC6 response: echo back the request (register address + value)
	res := &PDU{
		UnitId:       pdu.UnitId,
		FunctionCode: pdu.FunctionCode,
		Payload:      pdu.Payload[0:4], // Echo back address and value
	}
	return res
}

// FC0x17 (23) combines write and read in one single request. The request
// processing starts with writting the write values to the given write
// address. It proceeds with reading the given number of bytes. These read
// values are returned in the requests response.
//
// FC17 payload example: F1 FF 00 03 F1 FF 00 01 02 01 00
//
//	[readAddr(2)]       F1 FF
//	[readQty(2)]        00 03
//	[writeAddr(2)]      F1 FF
//	[writeQty(2)]       00 01
//	[byteCount(1)]         02
//	[writeValues(N)]    01 00
//
// Response payload:  06 81 04 04 09 00 00 1F 69
//
//	[readByteCount(1)]          06
//	[upgradeResponseCommand(1)] 81
//	[upgradeResponseLength(1)]  04
//	[upgradeResponseData(len)]  04 09 00 00
func (s *Slave) processFC17(pdu PDU) *PDU {
	readAddr := encoding.BytesToUint16(pdu.Payload[0:2])
	readQty := encoding.BytesToUint16(pdu.Payload[2:4])
	writeAddr := encoding.BytesToUint16(pdu.Payload[4:6])
	writeQty := encoding.BytesToUint16(pdu.Payload[6:8])
	byteCount := pdu.Payload[8]
	writeValues := pdu.Payload[9 : 9+byteCount]

	s.protocolPort.InfoX(message.NewEncoded(fmt.Sprintf("TX FC=%d UnitID=%d ReadAddr=0x%X ReadQty=%d WriteAddr=0x%X WriteQty=%d ByteCount=%d, WriteValues=0x%X",
		pdu.FunctionCode, pdu.UnitId, readAddr, readQty, writeAddr, writeQty, byteCount, writeValues)))

	// First, write the values
	for i := range writeQty {
		addr := writeAddr + i
		value := encoding.BytesToUint16(writeValues[i*2 : i*2+2])
		s.registers[addr] = value
		slog.Debug("FC17 Write Register", "unitID", pdu.UnitId, "addr", fmt.Sprintf("0x%04X", addr), "value", fmt.Sprintf("0x%04X", value))

		// Apply write rules
		if targetRegister, targetValue, applied := s.ruleEngine.ApplyWriteRules(addr, value, s.registers); applied {
			s.registers[targetRegister] = targetValue
			s.protocolPort.InfoX(message.NewEncoded(fmt.Sprintf("R1 FC=17 Rule applied UnitID=%d WriteAddress=0x%X NewValue=0x%X", pdu.UnitId, targetRegister, targetValue)))
		}
	}

	// Build read response
	// First byte: byte count (number of data bytes to follow = readQty * 2)
	readByteCount := uint8(readQty * 2)

	// Hardcoded response data
	// 0 => response code for command $01
	// 1 => 4 bytes of data
	// 2..5 => fw version little endian = $00000904 = 0.0.9.4
	responseData := []byte{0x81, 0x04, 0x04, 0x09, 0x00, 0x00}

	// Construct the full response: [byte count] + [data]
	responsePayload := append([]byte{readByteCount}, responseData...)
	s.protocolPort.InfoX(message.NewEncoded(fmt.Sprintf("RX FC=%d UnitID=%d ReadAddr=0x%X ReadQty=%d Payload=0x%X",
		pdu.FunctionCode, pdu.UnitId, readAddr, readQty, responsePayload)))

	res := &PDU{
		UnitId:       pdu.UnitId,
		FunctionCode: pdu.FunctionCode,
		Payload:      responsePayload,
	}
	return res
}
