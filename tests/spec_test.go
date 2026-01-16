// Package spec_test contains property-based tests that verify the implementation
// matches the claims made in SPECIFICATION.md.
//
// Each test is tagged with the specification section it validates.
package tests

import (
	"strings"
	"testing"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
	"github.com/ha1tch/fsm-toolkit/pkg/fsmfile"
)

// =============================================================================
// SPECIFICATION: FSM Types - DFA Guarantees
// =============================================================================

// TestSpec_DFA_NoEpsilon verifies:
// "No epsilon (null) transitions" - MUST level requirement
func TestSpec_DFA_NoEpsilon(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.AddState("s0")
	f.AddState("s1")
	f.SetInitial("s0")
	f.Alphabet = []string{"a"}
	
	// Add epsilon transition (nil input)
	f.AddTransition("s0", nil, []string{"s1"}, nil)
	
	err := f.Validate()
	if err == nil {
		t.Error("SPEC VIOLATION: DFA with epsilon transition should fail validation")
	}
	if err != nil && !strings.Contains(err.Error(), "epsilon") {
		t.Errorf("SPEC VIOLATION: Error should mention epsilon, got: %v", err)
	}
}

// TestSpec_DFA_NondeterministicWarning verifies:
// "At most one transition per (state, input)" - SHOULD level (warning, not error)
func TestSpec_DFA_NondeterministicWarning(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.AddState("s0")
	f.AddState("s1")
	f.AddState("s2")
	f.SetInitial("s0")
	f.Alphabet = []string{"a"}
	
	// Add two transitions on same input - nondeterministic
	input := "a"
	f.AddTransition("s0", &input, []string{"s1"}, nil)
	f.AddTransition("s0", &input, []string{"s2"}, nil)
	
	// Should pass validation (SHOULD, not MUST)
	err := f.Validate()
	if err != nil {
		t.Errorf("SPEC VIOLATION: Nondeterministic DFA should pass validation, got: %v", err)
	}
	
	// Should produce warning
	warnings := f.Analyse()
	found := false
	for _, w := range warnings {
		if w.Type == "nondeterministic" {
			found = true
			break
		}
	}
	if !found {
		t.Error("SPEC VIOLATION: Nondeterministic DFA should produce 'nondeterministic' warning")
	}
}

// TestSpec_DFA_IncompleteWarning verifies:
// "Transition for every (state, input)" - SHOULD level (warning, not error)
func TestSpec_DFA_IncompleteWarning(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.AddState("s0")
	f.AddState("s1")
	f.SetInitial("s0")
	f.Alphabet = []string{"a", "b"}
	
	// Only add transition for "a", not "b"
	input := "a"
	f.AddTransition("s0", &input, []string{"s1"}, nil)
	
	// Should pass validation
	err := f.Validate()
	if err != nil {
		t.Errorf("SPEC VIOLATION: Incomplete DFA should pass validation, got: %v", err)
	}
	
	// Should produce warning
	warnings := f.Analyse()
	found := false
	for _, w := range warnings {
		if w.Type == "incomplete" {
			found = true
			break
		}
	}
	if !found {
		t.Error("SPEC VIOLATION: Incomplete DFA should produce 'incomplete' warning")
	}
}

// =============================================================================
// SPECIFICATION: FSM Types - NFA Guarantees
// =============================================================================

// TestSpec_NFA_EpsilonAllowed verifies:
// "Epsilon transitions allowed" - MAY level
func TestSpec_NFA_EpsilonAllowed(t *testing.T) {
	f := fsm.New(fsm.TypeNFA)
	f.AddState("s0")
	f.AddState("s1")
	f.SetInitial("s0")
	f.Alphabet = []string{"a"}
	
	// Add epsilon transition
	f.AddTransition("s0", nil, []string{"s1"}, nil)
	
	err := f.Validate()
	if err != nil {
		t.Errorf("SPEC VIOLATION: NFA with epsilon transition should pass validation, got: %v", err)
	}
}

// TestSpec_NFA_MultipleTargetsAllowed verifies:
// "Multiple target states per transition allowed" - MAY level
func TestSpec_NFA_MultipleTargetsAllowed(t *testing.T) {
	f := fsm.New(fsm.TypeNFA)
	f.AddState("s0")
	f.AddState("s1")
	f.AddState("s2")
	f.SetInitial("s0")
	f.Alphabet = []string{"a"}
	
	// Single transition with multiple targets
	input := "a"
	f.AddTransition("s0", &input, []string{"s1", "s2"}, nil)
	
	err := f.Validate()
	if err != nil {
		t.Errorf("SPEC VIOLATION: NFA with multiple targets should pass validation, got: %v", err)
	}
}

