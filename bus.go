package modbuslabs

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/rwirdemann/modbuslabs/config"
	"github.com/rwirdemann/modbuslabs/message"
	"github.com/rwirdemann/modbuslabs/pkg/modbus"
	"github.com/rwirdemann/modbuslabs/rules"
)

type slave struct {
	unitID     uint8
	registers  map[uint16]uint16
	connected  bool
	ruleEngine *rules.Engine
}

// Bus represents a Modbus bus.
type Bus struct {
	handler      []TransportHandler
	protocolPort ProtocolPort
	slaves       map[string]map[uint8]*slave // map[url]map[unitID]slave
	slaveLock    *sync.Mutex
}

// NewBus creates a new Modbus bus.
func NewBus(handler []TransportHandler, protocolPort ProtocolPort) *Bus {
	b := &Bus{
		handler:      handler,
		protocolPort: protocolPort,
		slaves:       make(map[string]map[uint8]*slave),
		slaveLock:    new(sync.Mutex),
	}
	for _, h := range b.handler {
		b.slaves[h.Description()] = make(map[uint8]*slave)
	}
	return b
}

// Start starts the Modbus bus.
func (m *Bus) Start(ctx context.Context) error {
	for _, h := range m.handler {
		if err := h.Start(ctx, m.processPDU); err != nil {
			return err
		}
	}
	return nil
}

// Stop stops the Modbus bus.
func (m *Bus) Stop() error {
	for _, h := range m.handler {
		h.Stop()
	}
	return nil
}

func (b *Bus) findSlave(unitID uint8) (*slave, bool) {
	for _, h := range b.handler {
		if s, exists := b.slaves[h.Description()][unitID]; exists {
			slog.Debug("slave exists", "unitID", unitID)
			return s, true
		}
	}
	slog.Debug("slave does not exist", "slaves", b.slaves)

	return nil, false
}

