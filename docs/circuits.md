# Structural Connectivity

This document explains how fsm-toolkit models physical circuits alongside
behavioural state machines.

## The Dual Model

Most FSM tools model behaviour: states, transitions, inputs, outputs.
Most EDA tools model structure: components, pins, nets, footprints. The
fsm-toolkit does both in the same file.

A single FSM can be purely behavioural (a traffic light controller),
purely structural (a 74xx circuit with no meaningful state transitions),
or both at once (a security lock that has real state logic AND maps to
physical chips). The two views are independent — neither requires the
other, and neither constrains the other.

The key insight is that states and components are the same abstraction.
Both have identity (a name), properties (key-value metadata), and
connections (transitions to other states, nets to other pins). The
toolkit exploits this: a state IS a component instance, and a component
instance IS a state.


## A Concrete Example

Consider a simple circuit: a 7408 AND gate feeding a 7474 D flip-flop.

As a **behavioural FSM**, this might be a two-state machine where the
gate's output clocks the flip-flop. The states are `U1` and `U2`, there
is a transition on `clk`, and you can simulate it with `fsm run`.

As a **structural netlist**, U1 is a 14-pin DIP with four AND gates, U2
is a 14-pin DIP with two D flip-flops, and a net called `DATA_GATED`
connects U1's pin 3 (gate A output) to U2's pin 2 (FF1 data input).

Both descriptions live in the same JSON file:

```json
{
  "type": "dfa",
  "states": ["U1", "U2"],
  "alphabet": ["clk"],
  "transitions": [
    {"from": "U1", "input": "clk", "to": "U2"}
  ],
  "classes": {
    "7408_quad_and": { "ports": [...] },
    "7474_dual_d_flipflop": { "ports": [...] }
  },
  "state_classes": {
    "U1": "7408_quad_and",
    "U2": "7474_dual_d_flipflop"
  },
  "nets": [
    {
      "name": "DATA_GATED",
      "endpoints": [
        {"instance": "U1", "port": "1Y"},
        {"instance": "U2", "port": "1D"}
      ]
    }
  ]
}
```

The behavioural layer (states, transitions) and the structural layer
(classes, ports, nets) coexist. Remove the nets and you still have a
valid FSM. Remove the transitions and you still have a valid netlist.


## Classes

A **class** is a component type. It defines what a component looks like
and what pins it has, but not how many of them exist or where they are
in the circuit.

```json
"7400_quad_nand": {
  "properties": [
    {"name": "gate_count", "type": "int64"},
    {"name": "logic_function", "type": "[40]string"}
  ],
  "ports": [
    {"name": "1A", "direction": "input",  "pin_number": 1, "group": "GATE_A"},
    {"name": "1B", "direction": "input",  "pin_number": 2, "group": "GATE_A"},
    {"name": "1Y", "direction": "output", "pin_number": 3, "group": "GATE_A"},
    {"name": "GND", "direction": "power", "pin_number": 7},
    {"name": "VCC", "direction": "power", "pin_number": 14}
  ]
}
```

Classes are shared across states. Assigning the class `7400_quad_nand`
to state U1 means U1 has 14 pins, four AND gates, a VCC, and a GND —
just like every other 7400 in the circuit.

### Class Libraries

The toolkit ships with 49 pre-defined 74xx TTL components in seven
library files under `class-libraries/`. These cover the most common
families: gates, flip-flops, counters, registers, multiplexers,
arithmetic, buffers, and specialty (encoders, decoders, Schmitt
triggers, monostables).

Every component has complete pin-accurate port definitions, matching
the original Texas Instruments datasheets. Pin numbers, directions,
and functional groups are all specified.

Libraries are loaded into the editor via the component drawer. They
can also be referenced directly in JSON files by including the class
definition in the `"classes"` section.


## Ports

A **port** is one pin on a component. Every port has:

| Field | Purpose |
|-------|---------|
| `name` | Identifier within the class (e.g. `1Y`, `CLK`, `VCC`) |
| `direction` | `input`, `output`, `bidirectional`, or `power` |
| `pin_number` | Physical pin on the IC package (1-indexed) |
| `group` | Optional functional group (e.g. `GATE_A`, `FF1`) |

Directions matter for the connection detail view (which draws arrows
showing signal flow) and for validation (outputs normally drive inputs).
They do not constrain what you can connect — the toolkit does not
enforce electrical rules.

Groups organise pins into logical sub-units. A 7400 has four NAND
gates (GATE_A through GATE_D); a 7474 has two flip-flops (FF1, FF2).
Groups appear as section headers in the connection detail window and
help navigate complex components.

The `power` direction marks VCC and GND pins. Power ports are
distinguished from signal ports in several ways: power nets are
rendered in a muted colour on the canvas, the `fsm info` command
shows separate signal and power net counts, and the connection detail
window is reached only through signal connections (power is
infrastructure, not interesting topology).