// =============================================================================
// SPECIFICATION: FSM Types - Moore Guarantees
// =============================================================================

// TestSpec_Moore_OutputOptional verifies:
// "State outputs MAY be defined" - MAY level
func TestSpec_Moore_OutputOptional(t *testing.T) {
	f := fsm.New(fsm.TypeMoore)
	f.AddState("s0")
	f.AddState("s1")
	f.SetInitial("s0")
	f.Alphabet = []string{"a"}
	
	input := "a"
	f.AddTransition("s0", &input, []string{"s1"}, nil)
	
	// No state outputs defined - should be valid
	err := f.Validate()
	if err != nil {
		t.Errorf("SPEC VIOLATION: Moore without state outputs should pass validation, got: %v", err)
	}
}

// TestSpec_Moore_OutputAlphabetEnforced verifies:
// "If OutputAlphabet is defined, state outputs MUST be in it" - MUST level
func TestSpec_Moore_OutputAlphabetEnforced(t *testing.T) {
	f := fsm.New(fsm.TypeMoore)
	f.AddState("s0")
	f.SetInitial("s0")
	f.Alphabet = []string{}
	f.OutputAlphabet = []string{"valid_output"}
	f.SetStateOutput("s0", "invalid_output")
	
	err := f.Validate()
	if err == nil {
		t.Error("SPEC VIOLATION: Moore with output not in OutputAlphabet should fail validation")
	}
}

// TestSpec_Moore_MissingOutputReturnsEmpty verifies:
// "States without defined outputs produce empty string" - default behaviour
func TestSpec_Moore_MissingOutputReturnsEmpty(t *testing.T) {
	f := fsm.New(fsm.TypeMoore)
	f.AddState("s0")
	f.AddState("s1")
	f.SetInitial("s0")
	f.Alphabet = []string{"a"}
	f.SetStateOutput("s0", "output0")
	// s1 has no output defined
	
	input := "a"
	f.AddTransition("s0", &input, []string{"s1"}, nil)
	
	runner, err := fsm.NewRunner(f)
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}
	
	_, err = runner.Step("a")
	if err != nil {
		t.Fatalf("Step failed: %v", err)
	}
	
	output := runner.CurrentOutput()
	if output != "" {
		t.Errorf("SPEC VIOLATION: Missing Moore output should return empty string, got: %q", output)
	}
}

// =============================================================================
// SPECIFICATION: FSM Types - Mealy Guarantees
// =============================================================================

// TestSpec_Mealy_OutputAlphabetEnforced verifies:
// "If OutputAlphabet is defined, transition outputs MUST be in it" - MUST level
func TestSpec_Mealy_OutputAlphabetEnforced(t *testing.T) {
	f := fsm.New(fsm.TypeMealy)
	f.AddState("s0")
	f.AddState("s1")
	f.SetInitial("s0")
	f.Alphabet = []string{"a"}
	f.OutputAlphabet = []string{"valid"}
	
	input := "a"
	output := "invalid"
	f.AddTransition("s0", &input, []string{"s1"}, &output)
	
	err := f.Validate()
	if err == nil {
		t.Error("SPEC VIOLATION: Mealy with output not in OutputAlphabet should fail validation")
	}
}

// =============================================================================
// SPECIFICATION: Validation Rules
// =============================================================================

// TestSpec_V001_NoStates verifies error V001
func TestSpec_V001_NoStates(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	// No states added
	
	err := f.Validate()
	if err == nil {
		t.Error("SPEC VIOLATION V001: FSM with no states should fail validation")
	}
}

// TestSpec_V002_NoInitial verifies error V002
func TestSpec_V002_NoInitial(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.AddState("s0")
	// No initial state set
	
	err := f.Validate()
	if err == nil {
		t.Error("SPEC VIOLATION V002: FSM with no initial state should fail validation")
	}
}

// TestSpec_V003_InitialNotInStates verifies error V003
func TestSpec_V003_InitialNotInStates(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.AddState("s0")
	f.SetInitial("nonexistent")
	
	err := f.Validate()
	if err == nil {
		t.Error("SPEC VIOLATION V003: FSM with initial not in states should fail validation")
	}
}