func (h *Bus) processPDU(pdu modbus.PDU) *modbus.PDU {
	h.slaveLock.Lock()
	defer h.slaveLock.Unlock()
	slave, exists := h.findSlave(pdu.UnitId)
	if !exists || !slave.connected {
		h.protocolPort.Info(fmt.Sprintf("slave %d does not exist or is offline", pdu.UnitId))
		return nil
	}

	addr := modbus.BytesToUint16(pdu.Payload[0:2])
	switch pdu.FunctionCode {
	case modbus.FC2ReadDiscreteInput:
		return h.processFC2(slave, addr, pdu)
	}

	if pdu.FunctionCode == modbus.FC4ReadInputRegisters {
		quantity := modbus.BytesToUint16(pdu.Payload[2:4])
		h.protocolPort.InfoX(message.NewEncoded(fmt.Sprintf("TX FC=%d UnitID=%d Address=0x%X Quantity=%d", pdu.FunctionCode, pdu.UnitId, addr, quantity)))
		byteCount := uint8(quantity * 2)
		res := &modbus.PDU{
			UnitId:       pdu.UnitId,
			FunctionCode: pdu.FunctionCode,
			Payload:      make([]byte, 1+byteCount), // byte count + register values
		}
		res.Payload[0] = byteCount

		// Read values from registers map
		payloadIndex := 1 // Start after byte count
		values := ""
		for i := range quantity {
			currentAddr := addr + i
			var value uint16

			if len(values) > 0 {
				values += ", "
			}
			if regValue, exists := slave.registers[currentAddr]; exists {
				value = regValue
				values += fmt.Sprintf("0x%X => 0x%X", currentAddr, value)
				slog.Debug("FC4 reading from map", "unitID", pdu.UnitId, "addr", currentAddr, "value", value)
			} else {
				slog.Debug("no value for register", "regValue", regValue)
				values += fmt.Sprintf("0x%X => <none>", currentAddr)
			}

			// Write register value as 2 bytes (big endian) at correct position
			copy(res.Payload[payloadIndex:payloadIndex+2], modbus.Uint16ToBytes(value))
			payloadIndex += 2
		}

		h.protocolPort.InfoX(message.NewEncoded(fmt.Sprintf("RX FC=%d UnitID=%d Address=0x%X Values=%s", pdu.FunctionCode, pdu.UnitId, addr, values)))
		return res
	}

	if pdu.FunctionCode == modbus.FC5WriteSingleCoil {
		// FC5 payload format: [coilAddr(2 bytes)][value(2 bytes)]. Value is 0xFF00 for ON, 0x0000 for OFF
		slog.Debug("processPDU", "regAddr", fmt.Sprintf("%X", addr), "pdu", pdu)
		value := modbus.BytesToUint16(pdu.Payload[2:4])

		// Store the coil value (0xFF00 for true, 0x0000 for false)
		slave.registers[addr] = value
		slog.Debug("FC5 Write Single Coil", "unitID", pdu.UnitId, "addr", fmt.Sprintf("%X", addr), "value", fmt.Sprintf("%X", value))

		// FC5 response: echo back the request (coil address + value)
		res := &modbus.PDU{
			UnitId:       pdu.UnitId,
			FunctionCode: pdu.FunctionCode,
			Payload:      pdu.Payload[0:4], // Echo back address and value
		}
		h.protocolPort.Info(fmt.Sprintf("FC=%X UnitID=%d Address=%X Value=%X", pdu.FunctionCode, pdu.UnitId, addr, value))
		return res
	}

	if pdu.FunctionCode == modbus.FC6WriteSingleRegister {
		// FC6 payload format: [regAddr(2 bytes)][value(2 bytes)]
		value := modbus.BytesToUint16(pdu.Payload[2:4])

		// Store the value
		slave.registers[addr] = value
		slog.Debug("FC6 Write Single Register", "unitID", pdu.UnitId, "addr", addr, "value", value)

		// FC6 response: echo back the request (register address + value)
		res := &modbus.PDU{
			UnitId:       pdu.UnitId,
			FunctionCode: pdu.FunctionCode,
			Payload:      pdu.Payload[0:4], // Echo back address and value
		}
		return res
	}

	if pdu.FunctionCode == modbus.FC16WriteMultipleRegisters {
		// FC16 payload format: [startAddr(2 bytes)][quantity(2 bytes)][byteCount(1 byte)][values(N bytes)]
		// addr and quantity already extracted at the beginning
		quantity := modbus.BytesToUint16(pdu.Payload[2:4])
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

		// Write all register values
		valueIndex := 5 // Start after: addr(2) + quantity(2) + byteCount(1)
		values := ""
		for i := range quantity {
			currentAddr := addr + i
			value := modbus.BytesToUint16(pdu.Payload[valueIndex : valueIndex+2])
			slave.registers[currentAddr] = value
			slog.Debug("FC16 Write Register", "unitID", pdu.UnitId, "addr", fmt.Sprintf("%X", currentAddr), "value", fmt.Sprintf("%X", value))
			if len(values) > 0 {
				values += ", "
			}
			values += fmt.Sprintf("0x%X => 0x%X", currentAddr, value)
			valueIndex += 2
		}

		m := message.NewEncoded(fmt.Sprintf("TX FC=%d UnitID=%d Address=0x%04X Quantity=%d ByteCount=%d Values: %s",
			pdu.FunctionCode, pdu.UnitId, addr, quantity, byteCount, values))
		h.protocolPort.InfoX(m)

		// FC16 response: echo back starting address and quantity
		res := &modbus.PDU{
			UnitId:       pdu.UnitId,
			FunctionCode: pdu.FunctionCode,
			Payload:      pdu.Payload[0:4], // Echo back address and quantity
		}
		h.protocolPort.InfoX(message.NewEncoded(fmt.Sprintf("RX FC=%d UnitID=%d Payload=% X", res.FunctionCode, res.UnitId, res.Payload)))
		return res
	}

	return nil
}

func (h *Bus) ConnectSlave(unitID uint8, url string) {
	if _, exists := h.slaves[url][unitID]; !exists {
		h.slaves[url][unitID] = &slave{
			registers:  make(map[uint16]uint16),
			ruleEngine: rules.NewEngine(nil), // No rules
		}
	}
	h.slaves[url][unitID].connected = true
	slog.Debug("slave connected", "unitID", unitID, "url", url)
}