## Nets

A **net** is a named electrical connection between two or more ports
on different component instances.

```json
{
  "name": "DATA_BUS",
  "endpoints": [
    {"instance": "U1", "port": "3Y"},
    {"instance": "U2", "port": "1D"}
  ]
}
```

The `instance` field refers to a state name. The `port` field refers
to a port name on that state's class. The net model is simple and
flat — there is no hierarchy, no buses, no differential pairs. A net
connects two or more endpoints, and that is all.

### Net naming conventions

The toolkit does not enforce any naming convention. In practice, signal
nets tend to be named for their function (`DATA_BUS`, `CLK_INV`,
`RESET_N`) and power nets are named `VCC` and `GND`. The `_N` suffix
conventionally marks active-low signals.

### Multi-fan-out

A net can have any number of endpoints. A clock signal distributed to
five flip-flops is one net with six endpoints (the clock source plus
five destinations). In the connection detail window, when you examine
a pair of components, multi-fan-out beyond that pair is shown as a
footnote rather than cluttering the pin-to-pin view.

### Power nets

Power nets (VCC, GND) tend to have many endpoints because every IC
connects to the supply rails. In structural designs, you will also
see control inputs tied to power: a flip-flop's preset pin tied to
VCC (inactive), or unused mux select lines tied to GND. These are
normal practice — the tied pin appears as an endpoint on the power
net, which is electrically correct.


## Canvas Rendering

When a file has nets, the canvas shows them as coloured connections
between components. The rendering uses a simplified topology view, not
a full schematic:

- Two-endpoint nets are drawn as direct routes below the components
- Multi-endpoint nets are drawn as horizontal buses with vertical stubs
- Power nets are shown in a muted colour
- Signal nets are shown in orange

Press `N` in the editor to toggle net visibility. This is a display
toggle, not a data operation — the nets remain in the file either way.

The canvas does not attempt wire routing or crossing minimisation.
It shows which components are connected and how many nets exist between
them. For the actual pin-to-pin detail, use the connection detail
window (press `E` on a selected state).


## Connection Detail Window

Select a component on the canvas and press `E` to open the connection
detail window. This shows every signal-level pin-to-pin connection
between the selected component and a chosen peer.

If the component connects to multiple peers, a picker appears first:

```
Connected states:
  U3 [5 nets]
  U7 [3 nets]
  U7 (self) [2 nets]
```

The detail view then shows a three-column layout:

```
  U1 (7408_quad_and) <--> U3 (7474_dual_d_flipflop)

  [GATE_A]            Net            [FF1]
  1Y          ──── DATA_BUS ───>     1D
  [GATE_B]
  2Y          ──── CLK_INV  ───>     1CLK
```

Direction arrows show signal flow based on port directions. You can
add, delete, and rename connections from this view using `A`, `D`,
and `R` respectively.


## Exporting Netlists

The `fsm netlist` command exports the structural data in three formats:

### Text format

Human-readable, for verification:

```
# Netlist: 4-Bit Counter with 7-Segment Display
# Components: 3, Nets: 7

# Components
U1           74161_sync_4bit_counter        Package_DIP:DIP-16_W7.62mm
U2           7447_bcd_to_7seg_decoder       Package_DIP:DIP-16_W7.62mm
U3           7400_quad_nand                 Package_DIP:DIP-14_W7.62mm

# Nets
BIT_A:           U1.QA(pin14), U2.A(pin7), U3.1A(pin1)
BIT_B:           U1.QB(pin13), U2.B(pin1)
RESET_N:         U3.1Y(pin3), U1.CLR_N(pin1)
```

### KiCad format

S-expression `.net` file for import into KiCad's PCBnew:

```bash
fsm netlist circuit.json --format kicad -o circuit.net
```

The exporter auto-derives KiCad library references and DIP footprints
for 74xx components. A class named `7400_quad_nand` produces
`(lib 74xx) (part 7400)` and `(footprint Package_DIP:DIP-14_W7.62mm)`.
Non-74xx components get a placeholder library (`fsm-toolkit`) and are
flagged as unresolved in the export summary.

### JSON format

Structured projection of the netlist data for scripting or custom
tooling:

```bash
fsm netlist circuit.json --format json
```

### Baking KiCad fields

By default, the KiCad part name and footprint are derived at export
time from the class name and pin count. This derivation is transparent
and correct, but the fields are not stored in the source file.

The `--bake` flag writes the derived values back into the file:

```bash
fsm netlist circuit.json --bake
```

This makes the mapping visible and editable. You can then override the
footprint (say, SOIC instead of DIP for a surface-mount build) by
editing the `kicad_footprint` field directly. Once set, the derivation
will not overwrite your value.

Baking also works on class library files:

```bash
fsm netlist class-libraries/74xx_gates.classes.json --bake
```