// TestSpec_V004_AcceptingNotInStates verifies error V004
func TestSpec_V004_AcceptingNotInStates(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.AddState("s0")
	f.SetInitial("s0")
	f.SetAccepting([]string{"nonexistent"})
	
	err := f.Validate()
	if err == nil {
		t.Error("SPEC VIOLATION V004: FSM with accepting not in states should fail validation")
	}
}

// TestSpec_V005_TransitionFromUndefined verifies error V005
func TestSpec_V005_TransitionFromUndefined(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.AddState("s0")
	f.SetInitial("s0")
	f.Alphabet = []string{"a"}
	
	input := "a"
	f.Transitions = append(f.Transitions, fsm.Transition{
		From:  "nonexistent",
		Input: &input,
		To:    []string{"s0"},
	})
	
	err := f.Validate()
	if err == nil {
		t.Error("SPEC VIOLATION V005: Transition from undefined state should fail validation")
	}
}

// TestSpec_V006_TransitionToUndefined verifies error V006
func TestSpec_V006_TransitionToUndefined(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.AddState("s0")
	f.SetInitial("s0")
	f.Alphabet = []string{"a"}
	
	input := "a"
	f.AddTransition("s0", &input, []string{"nonexistent"}, nil)
	
	err := f.Validate()
	if err == nil {
		t.Error("SPEC VIOLATION V006: Transition to undefined state should fail validation")
	}
}

// TestSpec_V007_InputNotInAlphabet verifies error V007
func TestSpec_V007_InputNotInAlphabet(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.AddState("s0")
	f.AddState("s1")
	f.SetInitial("s0")
	f.Alphabet = []string{"a"}
	
	input := "b" // not in alphabet
	f.AddTransition("s0", &input, []string{"s1"}, nil)
	
	err := f.Validate()
	if err == nil {
		t.Error("SPEC VIOLATION V007: Transition with input not in alphabet should fail validation")
	}
}

// =============================================================================
// SPECIFICATION: Analysis Rules
// =============================================================================

