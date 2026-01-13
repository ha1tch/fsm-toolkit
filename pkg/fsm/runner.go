package fsm

import (
	"fmt"
)

// Runner executes an FSM interactively.
type Runner struct {
	fsm          *FSM
	currentState string
	history      []Step
}

// Step records one step of execution.
type Step struct {
	FromState string
	Input     string
	ToState   string
	Output    string // For Mealy/Moore
}

// NewRunner creates a runner for the given FSM.
func NewRunner(f *FSM) (*Runner, error) {
	if err := f.Validate(); err != nil {
		return nil, fmt.Errorf("invalid FSM: %w", err)
	}
	
	return &Runner{
		fsm:          f,
		currentState: f.Initial,
		history:      make([]Step, 0),
	}, nil
}

// CurrentState returns the current state.
func (r *Runner) CurrentState() string {
	return r.currentState
}

// CurrentOutput returns the current output (Moore machine).
func (r *Runner) CurrentOutput() string {
	if r.fsm.Type != TypeMoore {
		return ""
	}
	return r.fsm.StateOutputs[r.currentState]
}

// IsAccepting returns true if the current state is accepting.
func (r *Runner) IsAccepting() bool {
	return r.fsm.IsAccepting(r.currentState)
}

// AvailableInputs returns the inputs valid from the current state.
func (r *Runner) AvailableInputs() []string {
	seen := make(map[string]bool)
	var inputs []string
	
	for _, t := range r.fsm.Transitions {
		if t.From == r.currentState && t.Input != nil {
			if !seen[*t.Input] {
				seen[*t.Input] = true
				inputs = append(inputs, *t.Input)
			}
		}
	}
	
	return inputs
}

// Step processes an input and returns the output (if any).
// Returns an error if no valid transition exists.
func (r *Runner) Step(input string) (output string, err error) {
	transitions := r.fsm.GetTransitions(r.currentState, &input)
	
	if len(transitions) == 0 {
		return "", fmt.Errorf("no transition from state %q on input %q", r.currentState, input)
	}
	
	// For DFA/Moore/Mealy, take the first (should be only) transition
	// For NFA, this is a simplification - would need proper NFA simulation
	t := transitions[0]
	
	fromState := r.currentState
	toState := t.To[0] // Take first target for simplicity
	
	// Determine output
	switch r.fsm.Type {
	case TypeMealy:
		if t.Output != nil {
			output = *t.Output
		}
	case TypeMoore:
		output = r.fsm.StateOutputs[toState]
	}
	
	// Record step
	r.history = append(r.history, Step{
		FromState: fromState,
		Input:     input,
		ToState:   toState,
		Output:    output,
	})
	
	// Update state
	r.currentState = toState
	
	return output, nil
}

// Reset returns the runner to the initial state.
func (r *Runner) Reset() {
	r.currentState = r.fsm.Initial
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
	status := fmt.Sprintf("State: %s", r.currentState)
	
	if r.IsAccepting() {
		status += " [accepting]"
	}
	
	if r.fsm.Type == TypeMoore {
		if out := r.CurrentOutput(); out != "" {
			status += fmt.Sprintf(" -> %s", out)
		}
	}
	
	return status
}
