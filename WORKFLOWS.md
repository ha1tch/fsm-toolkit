# Workflows

This guide explains how to use the FSM Toolkit in practice. The toolkit has a lot of surface area — a CLI with 14 commands, a visual editor with 20 modes, a class system, code generators, bundle composition — and it is not always obvious which parts to use for a given task, or in what order. This document walks through the common workflows from simple to complex, showing how the pieces fit together.

**Prerequisites.** You have `fsm` and `fsmedit` built or installed. Graphviz is optional but helpful. See the [README](README.md) for build instructions.

---

## 1. Design a simple FSM

The most common starting point is a single machine with a handful of states. There are two natural entry points: the visual editor for exploratory design, and JSON for precise specification.

### Visual-first approach

Launch the editor with no arguments to start from a blank canvas.

```bash
fsmedit
```

The editor opens in menu mode. Press Esc to enter the canvas. Move the cursor with arrow keys, press Enter to place a state, and type a name. Repeat for each state. Press I to define input symbols, then select a state with Tab and press T to add transitions. Press S on a state to mark it as initial, A to toggle accepting. Press Ctrl+S to save — the editor prompts for a filename.

This approach works well when you are exploring a design and don't yet know exactly what the states and transitions should be. The canvas gives you spatial reasoning about the machine's structure, and you can rearrange states by grabbing them (G) and moving with arrow keys.

Once saved, you can render the machine to see a cleaner diagram:

```bash
fsm png my_machine.fsm --native
```

### JSON-first approach

If you already know the structure, writing JSON directly is faster than clicking through the editor. Create a file like this:

```json
{
  "type": "moore",
  "states": ["idle", "heating", "ready"],
  "alphabet": ["brew", "done", "timeout"],
  "initial": "idle",
  "accepting": [],
  "transitions": [
    {"from": "idle", "input": "brew", "to": "heating"},
    {"from": "heating", "input": "done", "to": "ready"},
    {"from": "ready", "input": "timeout", "to": "idle"}
  ],
  "state_outputs": {
    "idle": "off",
    "heating": "heating",
    "ready": "serve"
  },
  "output_alphabet": ["off", "heating", "serve"]
}
```

Validate it immediately:

```bash
fsm validate coffee.json
```

If validation passes, render it to see the structure:

```bash
fsm png coffee.json --native
```

Or open it in the editor for visual refinement:

```bash
fsmedit coffee.json
```

The editor loads the JSON, auto-layouts the states, and you can drag them into a sensible arrangement. When you save as `.fsm`, the layout is preserved for next time.

### Which approach to use

The visual editor is better for exploration, spatial reasoning, and iterating on layout. JSON is better for precision, version control diffs, and scripting. Most projects use both: sketch in the editor, export to JSON for review, or write JSON and open it in the editor to check the diagram. The tools are designed to round-trip cleanly between formats.


## 2. Validate and analyse

Once an FSM exists, the next step is checking its quality. This is a two-stage process: validation catches errors that would prevent execution, and analysis catches design issues that may indicate bugs.

```bash
# Stage 1: can it run at all?
fsm validate my_machine.fsm

# Stage 2: does it have design issues?
fsm analyse my_machine.fsm
```

Validation is pass/fail. If it fails, the error message tells you exactly what is wrong: a missing initial state, a transition targeting a nonexistent state, an input symbol not in the alphabet, or a type constraint violation (such as epsilon transitions in a DFA).

Analysis produces warnings. An FSM can pass validation and still have unreachable states, dead-end states, non-determinism in a DFA, incomplete transitions, or unused symbols in the alphabet. These are not errors — the machine will run — but they often indicate oversights in the design.

The editor provides the same checks interactively: press V to validate and L to analyse, with results shown in the status bar.

A reasonable discipline is to validate after every structural change and analyse before considering a design complete.


## 3. Test interactively

Before generating code or committing to a design, run the FSM interactively to check its behaviour against your expectations.

```bash
fsm run coffee.json
```

The runner drops you into a REPL where you type input symbols and see the machine respond. Moore machines show the current output after each state; Mealy machines show the transition output. Use `inputs` to see what the machine accepts from the current state, `history` to see the execution trace, and `reset` to start over.

This is the fastest way to find logical errors — states that should be reachable but aren't, transitions that go to the wrong place, outputs that don't match expectations. If something is wrong, go back to the editor or JSON, fix it, and run again.

For bundles with linked machines, the runner supports delegation automatically. When execution reaches a linked state, control transfers to the child machine. The prompt changes to show which machine is active, and you can type `machines` to see the delegation stack. This is the only way to test linked-state behaviour end-to-end from the command line.


## 4. Render diagrams

The toolkit produces diagrams in PNG, SVG, and DOT formats. There are two rendering paths.

