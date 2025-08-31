package modbus

import (
	"fmt"
	"strconv"
	"strings"
)

type Hex uint16

func NewHex(value string) (*Hex, error) {
	var h = new(Hex)
	if err := h.Set(value); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *Hex) Uint16() uint16 {
	return uint16(*h)
}

func (h *Hex) Set(value string) error {
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
