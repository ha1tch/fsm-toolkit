# FSM Toolkit Specification

This document defines the semantic guarantees of the FSM Toolkit. It serves as a contract between the toolkit and its users.

**Version**: 1.0  
**Status**: Stable  
**Last Updated**: January 2025

---

## Table of Contents

1. [Overview](#overview)
2. [FSM Types](#fsm-types)
3. [Validation Rules](#validation-rules)
4. [Analysis Rules](#analysis-rules)
5. [Runtime Semantics](#runtime-semantics)
6. [Correctness Model](#correctness-model)
7. [Alphabet Enforcement](#alphabet-enforcement)
8. [Edge Cases](#edge-cases)

---

## Overview

### Purpose

This specification defines:
- What constitutes a valid FSM of each type
- What guarantees the toolkit makes about FSM behaviour
- The distinction between errors (validation) and warnings (analysis)
- Runtime behaviour in edge cases

### Terminology

| Term | Definition |
|------|------------|
| **Valid** | Passes validation; can be executed without runtime errors |
| **Clean** | Valid and passes analysis with no warnings |
| **Complete** | Has transitions for all (state, input) combinations |
| **Deterministic** | Has at most one transition per (state, input) pair |
| **Reachable** | Can be reached from the initial state via some input sequence |

### Conformance Levels

| Level | Meaning |
|-------|---------|
| **MUST** | Absolute requirement; violation is an error |
| **SHOULD** | Recommended; violation is a warning |
| **MAY** | Optional; no enforcement |

---

## FSM Types

The toolkit supports four FSM types. Each has specific structural requirements.

### DFA (Deterministic Finite Automaton)

A DFA is intended for deterministic recognition of regular languages.

**Structural Requirements:**

| Requirement | Level | Rationale |
|-------------|-------|-----------|
| Has exactly one initial state | MUST | Defines computation start |
| Initial state is in states list | MUST | Structural integrity |
| All transition targets exist | MUST | Prevents runtime errors |
| All transition inputs are in alphabet | MUST | Contract enforcement |
| No epsilon (null) transitions | MUST | DFA definition |
| At most one transition per (state, input) | SHOULD | DFA semantics |
| Transition for every (state, input) | SHOULD | Completeness |
| All states reachable from initial | SHOULD | No dead code |
| All non-accepting states have outgoing transitions | SHOULD | No traps |

**Semantics:**

A DFA accepts an input string if, starting from the initial state, following the transitions for each input symbol leads to an accepting state.

If a DFA violates SHOULD requirements:
- Multiple transitions on same input: Runtime uses NFA semantics (set-based simulation)
- Missing transition: Input is rejected (no valid transition = rejection)
- Unreachable states: Ignored during execution

### NFA (Nondeterministic Finite Automaton)

An NFA allows nondeterminism and epsilon transitions.

**Structural Requirements:**

| Requirement | Level | Rationale |
|-------------|-------|-----------|
| Has exactly one initial state | MUST | Defines computation start |
| Initial state is in states list | MUST | Structural integrity |
| All transition targets exist | MUST | Prevents runtime errors |
| All transition inputs are in alphabet (if not epsilon) | MUST | Contract enforcement |
| Epsilon transitions allowed | MAY | NFA feature |
| Multiple transitions per (state, input) allowed | MAY | NFA feature |
| Multiple target states per transition allowed | MAY | NFA feature |

**Semantics:**

An NFA accepts an input string if there exists at least one path from the initial state to an accepting state following the input symbols. The runtime uses powerset simulation (tracking all possible current states simultaneously).

Epsilon transitions are followed automatically:
- On initialisation, the current state set is the epsilon closure of the initial state
- After each input, epsilon closure is applied to all reached states

### Moore Machine

A Moore machine produces output based on the current state.

**Structural Requirements:**

All DFA/NFA requirements apply, plus:

| Requirement | Level | Rationale |
|-------------|-------|-----------|
| State outputs MAY be defined | MAY | Output is optional |
| If OutputAlphabet is defined, state outputs MUST be in it | MUST | Contract enforcement |
| States without defined outputs produce empty string | — | Default behaviour |

**Semantics:**

After each transition, the output is determined by the destination state. If a state has no defined output, the empty string is produced.

```
Output = StateOutputs[CurrentState] or ""
```

### Mealy Machine

A Mealy machine produces output based on the transition taken.

**Structural Requirements:**

All DFA/NFA requirements apply, plus:

| Requirement | Level | Rationale |
|-------------|-------|-----------|
| Transition outputs MAY be defined | MAY | Output is optional |
| If OutputAlphabet is defined, transition outputs MUST be in it | MUST | Contract enforcement |
| Transitions without defined outputs produce empty string | — | Default behaviour |

**Semantics:**

Each transition may have an associated output. When a transition is taken, its output is produced. If no output is defined, the empty string is produced.

```
Output = Transition.Output or ""
```

For NFA-style execution with multiple simultaneous transitions, outputs from all taken transitions are collected and concatenated (comma-separated in display).

---

## Validation Rules

Validation determines if an FSM is structurally correct and can be executed.

### Error Conditions

The following conditions cause validation to fail:

| ID | Condition | Message Format |
|----|-----------|----------------|
| V001 | No states defined | "FSM has no states" |
| V002 | No initial state | "FSM has no initial state" |
| V003 | Initial state not in states | "initial state X not in states" |
| V004 | Accepting state not in states | "accepting state X not in states" |
| V005 | Transition from undefined state | "transition N: from state X not in states" |
| V006 | Transition to undefined state | "transition N: to state X not in states" |
| V007 | Transition input not in alphabet | "transition N: input X not in alphabet" |
| V008 | Epsilon transition in DFA | "transition N: epsilon transitions not allowed in DFA" |
| V009 | Transition output not in output alphabet | "transition N: output X not in output alphabet" |
| V010 | State output not in output alphabet | "state X: output Y not in output alphabet" |

### Validation Guarantees

If `Validate()` returns no error:
- The FSM can be executed via `NewRunner()`
- All states referenced by transitions exist
- All inputs referenced by transitions are in the alphabet
- All outputs (if alphabet defined) are in the output alphabet
- Type-specific constraints are satisfied

---

## Analysis Rules

Analysis identifies potential design issues that don't prevent execution.

### Warning Conditions

| ID | Type | Condition | Severity |
|----|------|-----------|----------|
| A001 | unreachable | State not reachable from initial | Medium |
| A002 | dead | Non-accepting state with no outgoing transitions | Medium |
| A003 | nondeterministic | DFA has multiple transitions on same (state, input) | High |
| A004 | incomplete | DFA state missing transitions for some inputs | Low |
| A005 | unused_input | Input symbol not used in any transition | Low |
| A006 | unused_output | Output symbol not used in any state/transition | Low |

### Warning Details

**A001 - Unreachable States**
```
States: [list of unreachable state names]
```
These states cannot affect FSM behaviour and may indicate design errors.

**A002 - Dead States**
```
States: [list of dead state names]
```
Non-accepting states with no exits trap the FSM. May be intentional (error states) or a bug.

**A003 - Nondeterministic DFA**
```
States: [list of states with multiple transitions on same input]
```
The FSM is labelled DFA but has NFA characteristics. Runtime will use NFA semantics.

**A004 - Incomplete DFA**
```
States: [list of states missing some transitions]
```
Missing transitions cause rejection. May be intentional (implicit error handling) or a bug.

**A005 - Unused Input**
```
Symbols: [list of unused input symbols]
```
Alphabet contains symbols never used. May indicate incomplete design.

**A006 - Unused Output**
```
Symbols: [list of unused output symbols]
```
Output alphabet contains symbols never produced. May indicate incomplete design.

### Analysis Guarantees

If `Analyse()` returns no warnings:
- All states are reachable from initial
- No non-accepting dead-end states exist
- DFA is deterministic (if typed as DFA)
- DFA is complete (if typed as DFA)
- All input symbols are used
- All output symbols are used (if applicable)

---

## Runtime Semantics

### Initialisation

```
runner = NewRunner(fsm)
```

1. Validate FSM (fail if invalid)
2. Set current state(s) to epsilon closure of initial state
3. Clear history

### Step Execution

```
output, err = runner.Step(input)
```

1. If input not in available inputs, return error
2. Find all transitions from current state(s) matching input
3. If no transitions found, return error (DFA) or remain in current states (NFA)
4. Collect target states from all matching transitions
5. Apply epsilon closure to target states
6. Collect outputs:
   - Mealy: from transitions taken
   - Moore: from target states
7. Update current state(s)
8. Record step in history
9. Return collected output

### Acceptance

```
accepting = runner.IsAccepting()
```

Returns true if ANY current state is an accepting state.

### Available Inputs

```
inputs = runner.AvailableInputs()
```

Returns inputs that have at least one transition from ANY current state.

### Output Retrieval

```
output = runner.CurrentOutput()  // Moore machines
```

For Moore machines, returns the output of the current state. For NFA execution with multiple current states, returns comma-separated outputs.

---

## Correctness Model

### Validity Hierarchy

```
                    ┌─────────────┐
                    │    Clean    │  No errors, no warnings
                    │             │
              ┌─────┴─────────────┴─────┐
              │         Valid           │  No errors, may have warnings
              │                         │
        ┌─────┴─────────────────────────┴─────┐
        │              Loadable               │  Can parse, may be invalid
        │                                     │
  ┌─────┴─────────────────────────────────────┴─────┐
  │                    Parseable                    │  Syntactically correct
  └─────────────────────────────────────────────────┘
```

### State Transitions

```
File/JSON  ──parse──►  FSM struct  ──validate──►  Valid FSM  ──analyse──►  Clean FSM
              │                          │                          │
              ▼                          ▼                          ▼
         Parse error              Validation error           Warnings only
```

### Guarantees by Stage

| Stage | Guarantee |
|-------|-----------|
| After parse | FSM struct is populated, types are correct |
| After validate | FSM can be executed without runtime panic |
| After analyse (clean) | FSM has no structural issues or dead code |

---

## Alphabet Enforcement

### Input Alphabet

The input alphabet (`Alphabet` field) is ALWAYS enforced:
- Transitions with inputs not in the alphabet fail validation
- Runtime rejects inputs not in the alphabet

### Output Alphabet

The output alphabet (`OutputAlphabet` field) is CONDITIONALLY enforced:
- If `OutputAlphabet` is empty or nil: no enforcement (outputs are free-form)
- If `OutputAlphabet` is defined: all outputs MUST be in it

This allows:
- Simple FSMs without output alphabet restrictions
- Strict FSMs with enumerated valid outputs

### Empty Outputs

- Transitions/states without outputs produce empty string `""`
- Empty string is always valid, regardless of output alphabet
- Empty string is NOT required to be in output alphabet

---

## Edge Cases

### Empty FSM

An FSM with no states is invalid (V001).

### Single State FSM

Valid if:
- The single state is the initial state
- Type constraints are met

A single-state DFA with self-loop is a valid accepting-all or rejecting-all machine.

### Disconnected States

States not reachable from initial:
- Are valid (pass validation)
- Produce warning A001
- Are ignored during execution
- Are included in generated code

### Self-Loops

Self-loops (transitions where from == to) are valid for all types.

### Epsilon Loops

NFA may have epsilon loops (epsilon transitions forming a cycle). The epsilon closure algorithm handles this correctly (uses visited set).

### Empty Alphabet

An FSM with empty alphabet:
- Is valid if it has no transitions (or only epsilon transitions for NFA)
- Produces warning A005 if transitions exist but reference no alphabet

### Unicode

State names, inputs, and outputs MAY contain Unicode. The binary format uses numeric IDs; labels preserve Unicode strings.

---

## Appendix: Quick Reference

### Is My FSM Valid?

| Question | Valid? |
|----------|--------|
| DFA with epsilon transition | No (V008) |
| DFA with multiple transitions on same input | Yes (warning A003) |
| DFA missing some transitions | Yes (warning A004) |
| Moore without output for every state | Yes |
| Mealy with output not in OutputAlphabet | No (V009) |
| FSM with unreachable states | Yes (warning A001) |
| FSM with no initial state | No (V002) |

### What Happens At Runtime?

| Situation | Behaviour |
|-----------|-----------|
| Input not in alphabet | Error returned |
| No transition for input (DFA) | Error returned |
| No transition for input (NFA) | State set becomes empty, not accepting |
| Multiple transitions (any type) | All targets added to state set |
| Missing Moore output | Empty string returned |
| Missing Mealy output | Empty string returned |

---

## Appendix: Verification

This specification is verified by automated tests:

### Property-Based Tests (`tests/spec_test.go`)

Each claim in this specification has a corresponding test:
- `TestSpec_DFA_NoEpsilon` — verifies V008
- `TestSpec_DFA_NondeterministicWarning` — verifies A003 is warning not error
- `TestSpec_Moore_OutputAlphabetEnforced` — verifies V010
- `TestSpec_Runtime_EpsilonClosureOnInit` — verifies NFA initialisation
- ... and 30+ more

Run with: `go test -v ./tests/`

### Fuzz Tests (`tests/fuzz/`)

Parsers and core functions are fuzz-tested to ensure they don't panic on malformed input:
- `FuzzParseHex` — hex record parser
- `FuzzParseJSON` — JSON parser  
- `FuzzRunner` — FSM execution engine
- `FuzzSVGNative` — SVG generator
- ... and 7 more

Run with: `go test -fuzz=FuzzParseHex -fuzztime=30s ./tests/fuzz/`

---

*This specification is normative. Behaviour not covered here is implementation-defined and may change.*
