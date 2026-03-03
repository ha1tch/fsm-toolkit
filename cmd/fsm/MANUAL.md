# fsm — Command-Line Reference

**fsm-toolkit CLI** for converting, rendering, analysing, validating, running, and generating code from finite state machines. Also supports structural netlist export, bundle management, and class property inspection.

**See also:** [Workflows guide](../../docs/workflows.md) for how the tools fit together, [fsmedit manual](../fsmedit/MANUAL.md) for the visual editor, [Specification](../../docs/specification.md) for formal semantics, [Compatibility](../../docs/compatibility.md) for version stability promises.

---

## Synopsis

```
fsm <command> [options]
fsm --version
fsm --help
```

## Installation

Copy the `fsm` binary to a directory on your PATH. No other files are required for the CLI itself. Optional dependencies:

- **Graphviz** (`dot` command) — required for `fsm png`, `fsm svg` (without `--native`), and `fsm view`. Not required if you always use `--native`. Install from https://graphviz.org/download/ or via your package manager (`brew install graphviz`, `apt install graphviz`, `choco install graphviz`).

- **fsmedit** — invoked by `fsm edit`. Searched in PATH, the current directory, and the directory containing the `fsm` binary.

## Supported Formats

The toolkit works with three file formats for FSM data, plus Graphviz DOT for rendering.

**JSON** (`.json`) is the human-readable interchange format. It stores the full FSM definition including state names, alphabets, transitions, and metadata. JSON files are typically the starting point for new FSMs and the easiest format to edit by hand.

**Hex** (`.hex`) is a compact text encoding where each record is 20 hexadecimal characters: `TYPE SSSS:IIII TTTT:OOOO`. Hex files contain only the numeric machine data with no labels or layout information. They are useful for low-level inspection and for environments where minimal file size matters.

**FSM** (`.fsm`) is a ZIP archive containing `machine.hex` (the binary data), optionally `labels.toml` (human-readable names for states, inputs, and outputs), optionally `layout.toml` (visual editor positions), and optionally `classes.json` (class definitions and per-state property values). This is the primary distribution format — it preserves all information including labels, editor layout, and class metadata, while remaining compact. FSM files can also be **bundles** containing multiple machines in a hierarchical composition.

**DOT** is the Graphviz graph description language, used as an intermediate format for rendering. The `fsm dot` command generates DOT output that can be piped to Graphviz tools or saved for manual editing.

## FSM Types

The toolkit supports four types of finite state machine:

**DFA** (Deterministic Finite Automaton) has exactly one transition per (state, input) pair. Produces accept/reject decisions. The strictest type — validation will reject epsilon transitions and warn about non-determinism or missing transitions.

**NFA** (Non-deterministic Finite Automaton) allows multiple transitions per (state, input) pair and epsilon (spontaneous) transitions. Also produces accept/reject decisions. NFAs are automatically converted to DFAs for code generation.

**Moore** machines associate an output with each state. The output depends only on the current state, not on the input that caused the transition. Useful for modelling systems where behaviour is tied to a condition (traffic lights, protocol states).

**Mealy** machines associate an output with each transition. The output depends on both the current state and the input symbol. Useful for modelling systems where behaviour depends on the triggering event (vending machines, parsers).

## Commands

### convert

Convert between JSON, hex, and FSM formats. Supports batch conversion with wildcards.

```
fsm convert <input>... [-o output] [--pretty] [--no-labels]
```

The output format is determined by the file extension of the `-o` argument. When no output is specified, the input extension is swapped: `.json` becomes `.fsm`, `.fsm` and `.hex` become `.json`. When `-o` starts with a dot (e.g., `-o .fsm`), it is treated as a target extension applied to each input file's basename, enabling batch conversion.

| Option | Description |
|--------|-------------|
| `-o, --output` | Output file or target extension |
| `--pretty` | Pretty-print JSON output with indentation |
| `--no-labels` | Omit `labels.toml` from FSM output (smaller file, numeric IDs only) |

Examples:

```bash
# Single file conversion
fsm convert input.json -o output.fsm
fsm convert input.fsm -o output.json --pretty

# Strip labels for minimal file size
fsm convert input.json --no-labels -o output.fsm

# Convert to raw hex
fsm convert input.json -o output.hex

# Batch: convert all JSON files to FSM
fsm convert *.json -o .fsm

# Batch: convert all FSM files to pretty JSON
fsm convert examples/*.fsm -o .json --pretty
```

