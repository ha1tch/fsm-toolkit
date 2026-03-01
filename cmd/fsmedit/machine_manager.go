package main

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// ====================================================================
// Machine Manager — bundle machine management overlay
// ====================================================================

func (ed *Editor) openMachineManager() {
	// If not in bundle mode, single-FSM: show just the current machine.
	if !ed.isBundle {
		// Ensure current state is cached before entering manager.
		ed.machMgrSelected = 0
		ed.machMgrScroll = 0
		ed.machMgrShowInfo = false
		ed.mode = ModeMachineManager
		return
	}

	// Save current machine to cache so info is accurate.
	ed.saveMachineToCache()

	ed.machMgrSelected = 0
	for i, name := range ed.bundleMachines {
		if name == ed.currentMachine {
			ed.machMgrSelected = i
			break
		}
	}
	ed.machMgrScroll = 0
	ed.machMgrShowInfo = false
	ed.mode = ModeMachineManager
}

// machMgrMachineNames returns the machine list for the manager.
// In single mode returns a single-element list, in bundle mode
// returns bundleMachines.
func (ed *Editor) machMgrMachineNames() []string {
	if !ed.isBundle {
		name := ed.fsm.Name
		if name == "" {
			name = "(unnamed)"
		}
		return []string{name}
	}
	return ed.bundleMachines
}

func (ed *Editor) drawMachineManager(w, h int) {
	machines := ed.machMgrMachineNames()
	machCount := len(machines)

	boxW := 72
	boxH := h - 6
	if boxH < 16 {
		boxH = 16
	}
	if boxH > h-4 {
		boxH = h - 4
	}

	cx, cy, _, ch := ed.drawOverlayBox("MACHINES", boxW, boxH, w, h)

	y := cy + 1

	// Mode indicator.
	if ed.isBundle {
		label := fmt.Sprintf("Bundle [%d machines]", machCount)
		ed.drawString(cx, y, label, styleOverlay)
	} else {
		ed.drawString(cx, y, "Single FSM (use Add to create a bundle)", styleOverlayDim)
	}
	y++

	// Determine list region size.
	var infoH int
	if ed.machMgrShowInfo {
		infoH = 8 // lines for info panel
	}
	listH := ch - 5 - infoH // header + mode + help + footer + gap
	if listH < 3 {
		listH = 3
	}

	// Scroll management.
	ed.machMgrScroll = ensureVisible(ed.machMgrSelected, ed.machMgrScroll, listH)

	// Column headers.
	y++
	header := fmt.Sprintf("  %-24s %5s %5s %-8s %s",
		"Name", "St", "Tr", "Type", "Links")
	ed.drawString(cx, y, header, styleOverlayHdr)
	y++

	// Machine rows.
	for i := 0; i < listH && ed.machMgrScroll+i < machCount; i++ {
		idx := ed.machMgrScroll + i
		name := machines[idx]

		var stCount, trCount int
		var fsmType string
		var linkCount int

		if ed.isBundle {
			if f, ok := ed.bundleFSMs[name]; ok {
				stCount = len(f.States)
				trCount = len(f.Transitions)
				fsmType = string(f.Type)
				linkCount = len(f.LinkedStates())
			}
		} else {
			stCount = len(ed.fsm.States)
			trCount = len(ed.fsm.Transitions)
			fsmType = string(ed.fsm.Type)
			linkCount = len(ed.fsm.LinkedStates())
		}

		// Indicate current machine.
		marker := "  "
		if ed.isBundle && name == ed.currentMachine {
			marker = "> "
		}

		linkStr := ""
		if linkCount > 0 {
			linkStr = fmt.Sprintf("%d", linkCount)
		}

		line := fmt.Sprintf("%s%-24s %5d %5d %-8s %s",
			marker,
			truncate(name, 24),
			stCount, trCount,
			truncate(fsmType, 8),
			linkStr)

		s := styleOverlay
		if idx == ed.machMgrSelected {
			s = styleOverlayHl
		}
		ed.drawString(cx, y, line, s)
		y++
	}

	// Scroll indicators.
	if ed.machMgrScroll > 0 {
		ed.drawString(cx+boxW-6, cy+4, " ↑ ", styleOverlayDim)
	}
	if ed.machMgrScroll+listH < machCount {
		ed.drawString(cx+boxW-6, cy+4+listH, " ↓ ", styleOverlayDim)
	}

	// Details panel.
	if ed.machMgrShowInfo && ed.machMgrSelected < machCount {
		y = cy + ch - 3 - infoH
		ed.drawMachineInfo(cx, y, boxW-4, machines[ed.machMgrSelected])
	}

	// Help line.
	helpY := cy + ch - 2
	if ed.isBundle {
		ed.drawString(cx, helpY,
			"[Enter] Switch  [A] Add  [R] Rename  [D] Delete  [Tab] Info  [Esc] Back",
			styleOverlayDim)
	} else {
		ed.drawString(cx, helpY,
			"[A] Add machine (creates bundle)  [Esc] Back",
			styleOverlayDim)
	}

	ed.drawString(cx, cy+ch-1, "[Esc] Back", styleOverlayDim)
}

