# Changelog

## [0.9.6] - 2026-03-01

### Added

#### Structural Connectivity (Phases 1–5)
- Port model: classes can define typed ports with name, direction (input/output/bidirectional/power), pin number, and functional group
- Net model: named electrical connections between ports on different component instances, with multi-fan-out support
- Canvas net rendering: two-endpoint direct routes, multi-endpoint horizontal buses with vertical stubs, signal/power colour distinction
- Connection detail window (E key): pin-to-pin view with direction arrows, grouped by functional unit, add/delete/rename connections
- Net visibility toggle (N key) on the editor canvas

#### Netlist Export
- `fsm netlist` command with three output formats: text (human-readable), KiCad (S-expression .net), and JSON
- Automatic KiCad library and footprint derivation for 74xx components (DIP-14/DIP-16/DIP-20/DIP-24)
- `--bake` flag to write derived KiCad fields back into source files for manual override
- Baking works on both FSM files and class library files; idempotent
- Machine selection for bundle export (`--machine` flag)

#### Vocabulary System (package-level)
- Vocabulary moved from fsmedit surface to `pkg/fsm/vocab.go` as a first-class FSM property
- Three vocabularies: `"fsm"` (state/transition/alphabet), `"circuit"` (component/connection/signal), `"generic"` (node/edge/label)
- `"auto"` mode: detects vocabulary from structural features (classes with ports or nets resolve to circuit)
- Persisted in JSON (`"vocabulary"` field) and labels.toml (`vocabulary` in `[fsm]` section)
- CLI commands (info, analyse, validate, run) use vocabulary-aware labels throughout
- fsmedit delegates to `pkg/fsm`, with settings panel cycling all four modes and auto-resolution preview

#### Example Circuits
- `gated_d_flipflop.json`: 7408 AND gate with 7404 inverter and 7474 D flip-flop
- `counter_7seg.json`: 74161 decade counter with 7447 BCD-to-7-segment decoder and 7400 auto-reset
- `comparator_leds.json`: 7485 4-bit magnitude comparator with 7404 LED drivers
- `codelock.fsm`: three-machine bundle (scanner, matcher, controller) with 8 component types, 14 instances, linked-state delegation, and full behavioural+structural coverage

### Fixed
- `CreateBundle` now preserves `classes.json` files during bundling (previously silently dropped class/port/net data)
- File picker now starts from the current working directory instead of a stale `last_dir` from the config file; import picker starts from the directory of the loaded file
- Connection detail and peer picker no longer crash to shell on any keypress (handlers incorrectly returned "quit" signal)
- Peer picker window title and labels now use vocabulary system instead of hardcoded terms
- Connection detail: highlight now fills the full row width continuously
- Connection detail: group labels (`[FF1]`, `[GATE_A]`) rendered in teal to distinguish from data rows
- Connection detail: power net rows shown in amber, signal net rows in cyan; colours preserved through highlight
- Connection detail: power nets now included in the display (previously silently excluded)

### Changed

#### Documentation Reorganisation
- Documentation moved from root to `docs/` directory with lowercase filenames
- `docs/index.md` as documentation hub with four sections: Guides, Tool Manuals, Reference, Examples
- New `docs/design-philosophy.md` explaining the toolkit's architecture, dual-plane model, and design boundaries
- New `docs/circuits.md` guide explaining the dual behavioural-structural model
- All cross-references updated (52 links across 13 files verified)
- README updated: docs table points to `docs/`, descriptions reflect netlist export and structural features, `pkg/export` added to Go packages listing

## [0.9.5] - 2026-03-01

### Added

#### Machine Management Window
- Consolidated machine manager overlay (B key or Machines menu)
- Add, rename, delete, and switch machines from a single interface
- Rename propagates to all `LinkedMachines` references across the bundle
- Rename updates navigation stack, cache maps, and current machine reference
- Delete warns about incoming links from other machines
- Delete clears dangling link references across the bundle
- Details panel (Tab) shows outgoing links, incoming links, and classes in use
- Scroll support for large machine lists

