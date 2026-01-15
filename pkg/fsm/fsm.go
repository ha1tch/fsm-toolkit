// Package fsm provides core finite state machine types and operations.
package fsm

import (
	"fmt"
	"strings"
)

// Type represents the kind of FSM.
type Type string

const (
	TypeDFA   Type = "dfa"
	TypeNFA   Type = "nfa"
	TypeMoore Type = "moore"
	TypeMealy Type = "mealy"
)

// Transition represents a state transition.
type Transition struct {
	From   string   `json:"from"`
	Input  *string  `json:"input"` // nil for epsilon
	To     []string `json:"to"`    // single element for DFA, multiple for NFA
	Output *string  `json:"output,omitempty"` // Mealy only
}

// FSM represents a finite state machine.
type FSM struct {
	Type        Type              `json:"type"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	States      []string          `json:"states"`
	Alphabet    []string          `json:"alphabet"`
	Initial     string            `json:"initial"`
	Accepting   []string          `json:"accepting"`
	Transitions []Transition      `json:"transitions"`
	StateOutputs map[string]string `json:"state_outputs,omitempty"` // Moore
	OutputAlphabet []string       `json:"output_alphabet,omitempty"`
}

// New creates a new FSM with the given type.
func New(t Type) *FSM {
	return &FSM{
		Type:         t,
		States:       make([]string, 0),
		Alphabet:     make([]string, 0),
		Accepting:    make([]string, 0),
		Transitions:  make([]Transition, 0),
		StateOutputs: make(map[string]string),
		OutputAlphabet: make([]string, 0),
	}
}

// AddState adds a state to the FSM.
func (f *FSM) AddState(name string) {
	for _, s := range f.States {
		if s == name {
			return
		}
	}
	f.States = append(f.States, name)
}

// AddInput adds an input symbol to the alphabet.
func (f *FSM) AddInput(symbol string) {
	for _, s := range f.Alphabet {
		if s == symbol {
			return
		}
	}
	f.Alphabet = append(f.Alphabet, symbol)
}

// AddOutput adds an output symbol to the output alphabet.
func (f *FSM) AddOutput(symbol string) {
	for _, s := range f.OutputAlphabet {
		if s == symbol {
			return
		}
	}
	f.OutputAlphabet = append(f.OutputAlphabet, symbol)
}

// AddTransition adds a transition to the FSM.
func (f *FSM) AddTransition(from string, input *string, to []string, output *string) {
	t := Transition{
		From:   from,
		Input:  input,
		To:     to,
		Output: output,
	}
	f.Transitions = append(f.Transitions, t)
}

// SetInitial sets the initial state.
func (f *FSM) SetInitial(state string) {
	f.Initial = state
}

// SetAccepting sets the accepting states.
func (f *FSM) SetAccepting(states []string) {
	f.Accepting = states
}

// SetStateOutput sets the output for a state (Moore machine).
func (f *FSM) SetStateOutput(state, output string) {
	if f.StateOutputs == nil {
		f.StateOutputs = make(map[string]string)
	}
	f.StateOutputs[state] = output
}

// Validate checks if the FSM is well-formed.
func (f *FSM) Validate() error {
	if len(f.States) == 0 {
		return fmt.Errorf("FSM has no states")
	}
	
	if f.Initial == "" {
		return fmt.Errorf("FSM has no initial state")
	}
	
	// Check initial state exists
	found := false
	for _, s := range f.States {
		if s == f.Initial {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("initial state %q not in states", f.Initial)
	}
	
	// Check accepting states exist
	for _, acc := range f.Accepting {
		found = false
		for _, s := range f.States {
			if s == acc {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("accepting state %q not in states", acc)
		}
	}
	
	// Check transitions reference valid states and inputs
	for i, t := range f.Transitions {
		found = false
		for _, s := range f.States {
			if s == t.From {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("transition %d: from state %q not in states", i, t.From)
		}
		
		for _, to := range t.To {
			found = false
			for _, s := range f.States {
				if s == to {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("transition %d: to state %q not in states", i, to)
			}
		}
		
		if t.Input != nil {
			found = false
			for _, a := range f.Alphabet {
				if a == *t.Input {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("transition %d: input %q not in alphabet", i, *t.Input)
			}
		}
	}
	
	return nil
}

// StateIndex returns the index of a state, or -1 if not found.
func (f *FSM) StateIndex(state string) int {
	for i, s := range f.States {
		if s == state {
			return i
		}
	}
	return -1
}

// InputIndex returns the index of an input, or -1 if not found.
func (f *FSM) InputIndex(input string) int {
	for i, a := range f.Alphabet {
		if a == input {
			return i
		}
	}
	return -1
}

// OutputIndex returns the index of an output, or -1 if not found.
func (f *FSM) OutputIndex(output string) int {
	for i, o := range f.OutputAlphabet {
		if o == output {
			return i
		}
	}
	return -1
}

// IsAccepting returns true if the state is an accepting state.
func (f *FSM) IsAccepting(state string) bool {
	for _, acc := range f.Accepting {
		if acc == state {
			return true
		}
	}
	return false
}

// GetTransitions returns all transitions from a state on a given input.
// For DFA, returns at most one transition. For NFA, may return multiple.
func (f *FSM) GetTransitions(from string, input *string) []Transition {
	var result []Transition
	for _, t := range f.Transitions {
		if t.From != from {
			continue
		}
		// Match input (nil matches nil for epsilon)
		if (t.Input == nil && input == nil) ||
			(t.Input != nil && input != nil && *t.Input == *input) {
			result = append(result, t)
		}
	}
	return result
}

// GetEpsilonTransitions returns all epsilon transitions from a state.
func (f *FSM) GetEpsilonTransitions(from string) []Transition {
	return f.GetTransitions(from, nil)
}

// String returns a string representation of the FSM.
func (f *FSM) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("FSM[%s]: %s\n", f.Type, f.Name))
	sb.WriteString(fmt.Sprintf("  States: %v\n", f.States))
	sb.WriteString(fmt.Sprintf("  Alphabet: %v\n", f.Alphabet))
	sb.WriteString(fmt.Sprintf("  Initial: %s\n", f.Initial))
	sb.WriteString(fmt.Sprintf("  Accepting: %v\n", f.Accepting))
	sb.WriteString(fmt.Sprintf("  Transitions: %d\n", len(f.Transitions)))
	return sb.String()
}

// ValidationWarning represents a non-fatal issue with the FSM.
type ValidationWarning struct {
	Type    string
	Message string
	States  []string // affected states, if applicable
}

// Analyse performs structural analysis and returns warnings.
// This checks for issues that don't prevent the FSM from running
// but may indicate design problems.
func (f *FSM) Analyse() []ValidationWarning {
	var warnings []ValidationWarning

	// Check for unreachable states
	unreachable := f.UnreachableStates()
	if len(unreachable) > 0 {
		warnings = append(warnings, ValidationWarning{
			Type:    "unreachable",
			Message: fmt.Sprintf("%d state(s) not reachable from initial state", len(unreachable)),
			States:  unreachable,
		})
	}

	// Check for dead states (no outgoing transitions)
	dead := f.DeadStates()
	if len(dead) > 0 {
		warnings = append(warnings, ValidationWarning{
			Type:    "dead",
			Message: fmt.Sprintf("%d state(s) have no outgoing transitions", len(dead)),
			States:  dead,
		})
	}

	// Check for non-determinism in DFA
	if f.Type == TypeDFA {
		nondet := f.NonDeterministicStates()
		if len(nondet) > 0 {
			warnings = append(warnings, ValidationWarning{
				Type:    "nondeterministic",
				Message: fmt.Sprintf("%d state(s) have multiple transitions on same input", len(nondet)),
				States:  nondet,
			})
		}
	}

	// Check for incomplete transitions (DFA should have transition for every input)
	if f.Type == TypeDFA {
		incomplete := f.IncompleteStates()
		if len(incomplete) > 0 {
			warnings = append(warnings, ValidationWarning{
				Type:    "incomplete",
				Message: fmt.Sprintf("%d state(s) missing transitions for some inputs", len(incomplete)),
				States:  incomplete,
			})
		}
	}

	// Check for unused inputs
	unusedInputs := f.UnusedInputs()
	if len(unusedInputs) > 0 {
		warnings = append(warnings, ValidationWarning{
			Type:    "unused_input",
			Message: fmt.Sprintf("%d input(s) not used in any transition", len(unusedInputs)),
		})
	}

	// Check for unused outputs (Moore/Mealy)
	if f.Type == TypeMoore || f.Type == TypeMealy {
		unusedOutputs := f.UnusedOutputs()
		if len(unusedOutputs) > 0 {
			warnings = append(warnings, ValidationWarning{
				Type:    "unused_output",
				Message: fmt.Sprintf("%d output(s) not used", len(unusedOutputs)),
			})
		}
	}

	return warnings
}

// UnreachableStates returns states not reachable from the initial state.
func (f *FSM) UnreachableStates() []string {
	if f.Initial == "" {
		return f.States // all unreachable if no initial
	}

	// BFS from initial state
	reachable := make(map[string]bool)
	queue := []string{f.Initial}
	reachable[f.Initial] = true

	// Build adjacency for faster lookup
	adj := make(map[string][]string)
	for _, t := range f.Transitions {
		adj[t.From] = append(adj[t.From], t.To...)
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, next := range adj[current] {
			if !reachable[next] {
				reachable[next] = true
				queue = append(queue, next)
			}
		}
	}

	// Find unreachable
	var unreachable []string
	for _, s := range f.States {
		if !reachable[s] {
			unreachable = append(unreachable, s)
		}
	}
	return unreachable
}

// DeadStates returns states with no outgoing transitions.
// Accepting states without outgoing transitions are not considered dead.
func (f *FSM) DeadStates() []string {
	// Count outgoing transitions per state
	outgoing := make(map[string]int)
	for _, t := range f.Transitions {
		outgoing[t.From]++
	}

	var dead []string
	for _, s := range f.States {
		if outgoing[s] == 0 && !f.IsAccepting(s) {
			dead = append(dead, s)
		}
	}
	return dead
}

// NonDeterministicStates returns states that have multiple transitions on the same input.
func (f *FSM) NonDeterministicStates() []string {
	// Map: state -> input -> count
	transitions := make(map[string]map[string]int)

	for _, t := range f.Transitions {
		if transitions[t.From] == nil {
			transitions[t.From] = make(map[string]int)
		}
		inputKey := ""
		if t.Input != nil {
			inputKey = *t.Input
		} else {
			inputKey = "\x00epsilon" // special key for epsilon
		}
		transitions[t.From][inputKey]++
	}

	var nondet []string
	for state, inputs := range transitions {
		for _, count := range inputs {
			if count > 1 {
				nondet = append(nondet, state)
				break
			}
		}
	}
	return nondet
}

// IncompleteStates returns states that don't have transitions for all inputs.
func (f *FSM) IncompleteStates() []string {
	// Map: state -> set of inputs with transitions
	covered := make(map[string]map[string]bool)

	for _, t := range f.Transitions {
		if t.Input == nil {
			continue // epsilon doesn't count
		}
		if covered[t.From] == nil {
			covered[t.From] = make(map[string]bool)
		}
		covered[t.From][*t.Input] = true
	}

	var incomplete []string
	for _, s := range f.States {
		if len(covered[s]) < len(f.Alphabet) {
			incomplete = append(incomplete, s)
		}
	}
	return incomplete
}

// UnusedInputs returns inputs not used in any transition.
func (f *FSM) UnusedInputs() []string {
	used := make(map[string]bool)
	for _, t := range f.Transitions {
		if t.Input != nil {
			used[*t.Input] = true
		}
	}

	var unused []string
	for _, inp := range f.Alphabet {
		if !used[inp] {
			unused = append(unused, inp)
		}
	}
	return unused
}

// UnusedOutputs returns outputs not used in any state (Moore) or transition (Mealy).
func (f *FSM) UnusedOutputs() []string {
	used := make(map[string]bool)

	if f.Type == TypeMoore {
		for _, out := range f.StateOutputs {
			used[out] = true
		}
	} else if f.Type == TypeMealy {
		for _, t := range f.Transitions {
			if t.Output != nil {
				used[*t.Output] = true
			}
		}
	}

	var unused []string
	for _, out := range f.OutputAlphabet {
		if !used[out] {
			unused = append(unused, out)
		}
	}
	return unused
}
