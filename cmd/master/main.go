package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	bmodbus "github.com/goburrow/modbus"
	"github.com/rwirdemann/modbuslabs/pkg/modbus"
)

func main() {
	addr := flag.String("address", "0x000", "0x0000 to 0x270F")
	value := flag.Int("value", 0, "the value as int")
	flag.Parse()

	addrHex, err := modbus.NewHex(*addr)
	if err != nil {
		log.Fatal(err)
	}

	handler := bmodbus.NewTCPClientHandler("localhost:5002")
	handler.Timeout = 1 * time.Second
	handler.SlaveId = 101

	err = handler.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer handler.Close()

	client := bmodbus.NewClient(handler)
	bb, err := client.WriteSingleRegister(addrHex.Uint16(), uint16(*value))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("response: %v", bb)
}