#### Class and Property System
- Seven property types: `float64`, `int64`, `uint64`, `[40]string`, `string`, `bool`, `list`
- Class editor for defining schemas with typed property definitions
- Class assignment grid for attaching classes to states
- Per-state property editor with type validation and constraints
- List property popup editor with add/remove/reorder
- JSON serialization of class assignments and property values
- Class data round-trips through all format conversions

#### Class Libraries and Component Drawer
- `.classes.json` file format for distributing class libraries
- Seven 74xx-series digital logic libraries (gates, buffers, registers, arithmetic, multiplexers, sequential, specialty)
- Component drawer (C key or [+] button) for browsing and instantiating components
- Category navigation, component preview, keyboard and drag-and-drop placement
- Components placed with class assigned and properties initialized to defaults

#### Settings Screen
- Configurable renderer (Graphviz, native PNG, native SVG)
- File type selection (FSM or JSON)
- FSM type switching
- Vocabulary system (Standard, Digital, Custom) for sidebar labels
- Class library directory browser with L key to load

#### Multi-Document Interface
- Import command adds machines from external files
- Import from bundles with multi-select picker (Space to toggle, A for all)
- Automatic promotion from single-FSM to bundle mode
- Sidebar header shows machine list with click-to-switch
- Bundle mode indicator in sidebar

#### CI/CD Pipeline
- GitHub Actions workflow with test, dist, and release jobs
- Cross-compilation for 12 platforms: Linux (amd64, arm64, armv7, armv6), macOS (amd64, arm64), Windows (amd64, arm64), FreeBSD (amd64, arm64), OpenBSD (amd64), NetBSD (amd64)
- Distribution archives with binaries, documentation, examples, and class libraries
- SHA256SUMS.txt checksum generation
- Automatic GitHub release creation on version tags
- Pre-release detection for `-rc`, `-beta`, `-alpha` tags
- Local `build-all.sh` script matching CI platforms and packaging

#### Documentation
- Separate CLI manual (`cmd/fsm/MANUAL.md`) with full command reference
- Separate editor manual (`cmd/fsmedit/MANUAL.md`) with complete key/mouse reference
- Workflow guide (`WORKFLOWS.md`) covering nine common usage patterns
- Root `MANUAL.md` refactored as documentation index
- All documents cross-linked with verified relative paths
- Per-tool manuals included in distribution archives

#### Version Management
- `pkg/version` package with canonical version string
- `VERSION` file as single source of truth
- `scripts/sync-version.sh` for synchronization and bumping

### Changed

#### Editor Code Organization
- Split monolithic `main.go` into seven logical files: config, handlers, file_ops, bundle, actions, undo, draw_modal
- Extracted UI helpers into separate file with nil-screen guards

#### Layout Algorithms
- Separated TUI and SVG layout paths for coordinate-system-appropriate metrics
- Cell-grid model for TUI with blank routing cells for high-degree states
- Improved horizontal spacing based on transition label widths

#### Menu Restructuring
- Replaced separate "Add Machine" and "Switch Machine" entries with unified "Machines" entry
- Import command accessible from main menu

#### README
- Streamlined from 136 lines to 80 lines
- Detail moved to per-tool manuals; README now links to all documentation

### Fixed

- Class and property data now persisted in `.fsm` files (previously silently dropped on save)
- `fsm info` now shows bundle summary when inspecting a bundle without `--machine` flag
- `fsm info` displays class assignments and property counts
- Seven bundle navigation bugs: keyboard shortcuts, machine switching, animation
- State bleeding between machines when switching in bundle mode
- Save/Save As functionality in bundle mode
- Class editor scrolling and mouse handling in library-loaded environments
- Component drawer button positioning and discoverability
- Help overlay updated to reflect all current features
- FSM Type display in settings screen

## [0.9.0] - 2025-01-18

### Added