// drawMachineInfo draws the details panel for a machine.
func (ed *Editor) drawMachineInfo(cx, y, maxW int, machineName string) {
	ed.drawString(cx, y, fmt.Sprintf("Details: %s", machineName), styleOverlayHdr)
	y++

	if !ed.isBundle {
		ed.drawString(cx+2, y, "(single FSM - no bundle details)", styleOverlayDim)
		return
	}

	f, ok := ed.bundleFSMs[machineName]
	if !ok {
		return
	}

	// Outgoing links (this machine's states that link to other machines).
	linked := f.LinkedStates()
	if len(linked) > 0 {
		parts := make([]string, 0, len(linked))
		for _, st := range linked {
			target := f.GetLinkedMachine(st)
			parts = append(parts, fmt.Sprintf("%s->%s", st, target))
		}
		line := "  Links out: " + strings.Join(parts, ", ")
		if len(line) > maxW {
			line = line[:maxW-2] + ".."
		}
		ed.drawString(cx, y, line, styleOverlay)
	} else {
		ed.drawString(cx, y, "  Links out: (none)", styleOverlayDim)
	}
	y++

	// Incoming links (other machines that link TO this machine).
	var incoming []string
	for _, otherName := range ed.bundleMachines {
		if otherName == machineName {
			continue
		}
		if otherFSM, ok := ed.bundleFSMs[otherName]; ok {
			for _, st := range otherFSM.LinkedStates() {
				if otherFSM.GetLinkedMachine(st) == machineName {
					incoming = append(incoming, fmt.Sprintf("%s.%s", otherName, st))
				}
			}
		}
	}
	if len(incoming) > 0 {
		line := "  Links in:  " + strings.Join(incoming, ", ")
		if len(line) > maxW {
			line = line[:maxW-2] + ".."
		}
		ed.drawString(cx, y, line, styleOverlay)
	} else {
		ed.drawString(cx, y, "  Links in:  (none)", styleOverlayDim)
	}
	y++

	// Classes in use.
	classNames := f.ClassNames()
	nonDefault := make([]string, 0)
	for _, cn := range classNames {
		if cn != fsm.DefaultClassName {
			nonDefault = append(nonDefault, cn)
		}
	}
	if len(nonDefault) > 0 {
		line := fmt.Sprintf("  Classes:   %d (%s)", len(nonDefault), strings.Join(nonDefault, ", "))
		if len(line) > maxW {
			line = line[:maxW-2] + ".."
		}
		ed.drawString(cx, y, line, styleOverlay)
	} else {
		ed.drawString(cx, y, "  Classes:   (default only)", styleOverlayDim)
	}
}

func (ed *Editor) handleMachineManagerKey(ev *tcell.EventKey) bool {
	machines := ed.machMgrMachineNames()
	machCount := len(machines)

	switch ev.Key() {
	case tcell.KeyEscape:
		ed.mode = ModeMenu
		return false
	case tcell.KeyTab:
		if ed.isBundle {
			ed.machMgrShowInfo = !ed.machMgrShowInfo
		}
	case tcell.KeyUp:
		if ed.machMgrSelected > 0 {
			ed.machMgrSelected--
		}
	case tcell.KeyDown:
		if ed.machMgrSelected < machCount-1 {
			ed.machMgrSelected++
		}
	case tcell.KeyEnter:
		if ed.isBundle && ed.machMgrSelected < machCount {
			ed.switchToMachine(machines[ed.machMgrSelected])
		}
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'a', 'A':
			ed.machMgrAddMachine()
		case 'r', 'R':
			if ed.isBundle && ed.machMgrSelected < machCount {
				ed.machMgrRenameMachine(machines[ed.machMgrSelected])
			}
		case 'd', 'D':
			if ed.isBundle && machCount > 1 && ed.machMgrSelected < machCount {
				ed.machMgrDeleteMachine(machines[ed.machMgrSelected])
			} else if ed.isBundle && machCount <= 1 {
				ed.showMessage("Cannot delete the last machine", MsgError)
			}
		}
	}
	return false
}

