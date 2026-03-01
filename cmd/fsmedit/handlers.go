// Top-level key dispatch, menu handling, and toggles for fsmedit.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
)

func (ed *Editor) handleKey(ev *tcell.EventKey) bool {
	// Global shortcuts (Ctrl or Cmd on macOS)
	// tcell maps Cmd to Ctrl on macOS in most terminals, but we also
	// check for Rune + Meta modifier for terminals that report it differently
	mod := ev.Modifiers()
	isCtrlOrCmd := func(key tcell.Key, r rune) bool {
		// Check standard Ctrl+key
		if ev.Key() == key {
			return true
		}
		// Check for Cmd+key (reported as Meta+rune on some terminals)
		if mod&tcell.ModMeta != 0 && ev.Rune() == r {
			return true
		}
		// Check for Alt+key as fallback (some terminals use Alt for Meta)
		if mod&tcell.ModAlt != 0 && ev.Rune() == r {
			return true
		}
		return false
	}

	// Clear any active flash on keypress
	ed.clearFlash()

	if isCtrlOrCmd(tcell.KeyCtrlC, 'c') {
		ed.copyToClipboard()
		return false
	}
	if isCtrlOrCmd(tcell.KeyCtrlV, 'v') {
		ed.pasteFromClipboard()
		return false
	}
	if isCtrlOrCmd(tcell.KeyCtrlS, 's') {
		ed.save()
		return false
	}
	if isCtrlOrCmd(tcell.KeyCtrlZ, 'z') {
		ed.undo()
		return false
	}
	if isCtrlOrCmd(tcell.KeyCtrlY, 'y') {
		ed.redo()
		return false
	}

	// Ctrl+D: Toggle canvas drag mode (with minimap)
	if ev.Key() == tcell.KeyCtrlD {
		ed.toggleCanvasDragMode()
		return false
	}

	switch ed.mode {
	case ModeMenu:
		return ed.handleMenuKey(ev)
	case ModeCanvas:
		return ed.handleCanvasKey(ev)
	case ModeInput:
		return ed.handleInputKey(ev)
	case ModeFilePicker:
		return ed.handleFilePickerKey(ev)
	case ModeSelectType:
		return ed.handleSelectTypeKey(ev)
	case ModeAddTransition:
		return ed.handleAddTransitionKey(ev)
	case ModeSelectInput:
		return ed.handleSelectInputKey(ev)
	case ModeSelectOutput:
		return ed.handleSelectOutputKey(ev)
	case ModeMove:
		return ed.handleMoveKey(ev)
	case ModeHelp:
		return ed.handleHelpKey(ev)
	case ModeCanvasDrag:
		return ed.handleCanvasDragKey(ev)
	case ModeSelectMachine:
		return ed.handleSelectMachineKey(ev)
	case ModeSelectLinkTarget:
		return ed.handleSelectLinkTargetKey(ev)
	case ModeImportMachineSelect:
		return ed.handleImportMachineSelectKey(ev)
	case ModeClassEditor:
		return ed.handleClassEditorKey(ev)
	case ModeClassAssign:
		return ed.handleClassAssignKey(ev)
	case ModePropertyEditor:
		return ed.handlePropertyEditorKey(ev)
	case ModeListEditor:
		return ed.handleListEditorKey(ev)
	case ModeSettings:
		return ed.handleSettingsKey(ev)
	case ModeDrawer:
		return ed.handleDrawerKey(ev)
	case ModeMachineManager:
		return ed.handleMachineManagerKey(ev)
	}
	return false
}

func (ed *Editor) handleMenuKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyUp:
		if ed.menuSelected > 0 {
			ed.menuSelected--
		}
	case tcell.KeyDown:
		if ed.menuSelected < len(ed.menuItems)-1 {
			ed.menuSelected++
		}
	case tcell.KeyEnter:
		return ed.executeMenuItem()
	case tcell.KeyEscape:
		if ed.filename != "" || len(ed.fsm.States) > 0 {
			ed.mode = ModeCanvas
		}
	}
	return false
}

