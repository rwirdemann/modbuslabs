package modbuslabs

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/rwirdemann/modbuslabs/config"
	"github.com/rwirdemann/modbuslabs/encoding"
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
	case FC2ReadDiscreteRegisters,
		FC4ReadInputRegisters,
		FC6WriteSingleRegister,
		FC16WriteMultipleRegisters,
		FC17ReadWriteMultipleRegisters:
		return slave.Process(pdu)
	}

	addr := encoding.BytesToUint16(pdu.Payload[0:2])

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

	g.slaves[url][unitID] = NewSlave(
		unitID, true, rules.NewEngine(nil, nil), g.protocolPort,
	)
	slog.Debug("slave connected", "unitID", unitID, "url", url)
	return nil
}

// ConnectSlaveWithConfig connects a slave with configuration and plugin
// registry. Pass a nil registry when no plugins are used.
func (h *Gateway) ConnectSlaveWithConfig(
	slaveConfig config.Slave,
	url string,
	registry rules.Registry,
) {
	if _, exists := h.slaves[url][slaveConfig.ID]; !exists {
		ruleEngine := rules.NewEngine(slaveConfig.Rules, registry)
		h.slaves[url][slaveConfig.ID] = NewSlave(
			slaveConfig.ID, true, ruleEngine, h.protocolPort,
		)
		slog.Debug(
			"Slave connected with rules",
			"unitID", slaveConfig.ID,
			"url", url,
			"ruleCount", len(slaveConfig.Rules),
		)
	}
}

func (h *Gateway) DisconnectSlave(unitID uint8) {
	for _, v := range h.slaves {
		if _, exists := v[unitID]; exists {
			v[unitID].connected = false
		}
	}
}

// WriteRegister writes one or more uint16 values to consecutive registers
// on the slave identified by unitID, starting at addr.
func (g *Gateway) WriteRegister(
	unitID uint8,
	addr uint16,
	values []uint16,
) error {
	g.slaveLock.Lock()
	defer g.slaveLock.Unlock()

	slave, exists := g.findSlave(unitID)
	if !exists || !slave.connected {
		return fmt.Errorf("slave %d not found", unitID)
	}
	for i, v := range values {
		slave.registers[addr+uint16(i)] = v
	}
	return nil
}

// Reset sets all registers of the slave identified by unitID to zero.
func (g *Gateway) Reset(unitID uint8) error {
	g.slaveLock.Lock()
	defer g.slaveLock.Unlock()

	slave, exists := g.findSlave(unitID)
	if !exists || !slave.connected {
		return fmt.Errorf("slave %d not found", unitID)
	}
	for addr := range slave.registers {
		slave.registers[addr] = 0
	}
	return nil
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
