package plugins

type ReceivePackage struct {
	pu uint32
}

func (r *ReceivePackage) Execute(
	register, value uint16,
	registers map[uint16]uint16,
	payload []byte,
) error {
	byteCount := payload[4]
	if byteCount == 246 {
		r.pu += uint32(byteCount - 4)
		addrHigh := uint16(r.pu >> 16)
		addrLow := uint16(r.pu & 0xFFFF)
		registers[0xA669] = addrHigh
		registers[0xA66A] = addrLow
		registers[0xA668] = 0x1000
	} else {
		registers[0xA668] = 0x1200
	}

	return nil
}
