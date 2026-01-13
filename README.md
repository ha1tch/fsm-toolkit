# FSM Toolkit

A compact binary format for finite state machines with converters, visualisation, and interactive execution.

Supports DFA, NFA, Moore, and Mealy machines. Up to 65K states, 65K inputs, 65K outputs.

**See [MANUAL.md](MANUAL.md) for complete documentation.**

## Quick Start

```bash
# Build Go CLI
go build -o fsm ./cmd/fsm

# Convert JSON to .fsm
./fsm convert examples/test_moore.json -o traffic.fsm

# Visualise
./fsm dot traffic.fsm | dot -Tpng -o traffic.png

# Run interactively
./fsm run traffic.fsm
```

## File Format

A `.fsm` file is a ZIP containing:

```
example.fsm
├── machine.hex      # required: hex records
└── labels.toml      # optional: human-readable names
```

Each hex record: `TYPE SSSS:IIII TTTT:OOOO` (20 hex chars)

## CLI Commands

| Command | Description |
|---------|-------------|
| `fsm convert` | Convert between json/hex/fsm |
| `fsm dot` | Generate Graphviz DOT |
| `fsm info` | Show FSM information |
| `fsm validate` | Validate FSM |
| `fsm run` | Interactive execution |

## Python Scripts

| Script | Description |
|--------|-------------|
| `fsm_converter.py` | Format conversion |
| `fsm_visualise.py` | DOT generation |
| `elevator_fsm.py` | Sample generator |

## Structure

```
fsm-toolkit/
├── cmd/fsm/         # Go CLI tool
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
Apache 2.0

# Author
Copyright (c)2026 haitch

h@ual.fi

https://oldbytes.space/@haitchfive


