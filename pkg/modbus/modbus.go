package modbus

import (
	"encoding/binary"
	"fmt"
	"math"
)

const (
	FC2ReadDiscreteInput       uint8 = 0x02
	FC4ReadInputRegisters      uint8 = 0x04
	FC5WriteSingleCoil         uint8 = 0x05
	FC6WriteSingleRegister     uint8 = 0x06
	FC16WriteMultipleRegisters uint8 = 0x10
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
	payload := Uint16ToBytes(txnId)

	// protocol identifier (always 0x0000)
	payload = append(payload, 0x00, 0x00)

	// length (covers unit identifier + function code + payload fields)
	payload = append(payload, Uint16ToBytes(uint16(2+len(p.Payload)))...)

	// unit identifier
	payload = append(payload, p.UnitId)

	// function code
	payload = append(payload, p.FunctionCode)

	// payload
	payload = append(payload, p.Payload...)

	return payload
}

func Uint16ToBytes(in uint16) []byte {
	out := make([]byte, 2)
	binary.BigEndian.PutUint16(out, in)
	return out
}

func BytesToUint16(in []byte) uint16 {
	return binary.BigEndian.Uint16(in)
}

// EncodeBools converts a boolean slice into a byte slice where each byte
// contains up to 8 boolean values packed as bits. The encoding uses LSB-first
// bit ordering, where the first boolean maps to bit 0 (least significant bit)
// of the first byte. This encoding is commonly used in Modbus for representing
// coil states.
//
// Example: []bool{true, false, true} -> []byte{0x05} (binary: 00000101)
func EncodeBools(in []bool) []byte {
	var i uint

	byteCount := uint(len(in)) / 8
	if len(in)%8 != 0 {
		byteCount++
	}

	out := make([]byte, byteCount)
	for i = range uint(len(in)) {
		if in[i] {
			out[i/8] |= (0x01 << (i % 8))
		}
	}

	return out
}

// Float32ToRegisters converts a float32 value to two uint16 registers (big endian).
func Float32ToRegisters(f float32) (uint16, uint16) {
	bits := math.Float32bits(f)
	high := uint16(bits >> 16)
	low := uint16(bits & 0xFFFF)
	return high, low
}

// RegistersToFloat32 converts two uint16 registers to a float32 value (big endian).
func RegistersToFloat32(high, low uint16) float32 {
	bits := (uint32(high) << 16) | uint32(low)
	return math.Float32frombits(bits)
}
