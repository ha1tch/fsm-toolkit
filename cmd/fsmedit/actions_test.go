// State/transition editing and analysis tests for fsmedit.
package main

import (
	"testing"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// --- addStateAtPosition ---

func TestAddStateAtPosition(t *testing.T) {
	ed := newTestEditor()
	ed.addStateAtPosition(15, 10)

	if len(ed.fsm.States) != 1 {
		t.Fatalf("expected 1 state, got %d", len(ed.fsm.States))
	}
	if ed.fsm.States[0] != "S0" {
		t.Errorf("expected state name 'S0', got %q", ed.fsm.States[0])
	}
	if len(ed.states) != 1 {
		t.Fatalf("expected 1 state position, got %d", len(ed.states))
	}
	if ed.states[0].X != 15 || ed.states[0].Y != 10 {
		t.Errorf("position: expected (15,10), got (%d,%d)", ed.states[0].X, ed.states[0].Y)
	}
}

func TestAddStateAtPosition_UniqueNames(t *testing.T) {
	ed := newTestEditorWithStates([]string{"S0", "S1"})
	ed.addStateAtPosition(30, 15)

	if len(ed.fsm.States) != 3 {
		t.Fatalf("expected 3 states, got %d", len(ed.fsm.States))
	}
	// S0 and S1 exist, so new state should be S2
	last := ed.fsm.States[2]
	if last != "S2" {
		t.Errorf("expected 'S2', got %q", last)
	}
}

func TestAddStateAtPosition_SetsInitialIfFirst(t *testing.T) {
	ed := newTestEditor()
	ed.addStateAtPosition(10, 10)

	if ed.fsm.Initial != "S0" {
		t.Errorf("first state should be initial, got %q", ed.fsm.Initial)
	}
}

func TestAddStateAtPosition_CreatesUndoSnapshot(t *testing.T) {
	ed := newTestEditor()
	ed.addStateAtPosition(10, 10)

	if len(ed.undoStack) != 1 {
		t.Errorf("expected 1 undo snapshot, got %d", len(ed.undoStack))
	}
}

// --- deleteSelected ---

func TestDeleteSelected_RemovesState(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0", "s1", "s2"})
	ed.fsm.Alphabet = []string{"a"}
	ed.fsm.AddTransition("s0", strPtr("a"), []string{"s1"}, nil)
	ed.fsm.AddTransition("s1", strPtr("a"), []string{"s2"}, nil)

	// Select s1 and delete it
	ed.selectedState = 1
	ed.deleteSelected()

	if len(ed.fsm.States) != 2 {
		t.Errorf("expected 2 states, got %d", len(ed.fsm.States))
	}
	for _, s := range ed.fsm.States {
		if s == "s1" {
			t.Error("s1 should have been removed")
		}
	}
}

func TestDeleteSelected_CleansUpTransitions(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0", "s1", "s2"})
	ed.fsm.Alphabet = []string{"a", "b"}
	ed.fsm.AddTransition("s0", strPtr("a"), []string{"s1"}, nil)
	ed.fsm.AddTransition("s1", strPtr("b"), []string{"s2"}, nil)
	ed.fsm.AddTransition("s2", strPtr("a"), []string{"s1"}, nil)

	// Delete s1 -- should remove transitions from s1 and to s1
	ed.selectedState = 1
	ed.deleteSelected()

	for _, tr := range ed.fsm.Transitions {
		if tr.From == "s1" {
			t.Error("transition FROM s1 should have been removed")
		}
		for _, to := range tr.To {
			if to == "s1" {
				t.Error("transition TO s1 should have been removed")
			}
		}
	}
}

func TestDeleteSelected_CleansUpAccepting(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0", "s1", "s2"})
	ed.fsm.Accepting = []string{"s1", "s2"}

	ed.selectedState = 1
	ed.deleteSelected()

	for _, a := range ed.fsm.Accepting {
		if a == "s1" {
			t.Error("s1 should have been removed from accepting")
		}
	}
	if len(ed.fsm.Accepting) != 1 || ed.fsm.Accepting[0] != "s2" {
		t.Errorf("expected accepting [s2], got %v", ed.fsm.Accepting)
	}
}

func TestDeleteSelected_NothingSelected(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0", "s1"})
	ed.selectedState = -1

	// Should not panic or modify
	ed.deleteSelected()
	if len(ed.fsm.States) != 2 {
		t.Error("states should be unchanged")
	}
}

// --- setInitialState ---

func TestSetInitialState(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0", "s1", "s2"})
	ed.fsm.Initial = "s0"
	ed.selectedState = 2

	ed.setInitialState()

	if ed.fsm.Initial != "s2" {
		t.Errorf("expected initial 's2', got %q", ed.fsm.Initial)
	}
	if !ed.modified {
		t.Error("expected modified flag set")
	}
}

func TestSetInitialState_NothingSelected(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0"})
	ed.fsm.Initial = "s0"
	ed.selectedState = -1

	ed.setInitialState()
	if ed.fsm.Initial != "s0" {
		t.Error("initial should be unchanged")
	}
}

// --- toggleAccepting ---

func TestToggleAccepting_AddAccepting(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0", "s1"})
	ed.fsm.Accepting = nil
	ed.selectedState = 1

	ed.toggleAccepting()

	if len(ed.fsm.Accepting) != 1 || ed.fsm.Accepting[0] != "s1" {
		t.Errorf("expected accepting [s1], got %v", ed.fsm.Accepting)
	}
}

func TestToggleAccepting_RemoveAccepting(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0", "s1"})
	ed.fsm.Accepting = []string{"s1"}
	ed.selectedState = 1

	ed.toggleAccepting()

	if len(ed.fsm.Accepting) != 0 {
		t.Errorf("expected empty accepting, got %v", ed.fsm.Accepting)
	}
}

func TestToggleAccepting_Toggle(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0", "s1"})
	ed.fsm.Accepting = nil
	ed.selectedState = 0

	ed.toggleAccepting()
	if len(ed.fsm.Accepting) != 1 {
		t.Fatalf("expected 1 accepting, got %d", len(ed.fsm.Accepting))
	}

	ed.toggleAccepting()
	if len(ed.fsm.Accepting) != 0 {
		t.Errorf("expected 0 accepting after toggle back, got %d", len(ed.fsm.Accepting))
	}
}

// --- cycleSelection ---

func TestCycleSelection_Wraps(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0", "s1", "s2"})
	ed.selectedState = 0

	ed.cycleSelection()
	if ed.selectedState != 1 {
		t.Errorf("expected 1, got %d", ed.selectedState)
	}

	ed.cycleSelection()
	if ed.selectedState != 2 {
		t.Errorf("expected 2, got %d", ed.selectedState)
	}

	ed.cycleSelection()
	if ed.selectedState != 0 {
		t.Errorf("expected 0 (wrap), got %d", ed.selectedState)
	}
}

func TestCycleSelection_EmptyStates(t *testing.T) {
	ed := newTestEditor()
	ed.selectedState = -1
	// Should not panic
	ed.cycleSelection()
}

// --- findStateAtCursor ---

func TestFindStateAtCursor_Hit(t *testing.T) {
	ed := newTestEditorWithStates([]string{"alpha", "beta"})
	ed.states[0] = StatePos{Name: "alpha", X: 10, Y: 5}
	ed.states[1] = StatePos{Name: "beta", X: 30, Y: 10}

	// Cursor on alpha (name=5 chars, box width = name+4 = 9, so X=10..18, Y=5)
	ed.canvasCursorX = 12
	ed.canvasCursorY = 5
	idx := ed.findStateAtCursor()
	if idx != 0 {
		t.Errorf("expected index 0 for alpha, got %d", idx)
	}

	// Cursor on beta
	ed.canvasCursorX = 32
	ed.canvasCursorY = 10
	idx = ed.findStateAtCursor()
	if idx != 1 {
		t.Errorf("expected index 1 for beta, got %d", idx)
	}
}

func TestFindStateAtCursor_Miss(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0"})
	ed.states[0] = StatePos{Name: "s0", X: 10, Y: 5}

	ed.canvasCursorX = 50
	ed.canvasCursorY = 50
	idx := ed.findStateAtCursor()
	if idx != -1 {
		t.Errorf("expected -1 for miss, got %d", idx)
	}
}

// --- fsmTypeIndex ---

func TestFsmTypeIndex(t *testing.T) {
	tests := []struct {
		fsmType fsm.Type
		want    int
	}{
		{fsm.TypeDFA, 0},
		{fsm.TypeNFA, 1},
		{fsm.TypeMoore, 2},
		{fsm.TypeMealy, 3},
	}

	for _, tt := range tests {
		t.Run(string(tt.fsmType), func(t *testing.T) {
			ed := newTestEditor()
			ed.fsm.Type = tt.fsmType
			got := ed.fsmTypeIndex()
			if got != tt.want {
				t.Errorf("fsmTypeIndex(%s) = %d, want %d", tt.fsmType, got, tt.want)
			}
		})
	}
}

// --- showMessage ---

func TestShowMessage(t *testing.T) {
	ed := newTestEditor()

	ed.showMessage("test error", MsgError)
	if ed.message != "test error" {
		t.Errorf("expected 'test error', got %q", ed.message)
	}
	if ed.messageType != MsgError {
		t.Errorf("expected MsgError, got %d", ed.messageType)
	}

	ed.showMessage("all good", MsgSuccess)
	if ed.message != "all good" {
		t.Errorf("expected 'all good', got %q", ed.message)
	}
}
