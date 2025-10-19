package modbus

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

const (
	MBAPHeaderLength = 7
	MaxFrameLength   = 260

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

// ReadMBAPFrame reads an entire frame (MBAP header + modbus PDU) from the reader. Example:
//
// 00 01 00 00 00 06 01 03 00 01 00 02
// 00 01      - Transaction ID (1)
// 00 00      - Protocol ID (0 = Modbus)
// 00 06      - Length (6 Bytes folgen)
// 01         - Unit ID (Slave 1)
// 03         - Function Code (Read Holding Registers)
// 00 01      - Start Address (Register 1)
// 00 02      - Quantity (2 Register)
//
// Returns the header, [PDU] and transaction id on success.
func ReadMBAPFrame(conn io.Reader) ([]byte, *PDU, uint16, error) {

	// read the MBAP header
	header := make([]byte, MBAPHeaderLength)
	_, err := io.ReadFull(conn, header)
	if err != nil {
		return nil, nil, 0, err
	}

	// decode the transaction identifier as unique request and response identifier
	txid := binary.BigEndian.Uint16(header[0:2])

	// decode the protocol identifier
	protocolId := binary.BigEndian.Uint16(header[2:4])

	// store the source unit id
	unitId := header[6]

	// determine how many more bytes we need to read
	bytesNeeded := binary.BigEndian.Uint16(header[4:6])

	// the byte count includes the unit ID field, which we already have
	bytesNeeded--

	// never read more than the max allowed frame length
	if int(bytesNeeded)+MBAPHeaderLength > MaxFrameLength {
		return nil, nil, 0, errors.New("protocol error: maxFrameLength exceeded")
	}

	// an MBAP length of 0 is illegal
	if bytesNeeded <= 0 {
		return nil, nil, 0, errors.New("protocol error: length is equal or less 0")
	}

	// read the PDU
	rxbuf := make([]byte, bytesNeeded)
	_, err = io.ReadFull(conn, rxbuf)
	if err != nil {
		return nil, nil, 0, err
	}

	// validate protocol id
	if protocolId != 0x0000 {
		return nil, nil, 0, errors.New("protocol error: invalid protocol id")
	}

	// store unit id, function code and payload in the PDU object
	pdu := &PDU{
		UnitId:       unitId,
		FunctionCode: rxbuf[0],
		Payload:      rxbuf[1:],
	}

	return header, pdu, txid, nil
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

func EncodeBools(in []bool) (out []byte) {
	var byteCount uint
	var i uint

	byteCount = uint(len(in)) / 8
	if len(in)%8 != 0 {
		byteCount++
	}

	out = make([]byte, byteCount)
	for i = range uint(len(in)) {
		if in[i] {
			out[i/8] |= (0x01 << (i % 8))
		}
	}

	return
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
