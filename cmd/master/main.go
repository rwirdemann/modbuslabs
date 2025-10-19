package main

import (
	"encoding/binary"
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
	valueFloat := flag.Float64("float", 0, "the value as float")
	boolValue := flag.Bool("bool", false, "the value as bool")
	transport := flag.String("transport", "tcp", "the modbus mode (tcp|rtu)")
	quantity := flag.Int("quantity", 1, "number of registers to read (for FC4)")
	fc := flag.Int("fc", int(modbus.FC6WriteSingleRegister), "the modbus function code (2|4|5|6|16)")
	flag.Parse()

	addrHex, err := modbus.NewHex(*addr)
	if err != nil {
		log.Fatal(err)
	}

	var handler bmodbus.ClientHandler
	if *transport == "tcp" {
		h := bmodbus.NewTCPClientHandler("localhost:502")
		h.Timeout = 1 * time.Second
		h.SlaveId = 101
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
	switch *fc {
	case int(modbus.FC2ReadDiscreteInput):
		bb, err := client.ReadDiscreteInputs(addrHex.Uint16(), uint16(*quantity))
		if err != nil {
			log.Fatal(err)
		}
		ts := time.Now().Format(time.DateTime)
		fmt.Printf("%s % X\n", ts, bb)

		// Print discrete input values (1 bit per input, packed in bytes)
		fmt.Printf("Discrete input values (%d inputs):\n", *quantity)
		for i := 0; i < *quantity; i++ {
			byteIndex := i / 8
			bitIndex := i % 8
			bitValue := (bb[byteIndex] >> bitIndex) & 0x01
			fmt.Printf("  Input 0x%04X: %d (%v)\n", addrHex.Uint16()+uint16(i), bitValue, bitValue == 1)
		}
	case int(modbus.FC4ReadInputRegisters):
		bb, err := client.ReadInputRegisters(addrHex.Uint16(), uint16(*quantity))
		if err != nil {
			log.Fatal(err)
		}
		ts := time.Now().Format(time.DateTime)
		fmt.Printf("%s % X\n", ts, bb)

		// Print register values (2 bytes per register)
		fmt.Printf("Register values (%d registers):\n", *quantity)
		for i := 0; i < *quantity; i++ {
			regValue := uint16(bb[i*2])<<8 | uint16(bb[i*2+1])
			fmt.Printf("  Register 0x%04X: 0x%04X (%d)\n", addrHex.Uint16()+uint16(i), regValue, regValue)
		}

		// If we read exactly 2 registers, try to decode as float32
		if *quantity == 2 {
			high := uint16(bb[0])<<8 | uint16(bb[1])
			low := uint16(bb[2])<<8 | uint16(bb[3])
			floatValue := modbus.RegistersToFloat32(high, low)
			fmt.Printf("\nFloat32 interpretation: %.6f\n", floatValue)
		}
	case int(modbus.FC5WriteSingleCoil):
		// Convert int value to Modbus coil format: 0xFF00 for ON, 0x0000 for OFF
		var coilValue uint16
		if *boolValue {
			coilValue = 0xFF00
		} else {
			coilValue = 0x0000
		}
		bb, err := client.WriteSingleCoil(addrHex.Uint16(), coilValue)
		if err != nil {
			log.Fatal(err)
		}
		ts := time.Now().Format(time.DateTime)
		fmt.Printf("%s % X\n", ts, bb)
		fmt.Printf("Coil 0x%04X set to %v\n", addrHex.Uint16(), *value != 0)
	case int(modbus.FC6WriteSingleRegister):
		bb, err := client.WriteSingleRegister(addrHex.Uint16(), uint16(*value))
		if err != nil {
			log.Fatal(err)
		}
		ts := time.Now().Format(time.DateTime)
		fmt.Printf("%s % X\n", ts, bb)
	case int(modbus.FC16WriteMultipleRegisters):
		// Convert float64 to float32
		f32 := float32(*valueFloat)

		// Convert float32 to two registers
		high, low := modbus.Float32ToRegisters(f32)
		fmt.Printf("Writing float32 value %.6f as registers: 0x%04X, 0x%04X\n", f32, high, low)

		// Convert register values to bytes (2 registers = 4 bytes)
		valueBytes := make([]byte, 4)
		binary.BigEndian.PutUint16(valueBytes[0:2], high)
		binary.BigEndian.PutUint16(valueBytes[2:4], low)

		// Write 2 registers starting at address
		bb, err := client.WriteMultipleRegisters(addrHex.Uint16(), 2, valueBytes)
		if err != nil {
			log.Fatal(err)
		}
		ts := time.Now().Format(time.DateTime)
		fmt.Printf("%s % X\n", ts, bb)
		fmt.Printf("Successfully wrote float32 to 2 registers starting at 0x%04X\n", addrHex.Uint16())
	default:
		slog.Error("unknown function code", "fc", *fc)
	}
}
