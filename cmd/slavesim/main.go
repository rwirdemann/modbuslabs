package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/rwirdemann/modbuslabs"
	"github.com/rwirdemann/modbuslabs/config"
	"github.com/rwirdemann/modbuslabs/console"
	"github.com/rwirdemann/modbuslabs/rtu"
	"github.com/rwirdemann/modbuslabs/tcp"
)

func main() {
	debug := flag.Bool("debug", false, "set log level to debug")
	out := flag.String("out", "console", "the output channel (console)")
	configFile := flag.String("config", "slavesim.toml", "path to TOML configuration file)")
	flag.Parse()

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

	var handler []modbuslabs.TransportHandler
	var slavesToConnect []struct {
		id      uint8
		address string
	}

	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Create transport handlers from config
	for _, t := range cfg.Transports {
		var h modbuslabs.TransportHandler
		var err error

		switch t.Type {
		case "tcp":
			h, err = tcp.NewHandler(fmt.Sprintf("tcp://%s", t.Address), protocolPort)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating TCP handler for %s: %v\n", t.Address, err)
				os.Exit(1)
			}
		case "rtu":
			h = rtu.NewHandler(t.Address, protocolPort)
		default:
			fmt.Fprintf(os.Stderr, "Unknown transport type: %s\n", t.Type)
			os.Exit(1)
		}
		handler = append(handler, h)
	}

	// Collect slaves from config
	for _, s := range cfg.Slaves {
		slavesToConnect = append(slavesToConnect, struct {
			id      uint8
			address string
		}{id: s.ID, address: s.Address})
	}

	slog.Info("Configuration loaded", "transports", len(cfg.Transports), "slaves", len(cfg.Slaves))

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
	for _, s := range slavesToConnect {
		modbus.ConnectSlave(s.id, s.address)
		slog.Debug("Connected slave", "id", s.id, "address", s.address)
	}

	<-ctx.Done()
}
