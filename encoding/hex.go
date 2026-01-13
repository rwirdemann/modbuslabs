package encoding

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

type Hex uint16

func NewHex(value string) (*Hex, error) {
	var h = new(Hex)
	if err := h.set(value); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *Hex) Uint16() uint16 {
	return uint16(*h)
}

func (h *Hex) set(value string) error {
	value = strings.TrimSpace(value)
	addr, err := strconv.ParseUint(value, 0, 64)
	if err != nil {
		return fmt.Errorf("invalid hex address: %v", err)
	}
	*h = Hex(addr)
	return nil
}

func (h *Hex) String() string {
	return fmt.Sprintf("0x%X", uint64(*h))
}

// HexStringToBytes converts a hex string (e.g., "00FF1234") to a byte array.
// The string should not have a "0x" prefix.
func HexStringToBytes(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")

	if len(s)%2 != 0 {
		return nil, fmt.Errorf("hex string must have even length")
	}

	return hex.DecodeString(s)
}
