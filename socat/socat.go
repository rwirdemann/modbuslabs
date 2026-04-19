// Package socat manages a socat subprocess that creates virtual serial
// port pairs for RTU transport testing.
package socat

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

// Start launches socat to create a virtual serial port pair. serverTTY
// is the slave-side TTY (used by slavesim); peerTTY is the client-side
// TTY (used by external tools such as the master command). It waits
// 100ms and verifies that serverTTY exists before returning.
func Start(serverTTY, peerTTY string) (*os.Process, error) {
	path, err := exec.LookPath("socat")
	if err != nil {
		return nil, fmt.Errorf(
			"socat not found: install socat first",
		)
	}

	cmd := exec.Command(
		path,
		fmt.Sprintf("pty,link=%s,raw,echo=0", serverTTY),
		fmt.Sprintf("pty,link=%s,raw,echo=0", peerTTY),
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start socat: %w", err)
	}

	time.Sleep(100 * time.Millisecond)

	if _, err := os.Stat(serverTTY); os.IsNotExist(err) {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("%s doesn't exist", serverTTY)
	}

	return cmd.Process, nil
}
