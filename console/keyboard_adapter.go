package console

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/rwirdemann/modbuslabs"
)

type KeyboardAdapter struct {
	simulator    simulatorPort
	protocolPort modbuslabs.ProtocolPort
}

func NewKeyboardAdapter(slaveSimulator simulatorPort, protocolPort modbuslabs.ProtocolPort) *KeyboardAdapter {
	return &KeyboardAdapter{simulator: slaveSimulator, protocolPort: protocolPort}
}

func (a *KeyboardAdapter) Start(cancel context.CancelFunc) {
	scanner := bufio.NewScanner(os.Stdin)
	a.protocolPort.Println("Enter 'h' followed by <enter> for help...")
	for scanner.Scan() {
		input := scanner.Text()
		parts := strings.Fields(input)
		if len(parts) == 0 {
			continue
		}

		command := parts[0]
		switch command {
		case "quit", "exit", "q":
			a.protocolPort.Println("Terminating simulator...")
			cancel()
			return
		case "status", "s":
			a.protocolPort.Println(a.simulator.Status())
		case "mute", "m":
			a.protocolPort.Mute()
			a.protocolPort.Println("Protocol output muted. Type 'u' to unmute.")
		case "unmute", "u":
			a.protocolPort.Unmute()
		case "connect", "c":
			if len(parts) < 2 {
				a.protocolPort.Println("Error: connect command requires a unit ID (e.g., 'connect 1')")
				continue
			}
			unitID, err := strconv.ParseUint(parts[1], 10, 8)
			if err != nil {
				a.protocolPort.Println(fmt.Sprintf("Error: invalid unit ID '%s', must be a number between 0-255", parts[1]))
				continue
			}
			a.simulator.ConnectSlave(uint8(unitID))
			a.protocolPort.Println(fmt.Sprintf("Connected slave with unit ID %d", unitID))
		case "disconnect", "d":
			if len(parts) < 2 {
				a.protocolPort.Println("Error: disconnect command requires a unit ID (e.g., 'connect 1')")
				continue
			}
			unitID, err := strconv.ParseUint(parts[1], 10, 8)
			if err != nil {
				a.protocolPort.Println(fmt.Sprintf("Error: invalid unit ID '%s', must be a number between 0-255", parts[1]))
				continue
			}
			a.simulator.DisconnectSlave(uint8(unitID))
			a.protocolPort.Println(fmt.Sprintf("Disconnected slave with unit ID %d", unitID))
		case "help", "h":
			a.protocolPort.Println("Commands:")
			a.protocolPort.Println("  quit/exit/q           - Quit simulator")
			a.protocolPort.Println("  status/s              - Show simulator status")
			a.protocolPort.Println("  mute/m                - Mute protocol output")
			a.protocolPort.Println("  unmute/u              - Unmute protocol output")
			a.protocolPort.Println("  connect/c <unitID>    - Connect slave")
			a.protocolPort.Println("  disconnect/d <unitID> - Disconnect slave")
			a.protocolPort.Println("  help/h                - Show help")
		default:
			fmt.Printf("Unknown command: %s (use 'h' for help)\n", input)
		}
	}
}

type simulatorPort interface {
	Status() string
	ConnectSlave(unitID uint8)
	DisconnectSlave(unitID uint8)
}
