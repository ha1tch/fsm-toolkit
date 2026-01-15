// Package tests contains integration tests for fsm-toolkit
package tests

import (
	"testing"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// TestUndoSnapshot tests the snapshot deep copy logic
func TestUndoSnapshot(t *testing.T) {
	// Create original FSM
	f := &fsm.FSM{
		Type:           fsm.TypeMoore,
		Name:           "Test",
		States:         []string{"s0", "s1", "s2"},
		Alphabet:       []string{"a", "b"},
		OutputAlphabet: []string{"x", "y"},
		Initial:        "s0",
		Accepting:      []string{"s2"},
		StateOutputs:   map[string]string{"s0": "x", "s1": "y"},
	}
	f.AddTransition("s0", strPtr("a"), []string{"s1"}, nil)
	f.AddTransition("s1", strPtr("b"), []string{"s2"}, nil)

	// Deep copy
	copy := copyFSM(f)

	// Modify original
	f.States = append(f.States, "s3")
	f.Alphabet = append(f.Alphabet, "c")
	f.Accepting = append(f.Accepting, "s3")
	f.StateOutputs["s3"] = "z"
	f.Transitions[0].To = append(f.Transitions[0].To, "s3")

	// Verify copy is unchanged
	if len(copy.States) != 3 {
		t.Errorf("States modified: got %d, want 3", len(copy.States))
	}
	if len(copy.Alphabet) != 2 {
		t.Errorf("Alphabet modified: got %d, want 2", len(copy.Alphabet))
	}
	if len(copy.Accepting) != 1 {
		t.Errorf("Accepting modified: got %d, want 1", len(copy.Accepting))
	}
	if len(copy.StateOutputs) != 2 {
		t.Errorf("StateOutputs modified: got %d, want 2", len(copy.StateOutputs))
	}
	if len(copy.Transitions[0].To) != 1 {
		t.Errorf("Transition To modified: got %d, want 1", len(copy.Transitions[0].To))
	}
}

// TestUndoStackLimit tests the 50-level limit
func TestUndoStackLimit(t *testing.T) {
	stack := make([]Snapshot, 0)
	maxLevels := 50

	// Push 60 snapshots
	for i := 0; i < 60; i++ {
		f := &fsm.FSM{Name: string(rune('A' + i%26))}
		stack = append(stack, Snapshot{FSM: f})
		if len(stack) > maxLevels {
			stack = stack[1:]
		}
	}

	if len(stack) != 50 {
		t.Errorf("Stack limit failed: got %d, want 50", len(stack))
	}

	// First item should be from iteration 10 (60-50)
	if stack[0].FSM.Name != "K" {
		t.Errorf("Stack rotation failed: first item is %s, want K", stack[0].FSM.Name)
	}
}

// TestUndoRedoCycle tests undo followed by redo restores state
func TestUndoRedoCycle(t *testing.T) {
	// Simulate editor state
	undoStack := make([]Snapshot, 0)
	redoStack := make([]Snapshot, 0)

	// Initial state
	current := &fsm.FSM{
		Name:   "Initial",
		States: []string{"s0"},
	}

	// Save snapshot before modification
	undoStack = append(undoStack, Snapshot{FSM: copyFSM(current)})

	// Modify
	current.Name = "Modified"
	current.States = append(current.States, "s1")

	// Undo
	if len(undoStack) > 0 {
		// Save current to redo
		redoStack = append(redoStack, Snapshot{FSM: copyFSM(current)})

		// Pop undo
		snapshot := undoStack[len(undoStack)-1]
		undoStack = undoStack[:len(undoStack)-1]
		current = snapshot.FSM
	}

	if current.Name != "Initial" {
		t.Errorf("Undo failed: name is %s, want Initial", current.Name)
	}
	if len(current.States) != 1 {
		t.Errorf("Undo failed: states count is %d, want 1", len(current.States))
	}

	// Redo
	if len(redoStack) > 0 {
		// Save current to undo
		undoStack = append(undoStack, Snapshot{FSM: copyFSM(current)})

		// Pop redo
		snapshot := redoStack[len(redoStack)-1]
		redoStack = redoStack[:len(redoStack)-1]
		current = snapshot.FSM
	}

	if current.Name != "Modified" {
		t.Errorf("Redo failed: name is %s, want Modified", current.Name)
	}
	if len(current.States) != 2 {
		t.Errorf("Redo failed: states count is %d, want 2", len(current.States))
	}
}

// TestUndoAfterNewActionClearsRedo tests that new actions clear redo stack
func TestUndoAfterNewActionClearsRedo(t *testing.T) {
	undoStack := make([]Snapshot, 0)
	redoStack := make([]Snapshot, 0)

	current := &fsm.FSM{Name: "v1"}

	// Make change 1
	undoStack = append(undoStack, Snapshot{FSM: copyFSM(current)})
	redoStack = nil // Clear redo on new action
	current.Name = "v2"

	// Make change 2
	undoStack = append(undoStack, Snapshot{FSM: copyFSM(current)})
	redoStack = nil
	current.Name = "v3"

	// Undo to v2
	redoStack = append(redoStack, Snapshot{FSM: copyFSM(current)})
	snapshot := undoStack[len(undoStack)-1]
	undoStack = undoStack[:len(undoStack)-1]
	current = snapshot.FSM

	if current.Name != "v2" {
		t.Errorf("First undo failed: got %s, want v2", current.Name)
	}
	if len(redoStack) != 1 {
		t.Errorf("Redo stack wrong size: got %d, want 1", len(redoStack))
	}

	// Make new change (should clear redo)
	undoStack = append(undoStack, Snapshot{FSM: copyFSM(current)})
	redoStack = nil // This is the key behavior
	current.Name = "v4"

	if len(redoStack) != 0 {
		t.Errorf("Redo stack not cleared: got %d, want 0", len(redoStack))
	}
}

// TestUndoWithTransitionPointers tests that transition input/output pointers are deep copied
func TestUndoWithTransitionPointers(t *testing.T) {
	f := &fsm.FSM{
		Type:     fsm.TypeMealy,
		States:   []string{"s0", "s1"},
		Alphabet: []string{"a"},
	}

	inp := "a"
	out := "x"
	f.Transitions = []fsm.Transition{
		{From: "s0", Input: &inp, To: []string{"s1"}, Output: &out},
	}

	copy := copyFSM(f)

	// Modify original pointers
	*f.Transitions[0].Input = "modified"
	*f.Transitions[0].Output = "modified"

	// Copy should be unchanged
	if *copy.Transitions[0].Input != "a" {
		t.Errorf("Input pointer not deep copied: got %s, want a", *copy.Transitions[0].Input)
	}
	if *copy.Transitions[0].Output != "x" {
		t.Errorf("Output pointer not deep copied: got %s, want x", *copy.Transitions[0].Output)
	}
}

// Helper types and functions to mirror fsmedit implementation

type Snapshot struct {
	FSM    *fsm.FSM
	States []StatePos
}

type StatePos struct {
	Name string
	X, Y int
}

func strPtr(s string) *string {
	return &s
}

func copyFSM(f *fsm.FSM) *fsm.FSM {
	fsmCopy := &fsm.FSM{
		Type:           f.Type,
		Name:           f.Name,
		Description:    f.Description,
		States:         make([]string, len(f.States)),
		Alphabet:       make([]string, len(f.Alphabet)),
		OutputAlphabet: make([]string, len(f.OutputAlphabet)),
		Transitions:    make([]fsm.Transition, len(f.Transitions)),
		Initial:        f.Initial,
		Accepting:      make([]string, len(f.Accepting)),
		StateOutputs:   make(map[string]string),
	}
	copy(fsmCopy.States, f.States)
	copy(fsmCopy.Alphabet, f.Alphabet)
	copy(fsmCopy.OutputAlphabet, f.OutputAlphabet)
	copy(fsmCopy.Accepting, f.Accepting)
	for i, t := range f.Transitions {
		fsmCopy.Transitions[i] = fsm.Transition{
			From: t.From,
			To:   make([]string, len(t.To)),
		}
		copy(fsmCopy.Transitions[i].To, t.To)
		if t.Input != nil {
			inp := *t.Input
			fsmCopy.Transitions[i].Input = &inp
		}
		if t.Output != nil {
			out := *t.Output
			fsmCopy.Transitions[i].Output = &out
		}
	}
	for k, v := range f.StateOutputs {
		fsmCopy.StateOutputs[k] = v
	}
	return fsmCopy
}