This is idempotent — running it twice changes nothing.


## Bundles and Structural Connectivity

The structural model works fully within bundles. Each machine in a
bundle can have its own classes, ports, and nets. When you export a
netlist from a bundle, you select which machine to export:

```bash
fsm netlist system.fsm --machine controller --format kicad -o controller.net
```

Linked states provide the behavioural bridge between machines. In a
circuit like the security code lock example, the scanner machine's
`VALID` state links to the matcher machine, and the matcher's `MATCH`
and `FAIL` states link to the controller. The state transitions model
the information flow; the nets model the physical wiring.

This separation is intentional. Behavioural links are about control
flow (which machine processes the next input). Structural nets are
about electrical connectivity (which pin connects to which pin). They
describe different aspects of the same system.


## Cost to Simple FSMs

If a file has no classes with ports and no nets, the structural
features are invisible:

- No new fields appear in the serialised file
- No new UI elements show in the editor
- No canvas overlay is drawn
- The `N` and `E` key bindings are inert

A 3-state traffic light FSM works exactly as it always has. The
structural features only activate when you assign a class with ports
to a state.


## Compared to EDA Tools

This is not a schematic editor or a PCB layout tool. It does not do:

- Wire routing or crossing minimisation
- Design rule checking (trace widths, clearances)
- Electrical rule checking (shorted outputs, floating inputs)
- Simulation (SPICE, digital, or mixed-signal)
- Component placement or auto-routing

What it does do is maintain a pin-accurate structural description
alongside a simulatable behavioural model, and export that structure
in formats that real EDA tools can import. The intended workflow is:

1. Design the state machine behaviour in fsm-toolkit
2. Assign physical components from the class libraries
3. Wire the signal and power nets in the connection detail window
4. Export a KiCad netlist
5. Import into KiCad for schematic capture, PCB layout, and fabrication

The toolkit handles steps 1-4. KiCad handles step 5 onward.


## Compared to FSM Tools

Most FSM tools stop at state diagrams and code generation. They model
behaviour but not structure. You can design a counter in an FSM tool,
but you cannot express that it is built from a 74161, a 7447, and a
7400 with specific pin connections between them.

The fsm-toolkit bridges this gap for a specific niche: digital circuits
built from discrete logic, where the state machine IS the circuit and
the circuit IS the state machine. This is common in:

- Retro computing projects (homebrew CPUs, peripheral controllers)
- Digital logic education (lab exercises, coursework)
- Hobby electronics (where 74xx TTL is the building block)

For these use cases, the ability to simulate the FSM behaviour and
then export a buildable netlist from the same file is the point.


## Class Libraries Reference

The toolkit ships seven 74xx library files with 49 components:

| Library | Components | Description |
|---------|-----------|-------------|
| 74xx_gates | 7400, 7402, 7404, 7408, 7432, 7486 | Basic logic gates |
| 74xx_sequential | 7474, 7476, 7490, 7493, 74161, 74164 | Flip-flops, counters, shift registers |
| 74xx_buffers | 7407, 7417, 74125, 74126, 74244, 74245 | Buffers and bus transceivers |
| 74xx_mux_demux | 74150, 74151, 74153, 74157, 74138, 74139, 74154 | Multiplexers and decoders |
| 74xx_registers_memory | 7475, 74373, 74374, 74173, 74194, 74195, 74189, 74170 | Latches, registers, RAM |
| 74xx_arithmetic | 7483, 74283, 7485, 74181, 74182, 74180 | Adders, comparators, ALU |
| 74xx_specialty | 7414, 74132, 74121, 74123, 7401, 7403, 74147, 74148, 7447, 7448 | Schmitt triggers, monostables, encoders, open-collector |

All pin numbers match the original Texas Instruments datasheets.
Packages are standard DIP (14, 16, 20, or 24 pin).


## Example Circuits

The `examples/circuits/` directory contains working examples:

| Example | Components | Description |
|---------|-----------|-------------|
| `gated_d_flipflop.json` | 7408, 7404, 7474 | AND-gated data with clock inversion |
| `counter_7seg.json` | 74161, 7447, 7400 | Decade counter with 7-segment decode and auto-reset |
| `comparator_leds.json` | 7485, 7404 | 4-bit magnitude comparator with LED drivers |
| `codelock.fsm` | 8 types, 14 instances | Three-machine bundle: scanner, matcher, lock controller |

The code lock is the most complete example. Each machine has meaningful
behavioural state transitions AND maps to physical 74xx components with
pin-accurate nets. The scanner links to the matcher, which links to the
controller. See `examples/circuits/README.md` for full documentation.


## See Also

- [Machines](machines.md) — Linked states and bundles
- [Netlist Design](netlist-design.md) — Internal design and implementation notes
- [Example Circuits](../examples/circuits/README.md) — Working 74xx circuit files
- [Specification](specification.md) — File format specification
