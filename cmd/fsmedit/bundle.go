// Bundle management, import, promotion, and navigation for fsmedit.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
	"github.com/ha1tch/fsm-toolkit/pkg/fsmfile"
)


// promoteToBundle promotes the current single-FSM session to a bundle.
// The current machine is stored under the given name. After this call,
// isBundle is true and all bundle caches are initialised.
func (ed *Editor) promoteToBundle(machineName string) {
	ed.isBundle = true
	ed.promotedFromSingle = true
	ed.originalFilename = ed.filename
	ed.currentMachine = machineName
	ed.bundleMachines = []string{machineName}
	ed.bundleFSMs = map[string]*fsm.FSM{machineName: ed.fsm}
	ed.bundleStates = map[string][]StatePos{machineName: ed.states}
	ed.bundleUndoStack = map[string][]Snapshot{machineName: ed.undoStack}
	ed.bundleRedoStack = map[string][]Snapshot{machineName: ed.redoStack}
	ed.bundleModified = map[string]bool{machineName: ed.modified}
	ed.bundleOffsets = map[string][2]int{machineName: {ed.canvasOffsetX, ed.canvasOffsetY}}
	ed.navStack = nil
	ed.updateMenuItems()
}

// promoteIfNeeded ensures we're in bundle mode, prompting for a machine name
// if currently in single-document mode. Calls continuation on success.
func (ed *Editor) promoteIfNeeded(continuation func()) {
	if ed.isBundle {
		continuation()
		return
	}
	// Need to promote: prompt for a name for the current machine
	defaultName := "main"
	if ed.filename != "" {
		base := filepath.Base(ed.filename)
		defaultName = strings.TrimSuffix(base, filepath.Ext(base))
	} else if ed.fsm.Name != "" {
		defaultName = ed.fsm.Name
	}
	ed.inputPrompt = "Name for current machine: "
	ed.inputBuffer = defaultName
	ed.inputAction = func(name string) {
		if name == "" {
			ed.showMessage("Cancelled", MsgInfo)
			ed.mode = ModeMenu
			return
		}
		ed.promoteToBundle(name)
		ed.showMessage("Promoted to bundle", MsgSuccess)
		continuation()
	}
	ed.mode = ModeInput
}

// importFilePicker opens the file picker in import mode.
func (ed *Editor) importFilePicker() {
	ed.importMode = true
	ed.currentDir = ed.config.LastDir
	if ed.currentDir == "" {
		ed.currentDir, _ = os.Getwd()
	}
	ed.refreshFilePicker()
	ed.filePickerFocus = 1
	ed.mode = ModeFilePicker
}

// handleImportFile is called when a file is selected in import mode.
func (ed *Editor) handleImportFile(path string) {
	ed.importMode = false
	
	ext := filepath.Ext(path)
	switch ext {
	case ".fsm":
		// Check if it's a bundle
		machines, err := fsmfile.ListMachines(path)
		if err != nil {
			ed.showMessage("Error reading file: "+err.Error(), MsgError)
			ed.mode = ModeMenu
			return
		}
		if len(machines) > 1 {
			// Bundle: show multi-select picker
			ed.importMachines = make([]string, len(machines))
			ed.importSelected = make([]bool, len(machines))
			ed.importCursor = 0
			for i, m := range machines {
				ed.importMachines[i] = m.Name
			}
			// Store the source path for the import
			ed.importSourcePath = path
			ed.mode = ModeImportMachineSelect
			return
		}
		// Single FSM file: import directly
		f, layout, err := fsmfile.ReadFSMFileWithLayout(path)
		if err != nil {
			ed.showMessage("Error: "+err.Error(), MsgError)
			ed.mode = ModeMenu
			return
		}
		baseName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		ed.importSingleMachine(baseName, f, layout)
	case ".json":
		data, err := os.ReadFile(path)
		if err != nil {
			ed.showMessage("Error: "+err.Error(), MsgError)
			ed.mode = ModeMenu
			return
		}
		f, err := fsmfile.ParseJSON(data)
		if err != nil {
			ed.showMessage("Error: "+err.Error(), MsgError)
			ed.mode = ModeMenu
			return
		}
		baseName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		ed.importSingleMachine(baseName, f, nil)
	default:
		ed.showMessage("Unsupported file type", MsgError)
		ed.mode = ModeMenu
	}
}

