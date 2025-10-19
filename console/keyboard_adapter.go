package console

import (
	"bufio"
	"context"
	"fmt"
	"os"
)

type KeyboardAdapter struct {
	simulator simulatorPort
}

func NewKeyboardAdapter(slaveSimulator simulatorPort) *KeyboardAdapter {
	return &KeyboardAdapter{simulator: slaveSimulator}
}

func (a *KeyboardAdapter) Start(cancel context.CancelFunc) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Enter 'h' followed by <enter> for help...")
	for scanner.Scan() {
		input := scanner.Text()
		switch input {
		case "quit", "exit", "q":
			fmt.Println("Terminating simulator...")
			cancel()
			return
		case "status", "s":
			fmt.Println(a.simulator.Status())
		case "help", "h":
			fmt.Println("Commands:")
			fmt.Println("  quit/exit/q - Quit simulator")
			fmt.Println("  status/s    - Show simulator status")
			fmt.Println("  help        - Show help")
		default:
			fmt.Printf("Unknown command: %s (use 'h' for help)\n", input)
		}
	}
}

type simulatorPort interface {
	Status() string
}
