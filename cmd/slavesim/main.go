package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/rwirdemann/modbuslabs"
	"github.com/rwirdemann/modbuslabs/config"
	"github.com/rwirdemann/modbuslabs/console"
	"github.com/rwirdemann/modbuslabs/rtu"
	"github.com/rwirdemann/modbuslabs/tcp"
)

// getHomeDir returns the home directory, handling sudo correctly. When running
// with sudo, it tries to get the original user's home directory.
func getHomeDir() string {
	// First check if SUDO_USER is set (program running with sudo)
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		// Construct home directory from SUDO_USER
		return filepath.Join("/home", sudoUser)
	}

	// Try to get home directory from os package
	if homeDir, err := os.UserHomeDir(); err == nil {
		return homeDir
	}

	// Last resort fallback
	return "."
}

func main() {
	homeDir := getHomeDir()
	defaultConfig := filepath.Join(homeDir, ".config", "slavesim", "slavesim.toml")

	debug := flag.Bool("debug", false, "set log level to debug")
	out := flag.String("out", "console", "the output channel (console)")
	configFile := flag.String("config", defaultConfig, "path to TOML configuration file")
	help := flag.Bool("help", false, "Print this help page.")
	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	if *debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	if *configFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	var protocolPort modbuslabs.ProtocolPort
	if *out == "console" {
		protocolPort = console.NewProtocolAdapter()
	} else {
		flag.Usage()
		os.Exit(1)
	}

	cfg, err := config.Load(*configFile)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	slog.Info("Configuration loaded", "transports", len(cfg.Transports), "slaves", len(cfg.Slaves))

	// Create transport handlers from config
	var handler []modbuslabs.TransportHandler
	for _, t := range cfg.Transports {
		var h modbuslabs.TransportHandler
		var err error

		switch t.Type {
		case "tcp":
			h, err = tcp.NewHandler(fmt.Sprintf("tcp://%s", t.Address), protocolPort)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error creating TCP handler for %s: %v\n", t.Address, err)
				os.Exit(1)
			}
		case "rtu":
			h = rtu.NewHandler(t.Address, protocolPort)
		default:
			_, _ = fmt.Fprintf(os.Stderr, "Unknown transport type: %s\n", t.Type)
			os.Exit(1)
		}
		handler = append(handler, h)
	}

	modbus := modbuslabs.NewBus(handler, protocolPort)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := modbus.Start(ctx); err != nil {
		panic(err)
	}
	defer modbus.Stop()

	driver := console.NewKeyboardAdapter(modbus, protocolPort)
	go driver.Start(cancel)

	// Connect all configured slaves
	for _, s := range cfg.Slaves {
		modbus.ConnectSlaveWithConfig(s, s.Address)
		slog.Info("Connected slave", "id", s.ID, "address", s.Address)
	}

	<-ctx.Done()
}
