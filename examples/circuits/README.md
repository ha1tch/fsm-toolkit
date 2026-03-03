# Circuit Examples

Three example circuits demonstrating the structural connectivity features
of fsm-toolkit. Each includes full pin-accurate net definitions that export
to valid KiCad netlists.

## Gated D Flip-Flop (`gated_d_flipflop.json`)

A D flip-flop with enable gating and clock inversion.

- **U1** — 7408 quad AND gate (gate A: enable AND data)
- **U2** — 7404 hex inverter (inverter 1: clock inversion)
- **U3** — 7474 dual D flip-flop (FF1: latches gated data)

Signal nets: DATA_GATED (U1 pin 3 → U3 pin 2), CLK_INV (U2 pin 2 → U3 pin 3).
Preset and clear on the 7474 are tied to VCC (inactive).

## 4-Bit Counter with 7-Segment Display (`counter_7seg.json`)

A decade counter (0–9) with automatic reset and BCD-to-7-segment decoding.

- **U1** — 74161 synchronous 4-bit counter
- **U2** — 7447 BCD to 7-segment decoder
- **U3** — 7400 quad NAND gate (gate A: decode-9 reset)

Data bus: BIT_A through BIT_D connect counter outputs (pins 14, 13, 12, 11)
to decoder inputs (pins 7, 1, 2, 6). Reset logic: NAND(QA, QD) produces a
low pulse on CLR_N when the count reaches 9 (binary 1001), resetting to 0.
Counter enable pins and decoder control pins are tied to appropriate levels.

## 4-Bit Comparator with LED Indicators (`comparator_leds.json`)

A magnitude comparator with inverted outputs for driving active-high LEDs.

- **U1** — 7485 4-bit magnitude comparator
- **U2** — 7404 hex inverter (inverters 1–3: output drivers)

Signal nets: GT_ACTIVE, EQ_ACTIVE, LT_ACTIVE connect comparator outputs
(pins 5, 6, 7) to inverter inputs (pins 1, 3, 5). Cascade inputs are tied
for single-stage operation (EQ_IN high, GT_IN and LT_IN low).

## Exporting

```bash
# Human-readable text netlist
fsm netlist counter_7seg.json

# KiCad .net file for PCBnew import
fsm netlist counter_7seg.json --format kicad -o counter_7seg.net

# Structural JSON for scripting
fsm netlist counter_7seg.json --format json -o counter_7seg_netlist.json

# Bake KiCad part/footprint fields into the source file
fsm netlist counter_7seg.json --bake
```

---

## Security Code Lock Bundle (`codelock.fsm`)

A three-machine bundle demonstrating linked states, behavioural FSMs,
and structural connectivity working together. Each machine has
meaningful state transitions AND maps to physical 74xx components with
pin-accurate nets.

### Machine 1: Scanner (`codelock_scanner.json`)

Captures and debounces keypad input, then signals "digit ready" to the
matcher via a linked state.

**Behavioural:** `IDLE → KEY_PRESSED → DEBOUNCE → VALID → (→ matcher)`

**Structural:**
- **IDLE / KEY_PRESSED** — 74148 8-to-3 priority encoder: converts
  one-hot keypad lines to binary
- **DEBOUNCE** — 74121 monostable: generates debounce timing pulse
- **VALID** — 7474 dual D flip-flop: latches debounced digit

Signal nets: KEY_STROBE (encoder GS_N → monostable B → latch 1D),
DEBOUNCE_CLK (monostable Q → latch 1CLK).

### Machine 2: Matcher (`codelock_matcher.json`)

Compares each entered digit against a stored code sequence. Tracks
position and match/mismatch, then signals the controller.

**Behavioural:** `DIGIT_1 → DIGIT_2 → MATCH/FAIL → (→ controller)`

**Structural:**
- **DIGIT_1** — 74161 sync 4-bit counter: position tracker (which digit)
- **DIGIT_2** — 7485 4-bit comparator: compares entered vs expected digit
- **FAIL** — 74157 quad 2-to-1 mux: selects reference digit by position
- **MATCH** — 7474 dual D flip-flop: latches comparison verdict

Signal nets: POS_SEL (counter QA → mux SEL), REF_BIT0-2 (mux Y → comparator B),
CMP_EQUAL (comparator EQ_OUT → verdict latch 1D), SEQ_DONE (counter RCO → verdict
1CLK).

### Machine 3: Controller (`codelock_controller.json`)

Controls the physical lock mechanism with timed unlock and alarm/lockout
paths.

**Behavioural:**
`LOCKED → UNLOCKING → UNLOCKED → RELOCKING → LOCKED` (success path)
`LOCKED → ALARM → LOCKOUT → LOCKED` (failure path)

**Structural:**
- **LOCKED / ALARM** — 7400 quad NAND: combinational gating for
  unlock and alarm signals
- **UNLOCKING** — 7474 dual D flip-flop: lock state (FF1) and alarm
  state (FF2) latches
- **UNLOCKED** — 74121 monostable: timed unlock pulse
- **RELOCKING / LOCKOUT** — 7404 hex inverter: output drivers and
  LED indicators

Signal nets: UNLOCK_GATE/UNLOCK_CLK (NAND gates → lock flip-flop),
LOCK_STATE (flip-flop Q → monostable → LED driver), RELOCK_N
(monostable Q_N → flip-flop CLR_N), ALARM_GATE/ALARM_CLK/ALARM_STATE
(alarm path through NAND → flip-flop → inverter), LED_LOCK/LED_ALARM
(inverter outputs for indicator LEDs).

### Working with the bundle

```bash
# List machines
fsm machines codelock.fsm

# View individual machine info
fsm info codelock.fsm --machine scanner

# Run behavioural simulation
fsm run codelock.fsm --machine controller

# Export individual machine netlists
fsm netlist codelock.fsm --machine matcher --format kicad -o matcher.net

# Extract a machine from the bundle
fsm extract codelock.fsm --machine controller -o controller.fsm
```

### Feature coverage

| Feature | How it's used |
|---------|--------------|
| Bundle with 3 machines | Scanner, matcher, controller |
| Linked states | VALID→matcher, MATCH→controller, FAIL→controller |
| Behavioural transitions | Every machine has meaningful state logic |
| 8 component types | 74148, 7474, 74121, 74161, 7485, 74157, 7400, 7404 |
| 14 component instances | Across 3 machines |
| Signal nets (~16) | Data paths, control signals, digit bus, LED outputs |
| Power nets | VCC/GND across all instances |
| Multi-fan-out | KEY_STROBE fans to 3 chips, VCC to multiple tie-offs |
| KiCad export | All DIP-14/DIP-16, all real 74xx library references |
| FSM simulation | `fsm run` with linked-state delegation |