// importSingleMachine imports one FSM, promoting to bundle if needed.
func (ed *Editor) importSingleMachine(name string, f *fsm.FSM, layout *fsmfile.Layout) {
	ed.promoteIfNeeded(func() {
		if !ed.machineNameExists(name) {
			// No collision, import directly
			ed.addMachineToBundle(name, f, layout)
			ed.showMessage("Imported: "+name, MsgSuccess)
			ed.mode = ModeCanvas
			return
		}
		// Name collision -- prompt for new name
		ed.inputPrompt = fmt.Sprintf("%q already exists. New name: ", name)
		ed.inputBuffer = name + "_2"
		ed.inputAction = func(newName string) {
			if newName == "" {
				ed.showMessage("Import cancelled", MsgInfo)
				ed.mode = ModeCanvas
				return
			}
			if ed.machineNameExists(newName) {
				ed.showMessage(newName+" also exists, import cancelled", MsgError)
				ed.mode = ModeCanvas
				return
			}
			ed.addMachineToBundle(newName, f, layout)
			ed.showMessage("Imported: "+newName, MsgSuccess)
			ed.mode = ModeCanvas
		}
		ed.mode = ModeInput
	})
}

// executeImport imports the selected machines from the import picker.
func (ed *Editor) executeImport() {
	// Collect selected machine names
	var selected []string
	for i, name := range ed.importMachines {
		if ed.importSelected[i] {
			selected = append(selected, name)
		}
	}
	if len(selected) == 0 {
		ed.showMessage("No machines selected", MsgInfo)
		ed.mode = ModeMenu
		return
	}
	
	sourcePath := ed.importSourcePath
	
	ed.promoteIfNeeded(func() {
		imported := 0
		for _, name := range selected {
			f, layout, err := fsmfile.ReadMachineFromBundle(sourcePath, name)
			if err != nil {
				ed.showMessage("Error importing "+name+": "+err.Error(), MsgError)
				continue
			}
			finalName := ed.resolveImportNameAuto(name)
			ed.addMachineToBundle(finalName, f, layoutToFsmfileLayout(layout))
			imported++
		}
		if imported > 0 {
			ed.showMessage(fmt.Sprintf("Imported %d machine(s)", imported), MsgSuccess)
		}
		ed.mode = ModeCanvas
	})
}

// resolveImportNameAuto resolves name collisions automatically by appending _2, _3, etc.
// Used during bulk import to avoid prompting for each machine.
func (ed *Editor) resolveImportNameAuto(proposed string) string {
	if !ed.machineNameExists(proposed) {
		return proposed
	}
	for i := 2; i < 100; i++ {
		candidate := fmt.Sprintf("%s_%d", proposed, i)
		if !ed.machineNameExists(candidate) {
			return candidate
		}
	}
	return proposed + "_import"
}

// machineNameExists checks if a machine name is already in the bundle.
func (ed *Editor) machineNameExists(name string) bool {
	for _, m := range ed.bundleMachines {
		if m == name {
			return true
		}
	}
	return false
}