### dot

Generate Graphviz DOT output. The result can be piped to Graphviz tools or saved for manual editing.

```
fsm dot <input> [-o output] [-t title] [-m machine]
```

| Option | Description |
|--------|-------------|
| `-o, --output` | Output file (default: stdout) |
| `-t, --title` | Graph title (default: FSM name or type summary) |
| `-m, --machine` | Select a specific machine from a bundle |

Examples:

```bash
# Pipe to Graphviz for PNG
fsm dot input.fsm | dot -Tpng -o output.png

# Save DOT for manual editing
fsm dot input.fsm -o output.dot

# Custom title
fsm dot input.fsm -t "My Protocol" | dot -Tsvg -o protocol.svg

# Specific machine from a bundle
fsm dot bundle.fsm -m child | dot -Tpng -o child.png
```

### png

Generate a PNG image directly. This is a convenience command equivalent to `fsm dot | dot -Tpng` but with additional support for the native renderer.

```
fsm png <input> [-o output] [-t title] [-m machine] [--all] [--native] [native options]
```

| Option | Description |
|--------|-------------|
| `-o, --output` | Output file (default: input basename + `.png`) |
| `-t, --title` | Diagram title |
| `-m, --machine` | Select machine from bundle |
| `--all` | Render all machines in a bundle to separate files |
| `--native` | Use the built-in renderer instead of Graphviz |
| `--font-size N` | Base font size in pixels (native only, default: 14) |
| `--spacing N` | Node spacing multiplier (native only, default: 1.5) |
| `--width N` | Canvas width in pixels (native only, default: 800) |
| `--height N` | Canvas height in pixels (native only, default: 600) |

Without `--native`, requires Graphviz. With `--native`, the built-in Sugiyama layout engine is used — no external dependencies. The native renderer handles state colouring (green for initial, orange for accepting, blue for both), double outlines for accepting states, self-loops, curved edges, and Mealy/Moore output labels.

When `--all` is used with a bundle, each machine is rendered to a separate file. If `-o` contains `%s`, it is replaced with the machine name; otherwise the machine name is appended to the output basename.

Examples:

```bash
# Graphviz rendering
fsm png beatles.fsm
fsm png beatles.fsm -o diagram.png -t "Fab Four"

# Native rendering (no Graphviz needed)
fsm png beatles.fsm --native
fsm png beatles.fsm --native --font-size 18 --spacing 2.0

# All machines in a bundle
fsm png bundle.fsm --all --native
```

### svg

Generate an SVG image. Identical options to `png`, with one additional native-only option.

```
fsm svg <input> [-o output] [-t title] [-m machine] [--all] [--native] [native options]
```

All options from `png` apply, plus:

| Option | Description |
|--------|-------------|
| `--shape SHAPE` | State node shape (native only): `circle`, `ellipse`, `rect`, `roundrect`, `diamond` |

The native SVG renderer produces clean, scalable output suitable for web embedding, documentation, and print. It uses the same Sugiyama layout algorithm as the native PNG renderer.

Examples:

```bash
# Graphviz rendering
fsm svg beatles.fsm -o diagram.svg

# Native rendering with custom shape
fsm svg beatles.fsm --native --shape roundrect

# Native with full customisation
fsm svg beatles.fsm --native --width 1200 --height 800 --font-size 16 --shape diamond
```

### info

Display information about an FSM: type, name, state count, alphabet, transitions, initial state, accepting states, linked state mappings, and class assignments. For detailed property values, use `fsm properties`.

```
fsm info <input> [-m machine]
```

| Option | Description |
|--------|-------------|
| `-m, --machine` | Select machine from bundle |

Example output:

```
Type:        moore
Name:        Traffic Light
States:      3
Inputs:      1
Outputs:     3
Transitions: 3
Initial:     green
Accepting:   []

Linked States:
  validate → validator

States:      [green yellow red]
Alphabet:    [timer]
Outputs:     [go caution stop]
```

### machines

List all machines contained in a bundle file. Shows name, type, state count, transition count, and description for each machine.

```
fsm machines <bundle.fsm>
```

Example output:

```
system.fsm contains 3 machines:

  NAME                 TYPE     STATES  TRANS  DESCRIPTION
  ----                 ----     ------  -----  -----------
  main                 dfa           5      8  Main controller
  parser               dfa          12     20  Input parser
  validator            dfa           4      6  Data validator
```

