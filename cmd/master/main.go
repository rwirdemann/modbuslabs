package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"time"

	bmodbus "github.com/goburrow/modbus"
	"github.com/rwirdemann/modbuslabs/pkg/modbus"
)

func main() {
	addr := flag.String("address", "0x000", "0x0000 to 0x270F")
	value := flag.Int("value", 0, "the value as int")
	mode := flag.String("mode", "tcp", "the modbus mode (tcp|rtu)")
	fc := flag.Int("fc", int(modbus.FCWriteSingleRegister), "the modbus function code (6)")
	flag.Parse()

	addrHex, err := modbus.NewHex(*addr)
	if err != nil {
		log.Fatal(err)
	}

	var handler bmodbus.ClientHandler
	if *mode == "tcp" {
		h := bmodbus.NewTCPClientHandler("localhost:5002")
		h.Timeout = 1 * time.Second
		h.SlaveId = 101
		err = h.Connect()
		if err != nil {
			log.Fatal(err)
		}
		defer h.Close()
		handler = h
	}
	if *mode == "rtu" {
		h := bmodbus.NewRTUClientHandler("/tmp/virtualcom1")
		h.Timeout = 5 * time.Second
		h.SlaveId = 101
		h.BaudRate = 9600
		h.Parity = "N"
		h.StopBits = 1
		h.DataBits = 8
		err = h.Connect()
		if err != nil {
			log.Fatal(err)
		}
		defer h.Close()
		handler = h
	}

	client := bmodbus.NewClient(handler)
	switch *fc {
	case int(modbus.FCWriteSingleRegister):
		bb, err := client.WriteSingleRegister(addrHex.Uint16(), uint16(*value))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("response: %v", bb)
	default:
		slog.Error("unknown function code", "fc", *fc)
	}
}
