package fsm

import (
	"fmt"
	"strings"
)

// MaxDelegationDepth is the maximum nesting depth for linked state delegation.
const MaxDelegationDepth = 16

// BundleRunner executes an FSM with support for linked states that delegate to other machines.
type BundleRunner struct {
	machines      map[string]*FSM    // All machines in the bundle
	runners       map[string]*Runner // Active runners for each machine
	mainMachine   string             // Name of the main/root machine
	activeRunner  *Runner            // Currently active runner
	activeMachine string             // Name of currently active machine
	
	// Delegation stack
	delegationStack []delegationFrame
	
	// History
	history []BundleStep
}

// delegationFrame tracks a single level of delegation.
type delegationFrame struct {
	machineName string
	stateName   string // The linked state that caused delegation
	runner      *Runner
}

// BundleStep records one step of bundle execution.
type BundleStep struct {
	Machine   string // Which machine processed this step
	FromState string
	Input     string
	ToState   string
	Output    string
	Delegated bool   // True if this step triggered delegation
	Returned  bool   // True if this step is a return from delegation
	Result    string // "accept" or "reject" if Returned
}

// NewBundleRunner creates a runner for a bundle of machines.
// mainMachine is the name of the machine to start execution from.
func NewBundleRunner(machines map[string]*FSM, mainMachine string) (*BundleRunner, error) {
	if len(machines) == 0 {
		return nil, fmt.Errorf("no machines provided")
	}
	
	main, ok := machines[mainMachine]
	if !ok {
		return nil, fmt.Errorf("main machine %q not found", mainMachine)
	}
	
	// Validate all machines
	for name, m := range machines {
		if err := m.Validate(); err != nil {
			return nil, fmt.Errorf("machine %q invalid: %w", name, err)
		}
	}
	
	// Create runner for main machine
	mainRunner, err := NewRunner(main)
	if err != nil {
		return nil, fmt.Errorf("failed to create runner for %q: %w", mainMachine, err)
	}
	
	br := &BundleRunner{
		machines:        machines,
		runners:         make(map[string]*Runner),
		mainMachine:     mainMachine,
		activeRunner:    mainRunner,
		activeMachine:   mainMachine,
		delegationStack: make([]delegationFrame, 0),
		history:         make([]BundleStep, 0),
	}
	
	br.runners[mainMachine] = mainRunner
	
	return br, nil
}

// CurrentMachine returns the name of the currently active machine.
func (br *BundleRunner) CurrentMachine() string {
	return br.activeMachine
}

// CurrentState returns the current state in the active machine.
func (br *BundleRunner) CurrentState() string {
	return br.activeRunner.CurrentState()
}

// DelegationDepth returns the current nesting depth.
func (br *BundleRunner) DelegationDepth() int {
	return len(br.delegationStack)
}

// IsInDelegation returns true if currently executing a delegated machine.
func (br *BundleRunner) IsInDelegation() bool {
	return len(br.delegationStack) > 0
}

// IsAccepting returns true if the current state is accepting.
func (br *BundleRunner) IsAccepting() bool {
	return br.activeRunner.IsAccepting()
}

// IsTerminal returns true if the active machine has no valid transitions.
func (br *BundleRunner) IsTerminal() bool {
	return len(br.activeRunner.AvailableInputs()) == 0
}

// AvailableInputs returns inputs valid from the current state.
func (br *BundleRunner) AvailableInputs() []string {
	return br.activeRunner.AvailableInputs()
}

// Step processes an input. If we enter a linked state, it delegates.
// Returns output and any error.
func (br *BundleRunner) Step(input string) (output string, err error) {
	// Check if we're at maximum depth
	if len(br.delegationStack) >= MaxDelegationDepth {
		return "", fmt.Errorf("maximum delegation depth (%d) exceeded", MaxDelegationDepth)
	}
	
	currentMachine := br.machines[br.activeMachine]
	currentState := br.activeRunner.CurrentState()
	
	// If currently in a linked state (at start of step), delegate first
	if currentMachine.IsLinked(currentState) && !br.IsInDelegation() {
		targetMachine := currentMachine.GetLinkedMachine(currentState)
		if targetMachine == "" {
			return "", fmt.Errorf("state %q is linked but has no target machine", currentState)
		}
		
		// Delegate to target machine
		err := br.delegate(currentState, targetMachine)
		if err != nil {
			return "", err
		}
		
		// Update current machine reference after delegation
		currentMachine = br.machines[br.activeMachine]
		
		// Record delegation step
		br.history = append(br.history, BundleStep{
			Machine:   br.activeMachine,
			FromState: currentState,
			Input:     "",
			ToState:   br.activeRunner.CurrentState(),
			Delegated: true,
		})
	}
	
	// Process input in active machine
	fromState := br.activeRunner.CurrentState()
	output, err = br.activeRunner.Step(input)
	if err != nil {
		return "", err
	}
	toState := br.activeRunner.CurrentState()
	
	// Record step
	br.history = append(br.history, BundleStep{
		Machine:   br.activeMachine,
		FromState: fromState,
		Input:     input,
		ToState:   toState,
		Output:    output,
	})
	
	// Check if delegated machine has terminated
	if br.IsInDelegation() && br.IsTerminal() {
		result := "reject"
		if br.IsAccepting() {
			result = "accept"
		}
		
		// Return from delegation
		err := br.returnFromDelegation(result)
		if err != nil {
			return output, err
		}
	}
	
	// Check if we've entered a linked state (after transition in parent)
	if !br.IsInDelegation() {
		newState := br.activeRunner.CurrentState()
		newMachine := br.machines[br.activeMachine]
		if newMachine.IsLinked(newState) {
			targetMachine := newMachine.GetLinkedMachine(newState)
			if targetMachine != "" {
				// Delegate to target machine
				err := br.delegate(newState, targetMachine)
				if err != nil {
					return output, err
				}
				
				// Record delegation step
				br.history = append(br.history, BundleStep{
					Machine:   br.activeMachine,
					FromState: newState,
					Input:     "",
					ToState:   br.activeRunner.CurrentState(),
					Delegated: true,
				})
			}
		}
	}
	
	return output, nil
}

