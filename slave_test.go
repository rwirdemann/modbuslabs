package modbuslabs

import (
	"testing"

	"github.com/rwirdemann/modbuslabs/encoding"
	"github.com/rwirdemann/modbuslabs/rules"
)

// newTestSlave returns a slave with no rules for register-level tests.
func newTestSlave() *Slave {
	return NewSlave(1, true, rules.NewEngine(nil, nil))
}

// readPDU builds an FC3 request PDU for addr and quantity.
func readPDU(fc uint8, addr, quantity uint16) PDU {
	payload := append(
		encoding.Uint16ToBytes(addr),
		encoding.Uint16ToBytes(quantity)...,
	)
	return PDU{UnitID: 1, FunctionCode: fc, Payload: payload}
}

// writePDU builds an FC6 write-single-register request PDU.
func writePDU(addr, value uint16) PDU {
	payload := append(
		encoding.Uint16ToBytes(addr),
		encoding.Uint16ToBytes(value)...,
	)
	return PDU{UnitID: 1, FunctionCode: FC6WriteSingleRegister,
		Payload: payload}
}

func TestFC3ReadsValueWrittenViaFC6(t *testing.T) {
	s := newTestSlave()
	s.Process(writePDU(0x9000, 42))

	res := s.Process(readPDU(FC3ReadHoldingRegisters, 0x9000, 1))

	if got := encoding.BytesToUint16(res.Payload[1:3]); got != 42 {
		t.Fatalf("want 42, got %d", got)
	}
	if got := res.Payload[0]; got != 2 {
		t.Fatalf("want byte count 2, got %d", got)
	}
}

func TestFC3ReadsUnsetRegisterAsZero(t *testing.T) {
	s := newTestSlave()

	res := s.Process(readPDU(FC3ReadHoldingRegisters, 0x1234, 1))

	if got := encoding.BytesToUint16(res.Payload[1:3]); got != 0 {
		t.Fatalf("want 0, got %d", got)
	}
}

func TestFC3ReadsMultipleRegistersInOrder(t *testing.T) {
	s := newTestSlave()
	s.Process(writePDU(0x0000, 10))
	s.Process(writePDU(0x0001, 20))
	s.Process(writePDU(0x0002, 30))

	res := s.Process(readPDU(FC3ReadHoldingRegisters, 0x0000, 3))

	if got := res.Payload[0]; got != 6 {
		t.Fatalf("want byte count 6, got %d", got)
	}
	want := []uint16{10, 20, 30}
	for i, w := range want {
		off := 1 + i*2
		if got := encoding.BytesToUint16(
			res.Payload[off : off+2],
		); got != w {
			t.Fatalf("register %d: want %d, got %d", i, w, got)
		}
	}
}

func TestFC3AndFC4ShareRegisterMap(t *testing.T) {
	s := newTestSlave()
	s.Process(writePDU(0x9000, 99))

	fc3 := s.Process(readPDU(FC3ReadHoldingRegisters, 0x9000, 1))
	fc4 := s.Process(readPDU(FC4ReadInputRegisters, 0x9000, 1))

	v3 := encoding.BytesToUint16(fc3.Payload[1:3])
	v4 := encoding.BytesToUint16(fc4.Payload[1:3])
	if v3 != 99 || v4 != 99 {
		t.Fatalf("want both 99, got fc3=%d fc4=%d", v3, v4)
	}
}