func (ed *Editor) executeMenuItem() bool {
	item := ed.menuItems[ed.menuSelected]
	
	switch {
	case item == "New":
		ed.confirmNew()
	case item == "Open File":
		ed.openFilePicker()
	case item == "Import":
		ed.importFilePicker()
	case item == "Machines":
		ed.openMachineManager()
	case item == "Save":
		ed.save()
	case item == "Save As":
		ed.saveAs()
	case item == "Edit Canvas":
		ed.mode = ModeCanvas
	case item == "Render":
		if len(ed.fsm.States) == 0 {
			ed.showMessage("Canvas is empty - nothing to render", MsgError)
		} else {
			ed.renderView()
		}
	case strings.HasPrefix(item, "Renderer:"):
		ed.toggleRenderer()
	case strings.HasPrefix(item, "File Type:"):
		ed.toggleFileType()
	case strings.HasPrefix(item, "FSM Type:"):
		ed.typeMenuSelected = int(ed.fsmTypeIndex())
		ed.mode = ModeSelectType
	case item == "Settings":
		ed.openSettings()
	case item == "Quit":
		// Check for unsaved changes - in bundles, check all machines
		hasUnsaved := ed.modified
		if ed.isBundle {
			// Save current to cache to ensure modified flag is tracked
			if ed.modified {
				ed.bundleModified[ed.currentMachine] = true
			}
			hasUnsaved = ed.anyBundleModified()
		}
		
		if hasUnsaved {
			prompt := "Unsaved changes. Quit anyway? (y/n): "
			if ed.isBundle {
				modMachines := ed.getModifiedMachines()
				if len(modMachines) > 1 {
					prompt = fmt.Sprintf("%d machines have unsaved changes. Quit anyway? (y/n): ", len(modMachines))
				}
			}
			ed.inputPrompt = prompt
			ed.inputBuffer = ""
			ed.inputAction = func(s string) {
				if strings.ToLower(s) == "y" {
					ed.screen.Fini()
					os.Exit(0)
				}
				ed.mode = ModeMenu
			}
			ed.mode = ModeInput
		} else {
			return true
		}
	}
	return false
}

func (ed *Editor) toggleRenderer() {
	if ed.config.Renderer == "native" {
		ed.config.Renderer = "graphviz"
		ed.showMessage("Renderer set to Graphviz", MsgInfo)
	} else {
		ed.config.Renderer = "native"
		ed.showMessage("Renderer set to Native", MsgInfo)
	}
	ed.updateMenuItems()
	if err := SaveConfig(ed.config); err != nil {
		ed.showMessage("Failed to save config: "+err.Error(), MsgError)
	}
}

func (ed *Editor) clearFlash() {
	ed.flashInput = ""
	ed.flashOutput = ""
	ed.flashTransIdx = -1
}

func (ed *Editor) toggleFileType() {
	if ed.config.FileType == "png" {
		ed.config.FileType = "svg"
		ed.showMessage("File type set to SVG", MsgInfo)
	} else {
		ed.config.FileType = "png"
		ed.showMessage("File type set to PNG", MsgInfo)
	}
	ed.updateMenuItems()
	if err := SaveConfig(ed.config); err != nil {
		ed.showMessage("Failed to save config: "+err.Error(), MsgError)
	}
}

func (ed *Editor) toggleSidebarCollapse() {
	if ed.sidebarCollapsed {
		// Expand to snap width
		ed.sidebarWidth = ed.sidebarSnapWidth
		ed.sidebarCollapsed = false
		ed.showMessage("Sidebar expanded", MsgInfo)
	} else {
		// Collapse to minimum
		ed.sidebarWidth = ed.sidebarMinWidth
		ed.sidebarCollapsed = true
		ed.showMessage("Sidebar collapsed", MsgInfo)
	}
}
