# FSM Toolkit Manual

## Table of Contents

1. [Introduction](#introduction)
2. [Concepts](#concepts)
3. [File Format Specification](#file-format-specification)
4. [Go CLI Tool](#go-cli-tool)
5. [Python Scripts](#python-scripts)
6. [Go Packages](#go-packages)
7. [Examples](#examples)

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
# Build the Go tool
go build -o fsm ./cmd/fsm

# Convert JSON to .fsm format
./fsm convert examples/test_moore.json -o traffic.fsm

# Visualise
./fsm dot traffic.fsm | dot -Tpng -o traffic.png

# Run interactively
./fsm run traffic.fsm
```

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

---

## Go CLI Tool

### Building

```bash
cd fsm-toolkit
go build -o fsm ./cmd/fsm
```

### Commands

#### convert

Convert between JSON, hex, and .fsm formats.

```bash
fsm convert <input> [-o output] [--pretty] [--no-labels]
```

**Options:**

| Option | Description |
|--------|-------------|
| `-o, --output` | Output file (default: change extension) |
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
```

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

// DOT
func GenerateDOT(f *fsm.FSM, title string) string

// Labels
func GenerateLabels(f *fsm.FSM, states, inputs, outputs map[int]string) string
func ParseLabels(text string) (*Labels, error)
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
