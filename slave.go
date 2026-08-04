package modbuslabs

import (
	"fmt"
	"log/slog"

	"github.com/rwirdemann/modbuslabs/encoding"
	"github.com/rwirdemann/modbuslabs/rules"
)

type Slave struct {
	unitID     uint8
	registers  map[uint16]uint16
	connected  bool
	ruleEngine *rules.Engine
}

func NewSlave(unitID uint8, connected bool, ruleEngine *rules.Engine) *Slave {
	return &Slave{
		unitID:     unitID,
		registers:  make(map[uint16]uint16),
		connected:  connected,
		ruleEngine: ruleEngine,
	}
}

func (s *Slave) Process(pdu PDU) *PDU {
	switch pdu.FunctionCode {
	case FC2ReadDiscreteRegisters:
		return s.processFC2(pdu)
	case FC3ReadHoldingRegisters:
		return s.processFC3(pdu)
	case FC4ReadInputRegisters:
		return s.processFC4(pdu)
	case FC5WriteSingleCoil:
		return s.processFC5(pdu)
	case FC6WriteSingleRegister:
		return s.processFC6(pdu)
	case FC16WriteMultipleRegisters:
		return s.processFC16(pdu)
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
	values := make([]bool, quantity)

	for i := range quantity {
		currentAddr := startAddr + i
		var value uint16
		if regValue, exists := h.registers[currentAddr]; exists {
			value = regValue
			slog.Debug("FC2 reading from map", "unitID", pdu.UnitID, "addr", currentAddr, "value", value)
		} else {
			slog.Debug("no value for discrete input", "addr", currentAddr)
		}
		if targetReg, newValue, modified := h.ruleEngine.ApplyReadRules(
			currentAddr, value,
		); modified {
			h.registers[targetReg] = newValue
		}
		values[i] = value != 0x0000
	}

	resCount := len(values)
	res := &PDU{
		UnitID:       pdu.UnitID,
		FunctionCode: pdu.FunctionCode,
		Payload:      []byte{0},
	}
	res.Payload[0] = uint8(resCount / 8)
	if resCount%8 != 0 {
		res.Payload[0]++
	}

	res.Payload = append(res.Payload, encoding.EncodeBools(values)...)
	return res
}

// processFC3 reads holding registers from the shared register map. It
// mirrors processFC4: same map, same response shape, differing only in
// the function code echoed back.
func (s *Slave) processFC3(pdu PDU) *PDU {
	addr := encoding.BytesToUint16(pdu.Payload[0:2])
	quantity := encoding.BytesToUint16(pdu.Payload[2:4])
	byteCount := uint8(quantity * 2)
	res := &PDU{
		UnitID:       pdu.UnitID,
		FunctionCode: pdu.FunctionCode,
		Payload:      make([]byte, 1+byteCount),
		IsResponse:   true,
	}
	res.Payload[0] = byteCount

	payloadIndex := 1
	for i := range quantity {
		currentAddr := addr + i
		value := s.registers[currentAddr]
		slog.Debug(
			"FC3 read",
			"unitID", pdu.UnitID,
			"addr", fmt.Sprintf("0x%04X", currentAddr),
			"value", fmt.Sprintf("0x%04X", value),
		)
		if targetReg, newValue, modified := s.ruleEngine.ApplyReadRules(
			currentAddr, value,
		); modified {
			s.registers[targetReg] = newValue
		}
		copy(
			res.Payload[payloadIndex:payloadIndex+2],
			encoding.Uint16ToBytes(value),
		)
		payloadIndex += 2
	}
	return res
}

func (s *Slave) processFC4(pdu PDU) *PDU {
	addr := encoding.BytesToUint16(pdu.Payload[0:2])
	quantity := encoding.BytesToUint16(pdu.Payload[2:4])
	byteCount := uint8(quantity * 2)
	res := &PDU{
		UnitID:       pdu.UnitID,
		FunctionCode: pdu.FunctionCode,
		Payload:      make([]byte, 1+byteCount),
		IsResponse:   true,
	}
	res.Payload[0] = byteCount

	payloadIndex := 1
	for i := range quantity {
		currentAddr := addr + i
		value := s.registers[currentAddr]
		slog.Debug(
			"FC4 read",
			"unitID", pdu.UnitID,
			"addr", fmt.Sprintf("0x%04X", currentAddr),
			"value", fmt.Sprintf("0x%04X", value),
		)
		if targetReg, newValue, modified := s.ruleEngine.ApplyReadRules(
			currentAddr, value,
		); modified {
			s.registers[targetReg] = newValue
		}
		copy(
			res.Payload[payloadIndex:payloadIndex+2],
			encoding.Uint16ToBytes(value),
		)
		payloadIndex += 2
	}
	return res
}

func (s *Slave) processFC5(pdu PDU) *PDU {
	addr := encoding.BytesToUint16(pdu.Payload[0:2])
	value := encoding.BytesToUint16(pdu.Payload[2:4])
	s.registers[addr] = value
	return &PDU{
		UnitID:       pdu.UnitID,
		FunctionCode: pdu.FunctionCode,
		Payload:      pdu.Payload[0:4],
	}
}

// FC6 payload format: [regAddr(2 bytes)][value(2 bytes)]
func (s *Slave) processFC6(pdu PDU) *PDU {
	addr := encoding.BytesToUint16(pdu.Payload[0:2])
	value := encoding.BytesToUint16(pdu.Payload[2:4])

	s.registers[addr] = value
	slog.Debug("FC6 Write Single Register", "unitID", pdu.UnitID, "addr", fmt.Sprintf("0x%04X", addr), "value", fmt.Sprintf("0x%04X", value))

	if targetRegister, targetValue, applied := s.ruleEngine.ApplyWriteRules(
		addr, value, s.registers, pdu.Payload,
	); applied {
		s.registers[targetRegister] = targetValue
	}

	return &PDU{
		UnitID:       pdu.UnitID,
		FunctionCode: pdu.FunctionCode,
		Payload:      pdu.Payload[0:4],
		IsResponse:   true,
	}
}

func (s *Slave) processFC16(pdu PDU) *PDU {
	// FC16 payload format: [startAddr(2 bytes)][quantity(2 bytes)][byteCount(1 byte)][values(N bytes)]
	// addr and quantity already extracted at the beginning
	addr := encoding.BytesToUint16(pdu.Payload[0:2])
	quantity := encoding.BytesToUint16(pdu.Payload[2:4])
	slog.Debug("processPDU", "regAddr", fmt.Sprintf("%X", addr), "quantitiy", quantity, "pdu", pdu)
	byteCount := pdu.Payload[4]

	// Validate payload length
	expectedLength := 5 + int(byteCount)
	if len(pdu.Payload) < expectedLength {
		slog.Debug("FC16 invalid payload length", "expected", expectedLength, "got", len(pdu.Payload))
		return nil
	}

	// Validate byte count matches quantity
	if byteCount != uint8(quantity*2) {
		slog.Debug("FC16 byte count mismatch", "expected", quantity*2, "got", byteCount)
		return nil
	}

	valueIndex := 5
	for i := range quantity {
		currentAddr := addr + i
		value := encoding.BytesToUint16(pdu.Payload[valueIndex : valueIndex+2])
		s.registers[currentAddr] = value
		slog.Debug("FC16 Write Register", "unitID", pdu.UnitID, "addr", fmt.Sprintf("%X", currentAddr), "value", fmt.Sprintf("%X", value))
		valueIndex += 2
	}

	s.ruleEngine.ApplyWriteRules(addr, 0, s.registers, pdu.Payload)
	return &PDU{
		UnitID:       pdu.UnitID,
		FunctionCode: pdu.FunctionCode,
		Payload:      pdu.Payload[0:4],
		IsResponse:   true,
	}
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
	readQty := encoding.BytesToUint16(pdu.Payload[2:4])
	writeAddr := encoding.BytesToUint16(pdu.Payload[4:6])
	writeQty := encoding.BytesToUint16(pdu.Payload[6:8])
	byteCount := pdu.Payload[8]
	writeValues := pdu.Payload[9 : 9+byteCount]

	for i := range writeQty {
		addr := writeAddr + i
		value := encoding.BytesToUint16(writeValues[i*2 : i*2+2])
		s.registers[addr] = value
		slog.Debug("FC17 Write Register", "unitID", pdu.UnitID, "addr", fmt.Sprintf("0x%04X", addr), "value", fmt.Sprintf("0x%04X", value))
		if targetRegister, targetValue, applied := s.ruleEngine.ApplyWriteRules(
			addr, value, s.registers, pdu.Payload,
		); applied {
			s.registers[targetRegister] = targetValue
		}
	}

	readByteCount := uint8(readQty * 2)
	responseData := []byte{0x81, 0x04, 0x04, 0x09, 0x00, 0x00}
	responsePayload := append([]byte{readByteCount}, responseData...)

	return &PDU{
		UnitID:       pdu.UnitID,
		FunctionCode: pdu.FunctionCode,
		Payload:      responsePayload,
	}
}
