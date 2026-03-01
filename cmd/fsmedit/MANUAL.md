# fsmedit — Visual Editor Reference

**fsmedit** is a terminal-based visual editor for creating and modifying finite state machines. It runs in any modern terminal emulator and provides a canvas for placing states, drawing transitions, managing bundles of linked machines, and assigning classes with typed properties.

**See also:** [Workflows guide](../../WORKFLOWS.md) for how the tools fit together, [fsm CLI manual](../fsm/MANUAL.md) for the command-line tool, [SPECIFICATION.md](../../SPECIFICATION.md) for formal semantics, [MACHINES.md](../../MACHINES.md) for linked states and bundles.

---

## Synopsis

```
fsmedit [file]
```

Launch the editor. If a file is given (`.fsm` or `.json`), it is opened immediately. Without a file, the editor starts with an empty DFA.

The editor can also be launched through the CLI wrapper: `fsm edit [file]`.

## Requirements

A terminal emulator with support for 256 colours and mouse events. Most modern terminals qualify: iTerm2 and Terminal.app on macOS, GNOME Terminal, Alacritty, and Kitty on Linux, and Windows Terminal under WSL2.

The editor is not tested on Windows CMD.EXE or PowerShell and is likely to have rendering issues. Windows users should run fsmedit under WSL2.

Optional: Graphviz (`dot` command) for the Render command. Without Graphviz, the built-in native renderer is used instead.


## Interface Layout

The screen is divided into three regions.

**Canvas** (left) is the working area where states are placed and transitions are drawn. The canvas is 512×512 logical units; the viewport shows a window into this space. States are displayed as labelled nodes; transitions as arcs between them when arc visibility is enabled. The cursor position is shown as a crosshair. A minimap appears during canvas drag mode (Ctrl+D) showing the full canvas with the viewport highlighted.

**Sidebar** (right) lists the current FSM's states, inputs, outputs, and transitions. In bundle mode, the sidebar header shows all machine names — click one to switch to it. The sidebar can be collapsed with `\` and resized by dragging the divider. A breadcrumb bar appears at the top when navigating into linked machines, showing the navigation path (e.g., `main > parser > validator`).

**Status bar** (bottom) shows the current file name, modification status, FSM type, mode indicator, and messages. The `[+]` button in the bottom-right corner opens the component drawer.


## Modes

The editor uses modal interaction. The current mode determines what keys do and what is drawn. The mode is always visible in the status bar.

**Menu** — the main menu for file operations, machine management, rendering, and settings. Reached by pressing Esc from the canvas, or on startup.

**Canvas** — the primary editing mode. Place states, select them, add transitions, assign classes, and navigate between machines. Most of the editor's functionality lives here.

**Input** — a text prompt for entering names (state names, machine names, file paths). Appears contextually when an operation needs text input. Enter confirms, Esc cancels.

**File Picker** — a file browser for Open and Save As. Navigate with arrow keys, Enter to select, Esc to cancel. Filters for `.fsm` and `.json` files.

**Settings** — an overlay for configuring the renderer, file type, FSM type, vocabulary, and class libraries. Reached from the menu or by pressing Esc from the canvas and selecting Settings.

**Help** — a scrollable overlay showing all keyboard shortcuts, grouped by function. Press H or ? on the canvas to open, Esc to close.

**Machine Manager** — an overlay for managing machines in a bundle: add, rename, delete, switch, and inspect link relationships. Reached from the menu (Machines) or by pressing B on the canvas.

**Class Editor** — a two-panel overlay for defining class schemas: create classes, add typed properties, set defaults. Reached from Settings (C key).

**Class Assignment** — a grid overlay for assigning classes to states and editing per-state property values. Reached from the canvas (X key).

**Property Editor** — a popup for editing a single state's property values. Reached from the canvas (P key) when a state with class properties is selected.

**Component Drawer** — a bottom panel showing instantiable components from loaded class libraries. Browse by category, preview properties, and place components on the canvas. Reached by pressing C on the canvas or clicking the `[+]` button.

**Canvas Drag** — a panning mode with minimap overlay. Reached with Ctrl+D or middle-mouse-drag. Arrow keys pan the viewport; Esc or Ctrl+D exits.

Several transitional modes exist for multi-step operations: adding transitions (select target state, then select input symbol), selecting link targets, and importing machines from bundles.


## Canvas Editing

### Creating States

Press **Enter** at the cursor position to create a new state. You will be prompted for a name. The state appears at the cursor location.

Right-click anywhere on the canvas to create a state at the mouse position.

From the component drawer, press Enter or drag a component card onto the canvas to create a state with a class already assigned and properties initialised.

### Selecting States

