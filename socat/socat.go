// Package socat manages socat subprocesses that create virtual serial
// port pairs for RTU transport testing.
package socat

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/rwirdemann/modbuslabs/config"
)

// StartAll starts a socat process for each RTU transport in cfg. It returns a
// cleanup function that kills all started processes.
func StartAll(transports []config.Transport) (func(), error) {
	var procs []*os.Process
	for _, t := range transports {
		if t.Type != "rtu" {
			continue
		}
		proc, err := start(t.Address, t.PeerAddress)
		if err != nil {
			for _, p := range procs {
				_ = p.Kill()
			}
			return func() {}, err
		}
		procs = append(procs, proc)
	}
	return func() {
		for _, p := range procs {
			_ = p.Kill()
		}
	}, nil
}

// start launches socat to create a virtual serial port pair. serverTTY is the
// slave-side TTY; peerTTY is the client-side TTY. It waits 100ms and verifies
// that serverTTY exists before returning.
func start(serverTTY, peerTTY string) (*os.Process, error) {
	path, err := exec.LookPath("socat")
	if err != nil {
		return nil, fmt.Errorf("socat not found: install socat first")
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
