# FSM Toolkit

A toolkit for finite state machines: create, convert, render, analyse, validate, execute, and generate code. Includes a terminal-based visual editor with canvas editing, bundle composition, a class/property system, and component libraries.

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
```

Pre-built binaries for Linux, macOS, Windows, FreeBSD, and other platforms are available on the [releases page](https://github.com/ha1tch/fsm-toolkit/releases).

## What It Does

**fsm** is a command-line tool with 14 commands: convert between JSON/hex/FSM formats, render to PNG/SVG (via Graphviz or built-in native renderers), generate standalone code in C/Rust/Go, validate structure, analyse design quality, run interactively with full execution trace, and manage bundles of linked machines. See the [CLI manual](cmd/fsm/MANUAL.md) for the full reference.

**fsmedit** is a terminal-based visual editor with keyboard and mouse support, canvas panning with minimap, undo/redo, a class system with seven property types, a component drawer for rapid instantiation from class libraries, and hierarchical bundle composition with linked-state navigation. See the [editor manual](cmd/fsmedit/MANUAL.md) for the full reference.

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

See [SPECIFICATION.md](SPECIFICATION.md) for the full format definition and [MACHINES.md](MACHINES.md) for the bundle and linked-state protocol.

## Documentation

| Document | Contents |
|----------|----------|
| [Workflows](WORKFLOWS.md) | How the tools fit together: design, validate, test, render, generate, bundle, automate |
| [CLI Manual](cmd/fsm/MANUAL.md) | All 14 commands, options, formats, correctness model, code generation, bundles |
| [Editor Manual](cmd/fsmedit/MANUAL.md) | Canvas editing, modes, bundles, class system, component drawer, key/mouse reference |
| [SPECIFICATION.md](SPECIFICATION.md) | Hex record format, validation semantics, formal guarantees |
| [MACHINES.md](MACHINES.md) | Linked states, delegation protocol, bundle structure |
| [COMPATIBILITY.md](COMPATIBILITY.md) | Version stability promises, forward/backward compatibility rules |
| [CHANGELOG.md](CHANGELOG.md) | Release history |

## Platform Support

**fsm** (CLI) works on Linux, macOS, Windows, FreeBSD, OpenBSD, and NetBSD. Pre-built binaries are available for all major architectures including ARM (Raspberry Pi).

**fsmedit** (editor) works in any modern terminal emulator on Unix-like systems. On Windows, use WSL2 with Windows Terminal — native CMD.EXE and PowerShell are not supported.

## Go Packages

The toolkit's core is available as importable Go libraries: `pkg/fsm` (types, validation, analysis, Runner, BundleRunner), `pkg/fsmfile` (format I/O, native renderers, Sugiyama layout), and `pkg/codegen` (C/Rust/Go code generation). See the [documentation index](MANUAL.md) for API details.

## License

Apache 2.0 — https://www.apache.org/licenses/LICENSE-2.0

Copyright (c) 2026 haitch — h@ual.fi — https://oldbytes.space/@haitchfive
