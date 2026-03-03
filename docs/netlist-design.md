# Structural Connectivity Design

This document defines the design for adding structural (netlist) connectivity
to the FSM Toolkit alongside the existing behavioural (transition) model.

**Version**: 0.1 (draft)
**Status**: All phases implemented
**Date**: 2026-03-01

---

## Table of Contents

1. [Design Principles](#design-principles)
2. [Phase 1 — Port Model](#phase-1--port-model)
3. [Phase 2 — Net Model](#phase-2--net-model)
4. [Phase 3 — Canvas Rendering](#phase-3--canvas-rendering)
5. [Phase 4 — Connection Detail Window](#phase-4--connection-detail-window)
6. [Phase 5 — Export Formats](#phase-5--export-formats)
7. [File Format Changes](#file-format-changes)
8. [Settings and Toggles](#settings-and-toggles)
9. [Undo/Redo](#undoredo)
10. [Design Decisions Log](#design-decisions-log)

---

## Design Principles

The FSM Toolkit starts small (3-state turnstile) and grows to structural
netlists (74xx digital circuits). The design must:

- **Cost nothing for simple FSMs.** Files with no ports and no nets behave
  exactly as they do today. No new fields appear in serialised output. No
  new UI elements visible.
- **Keep behavioural and structural models independent.** Transitions are
  behaviour. Nets are structure. Both live in the same FSM, both can be
  viewed and edited, but neither depends on the other.
- **Use two-level visual representation.** The canvas shows topology (one
  summary line per connected component pair). The detail window shows the
  actual pin-to-pin mappings. This avoids the wire-routing problem entirely.

---

## Phase 1 — Port Model

### New types in `pkg/fsm/port.go`

```go
type PortDir string

const (
    PortInput  PortDir = "input"
    PortOutput PortDir = "output"
    PortBidir  PortDir = "bidir"
    PortPower  PortDir = "power"
)

type Port struct {
    Name      string  `json:"name"`                 // "1CLK", "1D", "1Q", "VCC"
    Direction PortDir `json:"direction"`             // input, output, bidir, power
    PinNumber int     `json:"pin_number,omitempty"`  // physical pin; 0 = unassigned
    Group     string  `json:"group,omitempty"`       // "FF1", "FF2", "GATE_A"
}
```

### Changes to `Class`

```go
type Class struct {
    Name       string        `json:"name"`
    Parent     string        `json:"parent,omitempty"`
    Properties []PropertyDef `json:"properties"`
    Ports      []Port        `json:"ports,omitempty"`  // NEW
}
```

### Invariants

- A class with an empty `Ports` slice is a purely behavioural class.
  All existing classes remain behavioural until explicitly upgraded.
- Port names must be unique within a class.
- The `power` direction is a first-class concept because it drives
  rendering filters (see Phase 3, Decision D1).
- The `Group` field enables sub-unit identification within a package
  (e.g., four NAND gates in a 7400). It is optional. When present,
  the connection detail window uses it for visual grouping and for
  self-connection footnotes (see Decision D3).

### Class library updates

All seven 74xx `.classes.json` files gain a `"ports"` array alongside
the existing `"properties"` array. The `pin_names` list property is
retained for backward compatibility but is no longer the authoritative
pin data — ports are.

Example (7474 dual D flip-flop):

```json
{
  "7474_dual_d_flipflop": {
    "properties": [ ... ],
    "ports": [
      {"name": "1CLR_N", "direction": "input",  "pin_number": 1,  "group": "FF1"},
      {"name": "1D",     "direction": "input",  "pin_number": 2,  "group": "FF1"},
      {"name": "1CLK",   "direction": "input",  "pin_number": 3,  "group": "FF1"},
      {"name": "1PRE_N", "direction": "input",  "pin_number": 4,  "group": "FF1"},
      {"name": "1Q",     "direction": "output", "pin_number": 5,  "group": "FF1"},
      {"name": "1Q_N",   "direction": "output", "pin_number": 6,  "group": "FF1"},
      {"name": "GND",    "direction": "power",  "pin_number": 7},
      {"name": "2Q_N",   "direction": "output", "pin_number": 8,  "group": "FF2"},
      {"name": "2Q",     "direction": "output", "pin_number": 9,  "group": "FF2"},
      {"name": "2PRE_N", "direction": "input",  "pin_number": 10, "group": "FF2"},
      {"name": "2CLK",   "direction": "input",  "pin_number": 11, "group": "FF2"},
      {"name": "2D",     "direction": "input",  "pin_number": 12, "group": "FF2"},
      {"name": "2CLR_N", "direction": "input",  "pin_number": 13, "group": "FF2"},
      {"name": "VCC",    "direction": "power",  "pin_number": 14}
    ]
  }
}
```

### Helper methods on `Class`

- `HasPorts() bool` — true if len(Ports) > 0.
- `GetPort(name string) *Port` — lookup by name.
- `PortsByGroup() map[string][]Port` — group ports by sub-unit.
- `SignalPorts() []Port` — all ports where Direction != PortPower.
- `PowerPorts() []Port` — all ports where Direction == PortPower.

### Helper methods on `FSM`

- `EffectivePorts(state string) []Port` — returns the port list from
  the class assigned to the given state. Returns nil if the class has
  no ports.

---

## Phase 2 — Net Model

### New types in `pkg/fsm/net.go`

```go
type NetEndpoint struct {
    Instance string `json:"instance"` // state name = component ref designator
    Port     string `json:"port"`     // port name on the class
}

type Net struct {
    Name      string        `json:"name"`      // "CLK_BUS", "N42", "VCC"
    Endpoints []NetEndpoint `json:"endpoints"`
}
```

### Changes to `FSM`

```go
type FSM struct {
    // ... existing fields ...
    Nets []Net `json:"nets,omitempty"` // empty for behavioural FSMs
}
```

### Invariants

- A net must have at least two endpoints to be valid.
- An endpoint's `Instance` must refer to an existing state.
- An endpoint's `Port` must refer to a port defined on the class
  assigned to that instance (validated against `EffectivePorts`).
- Net names must be unique within an FSM.
- Power nets (all endpoints reference `power`-direction ports) are
  structurally valid but filtered from canvas rendering (Decision D1).

### Helper methods on `FSM`

- `HasNets() bool`
- `AddNet(net Net) error` — validates endpoints against states/ports.
- `RemoveNet(name string) bool`
- `GetNet(name string) *Net`
- `NetsForState(state string) []Net` — all nets touching a state.
- `NetsBetween(stateA, stateB string) []Net` — all nets that have at
  least one endpoint on each state.
- `IsPowerNet(net Net) bool` — true if every endpoint's port has
  direction `power`.

### Cascade rules

- Renaming a state updates all `NetEndpoint.Instance` values.
- Deleting a state removes all endpoints referencing it. Nets that
  fall below two endpoints are removed entirely.
- Changing a state's class invalidates endpoints referencing ports
  that no longer exist. A validation pass flags or removes these.

---

## Phase 3 — Canvas Rendering

### Summary line semantics

Between any two components A and B, at most one structural summary
line is drawn. This line represents: "at least one net connects a
signal port on A to a signal port on B."

### Colour and direction rules

The renderer examines all non-power nets between A and B. For each
net, it determines signal direction by checking port directions on
the A side and B side.

| Condition | Colour | Arrowhead |
|-----------|--------|-----------|
| All nets flow A→B (A has outputs, B has inputs) | Orange | Arrow toward B |
| All nets flow B→A | Orange | Arrow toward A |
| Mixed directions, or any bidir port | Ochre/beige | None |

Derivation is done at draw time — no stored state. Walk the nets,
tally directions, pick colour and arrowhead.

### Power net filtering (Decision D1)

Nets where every endpoint references a `power`-direction port are
excluded from the summary line calculation and from canvas rendering.
They exist in the data model and appear in exported netlists, but
they are invisible on the canvas. This prevents every component pair
from showing a spurious connection through VCC/GND rails.

### Self-connections

A structural self-connection (net with two or more endpoints on the
same component, e.g., gate A output to gate B input on a 7400) is
drawn as an orange/ochre self-loop arc, matching the existing
transition self-loop style but in the structural colour. No label
on the arc.

### Toggle between views

The view mode is controlled by a setting (see Settings section).
Options:

- **Behavioural** — cyan/blue transition lines visible, structural
  lines hidden.
- **Structural** — orange/ochre connection lines visible, transition
  lines hidden.
- **Both** — both line types visible simultaneously.

The view mode is display-only. It does not restrict editing operations.
Adding a transition always works regardless of view. Adding a net
always works. If an edit creates an element that is not currently
visible, a brief status bar message informs the user:

> "Transition added — switch to Behavioural or Both view to see it."
> "Connection added — switch to Structural or Both view to see it."

This behaviour is configurable: the user may choose in Settings
whether adding a hidden-view element triggers a status bar
notification, or silently proceeds. (Decision D4.)

---

## Phase 4 — Connection Detail Window

### Opening

The user selects a structural summary line between components A and B
(click or keyboard selection) and double-clicks (or presses Enter).
The connection detail window opens as a modal overlay.

### Layout

The window is generously sized — it should fill most of the canvas
area, leaving only a thin margin. Connection lines between port names
should render as proper lines (extended horizontal rules), not as a
single hyphen.

The detail window has three columns:

```
┌─────────────────────────────────────────────────────────────┐
│  Connections: U3 (7400 Quad NAND) ←→ U7 (7474 Dual D FF)   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  U3                         Net            U7               │
│  ─────────────────────────────────────────────────────────  │
│                                                             │
│  [GATE_A]                                  [FF1]            │
│  3Y  ──────────────────── DATA_IN ──────── 2D              │
│                                                             │
│  [GATE_B]                                  [FF1]            │
│  6Y  ──────────────────── CLK_BUS ──────── 2CLK            │
│                                                             │
│                                                             │
│  ─────────────────────────────────────────────────────────  │
│  CLK_BUS also connects to: U12.4CLK, U15.1CLK              │
│  ─────────────────────────────────────────────────────────  │
│                                                             │
│  [A]dd  [D]elete  [R]ename net  [Esc] Close                │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Features

- **Port grouping.** Ports are visually grouped by their `Group` field
  when present. Group headers appear as `[FF1]`, `[GATE_A]`, etc.
- **Multi-fan-out footnotes (Decision D2).** When a net in view has
  endpoints beyond the two components being examined, a footnote line
  appears below the connection table: "CLK_BUS also connects to:
  U12.4CLK, U15.1CLK". This is a guardrail — it alerts the designer
  to the full extent of a net without requiring them to open other
  detail windows.
- **Self-connection footnotes (Decision D3).** When both endpoints of
  a net belong to the same component, the detail window groups them
  and adds a footnote: "Internal connection: GATE_A.3Y → GATE_B.4A
  (within same package)". This uses the `Group` field to explain
  the self-connection context.
- **Scrolling.** The connection list scrolls vertically for components
  with many connections (e.g., a 74181 ALU connected to a register
  bank). Same scroll pattern as the machine manager.
- **Controls:**
  - `A` — Add a new connection. Presents port pickers for each side,
    prompts for net name.
  - `D` — Delete the selected connection (removes endpoint pair from
    the net; removes the net if it falls below two endpoints).
  - `R` — Rename the selected net.
  - `Esc` — Close the window.

---

## Phase 5 — Export Formats

### `pkg/export/netlist.go`

Given the port and net model, export functions walk the FSM and emit:

- **Simple text netlist** — human-readable, for validation:
  ```
  # Components
  U3  7400_quad_nand  Package_DIP:DIP-14_W7.62mm
  U7  7474_dual_d_flipflop  Package_DIP:DIP-14_W7.62mm

  # Nets
  DATA_IN: U3.1Y(pin3), U7.1D(pin2)
  CLK_BUS: U3.6Y(pin6), U7.2CLK(pin11)
  VCC: U3.VCC(pin14), U7.VCC(pin14)
  GND: U3.GND(pin7), U7.GND(pin7)
  ```

- **KiCad netlist** (.net S-expression) — for import into PCBnew.
  Uses `DeriveKiCadFields()` to auto-populate part names and DIP
  footprints for 74xx components. Non-74xx components use
  `fsm-toolkit` as library and report as unresolved.

- **Structural JSON** — focused projection (components + nets + pin
  mappings only) for scripting, custom tooling, or web UIs.

SPICE was dropped: it requires behavioural models (`.subckt`
definitions) that the toolkit does not provide. A structural SPICE
netlist without models cannot simulate and has no practical value.

### KiCad field derivation

Two optional fields on `Class`:

- `KiCadPart` (string, omitempty) — e.g. `"74xx:7400"`
- `KiCadFootprint` (string, omitempty) — e.g. `"Package_DIP:DIP-14_W7.62mm"`

`DeriveKiCadFields()` populates these from the class name and pin count:

- Leading numeric prefix starting with "74" → `74xx:<number>`
- Pin count → `Package_DIP:DIP-<n>_W7.62mm` (or `W15.24mm` for >28 pins)

Derivation does not overwrite existing values (user overrides are respected).
The Build() function works on copies — the source FSM is never mutated.

### CLI interface

```
fsm netlist [--format text|kicad|json] [--machine NAME] [-o OUTPUT] FILE
```

Defaults to `text` format on stdout. Warnings and status go to stderr.

---

## File Format Changes

### JSON

The `"nets"` field uses `omitempty`. Existing files without nets
round-trip unchanged. No version bump needed — the field is purely
additive.

The `"ports"` field on classes also uses `omitempty`. Existing class
definitions remain valid.

### Hex binary format

A new section type is required for nets. The hex format version must
be bumped. Older toolkit versions reading a new-format file will
encounter the unknown section type. The reader must:

- Check the format version header.
- Skip unknown sections gracefully (already the case if section
  lengths are encoded; verify this).
- Emit a warning if nets are present but the reader doesn't
  understand them.

### Labels.toml

A `[nets]` section is added:

```toml
[nets]
DATA_IN = "U3.3Y, U7.2D"
CLK_BUS = "U3.6Y, U7.2CLK, U12.4CLK"
```

Older readers ignore unknown sections.

### .classes.json

The `"ports"` array sits alongside `"properties"`. Older toolkit
versions loading a library with ports will ignore the unknown field
(Go's JSON unmarshaller drops unknown keys by default).

---

## Settings and Toggles

### New settings

| Setting | Values | Default | Location |
|---------|--------|---------|----------|
| Canvas view mode | Behavioural / Structural / Both | Behavioural | Settings screen |
| Hidden-view edit notification | On / Off | On | Settings screen |

The view mode can also be toggled with a keyboard shortcut (proposed:
`W` to cycle through modes) without opening Settings.

### Decision D4 — Hidden-view edit behaviour

When the user adds a transition while in Structural view, or adds a
net while in Behavioural view, and the notification setting is On:

- A status bar message appears for 3 seconds.
- The message names the element type and suggests the view to see it.
- The edit is applied regardless.

When the notification setting is Off, edits proceed silently.

---

## Undo/Redo

All net operations are undoable:

| Operation | Undo captures |
|-----------|---------------|
| Add net | Full net snapshot |
| Remove net | Full net snapshot |
| Add endpoint to net | Net state before addition |
| Remove endpoint | Net state before removal |
| Rename net | Previous name |

The undo stack already captures FSM state snapshots. Nets, being part
of the FSM struct, are included automatically if the undo system
snapshots the full FSM. Verify this is the case during implementation;
if the undo system uses targeted field captures instead of full
snapshots, add net-specific capture.

---

## Design Decisions Log

### D1 — Power nets are invisible on the canvas

Power-direction ports (`VCC`, `GND`) create nets that are valid in
the data model and exported in netlists, but are excluded from canvas
rendering and from the summary line direction calculation. Without
this filter, every component pair would show a spurious ochre line
through shared power rails.

### D2 — Multi-fan-out footnotes in the detail window

When a net visible in the connection detail window has endpoints
beyond the two components being examined, a footnote line lists the
other endpoints. This is a designer guardrail: it surfaces the full
extent of a net without requiring the user to open other windows.

### D3 — Self-connection footnotes in the detail window

When a net connects two ports on the same component (e.g., gate A
output to gate B input on a 7400), the detail window shows the
connection and adds a footnote: "Internal connection within same
package." The `Group` field on ports provides the sub-unit context
(e.g., "GATE_A.3Y → GATE_B.4A"). Without the Group field, the
footnote still appears but without sub-unit labelling.

### D4 — Hidden-view edit notification is a setting

Adding a behavioural element while in Structural view (or vice versa)
can optionally trigger a status bar notification. The user controls
this from the Settings screen. Default: On.

### D5 — Sub-unit decomposition is deferred

A 7400 package contains four NAND gates. The current model treats
the whole package as one state (one component instance). Individual
gates are identified by the `Group` field on ports, not by separate
state objects. Full sub-unit decomposition (U3A, U3B, U3C, U3D as
separate entities) is deferred. Pin numbers and group fields are
sufficient for netlist export. The detail window uses groups for
visual organisation.

### D6 — View mode is display-only

Toggling between Behavioural, Structural, and Both views affects
rendering only. It does not restrict which operations are available.
The user can always add transitions and nets regardless of view.

---

## Implementation Order

1. **Phase 1** — Port model (`pkg/fsm/port.go`), Class extension,
   helper methods, class library JSON updates. Tests.
   **DONE** — 15 tests, 49 components migrated.
2. **Phase 2** — Net model (`pkg/fsm/net.go`), FSM extension,
   cascade rules, validation. Tests. File format persistence
   (.fsm, .json, labels.toml, bundle).
   **DONE** — 36 net tests + 5 persistence tests.
3. **Phase 3** — Canvas rendering: summary lines, colour/direction
   logic, power filtering, view toggle (`N` key), self-loops.
   **DONE** — `drawNets`, `drawNetDirect`, `drawNetBus`, `drawNetVStub`.
4. **Phase 4** — Connection detail window: modal overlay, port
   grouping, footnotes, scrolling, add/delete/rename controls.
   Entry via `E` key on selected state.
   **DONE** — `net_detail.go`, peer picker, chained input prompts.
5. **Phase 5** — Export formats: text netlist, KiCad, JSON.
   CLI `fsm netlist` command. SPICE dropped (no behavioural models).
   **DONE** — `pkg/export/netlist.go`, 14 tests, CLI wired.

Each phase is independently testable and backward-compatible.