### validate

Check an FSM for structural errors. If validation passes, the FSM is guaranteed to be executable by `fsm run` without runtime crashes.

```
fsm validate <input> [-m machine] [--bundle]
```

| Option | Description |
|--------|-------------|
| `-m, --machine` | Select machine from bundle |
| `--bundle` | Validate linked state references across the entire bundle |

Validation checks: all referenced states exist, all referenced inputs are in the alphabet, the initial state is defined and present, accepting states exist, type-specific constraints are met (no epsilon transitions in DFA, outputs in output alphabet if defined).

Bundle validation (`--bundle`) additionally checks: all linked target machines exist, linked targets are DFAs (required for delegation), no circular links (A links to B links to A), and no self-links.

Exit code 0 means valid; exit code 1 means validation failed.

Examples:

```bash
# Single machine
fsm validate traffic_light.fsm

# Specific machine in bundle
fsm validate system.fsm -m parser

# Full bundle link validation
fsm validate system.fsm --bundle
```

### analyse

Analyse an FSM for design quality issues. These are warnings, not errors — the FSM can still run, but may have structural problems worth addressing. Also accepts the American spelling `analyze`.

```
fsm analyse <input> [-m machine] [--all]
fsm analyze <input> [-m machine] [--all]
```

| Option | Description |
|--------|-------------|
| `-m, --machine` | Select machine from bundle |
| `--all` | Analyse all machines plus cross-machine issues |

Per-machine checks:

| Warning | Meaning |
|---------|---------|
| `unreachable` | States not reachable from the initial state |
| `dead` | Non-accepting states with no outgoing transitions |
| `nondeterministic` | DFA with multiple transitions on the same (state, input) pair |
| `incomplete` | DFA states missing transitions for some input symbols |
| `unused_input` | Input symbols defined in the alphabet but never used |
| `unused_output` | Output symbols defined but never referenced |

Cross-machine checks (with `--all`):

| Warning | Meaning |
|---------|---------|
| `ORPHAN` | Machine not linked from any other machine in the bundle |
| `MISSING_TARGET` | A linked state references a machine that does not exist |
| `MISSING_ACCEPT` | Machine has linked states but no `accept` input defined |
| `MISSING_REJECT` | Machine has linked states but no `reject` input defined |

Examples:

```bash
fsm analyse traffic_light.fsm
fsm analyse system.fsm --all
```

### generate

Generate executable source code from an FSM definition. The generated code is standalone with no runtime dependencies.

```
fsm generate <input> --lang <c|rust|go|tinygo> [-o output] [--package name] [-m machine] [--all]
```

| Option | Description |
|--------|-------------|
| `--lang, -l` | Target language (required) |
| `-o, --output` | Output file (default: stdout) |
| `--package, -p` | Go package name (default: `fsm`) |
| `-m, --machine` | Select machine from bundle |
| `--all` | Generate a separate file for each machine in the bundle |

Supported languages:

**C** generates a header-only library (`.h`). Define `MYFSM_IMPLEMENTATION` in exactly one `.c` file before including the header. Uses `uint16_t` types, `#define` constants, switch-based dispatch. C89 compatible except for `bool`. No heap allocation.

**Rust** generates an idiomatic module (`.rs`) with `#[repr(u16)]` enums, `#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]`, `Display` implementations, and pattern-matching dispatch.

**Go** generates a standard package (`.go`) using `uint16` types, `String()` methods, and switch-based dispatch. Compatible with TinyGo for WASM and embedded targets. No reflection, no `interface{}`, no heap allocation in `Step()`.

**TinyGo** is an alias for Go.

All languages generate an equivalent API: `init`/`new`, `reset`, `step`, `can_step`, `state`, `output`, `is_accepting`, plus name-to-string conversions.

NFAs are automatically converted to DFAs (powerset construction) before code generation. For very large NFAs, the resulting DFA may have many composite states.

With `--all`, each machine in a bundle produces a separate output file named `<machine>.<ext>`.

Examples:

```bash
fsm generate machine.fsm --lang c -o machine.h
fsm generate machine.fsm --lang rust -o machine.rs
fsm generate machine.fsm --lang go --package myfsm -o myfsm.go
fsm generate bundle.fsm --all --lang go --package fsms
fsm generate bundle.fsm -m child --lang c -o child.h
```

