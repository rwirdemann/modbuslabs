# Modbuslabs

## VirtualONS

VirtualONS is a cross-platform Modbus slave simulator. Slavesim serves the development of Modbus master applications without requiring actual Modbus devices. Slavesim simulates up to two buses, on which multiple Modbus slaves can be connected. Slaves can be connected and disconnected independently of each other, so that master applications can be developed for handling fragile connections. Each slave manages its own register tables, which can be written to and read from by the master.

## Configuration

Slavesim searches for its configurtion file slavesim.toml in $HOME/.config/slavesim. See [slavesim.example.toml](slavesim.example.toml) for avaialble settings.

## Supported Modbus Functions

- FC2: Read discrete registers

### Design

![virtualons](docs/core-design.drawio.png)

### Usage

## Modbus RTU

When a transport is configured with `type = "rtu"`, slavesim automatically
launches a `socat` process to create a virtual serial port pair. The two
TTY paths in the config have distinct roles:

- `address` — the slave-side TTY that slavesim's RTU handler listens on
- `peer_address` — the client-side TTY that the master or any other tool
  connects to

```toml
[[transport]]
type        = "rtu"
address     = "/tmp/ttyV0"
peer_address = "/tmp/ttyV1"
```

slavesim owns the lifecycle of the socat process: it is started before the
gateway comes up and killed when slavesim exits. No manual socat setup is
required.

#### Read or write data

```bash
# Write coil
go run cmd/master/main.go --value=true --address=0x7E33 --fc=5 --transport=tcp
```

```bash
# Write float32
go run cmd/master/main.go --value=12.33 --address=0x9000 --fc=16 --transport=tcp
```

```bash
# Read float32
go run cmd/master/main.go --address 0x9000 --fc=4 --quantity=2 --transport=tcp
```

## Note about port forwarding
Slavesim runs on tcp:502 and 503 (see default config). If you want to run the simulator without sudo, change the ports in your config to non-privileged ones like 5502. The following macOS command forwards 502 traffic to 5502 if you still want to be able to serve masters connecting via 502.

```
echo "rdr pass on lo0 inet proto tcp from any to any port 502 -> 127.0.0.1 port 5502" | sudo pfctl -ef -
```