Click a state with the left mouse button, or press **Tab** to cycle selection through all states. The selected state is highlighted in the sidebar, and its properties (if any) are shown.

### Moving States

Press **G** to grab the selected state. Arrow keys move it; Enter confirms the new position; Esc cancels and restores the original position.

Alternatively, left-click and drag a state to reposition it. Drag works with both left and right mouse buttons for laptop touchpad accessibility.

### Editing States

With a state selected:

| Key | Action |
|-----|--------|
| S | Set as the initial state (replaces any existing initial) |
| A | Toggle accepting status |
| M | Set Moore output (Moore machines only; prompts for the output symbol) |
| K | Link to another machine (see Linked States below) |
| P | Edit property values (if the state has a class assigned) |
| X | Open the class assignment grid |
| Del | Delete the state and all its transitions |

Double-click a state to rename it. If the state is linked, double-click dives into the linked machine instead.

### Transitions

Press **T** with a state selected to add a transition. The editor enters target-selection mode — use Tab or click to choose the destination state, then press Enter. If the alphabet has input symbols, you are prompted to choose one. For Mealy machines, you also choose an output symbol.

Press **I** to add a new input symbol to the alphabet. Press **O** to add a new output symbol (Mealy/Moore).

### Display

Press **W** to toggle arc visibility — showing or hiding transition arcs on the canvas. Arcs are drawn as lines with arrow heads and labelled with their input (and output for Mealy) symbols.

Press **R** to render the FSM to an image and open it in the system viewer.

Press **\\** to collapse or expand the sidebar. Drag the divider to resize it.


## Viewport Navigation

The canvas is larger than the screen. Several methods are available for navigating:

**Arrow keys** move the cursor. When the cursor reaches the viewport edge, the viewport scrolls to follow.

**Shift + arrow keys** pan the viewport directly without moving the cursor. This is the quickest way to scroll.

**Ctrl+D** enters canvas drag mode. A minimap appears showing the full 512×512 canvas with the current viewport highlighted. Arrow keys pan the viewport. Press Esc or Ctrl+D to exit. Middle-mouse-drag also enters this mode.


## Bundle Mode

The editor supports working with multiple machines in a single file. A bundle contains two or more named machines. The sidebar header lists all machines; the current machine is highlighted.

### Entering Bundle Mode

There are several ways to enter bundle mode from a single-FSM session:

- **Machines > Add** (menu or B key on canvas, then A) — prompts for a name, creates an empty machine, and promotes to bundle mode. The original FSM becomes the first machine.
- **Import** (menu) — importing from another file promotes to bundle mode.
- **K on a state with no link targets** — offers to create a new machine and link to it.

### Switching Machines

Click a machine name in the sidebar header, or use the Machine Manager (B / Machines menu) and press Enter on a machine. The current machine's state is saved to an in-memory cache before switching, and restored when you switch back.

### Machine Manager

Press **B** on the canvas or select **Machines** from the menu to open the machine manager overlay.

| Key | Action |
|-----|--------|
| Up/Down | Select machine |
| Enter | Switch to the selected machine and return to canvas |
| A | Add a new machine (prompts for name) |
| R | Rename the selected machine (propagates to all link references) |
| D | Delete the selected machine (confirms, warns about incoming links) |
| Tab | Toggle the details panel |
| Esc | Close and return to menu |

The details panel shows outgoing links (states in this machine that link to others), incoming links (states in other machines that link to this one), and classes in use.

**Rename** updates the machine name everywhere: the machine list, all cache maps, all `LinkedMachines` references across every machine in the bundle, and the navigation stack. This is a safe, atomic operation.

**Delete** removes the machine from the bundle and clears all dangling link references in other machines. If the deleted machine is the one currently being edited, the editor switches to the first remaining machine. The last machine in a bundle cannot be deleted.


## Linked States

A linked state delegates execution to another machine. When the FSM runner reaches a linked state, it transfers control to the linked machine's initial state. Linked states are displayed in fuchsia with a `↗` suffix.

### Creating Links

Press **K** on a selected state. If other machines exist in the bundle, a selector appears. If no machines exist, the editor offers to create one. Pressing K on an already-linked state prompts to unlink it.

### Navigating Links

When a linked state is selected:

| Key | Action |
|-----|--------|
| Space | Dive into the linked machine |
| Shift+Right | Dive into the linked machine |
| Enter | Dive into the linked machine (also creates states if not linked) |
| Double-click | Dive into the linked machine |

To return to the parent machine:

| Key | Action |
|-----|--------|
| Shift+Left | Go back to parent |
| Ctrl+B | Go back to parent |
| Breadcrumb click | Click any ancestor in the breadcrumb bar |