// TestSpec_A001_UnreachableStates verifies warning A001
func TestSpec_A001_UnreachableStates(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.AddState("s0")
	f.AddState("s1")
	f.AddState("orphan") // unreachable
	f.SetInitial("s0")
	f.Alphabet = []string{"a"}
	
	input := "a"
	f.AddTransition("s0", &input, []string{"s1"}, nil)
	// orphan has no incoming transitions
	
	warnings := f.Analyse()
	found := false
	for _, w := range warnings {
		if w.Type == "unreachable" {
			for _, s := range w.States {
				if s == "orphan" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("SPEC VIOLATION A001: Unreachable state should produce warning")
	}
}

// TestSpec_A002_DeadStates verifies warning A002
func TestSpec_A002_DeadStates(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.AddState("s0")
	f.AddState("trap") // no outgoing, not accepting
	f.SetInitial("s0")
	f.Alphabet = []string{"a"}
	
	input := "a"
	f.AddTransition("s0", &input, []string{"trap"}, nil)
	// trap has no outgoing transitions and is not accepting
	
	warnings := f.Analyse()
	found := false
	for _, w := range warnings {
		if w.Type == "dead" {
			for _, s := range w.States {
				if s == "trap" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("SPEC VIOLATION A002: Dead state should produce warning")
	}
}

// TestSpec_A002_AcceptingNotDead verifies accepting states are not flagged as dead
func TestSpec_A002_AcceptingNotDead(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.AddState("s0")
	f.AddState("final") // no outgoing, but accepting
	f.SetInitial("s0")
	f.SetAccepting([]string{"final"})
	f.Alphabet = []string{"a"}
	
	input := "a"
	f.AddTransition("s0", &input, []string{"final"}, nil)
	
	warnings := f.Analyse()
	for _, w := range warnings {
		if w.Type == "dead" {
			for _, s := range w.States {
				if s == "final" {
					t.Error("SPEC VIOLATION: Accepting state without outgoing should NOT be flagged as dead")
				}
			}
		}
	}
}

// TestSpec_A005_UnusedInput verifies warning A005
func TestSpec_A005_UnusedInput(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.AddState("s0")
	f.AddState("s1")
	f.SetInitial("s0")
	f.Alphabet = []string{"a", "b"} // b is unused
	
	input := "a"
	f.AddTransition("s0", &input, []string{"s1"}, nil)
	
	warnings := f.Analyse()
	found := false
	for _, w := range warnings {
		if w.Type == "unused_input" {
			for _, s := range w.Symbols {
				if s == "b" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("SPEC VIOLATION A005: Unused input should produce warning with symbol name")
	}
}

// TestSpec_A006_UnusedOutput verifies warning A006
func TestSpec_A006_UnusedOutput(t *testing.T) {
	f := fsm.New(fsm.TypeMoore)
	f.AddState("s0")
	f.SetInitial("s0")
	f.Alphabet = []string{}
	f.OutputAlphabet = []string{"used", "unused"}
	f.SetStateOutput("s0", "used")
	
	warnings := f.Analyse()
	found := false
	for _, w := range warnings {
		if w.Type == "unused_output" {
			for _, s := range w.Symbols {
				if s == "unused" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("SPEC VIOLATION A006: Unused output should produce warning with symbol name")
	}
}

// =============================================================================
// SPECIFICATION: Runtime Semantics
// =============================================================================

// TestSpec_Runtime_EpsilonClosureOnInit verifies:
// "Set current state(s) to epsilon closure of initial state"
func TestSpec_Runtime_EpsilonClosureOnInit(t *testing.T) {
	f := fsm.New(fsm.TypeNFA)
	f.AddState("s0")
	f.AddState("s1")
	f.AddState("s2")
	f.SetInitial("s0")
	f.Alphabet = []string{"a"}
	
	// s0 --ε--> s1 --ε--> s2
	f.AddTransition("s0", nil, []string{"s1"}, nil)
	f.AddTransition("s1", nil, []string{"s2"}, nil)
	
	runner, err := fsm.NewRunner(f)
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}
	
	states := runner.CurrentStates()
	if len(states) != 3 {
		t.Errorf("SPEC VIOLATION: Epsilon closure on init should include all reachable states, got: %v", states)
	}
}

// TestSpec_Runtime_AcceptingAny verifies:
// "Returns true if ANY current state is an accepting state"
func TestSpec_Runtime_AcceptingAny(t *testing.T) {
	f := fsm.New(fsm.TypeNFA)
	f.AddState("s0")
	f.AddState("s1")
	f.AddState("s2")
	f.SetInitial("s0")
	f.SetAccepting([]string{"s2"})
	f.Alphabet = []string{"a"}
	
	// s0 --a--> s1, s2 (one accepting, one not)
	input := "a"
	f.AddTransition("s0", &input, []string{"s1", "s2"}, nil)
	
	runner, err := fsm.NewRunner(f)
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}
	
	runner.Step("a")
	
	if !runner.IsAccepting() {
		t.Error("SPEC VIOLATION: IsAccepting should return true if ANY current state is accepting")
	}
}

// TestSpec_Runtime_AvailableInputsFromAny verifies:
// "Returns inputs that have at least one transition from ANY current state"
func TestSpec_Runtime_AvailableInputsFromAny(t *testing.T) {
	f := fsm.New(fsm.TypeNFA)
	f.AddState("s0")
	f.AddState("s1")
	f.AddState("s2")
	f.SetInitial("s0")
	f.Alphabet = []string{"a", "b", "c"}
	
	// s0 --ε--> s1 (so we start in both s0 and s1)
	// s0 has transition on "a"
	// s1 has transition on "b"
	// neither has transition on "c"
	f.AddTransition("s0", nil, []string{"s1"}, nil)
	
	inputA := "a"
	inputB := "b"
	f.AddTransition("s0", &inputA, []string{"s2"}, nil)
	f.AddTransition("s1", &inputB, []string{"s2"}, nil)
	
	runner, err := fsm.NewRunner(f)
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}
	
	available := runner.AvailableInputs()
	hasA := false
	hasB := false
	hasC := false
	for _, inp := range available {
		switch inp {
		case "a":
			hasA = true
		case "b":
			hasB = true
		case "c":
			hasC = true
		}
	}
	
	if !hasA || !hasB {
		t.Errorf("SPEC VIOLATION: AvailableInputs should include inputs from ANY current state, got: %v", available)
	}
	if hasC {
		t.Errorf("SPEC VIOLATION: AvailableInputs should not include inputs with no transitions, got: %v", available)
	}
}

// =============================================================================
// SPECIFICATION: Correctness Model - Validity Hierarchy
// =============================================================================

// TestSpec_ValidImpliesRunnable verifies:
// "If Validate() returns no error... The FSM can be executed via NewRunner()"
func TestSpec_ValidImpliesRunnable(t *testing.T) {
	testCases := []string{
		`{"type":"dfa","states":["s0"],"alphabet":["a"],"initial":"s0","transitions":[{"from":"s0","to":"s0","input":"a"}]}`,
		`{"type":"nfa","states":["s0","s1"],"alphabet":["a"],"initial":"s0","transitions":[{"from":"s0","to":["s0","s1"],"input":"a"}]}`,
		`{"type":"moore","states":["s0"],"alphabet":[],"initial":"s0","state_outputs":{"s0":"x"}}`,
		`{"type":"mealy","states":["s0"],"alphabet":["a"],"initial":"s0","transitions":[{"from":"s0","to":"s0","input":"a","output":"x"}]}`,
	}
	
	for _, tc := range testCases {
		f, err := fsmfile.ParseJSON([]byte(tc))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		
		if f.Validate() == nil {
			// Valid FSM must be runnable
			runner, err := fsm.NewRunner(f)
			if err != nil {
				t.Errorf("SPEC VIOLATION: Valid FSM should be runnable, got error: %v\nFSM: %s", err, tc)
			}
			if runner == nil {
				t.Errorf("SPEC VIOLATION: Valid FSM should produce non-nil runner\nFSM: %s", tc)
			}
		}
	}
}

// =============================================================================
// SPECIFICATION: Alphabet Enforcement
// =============================================================================

// TestSpec_OutputAlphabetOptional verifies:
// "If OutputAlphabet is empty or nil: no enforcement"
func TestSpec_OutputAlphabetOptional(t *testing.T) {
	f := fsm.New(fsm.TypeMealy)
	f.AddState("s0")
	f.SetInitial("s0")
	f.Alphabet = []string{"a"}
	// OutputAlphabet not set (nil)
	
	input := "a"
	output := "any_output_is_fine"
	f.AddTransition("s0", &input, []string{"s0"}, &output)
	
	err := f.Validate()
	if err != nil {
		t.Errorf("SPEC VIOLATION: Output without alphabet should not require validation, got: %v", err)
	}
}

// =============================================================================
// SPECIFICATION: Edge Cases
// =============================================================================

// TestSpec_SingleStateValid verifies:
// "A single-state DFA with self-loop is a valid accepting-all or rejecting-all machine"
func TestSpec_SingleStateValid(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.AddState("s0")
	f.SetInitial("s0")
	f.SetAccepting([]string{"s0"})
	f.Alphabet = []string{"a"}
	
	input := "a"
	f.AddTransition("s0", &input, []string{"s0"}, nil)
	
	err := f.Validate()
	if err != nil {
		t.Errorf("SPEC VIOLATION: Single-state DFA with self-loop should be valid, got: %v", err)
	}
	
	runner, _ := fsm.NewRunner(f)
	runner.Step("a")
	runner.Step("a")
	runner.Step("a")
	
	if !runner.IsAccepting() {
		t.Error("SPEC VIOLATION: Single accepting state with self-loop should accept all inputs")
	}
}

// TestSpec_SelfLoopValid verifies:
// "Self-loops (transitions where from == to) are valid for all types"
func TestSpec_SelfLoopValid(t *testing.T) {
	types := []fsm.Type{fsm.TypeDFA, fsm.TypeNFA, fsm.TypeMoore, fsm.TypeMealy}
	
	for _, fsmType := range types {
		f := fsm.New(fsmType)
		f.AddState("s0")
		f.SetInitial("s0")
		f.Alphabet = []string{"a"}
		
		input := "a"
		f.AddTransition("s0", &input, []string{"s0"}, nil)
		
		err := f.Validate()
		if err != nil {
			t.Errorf("SPEC VIOLATION: Self-loop should be valid for %s, got: %v", fsmType, err)
		}
	}
}

// TestSpec_DisconnectedValid verifies:
// "States not reachable from initial... Are valid (pass validation)"
func TestSpec_DisconnectedValid(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.AddState("s0")
	f.AddState("orphan")
	f.SetInitial("s0")
	f.Alphabet = []string{}
	
	err := f.Validate()
	if err != nil {
		t.Errorf("SPEC VIOLATION: Disconnected states should pass validation, got: %v", err)
	}
}
