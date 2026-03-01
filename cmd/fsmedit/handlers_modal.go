// Modal dialog and selector key handlers for fsmedit.
package main

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

func (ed *Editor) handleSelectLinkTargetKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyUp:
		if ed.linkTargetSelected > 0 {
			ed.linkTargetSelected--
		}
	case tcell.KeyDown:
		if ed.linkTargetSelected < len(ed.linkTargetMachines)-1 {
			ed.linkTargetSelected++
		}
	case tcell.KeyEnter:
		if ed.linkTargetSelected >= 0 && ed.linkTargetSelected < len(ed.linkTargetMachines) {
			targetMachine := ed.linkTargetMachines[ed.linkTargetSelected]
			stateName := ed.states[ed.selectedState].Name
			ed.saveSnapshot()
			ed.fsm.SetLinkedMachine(stateName, targetMachine)
			ed.showMessage(stateName+" → "+targetMachine, MsgSuccess)
			ed.modified = true
			ed.mode = ModeCanvas
		}
	case tcell.KeyEscape:
		ed.mode = ModeCanvas
	}
	return false
}

func (ed *Editor) handleImportMachineSelectKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		ed.mode = ModeMenu
	case tcell.KeyUp:
		if ed.importCursor > 0 {
			ed.importCursor--
		}
	case tcell.KeyDown:
		if ed.importCursor < len(ed.importMachines)-1 {
			ed.importCursor++
		}
	case tcell.KeyEnter:
		// Execute import of selected machines
		ed.executeImport()
	case tcell.KeyRune:
		switch ev.Rune() {
		case ' ':
			// Toggle selection of current item
			if ed.importCursor >= 0 && ed.importCursor < len(ed.importSelected) {
				ed.importSelected[ed.importCursor] = !ed.importSelected[ed.importCursor]
			}
		case 'a', 'A':
			// Toggle all
			allSelected := true
			for _, s := range ed.importSelected {
				if !s {
					allSelected = false
					break
				}
			}
			for i := range ed.importSelected {
				ed.importSelected[i] = !allSelected
			}
		}
	}
	return false
}

func (ed *Editor) handleSelectMachineKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyUp:
		if ed.machineSelected > 0 {
			ed.machineSelected--
		}
	case tcell.KeyDown:
		if ed.machineSelected < len(ed.machineList)-1 {
			ed.machineSelected++
		}
	case tcell.KeyEnter:
		if ed.machineSelected >= 0 && ed.machineSelected < len(ed.machineList) {
			machineName := ed.machineList[ed.machineSelected].Name
			if err := ed.loadMachineFromBundle(machineName); err != nil {
				ed.showMessage("Error loading machine: "+err.Error(), MsgError)
			} else {
				ed.showMessage("Loaded machine: "+machineName, MsgSuccess)
			}
		}
	case tcell.KeyEscape:
		if ed.currentMachine != "" {
			// Mid-edit: return to canvas with current machine
			ed.mode = ModeCanvas
		} else {
			// Initial load: go back to menu/file picker
			ed.mode = ModeMenu
			ed.isBundle = false
			ed.bundleFSMs = nil
			ed.bundleStates = nil
			ed.bundleUndoStack = nil
			ed.bundleRedoStack = nil
			ed.bundleModified = nil
			ed.bundleOffsets = nil
		}
	}
	return false
}

func (ed *Editor) handleInputKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		ed.mode = ModeMenu
	case tcell.KeyEnter:
		if ed.inputAction != nil {
			ed.inputAction(ed.inputBuffer)
		}
		ed.inputBuffer = ""
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(ed.inputBuffer) > 0 {
			ed.inputBuffer = ed.inputBuffer[:len(ed.inputBuffer)-1]
		}
	case tcell.KeyRune:
		ed.inputBuffer += string(ev.Rune())
	}
	return false
}

