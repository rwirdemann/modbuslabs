package modbuslabs

import (
	"fmt"

	"github.com/rwirdemann/modbuslabs/encoding"
)

const (
	FC2ReadDiscreteRegisters       uint8 = 0x02
	FC4ReadInputRegisters          uint8 = 0x04
	FC5WriteSingleCoil             uint8 = 0x05
	FC6WriteSingleRegister         uint8 = 0x06
	FC16WriteMultipleRegisters     uint8 = 0x10
	FC17ReadWriteMultipleRegisters uint8 = 0x17
)

// PDU is a struct to represent a Modbus Protocol Data unit.
type PDU struct {
	UnitID       uint8
	FunctionCode uint8
	Payload      []byte
	IsResponse   bool
}

func (p PDU) String() string {
	if p.IsResponse {
		return fmt.Sprintf(
			"FC=%d UnitID=%d Payload=% X",
			p.FunctionCode, p.UnitID, p.Payload,
		)
	}
	addr := encoding.BytesToUint16(p.Payload[0:2])
	switch p.FunctionCode {
	case FC2ReadDiscreteRegisters, FC4ReadInputRegisters:
		qty := encoding.BytesToUint16(p.Payload[2:4])
		return fmt.Sprintf(
			"FC=%d UnitID=%d Addr=%d Qty=%X",
			p.FunctionCode, p.UnitID, addr, qty,
		)
	case FC5WriteSingleCoil, FC6WriteSingleRegister:
		value := p.Payload[2:4]
		return fmt.Sprintf(
			"FC=%d UnitID=%d Addr=%X Value=% X",
			p.FunctionCode, p.UnitID, addr, value,
		)
	case FC16WriteMultipleRegisters:
		qty := encoding.BytesToUint16(p.Payload[2:4])
		value := p.Payload[5:]
		return fmt.Sprintf(
			"FC=%d UnitID=%d Addr=%X Qty=%d Value=% X",
			p.FunctionCode, p.UnitID, addr, qty, value,
		)
	case FC17ReadWriteMultipleRegisters:
		readQty := encoding.BytesToUint16(p.Payload[2:4])
		writeAddr := encoding.BytesToUint16(p.Payload[4:6])
		writeQty := encoding.BytesToUint16(p.Payload[6:8])
		byteCount := p.Payload[8]
		writeValues := p.Payload[9 : 9+byteCount]
		return fmt.Sprintf(
			"FC=%d UnitID=%d ReadAddr=%d ReadQty=%d WriteAddr=%d WriteQty=%d Value=% X",
			p.FunctionCode, p.UnitID, addr, readQty, writeAddr, writeQty, writeValues,
		)
	}
	return ""
}

// AssembleMBAPFrame turns a PDU into an MBAP frame (MBAP header + PDU) and
// returns it as bytes.
func AssembleMBAPFrame(txnID uint16, p *PDU) []byte {
	// transaction identifier
	payload := encoding.Uint16ToBytes(txnID)

	// protocol identifier (always 0x0000)
	payload = append(payload, 0x00, 0x00)

	// length (covers unit identifier + function code + payload fields)
	payload = append(payload, encoding.Uint16ToBytes(uint16(2+len(p.Payload)))...)

	// unit identifier
	payload = append(payload, p.UnitID)

	// function code
	payload = append(payload, p.FunctionCode)

	// payload
	payload = append(payload, p.Payload...)

	return payload
}