**Native rendering** uses the built-in Sugiyama layout engine and requires no external tools. It handles state colouring, accepting-state double outlines, self-loops, curved edges, and transition labels. Use the `--native` flag:

```bash
fsm png my_machine.fsm --native
fsm svg my_machine.fsm --native --shape roundrect
```

Native rendering is good enough for most purposes and is the right choice for CI pipelines and environments where you don't want to install Graphviz.

**Graphviz rendering** produces higher-quality layout for complex graphs. It requires the `dot` command to be installed:

```bash
fsm png my_machine.fsm
fsm svg my_machine.fsm
```

For a quick look without saving a file, `fsm view` generates a PNG and opens it directly in your system's image viewer.

The editor's Render command (R key, or from the menu) uses whichever renderer is configured in Settings. If you want to switch between Graphviz and native, change it there.


## 5. Generate code

Once a design is validated and tested, you can generate standalone source code for embedding in an application. The generated code has no runtime dependencies — it uses switch statements, integer types, and static data.

```bash
# C header (include in one .c file with #define COFFEE_IMPLEMENTATION)
fsm generate coffee.fsm --lang c -o coffee.h

# Rust module
fsm generate coffee.fsm --lang rust -o coffee.rs

# Go package
fsm generate coffee.fsm --lang go --package coffee -o coffee/coffee.go
```

The generated API is consistent across languages: init/new, step, can_step, state, output, is_accepting, reset, and name-to-string conversions.

A typical code generation workflow is:

1. Design and validate the FSM.
2. Test interactively with `fsm run`.
3. Generate code.
4. Integrate the generated code into your project.
5. When the design changes, regenerate — the generated file is a build artifact, not a hand-edited source.

For bundles, `--all` generates a separate file per machine:

```bash
fsm generate system.fsm --all --lang go --package fsms
```

This produces one `.go` file per machine. The generated code does not currently handle linked-state delegation — each machine is standalone. If you need cross-machine execution, use the `pkg/fsm` library's `BundleRunner` directly.


## 6. Convert between formats

Format conversion is a utility operation that supports several workflows.

**JSON to FSM** for distribution. JSON is readable but large; FSM files are compact and include layout. This is the standard packaging step before sharing a machine:

```bash
fsm convert my_machine.json -o my_machine.fsm
```