### run

Run an FSM interactively in the terminal. Type input symbols to advance the machine, and use built-in commands to inspect state.

```
fsm run <input> [-m machine]
```

| Option | Description |
|--------|-------------|
| `-m, --machine` | Select the main machine from a bundle |

Interactive commands:

| Command | Action |
|---------|--------|
| *any text* | Send as input symbol to the FSM |
| `reset` | Return to the initial state |
| `status` | Show current state, accepting status, and output |
| `history` | Show the full execution trace |
| `inputs` | List input symbols available from the current state |
| `help` | Show command help |
| `quit` | Exit (also: `exit`, `q`) |

For Moore machines, the current output is displayed after each state. For Mealy machines, the transition output is displayed after each step. The status line shows `[accepting]` when the current state is an accepting state.

**Bundle execution.** When the input file is a bundle, `fsm run` creates a BundleRunner that supports linked state delegation. When execution reaches a linked state, control automatically transfers to the child machine's initial state. The prompt changes to show the active machine (`>>` prefix for delegated machines). The child runs until it reaches an accepting state (returns `accept` to the parent) or a dead end (returns `reject`). Additional bundle commands:

| Command | Action |
|---------|--------|
| `machines` | Show which machine is active and the full machine list |

If no `-m` is specified for a bundle, the first machine is used as the entry point.

Example session:

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
> quit
```

### view

Generate a PNG image and open it with the system's default image viewer. This is a convenience command for quick visual inspection.

```
fsm view <input> [-t title]
```

| Option | Description |
|--------|-------------|
| `-t, --title` | Diagram title |

Requires Graphviz. The viewer is selected by platform: `open` on macOS, `xdg-open` on Linux, `explorer.exe` on Windows.

### edit

Open the visual FSM editor. This is a convenience wrapper that locates `fsmedit` and passes all arguments through to it.

```
fsm edit [file] [options]
```

The `fsmedit` binary is searched in three locations, in order: the PATH environment variable, the current working directory, and the same directory as the `fsm` binary itself.

See the [fsmedit manual](../fsmedit/MANUAL.md) for full editor documentation.

### bundle

Combine multiple FSM files into a single bundle. Each input file becomes a named machine in the bundle, preserving its layout and labels.

```
fsm bundle <input1> <input2> ... -o <output.fsm>
```

The `-o` flag is required. At least one input file is required.

Example:

```bash
fsm bundle main.fsm parser.fsm validator.fsm -o system.fsm
```

### extract

Extract a single machine from a bundle into a standalone FSM file, preserving its layout and labels.

```
fsm extract <bundle.fsm> --machine <name> [-o output.fsm]
```

| Option | Description |
|--------|-------------|
| `--machine, -m` | Name of the machine to extract (required) |
| `-o` | Output file (default: `<name>.fsm`) |

Example:

```bash
fsm extract system.fsm --machine parser -o parser.fsm
```

### netlist

Export a structural netlist from an FSM or circuit definition. Useful for EDA tool integration and PCB design workflows.

```
fsm netlist <input> [--format <fmt>] [-o output] [-m machine] [--bake]
```

| Option | Description |
|--------|-------------|
| `--format` | Output format: `text` (default), `kicad`, `json` |
| `-o, --output` | Output file (default: stdout) |
| `-m, --machine` | Select machine from bundle |
| `--bake` | Write derived KiCad fields (part, footprint) back into the source file |

The **text** format produces a human-readable listing of components and connections. The **KiCad** format produces a KiCad S-expression netlist (`.net`) suitable for import into KiCad PCB. The **json** format produces a structured JSON representation of the netlist for programmatic processing.

KiCad part and footprint strings are derived automatically for 74xx-series components from the class name and pin count. Use `--bake` to write these derived values back into the source file so they can be overridden manually.

Examples:

```bash
# Human-readable netlist
fsm netlist circuit.json

# KiCad netlist export
fsm netlist circuit.json --format kicad -o circuit.net

# JSON for scripting
fsm netlist circuit.json --format json -o netlist.json

