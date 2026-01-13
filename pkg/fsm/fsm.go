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
