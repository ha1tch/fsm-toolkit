# FSM Toolkit

A compact binary format for finite state machines with converters, visualisation, code generation, interactive runner, and visual editor.

Supports DFA, NFA, Moore, and Mealy machines. Up to 65K states, 65K inputs, 65K outputs.

**See [MANUAL.md](MANUAL.md) for complete documentation.**

## Quick Start

```bash
# Build (creates bin/fsm and bin/fsmedit)
./build.sh

# Or build manually
go build -o fsm ./cmd/fsm/
go build -o fsmedit ./cmd/fsmedit/

# Convert JSON to .fsm
./bin/fsm convert examples/beatles.json -o beatles.fsm

# Generate images directly
./bin/fsm png beatles.fsm
./bin/fsm svg beatles.fsm -o diagram.svg

# Or open in system viewer
./bin/fsm view beatles.fsm

# Generate code
./bin/fsm generate beatles.fsm --lang c -o beatles.h
./bin/fsm generate beatles.fsm --lang rust -o beatles.rs
./bin/fsm generate beatles.fsm --lang go -o beatles.go

# Analyse for issues
./bin/fsm analyse beatles.fsm

# Run interactively
./bin/fsm run beatles.fsm

# Visual editor
./bin/fsmedit beatles.fsm
```

## File Format

A `.fsm` file is a ZIP containing:

```
example.fsm
├── machine.hex      # required: hex records
├── labels.toml      # optional: human-readable names
└── layout.toml      # optional: visual positions (fsmedit)
```

Each hex record: `TYPE SSSS:IIII TTTT:OOOO` (20 hex chars)

## CLI Commands

| Command | Description |
|---------|-------------|
| `fsm convert` | Convert between json/hex/fsm |
| `fsm dot` | Generate Graphviz DOT |
| `fsm png` | Generate PNG image |
| `fsm svg` | Generate SVG image |
| `fsm generate` | Generate code (C, Rust, Go/TinyGo) |
| `fsm info` | Show FSM information |
| `fsm analyse` | Analyse for potential issues |
| `fsm validate` | Validate FSM structure |
| `fsm run` | Interactive execution |
| `fsm view` | Visualise (generates PNG, opens viewer) |
| `fsm edit` | Open visual editor (invokes fsmedit) |

## Python Scripts

| Script | Description |
|--------|-------------|
| `fsm_converter.py` | Format conversion |
| `fsm_visualise.py` | DOT generation |
| `elevator_fsm.py` | Sample generator |

## Structure

```
fsm-toolkit/
├── cmd/fsm/         # CLI tool
├── cmd/fsmedit/     # Visual editor
├── pkg/fsm/         # Core FSM types + runner
├── pkg/fsmfile/     # File format handling
├── *.py             # Python scripts
└── examples/        # Sample FSMs
```

## Example: Interactive Run

```
$ fsm run traffic_light.fsm
State: green -> go
> timer
Output: caution
State: yellow -> caution
> timer
Output: stop
State: red -> stop
> history
  1: green --timer--> yellow [caution]
  2: yellow --timer--> red [stop]
```

## License

Apache 2.0 — see https://www.apache.org/licenses/LICENSE-2.0

Copyright (c) 2026 haitch — h@ual.fi
