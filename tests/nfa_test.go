package tests

import (
	"testing"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// TestNFARunnerMultipleTargets tests that NFA runner handles multiple target states
func TestNFARunnerMultipleTargets(t *testing.T) {
	// NFA that on input "a" from s0 can go to either s1 or s2
	f := &fsm.FSM{
		Type:      fsm.TypeNFA,
		States:    []string{"s0", "s1", "s2"},
		Alphabet:  []string{"a", "b"},
		Initial:   "s0",
		Accepting: []string{"s2"},
	}
	f.Transitions = []fsm.Transition{
		{From: "s0", Input: strPtr("a"), To: []string{"s1", "s2"}}, // non-deterministic!
	}

	runner, err := fsm.NewRunner(f)
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	// Initially in s0
	states := runner.CurrentStates()
	if len(states) != 1 || states[0] != "s0" {
		t.Errorf("Expected initial state [s0], got %v", states)
	}

	// After "a", should be in BOTH s1 and s2
	_, err = runner.Step("a")
	if err != nil {
		t.Fatalf("Step failed: %v", err)
	}

	states = runner.CurrentStates()
	if len(states) != 2 {
		t.Errorf("Expected 2 states after NFA step, got %d: %v", len(states), states)
	}

	// Should be accepting (s2 is accepting)
	if !runner.IsAccepting() {
		t.Error("Expected accepting state (s2 is in current states)")
	}
}

// TestNFAEpsilonClosure tests epsilon transitions
func TestNFAEpsilonClosure(t *testing.T) {
	// NFA with epsilon transition: s0 --ε--> s1
	f := &fsm.FSM{
		Type:      fsm.TypeNFA,
		States:    []string{"s0", "s1", "s2"},
		Alphabet:  []string{"a"},
		Initial:   "s0",
		Accepting: []string{"s2"},
	}
	f.Transitions = []fsm.Transition{
		{From: "s0", Input: nil, To: []string{"s1"}},              // epsilon
		{From: "s1", Input: strPtr("a"), To: []string{"s2"}},
	}

	runner, err := fsm.NewRunner(f)
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	// Initial state should include s0 AND s1 (epsilon closure)
	states := runner.CurrentStates()
	if len(states) != 2 {
		t.Errorf("Expected 2 initial states (epsilon closure), got %v", states)
	}

	// Should be able to accept "a" because we're also in s1
	_, err = runner.Step("a")
	if err != nil {
		t.Errorf("Expected 'a' to be valid (via epsilon closure): %v", err)
	}

	if !runner.IsAccepting() {
		t.Error("Expected accepting state after 'a'")
	}
}

// TestNFAToDFA tests conversion of NFA to DFA
func TestNFAToDFA(t *testing.T) {
	// NFA: s0 --a--> {s1, s2}, s1 --b--> s3, s2 --b--> s3
	f := &fsm.FSM{
		Type:      fsm.TypeNFA,
		States:    []string{"s0", "s1", "s2", "s3"},
		Alphabet:  []string{"a", "b"},
		Initial:   "s0",
		Accepting: []string{"s3"},
	}
	f.Transitions = []fsm.Transition{
		{From: "s0", Input: strPtr("a"), To: []string{"s1", "s2"}},
		{From: "s1", Input: strPtr("b"), To: []string{"s3"}},
		{From: "s2", Input: strPtr("b"), To: []string{"s3"}},
	}

	dfa := f.ToDFA()

	if dfa.Type != fsm.TypeDFA {
		t.Errorf("Expected DFA type, got %s", dfa.Type)
	}

	// DFA should have states: "s0", "s1,s2", "s3"
	if len(dfa.States) != 3 {
		t.Errorf("Expected 3 DFA states, got %d: %v", len(dfa.States), dfa.States)
	}

	// Run through DFA
	runner, err := fsm.NewRunner(dfa)
	if err != nil {
		t.Fatalf("Failed to create DFA runner: %v", err)
	}

	// a, b should reach accepting state
	runner.Step("a")
	runner.Step("b")

	if !runner.IsAccepting() {
		t.Error("Expected DFA to accept 'ab'")
	}
}

// TestNFAToDFAWithEpsilon tests NFA-to-DFA with epsilon transitions
func TestNFAToDFAWithEpsilon(t *testing.T) {
	// NFA: s0 --ε--> s1, s1 --a--> s2 (accepting)
	f := &fsm.FSM{
		Type:      fsm.TypeNFA,
		States:    []string{"s0", "s1", "s2"},
		Alphabet:  []string{"a"},
		Initial:   "s0",
		Accepting: []string{"s2"},
	}
	f.Transitions = []fsm.Transition{
		{From: "s0", Input: nil, To: []string{"s1"}}, // epsilon
		{From: "s1", Input: strPtr("a"), To: []string{"s2"}},
	}

	dfa := f.ToDFA()

	// Initial DFA state should be epsilon closure of s0, which is {s0, s1}
	if dfa.Initial != "s0,s1" {
		t.Errorf("Expected initial state 's0,s1', got '%s'", dfa.Initial)
	}

	runner, _ := fsm.NewRunner(dfa)
	runner.Step("a")

	if !runner.IsAccepting() {
		t.Error("Expected DFA to accept 'a'")
	}
}

// TestDFAToDFANoop tests that ToDFA on a DFA just returns a copy
func TestDFAToDFANoop(t *testing.T) {
	f := &fsm.FSM{
		Type:     fsm.TypeDFA,
		States:   []string{"s0", "s1"},
		Alphabet: []string{"a"},
		Initial:  "s0",
	}
	f.AddTransition("s0", strPtr("a"), []string{"s1"}, nil)

	dfa := f.ToDFA()

	if dfa.Type != fsm.TypeDFA {
		t.Error("Expected DFA type")
	}
	if len(dfa.States) != 2 {
		t.Errorf("Expected 2 states, got %d", len(dfa.States))
	}
}

// TestNFARunnerAvailableInputs tests that available inputs come from all current states
func TestNFARunnerAvailableInputs(t *testing.T) {
	f := &fsm.FSM{
		Type:     fsm.TypeNFA,
		States:   []string{"s0", "s1", "s2"},
		Alphabet: []string{"a", "b", "c"},
		Initial:  "s0",
	}
	f.Transitions = []fsm.Transition{
		{From: "s0", Input: strPtr("a"), To: []string{"s1", "s2"}},
		{From: "s1", Input: strPtr("b"), To: []string{"s0"}},
		{From: "s2", Input: strPtr("c"), To: []string{"s0"}},
	}

	runner, _ := fsm.NewRunner(f)
	runner.Step("a") // Now in {s1, s2}

	inputs := runner.AvailableInputs()
	if len(inputs) != 2 {
		t.Errorf("Expected 2 available inputs (b from s1, c from s2), got %v", inputs)
	}
}