**FSM to JSON** for inspection or version control. JSON diffs are meaningful; FSM diffs are not (it's a ZIP file). For projects under version control, you may want to keep the JSON as the source of truth:

```bash
fsm convert my_machine.fsm -o my_machine.json --pretty
```

**Batch conversion** for processing a directory of files:

```bash
fsm convert examples/*.json -o .fsm
```

The `-o .fsm` syntax means "apply this extension to each input file's basename." This converts every JSON in the directory to FSM format in one command.

**Stripping labels** for minimal size. If you only need the machine data (no human-readable names), the `--no-labels` flag produces a smaller FSM file:

```bash
fsm convert my_machine.json --no-labels -o my_machine.fsm
```


## 7. Work with bundles

Bundles are the mechanism for composing multiple machines into a single file. They are essential when your system is too complex for a single flat FSM, or when you want to reuse a sub-machine across multiple contexts.

### Creating a bundle from the editor

The easiest way to create a bundle is from the editor. Start with a single FSM, then:

1. Press B (or select Machines from the menu) to open the machine manager.
2. Press A to add a new machine. The editor promotes your single FSM to a bundle and creates the new machine.
3. Switch between machines by pressing Enter on a machine in the manager, or by clicking machine names in the sidebar header.

To link machines together, select a state and press K. If other machines exist in the bundle, a selector appears. The linked state is displayed in fuchsia with a ↗ suffix, and you can dive into it with Space or Shift+Right.

### Creating a bundle from the CLI

If you have separate FSM files, combine them:

```bash
fsm bundle main.fsm parser.fsm validator.fsm -o system.fsm
```

Then open the bundle in the editor to establish links between machines:

```bash
fsmedit system.fsm
```

### Working within a bundle

Once in bundle mode, the editor maintains separate canvases, undo stacks, and layouts for each machine. Switching machines saves the current state to an in-memory cache, so nothing is lost. The machine manager (B key) provides add, rename, delete, and switch operations.

The sidebar header lists all machines. The current machine is marked with `>`. Click a machine name to switch, or use the machine manager for operations that affect the bundle structure.

The breadcrumb bar at the top of the screen shows your navigation path when you dive into linked states. Click any segment to jump back to that level.

### Managing bundles from the CLI

List machines in a bundle:

```bash
fsm machines system.fsm
```

Analyse all machines plus cross-machine issues (orphaned machines, missing link targets, missing accept/reject inputs):

```bash
fsm analyse system.fsm --all
```

Validate link integrity across the bundle:

```bash
fsm validate system.fsm --bundle
```

Extract a single machine for independent use:

```bash
fsm extract system.fsm --machine parser -o parser.fsm
```

Render all machines to separate images:

```bash
fsm png system.fsm --all --native
```


## 8. Use the class system

The class system attaches typed, structured metadata to states. This is useful when states represent more than just abstract nodes — when they represent hardware components, protocol phases, organisational roles, or any domain where states carry properties beyond their name and transitions.

### Defining classes

Open the editor and go to Settings (Esc → Settings). Press C to open the class editor. Here you create classes and define their properties. A class is a schema: a named collection of typed property definitions. For example, a `logic_gate` class might have properties like `part_number` (string), `pin_count` (int), `propagation_delay_ns` (float), and `technology` (enum: TTL, CMOS, BiCMOS).

Each property has a type, a default value, and optional constraints (min/max for numbers, allowed values for enums). Seven types are available: string, int, float, bool, enum, list, and map.

### Assigning classes to states

Press X on the canvas to open the class assignment grid. This shows a matrix of states versus available classes. Toggle assignments with Enter. When a class is assigned, the state inherits all its properties with their default values. Press P on a state to edit its specific property values.

### Using class libraries

The toolkit ships with 74xx-series digital logic libraries in the `class-libraries/` directory. These provide pre-defined classes for logic gates, buffers, registers, arithmetic units, multiplexers, and more — each with appropriate properties (pin count, package type, propagation delay, logic family).

To load them: Settings → navigate to the class library path → Enter to browse → select the `class-libraries` directory → L to load. The loaded classes appear in the class editor and the component drawer.

### The component drawer

Once libraries are loaded, press C on the canvas (or click the [+] button) to open the component drawer. This is a visual catalogue of instantiable components. Browse by category with Tab, select a component with arrow keys, and press Enter to place it on the canvas. The new state is created with the class already assigned and all properties initialised to defaults.

For projects with many typed components (digital circuits, network topologies, organisational charts), the drawer provides a significant speed advantage over manually creating states and assigning classes.

### What the class system produces

Class data persists in JSON. Every state's class assignments and property values are saved and restored faithfully. The data is available to any tool that reads the JSON output: `fsm info` shows it, `fsm convert` preserves it, and external tools can parse the JSON to extract property data for their own purposes (BOM generation, simulation parameters, pin mapping, or whatever the domain requires).

The class system is a metadata authoring tool. The FSM Toolkit does not itself consume property values for rendering, code generation, or execution — the runner steps through transitions regardless of what properties a state carries. The value is in the structured, typed, validated data that the toolkit produces and external tools can consume.


## 9. Automate with scripts and CI

The CLI is designed for scripting. All commands accept file arguments, produce structured output, and use exit codes (0 for success, 1 for failure).

### Validation in CI

Add a validation step to your build pipeline:

```bash
fsm validate my_machine.fsm
fsm analyse my_machine.fsm --all
```

If either fails or reports warnings you want to treat as errors, the exit code can gate the build.

### Batch rendering

Generate diagrams for all machines in a project:

```bash
for f in designs/*.fsm; do
    fsm png "$f" --native -o "docs/$(basename "$f" .fsm).png"
done
```

Or for all machines in a bundle:

```bash
fsm png system.fsm --all --native
```

### Code generation as a build step

Treat generated code as a build artifact:

```bash
fsm generate protocol.fsm --lang go --package protocol -o internal/protocol/fsm.go
go build ./...
```

The generated file is reproducible from the FSM source. Don't edit it by hand — regenerate it when the design changes.

### Format conversion for version control

Keep JSON as the version-controlled source, and generate FSM files for distribution:

```bash
# In your build script
fsm convert src/machines/*.json -o .fsm
```

JSON diffs are readable; FSM file diffs are not. If you version-control FSM files, consider also committing the JSON alongside for reviewability.


## Choosing the right starting point

If you are modelling a protocol, start with JSON. Protocols have well-defined states and transitions, and writing them out explicitly catches ambiguities early.

If you are exploring a design and don't know the full structure yet, start in the editor. The canvas lets you sketch, rearrange, and iterate quickly.

If you are building a library of typed components (hardware, network devices, organisational units), start by defining classes and loading or creating class libraries. The component drawer then becomes your primary creation tool.

If you are integrating an FSM into software, the typical pipeline is: design (editor or JSON) → validate → test (`fsm run`) → generate code → integrate. Iterate on the FSM design, not the generated code.

If you are building a hierarchical system with delegation between sub-machines, start with individual machines, validate each one, then bundle them and establish links. Test the complete bundle with `fsm run` before generating code or deploying.

---

**Reference:** [fsm CLI manual](cmd/fsm/MANUAL.md) — [fsmedit manual](cmd/fsmedit/MANUAL.md) — [README](README.md)
