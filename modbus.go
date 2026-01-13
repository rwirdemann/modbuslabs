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
	UnitId       uint8
	FunctionCode uint8
	Payload      []byte
}

func (p PDU) String() string {
	return fmt.Sprintf("UnitId:%d FC:%d Payload:% X", p.UnitId, p.FunctionCode, p.Payload)
}

// AssembleMBAPFrame turns a PDU into an MBAP frame (MBAP header + PDU) and returns it as bytes.
func AssembleMBAPFrame(txnId uint16, p *PDU) []byte {
	// transaction identifier
	payload := encoding.Uint16ToBytes(txnId)

	// protocol identifier (always 0x0000)
	payload = append(payload, 0x00, 0x00)

	// length (covers unit identifier + function code + payload fields)
	payload = append(payload, encoding.Uint16ToBytes(uint16(2+len(p.Payload)))...)

	// unit identifier
	payload = append(payload, p.UnitId)

	// function code
	payload = append(payload, p.FunctionCode)

	// payload
	payload = append(payload, p.Payload...)

	return payload
}
