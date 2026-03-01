# Linked States and Bundles

This document describes how to create composite finite state machines using linked states and bundles in fsm-toolkit.

## Overview

A **bundle** is a `.fsm` file containing multiple machines that can work together. **Linked states** allow one machine to delegate processing to another machine in the bundle.

When an FSM enters a linked state, it spawns the linked child machine. The child processes input until it reaches an accepting state (accept) or a non-accepting terminal state (reject). The parent then transitions based on the child's result.

## Creating Bundles

### From the Command Line

Combine multiple `.fsm` files into a bundle:

```bash
fsm bundle main.fsm validator.fsm parser.fsm -o system.fsm
```

### Listing Machines in a Bundle

```bash
fsm machines system.fsm
```

Output:
```
system.fsm contains 3 machines:

  NAME        TYPE   STATES  TRANS  DESCRIPTION
  ----        ----   ------  -----  -----------
  main        DFA         5      8  Main controller
  validator   DFA         3      4  Input validator
  parser      DFA         7     12  Protocol parser
```

### Extracting Machines

Extract a single machine from a bundle:

```bash
fsm extract system.fsm -m validator -o validator.fsm
```

## Linked States

### Concept

A linked state delegates to another machine:

```
┌─────────────────────────────────────────┐
│  Parent Machine                          │
│                                          │
│   ┌───────┐      ┌─────────────┐        │
│   │ idle  │─────▶│  validating │        │
│   └───────┘      │   ↗validator │        │
│                  └──────┬──────┘        │
│                    accept│reject         │
│                  ┌───────┴───────┐       │
│                  ▼               ▼       │
│             ┌───────┐       ┌───────┐   │
│             │  ok   │       │ error │   │
│             └───────┘       └───────┘   │
└─────────────────────────────────────────┘
```

When the parent enters `validating`:
1. The `validator` machine is spawned
2. Input is routed to the child until it terminates
3. Parent receives `accept` or `reject` based on child's final state
4. Parent transitions accordingly

### Creating Linked States

#### In JSON

```json
{
  "type": "dfa",
  "name": "parent",
  "states": ["idle", "validating", "ok", "error"],
  "alphabet": ["start", "accept", "reject"],
  "initial": "idle",
  "accepting": ["ok"],
  "transitions": [
    {"from": "idle", "input": "start", "to": ["validating"]},
    {"from": "validating", "input": "accept", "to": ["ok"]},
    {"from": "validating", "input": "reject", "to": ["error"]}
  ],
  "linked_machines": {
    "validating": "validator"
  }
}
```

#### In fsmedit

1. Select a state (Tab to cycle, or click)
2. Press `k` to set link
3. Enter the target machine name (or select from list in bundle mode)

To unlink: select the linked state and press `k` again.

### Visual Indicators

Linked states are displayed with distinct styling:

| Format | Appearance |
|--------|------------|
| PNG/SVG | Purple fill (#f3e5f5), purple border (#8e24aa), dashed inner ring |
| fsmedit | Fuchsia colour, ↗ suffix, target machine name below |
| `fsm info` | Listed in "Linked States" section |

### Reserved Inputs

When a linked state's child machine terminates, the parent receives one of:

- `accept` — child reached an accepting state
- `reject` — child terminated in a non-accepting state

These should be defined in your parent machine's alphabet if you want explicit transitions on child completion.

## labels.toml Format

Linked machines are stored in a `[machines]` section:

```toml
[fsm]
version = 1
type = "dfa"
name = "parent"

[states]
0x0000 = "idle"
0x0001 = "validating"
0x0002 = "ok"
0x0003 = "error"

[inputs]
0x0000 = "start"
0x0001 = "accept"
0x0002 = "reject"

[machines]
"validating" = "validator"
```

## Hex Format

Linked states use bit 2 (0x0004) in the state flags:

```
0002 0001:0004 0000:0000  # State 1, flags=linked
```

State flag bits:
- Bit 0 (0x0001): Initial state
- Bit 1 (0x0002): Accepting state
- Bit 2 (0x0004): Linked state

## Validation

Validate linked state references in a bundle:

```bash
fsm validate system.fsm --bundle
```

This checks:
- All linked machines exist in the bundle
- Linked machines are DFAs (required for deterministic delegation)
- No circular links (A→B→A)
- No self-links (A→A)

Example output for invalid bundle:
```
Errors:
  ✗ main: state "process" links to non-existent machine "processor"
  ✗ parser: state "validate" links to main which is NFA (must be DFA)

system.fsm: bundle validation failed
```

## Working with Bundles

### Selecting a Machine

Most commands accept `-m` or `--machine` to select a specific machine:

```bash
fsm info system.fsm -m validator
fsm png system.fsm -m main -o main.png
fsm analyse system.fsm -m parser
```

### Batch Operations

Render all machines at once:

```bash
fsm png system.fsm --all
# Creates: main.png, validator.png, parser.png

fsm svg system.fsm --all
# Creates: main.svg, validator.svg, parser.svg
```

### Editing Bundles

Open a bundle in fsmedit:

```bash
fsmedit system.fsm
```

The editor will prompt you to select which machine to edit. Use "Switch Machine" from the File menu to change machines within the bundle.

## Design Patterns

### Validation Pattern

Use linked states to validate input before processing:

```
idle ──start──▶ validating ──accept──▶ processing ──done──▶ complete
                    │
                  reject
                    │
                    ▼
                 error
```

### Protocol Layers

Model protocol layers as separate machines:

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  transport  │────▶│   session   │────▶│ application │
└─────────────┘     └─────────────┘     └─────────────┘
```

### Reusable Components

Create reusable validation or parsing machines:

```bash
# Create reusable email validator
fsm convert email_validator.json -o email_validator.fsm

# Use in multiple bundles
fsm bundle user_registration.fsm email_validator.fsm -o registration.fsm
fsm bundle contact_form.fsm email_validator.fsm -o contact.fsm
```

## Limitations

- Linked machines must be DFAs (deterministic)
- Circular links are not allowed
- Maximum delegation depth is implementation-defined (typically 16)
- Child machines cannot access parent state

## See Also

- [fsm CLI Manual](cmd/fsm/MANUAL.md) — CLI reference (bundles, validate, run)
- [fsmedit Manual](cmd/fsmedit/MANUAL.md) — Visual editor reference (bundle mode, machine manager, linked state navigation)
- [SPECIFICATION.md](SPECIFICATION.md) — File format specification
- [README.md](README.md) — Quick start guide
