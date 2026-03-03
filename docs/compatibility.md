# FSM Toolkit Compatibility

This document defines the compatibility guarantees of the FSM Toolkit.

**Version**: 1.0  
**Status**: Stable  
**Last Updated**: January 2025

---

## Binary Format Compatibility

### Record Type Stability

| Range | Status | Meaning |
|-------|--------|---------|
| 0000-0003 | **FROZEN** | Semantics will never change |
| 0004-00FF | Reserved | Future core features |
| 0100-FFFF | Extension | Third-party extensions |

### Frozen Record Types

These record types are permanently defined:

| Type | Name | Format | Status |
|------|------|--------|--------|
| 0000 | DFA/NFA Transition | `0000 FROM:INPUT TO:0000` | Frozen |
| 0001 | Mealy Transition | `0001 FROM:INPUT TO:OUTPUT` | Frozen |
| 0002 | State Declaration | `0002 ID:FLAGS OUTPUT:0000` | Frozen |
| 0003 | NFA Multi-Target | `0003 FROM:INPUT TO1:TO2` | Frozen |

**Guarantee**: Files using only types 0000-0003 will load correctly in all future versions.

### Forward Compatibility

Tools MUST handle unknown record types gracefully:

1. Unknown record types (≥0004) MUST be ignored, not rejected
2. Files with unknown types MUST still load
3. Unknown types MAY cause feature loss (e.g., extension data not interpreted)
4. Tools SHOULD emit a warning when ignoring unknown types

**Example**: A v1.0 tool loading a v2.0 file with type 0010 records will:
- Load all 0000-0003 records normally
- Ignore 0010 records (possibly with warning)
- Produce a usable FSM (missing v2.0-specific data)

### Backward Compatibility

All future versions MUST:

1. Read all past file versions without error
2. Interpret frozen record types identically
3. Preserve round-trip fidelity for known types

**Guarantee**: A file saved today will load correctly in all future versions.

### Breaking Changes Policy

If a fundamental change is required:

1. A new file extension MUST be introduced (e.g., `.fsm2`)
2. The old format MUST remain supported for reading
3. Migration tooling MUST be provided
4. Deprecation notice MUST be given at least one major version in advance

---

## File Format Sections

### .fsm Archive Structure

| Section | Status | Compatibility |
|---------|--------|---------------|
| `machine.hex` | Stable | Required, format frozen |
| `labels.toml` | Stable | Optional, format frozen |
| `layout.toml` | Stable | Optional, format frozen |

### Section Independence

- Files without `labels.toml` use numeric identifiers
- Files without `layout.toml` use automatic layout
- Unknown sections in the archive MUST be preserved on re-save

---

## JSON Format Compatibility

### Schema Stability

The JSON schema is stable. Fields:

| Field | Status | Notes |
|-------|--------|-------|
| `type` | Frozen | "dfa", "nfa", "moore", "mealy" |
| `name` | Frozen | Optional string |
| `description` | Frozen | Optional string |
| `states` | Frozen | Array of strings |
| `alphabet` | Frozen | Array of strings |
| `output_alphabet` | Frozen | Optional array of strings |
| `initial` | Frozen | String |
| `accepting` | Frozen | Array of strings |
| `transitions` | Frozen | Array of transition objects |
| `state_outputs` | Frozen | Optional map (Moore) |

### Transition Object

| Field | Status | Notes |
|-------|--------|-------|
| `from` | Frozen | String |
| `to` | Frozen | String or array of strings |
| `input` | Frozen | String or null (epsilon) |
| `output` | Frozen | Optional string (Mealy) |

### Extension Fields

- Unknown fields at the root level MUST be ignored
- Unknown fields in transitions MUST be ignored
- Parsers MUST NOT fail on unknown fields

---

## Go API Compatibility

### Package Stability

