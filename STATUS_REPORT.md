# Status Report: modbuslabs / slavesim

*As of 2026-08-03*

## Architecture

Clean, small layered architecture (~2,350 LOC of Go):

```
TransportHandler (tcp/rtu) → Gateway → Slave → RuleEngine/Plugins
```

- **`TransportHandler`** interface (`transport_handler.go`) with two implementations: `tcp/handler.go` (MBAP framing over `net.Listener`) and `rtu/handler.go` (serial interface via `goburrow/serial`, custom CRC16, manual framing).
- **`Gateway`** (`gateway.go`) dispatches PDUs to the matching `Slave` instance by unit ID, manages Connect/Disconnect/Reset/WriteRegister, and is the only place with a mutex (`slaveLock`).
- **`Slave`** (`slave.go`) holds the register map (`map[uint16]uint16`) and the FC dispatch logic.
- **`rules.Engine`** (`rules/engine.go`) allows TOML-configurable behavior (`set_value`, `increment`, `write_register`, plugins) on read/write of specific registers — cleanly separated from the core protocol.
- **`plugins`**: an extension point for stateful Go objects (e.g. `ReceivePackage`, which simulates firmware uploads across multiple writes).
- For RTU, slavesim itself launches `socat` to create virtual TTY pairs (`socat/socat.go`) — convenient, but an external process dependency.

The Transport/Gateway/Slave/Rules separation is well thought out and extensible. For a simulator project, this is appropriately sized — no over-engineering.

## Is the Modbus protocol fully implemented? — No

Only these are supported:

| FC | Function |
|----|----------|
| 2  | Read Discrete Inputs |
| 4  | Read Input Registers |
| 5  | Write Single Coil |
| 6  | Write Single Register |
| 16 | Write Multiple Registers |
| 17 | Read/Write Multiple Registers (only partially, see below) |

**Missing — and these are the most important gaps:**

1. **FC3 (Read Holding Registers) is missing entirely.** This is the most common read function in real Modbus masters. FC6/FC16 write into the same register map, but it can only be read back via FC4 (Input Registers) — a master that writes via FC6/16 and reads back via FC3 gets no response at all.
2. **FC1 (Read Coils) and FC15 (Write Multiple Coils)** are also missing.
3. **No Modbus exception responses.** On an unknown function code, invalid unit ID, or malformed payload (e.g. the FC16 length check at `slave.go:163`), the code simply returns `nil` — the master gets no response at all and times out, instead of receiving a proper exception (Illegal Function / Illegal Data Address / Illegal Data Value, FC | 0x80).
4. **FC17 is a stub, not a generic implementation:** the read portion ignores the actual register map and returns a hardcoded response payload (`0x81 0x04 0x04 0x09 0x00 0x00`, see `slave.go:232`). This looks like a device-specific hack for one particular device, not real FC17 support.

Bottom line: core functionality is missing for the most common test scenarios (reading coils, reading holding registers). The feature set looks like it grew on demand ("whatever the current master needed"), not as a complete protocol implementation.

## RTU and TCP — both supported, but with different robustness levels

- **TCP** (`tcp/handler.go`): MBAP header parsed correctly, length and protocol-ID validation present, clean EOF handling. Solid.
- **RTU** (`rtu/handler.go`): CRC16 implemented and checked in-house — fundamentally correct. But:
  - No real frame timing (Modbus RTU is supposed to detect frame end via a 3.5-character inter-frame silence); here it's a single `Read()` into a 256-byte buffer, hoping a whole request lands in one read. This would break on fragmented/split serial reads (a real possibility with USB-serial adapters).
  - Baud rate is **hardcoded to 9600** (`rtu/handler.go:29`) and not configurable.
  - On a CRC mismatch, the message is simply dropped (no exception response), consistent with point 3 above.

## Robustness — moderate, with clear weak spots

