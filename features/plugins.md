# Plugins

## Why

The rules engine supports only built-in actions (set_value, increment,
etc.). Custom simulator behaviour — e.g. receiving a firmware package
over consecutive register writes — cannot be expressed in TOML alone.
Plugins let users attach stateful Go objects to rules without forking
the engine.

## What

A named-plugin system: users register Plugin instances against a name;
a rule with `plugin = "name"` calls that instance instead of a built-in
action. Plugins implement an interface and may hold internal state
(e.g. accumulating bytes across multiple writes). The existing
`slavesim.example.toml` plugin rule works end-to-end.

## Context

**Relevant files:**
- `config/config.go` — `Rule` struct; needs `Plugin string` field and
  `value = any` wildcard support (`Value *uint16` can't express "any")
- `rules/engine.go` — `ApplyWriteRules`/`ApplyReadRules`; must look up
  and call registered plugins
- `slave.go` — calls `ApplyWriteRules`; passes registers map already
- `slavesim.example.toml` — intended usage already drafted

**Patterns to follow:**
- Rule validation in `config.Rule.Validate()` — add plugin to valid
  actions, enforce mutual exclusion with `action`
- Engine construction via `rules.NewEngine(configRules)` — extend to
  accept a registry

**Key decisions already made:**
- Plugin = registered Go struct implementing an interface, not
  `plugin.Open` (.so). Plugins are wired at process start in
  `cmd/slavesim/main.go`.
- Plugins are stateful: the same instance is called for every matching
  write, allowing state accumulation across calls (e.g. collecting
  firmware bytes).
- `value = any` is represented as a new `ValueAny bool` field in
  `config.Rule` (TOML: `value = "any"`). Engine skips value comparison
  when `ValueAny` is true.

## Constraints

**Must:**
- Plugin interface:
  ```go
  type Plugin interface {
      Execute(
          register  uint16,
          value     uint16,
          registers map[uint16]uint16,
      ) error
  }
  ```
- Keep `NewEngine` backwards-compatible (nil registry = no plugins)
- Validate: a rule must have exactly one of `action` or `plugin`

**Must not:**
- No new external dependencies
- Don't modify TCP/RTU transport handlers
- Don't refactor existing rule actions

**Out of scope:**
- Read-trigger plugins (on_read)
- Hot-reload / dynamic plugin loading
- Plugin return values mutating the PDU response

## Tasks

### T1: Config — Plugin field + value=any

**Do:**
- Add `Plugin string` and `ValueAny bool` to `config.Rule`
- Update TOML parsing: `value = "any"` sets `ValueAny = true`,
  `Value = nil`
- Update `Rule.Validate()`: accept `plugin` as alternative to `action`;
  reject rules with both or neither

**Files:** `config/config.go`

**Verify:** `go test ./config/...` — add table tests covering:
- rule with `plugin` + no `action` → valid
- rule with both → error
- rule with `value = "any"` → `ValueAny == true`, `Value == nil`

---

### T2: Plugin interface + registry in rules package

**Do:**
- Create `rules/plugin.go` with `Plugin` interface and `Registry` type:
  ```go
  type Plugin interface {
      Execute(
          register  uint16,
          value     uint16,
          registers map[uint16]uint16,
      ) error
  }

  type Registry map[string]Plugin
  ```
- Extend `NewEngine` signature:
  `NewEngine(configRules []config.Rule, registry Registry) *Engine`
  (nil registry is safe)
- In `ApplyWriteRules`: when rule has `Plugin != ""`, look up name in
  registry and call `Execute`; skip value-match when `ValueAny` is true
- Update every `NewEngine` call site (`gateway.go`)

**Files:** `rules/plugin.go`, `rules/engine.go`, `gateway.go`

**Verify:** `go build ./...` — no compile errors; existing rule tests
still pass (`go test ./rules/...`)

---

### T3: receive_package plugin + wiring

**Do:**
- Add `plugins/receive_package.go` implementing `rules.Plugin`;
  the struct accumulates written values in a `[]uint16` slice
  (models receiving firmware package chunks)
- Wire it in `cmd/slavesim/main.go`:
  ```go
  registry := rules.Registry{
      "receive_package": &plugins.ReceivePackage{},
  }
  ```
- Update `ConnectSlaveWithConfig` in `gateway.go` to accept and pass
  through `rules.Registry`

**Files:** `plugins/receive_package.go`, `cmd/slavesim/main.go`,
`gateway.go`

**Verify:**
- `go build ./...`
- Manual: start slavesim with `slavesim.example.toml`; write register
  `0xA672` multiple times; log shows accumulated values growing

## Done

- [ ] `go build ./...`
- [ ] `go test ./...`
- [ ] Manual: slavesim starts, repeated FC6 writes to `0xA672` trigger
  `receive_package` with accumulating state visible in logs
- [ ] No regressions: existing rules (set_value, write_register) still
  work for slave 101 rule on `0xA66D`