// addMachineToBundle adds a loaded FSM to the bundle caches.
func (ed *Editor) addMachineToBundle(name string, f *fsm.FSM, layout *fsmfile.Layout) {
	ed.bundleMachines = append(ed.bundleMachines, name)
	ed.bundleFSMs[name] = f
	
	// Generate state positions from layout or auto-layout
	states := make([]StatePos, len(f.States))
	if layout != nil && len(layout.States) > 0 {
		for i, sName := range f.States {
			if sl, ok := layout.States[sName]; ok {
				states[i] = StatePos{Name: sName, X: sl.X, Y: sl.Y}
			} else {
				col := i % 5
				row := i / 5
				states[i] = StatePos{Name: sName, X: 5 + col*15, Y: 2 + row*4}
			}
		}
		if layout.Editor.CanvasOffsetX != 0 || layout.Editor.CanvasOffsetY != 0 {
			ed.bundleOffsets[name] = [2]int{layout.Editor.CanvasOffsetX, layout.Editor.CanvasOffsetY}
		}
	} else {
		w, h := 80, 24
		if ed.screen != nil {
			w, h = ed.screen.Size()
			w = w - ed.sidebarWidth - 5
			h = h - 4
		}
		autoPositions := fsmfile.SmartLayoutTUI(f, w, h)
		for i, sName := range f.States {
			if pos, ok := autoPositions[sName]; ok {
				states[i] = StatePos{Name: sName, X: pos[0], Y: pos[1]}
			} else {
				col := i % 5
				row := i / 5
				states[i] = StatePos{Name: sName, X: 5 + col*15, Y: 2 + row*4}
			}
		}
	}
	
	ed.bundleStates[name] = states
	ed.bundleUndoStack[name] = nil
	ed.bundleRedoStack[name] = nil
	ed.bundleModified[name] = true
	ed.updateMenuItems()
}

// layoutToFsmfileLayout converts a *fsmfile.Layout to itself (type assertion helper).
// ReadMachineFromBundle already returns *fsmfile.Layout, so this is a passthrough.
func layoutToFsmfileLayout(layout *fsmfile.Layout) *fsmfile.Layout {
	return layout
}

// newMachine creates a new empty FSM in the bundle (or promotes first).
// addMachine adds a new machine to the project. If in single-FSM mode,
// automatically promotes to a bundle first.
func (ed *Editor) addMachine() {
	if !ed.isBundle {
		ed.promoteIfNeeded(func() {
			ed.newMachinePrompt()
		})
		return
	}
	ed.newMachinePrompt()
}

func (ed *Editor) newMachinePrompt() {
	ed.inputPrompt = "New machine name: "
	ed.inputBuffer = ""
	ed.inputAction = func(name string) {
		if name == "" {
			ed.mode = ModeMenu
			return
		}
		if ed.machineNameExists(name) {
			ed.showMessage("Machine already exists: "+name, MsgError)
			ed.mode = ModeMenu
			return
		}
		// Prompt for FSM type
		ed.pendingNewMachineName = name
		ed.typeMenuSelected = 0
		ed.mode = ModeSelectType
		// Override the type selector action
		ed.newMachineTypeSelect = true
	}
	ed.mode = ModeInput
}

// loadMachineFromBundle loads a specific machine from the current bundle
func (ed *Editor) loadMachineFromBundle(machineName string) error {
	// Check cache first — if both FSM and states are cached, use them.
	// An empty states slice is valid (newly created machines have no states).
	if cachedFSM, ok := ed.bundleFSMs[machineName]; ok {
		if cachedStates, ok := ed.bundleStates[machineName]; ok {
			ed.fsm = cachedFSM
			ed.currentMachine = machineName
			ed.modified = ed.bundleModified[machineName]
			ed.states = cachedStates
			ed.undoStack = ed.bundleUndoStack[machineName]
			ed.redoStack = ed.bundleRedoStack[machineName]
			if offsets, ok := ed.bundleOffsets[machineName]; ok {
				ed.canvasOffsetX = offsets[0]
				ed.canvasOffsetY = offsets[1]
			} else {
				ed.canvasOffsetX = 0
				ed.canvasOffsetY = 0
			}
			ed.selectedState = -1
			ed.mode = ModeCanvas
			return nil
		}
	}
	
	// Load from file
	f, layout, err := fsmfile.ReadMachineFromBundle(ed.filename, machineName)
	if err != nil {
		return err
	}

	ed.fsm = f
	ed.modified = false
	ed.currentMachine = machineName

	// Apply layout if present, otherwise generate default positions
	ed.states = make([]StatePos, len(f.States))

	if layout != nil && len(layout.States) > 0 {
		ed.canvasOffsetX = layout.Editor.CanvasOffsetX
		ed.canvasOffsetY = layout.Editor.CanvasOffsetY

		for i, name := range f.States {
			if sl, ok := layout.States[name]; ok {
				ed.states[i] = StatePos{
					Name: name,
					X:    sl.X,
					Y:    sl.Y,
				}
			} else {
				col := i % 5
				row := i / 5
				ed.states[i] = StatePos{
					Name: name,
					X:    5 + col*15,
					Y:    2 + row*4,
				}
			}
		}
	} else {
		// Generate smart layout
		w, h := 80, 24
		if ed.screen != nil {
			w, h = ed.screen.Size()
			w = w - ed.sidebarWidth - 5
			h = h - 4
		}

		autoPositions := fsmfile.SmartLayoutTUI(f, w, h)
		for i, name := range f.States {
			if pos, ok := autoPositions[name]; ok {
				ed.states[i] = StatePos{
					Name: name,
					X:    pos[0],
					Y:    pos[1],
				}
			} else {
				col := i % 5
				row := i / 5
				ed.states[i] = StatePos{
					Name: name,
					X:    5 + col*15,
					Y:    2 + row*4,
				}
			}
		}
	}

	// Save to cache
	ed.saveMachineToCache()
	
	ed.selectedState = -1
	ed.mode = ModeCanvas
	return nil
}

