// Undo/redo tests for fsmedit (in-package, using real Editor).
package main

import (
	"testing"
)

func TestEditorUndo_Basic(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0", "s1"})
	ed.fsm.Alphabet = []string{"a"}
	ed.fsm.AddTransition("s0", strPtr("a"), []string{"s1"}, nil)

	// Take snapshot, then modify
	ed.saveSnapshot()
	ed.fsm.States = append(ed.fsm.States, "s2")
	ed.states = append(ed.states, StatePos{Name: "s2", X: 40, Y: 10})

	if len(ed.fsm.States) != 3 {
		t.Fatalf("expected 3 states before undo, got %d", len(ed.fsm.States))
	}

	ed.undo()

	if len(ed.fsm.States) != 2 {
		t.Errorf("expected 2 states after undo, got %d", len(ed.fsm.States))
	}
	if len(ed.states) != 2 {
		t.Errorf("expected 2 state positions after undo, got %d", len(ed.states))
	}
}

func TestEditorRedo(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0", "s1"})

	ed.saveSnapshot()
	ed.fsm.States = append(ed.fsm.States, "s2")
	ed.states = append(ed.states, StatePos{Name: "s2", X: 40, Y: 10})

	ed.undo()
	if len(ed.fsm.States) != 2 {
		t.Fatalf("undo failed: %d states", len(ed.fsm.States))
	}

	ed.redo()
	if len(ed.fsm.States) != 3 {
		t.Errorf("expected 3 states after redo, got %d", len(ed.fsm.States))
	}
}

func TestEditorUndo_StackLimit(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0"})

	// Push more than maxUndoLevels snapshots
	for i := 0; i < maxUndoLevels+10; i++ {
		ed.saveSnapshot()
	}

	if len(ed.undoStack) > maxUndoLevels {
		t.Errorf("undo stack should be capped at %d, got %d", maxUndoLevels, len(ed.undoStack))
	}
}

func TestEditorUndo_NewActionClearsRedo(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0", "s1"})

	ed.saveSnapshot()
	ed.fsm.States = append(ed.fsm.States, "s2")

	ed.undo()

	// Now take a new action (should clear redo stack)
	ed.saveSnapshot()
	ed.fsm.States = append(ed.fsm.States, "s3")

	if len(ed.redoStack) != 0 {
		t.Errorf("redo stack should be empty after new action, got %d", len(ed.redoStack))
	}
}

func TestEditorUndo_EmptyStackNoPanic(t *testing.T) {
	ed := newTestEditor()
	// Should not panic
	ed.undo()
	ed.redo()
}

func TestCopyFSM_DeepCopy(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0", "s1", "s2"})
	ed.fsm.Alphabet = []string{"a", "b"}
	ed.fsm.Accepting = []string{"s2"}
	ed.fsm.Type = "moore"
	ed.fsm.StateOutputs = map[string]string{"s0": "x"}
	ed.fsm.AddTransition("s0", strPtr("a"), []string{"s1"}, nil)

	cp := ed.copyFSM()

	// Modify original
	ed.fsm.States = append(ed.fsm.States, "s3")
	ed.fsm.Alphabet = append(ed.fsm.Alphabet, "c")
	ed.fsm.Accepting = append(ed.fsm.Accepting, "s3")
	ed.fsm.StateOutputs["s1"] = "y"
	ed.fsm.Transitions[0].To = append(ed.fsm.Transitions[0].To, "s3")

	// Verify copy is isolated
	if len(cp.States) != 3 {
		t.Errorf("copy states modified: got %d, want 3", len(cp.States))
	}
	if len(cp.Alphabet) != 2 {
		t.Errorf("copy alphabet modified: got %d, want 2", len(cp.Alphabet))
	}
	if len(cp.Accepting) != 1 {
		t.Errorf("copy accepting modified: got %d, want 1", len(cp.Accepting))
	}
	if cp.StateOutputs["s1"] != "" {
		t.Errorf("copy StateOutputs modified: s1=%q", cp.StateOutputs["s1"])
	}
	if len(cp.Transitions[0].To) != 1 {
		t.Errorf("copy transitions modified: got %d targets, want 1", len(cp.Transitions[0].To))
	}
}

func TestEditorUndo_PreservesPositions(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0", "s1"})
	ed.states[0] = StatePos{Name: "s0", X: 10, Y: 20}
	ed.states[1] = StatePos{Name: "s1", X: 30, Y: 40}

	ed.saveSnapshot()

	// Move s0
	ed.states[0] = StatePos{Name: "s0", X: 50, Y: 60}

	ed.undo()

	if ed.states[0].X != 10 || ed.states[0].Y != 20 {
		t.Errorf("s0 position not restored: got (%d,%d), want (10,20)", ed.states[0].X, ed.states[0].Y)
	}
}
