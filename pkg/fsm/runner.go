package fsm

import (
	"fmt"
	"sort"
	"strings"
)

// Runner executes an FSM interactively.
// For NFAs, it tracks all possible current states simultaneously.
type Runner struct {
	fsm           *FSM
	currentStates map[string]bool // Set of current states (for NFA)
	history       []Step
}

// Step records one step of execution.
type Step struct {
	FromState  string   // For DFA: single state. For NFA: comma-separated states
	FromStates []string // For NFA: all source states
	Input      string
	ToState    string   // For DFA: single state. For NFA: comma-separated states
	ToStates   []string // For NFA: all target states
	Output     string   // For Mealy/Moore
}

// NewRunner creates a runner for the given FSM.
func NewRunner(f *FSM) (*Runner, error) {
	if err := f.Validate(); err != nil {
		return nil, fmt.Errorf("invalid FSM: %w", err)
	}

	r := &Runner{
		fsm:           f,
		currentStates: make(map[string]bool),
		history:       make([]Step, 0),
	}

	// Start with initial state and its epsilon closure
	r.currentStates[f.Initial] = true
	if f.Type == TypeNFA {
		r.currentStates = r.epsilonClosure(r.currentStates)
	}

	return r, nil
}

// epsilonClosure computes the epsilon closure of a set of states.
// Returns all states reachable via epsilon (nil input) transitions.
func (r *Runner) epsilonClosure(states map[string]bool) map[string]bool {
	closure := make(map[string]bool)
	for s := range states {
		closure[s] = true
	}

	changed := true
	for changed {
		changed = false
		for state := range closure {
			for _, t := range r.fsm.GetEpsilonTransitions(state) {
				for _, to := range t.To {
					if !closure[to] {
						closure[to] = true
						changed = true
					}
				}
			}
		}
	}

	return closure
}

// CurrentState returns the current state(s) as a string.
// For NFA, returns comma-separated list of states.
func (r *Runner) CurrentState() string {
	states := r.CurrentStates()
	if len(states) == 1 {
		return states[0]
	}
	return "{" + strings.Join(states, ", ") + "}"
}

// CurrentStates returns the current states as a sorted slice.
func (r *Runner) CurrentStates() []string {
	var states []string
	for s := range r.currentStates {
		states = append(states, s)
	}
	sort.Strings(states)
	return states
}

// CurrentOutput returns the current output (Moore machine or NFA with state outputs).
// For NFA/Moore with multiple states, returns outputs from all states.
func (r *Runner) CurrentOutput() string {
	// Only Moore machines and NFAs can have state outputs
	if r.fsm.Type != TypeMoore && r.fsm.Type != TypeNFA {
		return ""
	}
	
	// Check if there are any state outputs defined
	if len(r.fsm.StateOutputs) == 0 {
		return ""
	}

	var outputs []string
	seen := make(map[string]bool)
	for state := range r.currentStates {
		if out, ok := r.fsm.StateOutputs[state]; ok && !seen[out] {
			seen[out] = true
			outputs = append(outputs, out)
		}
	}

	if len(outputs) == 1 {
		return outputs[0]
	}
	sort.Strings(outputs)
	return strings.Join(outputs, ", ")
}

// IsAccepting returns true if any current state is accepting.
func (r *Runner) IsAccepting() bool {
	for state := range r.currentStates {
		if r.fsm.IsAccepting(state) {
			return true
		}
	}
	return false
}

// AvailableInputs returns the inputs valid from any current state.
func (r *Runner) AvailableInputs() []string {
	seen := make(map[string]bool)
	var inputs []string

	for state := range r.currentStates {
		for _, t := range r.fsm.Transitions {
			if t.From == state && t.Input != nil {
				if !seen[*t.Input] {
					seen[*t.Input] = true
					inputs = append(inputs, *t.Input)
				}
			}
		}
	}

	sort.Strings(inputs)
	return inputs
}

// Step processes an input and returns the output (if any).
// For NFA, explores all possible transitions simultaneously.
// Returns an error if no valid transition exists from any current state.
func (r *Runner) Step(input string) (output string, err error) {
	fromStates := r.CurrentStates()

	// Collect all target states from all current states
	nextStates := make(map[string]bool)
	var outputs []string
	seenOutputs := make(map[string]bool)

	for state := range r.currentStates {
		transitions := r.fsm.GetTransitions(state, &input)
		for _, t := range transitions {
			for _, to := range t.To {
				nextStates[to] = true

				// Collect Mealy outputs
				if r.fsm.Type == TypeMealy && t.Output != nil {
					if !seenOutputs[*t.Output] {
						seenOutputs[*t.Output] = true
						outputs = append(outputs, *t.Output)
					}
				}
			}
		}
	}

	if len(nextStates) == 0 {
		return "", fmt.Errorf("no transition from state %s on input %q", r.CurrentState(), input)
	}

	// Apply epsilon closure for NFA
	if r.fsm.Type == TypeNFA {
		nextStates = r.epsilonClosure(nextStates)
	}

	// Collect Moore outputs from target states
	if r.fsm.Type == TypeMoore {
		for state := range nextStates {
			if out, ok := r.fsm.StateOutputs[state]; ok {
				if !seenOutputs[out] {
					seenOutputs[out] = true
					outputs = append(outputs, out)
				}
			}
		}
	}

	// Format output
	sort.Strings(outputs)
	if len(outputs) == 1 {
		output = outputs[0]
	} else if len(outputs) > 1 {
		output = strings.Join(outputs, ", ")
	}

	// Update state
	r.currentStates = nextStates
	toStates := r.CurrentStates()

	// Record step
	r.history = append(r.history, Step{
		FromState:  formatStateSet(fromStates),
		FromStates: fromStates,
		Input:      input,
		ToState:    formatStateSet(toStates),
		ToStates:   toStates,
		Output:     output,
	})

	return output, nil
}

// formatStateSet formats a slice of states as a string.
func formatStateSet(states []string) string {
	if len(states) == 1 {
		return states[0]
	}
	return "{" + strings.Join(states, ", ") + "}"
}

// Reset returns the runner to the initial state.
func (r *Runner) Reset() {
	r.currentStates = make(map[string]bool)
	r.currentStates[r.fsm.Initial] = true
	if r.fsm.Type == TypeNFA {
		r.currentStates = r.epsilonClosure(r.currentStates)
	}
	r.history = make([]Step, 0)
}

// History returns the execution history.
func (r *Runner) History() []Step {
	return r.history
}

// Run processes a sequence of inputs and returns all outputs.
func (r *Runner) Run(inputs []string) ([]string, error) {
	var outputs []string

	for _, input := range inputs {
		output, err := r.Step(input)
		if err != nil {
			return outputs, err
		}
		outputs = append(outputs, output)
	}

	return outputs, nil
}

// RunString processes a sequence of single-character inputs.
func (r *Runner) RunString(input string) ([]string, error) {
	var inputs []string
	for _, c := range input {
		inputs = append(inputs, string(c))
	}
	return r.Run(inputs)
}

// Status returns a status string for the current state.
func (r *Runner) Status() string {
	status := fmt.Sprintf("State: %s", r.CurrentState())

	if r.IsAccepting() {
		status += " [accepting]"
	}

	// Show outputs for Moore machines and NFAs with state outputs
	if r.fsm.Type == TypeMoore || (r.fsm.Type == TypeNFA && len(r.fsm.StateOutputs) > 0) {
		if out := r.CurrentOutput(); out != "" {
			status += fmt.Sprintf(" -> %s", out)
		}
	}

	return status
}
