# FSM Toolkit Manual

## Table of Contents

1. [Introduction](#introduction)
2. [Correctness Model](#correctness-model)
3. [Concepts](#concepts)
4. [File Format Specification](#file-format-specification)
5. [Go CLI Tools](#go-cli-tools)
6. [Python Scripts](#python-scripts)
7. [Go Packages](#go-packages)
8. [Examples](#examples)

**See also:**
- [SPECIFICATION.md](SPECIFICATION.md) — Formal semantic guarantees
- [COMPATIBILITY.md](COMPATIBILITY.md) — Version stability promises

---

## Introduction

The FSM Toolkit provides a compact binary format for representing finite state machines, along with tools for conversion, visualisation, and interactive execution.

### Design Goals

- **Compact representation**: A 24-floor elevator FSM fits in ~5KB
- **Human-readable when needed**: Optional labels preserve meaningful names
- **Universal**: Supports DFA, NFA, Moore, and Mealy machines
- **Portable**: Both Python and Go implementations

### Quick Start

```bash
# Build the Go tools
./build.sh

# Convert JSON to .fsm format
./bin/fsm convert examples/test_moore.json -o traffic.fsm

# Visualise (with Graphviz)
./bin/fsm png traffic.fsm

# Visualise (without Graphviz)
./bin/fsm svg traffic.fsm --native

# Run interactively
./bin/fsm run traffic.fsm

# Edit visually
./bin/fsmedit traffic.fsm
```

---

## Correctness Model

Understanding what "valid" and "clean" mean is essential for using the toolkit effectively.

### Validation vs Analysis

| Check Type | Purpose | Failure Means |
|------------|---------|---------------|
| **Validation** | Structural correctness | FSM cannot run |
| **Analysis** | Design quality | FSM can run but may have issues |

### What "Valid" Means

An FSM is **valid** if it can be executed without runtime errors:

- Initial state exists and is in the state list
- All transition targets exist
- All inputs are in the alphabet
- Type-specific constraints are met (e.g., no epsilon in DFA)
- Outputs are in output alphabet (if alphabet is defined)

**Guarantee**: If `fsm validate` passes, `fsm run` will not crash.

### What "Clean" Means

An FSM is **clean** if it passes validation AND analysis with no warnings:

- All states are reachable from initial
- No dead-end states (non-accepting with no exits)
- DFA is deterministic and complete
- All alphabet symbols are used

**Guarantee**: If `fsm analyse` shows no warnings, the FSM has no structural issues.

### Enforcement Levels

| Constraint | Enforced? | If Violated |
|------------|-----------|-------------|
| Initial state exists | Always | Error |
| Transitions reference valid states | Always | Error |
| Inputs in alphabet | Always | Error |
| Outputs in output alphabet | If alphabet defined | Error |
| DFA has no epsilon | Always | Error |
| DFA is deterministic | Warning only | Runs as NFA |
| DFA is complete | Warning only | Rejects on missing |
| Moore has all outputs | Never | Missing = "" |
| All states reachable | Warning only | Dead code |

### Quick Reference

| Question | Answer |
|----------|--------|
| Can a DFA have epsilon transitions? | No — validation error |
| Can a DFA have multiple transitions on same input? | Yes — warning, runs as NFA |
| Can a DFA be incomplete (missing transitions)? | Yes — warning, rejects on missing |
| Must Moore machines have outputs for all states? | No — missing outputs return "" |
| Is output alphabet enforced? | Only if defined |
| Will my FSM load in future versions? | Yes — see COMPATIBILITY.md |

For complete semantic definitions, see [SPECIFICATION.md](SPECIFICATION.md).

---

## Concepts

### Finite State Machine Types

| Type | Memory | Output | Description |
|------|--------|--------|-------------|
| **DFA** | None | Accept/Reject | Deterministic Finite Automaton. One transition per (state, input) pair. |
| **NFA** | None | Accept/Reject | Non-deterministic. Multiple transitions allowed, plus epsilon (spontaneous) transitions. |
| **Moore** | None | Per state | Output determined by current state only. |
| **Mealy** | None | Per transition | Output determined by (state, input) pair. |

### Formal Definition

A finite state machine is a 5-tuple **M = (Q, Σ, δ, q₀, F)** where:

- **Q** — Finite set of states
- **Σ** — Input alphabet
- **δ** — Transition function
- **q₀** — Initial state
- **F** — Set of accepting states

For Moore/Mealy machines, add:

- **Γ** — Output alphabet
- **λ** — Output function (Q → Γ for Moore, Q × Σ → Γ for Mealy)

### Capacity Limits

| Property | Maximum |
|----------|---------|
| States | 65,536 |
| Input symbols | 65,535 |
| Output symbols | 65,536 |
| Transitions | Unlimited |

---

## File Format Specification

### Overview

The `.fsm` file format is a ZIP archive containing:

| File | Required | Description |
|------|----------|-------------|
| `machine.hex` | Yes | Binary state machine data |
| `labels.toml` | No | Human-readable names |
| `layout.toml` | No | Visual editor positions |

### machine.hex Format

Each record is 20 hexadecimal characters arranged as:

```
TYPE SSSS:IIII TTTT:OOOO
```

**Field layout:**

| Position | Field | Size | Description |
|----------|-------|------|-------------|
| 0-3 | TYPE | 16 bits | Record type |
| 5-8 | SSSS | 16 bits | Source state ID |
| 10-13 | IIII | 16 bits | Input symbol ID |
| 15-18 | TTTT | 16 bits | Target state ID |
| 20-23 | OOOO | 16 bits | Output/flags |

Separators (space and colon) are for readability and ignored during parsing.

### Version Compatibility

The hex format uses record type codes for extensibility:

**Current record types:** 0000-0003

**Compatibility rules:**

1. **Forward compatibility**: Readers should ignore unknown record types (≥0004). This allows older tools to read files with newer record types, skipping features they don't understand.

2. **Backward compatibility**: New tools must support all existing record types (0000-0003).

3. **Type reservation**: Record types 0000-00FF are reserved for core FSM semantics. Types 0100-FFFF are available for extensions.

4. **Breaking changes**: If a future version changes the semantics of an existing record type, it must use a new type code instead.

The labels.toml and layout.toml sections are optional metadata. Readers that don't support TOML should ignore everything after the hex records.

### Record Types

#### Type 0000: DFA/NFA Transition

```
0000 SSSS:IIII TTTT:0000
```

- SSSS: Source state
- IIII: Input symbol (FFFF = epsilon)
- TTTT: Target state
- OOOO: Reserved (0000)

#### Type 0001: Mealy Transition

```
0001 SSSS:IIII TTTT:OOOO
```

- SSSS: Source state
- IIII: Input symbol
- TTTT: Target state
- OOOO: Output symbol

#### Type 0002: State Declaration

```
0002 SSSS:FFFF OOOO:0000
```

- SSSS: State ID
- FFFF: Flags
  - Bit 0: Initial state
  - Bit 1: Accepting state
- OOOO: Moore output (0 = none, otherwise output_id + 1)

#### Type 0003: NFA Multi-Target

```
0003 SSSS:IIII TTTT:CCCC
```

Used when one (state, input) pair leads to multiple target states.

- SSSS: Source state
- IIII: Input symbol
- TTTT: Target state
- CCCC: Continuation (0001 = more targets follow, 0000 = last)

### labels.toml Format

```toml
[fsm]
version = 1
type = "moore"
name = "Traffic Light Controller"
description = "A simple 3-state traffic light"

[states]
0x0000 = "green"
0x0001 = "yellow"
0x0002 = "red"

[inputs]
0x0000 = "timer"

[outputs]
0x0000 = "go"
0x0001 = "caution"
0x0002 = "stop"
```

**Sections:**

| Section | Description |
|---------|-------------|
| `[fsm]` | Metadata: version, type, name, description |
| `[states]` | Map of state ID (hex) to name |
| `[inputs]` | Map of input ID (hex) to name |
| `[outputs]` | Map of output ID (hex) to name |

Keys must be hexadecimal with `0x` prefix. Values are quoted strings.

### layout.toml Format

Optional file storing visual editor positions:

```toml
[layout]
version = 1

[editor]
canvas_offset_x = 0
canvas_offset_y = 0

[states."green"]
x = 5
y = 2

[states."yellow"]
x = 20
y = 2

[states."red"]
x = 35
y = 2
```

**Sections:**

| Section | Description |
|---------|-------------|
| `[layout]` | Metadata version |
| `[editor]` | Canvas scroll offset |
| `[states."name"]` | Per-state X/Y coordinates |

If `layout.toml` is missing, fsmedit generates a default grid layout.

---

## Go CLI Tools

### Building

```bash
cd fsm-toolkit
go build -o fsm ./cmd/fsm/
go build -o fsmedit ./cmd/fsmedit/
```

### fsm - Command Line Tool

#### Commands

| Command | Description |
|---------|-------------|
| `convert` | Convert between formats (json, hex, fsm) |
| `dot` | Generate Graphviz DOT output |
| `png` | Generate PNG image |
| `svg` | Generate SVG image |
| `generate` | Generate code (C, Rust, Go/TinyGo) |
| `info` | Show FSM information |
| `analyse` | Analyse for potential issues |
| `validate` | Validate FSM structure |
| `run` | Interactive execution |
| `view` | Visualise (PNG + open viewer) |
| `edit` | Open visual editor (invokes fsmedit) |

#### convert

Convert between JSON, hex, and .fsm formats. Supports wildcards for batch conversion.

```bash
fsm convert <input>... [-o output] [--pretty] [--no-labels]
```

**Options:**

| Option | Description |
|--------|-------------|
| `-o, --output` | Output file or extension (default: change extension) |
| `--pretty` | Pretty-print JSON output |
| `--no-labels` | Omit labels.toml from .fsm output |

**Examples:**

```bash
# JSON to .fsm with labels
fsm convert input.json -o output.fsm

# JSON to .fsm without labels (smaller file)
fsm convert input.json --no-labels -o output.fsm

# .fsm to pretty JSON
fsm convert input.fsm -o output.json --pretty

# JSON to raw hex
fsm convert input.json -o output.hex

# Batch convert all JSON files to .fsm
fsm convert *.json -o .fsm

# Batch convert with pretty JSON
fsm convert examples/*.fsm -o .json --pretty
```

When `-o` starts with a dot (e.g., `.fsm`), it is treated as an extension applied to each input file's basename.

#### dot

Generate Graphviz DOT output.

```bash
fsm dot <input> [-o output] [-t title]
```

**Options:**

| Option | Description |
|--------|-------------|
| `-o, --output` | Output file (default: stdout) |
| `-t, --title` | Graph title |

**Examples:**

```bash
# Generate PNG
fsm dot input.fsm | dot -Tpng -o output.png

# Generate SVG with custom title
fsm dot input.fsm -t "My State Machine" | dot -Tsvg -o output.svg

# Save DOT file
fsm dot input.fsm -o output.dot
```

#### png

Generate PNG image directly (shorthand for `dot | dot -Tpng`).

```bash
fsm png <input> [-o output] [-t title]
```

**Options:**

| Option | Description |
|--------|-------------|
| `-o, --output` | Output file (default: input name + .png) |
| `-t, --title` | Diagram title |

**Examples:**

```bash
# Generate beatles.png from beatles.fsm
fsm png beatles.fsm

# Custom output and title
fsm png beatles.fsm -o fab_four.png -t "Fab Four Workflow"
```

Requires Graphviz `dot` to be installed.

#### svg

Generate SVG image directly (shorthand for `dot | dot -Tsvg`).

```bash
fsm svg <input> [-o output] [-t title] [--native]
```

Same options as `png`. SVG is useful for web embedding and scalable graphics.

**Options:**

| Option | Description |
|--------|-------------|
| `-o, --output` | Output file (default: input with .svg extension) |
| `-t, --title` | Diagram title |
| `--native` | Use built-in renderer (no Graphviz required) |

```bash
# Using Graphviz (better layout)
fsm svg beatles.fsm -o diagram.svg

# Using native renderer (no dependencies)
fsm svg beatles.fsm -o diagram.svg --native
```

The native renderer uses the built-in layout algorithms and produces clean SVG output without requiring Graphviz. It's useful for:
- Systems without Graphviz installed
- CI/CD pipelines
- Embedded/minimal environments

#### info

Display FSM information.

```bash
fsm info <input>
```

**Output:**

```
Type:        moore
Name:        Traffic Light
States:      3
Inputs:      1
Outputs:     3
Transitions: 3
Initial:     green
Accepting:   []

States:      [green yellow red]
Alphabet:    [timer]
Outputs:     [go caution stop]
```

#### validate

Check FSM for errors.

```bash
fsm validate <input>
```

Validates:
- All referenced states exist
- All referenced inputs exist
- Initial state is defined
- Accepting states exist

#### analyse

Analyse FSM for potential design issues (warnings, not errors).

```bash
fsm analyse <input>
```

Also accepts American spelling: `fsm analyze`

**Checks performed:**

| Check | Description |
|-------|-------------|
| `unreachable` | States not reachable from initial state |
| `dead` | States with no outgoing transitions (except accepting states) |
| `nondeterministic` | DFA states with multiple transitions on same input |
| `incomplete` | DFA states missing transitions for some inputs |
| `unused_input` | Inputs defined but never used in transitions |
| `unused_output` | Outputs defined but never used (Moore/Mealy) |

**Example output:**

```
Found 3 issue(s):

  [unreachable] 1 state(s) not reachable from initial state
    States: [orphan]
  [dead] 1 state(s) have no outgoing transitions
    States: [sink]
  [unused_input] 2 input(s) not used in any transition
```

#### generate

Generate executable code from FSM definition.

```bash
fsm generate <input> --lang <c|rust|go|tinygo> [-o output] [--package name]
```

**Options:**

| Option | Description |
|--------|-------------|
| `--lang, -l` | Target language (required) |
| `-o, --output` | Output file (default: stdout) |
| `--package, -p` | Package name (Go only, default: fsm) |

**Languages:**

| Language | Output | Description |
|----------|--------|-------------|
| `c` | Header file (.h) | Single-file header with implementation |
| `rust` | Module (.rs) | Idiomatic Rust with enums |
| `go` | Package (.go) | Standard Go, TinyGo compatible |
| `tinygo` | Package (.go) | Alias for `go` |

**Examples:**

```bash
# Generate C header
fsm generate machine.fsm --lang c -o machine.h

# Generate Rust module
fsm generate machine.fsm --lang rust -o machine.rs

# Generate Go package
fsm generate machine.fsm --lang go --package myfsm -o myfsm.go
```

See the **Code Generation** section below for detailed usage and limitations.

#### run

Run FSM interactively.

```bash
fsm run <input>
```

**Interactive commands:**

| Command | Description |
|---------|-------------|
| `<input>` | Send input symbol to FSM |
| `reset` | Return to initial state |
| `status` | Show current state and output |
| `history` | Show execution history |
| `inputs` | List available inputs |
| `help` | Show help |
| `quit` | Exit |

**Example session:**

```
$ fsm run traffic_light.fsm
FSM: Traffic Light (moore)
Commands: <input>, reset, status, history, inputs, quit

State: green -> go
> timer
Output: caution
State: yellow -> caution
> timer
Output: stop
State: red -> stop
> history
History:
  1: green --timer--> yellow [caution]
  2: yellow --timer--> red [stop]
> reset
Reset to initial state
State: green -> go
> quit
```

#### view

Visualise FSM as PNG and open with system viewer.

```bash
fsm view <input> [-t title]
```

**Options:**

| Option | Description |
|--------|-------------|
| `-t, --title` | Set diagram title (default: FSM name) |

**Requirements:**

Requires Graphviz `dot` command. Install from https://graphviz.org/download/

```bash
# macOS
brew install graphviz

# Ubuntu/Debian
sudo apt install graphviz

# Windows
choco install graphviz
```

**Behaviour:**

1. Generates DOT representation
2. Converts to PNG using `dot`
3. Opens PNG with system viewer:
   - macOS: `open`
   - Linux: `xdg-open`
   - Windows: `explorer.exe`

**Examples:**

```bash
# View FSM diagram
fsm view beatles.fsm

# With custom title
fsm view beatles.fsm -t "Fab Four Workflow"
```

#### edit

Open the visual FSM editor (fsmedit).

```bash
fsm edit [file]
```

**Search order for fsmedit:**

1. `PATH` environment variable
2. Current working directory
3. Same directory as the `fsm` executable

**Examples:**

```bash
# Start with empty FSM
fsm edit

# Open existing file
fsm edit examples/beatles.fsm
```

This is a convenience wrapper that locates and invokes `fsmedit`. All arguments are passed through.

### fsmedit - Visual Editor

A text-based visual editor for creating and modifying FSMs.

```bash
# Start with empty FSM
./fsmedit

# Open existing file
./fsmedit examples/traffic_light.fsm
```

#### Interface

The editor has three main areas:

1. **Canvas** (left) - Visual state placement and cursor
2. **Sidebar** (right) - Lists states, inputs, outputs, transitions
3. **Status bar** (bottom) - File info, mode, messages, help

#### Modes

| Mode | Description |
|------|-------------|
| MENU | Main menu for file operations |
| CANVAS | Edit states and transitions visually |
| INPUT | Text input for names |
| FILE SELECT | Choose file to open |
| SELECT TYPE | Choose FSM type |

#### Main Menu

| Item | Description |
|------|-------------|
| New FSM | Create a new empty FSM |
| Open File | Open an existing .fsm or .json file |
| Save | Save to current filename |
| Save As | Save to a new filename |
| Edit Canvas | Enter canvas editing mode |
| Render (Graphviz) | Generate PNG and open in system viewer |
| Set FSM Type | Change FSM type (DFA/NFA/Moore/Mealy) |
| Quit | Exit the editor |

#### Canvas Mode Keys

| Key | Action |
|-----|--------|
| Arrow keys | Move cursor |
| Enter | Add state at cursor |
| Tab | Cycle through states |
| Delete/Backspace | Delete selected state |
| T | Add transition from selected state |
| I | Add input symbol |
| O | Add output symbol |
| S | Set selected as initial state |
| A | Toggle accepting state |
| M | Set Moore output (Moore machines) |
| G | Grab/move selected state (enter move mode) |
| L | Lint/analyse FSM for warnings |
| V | Validate FSM structure (errors) |
| R | Render with Graphviz and open viewer |
| W | Toggle arc (wire) visibility |
| Esc | Return to menu |

#### Move Mode Keys

When in move mode (after pressing G on a selected state):

| Key | Action |
|-----|--------|
| Arrow keys | Move the state |
| Enter | Confirm new position |
| Esc | Cancel and restore original position |

#### Global Keys

| Key | Action |
|-----|--------|
| Ctrl+C (Cmd+C) | Copy FSM as hex to clipboard |
| Ctrl+S (Cmd+S) | Save |
| Ctrl+Z (Cmd+Z) | Undo |
| Ctrl+Y (Cmd+Y) | Redo |

#### Mouse Support

- **Left click** on canvas to move cursor
- **Left click** on state to select it
- **Left click + drag** on state to reposition it
- **Left click** on menu items to activate
- **Right click + drag** on state to reposition (alternative method)

Dragging works with both left and right mouse buttons, making it accessible on laptops with touchpads.

#### Layout Persistence

State positions are automatically saved to `layout.toml` inside the .fsm file. When reopening a file, states appear where you left them.

#### Automatic Layout

When opening an FSM without saved positions, fsmedit automatically arranges states using a smart layout algorithm:

| Algorithm | Used When | Description |
|-----------|-----------|-------------|
| Hierarchical | Linear chains, small FSMs (≤8 states) | BFS layers from initial state |
| Circular | Medium FSMs (5-15 states) | States around a circle |
| Force-Directed | Dense graphs (>15 states) | Physics simulation for spacing |

The algorithm is chosen based on:
- Number of states
- Graph structure (linear, cyclic, dense)
- Transition density

After auto-layout, you can drag states to refine positions.

#### Workflow Example

1. Start editor: `./fsmedit`
2. Select "New FSM" from menu
3. Press Esc to enter canvas mode
4. Use arrow keys to position cursor
5. Press Enter to add a state
6. Press I to add input symbols
7. Select a state with Tab
8. Press T to add transition
9. Press Ctrl+S to save

---

## Python Scripts

### fsm_converter.py

Full-featured converter supporting all formats.

```bash
python3 fsm_converter.py --to-fsm <input> [-o output] [--no-labels]
python3 fsm_converter.py --to-json <input> [-o output] [--pretty]
python3 fsm_converter.py --to-hex <input> [-o output] [--width N]
```

**Options:**

| Option | Description |
|--------|-------------|
| `--to-fsm` | Convert to .fsm format |
| `--to-json` | Convert to JSON format |
| `--to-hex` | Convert to raw hex format |
| `-o, --output` | Output file |
| `--pretty` | Pretty-print JSON |
| `--no-labels` | Omit labels.toml |
| `--width` | Records per line (default: 4) |

### fsm_visualise.py

Generate Graphviz DOT from any format.

```bash
python3 fsm_visualise.py <input> [-o output] [-t title] [--format FORMAT]
```

**Options:**

| Option | Description |
|--------|-------------|
| `-o, --output` | Output file (default: stdout) |
| `-t, --title` | Graph title |
| `--format` | Force input format (json, hex, fsm) |

### elevator_fsm.py

Generate a sample 24-floor elevator FSM.

```bash
python3 elevator_fsm.py > elevator.hex
```

---

## Code Generation

The `fsm generate` command produces executable code from FSM definitions. Generated code is standalone with no runtime dependencies.

### Use Cases

**Embedded systems**: Generate C code for microcontrollers. The code uses only `uint16_t` types and switch statements — no heap allocation, no dynamic dispatch.

**Rust applications**: Generate type-safe Rust with pattern matching. Integrates naturally with Rust's ownership model.

**Go/TinyGo**: Generate Go packages suitable for both standard Go and TinyGo (for WebAssembly, embedded). Uses `uint16` types and avoids reflection.

**Protocol implementations**: Model protocol state machines (TCP, USB, custom protocols) and generate the state handling code.

**Game AI**: Define NPC behaviour as FSMs, generate code for game engines.

### Generated API

All languages generate equivalent APIs:

| Function | C | Rust | Go |
|----------|---|------|-----|
| Create/init | `name_init(&fsm)` | `Name::new()` | `NewName()` |
| Reset | `name_reset(&fsm)` | `fsm.reset()` | `fsm.Reset()` |
| Step | `name_step(&fsm, input)` | `fsm.step(input)` | `fsm.Step(input)` |
| Can step | `name_can_step(&fsm, input)` | `fsm.can_step(input)` | `fsm.CanStep(input)` |
| Get state | `name_get_state(&fsm)` | `fsm.state()` | `fsm.State()` |
| Get output | `name_get_output(&fsm)` | `fsm.output()` | `fsm.Output()` |
| Is accepting | `name_is_accepting(&fsm)` | `fsm.is_accepting()` | `fsm.IsAccepting()` |
| State name | `name_state_name(state)` | `state.to_string()` | `state.String()` |
| Input name | `name_input_name(input)` | `input.to_string()` | `input.String()` |
| Output name | `name_output_name(output)` | `output.to_string()` | `output.String()` |

### C Output

Header-only library using `#define` guards:

```c
// In exactly ONE .c file:
#define MYFSM_IMPLEMENTATION
#include "myfsm.h"

// In other files:
#include "myfsm.h"

// Usage:
myfsm_t fsm;
myfsm_init(&fsm);

if (myfsm_can_step(&fsm, MYFSM_INPUT_START)) {
    myfsm_step(&fsm, MYFSM_INPUT_START);
    printf("State: %s\n", myfsm_state_name(myfsm_get_state(&fsm)));
}
```

**Features:**
- `typedef uint16_t` for state/input/output types
- `#define` constants for states, inputs, outputs
- Name lookup functions for debugging
- No heap allocation
- C89 compatible (except for `bool` from `<stdbool.h>`)

### Rust Output

Idiomatic Rust module:

```rust
use myfsm::{MyFsm, MyFsmInput, MyFsmState};

let mut fsm = MyFsm::new();

if fsm.can_step(MyFsmInput::Start) {
    fsm.step(MyFsmInput::Start);
    println!("State: {}", fsm.state());
}
```

**Features:**
- `#[repr(u16)]` enums for compact representation
- `#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]` on enums
- `std::fmt::Display` implementations
- `Default` implementation
- Pattern matching in `step()` and `can_step()`

### Go Output

Standard Go package (TinyGo compatible):

```go
import "myfsm"

fsm := myfsm.NewMyFsm()

if fsm.CanStep(myfsm.MyFsmInputStart) {
    fsm.Step(myfsm.MyFsmInputStart)
    fmt.Println("State:", fsm.State())
}
```

**Features:**
- `uint16` types for state/input/output
- `String()` methods implementing `fmt.Stringer`
- No reflection, no `interface{}`
- No heap allocation in `Step()`
- Compatible with TinyGo for WASM/embedded

### Limitations

**NFA in code generation**: NFAs are automatically converted to DFAs (powerset construction) before code generation. The resulting DFA may have composite state names (e.g., `q0_q1_q2`). For very large NFAs with many epsilon transitions, the DFA state explosion can produce large code.

**No runtime modification**: The FSM structure is compiled into switch statements. You cannot add/remove states or transitions at runtime.

**Large FSMs**: For FSMs with thousands of states, the generated switch statements may be large. Consider table-driven approaches for very large FSMs.

**No history/trace**: Generated code only tracks current state and output. For debugging, you need to add your own logging around `step()` calls.

**Name sanitisation**: State/input/output names are converted to valid identifiers. Special characters become underscores; names starting with digits get a prefix.

### Examples

Generate code for the Beatles FSM:

```bash
# C header
fsm generate beatles.fsm --lang c -o beatles.h

# Rust module  
fsm generate beatles.fsm --lang rust -o beatles.rs

# Go package
fsm generate beatles.fsm --lang go --package beatles -o beatles/beatles.go
```

---

## Go Packages

### pkg/fsm

Core FSM types and runner.

#### Types

```go
type Type string

const (
    TypeDFA   Type = "dfa"
    TypeNFA   Type = "nfa"
    TypeMoore Type = "moore"
    TypeMealy Type = "mealy"
)

type Transition struct {
    From   string
    Input  *string  // nil for epsilon
    To     []string // multiple for NFA
    Output *string  // Mealy only
}

type FSM struct {
    Type           Type
    Name           string
    Description    string
    States         []string
    Alphabet       []string
    Initial        string
    Accepting      []string
    Transitions    []Transition
    StateOutputs   map[string]string // Moore
    OutputAlphabet []string
}
```

#### Functions

```go
// Create new FSM
func New(t Type) *FSM

// FSM methods
func (f *FSM) AddState(name string)
func (f *FSM) AddInput(symbol string)
func (f *FSM) AddOutput(symbol string)
func (f *FSM) AddTransition(from string, input *string, to []string, output *string)
func (f *FSM) SetInitial(state string)
func (f *FSM) SetAccepting(states []string)
func (f *FSM) SetStateOutput(state, output string)
func (f *FSM) Validate() error
func (f *FSM) IsAccepting(state string) bool
func (f *FSM) GetTransitions(from string, input *string) []Transition
```

#### Runner

```go
// Create runner
runner, err := fsm.NewRunner(f)

// Runner methods
func (r *Runner) CurrentState() string
func (r *Runner) CurrentOutput() string  // Moore only
func (r *Runner) IsAccepting() bool
func (r *Runner) AvailableInputs() []string
func (r *Runner) Step(input string) (output string, err error)
func (r *Runner) Reset()
func (r *Runner) History() []Step
func (r *Runner) Run(inputs []string) ([]string, error)
```

### pkg/fsmfile

File format handling.

#### Reading/Writing

```go
// .fsm files
func ReadFSMFile(path string) (*fsm.FSM, error)
func WriteFSMFile(path string, f *fsm.FSM, includeLabels bool) error

// JSON
func ParseJSON(data []byte) (*fsm.FSM, error)
func ToJSON(f *fsm.FSM, pretty bool) ([]byte, error)

// Hex records
func ParseHex(text string) ([]Record, error)
func FormatHex(records []Record, width int) string
func FSMToRecords(f *fsm.FSM) ([]Record, map[int]string, map[int]string, map[int]string)
func RecordsToFSM(records []Record, labels *Labels) (*fsm.FSM, error)

// DOT (requires Graphviz)
func GenerateDOT(f *fsm.FSM, title string) string

// Native SVG (no external dependencies)
func GenerateSVGNative(f *fsm.FSM, opts SVGOptions) string
func DefaultSVGOptions() SVGOptions

// Labels
func GenerateLabels(f *fsm.FSM, states, inputs, outputs map[int]string) string
func ParseLabels(text string) (*Labels, error)

// Layout
func SmartLayout(f *fsm.FSM, width, height int) map[string][2]int
func AutoLayout(f *fsm.FSM, algorithm LayoutAlgorithm, width, height int) map[string][2]int
```

#### Native SVG Rendering

The `GenerateSVGNative` function produces SVG output without requiring Graphviz:

```go
opts := fsmfile.DefaultSVGOptions()
opts.Title = "My FSM"
opts.Width = 800
opts.Height = 600
opts.StateShape = fsmfile.ShapeEllipse
opts.NodeSpacing = 1.5
svg := fsmfile.GenerateSVGNative(myFSM, opts)
```

**SVGOptions fields:**

| Field | Default | Description |
|-------|---------|-------------|
| `Width` | 800 | Canvas width in pixels |
| `Height` | 600 | Canvas height in pixels |
| `Title` | "" | Diagram title |
| `FontSize` | 14 | Base font size for state labels |
| `LabelSize` | 12 | Font size for transition labels |
| `TitleSize` | 18 | Font size for diagram title |
| `StateRadius` | 30 | Base radius/height of state shapes |
| `StateShape` | Ellipse | Shape of state nodes (see below) |
| `Padding` | 50 | Padding around edges |
| `NodeSpacing` | 1.5 | Multiplier for spacing between nodes |

**State shapes:**

| Shape | Constant | Description |
|-------|----------|-------------|
| Circle | `ShapeCircle` | Fixed-size circles |
| Ellipse | `ShapeEllipse` | Ellipses sized to fit label (default) |
| Rectangle | `ShapeRect` | Rectangles sized to fit label |
| Rounded Rect | `ShapeRoundRect` | Rounded rectangles |
| Diamond | `ShapeDiamond` | Diamond/rhombus shape |

**CLI options for native SVG:**

```bash
fsm svg input.json --native [options]

Options:
  --font-size N    Base font size (default: 14)
  --shape SHAPE    circle, ellipse, rect, roundrect, diamond
  --spacing N      Node spacing multiplier (default: 1.5)
  --width N        Canvas width (default: 800)
  --height N       Canvas height (default: 600)
```

**Features:**
- Automatic layout using `SmartLayout` (Sugiyama algorithm)
- State colouring: green (initial), orange (accepting), blue (both)
- Double outline for accepting states
- Self-loops rendered above states
- Curved arrows for long edges and back-edges
- Curved arrows for bidirectional transitions
- Mealy outputs shown as `input/output` on transitions
- Moore outputs shown in italic below states

#### Layout Algorithms

The toolkit includes five layout algorithms:

| Algorithm | Best for |
|-----------|----------|
| `LayoutSugiyama` | **Default.** Layered graphs, DAGs, most FSMs |
| `LayoutHierarchical` | Simple linear chains |
| `LayoutCircular` | Highly cyclic FSMs |
| `LayoutForceDirected` | Dense graphs with many cross-edges |
| `LayoutGrid` | Fallback, uniform distribution |

**Sugiyama Layout** (the default) implements a layered graph algorithm inspired by Graphviz:

1. **Layer assignment** — BFS from initial state assigns each node to a layer
2. **Crossing minimisation** — barycenter heuristic reorders nodes within layers
3. **Horizontal positioning** — median heuristic aligns connected nodes
4. **Edge routing** — long edges and back-edges rendered as curves

`SmartLayout` uses Sugiyama for most FSMs, falling back to force-directed for very dense cyclic graphs.

```go
// Use Sugiyama explicitly
positions := fsmfile.AutoLayout(myFSM, fsmfile.LayoutSugiyama, 80, 40)

// Or let SmartLayout choose
positions := fsmfile.SmartLayout(myFSM, 80, 40)
```

#### Record Type

```go
type Record struct {
    Type   uint16
    Field1 uint16
    Field2 uint16
    Field3 uint16
    Field4 uint16
}

func FormatRecord(r Record) string
func ParseRecord(s string) (Record, error)
```

---

## Examples

The `examples/` directory contains a variety of FSM definitions:

| File | Type | Description |
|------|------|-------------|
| `traffic_light.fsm` | Moore | Simple traffic light controller |
| `turnstile.json` | Mealy | Classic locked/unlocked turnstile |
| `vending_machine.json` | Mealy | Coin-operated vending machine |
| `binary_divisible_by_3.json` | DFA | Accepts binary numbers divisible by 3 |
| `tcp_connection.json` | DFA | Simplified TCP state machine |
| `door_lock.json` | DFA | 4-digit code lock with auto-reset |
| `password_strength.json` | Moore | Password strength validator |
| `game_enemy_ai.json` | Moore | Game enemy behavior AI |
| `http_parser.json` | DFA | HTTP request line parser |
| `regex_ab_star.json` | NFA | Accepts strings matching `(ab)*` |

The `examples/bad/` directory contains FSMs with intentional errors for testing validation and analysis:

| File | Issue |
|------|-------|
| `missing_initial.json` | Initial state not in states list |
| `undefined_target.json` | Transition targets non-existent state |
| `dfa_with_epsilon.json` | DFA contains epsilon (null) transition |
| `invalid_output.json` | Output symbol not in output alphabet |
| `unreachable_states.json` | States not reachable from initial |
| `dead_ends.json` | States with no outgoing transitions |
| `nondeterministic_dfa.json` | DFA with multiple transitions on same input |
| `incomplete_dfa.json` | DFA missing transitions for some inputs |
| `unused_alphabet.json` | Input/output symbols never used |
| `multiple_issues.json` | Combination of several issues |

### Traffic Light (Moore)

A Moore machine where output depends on current state.

**JSON:**
```json
{
  "type": "moore",
  "states": ["green", "yellow", "red"],
  "alphabet": ["timer"],
  "initial": "green",
  "accepting": [],
  "transitions": [
    {"from": "green", "input": "timer", "to": "yellow"},
    {"from": "yellow", "input": "timer", "to": "red"},
    {"from": "red", "input": "timer", "to": "green"}
  ],
  "state_outputs": {
    "green": "go",
    "yellow": "caution",
    "red": "stop"
  },
  "output_alphabet": ["go", "caution", "stop"]
}
```

**Hex:**
```
0002 0000:0001 0001:0000   0002 0001:0000 0002:0000   0002 0002:0000 0003:0000
0000 0000:0000 0001:0000   0000 0001:0000 0002:0000   0000 0002:0000 0000:0000
```

### Vending Machine (Mealy)

A Mealy machine where output depends on transition.

**JSON:**
```json
{
  "type": "mealy",
  "states": ["idle", "five", "ten"],
  "alphabet": ["nickel", "dime", "button"],
  "initial": "idle",
  "accepting": [],
  "transitions": [
    {"from": "idle", "input": "nickel", "to": "five", "output": "none"},
    {"from": "idle", "input": "dime", "to": "ten", "output": "none"},
    {"from": "five", "input": "nickel", "to": "ten", "output": "none"},
    {"from": "five", "input": "dime", "to": "idle", "output": "dispense"},
    {"from": "ten", "input": "nickel", "to": "idle", "output": "dispense"},
    {"from": "ten", "input": "dime", "to": "idle", "output": "dispense_change"},
    {"from": "ten", "input": "button", "to": "idle", "output": "dispense"}
  ],
  "output_alphabet": ["none", "dispense", "dispense_change"]
}
```

### NFA with Epsilon

An NFA demonstrating epsilon transitions and multi-target transitions.

**JSON:**
```json
{
  "type": "nfa",
  "states": ["q0", "q1", "q2", "q3"],
  "alphabet": ["a", "b"],
  "initial": "q0",
  "accepting": ["q3"],
  "transitions": [
    {"from": "q0", "input": "a", "to": ["q0", "q1"]},
    {"from": "q0", "input": "b", "to": "q0"},
    {"from": "q1", "input": null, "to": "q2"},
    {"from": "q2", "input": "b", "to": "q3"}
  ]
}
```

Note: `"input": null` represents an epsilon (ε) transition.

### DFA Pattern Matcher

A DFA that accepts strings ending in "aa".

**JSON:**
```json
{
  "type": "dfa",
  "states": ["q0", "q1", "q2"],
  "alphabet": ["a", "b"],
  "initial": "q0",
  "accepting": ["q2"],
  "transitions": [
    {"from": "q0", "input": "a", "to": "q1"},
    {"from": "q0", "input": "b", "to": "q0"},
    {"from": "q1", "input": "a", "to": "q2"},
    {"from": "q1", "input": "b", "to": "q0"},
    {"from": "q2", "input": "a", "to": "q2"},
    {"from": "q2", "input": "b", "to": "q0"}
  ]
}
```

---

## Appendix: Size Comparison

| Format | Traffic Light | 24-Floor Elevator |
|--------|---------------|-------------------|
| JSON | ~450 bytes | ~40 KB |
| .fsm (with labels) | ~380 bytes | ~6 KB |
| .fsm (no labels) | ~160 bytes | ~5 KB |
| Raw hex | ~160 bytes | ~5 KB |

The .fsm format achieves 8-10x compression over JSON for complex FSMs.