| Package | Status | Guarantee |
|---------|--------|-----------|
| `pkg/fsm` | **Stable** | No breaking changes |
| `pkg/fsmfile` | **Stable** | No breaking changes |
| `pkg/codegen` | Unstable | May change between minor versions |

### Stable API Surface

The following are guaranteed stable:

**pkg/fsm:**
```go
// Types
type FSM struct { ... }
type Transition struct { ... }
type FSMType string
const TypeDFA, TypeNFA, TypeMoore, TypeMealy FSMType

// Functions
func New(t FSMType) *FSM
func NewRunner(f *FSM) (*Runner, error)

// FSM methods
func (f *FSM) AddState(name string)
func (f *FSM) AddTransition(from string, input *string, to []string, output *string)
func (f *FSM) SetInitial(state string)
func (f *FSM) SetAccepting(states []string)
func (f *FSM) SetStateOutput(state, output string)
func (f *FSM) Validate() error
func (f *FSM) Analyse() []ValidationWarning
func (f *FSM) IsAccepting(state string) bool

// Runner methods
func (r *Runner) CurrentState() string
func (r *Runner) CurrentStates() []string
func (r *Runner) CurrentOutput() string
func (r *Runner) IsAccepting() bool
func (r *Runner) AvailableInputs() []string
func (r *Runner) Step(input string) (*Step, error)
func (r *Runner) Reset()
func (r *Runner) History() []Step
```

**pkg/fsmfile:**
```go
func ReadFSMFile(path string) (*fsm.FSM, *Layout, error)
func WriteFSMFile(path string, f *fsm.FSM, includeLabels bool) error
func ParseJSON(data []byte) (*fsm.FSM, error)
func ToJSON(f *fsm.FSM, pretty bool) ([]byte, error)
func GenerateDOT(f *fsm.FSM, title string) string
func GenerateSVGNative(f *fsm.FSM, opts SVGOptions) string
func SmartLayout(f *fsm.FSM, width, height int) map[string][2]int
```

### Unstable API

The `pkg/codegen` package may change:
- Function signatures may change
- Generated code format may change
- New target languages may be added
- Existing targets may be improved

**Recommendation**: Pin to specific versions if depending on codegen output format.

---

## CLI Compatibility

### Command Stability

| Command | Status | Guarantee |
|---------|--------|-----------|
| `convert` | Stable | Flags and behaviour frozen |
| `dot` | Stable | Output format frozen |
| `png` | Stable | Requires Graphviz |
| `svg` | Stable | `--native` flag stable |
| `info` | Stable | Output format frozen |
| `validate` | Stable | Exit codes frozen |
| `analyse` | Stable | Warning types frozen |
| `run` | Stable | Interactive mode frozen |
| `view` | Stable | Output format frozen |
| `generate` | Unstable | Generated code may change |
| `edit` | Stable | Launches fsmedit |

### Exit Codes

| Code | Meaning | Stable |
|------|---------|--------|
| 0 | Success | Yes |
| 1 | Error (general) | Yes |

### Output Format

- `info` output format is stable (field names, order)
- `analyse` warning type names are stable
- `validate` error messages may change wording (not structure)

---

## Deprecation Policy

### Process

1. Feature marked deprecated in release notes
2. Deprecation warning emitted on use
3. Minimum one major version before removal
4. Migration guide provided

### Current Deprecations

None.

---

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | Jan 2025 | Initial specification |

---

## Summary of Guarantees

### What Will Never Break

- Loading files created with record types 0000-0003
- JSON schema for FSM definitions
- `pkg/fsm` and `pkg/fsmfile` API signatures
- Core CLI commands and their primary flags

### What May Change

- Generated code format (codegen)
- Error message wording
- Warning message wording
- Default values for optional parameters
- Performance characteristics

### What Requires Major Version

- New required fields in JSON/binary format
- Removal of stable API methods
- Change to validation semantics
- Change to runtime behaviour

---

*This compatibility document is a contract. Violations are bugs.*
