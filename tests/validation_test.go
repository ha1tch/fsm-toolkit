package tests

import (
	"testing"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

func TestUnreachableStates(t *testing.T) {
	f := &fsm.FSM{
		Type:    fsm.TypeDFA,
		States:  []string{"s0", "s1", "s2", "unreachable"},
		Initial: "s0",
	}
	f.AddTransition("s0", strPtr("a"), []string{"s1"}, nil)
	f.AddTransition("s1", strPtr("b"), []string{"s2"}, nil)
	// "unreachable" has no incoming transitions

	unreachable := f.UnreachableStates()
	if len(unreachable) != 1 {
		t.Errorf("Expected 1 unreachable state, got %d", len(unreachable))
	}
	if len(unreachable) > 0 && unreachable[0] != "unreachable" {
		t.Errorf("Expected 'unreachable', got %s", unreachable[0])
	}
}

func TestDeadStates(t *testing.T) {
	f := &fsm.FSM{
		Type:      fsm.TypeDFA,
		States:    []string{"s0", "s1", "dead", "accepting"},
		Initial:   "s0",
		Accepting: []string{"accepting"},
	}
	f.AddTransition("s0", strPtr("a"), []string{"s1"}, nil)
	f.AddTransition("s0", strPtr("b"), []string{"dead"}, nil)
	f.AddTransition("s1", strPtr("c"), []string{"accepting"}, nil)
	// "dead" has no outgoing transitions and is not accepting
	// "accepting" has no outgoing but IS accepting (not dead)

	dead := f.DeadStates()
	if len(dead) != 1 {
		t.Errorf("Expected 1 dead state, got %d: %v", len(dead), dead)
	}
	if len(dead) > 0 && dead[0] != "dead" {
		t.Errorf("Expected 'dead', got %s", dead[0])
	}
}

func TestNonDeterministicStates(t *testing.T) {
	f := &fsm.FSM{
		Type:     fsm.TypeDFA,
		States:   []string{"s0", "s1", "s2"},
		Alphabet: []string{"a"},
		Initial:  "s0",
	}
	// s0 has TWO transitions on input "a" - non-deterministic
	f.AddTransition("s0", strPtr("a"), []string{"s1"}, nil)
	f.AddTransition("s0", strPtr("a"), []string{"s2"}, nil)

	nondet := f.NonDeterministicStates()
	if len(nondet) != 1 {
		t.Errorf("Expected 1 non-deterministic state, got %d", len(nondet))
	}
	if len(nondet) > 0 && nondet[0] != "s0" {
		t.Errorf("Expected 's0', got %s", nondet[0])
	}
}

func TestIncompleteStates(t *testing.T) {
	f := &fsm.FSM{
		Type:     fsm.TypeDFA,
		States:   []string{"s0", "s1"},
		Alphabet: []string{"a", "b", "c"},
		Initial:  "s0",
	}
	// s0 only has transition for "a", missing "b" and "c"
	f.AddTransition("s0", strPtr("a"), []string{"s1"}, nil)
	// s1 has no transitions at all

	incomplete := f.IncompleteStates()
	if len(incomplete) != 2 {
		t.Errorf("Expected 2 incomplete states, got %d", len(incomplete))
	}
}

func TestUnusedInputs(t *testing.T) {
	f := &fsm.FSM{
		Type:     fsm.TypeDFA,
		States:   []string{"s0", "s1"},
		Alphabet: []string{"a", "b", "unused"},
		Initial:  "s0",
	}
	f.AddTransition("s0", strPtr("a"), []string{"s1"}, nil)
	f.AddTransition("s1", strPtr("b"), []string{"s0"}, nil)
	// "unused" is never used

	unused := f.UnusedInputs()
	if len(unused) != 1 {
		t.Errorf("Expected 1 unused input, got %d", len(unused))
	}
	if len(unused) > 0 && unused[0] != "unused" {
		t.Errorf("Expected 'unused', got %s", unused[0])
	}
}

func TestUnusedOutputsMoore(t *testing.T) {
	f := &fsm.FSM{
		Type:           fsm.TypeMoore,
		States:         []string{"s0", "s1"},
		OutputAlphabet: []string{"x", "y", "unused"},
		StateOutputs:   map[string]string{"s0": "x", "s1": "y"},
		Initial:        "s0",
	}

	unused := f.UnusedOutputs()
	if len(unused) != 1 {
		t.Errorf("Expected 1 unused output, got %d", len(unused))
	}
	if len(unused) > 0 && unused[0] != "unused" {
		t.Errorf("Expected 'unused', got %s", unused[0])
	}
}

func TestUnusedOutputsMealy(t *testing.T) {
	f := &fsm.FSM{
		Type:           fsm.TypeMealy,
		States:         []string{"s0", "s1"},
		Alphabet:       []string{"a"},
		OutputAlphabet: []string{"x", "unused"},
		Initial:        "s0",
	}
	out := "x"
	f.Transitions = []fsm.Transition{
		{From: "s0", Input: strPtr("a"), To: []string{"s1"}, Output: &out},
	}

	unused := f.UnusedOutputs()
	if len(unused) != 1 {
		t.Errorf("Expected 1 unused output, got %d", len(unused))
	}
}

func TestAnalyseCleanFSM(t *testing.T) {
	f := &fsm.FSM{
		Type:      fsm.TypeDFA,
		States:    []string{"s0", "s1"},
		Alphabet:  []string{"a", "b"},
		Initial:   "s0",
		Accepting: []string{"s1"},
	}
	// Complete DFA - every state has transition for every input
	f.AddTransition("s0", strPtr("a"), []string{"s1"}, nil)
	f.AddTransition("s0", strPtr("b"), []string{"s0"}, nil)
	f.AddTransition("s1", strPtr("a"), []string{"s1"}, nil)
	f.AddTransition("s1", strPtr("b"), []string{"s0"}, nil)

	warnings := f.Analyse()
	if len(warnings) != 0 {
		t.Errorf("Expected no warnings for clean FSM, got %d: %v", len(warnings), warnings)
	}
}

func TestAnalyseMultipleIssues(t *testing.T) {
	f := &fsm.FSM{
		Type:     fsm.TypeDFA,
		States:   []string{"s0", "s1", "unreachable", "dead"},
		Alphabet: []string{"a", "unused"},
		Initial:  "s0",
	}
	f.AddTransition("s0", strPtr("a"), []string{"s1"}, nil)
	f.AddTransition("s0", strPtr("a"), []string{"dead"}, nil) // non-deterministic!

	warnings := f.Analyse()
	
	// Should find: unreachable, dead, nondeterministic, incomplete, unused_input
	if len(warnings) < 3 {
		t.Errorf("Expected at least 3 warnings, got %d", len(warnings))
	}

	// Check specific warnings exist
	types := make(map[string]bool)
	for _, w := range warnings {
		types[w.Type] = true
	}

	if !types["unreachable"] {
		t.Error("Missing 'unreachable' warning")
	}
	if !types["nondeterministic"] {
		t.Error("Missing 'nondeterministic' warning")
	}
}
