// Test helpers for fsmedit unit tests.
package main

import (
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// newTestEditor creates a minimal Editor suitable for unit tests.
// No screen is initialised -- all screen-dependent code is nil-guarded.
func newTestEditor() *Editor {
	f := fsm.New(fsm.TypeDFA)
	return &Editor{
		fsm:             f,
		states:          make([]StatePos, 0),
		selectedState:   -1,
		selectedTrans:   -1,
		undoStack:        make([]Snapshot, 0),
		redoStack:        make([]Snapshot, 0),
		config:          DefaultConfig(),
		sidebarWidth:    30,
	}
}

// newTestEditorWithStates creates a test Editor with states and positions.
func newTestEditorWithStates(stateNames []string) *Editor {
	ed := newTestEditor()
	for i, name := range stateNames {
		ed.fsm.States = append(ed.fsm.States, name)
		ed.states = append(ed.states, StatePos{
			Name: name,
			X:    5 + i*15,
			Y:    5 + i*4,
		})
	}
	if len(stateNames) > 0 {
		ed.fsm.Initial = stateNames[0]
		ed.selectedState = 0
	}
	return ed
}

// newTestBundle creates a test Editor in bundle mode with N machines.
func newTestBundle(machineNames []string) *Editor {
	ed := newTestEditor()
	ed.isBundle = true
	ed.bundleMachines = make([]string, len(machineNames))
	ed.bundleFSMs = make(map[string]*fsm.FSM)
	ed.bundleStates = make(map[string][]StatePos)
	ed.bundleUndoStack = make(map[string][]Snapshot)
	ed.bundleRedoStack = make(map[string][]Snapshot)
	ed.bundleModified = make(map[string]bool)
	ed.bundleOffsets = make(map[string][2]int)

	for i, name := range machineNames {
		ed.bundleMachines[i] = name
		f := fsm.New(fsm.TypeDFA)
		f.Name = name
		f.States = []string{name + "_s0", name + "_s1"}
		f.Initial = name + "_s0"
		f.Alphabet = []string{"a", "b"}
		f.AddTransition(name+"_s0", strPtr("a"), []string{name + "_s1"}, nil)
		ed.bundleFSMs[name] = f
		ed.bundleStates[name] = []StatePos{
			{Name: name + "_s0", X: 5, Y: 5},
			{Name: name + "_s1", X: 20, Y: 5},
		}
		ed.bundleUndoStack[name] = nil
		ed.bundleRedoStack[name] = nil
		ed.bundleModified[name] = false
		ed.bundleOffsets[name] = [2]int{0, 0}
	}

	if len(machineNames) > 0 {
		first := machineNames[0]
		ed.currentMachine = first
		ed.fsm = ed.bundleFSMs[first]
		ed.states = ed.bundleStates[first]
		ed.selectedState = 0
	}
	return ed
}

func strPtr(s string) *string {
	return &s
}
