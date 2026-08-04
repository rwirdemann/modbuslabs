# FC3 — Read Holding Registers

## Why

Real Modbus masters read back written values via FC3 (Read Holding
Registers), not FC4 (Read Input Registers). slavesim currently has no
FC3 handler at all: a master that writes registers via FC6/FC16 and
then reads them back via FC3 gets no response and times out. FC4 is
the wrong function code for this — it exists in the spec for a
logically separate register space and happens to share slavesim's
register map only by implementation accident. This is the
highest-priority gap flagged in `STATUS_REPORT.md`.

## What

Add support for Function Code 3 (`0x03`) to the protocol core.
Request and response shapes are identical to FC4 (start address +
quantity in the request; byte count + N x 2-byte values in the
response), because slavesim models a single shared register map with
no separate holding/input address spaces.

## Context

**Relevant files:**
- `modbus.go` — FC constants (`FC2ReadDiscreteRegisters`,
  `FC4ReadInputRegisters`, ...); `PDU.String()` has a switch-case that
  formats read requests (`case FC2ReadDiscreteRegisters,
  FC4ReadInputRegisters:`) and needs FC3 added there for correct RX
  logging
- `slave.go` — `Process()` dispatch switch; `processFC4` is the
  template — same register map, same response shape, only the
  function code differs
- No test file exists yet for `slave.go` (see `STATUS_REPORT.md` TODO
  "write unit tests for the protocol core") — this spec includes
  adding one, scoped to FC3 plus a minimal FC4/FC6 interaction test to
  prove they share the register map correctly

**Patterns to follow:**
- `processFC4`'s structure (byte-count computation, response payload
  assembly, per-register `ApplyReadRules` call) applies to FC3
  unchanged except for `FunctionCode`

**Key decisions already made:**
- FC3 reads from the **same** `s.registers` map as FC4/FC5/FC6/FC16.
  slavesim does not and will not model separate holding vs. input
  register address spaces (confirmed scope boundary, not a TODO)
- No exception-response handling is bundled into this spec — that's
  tracked as its own item in `STATUS_REPORT.md` and applies uniformly
  to all function codes, not just FC3

## Constraints

**Must:**
- Add `FC3ReadHoldingRegisters uint8 = 0x03` to the FC constants in
  `modbus.go`
- Implement `processFC3` in `slave.go`, wired into the `Process()`
  switch
- Response payload format: `[byteCount(1)][value1(2)]...[valueN(2)]`,
  matching FC4 exactly
- Add FC3 to the `PDU.String()` read-request case
  (`case FC2ReadDiscreteRegisters, FC4ReadInputRegisters:` →
  include `FC3ReadHoldingRegisters`) so RX logs show `Addr`/`Qty`
  correctly instead of falling through to the empty default case
- Apply read rules via `ruleEngine.ApplyReadRules` per register, at
  parity with FC4

**Must not:**
- Don't introduce a separate register namespace for holding vs. input
  registers
- Don't touch FC1, FC15, or exception-response handling — separate
  TODO items in `STATUS_REPORT.md`
- Don't add payload/bounds validation beyond what `processFC4`
  currently has (keeping parity; validation hardening is a separate
  task)

**Out of scope:**
- Modbus exception responses for invalid FC3 requests (illegal
  address, etc.)
- General unit-test infrastructure — only FC3-scoped tests are added
  here

## Tasks

### T1: FC3 constant + PDU logging

**Do:**
- Add `FC3ReadHoldingRegisters uint8 = 0x03` next to the existing FC
  constants
- Add `FC3ReadHoldingRegisters` to the read-request case in
  `PDU.String()`

**Files:** `modbus.go`

**Verify:** `go build ./...`

---

### T2: processFC3 in slave.go

**Do:**
- Implement `processFC3`, copying `processFC4`'s logic
  (byte count = `quantity * 2`, loop over addresses, read from
  `s.registers`, apply read rules, assemble response)
- Add `case FC3ReadHoldingRegisters: return s.processFC3(pdu)` to
  `Process()`

**Files:** `slave.go`

**Verify:**
- `go build ./...`
- Manual: `go run cmd/master/main.go --fc=6 --address=0x9000
  --value=42 --transport=tcp` then `go run cmd/master/main.go --fc=3
  --address=0x9000 --quantity=1 --transport=tcp` returns `42`

---

### T3: Unit tests

**Do:**
- Add `slave_test.go` covering:
  - FC3 read of a register previously written via FC6 returns the
    written value
  - FC3 read of an unset register returns `0`
  - FC3 read of multiple consecutive registers returns them in order
  - FC3 and FC4 read the same address independently and see the same
    underlying value (proves the shared-map decision, guards against
    a future accidental split)

**Files:** `slave_test.go` (new)

**Verify:** `go test ./...`

## Done

- [ ] `go build ./...`
- [ ] `go test ./...`
- [ ] Manual: write via FC6/FC16, read back the same address via FC3,
  values match
- [ ] No regression: FC4 still reads the same map independently