// ConnectSlaveWithConfig connects a slave with configuration including rules
func (h *Bus) ConnectSlaveWithConfig(slaveConfig config.Slave, url string) {
	if _, exists := h.slaves[url][slaveConfig.ID]; !exists {
		ruleEngine := rules.NewEngine(slaveConfig.Rules)
		h.slaves[url][slaveConfig.ID] = &slave{
			unitID:     slaveConfig.ID,
			registers:  make(map[uint16]uint16),
			ruleEngine: ruleEngine,
		}
		slog.Info("Slave connected with rules",
			"unitID", slaveConfig.ID,
			"url", url,
			"ruleCount", len(slaveConfig.Rules))
	}
	h.slaves[url][slaveConfig.ID].connected = true
}

func (h *Bus) DisconnectSlave(unitID uint8) {
	for _, v := range h.slaves {
		if _, exists := v[unitID]; exists {
			v[unitID].connected = false
		}
	}
}

func (h *Bus) Status() string {
	var status string
	status = "Configuration:"
	for i, p := range h.handler {
		status += fmt.Sprintf("\n  Port %d: %s", i, p.Description())
		if len(h.slaves[p.Description()]) == 0 {
			status += "\n    <no slaves connected>"
		}
		for unitID, slave := range h.slaves[p.Description()] {
			connectStatus := "disconnected"
			if slave.connected {
				connectStatus = "connected"
			}
			status += fmt.Sprintf("\n    - Unit %d: %s", unitID, connectStatus)
			status += slave.ruleEngine.Status()
			if len(slave.registers) > 0 {
				for addr, value := range slave.registers {
					status += "\n      Registers:"
					status += fmt.Sprintf("\n      - 0x%X => 0x%X", addr, value)
				}
			}
		}
	}
	return status
}

// Response Payload:  [Byte Count] [Status Byte 1] [Status Byte 2] ... Each
// status byte contains up to 8 coils.
func (h *Bus) processFC2(slave *slave, registerAddr uint16, pdu modbus.PDU) *modbus.PDU {
	quantity := modbus.BytesToUint16(pdu.Payload[2:4])
	h.info(message.NewEncoded(fmt.Sprintf("TX FC=%d UnitID=%d Address=0x%X Quantity=%d", pdu.FunctionCode, pdu.UnitId, registerAddr, quantity)))
	slog.Debug("processPDU", "regAddr", fmt.Sprintf("%X", registerAddr), "quantitiy", quantity, "pdu", pdu)
	var values = make([]bool, quantity)

	// Read values from registers map
	for i := range quantity {
		currentAddr := registerAddr + i
		var value uint16

		if regValue, exists := slave.registers[currentAddr]; exists {
			value = regValue
			slog.Debug("FC2 reading from map", "unitID", pdu.UnitId, "addr", currentAddr, "value", value)
		} else {
			slog.Debug("no value for discrete input", "addr", currentAddr)
		}

		// Apply read rules. The rule is applied after the register value has been read
		// from the store. The read value is the value that is going to be changed after
		// it has been returned to the master. The new value is update in the store.
		if newValue, modified := slave.ruleEngine.ApplyRead(currentAddr, value); modified {
			slave.registers[currentAddr] = newValue
			h.info(message.NewEncoded(fmt.Sprintf("R1 FC=2 Rule=set_value UnitID=%d Address=0x%X NewValue(after read)=0x%X", pdu.UnitId, currentAddr, newValue)))
		}

		// Convert register value to boolean (0x0000 = false, anything else = true)
		// For coils written with FC5, 0xFF00 = true
		values[i] = value != 0x0000
	}

	resCount := len(values)
	res := &modbus.PDU{
		UnitId:       pdu.UnitId,
		FunctionCode: pdu.FunctionCode,
		Payload:      []byte{0},
	}

	// byte count (1 byte for 8 coils)
	res.Payload[0] = uint8(resCount / 8)
	if resCount%8 != 0 {
		res.Payload[0]++
	}
	h.protocolPort.InfoX(message.NewEncoded(fmt.Sprintf("RX FC=%d UnitID=%d Address=0x%X Quantity=%d Values=%v", pdu.FunctionCode, pdu.UnitId, registerAddr, quantity, values)))

	// coil values
	res.Payload = append(res.Payload, modbus.EncodeBools(values)...)
	return res
}

func (b *Bus) info(m message.Message) {
	b.protocolPort.InfoX(m)
}