// diveIntoLinkedState navigates into a linked state's target machine
func (ed *Editor) diveIntoLinkedState(stateIdx int) {
	if stateIdx < 0 || stateIdx >= len(ed.states) {
		return
	}
	
	stateName := ed.states[stateIdx].Name
	if !ed.fsm.IsLinked(stateName) {
		ed.showMessage("State is not linked", MsgError)
		return
	}
	
	targetMachine := ed.fsm.GetLinkedMachine(stateName)
	if targetMachine == "" {
		ed.showMessage("Linked state has no target machine", MsgError)
		return
	}
	
	// Check target exists in bundle
	if ed.bundleFSMs == nil {
		ed.showMessage("Not in a bundle", MsgError)
		return
	}
	
	if _, ok := ed.bundleFSMs[targetMachine]; !ok {
		ed.showMessage("Target machine not found: "+targetMachine, MsgError)
		return
	}
	
	// Save current machine state to bundle cache
	ed.saveMachineToCache()
	
	// Push navigation frame
	frame := NavFrame{
		MachineName:   ed.currentMachine,
		LinkedState:   stateName,
		LinkedStateX:  ed.states[stateIdx].X,
		LinkedStateY:  ed.states[stateIdx].Y,
		CanvasOffsetX: ed.canvasOffsetX,
		CanvasOffsetY: ed.canvasOffsetY,
		SelectedState: ed.selectedState,
	}
	ed.navStack = append(ed.navStack, frame)
	
	// Start zoom-in animation
	ed.animating = true
	ed.animStartTime = time.Now().UnixMilli()
	ed.animDuration = 120 // milliseconds
	ed.animZoomIn = true
	ed.animCenterX = ed.states[stateIdx].X
	ed.animCenterY = ed.states[stateIdx].Y
	ed.animTargetMachine = targetMachine
}

// navigateBack returns to the parent machine
func (ed *Editor) navigateBack() {
	if len(ed.navStack) == 0 {
		ed.showMessage("Already at root", MsgInfo)
		return
	}
	
	// Save current machine state
	ed.saveMachineToCache()
	
	// Pop navigation frame
	frame := ed.navStack[len(ed.navStack)-1]
	ed.navStack = ed.navStack[:len(ed.navStack)-1]
	
	// Start zoom-out animation
	ed.animating = true
	ed.animStartTime = time.Now().UnixMilli()
	ed.animDuration = 120
	ed.animZoomIn = false
	ed.animCenterX = frame.LinkedStateX
	ed.animCenterY = frame.LinkedStateY
	ed.animTargetMachine = frame.MachineName
}

// navigateToBreadcrumb jumps to a specific level in the navigation stack
func (ed *Editor) navigateToBreadcrumb(level int) {
	// level 0 = root, level 1 = first child, etc.
	// Current depth is len(ed.navStack)
	
	if level < 0 || level >= len(ed.navStack) {
		return
	}
	
	// Save current machine
	ed.saveMachineToCache()
	
	// Get the frame at the target level
	frame := ed.navStack[level]
	
	// Truncate nav stack to that level
	ed.navStack = ed.navStack[:level]
	
	// Start zoom-out animation to that machine
	ed.animating = true
	ed.animStartTime = time.Now().UnixMilli()
	ed.animDuration = 120
	ed.animZoomIn = false
	ed.animCenterX = frame.LinkedStateX
	ed.animCenterY = frame.LinkedStateY
	ed.animTargetMachine = frame.MachineName
}

