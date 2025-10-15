package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/rwirdemann/modbuslabs"
	"github.com/rwirdemann/modbuslabs/console"
	"github.com/rwirdemann/modbuslabs/rtu"
	"github.com/rwirdemann/modbuslabs/tcp"
)

func main() {
	debug := flag.Bool("debug", false, "set log level to debug")
	out := flag.String("out", "console", "the output channel (console)")
	transport := flag.String("transport", "tcp", "transport protocol (tcp|rtu)")
	flag.Parse()

	if *debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	var protocolPort modbuslabs.ProtocolPort
	if *out == "console" {
		protocolPort = console.ProtocolAdapter{}
	} else {
		flag.Usage()
		os.Exit(1)
	}

	var handler modbuslabs.TransportHandler
	switch *transport {
	case "tcp":
		var err error
		handler, err = tcp.NewHandler("tcp://localhost:502", protocolPort)
		if err != nil {
			panic(err)
		}
	case "rtu":
		handler = rtu.NewHandler("/tmp/virtualcom0", protocolPort)
	default:
		flag.Usage()
		os.Exit(1)
	}

	modbus := modbuslabs.NewBus(handler, protocolPort)
	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	if err := modbus.Start(ctx); err != nil {
		panic(err)
	}
	defer modbus.Stop()

	<-ctx.Done()
}