func (ed *Editor) handleMachineManagerMouse(ev *tcell.EventMouse, w, h int) {
	_, my := ev.Position()
	buttons := ev.Buttons()

	machines := ed.machMgrMachineNames()
	machCount := len(machines)

	// Compute layout to match drawMachineManager.
	boxH := h - 6
	if boxH < 16 {
		boxH = 16
	}
	if boxH > h-4 {
		boxH = h - 4
	}
	boxY := (h - boxH) / 2
	if boxY < 1 {
		boxY = 1
	}
	cy := boxY + 1
	ch := boxH - 2

	var infoH int
	if ed.machMgrShowInfo {
		infoH = 8
	}
	listH := ch - 5 - infoH
	if listH < 3 {
		listH = 3
	}

	listStartY := cy + 4 // after mode line + blank + header
	listEndY := listStartY + listH

	if buttons&tcell.WheelUp != 0 {
		if ed.machMgrSelected > 0 {
			ed.machMgrSelected--
		}
		return
	}
	if buttons&tcell.WheelDown != 0 {
		if ed.machMgrSelected < machCount-1 {
			ed.machMgrSelected++
		}
		return
	}

	if buttons&tcell.Button1 != 0 && my >= listStartY && my < listEndY {
		idx := ed.machMgrScroll + (my - listStartY)
		if idx >= 0 && idx < machCount {
			ed.machMgrSelected = idx
		}
	}
}

// switchToMachine switches to the selected machine and returns to canvas.
func (ed *Editor) switchToMachine(name string) {
	if name == ed.currentMachine {
		ed.mode = ModeCanvas
		return
	}
	ed.saveMachineToCache()
	ed.loadMachineFromCache(name)
	ed.showMessage("Switched to: "+name, MsgSuccess)
	ed.mode = ModeCanvas
}

// machMgrAddMachine adds a new machine, promoting to bundle if needed.
func (ed *Editor) machMgrAddMachine() {
	if !ed.isBundle {
		ed.promoteIfNeeded(func() {
			ed.newMachinePromptReturn(ModeMachineManager)
		})
		return
	}
	ed.newMachinePromptReturn(ModeMachineManager)
}

// newMachinePromptReturn is like newMachinePrompt but returns to the
// specified mode instead of the menu after creation.
func (ed *Editor) newMachinePromptReturn(returnMode Mode) {
	ed.inputPrompt = "New machine name: "
	ed.inputBuffer = ""
	ed.inputAction = func(name string) {
		if name == "" {
			ed.mode = returnMode
			return
		}
		if ed.machineNameExists(name) {
			ed.showMessage("Machine already exists: "+name, MsgError)
			ed.mode = returnMode
			return
		}
		// Create with default type (DFA).
		newFSM := fsm.New(fsm.TypeDFA)
		newFSM.Name = name
		ed.addMachineToBundle(name, newFSM, nil)
		ed.updateMenuItems()
		ed.showMessage("Created machine: "+name, MsgSuccess)
		ed.mode = returnMode
	}
	ed.mode = ModeInput
}