// saveMachineToCache saves current FSM and positions to bundle cache
func (ed *Editor) saveMachineToCache() {
	if ed.bundleFSMs == nil {
		ed.bundleFSMs = make(map[string]*fsm.FSM)
	}
	if ed.bundleStates == nil {
		ed.bundleStates = make(map[string][]StatePos)
	}
	if ed.bundleUndoStack == nil {
		ed.bundleUndoStack = make(map[string][]Snapshot)
	}
	if ed.bundleRedoStack == nil {
		ed.bundleRedoStack = make(map[string][]Snapshot)
	}
	if ed.bundleModified == nil {
		ed.bundleModified = make(map[string]bool)
	}
	if ed.bundleOffsets == nil {
		ed.bundleOffsets = make(map[string][2]int)
	}
	
	ed.bundleFSMs[ed.currentMachine] = ed.fsm
	ed.bundleStates[ed.currentMachine] = ed.states
	ed.bundleUndoStack[ed.currentMachine] = ed.undoStack
	ed.bundleRedoStack[ed.currentMachine] = ed.redoStack
	ed.bundleOffsets[ed.currentMachine] = [2]int{ed.canvasOffsetX, ed.canvasOffsetY}
	if ed.modified {
		ed.bundleModified[ed.currentMachine] = true
	}
}

// loadMachineFromCache loads a machine from the bundle cache.
// If states aren't in the cache at all, it generates positions from file or auto-layout.
func (ed *Editor) loadMachineFromCache(machineName string) {
	if f, ok := ed.bundleFSMs[machineName]; ok {
		ed.fsm = f
	}
	if states, ok := ed.bundleStates[machineName]; ok {
		ed.states = states
	} else {
		// States not cached — try loading layout from file, fall back to auto-layout
		ed.states = ed.generateStatesForMachine(machineName)
		// Cache the generated states
		if ed.bundleStates == nil {
			ed.bundleStates = make(map[string][]StatePos)
		}
		ed.bundleStates[machineName] = ed.states
	}
	if undo, ok := ed.bundleUndoStack[machineName]; ok {
		ed.undoStack = undo
	} else {
		ed.undoStack = nil
	}
	if redo, ok := ed.bundleRedoStack[machineName]; ok {
		ed.redoStack = redo
	} else {
		ed.redoStack = nil
	}
	ed.modified = ed.bundleModified[machineName]
	if offsets, ok := ed.bundleOffsets[machineName]; ok {
		ed.canvasOffsetX = offsets[0]
		ed.canvasOffsetY = offsets[1]
	} else {
		ed.canvasOffsetX = 0
		ed.canvasOffsetY = 0
	}
	ed.currentMachine = machineName
	ed.selectedState = -1
}

