package modbuslabs

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/rwirdemann/modbuslabs/config"
	"github.com/rwirdemann/modbuslabs/encoding"
	"github.com/rwirdemann/modbuslabs/message"
	"github.com/rwirdemann/modbuslabs/rules"
)

// Gateway represents a gateway with modbus devices.
type Gateway struct {
	handler      []TransportHandler
	protocolPort ProtocolPort
	slaves       map[string]map[uint8]*Slave // map[url]map[unitID]slave
	slaveLock    *sync.Mutex
}

// NewGateway creates a new gateway.
func NewGateway(handler []TransportHandler, protocolPort ProtocolPort) *Gateway {
	b := &Gateway{
		handler:      handler,
		protocolPort: protocolPort,
		slaves:       make(map[string]map[uint8]*Slave),
		slaveLock:    new(sync.Mutex),
	}
	for _, h := range b.handler {
		b.slaves[h.Description()] = make(map[uint8]*Slave)
	}
	return b
}

// Start starts the gateway.
func (m *Gateway) Start(ctx context.Context) error {
	for _, h := range m.handler {
		if err := h.Start(ctx, m.processPDU); err != nil {
			return err
		}
	}
	return nil
}

// Stop stops gateway.
func (m *Gateway) Stop() error {
	for _, h := range m.handler {
		h.Stop()
	}
	return nil
}

func (b *Gateway) findSlave(unitID uint8) (*Slave, bool) {
	for _, h := range b.handler {
		if s, exists := b.slaves[h.Description()][unitID]; exists {
			slog.Debug("slave exists", "unitID", unitID)
			return s, true
		}
	}
	slog.Debug("slave does not exist", "slaves", b.slaves)

	return nil, false
}

func (h *Gateway) processPDU(pdu PDU) *PDU {
	h.slaveLock.Lock()
	defer h.slaveLock.Unlock()
	slave, exists := h.findSlave(pdu.UnitId)
	if !exists || !slave.connected {
		h.protocolPort.Info(fmt.Sprintf("slave %d does not exist or is offline", pdu.UnitId))
		return nil
	}

	switch pdu.FunctionCode {
	case FC2ReadDiscreteRegisters, FC6WriteSingleRegister, FC17ReadWriteMultipleRegisters:
		return slave.Process(pdu)
	}

	addr := encoding.BytesToUint16(pdu.Payload[0:2])
	if pdu.FunctionCode == FC4ReadInputRegisters {
		quantity := encoding.BytesToUint16(pdu.Payload[2:4])
		h.protocolPort.InfoX(message.NewEncoded(fmt.Sprintf("TX FC=%d UnitID=%d Address=0x%X Quantity=%d", pdu.FunctionCode, pdu.UnitId, addr, quantity)))
		byteCount := uint8(quantity * 2)
		res := &PDU{
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
			copy(res.Payload[payloadIndex:payloadIndex+2], encoding.Uint16ToBytes(value))
			payloadIndex += 2
		}

		h.protocolPort.InfoX(message.NewEncoded(fmt.Sprintf("RX FC=%d UnitID=%d Address=0x%X Values=%s", pdu.FunctionCode, pdu.UnitId, addr, values)))
		return res
	}

	if pdu.FunctionCode == FC5WriteSingleCoil {
		// FC5 payload format: [coilAddr(2 bytes)][value(2 bytes)]. Value is 0xFF00 for ON, 0x0000 for OFF
		slog.Debug("processPDU", "regAddr", fmt.Sprintf("%X", addr), "pdu", pdu)
		value := encoding.BytesToUint16(pdu.Payload[2:4])

		// Store the coil value (0xFF00 for true, 0x0000 for false)
		slave.registers[addr] = value
		slog.Debug("FC5 Write Single Coil", "unitID", pdu.UnitId, "addr", fmt.Sprintf("%X", addr), "value", fmt.Sprintf("%X", value))

		// FC5 response: echo back the request (coil address + value)
		res := &PDU{
			UnitId:       pdu.UnitId,
			FunctionCode: pdu.FunctionCode,
			Payload:      pdu.Payload[0:4], // Echo back address and value
		}
		h.protocolPort.Info(fmt.Sprintf("FC=%X UnitID=%d Address=%X Value=%X", pdu.FunctionCode, pdu.UnitId, addr, value))
		return res
	}

	if pdu.FunctionCode == FC16WriteMultipleRegisters {
		// FC16 payload format: [startAddr(2 bytes)][quantity(2 bytes)][byteCount(1 byte)][values(N bytes)]
		// addr and quantity already extracted at the beginning
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

		// Write all register values
		valueIndex := 5 // Start after: addr(2) + quantity(2) + byteCount(1)
		values := ""
		for i := range quantity {
			currentAddr := addr + i
			value := encoding.BytesToUint16(pdu.Payload[valueIndex : valueIndex+2])
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
		res := &PDU{
			UnitId:       pdu.UnitId,
			FunctionCode: pdu.FunctionCode,
			Payload:      pdu.Payload[0:4], // Echo back address and quantity
		}
		h.protocolPort.InfoX(message.NewEncoded(fmt.Sprintf("RX FC=%d UnitID=%d Payload=% X", res.FunctionCode, res.UnitId, res.Payload)))
		return res
	}

	return nil
}

func (g *Gateway) ConnectSlave(unitID uint8, url string) error {
	if _, exists := g.slaves[url]; !exists {
		return fmt.Errorf("URl %s not configured", url)
	}

	if _, exists := g.slaves[url][unitID]; exists {
		g.slaves[url][unitID].connected = true
		slog.Debug("slave reconnected", "unitID", unitID, "url", url)
		return nil
	}

	g.slaves[url][unitID] = NewSlave(unitID, true, rules.NewEngine(nil), g.protocolPort)
	slog.Debug("slave connected", "unitID", unitID, "url", url)
	return nil
}

// ConnectSlaveWithConfig connects a slave with configuration including rules
func (h *Gateway) ConnectSlaveWithConfig(slaveConfig config.Slave, url string) {
	if _, exists := h.slaves[url][slaveConfig.ID]; !exists {
		ruleEngine := rules.NewEngine(slaveConfig.Rules)
		h.slaves[url][slaveConfig.ID] = NewSlave(slaveConfig.ID, true, ruleEngine, h.protocolPort)
		slog.Info("Slave connected with rules", "unitID", slaveConfig.ID, "url", url, "ruleCount", len(slaveConfig.Rules))
	}
}

func (h *Gateway) DisconnectSlave(unitID uint8) {
	for _, v := range h.slaves {
		if _, exists := v[unitID]; exists {
			v[unitID].connected = false
		}
	}
}

func (h *Gateway) Status() string {
	var status string
	for i, p := range h.handler {
		status = fmt.Sprintf("%sPort %d: %s", status, i, p.Description())
		if len(h.slaves[p.Description()]) == 0 {
			status += "\n  <no slaves connected>"
		}
		for unitID, slave := range h.slaves[p.Description()] {
			connectStatus := "disconnected"
			if slave.connected {
				connectStatus = "connected"
			}
			status = fmt.Sprintf("%s\n  - Unit %d: %s", status, unitID, connectStatus)
			status += slave.ruleEngine.Status()
			if len(slave.registers) > 0 {
				for addr, value := range slave.registers {
					status += "\n    Registers:"
					status += fmt.Sprintf("\n    - 0x%X => 0x%X", addr, value)
				}
			}
		}
	}
	return status
}