// machMgrRenameMachine renames a machine, propagating to all link references.
func (ed *Editor) machMgrRenameMachine(oldName string) {
	ed.inputPrompt = fmt.Sprintf("Rename \"%s\" to: ", oldName)
	ed.inputBuffer = oldName
	ed.inputAction = func(newName string) {
		newName = strings.TrimSpace(newName)
		if newName == "" || newName == oldName {
			ed.mode = ModeMachineManager
			return
		}
		if ed.machineNameExists(newName) {
			ed.showMessage("Machine already exists: "+newName, MsgError)
			ed.mode = ModeMachineManager
			return
		}

		// Save current machine to cache first.
		ed.saveMachineToCache()

		// Update bundleMachines list.
		for i, name := range ed.bundleMachines {
			if name == oldName {
				ed.bundleMachines[i] = newName
				break
			}
		}

		// Rekey all cache maps.
		if f, ok := ed.bundleFSMs[oldName]; ok {
			f.Name = newName
			ed.bundleFSMs[newName] = f
			delete(ed.bundleFSMs, oldName)
		}
		if s, ok := ed.bundleStates[oldName]; ok {
			ed.bundleStates[newName] = s
			delete(ed.bundleStates, oldName)
		}
		if u, ok := ed.bundleUndoStack[oldName]; ok {
			ed.bundleUndoStack[newName] = u
			delete(ed.bundleUndoStack, oldName)
		}
		if r, ok := ed.bundleRedoStack[oldName]; ok {
			ed.bundleRedoStack[newName] = r
			delete(ed.bundleRedoStack, oldName)
		}
		if m, ok := ed.bundleModified[oldName]; ok {
			ed.bundleModified[newName] = m
			delete(ed.bundleModified, oldName)
		}
		if o, ok := ed.bundleOffsets[oldName]; ok {
			ed.bundleOffsets[newName] = o
			delete(ed.bundleOffsets, oldName)
		}

		// Propagate rename to all linked state references across the bundle.
		for _, machName := range ed.bundleMachines {
			if f, ok := ed.bundleFSMs[machName]; ok {
				for state, target := range f.LinkedMachines {
					if target == oldName {
						f.LinkedMachines[state] = newName
					}
				}
			}
		}

		// Update currentMachine if we renamed the active one.
		if ed.currentMachine == oldName {
			ed.currentMachine = newName
			ed.fsm.Name = newName
		}

		// Update navigation stack.
		for i := range ed.navStack {
			if ed.navStack[i].MachineName == oldName {
				ed.navStack[i].MachineName = newName
			}
		}

		// Mark as modified.
		ed.modified = true
		ed.bundleModified[newName] = true

		ed.showMessage(fmt.Sprintf("Renamed: %s -> %s", oldName, newName), MsgSuccess)
		ed.mode = ModeMachineManager
	}
	ed.mode = ModeInput
}

// machMgrDeleteMachine deletes a machine from the bundle with safety checks.
func (ed *Editor) machMgrDeleteMachine(name string) {
	// Check if any other machines link to this one.
	var incomingRefs []string
	for _, otherName := range ed.bundleMachines {
		if otherName == name {
			continue
		}
		if f, ok := ed.bundleFSMs[otherName]; ok {
			for _, st := range f.LinkedStates() {
				if f.GetLinkedMachine(st) == name {
					incomingRefs = append(incomingRefs, otherName+"."+st)
				}
			}
		}
	}

	prompt := fmt.Sprintf("Delete machine \"%s\"? (y/n): ", name)
	if len(incomingRefs) > 0 {
		refs := strings.Join(incomingRefs, ", ")
		if len(refs) > 40 {
			refs = refs[:37] + "..."
		}
		prompt = fmt.Sprintf("Delete \"%s\"? Linked from: %s (y/n): ", name, refs)
	}

	ed.inputPrompt = prompt
	ed.inputBuffer = ""
	ed.inputAction = func(answer string) {
		if strings.ToLower(answer) != "y" {
			ed.mode = ModeMachineManager
			return
		}

		// Save current to cache.
		ed.saveMachineToCache()

		// Remove from machine list.
		newList := make([]string, 0, len(ed.bundleMachines)-1)
		for _, m := range ed.bundleMachines {
			if m != name {
				newList = append(newList, m)
			}
		}
		ed.bundleMachines = newList

		// Remove from caches.
		delete(ed.bundleFSMs, name)
		delete(ed.bundleStates, name)
		delete(ed.bundleUndoStack, name)
		delete(ed.bundleRedoStack, name)
		delete(ed.bundleModified, name)
		delete(ed.bundleOffsets, name)

		// Clear dangling link references.
		for _, machName := range ed.bundleMachines {
			if f, ok := ed.bundleFSMs[machName]; ok {
				for state, target := range f.LinkedMachines {
					if target == name {
						delete(f.LinkedMachines, state)
					}
				}
			}
		}

		// If we deleted the current machine, switch to another.
		if ed.currentMachine == name {
			if len(ed.bundleMachines) > 0 {
				ed.loadMachineFromCache(ed.bundleMachines[0])
			}
		}

		// If only one machine remains, could demote from bundle,
		// but that's destructive — leave as bundle for now.

		// Adjust selection.
		if ed.machMgrSelected >= len(ed.bundleMachines) {
			ed.machMgrSelected = len(ed.bundleMachines) - 1
		}

		ed.modified = true
		ed.showMessage("Deleted machine: "+name, MsgSuccess)
		ed.mode = ModeMachineManager
	}
	ed.mode = ModeInput
}
