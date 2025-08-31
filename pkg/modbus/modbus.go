package modbus

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	MBAPHeaderLength = 7
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
// Returns the [PDU] and transaction on success.
func ReadMBAPFrame(conn io.Reader, maxFrameLength int) (*PDU, uint16, error) {

	// read the MBAP header
	rxbuf := make([]byte, MBAPHeaderLength)
	_, err := io.ReadFull(conn, rxbuf)
	if err != nil {
		return nil, 0, err
	}

	// decode the transaction identifier as unique request and response identifier
	txid := binary.BigEndian.Uint16(rxbuf[0:2])

	// decode the protocol identifier
	protocolId := binary.BigEndian.Uint16(rxbuf[2:4])

	// store the source unit id
	unitId := rxbuf[6]

	// determine how many more bytes we need to read
	bytesNeeded := binary.BigEndian.Uint16(rxbuf[4:6])

	// the byte count includes the unit ID field, which we already have
	bytesNeeded--

	// never read more than the max allowed frame length
	if int(bytesNeeded)+MBAPHeaderLength > maxFrameLength {
		return nil, 0, errors.New("protocol error: maxTCPFrameLength exceeded")
	}

	// an MBAP length of 0 is illegal
	if bytesNeeded <= 0 {
		return nil, 0, errors.New("protocol error: length is equal or less 0")
	}

	// read the PDU
	rxbuf = make([]byte, bytesNeeded)
	_, err = io.ReadFull(conn, rxbuf)
	if err != nil {
		return nil, 0, err
	}

	// validate protocol id
	if protocolId != 0x0000 {
		return nil, 0, errors.New("protocol error: invalid protocol id")
	}

	// store unit id, function code and payload in the PDU object
	pdu := &PDU{
		UnitId:       unitId,
		FunctionCode: rxbuf[0],
		Payload:      rxbuf[1:],
	}

	return pdu, txid, nil
}
