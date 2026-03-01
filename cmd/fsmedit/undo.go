// Undo/redo snapshot management for fsmedit.
package main

import (
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)


// Undo/Redo operations

const maxUndoLevels = 50

// saveSnapshot saves current state for undo
func (ed *Editor) saveSnapshot() {
	// Deep copy FSM
	fsmCopy := &fsm.FSM{
		Type:           ed.fsm.Type,
		Name:           ed.fsm.Name,
		Description:    ed.fsm.Description,
		States:         make([]string, len(ed.fsm.States)),
		Alphabet:       make([]string, len(ed.fsm.Alphabet)),
		OutputAlphabet: make([]string, len(ed.fsm.OutputAlphabet)),
		Transitions:    make([]fsm.Transition, len(ed.fsm.Transitions)),
		Initial:        ed.fsm.Initial,
		Accepting:      make([]string, len(ed.fsm.Accepting)),
		StateOutputs:   make(map[string]string),
		LinkedMachines: make(map[string]string),
	}
	copy(fsmCopy.States, ed.fsm.States)
	copy(fsmCopy.Alphabet, ed.fsm.Alphabet)
	copy(fsmCopy.OutputAlphabet, ed.fsm.OutputAlphabet)
	copy(fsmCopy.Accepting, ed.fsm.Accepting)
	for i, t := range ed.fsm.Transitions {
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
	for k, v := range ed.fsm.StateOutputs {
		fsmCopy.StateOutputs[k] = v
	}
	for k, v := range ed.fsm.LinkedMachines {
		fsmCopy.LinkedMachines[k] = v
	}

	// Copy state positions
	statesCopy := make([]StatePos, len(ed.states))
	copy(statesCopy, ed.states)

	snapshot := Snapshot{
		FSM:    fsmCopy,
		States: statesCopy,
	}

	ed.undoStack = append(ed.undoStack, snapshot)
	if len(ed.undoStack) > maxUndoLevels {
		ed.undoStack = ed.undoStack[1:]
	}

	// Clear redo stack on new action
	ed.redoStack = nil
}

func (ed *Editor) undo() {
	if len(ed.undoStack) == 0 {
		ed.showMessage("Nothing to undo", MsgInfo)
		return
	}

	// Save current state to redo stack
	ed.saveToRedo()

	// Pop from undo stack
	snapshot := ed.undoStack[len(ed.undoStack)-1]
	ed.undoStack = ed.undoStack[:len(ed.undoStack)-1]

	// Restore
	ed.fsm = snapshot.FSM
	ed.states = snapshot.States
	ed.modified = true
	ed.selectedState = -1

	ed.showMessage("Undo", MsgInfo)
}

func (ed *Editor) redo() {
	if len(ed.redoStack) == 0 {
		ed.showMessage("Nothing to redo", MsgInfo)
		return
	}

	// Save current state to undo stack (without clearing redo)
	ed.saveToUndo()

	// Pop from redo stack
	snapshot := ed.redoStack[len(ed.redoStack)-1]
	ed.redoStack = ed.redoStack[:len(ed.redoStack)-1]

	// Restore
	ed.fsm = snapshot.FSM
	ed.states = snapshot.States
	ed.modified = true
	ed.selectedState = -1

	ed.showMessage("Redo", MsgInfo)
}

func (ed *Editor) saveToUndo() {
	fsmCopy := ed.copyFSM()
	statesCopy := make([]StatePos, len(ed.states))
	copy(statesCopy, ed.states)
	ed.undoStack = append(ed.undoStack, Snapshot{FSM: fsmCopy, States: statesCopy})
}

func (ed *Editor) saveToRedo() {
	fsmCopy := ed.copyFSM()
	statesCopy := make([]StatePos, len(ed.states))
	copy(statesCopy, ed.states)
	ed.redoStack = append(ed.redoStack, Snapshot{FSM: fsmCopy, States: statesCopy})
}

func (ed *Editor) copyFSM() *fsm.FSM {
	fsmCopy := &fsm.FSM{
		Type:           ed.fsm.Type,
		Name:           ed.fsm.Name,
		Description:    ed.fsm.Description,
		States:         make([]string, len(ed.fsm.States)),
		Alphabet:       make([]string, len(ed.fsm.Alphabet)),
		OutputAlphabet: make([]string, len(ed.fsm.OutputAlphabet)),
		Transitions:    make([]fsm.Transition, len(ed.fsm.Transitions)),
		Initial:        ed.fsm.Initial,
		Accepting:      make([]string, len(ed.fsm.Accepting)),
		StateOutputs:   make(map[string]string),
	}
	copy(fsmCopy.States, ed.fsm.States)
	copy(fsmCopy.Alphabet, ed.fsm.Alphabet)
	copy(fsmCopy.OutputAlphabet, ed.fsm.OutputAlphabet)
	copy(fsmCopy.Accepting, ed.fsm.Accepting)
	for i, t := range ed.fsm.Transitions {
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
	for k, v := range ed.fsm.StateOutputs {
		fsmCopy.StateOutputs[k] = v
	}
	return fsmCopy
}