func (ed *Editor) handleFilePickerKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		ed.importMode = false
		ed.dirPickerMode = false
		ed.dirPickerAction = nil
		ed.mode = ModeMenu
	case tcell.KeyTab:
		// Switch focus between directories and files (not in dir picker mode).
		if !ed.dirPickerMode {
			ed.filePickerFocus = 1 - ed.filePickerFocus
		}
	case tcell.KeyLeft:
		ed.filePickerFocus = 0 // Focus directories
	case tcell.KeyRight:
		if !ed.dirPickerMode {
			ed.filePickerFocus = 1 // Focus files
		}
	case tcell.KeyUp:
		if ed.filePickerFocus == 0 {
			if ed.dirSelected > 0 {
				ed.dirSelected--
			}
		} else {
			if ed.fileSelected > 0 {
				ed.fileSelected--
			}
		}
	case tcell.KeyDown:
		if ed.filePickerFocus == 0 {
			if ed.dirSelected < len(ed.dirList)-1 {
				ed.dirSelected++
			}
		} else {
			if ed.fileSelected < len(ed.fileList)-1 {
				ed.fileSelected++
			}
		}
	case tcell.KeyEnter:
		if ed.dirPickerMode {
			// In dir picker mode, Enter always navigates into the selected directory.
			if ed.filePickerFocus == 0 && len(ed.dirList) > 0 {
				selectedDir := ed.dirList[ed.dirSelected]
				var newDir string
				if selectedDir == ".." {
					newDir = filepath.Dir(ed.currentDir)
				} else {
					newDir = filepath.Join(ed.currentDir, selectedDir)
				}
				ed.currentDir = newDir
				ed.refreshFilePicker()
			}
		} else if ed.filePickerFocus == 0 {
			// Navigate to selected directory
			selectedDir := ed.dirList[ed.dirSelected]
			var newDir string
			if selectedDir == ".." {
				newDir = filepath.Dir(ed.currentDir)
			} else {
				newDir = filepath.Join(ed.currentDir, selectedDir)
			}
			ed.currentDir = newDir
			ed.refreshFilePicker()
		} else {
			// Open or import selected file
			if len(ed.fileList) > 0 {
				fullPath := filepath.Join(ed.currentDir, ed.fileList[ed.fileSelected])
				
				if ed.importMode {
					// Import flow
					ed.handleImportFile(fullPath)
				} else {
					// Normal open flow
					ed.filename = fullPath
					if err := ed.loadFile(fullPath); err != nil {
						ed.showMessage("Error: "+err.Error(), MsgError)
					} else {
						// Save last used directory
						ed.config.LastDir = ed.currentDir
						SaveConfig(ed.config)
						
						ed.showMessage("Loaded: "+ed.filename, MsgSuccess)
						ed.mode = ModeCanvas
					}
				}
			}
		}
	case tcell.KeyRune:
		if ed.dirPickerMode && (ev.Rune() == 's' || ev.Rune() == 'S') {
			// Select current directory.
			if ed.dirPickerAction != nil {
				ed.dirPickerAction(ed.currentDir)
			}
			ed.dirPickerMode = false
			ed.dirPickerAction = nil
			ed.mode = ModeSettings
		}
	}
	return false
}

func (ed *Editor) handleSelectTypeKey(ev *tcell.EventKey) bool {
	types := []fsm.Type{fsm.TypeDFA, fsm.TypeNFA, fsm.TypeMoore, fsm.TypeMealy}
	switch ev.Key() {
	case tcell.KeyEscape:
		ed.newMachineTypeSelect = false
		ed.mode = ModeMenu
	case tcell.KeyUp:
		if ed.typeMenuSelected > 0 {
			ed.typeMenuSelected--
		}
	case tcell.KeyDown:
		if ed.typeMenuSelected < len(types)-1 {
			ed.typeMenuSelected++
		}
	case tcell.KeyEnter:
		selectedType := types[ed.typeMenuSelected]
		
		if ed.newMachineTypeSelect {
			// Creating a new machine in bundle mode
			ed.newMachineTypeSelect = false
			name := ed.pendingNewMachineName
			ed.pendingNewMachineName = ""
			
			newFSM := fsm.New(selectedType)
			newFSM.Name = name
			ed.addMachineToBundle(name, newFSM, nil)
			
			// Switch to the new machine
			ed.saveMachineToCache()
			ed.loadMachineFromCache(name)
			ed.showMessage("Created machine: "+name, MsgSuccess)
			ed.mode = ModeCanvas
		} else {
			// Normal FSM type change
			ed.fsm.Type = selectedType
			ed.modified = true
			ed.updateMenuItems()
			ed.showMessage("FSM type set to "+fsmTypeDisplayName(ed.fsm.Type), MsgSuccess)
			ed.mode = ModeMenu
		}
	}
	return false
}

func (ed *Editor) handleAddTransitionKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		ed.mode = ModeCanvas
	case tcell.KeyUp:
		if ed.menuSelected > 0 {
			ed.menuSelected--
		}
	case tcell.KeyDown:
		if ed.menuSelected < len(ed.validTargets)-1 {
			ed.menuSelected++
		}
	case tcell.KeyEnter:
		ed.completeAddTransition()
	}
	return false
}

func (ed *Editor) handleSelectInputKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		ed.mode = ModeCanvas
	case tcell.KeyUp:
		if ed.menuSelected > 0 {
			ed.menuSelected--
		}
	case tcell.KeyDown:
		if ed.menuSelected < len(ed.fsm.Alphabet) {
			ed.menuSelected++
		}
	case tcell.KeyEnter:
		ed.completeSelectInput()
	}
	return false
}

func (ed *Editor) handleSelectOutputKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		ed.mode = ModeCanvas
	case tcell.KeyUp:
		if ed.menuSelected > 0 {
			ed.menuSelected--
		}
	case tcell.KeyDown:
		if ed.menuSelected < len(ed.fsm.OutputAlphabet)-1 {
			ed.menuSelected++
		}
	case tcell.KeyEnter:
		ed.completeSelectOutput()
	}
	return false
}