**Strengths:**
- Locking in the gateway is correct (`slaveLock` guards all register access).
- Configuration validation is solid (`config.go` checks transport types, rule consistency, etc.).
- TCP disconnects are detected and handled cleanly.

**Weaknesses:**
- No exception responses (see above) — on the master side this causes timeouts instead of clear errors.
- RTU framing is fragile under fragmentation (no inter-frame timeout, no frame reassembly).
- `slave.go:217` (`writeValues := pdu.Payload[9 : 9+byteCount]`) and similar spots in FC17/FC6 have **no bounds checks** against malformed/short payloads — a broken master could trigger an index-out-of-range panic here (no `recover()` in request processing). FC16 at least has a length check; FC17 does not.
- Hardcoded baud rate (see above).

## Test coverage — very thin

- **A single** test file in the entire project: `config/config_test.go` (78 lines, 21.9% coverage — and only for the `config` package).
- Every other package (including the entire protocol core `modbuslabs`, plus `tcp`, `rtu`, `rules`, `plugins`, `encoding`, `console`, `socat`) has **0% coverage** — no unit tests exist.
- There are `.feature` files (`features/write_register.feature`, `features/start_socat.feature`) in Gherkin format, but **no Cucumber/Godog dependency and no step definitions** anywhere in the repo — these are pure specification documents, not executable tests.
- The actual "tests" are shell scripts (`test/test-all.sh`, `test-write-read.sh`, `setup-virtual-ports.sh`) that run the binary manually against TCP/RTU — functional end-to-end checks, but not wired into CI (no `.github/workflows`), so they're run manually only.
- No `go vet`/lint issues found; the build is green.

## Summary

| Aspect | Assessment |
|---|---|
| Architecture | Well-structured, appropriately simple |
| Protocol completeness | Gappy — FC1/FC3/FC15 missing, no exceptions, FC17 is a hardcoded hack |
| TCP | Solid |
| RTU | Functional, but fragile framing, fixed baud rate |
| Robustness | Moderate — missing bounds checks/exceptions are real crash/timeout risks |
| Test coverage | Very low — effectively only the `config` package, no CI |

## TODO

### High priority

- [ ] **Implement FC3 (Read Holding Registers)** — the most important missing read function; masters that write via FC6/FC16 currently can't read the values back
- [ ] **Introduce generic exception responses** (Illegal Function `0x01`, Illegal Data Address `0x02`, Illegal Data Value `0x03`) instead of returning `nil`, for unknown FCs, invalid unit IDs, and malformed payloads
- [ ] **Add bounds checks in FC17 (`slave.go:212-240`) and FC6/FC16** to prevent index-out-of-range panics on broken/short payloads
- [ ] **Write unit tests for the protocol core**: `modbus.go` (PDU assembly), `slave.go` (all FC handlers incl. edge cases), `rules/engine.go`
- [ ] **Write unit tests for `tcp/handler.go`** (MBAP framing, invalid headers, EOF handling) and **`rtu/handler.go`** (CRC validation, frame parsing)

### Medium priority

- [ ] **Add FC1 (Read Coils) and FC15 (Write Multiple Coils)**
- [ ] **Make FC17 generic** — read the real value from the register map instead of returning a hardcoded response payload (or explicitly document/flag it as a device-specific special case)
- [ ] **Harden RTU framing**: add inter-frame timeout / frame reassembly instead of assuming a whole request arrives in a single `Read()` call
- [ ] **Make baud rate configurable** (currently hardcoded to 9600 in `rtu/handler.go:29`)
- [ ] **Set up CI** (e.g. GitHub Actions): `go build`, `go vet`, `go test -cover` on every push/PR

### Low priority

- [ ] Decide whether the `.feature` files should become executable tests (e.g. via Godog) or remain deliberately as pure specification
- [ ] Integrate the shell-based tests (`test/test-all.sh` etc.) into CI, or replace them with Go integration tests
- [ ] Define a coverage target and enforce it as a CI gate
