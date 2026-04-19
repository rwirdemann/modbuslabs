package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/rwirdemann/modbuslabs"
	"github.com/rwirdemann/modbuslabs/config"
	"github.com/rwirdemann/modbuslabs/console"
	"github.com/rwirdemann/modbuslabs/rtu"
	"github.com/rwirdemann/modbuslabs/socat"
	"github.com/rwirdemann/modbuslabs/tcp"
)

func main() {
	os.Exit(run())
}

func run() int {
	homeDir := getHomeDir()
	defaultConfig := filepath.Join(
		homeDir, ".config", "slavesim", "slavesim.toml",
	)

	debug := flag.Bool("debug", false, "set log level to debug")
	out := flag.String("out", "console", "the output channel (console)")
	configFile := flag.String(
		"config", defaultConfig, "path to TOML configuration file",
	)
	help := flag.Bool("help", false, "Print this help page.")
	flag.Parse()

	if *help {
		flag.Usage()
		return 0
	}

	if *debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	if *configFile == "" {
		flag.Usage()
		return 1
	}

	var protocolPort modbuslabs.ProtocolPort
	if *out == "console" {
		protocolPort = console.NewProtocolAdapter()
	} else {
		flag.Usage()
		return 1
	}

	cfg, err := config.Load(*configFile)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		return 1
	}
	slog.Debug(
		"Configuration loaded",
		"transports", len(cfg.Transports),
		"slaves", len(cfg.Slaves),
	)

	stopSocat, err := socat.StartAll(cfg.Transports)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "socat: %v\n", err)
		return 1
	}
	defer stopSocat()

	handlers, err := buildHandlers(cfg, protocolPort)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	ctx, cancel := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	modbus := modbuslabs.NewGateway(handlers, protocolPort)
	if err := modbus.Start(ctx); err != nil {
		panic(err)
	}
	defer modbus.Stop()

	go console.NewKeyboardAdapter(modbus, protocolPort).Start(cancel)

	for _, s := range cfg.Slaves {
		modbus.ConnectSlaveWithConfig(s, s.Address)
		slog.Debug("Connected slave", "id", s.ID, "address", s.Address)
	}

	<-ctx.Done()
	return 0
}

// buildHandlers creates a TransportHandler for each configured transport.
func buildHandlers(
	cfg *config.Config,
	port modbuslabs.ProtocolPort,
) ([]modbuslabs.TransportHandler, error) {
	var handlers []modbuslabs.TransportHandler
	for _, t := range cfg.Transports {
		switch t.Type {
		case "tcp":
			h, err := tcp.NewHandler(
				fmt.Sprintf("tcp://%s", t.Address), port,
			)
			if err != nil {
				return nil, fmt.Errorf(
					"TCP handler %s: %w", t.Address, err,
				)
			}
			handlers = append(handlers, h)
		case "rtu":
			handlers = append(handlers, rtu.NewHandler(t.Address, port))
		}
	}
	return handlers, nil
}

// getHomeDir returns the home directory, handling sudo correctly. When
// running with sudo, it uses the original user's home directory.
func getHomeDir() string {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		base := "/home"
		if runtime.GOOS == "darwin" {
			base = "/Users"
		}
		return filepath.Join(base, sudoUser)
	}
	if homeDir, err := os.UserHomeDir(); err == nil {
		return homeDir
	}
	return "."
}
