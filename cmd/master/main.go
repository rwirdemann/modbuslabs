package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	bmodbus "github.com/goburrow/modbus"
	"github.com/rwirdemann/modbuslabs/encoding"
)

func connect(transport, url string, slaveID int) (bmodbus.Client, func()) {
	switch transport {
	case "tcp":
		h := bmodbus.NewTCPClientHandler(url)
		h.Timeout = 1 * time.Second
		h.SlaveId = uint8(slaveID)
		if err := h.Connect(); err != nil {
			log.Fatal(err)
		}
		return bmodbus.NewClient(h), func() { h.Close() }
	case "rtu":
		h := bmodbus.NewRTUClientHandler("/tmp/ttyV1")
		h.Timeout = 5 * time.Second
		h.SlaveId = 101
		h.BaudRate = 9600
		h.Parity = "N"
		h.StopBits = 1
		h.DataBits = 8
		if err := h.Connect(); err != nil {
			log.Fatal(err)
		}
		return bmodbus.NewClient(h), func() { h.Close() }
	default:
		log.Fatalf("unknown transport: %s", transport)
		return nil, nil
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: master <subcommand> [flags]

Subcommands:
  fc2   Read Discrete Inputs
  fc4   Read Input Registers
  fc5   Write Single Coil
  fc6   Write Single Register
  fc16  Write Multiple Registers
  fc17  Read/Write Multiple Registers

Run 'master <subcommand> -h' for subcommand-specific flags.

Example:
  master fc16 -addr 0x0100 -value 65536 -quantity 2 -transport tcp -url localhost:502 -slave 101`)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "help", "-h", "--help":
		printUsage()
		os.Exit(0)

	case "fc2":
		cmd := flag.NewFlagSet("fc2", flag.ExitOnError)
		addr := cmd.String("addr", "0x000", "0x0000 to 0x270F")
		transport := cmd.String("transport", "tcp", "tcp|rtu")
		slaveID := cmd.Int("slave", 101, "slave id")
		url := cmd.String("url", "localhost:502", "url to connect")
		quantity := cmd.Int("quantity", 1, "number of discrete inputs to read")
		cmd.Parse(os.Args[2:])

		addrHex, err := encoding.NewHex(*addr)
		if err != nil {
			log.Fatal(err)
		}
		client, cleanup := connect(*transport, *url, *slaveID)
		defer cleanup()

		bb, err := client.ReadDiscreteInputs(addrHex.Uint16(), uint16(*quantity))
		if err != nil {
			log.Fatal(err)
		}
		ts := time.Now().Format(time.DateTime)
		fmt.Printf("%s % X\n", ts, bb)
		fmt.Printf("Discrete input values (%d inputs):\n", *quantity)
		for i := 0; i < *quantity; i++ {
			byteIndex := i / 8
			bitIndex := i % 8
			bitValue := (bb[byteIndex] >> bitIndex) & 0x01
			fmt.Printf("  Input 0x%04X: %d (%v)\n", addrHex.Uint16()+uint16(i), bitValue, bitValue == 1)
		}

	case "fc4":
		cmd := flag.NewFlagSet("fc4", flag.ExitOnError)
		addr := cmd.String("addr", "0x000", "0x0000 to 0x270F")
		transport := cmd.String("transport", "tcp", "tcp|rtu")
		slaveID := cmd.Int("slave", 101, "slave id")
		url := cmd.String("url", "localhost:502", "url to connect")
		quantity := cmd.Int("quantity", 1, "number of registers to read")
		typ := cmd.String("type", "uint16", "interpretation: uint16|int16|uint32|int32|float32")
		cmd.Parse(os.Args[2:])

		addrHex, err := encoding.NewHex(*addr)
		if err != nil {
			log.Fatal(err)
		}

		readQty := *quantity
		if *typ == "uint32" || *typ == "int32" || *typ == "float32" {
			readQty = 2
		}

		client, cleanup := connect(*transport, *url, *slaveID)
		defer cleanup()

		bb, err := client.ReadInputRegisters(addrHex.Uint16(), uint16(readQty))
		if err != nil {
			log.Fatal(err)
		}

		switch *typ {
		case "uint16":
			for i := 0; i < readQty; i++ {
				fmt.Println(uint16(bb[i*2])<<8 | uint16(bb[i*2+1]))
			}
		case "int16":
			for i := 0; i < readQty; i++ {
				fmt.Println(int16(uint16(bb[i*2])<<8 | uint16(bb[i*2+1])))
			}
		case "uint32":
			fmt.Println(uint32(bb[0])<<24 | uint32(bb[1])<<16 | uint32(bb[2])<<8 | uint32(bb[3]))
		case "int32":
			fmt.Println(int32(uint32(bb[0])<<24 | uint32(bb[1])<<16 | uint32(bb[2])<<8 | uint32(bb[3])))
		case "float32":
			high := uint16(bb[0])<<8 | uint16(bb[1])
			low := uint16(bb[2])<<8 | uint16(bb[3])
			fmt.Printf("%.6f\n", encoding.RegistersToFloat32(high, low))
		default:
			slog.Error("unknown type", "type", *typ)
			os.Exit(1)
		}

	case "fc5":
		cmd := flag.NewFlagSet("fc5", flag.ExitOnError)
		addr := cmd.String("addr", "0x000", "0x0000 to 0x270F")
		transport := cmd.String("transport", "tcp", "tcp|rtu")
		slaveID := cmd.Int("slave", 101, "slave id")
		url := cmd.String("url", "localhost:502", "url to connect")
		value := cmd.String("value", "", "true or false")
		cmd.Parse(os.Args[2:])

		addrHex, err := encoding.NewHex(*addr)
		if err != nil {
			log.Fatal(err)
		}
		client, cleanup := connect(*transport, *url, *slaveID)
		defer cleanup()

		var coilValue uint16
		if *value == "true" {
			coilValue = 0xFF00
		}
		bb, err := client.WriteSingleCoil(addrHex.Uint16(), coilValue)
		if err != nil {
			log.Fatal(err)
		}
		ts := time.Now().Format(time.DateTime)
		fmt.Printf("%s % X\n", ts, bb)
		fmt.Printf("Coil 0x%04X set to %s\n", addrHex.Uint16(), *value)

	case "fc6":
		cmd := flag.NewFlagSet("fc6", flag.ExitOnError)
		addr := cmd.String("addr", "0x000", "0x0000 to 0x270F")
		transport := cmd.String("transport", "tcp", "tcp|rtu")
		slaveID := cmd.Int("slave", 101, "slave id")
		url := cmd.String("url", "localhost:502", "url to connect")
		value := cmd.String("value", "", "uint16 value")
		cmd.Parse(os.Args[2:])

		addrHex, err := encoding.NewHex(*addr)
		if err != nil {
			log.Fatal(err)
		}
		i, err := strconv.ParseUint(*value, 10, 16)
		if err != nil {
			slog.Error("invalid uint16 value", "err", err)
		}
		client, cleanup := connect(*transport, *url, *slaveID)
		defer cleanup()

		bb, err := client.WriteSingleRegister(addrHex.Uint16(), uint16(i))
		if err != nil {
			log.Fatal(err)
		}
		ts := time.Now().Format(time.DateTime)
		fmt.Printf("%s % X\n", ts, bb)
		fmt.Printf("Register 0x%04X set to %d\n", addrHex.Uint16(), i)

	case "fc16":
		cmd := flag.NewFlagSet("fc16", flag.ExitOnError)
		addr := cmd.String("addr", "0x000", "0x0000 to 0x270F")
		transport := cmd.String("transport", "tcp", "tcp|rtu")
		slaveID := cmd.Int("slave", 101, "slave id")
		url := cmd.String("url", "localhost:502", "url to connect")
		value := cmd.String("value", "", "float32, uint16, or integer value")
		quantity := cmd.Int("quantity", 1, "number of registers to write integer value across")
		cmd.Parse(os.Args[2:])

		addrHex, err := encoding.NewHex(*addr)
		if err != nil {
			log.Fatal(err)
		}
		client, cleanup := connect(*transport, *url, *slaveID)
		defer cleanup()

		if strings.Contains(*value, ".") {
			f, err := strconv.ParseFloat(*value, 32)
			if err != nil {
				slog.Error("invalid float value", "err", err)
			}
			high, low := encoding.Float32ToRegisters(float32(f))
			fmt.Printf("Writing float32 value %.6f as registers: 0x%04X, 0x%04X\n", f, high, low)
			valueBytes := make([]byte, 4)
			binary.BigEndian.PutUint16(valueBytes[0:2], high)
			binary.BigEndian.PutUint16(valueBytes[2:4], low)
			bb, err := client.WriteMultipleRegisters(addrHex.Uint16(), 2, valueBytes)
			if err != nil {
				log.Fatal(err)
			}
			ts := time.Now().Format(time.DateTime)
			fmt.Printf("%s % X\n", ts, bb)
			fmt.Printf("Successfully wrote float32 to 2 registers starting at 0x%04X\n", addrHex.Uint16())
		} else if *quantity > 1 {
			n, err := strconv.ParseUint(*value, 10, 64)
			if err != nil {
				slog.Error("invalid integer value", "err", err)
				log.Fatal(err)
			}
			valueBytes := make([]byte, *quantity*2)
			for i := *quantity - 1; i >= 0; i-- {
				binary.BigEndian.PutUint16(valueBytes[i*2:i*2+2], uint16(n&0xFFFF))
				n >>= 16
			}
			bb, err := client.WriteMultipleRegisters(addrHex.Uint16(), uint16(*quantity), valueBytes)
			if err != nil {
				log.Fatal(err)
			}
			ts := time.Now().Format(time.DateTime)
			fmt.Printf("%s % X\n", ts, bb)
			fmt.Printf("Successfully wrote %s to %d registers starting at 0x%04X\n", *value, *quantity, addrHex.Uint16())
		} else {
			n, _ := strconv.ParseUint(*value, 10, 16)
			fmt.Printf("Writing uint16 %d to register: 0x%04X\n", n, addrHex.Uint16())
			valueBytes := make([]byte, 2)
			binary.BigEndian.PutUint16(valueBytes[0:2], uint16(n))
			bb, err := client.WriteMultipleRegisters(addrHex.Uint16(), 1, valueBytes)
			if err != nil {
				log.Fatal(err)
			}
			ts := time.Now().Format(time.DateTime)
			fmt.Printf("%s % X\n", ts, bb)
			fmt.Printf("Successfully wrote uint16 to register 0x%04X\n", addrHex.Uint16())
		}

	// Sample FC17 Request:
	//   go run cmd/master/main.go fc17 \
	//       -addr 0xF1FF \
	//       -write-address 0xF1FF \
	//       -quantity 3 \
	//       -value 0100 \
	//       -slave 101 \
	//       -url localhost:502
	case "fc17":
		cmd := flag.NewFlagSet("fc17", flag.ExitOnError)
		addr := cmd.String("addr", "0x000", "0x0000 to 0x270F")
		transport := cmd.String("transport", "tcp", "tcp|rtu")
		slaveID := cmd.Int("slave", 101, "slave id")
		url := cmd.String("url", "localhost:502", "url to connect")
		quantity := cmd.Int("quantity", 1, "number of registers to read")
		value := cmd.String("value", "", "hex string to write")
		writeAddr := cmd.String("write-address", "0x000", "write address")
		cmd.Parse(os.Args[2:])

		addrHex, err := encoding.NewHex(*addr)
		if err != nil {
			log.Fatal(err)
		}
		writeAddrHex, err := encoding.NewHex(*writeAddr)
		if err != nil {
			log.Fatal(err)
		}
		valueBytes, err := encoding.HexStringToBytes(*value)
		if err != nil {
			slog.Error("invalid hex value", "err", err, "value", *value)
			log.Fatal(err)
		}
		writeQuantity := len(valueBytes) / 2
		if len(valueBytes)%2 != 0 {
			slog.Error("hex value must have even number of characters (2 bytes per register)")
			log.Fatal("invalid hex value length")
		}

		client, cleanup := connect(*transport, *url, *slaveID)
		defer cleanup()

		fmt.Printf("read quantity: %d\n", uint16(*quantity))
		bb, err := client.ReadWriteMultipleRegisters(addrHex.Uint16(), uint16(*quantity), writeAddrHex.Uint16(), uint16(writeQuantity), valueBytes)
		if err != nil {
			log.Fatal(err)
		}
		ts := time.Now().Format(time.DateTime)
		fmt.Printf("%s % X\n", ts, bb)
		fmt.Printf("Read %d registers from 0x%04X:\n", *quantity, addrHex.Uint16())
		for i := 0; i < *quantity; i++ {
			regValue := uint16(bb[i*2])<<8 | uint16(bb[i*2+1])
			fmt.Printf("  Register 0x%04X: %d (0x%04X)\n", addrHex.Uint16()+uint16(i), regValue, regValue)
		}
		fmt.Printf("Wrote %d registers to 0x%04X with value: %s\n", writeQuantity, writeAddrHex.Uint16(), *value)

	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\nUsage: master <fc2|fc4|fc5|fc6|fc16|fc17> [flags]\n", os.Args[1])
		os.Exit(1)
	}
}
