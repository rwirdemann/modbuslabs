package plugins

import (
	"fmt"
	"log/slog"
)

// ReceivePackage accumulates register values written across consecutive
// FC6 writes, modeling a multi-chunk firmware upload.
type ReceivePackage struct {
	chunks []uint16
}

// Execute appends value to the accumulated chunk list and logs progress.
func (r *ReceivePackage) Execute(
	register, value uint16,
	_ map[uint16]uint16,
) error {
	r.chunks = append(r.chunks, value)
	slog.Info("receive_package",
		"register", fmt.Sprintf("0x%04X", register),
		"value", fmt.Sprintf("0x%04X", value),
		"total_chunks", len(r.chunks),
	)
	return nil
}
