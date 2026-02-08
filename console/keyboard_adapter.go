package console

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/chzyer/readline"
	"github.com/rwirdemann/modbuslabs"
)

type KeyboardAdapter struct {
	simulator    modbuslabs.ControlPort
	protocolPort modbuslabs.ProtocolPort
}

func NewKeyboardAdapter(slaveSimulator modbuslabs.ControlPort, protocolPort modbuslabs.ProtocolPort) *KeyboardAdapter {
	return &KeyboardAdapter{simulator: slaveSimulator, protocolPort: protocolPort}
}

func (a *KeyboardAdapter) Start(cancel context.CancelFunc) {
	rl, err := readline.New("> ")
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	// Set readline's stdout as the writer for protocol output
	if adapter, ok := a.protocolPort.(*ProtocolAdapter); ok {
		adapter.SetWriter(rl.Stdout())
	}

	a.protocolPort.Println("Enter 'h' for help (use arrow keys for command history)...")

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == io.EOF {
				a.protocolPort.Println("\nTerminating simulator...")
				cancel()
				return
			}
			break
		}

		input := strings.TrimSpace(line)
		parts := strings.Fields(input)
		if len(parts) == 0 {
			continue
		}

		// Always output a separator after user input
		a.protocolPort.ForceSeparator()

		command := parts[0]
		switch command {
		case "quit", "exit", "q":
			a.protocolPort.Println("Terminating simulator...")
			cancel()
			return
		case "status", "s":
			a.protocolPort.Println(a.simulator.Status())
			a.protocolPort.Separator()
		case "mute", "m":
			a.protocolPort.Mute()
			a.protocolPort.Println("Protocol output muted. Type 'u' to unmute.")
		case "toggle", "t":
			a.protocolPort.Toggle()
		case "unmute", "u":
			a.protocolPort.Unmute()
		case "connect", "c":
			if len(parts) < 3 {
				a.protocolPort.Println("Error: connect command requires a unit ID (e.g., 'connect 1 localhost:502')")
				a.protocolPort.Separator()
				continue
			}
			unitID, err := strconv.ParseUint(parts[1], 10, 8)
			if err != nil {
				a.protocolPort.Println(fmt.Sprintf("Error: invalid unit ID '%s', must be a number between 0-255", parts[1]))
				a.protocolPort.Separator()
				continue
			}
			url := parts[2]
			if err := a.simulator.ConnectSlave(uint8(unitID), url); err == nil {
				a.protocolPort.Println(fmt.Sprintf("Connected slave with unit ID %d to %s", unitID, url))
				a.protocolPort.Separator()
			} else {
				a.protocolPort.Println(fmt.Sprintf("Error: %s", err))
				a.protocolPort.Separator()
			}
		case "disconnect", "d":
			if len(parts) < 2 {
				a.protocolPort.Println("Error: disconnect command requires a unit ID (e.g., 'connect 1')")
				a.protocolPort.Separator()
				continue
			}
			unitID, err := strconv.ParseUint(parts[1], 10, 8)
			if err != nil {
				a.protocolPort.Println(fmt.Sprintf("Error: invalid unit ID '%s', must be a number between 0-255", parts[1]))
				a.protocolPort.Separator()
				continue
			}
			a.simulator.DisconnectSlave(uint8(unitID))
			a.protocolPort.Println(fmt.Sprintf("Disconnected slave with unit ID %d", unitID))
			a.protocolPort.Separator()
		case "help", "h":
			a.protocolPort.Println("Commands:")
			a.protocolPort.Println("  quit/exit/q              - Quit simulator")
			a.protocolPort.Println("  status/s                 - Show simulator status")
			a.protocolPort.Println("  mute/m                   - Mute protocol output")
			a.protocolPort.Println("  unmute/u                 - Unmute protocol output")
			a.protocolPort.Println("  connect/c <unitID> <url> - Connect slave")
			a.protocolPort.Println("  disconnect/d <unitID>    - Disconnect slave")
			a.protocolPort.Println("  toggle/t                 - Toggle output format")
			a.protocolPort.Println("  help/h                   - Show help")
			a.protocolPort.Separator()
		default:
			a.protocolPort.Println(fmt.Sprintf("Unknown command: %s (use 'h' for help)", input))
			a.protocolPort.Separator()
		}
	}
}
