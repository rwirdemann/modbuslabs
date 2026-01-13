package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"time"

	bmodbus "github.com/goburrow/modbus"
)

func main() {
	slaveID := flag.Int("slave", 101, "the slave id")
	transport := flag.String("transport", "tcp", "the modbus mode (tcp|rtu)")
	url := flag.String("url", "localhost:502", "the url to connect")
	cmd := flag.String("cmd", "read-firmware-version", "read formware version")
	flag.Parse()

	var handler bmodbus.ClientHandler
	var err error
	if *transport == "tcp" {
		h := bmodbus.NewTCPClientHandler(*url)
		h.Timeout = 1 * time.Second
		h.SlaveId = uint8(*slaveID)
		err = h.Connect()
		if err != nil {
			log.Fatal(err)
		}
		defer h.Close()
		handler = h
	}
	if *transport == "rtu" {
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
	switch *cmd {
	case "read-firmware-version":
		readAddrHex := 0xF1FF
		readQty := 3
		writeAddrHex := 0xF1FF
		writeValue := []byte{0x01, 0x00}
		writeQuantity := 1
		bb, err := client.ReadWriteMultipleRegisters(uint16(readAddrHex), uint16(readQty), uint16(writeAddrHex), uint16(writeQuantity), writeValue)
		if err != nil {
			log.Fatal(err)
		}

		if len(bb) >= 6 {
			versionBytes := bb[len(bb)-4:]
			fmt.Printf("Firmware version: %d.%d.%d.%d\n",
				versionBytes[3], versionBytes[2], versionBytes[1], versionBytes[0])
		}

	default:
		slog.Error("unknown function command", "cmd", *cmd)
	}
}