// generateStatesForMachine creates state positions for a machine.
// Tries to load layout from the bundle file first; falls back to SmartLayout.
func (ed *Editor) generateStatesForMachine(machineName string) []StatePos {
	f := ed.fsm
	
	// Try to load layout from bundle file
	if ed.filename != "" {
		_, layout, err := fsmfile.ReadMachineFromBundle(ed.filename, machineName)
		if err == nil && layout != nil && len(layout.States) > 0 {
			if offsets, ok := ed.bundleOffsets[machineName]; !ok {
				// Store offsets from file if not already cached
				if ed.bundleOffsets == nil {
					ed.bundleOffsets = make(map[string][2]int)
				}
				ed.bundleOffsets[machineName] = [2]int{layout.Editor.CanvasOffsetX, layout.Editor.CanvasOffsetY}
				ed.canvasOffsetX = layout.Editor.CanvasOffsetX
				ed.canvasOffsetY = layout.Editor.CanvasOffsetY
			} else {
				ed.canvasOffsetX = offsets[0]
				ed.canvasOffsetY = offsets[1]
			}
			
			states := make([]StatePos, len(f.States))
			for i, name := range f.States {
				if sl, ok := layout.States[name]; ok {
					states[i] = StatePos{Name: name, X: sl.X, Y: sl.Y}
				} else {
					col := i % 5
					row := i / 5
					states[i] = StatePos{Name: name, X: 5 + col*15, Y: 2 + row*4}
				}
			}
			return states
		}
	}
	
	// Fall back to auto-layout
	w, h := 80, 24
	if ed.screen != nil {
		w, h = ed.screen.Size()
		w = w - ed.sidebarWidth - 5
		h = h - 4
	}
	
	states := make([]StatePos, len(f.States))
	autoPositions := fsmfile.SmartLayoutTUI(f, w, h)
	for i, name := range f.States {
		if pos, ok := autoPositions[name]; ok {
			states[i] = StatePos{Name: name, X: pos[0], Y: pos[1]}
		} else {
			col := i % 5
			row := i / 5
			states[i] = StatePos{Name: name, X: 5 + col*15, Y: 2 + row*4}
		}
	}
	return states
}

// anyBundleModified returns true if any machine in the bundle has unsaved changes
func (ed *Editor) anyBundleModified() bool {
	// Check current machine
	if ed.modified {
		return true
	}
	// Check cached machines
	for _, mod := range ed.bundleModified {
		if mod {
			return true
		}
	}
	return false
}

// getModifiedMachines returns list of machine names with unsaved changes
func (ed *Editor) getModifiedMachines() []string {
	var modified []string
	
	// Save current to cache first to ensure it's tracked
	if ed.modified {
		ed.bundleModified[ed.currentMachine] = true
	}
	
	for name, mod := range ed.bundleModified {
		if mod {
			modified = append(modified, name)
		}
	}
	return modified
}

// finishAnimation completes the zoom animation and switches machines
func (ed *Editor) finishAnimation() {
	ed.animating = false
	ed.loadMachineFromCache(ed.animTargetMachine)
	
	// If zooming out, restore viewport position
	if !ed.animZoomIn && len(ed.navStack) >= 0 {
		// Find the state we came from and center on it
		for i, sp := range ed.states {
			if sp.X == ed.animCenterX && sp.Y == ed.animCenterY {
				ed.selectedState = i
				break
			}
		}
	}
	
	ed.mode = ModeCanvas
}

// getBreadcrumbs returns the navigation path as machine names
func (ed *Editor) getBreadcrumbs() []string {
	crumbs := make([]string, 0, len(ed.navStack)+1)
	for _, frame := range ed.navStack {
		crumbs = append(crumbs, frame.MachineName)
	}
	crumbs = append(crumbs, ed.currentMachine)
	return crumbs
}

// offerCreateMachineForLink prompts the user to create a new machine
// when pressing K (link) but no other machines exist to link to.
func (ed *Editor) offerCreateMachineForLink(stateName string) {
	ed.inputPrompt = "No machines to link to. Create new machine (name, or empty to cancel): "
	ed.inputBuffer = ""
	ed.inputAction = func(name string) {
		if name == "" {
			ed.mode = ModeCanvas
			return
		}

		doCreate := func() {
			if ed.machineNameExists(name) {
				ed.showMessage("Machine already exists: "+name, MsgError)
				ed.mode = ModeCanvas
				return
			}
			// Create the new machine (DFA by default).
			newFSM := fsm.New(fsm.TypeDFA)
			newFSM.Name = name
			ed.addMachineToBundle(name, newFSM, nil)
			ed.updateMenuItems()

			// Set the link.
			ed.saveSnapshot()
			ed.fsm.SetLinkedMachine(stateName, name)
			ed.modified = true
			ed.showMessage(stateName+" → "+name+" (new machine created)", MsgSuccess)
			ed.mode = ModeCanvas
		}

		if !ed.isBundle {
			ed.promoteIfNeeded(doCreate)
		} else {
			doCreate()
		}
	}
	ed.mode = ModeInput
}
