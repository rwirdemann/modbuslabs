package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/rwirdemann/modbuslabs"
	"github.com/rwirdemann/modbuslabs/tcp"
)

func main() {
	handler, err := tcp.NewHandler("tcp://localhost:5002")
	if err != nil {
		panic(err)
	}
	modbus := modbuslabs.NewBus(handler)
	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	if err := modbus.Start(ctx); err != nil {
		panic(err)
	}
	defer modbus.Stop()

	<-ctx.Done()
}
