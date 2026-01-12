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
