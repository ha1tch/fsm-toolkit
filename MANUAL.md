# FSM Toolkit — Documentation

## Manuals

| Document | Description |
|----------|-------------|
| [Workflows](WORKFLOWS.md) | How the tools fit together in practice — from simple design to CI automation |
| [fsm CLI Manual](cmd/fsm/MANUAL.md) | Command-line tool: convert, render, analyse, validate, run, generate code, manage bundles |
| [fsmedit Manual](cmd/fsmedit/MANUAL.md) | Visual editor: canvas editing, bundle management, class system, component drawer |

## Specifications

| Document | Description |
|----------|-------------|
| [SPECIFICATION.md](SPECIFICATION.md) | Formal semantic guarantees and hex record format |
| [COMPATIBILITY.md](COMPATIBILITY.md) | Version stability promises |
| [MACHINES.md](MACHINES.md) | Linked states, delegation protocol, bundle structure |

## Quick Start

```bash
# Build
go build -o fsm ./cmd/fsm/
go build -o fsmedit ./cmd/fsmedit/

# Convert and render
fsm convert examples/beatles.json -o beatles.fsm
fsm png beatles.fsm --native

# Run interactively
fsm run examples/traffic_light.fsm

# Visual editor
fsmedit beatles.fsm
```

## Concepts

The toolkit works with four types of finite state machine:

- **DFA** — Deterministic Finite Automaton. One transition per (state, input) pair. Accept/reject output.
- **NFA** — Non-deterministic. Multiple transitions and epsilon moves allowed.
- **Moore** — Output determined by current state.
- **Mealy** — Output determined by (state, input) pair.

Three file formats are supported:

- **JSON** (`.json`) — Human-readable interchange format.
- **FSM** (`.fsm`) — ZIP archive with hex records, labels, and layout. Primary distribution format.
- **Hex** (`.hex`) — Compact text encoding. 20 hex characters per record.

## Correctness Model

**Validation** checks structural correctness: if `fsm validate` passes, `fsm run` will not crash. **Analysis** checks design quality: unreachable states, dead ends, non-determinism, unused symbols. An FSM is **clean** when both produce no issues.

For the full correctness model, enforcement levels, and type-specific rules, see the [fsm CLI manual](cmd/fsm/MANUAL.md#correctness-model).

## File Format

A `.fsm` file is a ZIP containing:

```
example.fsm
├── machine.hex      # required: hex records
├── labels.toml      # optional: human-readable names
└── layout.toml      # optional: visual editor positions
```

Each hex record: `TYPE SSSS:IIII TTTT:OOOO` (20 hex chars, four 16-bit fields).

| Record Type | Purpose |
|-------------|---------|
| 0000 | DFA/NFA transition |
| 0001 | Mealy transition (with output) |
| 0002 | State declaration (flags + Moore output) |
| 0003 | NFA multi-target continuation |

For the full format specification, see [SPECIFICATION.md](SPECIFICATION.md).

## Go Packages

The toolkit's functionality is available as Go libraries for integration into other projects.

**pkg/fsm** — Core FSM types, validation, analysis, Runner, and BundleRunner. Create FSMs programmatically, validate structure, analyse quality, and execute interactively.

**pkg/fsmfile** — File format handling: JSON, hex, and FSM reading/writing. Native SVG and PNG renderers. Graphviz DOT generation. Sugiyama layout engine. Bundle management.

**pkg/codegen** — Code generation for C, Rust, and Go/TinyGo. Standalone switch-based implementations with no runtime dependencies.

## License

Apache 2.0 — https://www.apache.org/licenses/LICENSE-2.0

Copyright (c) 2026 haitch — h@ual.fi