The breadcrumb bar at the top of the screen shows the navigation path: `main > parser > validator`. Each segment is clickable.


## Class System

The class system allows you to define typed schemas and assign them to states. A class is a named collection of property definitions, each with a type, default value, and optional constraints. When a class is assigned to a state, that state inherits all the class's properties, which can then be customised per-state.

### Property Types

| Type | Description | Example |
|------|-------------|---------|
| `string` | Free text | `"74LS00"` |
| `int` | Integer with optional min/max | `14` |
| `float` | Floating-point with optional min/max | `3.3` |
| `bool` | True or false | `true` |
| `enum` | One of a defined set of values | `"CMOS"` |
| `list` | Ordered list of strings | `["VCC", "GND"]` |
| `map` | Key-value pairs | `{"1A": "input", "1B": "input"}` |

### Class Editor

Open from Settings (press C). The class editor has two panels: the class list on top, and the property list for the selected class below.

| Key | Action |
|-----|--------|
| Up/Down | Navigate classes |
| N | Create a new class (prompts for name) |
| D | Delete the selected class |
| Enter | Edit the selected class's properties |
| Tab | Switch focus between class list and property list |
| A | Add a property to the current class |
| E | Edit the selected property |
| Backspace | Delete the selected property |
| Esc | Return to Settings |

### Class Assignment Grid

Press **X** on the canvas to open the assignment grid. This shows a matrix of states versus available classes. Toggle assignments with Enter or Space. When a class is assigned, the state inherits its properties. Multiple classes can be assigned to a single state.

| Key | Action |
|-----|--------|
| Up/Down | Navigate states |
| Left/Right | Navigate classes |
| Enter/Space | Toggle class assignment |
| P | Edit property values for the selected state |
| Esc | Return to canvas |

### Property Editor

Press **P** on the canvas with a state selected (the state must have at least one class assigned). This opens a popup showing all properties inherited from the state's classes, with their current values. Edit values inline — the editor validates against the property type and constraints.

### Class Libraries

Class libraries are JSON files containing pre-defined class schemas. They are loaded from a directory configured in Settings. The toolkit ships with 74xx-series digital logic libraries in the `class-libraries/` directory, providing classes for gates, buffers, registers, arithmetic units, multiplexers, and more.

To load libraries: open Settings, navigate to the class library path setting, press Enter to browse for the directory, then press L to load. Loaded classes appear in the class editor and the component drawer.


## Component Drawer

The component drawer provides a visual catalogue of instantiable components from loaded class libraries. It appears as a bottom panel.

Press **C** on the canvas or click the **[+]** button (bottom-right corner) to open.

| Key | Action |
|-----|--------|
| Tab / Shift+Tab | Switch category |
| Left / Right | Browse components within category |
| Enter | Place the selected component at the canvas centre |
| Drag | Drag a component card onto the canvas |
| Esc | Close the drawer |

When a component is placed, a new state is created with the component's class assigned and all properties initialised to their defaults. The state name is derived from the class name (with a numeric suffix to avoid duplicates).

This provides a rapid instantiation workflow for projects with many typed components — drag from the drawer to get a fully-typed, property-initialised state in one action.


## Settings

The settings overlay configures editor behaviour. Open from the menu or by pressing Esc from the canvas and selecting Settings.

| Setting | Values | Description |
|---------|--------|-------------|
| Renderer | Graphviz / Native PNG / Native SVG | Which renderer to use for the Render command |
| File Type | `.fsm` / `.json` | Default save format |
| FSM Type | DFA / NFA / Moore / Mealy | Machine type (can be changed at any time) |
| Vocabulary | Standard / Digital / Custom | Cosmetic labels for sidebar headers |
| Class Library Path | Directory path | Where to load `.classes.json` files from |

| Key | Action |
|-----|--------|
| Up/Down | Navigate settings |
| Left/Right | Cycle values |
| Enter | Browse for directory (class library path) |
| L | Load class libraries from the configured path |
| C | Open the class editor |
| Esc | Return to menu |


## Validation and Analysis

Press **V** on the canvas to validate the FSM. Validation checks structural correctness — if it passes, the FSM can be executed. Errors are displayed in the status bar.

Press **L** on the canvas to run analysis (lint). Analysis checks for design quality issues: unreachable states, dead-end states, non-determinism in DFAs, incomplete transitions, unused symbols. Warnings are displayed in the status bar.


## Undo and Redo

The editor maintains an undo/redo stack for canvas operations (adding/removing states, moving states, adding/removing transitions, changing properties).

| Key | Action |
|-----|--------|
| Ctrl+Z | Undo |
| Ctrl+Y | Redo |

In bundle mode, each machine has its own independent undo/redo stack.


## Clipboard