// delegate pushes current context and switches to target machine.
func (br *BundleRunner) delegate(linkedState, targetMachine string) error {
	target, ok := br.machines[targetMachine]
	if !ok {
		return fmt.Errorf("target machine %q not found", targetMachine)
	}
	
	// Push current context
	br.delegationStack = append(br.delegationStack, delegationFrame{
		machineName: br.activeMachine,
		stateName:   linkedState,
		runner:      br.activeRunner,
	})
	
	// Create or reset runner for target
	runner, ok := br.runners[targetMachine]
	if !ok {
		var err error
		runner, err = NewRunner(target)
		if err != nil {
			// Pop the frame we just pushed
			br.delegationStack = br.delegationStack[:len(br.delegationStack)-1]
			return fmt.Errorf("failed to create runner for %q: %w", targetMachine, err)
		}
		br.runners[targetMachine] = runner
	} else {
		runner.Reset()
	}
	
	br.activeRunner = runner
	br.activeMachine = targetMachine
	
	return nil
}

// returnFromDelegation pops delegation stack and sends result to parent.
func (br *BundleRunner) returnFromDelegation(result string) error {
	if len(br.delegationStack) == 0 {
		return fmt.Errorf("not in delegation")
	}
	
	// Pop frame
	frame := br.delegationStack[len(br.delegationStack)-1]
	br.delegationStack = br.delegationStack[:len(br.delegationStack)-1]
	
	// Restore parent context
	br.activeRunner = frame.runner
	br.activeMachine = frame.machineName
	
	// Record return step
	br.history = append(br.history, BundleStep{
		Machine:   br.activeMachine,
		FromState: frame.stateName,
		Returned:  true,
		Result:    result,
	})
	
	// Process result as input to parent
	// The parent should have transitions on "accept" or "reject"
	fromState := br.activeRunner.CurrentState()
	_, err := br.activeRunner.Step(result)
	if err != nil {
		// No transition for result - this is an error in the FSM design
		return fmt.Errorf("parent machine %q has no transition from %q on %q",
			br.activeMachine, fromState, result)
	}
	
	// Record the result transition
	br.history = append(br.history, BundleStep{
		Machine:   br.activeMachine,
		FromState: fromState,
		Input:     result,
		ToState:   br.activeRunner.CurrentState(),
	})
	
	return nil
}

// Reset returns all machines to initial state.
func (br *BundleRunner) Reset() {
	// Reset main runner
	mainRunner := br.runners[br.mainMachine]
	mainRunner.Reset()
	
	// Clear delegation stack
	br.delegationStack = make([]delegationFrame, 0)
	
	// Set main as active
	br.activeRunner = mainRunner
	br.activeMachine = br.mainMachine
	
	// Clear history
	br.history = make([]BundleStep, 0)
}

// History returns the execution history.
func (br *BundleRunner) History() []BundleStep {
	return br.history
}

// Status returns a status string.
func (br *BundleRunner) Status() string {
	var sb strings.Builder
	
	if br.IsInDelegation() {
		sb.WriteString(fmt.Sprintf("[%s] ", br.activeMachine))
	}
	
	sb.WriteString(fmt.Sprintf("State: %s", br.CurrentState()))
	
	if br.IsAccepting() {
		sb.WriteString(" [accepting]")
	}
	
	if br.IsTerminal() {
		sb.WriteString(" [terminal]")
	}
	
	if depth := br.DelegationDepth(); depth > 0 {
		sb.WriteString(fmt.Sprintf(" (depth: %d)", depth))
	}
	
	return sb.String()
}

// Prompt returns the appropriate prompt string for interactive mode.
func (br *BundleRunner) Prompt() string {
	if br.IsInDelegation() {
		// Show nested prompts for delegation depth
		prefix := strings.Repeat(">", br.DelegationDepth())
		return fmt.Sprintf("%s %s> ", prefix, br.activeMachine)
	}
	return "> "
}