# Write derived KiCad fields back to source
fsm netlist circuit.json --bake
```

### properties

Query state class assignments and property values. Useful for inspecting metadata attached to states — particularly agent permissions, timer conditions, plane assignments, and annotations added by external tooling.

```
fsm properties <input> [--state <n>] [--class <n>] [--machine <n>] [--all] [--format <fmt>]
```

| Option | Description |
|--------|-------------|
| `--state, -s` | Show only the given state |
| `--class, -c` | Show only states assigned to this class |
| `--machine, -m` | Select machine from bundle (default: first) |
| `--all, -a` | Iterate all machines in a bundle |
| `--format, -f` | Output format: `text` (default), `json`, `csv`, `asciitable`, `htmltable` |

`--state` and `--class` are independent and composable. Either, both, or neither may be specified.

Output columns: **Machine**, **State**, **Class**, **Property**, **Value**. States with a class assignment but no populated property values appear as a single row with property `(none)`.

The `text` format groups output hierarchically by machine and state. The `json` format produces a nested object keyed by machine name, then state name, containing `class` and `properties` fields. The `csv`, `asciitable`, and `htmltable` formats produce flat tabular output suitable for spreadsheets, terminal display, and web embedding respectively.

Examples:

```bash
# All properties for all states
fsm properties heladera.fsm

# Single state
fsm properties heladera.fsm --state temperatura_elevada

# All states of a given class
fsm properties heladera.fsm --class shelf_time_driven

# All machines in a bundle, terminal states only, as CSV
fsm properties paracaidas.fsm --all --class shelf_terminal --format csv

# All machines in a bundle as JSON
fsm properties bundle.fsm --all --format json

# ASCII table for terminal display
fsm properties bundle.fsm --all --format asciitable

# HTML table for reporting
fsm properties bundle.fsm --format htmltable > report.html
```

## Bundles and Linked States

A bundle is an FSM file containing multiple machines. Machines within a bundle can reference each other through **linked states**: a state in one machine can delegate to another machine. When execution reaches a linked state, the linked machine runs from its initial state. When the linked machine reaches an accepting state, control returns to the parent.

Linked states enable hierarchical FSM composition — a complex system can be decomposed into independent machines that communicate through delegation. The parent machine needs `accept` and `reject` inputs in its alphabet to handle child machine results.

Use `fsm machines` to list machines in a bundle, `fsm bundle` to create bundles, `fsm extract` to pull machines out, and `fsm validate --bundle` to check link integrity. The `fsm run` command fully supports linked state delegation when given a bundle file.

For a detailed treatment of linked states and the delegation protocol, see the [Machines guide](../../docs/machines.md).

## Correctness Model

**Validation** checks structural correctness: can the FSM execute without crashing? If `fsm validate` passes, `fsm run` is guaranteed not to crash. Validation failures are errors — the FSM cannot run.

**Analysis** checks design quality: does the FSM have issues worth fixing? Analysis findings are warnings — the FSM can still run but may behave unexpectedly (unreachable states, dead ends, non-determinism in DFAs).

An FSM is considered **clean** when both validation and analysis produce no issues.

| Constraint | Enforced? | Consequence |
|------------|-----------|-------------|
| Initial state exists | Always | Validation error |
| Transition targets exist | Always | Validation error |
| Inputs in alphabet | Always | Validation error |
| Outputs in output alphabet | If alphabet defined | Validation error |
| DFA has no epsilon | Always | Validation error |
| DFA is deterministic | Warning only | Runs as NFA |
| DFA is complete | Warning only | Rejects on missing |
| Moore has all outputs | Never | Missing outputs return `""` |
| All states reachable | Warning only | Dead code |

## File Format Details

The `.fsm` file is a ZIP archive. The hex record format uses 20-character records with four 16-bit fields and a record type. Record types 0000-0003 are currently defined: DFA/NFA transition (0000), Mealy transition (0001), state declaration (0002), and NFA multi-target (0003). Record types 0100-FFFF are reserved for extensions. Readers should ignore unknown record types for forward compatibility.

For the complete hex format specification, see the [Specification](../../docs/specification.md).

## Capacity Limits

| Property | Maximum |
|----------|---------|
| States | 65,536 |
| Input symbols | 65,535 |
| Output symbols | 65,536 |
| Transitions | Unlimited |
| Machines per bundle | Unlimited |
| Delegation depth | 16 (implementation-defined) |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Error (invalid input, missing file, validation failure, etc.) |

## License

Apache 2.0 — https://www.apache.org/licenses/LICENSE-2.0

Copyright (c) 2026 haitch — h@ual.fi