Press **Ctrl+C** to copy the current FSM to the system clipboard in hex format. Press **Ctrl+V** to paste an FSM from the clipboard (replaces the current machine).


## File Operations

### New

Select **New** from the menu. If unsaved work exists, the editor prompts for confirmation before clearing. Creates an empty DFA.

### Open File

Select **Open File** from the menu. The file picker shows `.fsm` and `.json` files. Navigate with arrow keys, Enter to open.

### Import

Select **Import** from the menu. Importing adds machines from another file into the current project. If the file is a bundle, a multi-select picker appears — use Space to toggle individual machines, A to toggle all, Enter to import selected. Importing from a single-FSM file adds that FSM as a new machine. If the current project is a single FSM, importing promotes it to a bundle.

### Save / Save As

**Save** writes to the current file. **Save As** prompts for a new file path. The format is determined by the File Type setting (`.fsm` or `.json`). FSM files include labels and layout; JSON files include layout in a `_layout` field.

Press **Ctrl+S** to quick-save from any mode.

### Render

Select **Render** from the menu or press **R** on the canvas. Generates an image using the configured renderer (Graphviz, native PNG, or native SVG) and opens it with the system viewer.


## Layout Persistence

State positions are stored in `layout.toml` inside `.fsm` files. When a file is reopened, states appear where they were left. Each machine in a bundle has its own saved positions.

When opening an FSM without saved positions, the editor automatically arranges states using a layout algorithm selected based on the graph structure: Sugiyama for typical FSMs, circular for highly cyclic ones, force-directed for very dense graphs, and hierarchical for simple linear chains. After auto-layout, drag states to refine positions.


## Mouse Reference

| Action | Effect |
|--------|--------|
| Left-click on canvas | Move cursor |
| Left-click on state | Select state |
| Left-click and drag state | Move state |
| Right-click on canvas | Create state at position |
| Double-click on state | Rename (or dive into linked state) |
| Middle-drag on canvas | Enter drag mode with minimap |
| Click machine name in sidebar | Switch to that machine |
| Click breadcrumb segment | Navigate to that machine |
| Scroll wheel | Scroll sidebar or overlay lists |
| Click `[+]` button | Open component drawer |
| Drag divider | Resize sidebar |


## Complete Key Reference

### Canvas Mode

| Key | Action |
|-----|--------|
| Arrow keys | Move cursor |
| Shift+Arrow keys | Pan viewport |
| Tab | Cycle state selection |
| Enter | Create state (or dive into linked state) |
| Del/Backspace | Delete selected state |
| G | Grab and move selected state |
| T | Add transition from selected state |
| I | Add input symbol |
| O | Add output symbol |
| S | Set selected as initial state |
| A | Toggle accepting state |
| M | Set Moore output |
| K | Link state to machine |
| P | Edit state properties |
| X | Open class assignment grid |
| B | Open machine manager |
| C | Open component drawer |
| V | Validate FSM |
| L | Analyse FSM |
| R | Render to image |
| W | Toggle arc visibility |
| H / ? | Open help overlay |
| \\ | Toggle sidebar |
| Ctrl+D | Canvas drag mode |
| Ctrl+S | Save |
| Ctrl+C | Copy to clipboard |
| Ctrl+V | Paste from clipboard |
| Ctrl+Z | Undo |
| Ctrl+Y | Redo |
| Ctrl+B | Go back to parent machine |
| Space | Dive into linked state |
| Shift+Right | Dive into linked state |
| Shift+Left | Go back to parent machine |
| Esc | Return to menu |

### Menu Mode

| Key | Action |
|-----|--------|
| Up/Down | Navigate menu items |
| Enter | Select item |
| Esc | Return to canvas |

### Move Mode (after G)

| Key | Action |
|-----|--------|
| Arrow keys | Move the grabbed state |
| Enter | Confirm position |
| Esc | Cancel, restore original position |


## Platform Notes

**macOS.** Works in Terminal.app and iTerm2. Ctrl keys are interpreted as Ctrl, not Cmd, by most terminal emulators. Some terminals map Cmd+C to Ctrl+C; check your terminal settings if clipboard operations don't work.

**Linux.** Works in GNOME Terminal, Konsole, Alacritty, Kitty, and most other modern terminals. Ensure your TERM environment variable is set correctly (typically `xterm-256color`).

**Windows.** Use WSL2 with Windows Terminal. Native Windows terminals are not supported.

**FreeBSD / OpenBSD / NetBSD.** Works in xterm, rxvt-unicode, and other standard terminal emulators.


## License

Apache 2.0 — https://www.apache.org/licenses/LICENSE-2.0

Copyright (c) 2026 haitch — h@ual.fi