#### Linked States
- States can now delegate to other machines in a bundle
- `FSM.LinkedMachines` map stores state → machine name mappings
- `FSM.IsLinked()`, `GetLinkedMachine()`, `SetLinkedMachine()`, `LinkedStates()`, `HasLinkedStates()` helper methods
- JSON format supports `"linked_machines": {"state": "machine"}` field
- labels.toml supports `[machines]` section for link definitions
- Hex format uses bit 2 (0x0004) in state flags for linked states

#### Bundle Runner (Linked State Execution)
- `BundleRunner` type executes FSMs with automatic linked state delegation
- Automatic delegation when entering a linked state
- Automatic return to parent when child reaches terminal state
- Child acceptance/rejection determines parent's next transition
- `>>` prompt indicates delegation depth in interactive mode
- `machines` command shows active machine and delegation depth
- Maximum delegation depth of 16 to prevent infinite loops
- Full execution history including delegation/return events

#### Visual Indicators for Linked States
- **PNG**: Light purple fill (#f3e5f5), purple border (#8e24aa), dashed inner ring, "→machineName" label
- **SVG**: `.state-linked` CSS class with same styling, `stroke-dasharray` for dashed ring
- **fsmedit**: Fuchsia colour, ↗ suffix on state name, target machine displayed below

#### Bundle Validation
- `fsm validate <bundle> --bundle` validates linked state references
- Checks: target machines exist, targets are DFAs, no circular links, no self-links
- Reports warnings for linked states without target machine names

#### fsmedit Linked State Editing
- Press `k` on selected state to set/toggle linked machine
- In bundle mode: shows machine selector popup
- Standalone mode: prompts for machine name
- Unlink by pressing `k` on already-linked state

#### Bundle Support (Composite FSMs)
- Multiple FSMs can be stored in a single `.fsm` file
- Each machine is named after its source file
- `fsm machines <bundle.fsm>` lists all machines in a bundle
- `fsm bundle <a.fsm> <b.fsm> -o combined.fsm` creates bundles
- `fsm extract <bundle.fsm> --machine <n> -o out.fsm` extracts single machine
- `--machine` flag for all commands: `info`, `png`, `svg`, `dot`, `generate`, `analyse`, `validate`, `run`

#### Batch Code Generation
- `fsm generate bundle.fsm --all --lang <c|rust|go>` generates code for all machines
- Output files named `<machine>.<ext>` (e.g., `parent.go`, `child.go`)
- Each machine gets its own file with appropriate package/module structure

#### Batch Analysis
- `fsm analyse bundle.fsm --all` analyses all machines plus cross-machine issues
- `fsm analyze` accepted as American spelling alias
- Cross-machine checks: orphaned machines, missing link targets, accept/reject inputs
- Per-machine analysis followed by bundle-wide summary

#### Batch Rendering
- `fsm png bundle.fsm --all --native` renders all machines to separate files
- `fsm svg bundle.fsm --all -o /path/%s.svg` with pattern for output names
- Creates output directories automatically

#### fsmedit Bundle Support
- Machine selector appears when opening a bundle
- "Switch Machine" menu item when editing a bundle
- Status bar shows current machine name: `file.fsm [machine_name]`
- Navigate between machines without closing the file

#### fsmedit Linked State Navigation
- **Breadcrumb bar**: Shows navigation path when editing child machines (◀ main › child)
- **Enter key**: Dives into linked state when selected
- **Double-click**: Dives into linked state (or edits name for non-linked)
- **Ctrl+B**: Navigates back to parent machine
- **◀ button**: Click breadcrumb back button to return
- **Breadcrumb segments**: Click any ancestor to jump directly to it
- **Zoom animation**: Visual transition when navigating between machines
- **Navigation stack**: Preserves edits and state positions at each level

#### fsmedit Bundle Save
- Save (Ctrl+S) saves all modified machines in the bundle
- Status bar shows `* (+N)` where N = other modified machines
- Quit warns "N machines have unsaved changes" for bundles
- Per-machine modified tracking with visual indicator

#### File Format Extensions
- Bundles use namespaced files: `main.hex`, `child.hex`, `main.labels.toml`, etc.
- Backward compatible: single-machine files still use `machine.hex`

---

## [0.8.6] - 2025-01-18

### Added

#### Virtual Canvas (512×512)
- Canvas is now 512×512 logical units, independent of screen size
- States can be positioned anywhere within the virtual canvas
- Viewport shows a window into the larger canvas space

#### Canvas Navigation
- `Ctrl+D` enters canvas drag mode with minimap overlay
- Middle mouse button drag also enters drag mode
- Arrow keys pan viewport while in drag mode
- `Shift+Arrow` for quick panning without minimap
- `Esc` or `Ctrl+D` exits drag mode

#### Minimap Overlay
- Shows entire 512×512 canvas scaled to fit
- States displayed as green dots
- Yellow rectangle shows current viewport position
- Appears automatically during canvas drag mode

#### Scroll Indicators
- Arrow indicators appear at canvas edges when content exists off-screen
- Visual feedback for navigation: `◀` `▶` `▲` `▼`

#### Drag-to-Edge Auto-Scroll
- Dragging a state near the viewport edge automatically scrolls
- Allows repositioning states beyond the initial visible area
- States clamped to canvas bounds (512×512)

### Changed

- State positions now stored in abstract canvas coordinates
- Help overlay updated with canvas navigation documentation

---

## [0.8.5] - 2025-01-18

### Added

#### Interactive Sidebar
- Click states in sidebar to select them on canvas
- Click inputs to flash all transitions using that input symbol
- Click outputs to flash all transitions using that output symbol  
- Click transitions to flash that specific transition on canvas
- Flashing persists until keypress or new selection (alternating white/blue, 200ms)
- Selected items highlighted in cyan in sidebar during flash

#### Sidebar Scrolling
- Scrollbar appears when content exceeds visible area
- Click and drag scrollbar thumb to navigate
- Mouse wheel scrolling support
- Click anywhere on scrollbar track to jump to position

#### Collapsible/Resizable Sidebar
- Drag the divider line to resize sidebar width
- Press `\` to toggle sidebar collapse/expand
- Snap points at default width (30) and maximum width (60)
- Visual indicators: `▶` when expanded, `◀` when collapsed

#### Clipboard Paste with Smart Layout
- `Ctrl+V` / `Cmd+V` pastes FSM data from clipboard
- Preserves relative positions from copied layout
- Automatic conflict resolution: renamed states get `_1`, `_2` suffixes
- Intelligent placement: fills left-to-right, wraps to new row when full
- Merges alphabets and transitions into existing FSM

#### Clipboard Copy Enhancement  
- `Ctrl+C` / `Cmd+C` now copies complete FSM data including layout
- Format includes hex records + labels.toml + layout.toml
- Enables copy/paste workflow between sessions and instances

#### Right-Click Instant State Creation
- Right-click on empty canvas creates state immediately
- Auto-generates unique name (S0, S1, S2...)
- No prompt interruption — rename later via double-click

### Changed

- Empty canvas now shows error message when attempting to render
- Sidebar divider highlighted yellow while dragging

### Fixed

- Paste positioning no longer overlaps with existing content
- Vertical spacing when paste wraps to new row

---

## [0.8.0] - 2025-01-17

### Added

- Native PNG renderer with 4x supersampling antialiasing
- Intelligent self-loop positioning (finds unoccupied side)
- Quadratic Bézier curves for transitions
- Message flash animation for errors/warnings/state changes
- FSM type display normalized to uppercase (DFA, NFA, MOORE, MEALY)
- Scrollable help overlay with PgUp/PgDn support
- Double-click to rename states
- Mouse drag to move states
- Hybrid visibility routing for transition arcs

### Changed

- Improved arc routing to avoid state overlaps
- Help documentation updated to reflect actual features

