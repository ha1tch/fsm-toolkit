# FSM Toolkit

A toolkit for finite state machines: create, convert, render, analyse, validate, execute, and generate code. Includes a terminal-based visual editor with canvas editing, bundle composition, a class/property system, component libraries, and structural netlist export to KiCad.

Supports DFA, NFA, Moore, and Mealy machines. Up to 65K states, 65K inputs, 65K outputs. Zero external dependencies at runtime (Graphviz optional for rendering).

## Quick Start

```bash
# Build
go build -o fsm ./cmd/fsm/
go build -o fsmedit ./cmd/fsmedit/

# Convert, render, run
fsm convert examples/traffic_light.fsm -o traffic.json --pretty
fsm png traffic_light.fsm --native
fsm run traffic_light.fsm

# Generate code
fsm generate traffic_light.fsm --lang c -o traffic.h
fsm generate traffic_light.fsm --lang rust -o traffic.rs
fsm generate traffic_light.fsm --lang go -o traffic.go

# Visual editor
fsmedit traffic_light.fsm

# Structural circuits
fsm netlist examples/circuits/counter_7seg.json
fsm netlist examples/circuits/counter_7seg.json --format kicad -o counter.net
```

Pre-built binaries for Linux, macOS, Windows, FreeBSD, and other platforms are available on the [releases page](https://github.com/ha1tch/fsm-toolkit/releases).

## What It Does

**fsm** is a command-line tool with 16 commands: convert between JSON/hex/FSM formats, render to PNG/SVG (via Graphviz or built-in native renderers), generate standalone code in C/Rust/Go, export structural netlists to KiCad/text/JSON, validate structure, analyse design quality, run interactively with full execution trace, manage bundles of linked machines, and query state class assignments and property values. See the [CLI manual](cmd/fsm/MANUAL.md) for the full reference.

**fsmedit** is a terminal-based visual editor with keyboard and mouse support, canvas panning with minimap, undo/redo, a class system with seven property types, a component drawer for rapid instantiation from class libraries, net rendering and a connection detail window for pin-to-pin wiring, and hierarchical bundle composition with linked-state navigation. See the [editor manual](cmd/fsmedit/MANUAL.md) for the full reference.

```
$ fsm run traffic_light.fsm
FSM: Traffic Light (moore)
State: green -> go
> timer
Output: caution
State: yellow -> caution
> timer
Output: stop
State: red -> stop
```

## File Format

A `.fsm` file is a ZIP archive containing hex-encoded machine data, optional human-readable labels, and optional editor layout. The hex format uses 20-character records (`TYPE SSSS:IIII TTTT:OOOO`) with four 16-bit fields. JSON is supported as an interchange format. Bundles pack multiple machines into a single `.fsm` file with linked-state delegation between them.

See the [Specification](docs/specification.md) for the full format definition and [Machines](docs/machines.md) for the bundle and linked-state protocol.

## Documentation

Full documentation is in the [docs/](docs/index.md) directory.

| Document | Contents |
|----------|----------|
| [Documentation Index](docs/index.md) | Overview, concepts, file format, Go packages |
| [Design Philosophy](docs/design-philosophy.md) | What the toolkit is, what it optimises for, how to think about it |
| [Workflows](docs/workflows.md) | How the tools fit together: design, validate, test, render, generate, bundle, automate |
| [Circuits](docs/circuits.md) | Structural connectivity: classes, ports, nets, KiCad export, the dual model |
| [CLI Manual](cmd/fsm/MANUAL.md) | All 16 commands, options, formats, correctness model, code generation, bundles |
| [Editor Manual](cmd/fsmedit/MANUAL.md) | Canvas editing, modes, bundles, class system, component drawer, key/mouse reference |
| [Specification](docs/specification.md) | Hex record format, validation semantics, formal guarantees |
| [Machines](docs/machines.md) | Linked states, delegation protocol, bundle structure |
| [Compatibility](docs/compatibility.md) | Version stability promises, forward/backward compatibility rules |
| [Changelog](CHANGELOG.md) | Release history |

## Platform Support

**fsm** (CLI) works on Linux, macOS, Windows, FreeBSD, OpenBSD, and NetBSD. Pre-built binaries are available for all major architectures including ARM (Raspberry Pi).

**fsmedit** (editor) works in any modern terminal emulator on Unix-like systems. On Windows, use WSL2 with Windows Terminal — native CMD.EXE and PowerShell are not supported.

## Go Packages

The toolkit's core is available as importable Go libraries: `pkg/fsm` (types, validation, analysis, Runner, BundleRunner), `pkg/fsmfile` (format I/O, native renderers, Sugiyama layout), `pkg/codegen` (C/Rust/Go code generation), and `pkg/export` (netlist export to KiCad, text, and JSON). See the [documentation index](docs/index.md) for API details.

## License

Apache 2.0 — https://www.apache.org/licenses/LICENSE-2.0

Copyright (c) 2026 haitch — h@ual.fi — https://oldbytes.space/@haitchfive
