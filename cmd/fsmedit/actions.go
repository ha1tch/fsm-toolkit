// State/transition editing actions for fsmedit.
package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
	"github.com/ha1tch/fsm-toolkit/pkg/fsmfile"
)


func (ed *Editor) copyToClipboard() {
	// Generate hex representation of the FSM
	records, stateNames, inputNames, outputNames := fsmfile.FSMToRecords(ed.fsm)
	hex := fsmfile.FormatHex(records, 1) // width=1 means one record per line

	// Generate labels.toml content
	labels := fsmfile.GenerateLabels(ed.fsm, stateNames, inputNames, outputNames)

	// Generate layout.toml content from current state positions
	positions := make(map[string][2]int)
	for _, sp := range ed.states {
		positions[sp.Name] = [2]int{sp.X, sp.Y}
	}
	layout := fsmfile.GenerateLayout(positions, ed.canvasOffsetX, ed.canvasOffsetY)

	// Combine all content with separators
	var sb strings.Builder
	sb.WriteString(hex)
	sb.WriteString("\n# ---- labels.toml -----------------------------------\n")
	sb.WriteString(labels)
	sb.WriteString("# ---- layout.toml -----------------------------------\n")
	sb.WriteString(layout)

	content := sb.String()

	// Find appropriate clipboard command for the OS
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		// Try xclip first, fall back to xsel
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else if _, err := exec.LookPath("wl-copy"); err == nil {
			// Wayland
			cmd = exec.Command("wl-copy")
		} else {
			ed.showMessage("No clipboard tool found (install xclip or xsel)", MsgError)
			return
		}
	case "windows":
		cmd = exec.Command("clip")
	default:
		ed.showMessage("Clipboard not supported on "+runtime.GOOS, MsgError)
		return
	}

	// Pipe the content to the clipboard command
	stdin, err := cmd.StdinPipe()
	if err != nil {
		ed.showMessage("Clipboard error: "+err.Error(), MsgError)
		return
	}

	if err := cmd.Start(); err != nil {
		ed.showMessage("Clipboard error: "+err.Error(), MsgError)
		return
	}

	stdin.Write([]byte(content))
	stdin.Close()

	if err := cmd.Wait(); err != nil {
		ed.showMessage("Clipboard error: "+err.Error(), MsgError)
		return
	}

	ed.showMessage(fmt.Sprintf("Copied FSM to clipboard (%d records)", len(records)), MsgSuccess)
}

func (ed *Editor) pasteFromClipboard() {
	// Get clipboard content using OS-specific command
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbpaste")
	case "linux":
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard", "-o")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--output")
		} else if _, err := exec.LookPath("wl-paste"); err == nil {
			cmd = exec.Command("wl-paste")
		} else {
			ed.showMessage("No clipboard tool found (install xclip or xsel)", MsgError)
			return
		}
	case "windows":
		// Windows doesn't have a simple paste command, use PowerShell
		cmd = exec.Command("powershell", "-command", "Get-Clipboard")
	default:
		ed.showMessage("Clipboard not supported on "+runtime.GOOS, MsgError)
		return
	}

	output, err := cmd.Output()
	if err != nil {
		ed.showMessage("Clipboard error: "+err.Error(), MsgError)
		return
	}

	content := string(output)
	
	// Remove BOM if present (UTF-8 BOM: EF BB BF)
	content = strings.TrimPrefix(content, "\xef\xbb\xbf")
	// Remove any leading/trailing whitespace
	content = strings.TrimSpace(content)
	// Normalize line endings to \n
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	
	if content == "" {
		ed.showMessage("Clipboard is empty", MsgError)
		return
	}

	// Parse the clipboard content - look for our format with separators
	hexPart := ""
	labelsPart := ""
	layoutPart := ""

	labelsMarker := "# ---- labels.toml -----------------------------------"
	layoutMarker := "# ---- layout.toml -----------------------------------"

	labelsIdx := strings.Index(content, labelsMarker)
	layoutIdx := strings.Index(content, layoutMarker)

	if labelsIdx == -1 || layoutIdx == -1 {
		// Try to parse as just hex records (legacy format)
		hexPart = strings.TrimSpace(content)
	} else {
		hexPart = strings.TrimSpace(content[:labelsIdx])
		labelsPart = strings.TrimSpace(content[labelsIdx+len(labelsMarker) : layoutIdx])
		layoutPart = strings.TrimSpace(content[layoutIdx+len(layoutMarker):])
	}

	// Validate hex format - check we have content and first char looks like hex
	if len(hexPart) < 4 {
		ed.showMessage("Invalid clipboard format (no hex data found)", MsgError)
		return
	}

	// Parse hex records
	records, err := fsmfile.ParseHex(hexPart)
	if err != nil {
		ed.showMessage("Invalid hex data: "+err.Error(), MsgError)
		return
	}

	if len(records) == 0 {
		ed.showMessage("No valid hex records found", MsgError)
		return
	}

	// Parse labels if present
	var labels *fsmfile.Labels
	if labelsPart != "" {
		labels, err = fsmfile.ParseLabels(labelsPart)
		if err != nil {
			ed.showMessage("Invalid labels: "+err.Error(), MsgError)
			return
		}
	}

	// Parse layout if present
	var layout *fsmfile.Layout
	if layoutPart != "" {
		layout, err = fsmfile.ParseLayout(layoutPart)
		if err != nil {
			// Layout errors are non-fatal, just ignore layout
			layout = nil
		}
	}

	// Convert records to FSM
	pastedFSM, err := fsmfile.RecordsToFSM(records, labels)
	if err != nil {
		ed.showMessage("Invalid FSM data: "+err.Error(), MsgError)
		return
	}

	if len(pastedFSM.States) == 0 {
		ed.showMessage("No states in clipboard data", MsgError)
		return
	}

	// Save current state for undo
	ed.saveSnapshot()

	// Build name mapping for conflicts (old name -> new name)
	stateRename := make(map[string]string)
	inputRename := make(map[string]string)
	outputRename := make(map[string]string)

	// Check for state name conflicts and generate new names
	existingStates := make(map[string]bool)
	for _, s := range ed.fsm.States {
		existingStates[s] = true
	}
	for _, s := range pastedFSM.States {
		if existingStates[s] {
			// Find a unique name
			newName := s
			for i := 1; existingStates[newName]; i++ {
				newName = fmt.Sprintf("%s_%d", s, i)
			}
			stateRename[s] = newName
			existingStates[newName] = true
		} else {
			stateRename[s] = s
			existingStates[s] = true
		}
	}

	// Check for input symbol conflicts
	existingInputs := make(map[string]bool)
	for _, a := range ed.fsm.Alphabet {
		existingInputs[a] = true
	}
	for _, a := range pastedFSM.Alphabet {
		if existingInputs[a] {
			// Input already exists, no rename needed (shared symbol)
			inputRename[a] = a
		} else {
			inputRename[a] = a
			// Add to existing FSM alphabet
			ed.fsm.Alphabet = append(ed.fsm.Alphabet, a)
		}
	}

	// Check for output symbol conflicts
	existingOutputs := make(map[string]bool)
	for _, o := range ed.fsm.OutputAlphabet {
		existingOutputs[o] = true
	}
	for _, o := range pastedFSM.OutputAlphabet {
		if existingOutputs[o] {
			outputRename[o] = o
		} else {
			outputRename[o] = o
			ed.fsm.OutputAlphabet = append(ed.fsm.OutputAlphabet, o)
		}
	}

	// Find bounds of existing states to place pasted content
	// Track the rightmost edge at each "row band" (group of Y coordinates)
	type rowBand struct {
		minY, maxY int
		maxX       int
	}
	var bands []rowBand

	// Group states into row bands (states within the height of a typical FSM of each other)
	for _, sp := range ed.states {
		// State box width: prefix (2) + "[" + name + "]" + suffix (1) + padding for labels
		// "→ [name]*" worst case, plus some space for transition labels
		rightEdge := sp.X + len(sp.Name) + 10
		found := false
		for i := range bands {
			// Use larger tolerance for band grouping to handle taller FSMs
			if sp.Y >= bands[i].minY-2 && sp.Y <= bands[i].maxY+2 {
				// Belongs to this band
				if sp.Y < bands[i].minY {
					bands[i].minY = sp.Y
				}
				if sp.Y > bands[i].maxY {
					bands[i].maxY = sp.Y
				}
				if rightEdge > bands[i].maxX {
					bands[i].maxX = rightEdge
				}
				found = true
				break
			}
		}
		if !found {
			bands = append(bands, rowBand{minY: sp.Y, maxY: sp.Y, maxX: rightEdge})
		}
	}

	// Calculate pasted FSM bounds from layout
	pastedMinX, pastedMinY := 0, 0
	pastedMaxX, pastedMaxY := 0, 0
	pastedWidth, pastedHeight := 0, 0
	if layout != nil && len(layout.States) > 0 {
		first := true
		for stateName, pos := range layout.States {
			// Account for full state box width plus padding
			stateWidth := len(stateName) + 10
			if first {
				pastedMinX, pastedMinY = pos.X, pos.Y
				pastedMaxX, pastedMaxY = pos.X+stateWidth, pos.Y
				first = false
			} else {
				if pos.X < pastedMinX {
					pastedMinX = pos.X
				}
				if pos.Y < pastedMinY {
					pastedMinY = pos.Y
				}
				if pos.X+stateWidth > pastedMaxX {
					pastedMaxX = pos.X + stateWidth
				}
				if pos.Y > pastedMaxY {
					pastedMaxY = pos.Y
				}
			}
		}
		pastedWidth = pastedMaxX - pastedMinX
		pastedHeight = pastedMaxY - pastedMinY + 1
	} else {
		// Estimate size for auto-layout (5 states per row, 15 chars apart)
		cols := 5
		rows := (len(pastedFSM.States) + cols - 1) / cols
		pastedWidth = cols * 15
		pastedHeight = rows * 4
	}

	// Find placement: try to fit in existing row bands first, then create new band below
	canvasWidthThreshold := 150
	var offsetX, offsetY int

	if len(ed.states) == 0 {
		// Empty canvas, place at default position
		offsetX = 5
		offsetY = 3
	} else {
		placed := false
		// Try each existing band
		for _, band := range bands {
			if band.maxX+6+pastedWidth <= canvasWidthThreshold {
				// Fits in this band - place right next to existing content
				offsetX = band.maxX + 6
				offsetY = band.minY
				placed = true
				break
			}
		}
		if !placed {
			// Create new row below all existing content
			maxY := 0
			for _, band := range bands {
				if band.maxY > maxY {
					maxY = band.maxY
				}
			}
			offsetX = 5
			// Place just below the previous row with small padding (2 rows gap)
			offsetY = maxY + 3
		}
	}
	_ = pastedHeight // Used for width threshold calculation

	// Add states with renamed names and adjusted positions
	statesAdded := 0
	for _, oldName := range pastedFSM.States {
		newName := stateRename[oldName]
		ed.fsm.States = append(ed.fsm.States, newName)

		// Determine position
		var posX, posY int
		if layout != nil {
			if pos, ok := layout.States[oldName]; ok {
				posX = pos.X - pastedMinX + offsetX
				posY = pos.Y - pastedMinY + offsetY
			} else {
				posX = offsetX + statesAdded*12
				posY = offsetY
			}
		} else {
			posX = offsetX + (statesAdded%5)*15
			posY = offsetY + (statesAdded/5)*4
		}

		ed.states = append(ed.states, StatePos{
			Name: newName,
			X:    posX,
			Y:    posY,
		})
		statesAdded++
	}

	// Add transitions with renamed states and symbols
	transAdded := 0
	for _, t := range pastedFSM.Transitions {
		newFrom := stateRename[t.From]
		newTo := make([]string, len(t.To))
		for i, to := range t.To {
			newTo[i] = stateRename[to]
		}

		var newInput *string
		if t.Input != nil {
			inp := inputRename[*t.Input]
			newInput = &inp
		}

		var newOutput *string
		if t.Output != nil {
			out := outputRename[*t.Output]
			newOutput = &out
		}

		ed.fsm.Transitions = append(ed.fsm.Transitions, fsm.Transition{
			From:   newFrom,
			Input:  newInput,
			To:     newTo,
			Output: newOutput,
		})
		transAdded++
	}

	// Add Moore state outputs with renamed states
	if pastedFSM.StateOutputs != nil {
		if ed.fsm.StateOutputs == nil {
			ed.fsm.StateOutputs = make(map[string]string)
		}
		for oldState, out := range pastedFSM.StateOutputs {
			newState := stateRename[oldState]
			ed.fsm.StateOutputs[newState] = outputRename[out]
		}
	}

	// Note: We don't merge accepting states or initial state from pasted FSM
	// as those are FSM-level properties, not additive

	ed.modified = true
	ed.selectedState = -1
	ed.selectedTrans = -1
	ed.mode = ModeCanvas
	ed.updateMenuItems()

	// Count renamed states for message
	renamed := 0
	for old, new := range stateRename {
		if old != new {
			renamed++
		}
	}

	msg := fmt.Sprintf("Pasted %d states, %d transitions", statesAdded, transAdded)
	if renamed > 0 {
		msg += fmt.Sprintf(" (%d renamed)", renamed)
	}
	ed.showMessage(msg, MsgSuccess)
}

func (ed *Editor) addStateAtCursor() {
	ed.inputPrompt = "State name: "
	ed.inputBuffer = fmt.Sprintf("S%d", len(ed.fsm.States))
	ed.inputAction = func(name string) {
		if name == "" {
			ed.mode = ModeCanvas
			return
		}
		// Check duplicate
		for _, s := range ed.fsm.States {
			if s == name {
				ed.showMessage("State already exists", MsgError)
				ed.mode = ModeCanvas
				return
			}
		}
		ed.saveSnapshot()
		ed.fsm.AddState(name)
		ed.states = append(ed.states, StatePos{
			Name: name,
			X:    ed.canvasCursorX,
			Y:    ed.canvasCursorY,
		})
		// Set as initial if first state
		if len(ed.fsm.States) == 1 {
			ed.fsm.SetInitial(name)
		}
		ed.modified = true
		ed.selectedState = len(ed.states) - 1
		ed.showMessage("Added state: "+name, MsgSuccess)
		ed.mode = ModeCanvas
	}
	ed.mode = ModeInput
}

// addStateAtPosition adds a new state at the specified canvas position (for right-click)
// Creates state immediately with auto-generated name, no prompt
func (ed *Editor) addStateAtPosition(posX, posY int) {
	// Generate unique state name
	name := fmt.Sprintf("S%d", len(ed.fsm.States))
	// Ensure uniqueness
	for {
		exists := false
		for _, s := range ed.fsm.States {
			if s == name {
				exists = true
				break
			}
		}
		if !exists {
			break
		}
		// Try next number
		name = fmt.Sprintf("S%d", len(ed.fsm.States)+1)
	}

	ed.saveSnapshot()
	ed.fsm.AddState(name)
	ed.states = append(ed.states, StatePos{
		Name: name,
		X:    posX,
		Y:    posY,
	})
	// Set as initial if first state
	if len(ed.fsm.States) == 1 {
		ed.fsm.SetInitial(name)
	}
	ed.modified = true
	ed.selectedState = len(ed.states) - 1
	ed.showMessage("Added state: "+name, MsgSuccess)
}

// editStateName allows renaming a state (for double-click)
func (ed *Editor) editStateName(stateIdx int) {
	if stateIdx < 0 || stateIdx >= len(ed.states) {
		return
	}
	oldName := ed.states[stateIdx].Name
	ed.inputPrompt = "Rename state: "
	ed.inputBuffer = oldName
	ed.inputAction = func(newName string) {
		if newName == "" || newName == oldName {
			ed.mode = ModeCanvas
			return
		}
		// Check duplicate
		for _, s := range ed.fsm.States {
			if s == newName {
				ed.showMessage("State already exists", MsgError)
				ed.mode = ModeCanvas
				return
			}
		}
		ed.saveSnapshot()

		// Update state name in FSM
		for i, s := range ed.fsm.States {
			if s == oldName {
				ed.fsm.States[i] = newName
				break
			}
		}

		// Update initial state if needed
		if ed.fsm.Initial == oldName {
			ed.fsm.Initial = newName
		}

		// Update accepting states
		for i, s := range ed.fsm.Accepting {
			if s == oldName {
				ed.fsm.Accepting[i] = newName
			}
		}

		// Update transitions
		for i := range ed.fsm.Transitions {
			if ed.fsm.Transitions[i].From == oldName {
				ed.fsm.Transitions[i].From = newName
			}
			// To is a slice
			for j, to := range ed.fsm.Transitions[i].To {
				if to == oldName {
					ed.fsm.Transitions[i].To[j] = newName
				}
			}
		}

		// Update Moore outputs (StateOutputs)
		if ed.fsm.StateOutputs != nil {
			if out, ok := ed.fsm.StateOutputs[oldName]; ok {
				delete(ed.fsm.StateOutputs, oldName)
				ed.fsm.StateOutputs[newName] = out
			}
		}

		// Update position record
		ed.states[stateIdx].Name = newName

		ed.modified = true
		ed.showMessage("Renamed: "+oldName+" → "+newName, MsgSuccess)
		ed.mode = ModeCanvas
	}
	ed.mode = ModeInput
}

func (ed *Editor) deleteSelected() {
	if ed.selectedState >= 0 && ed.selectedState < len(ed.states) {
		ed.saveSnapshot()
		name := ed.states[ed.selectedState].Name
		// Remove from FSM
		newStates := make([]string, 0)
		for _, s := range ed.fsm.States {
			if s != name {
				newStates = append(newStates, s)
			}
		}
		ed.fsm.States = newStates

		// Remove transitions involving this state
		newTrans := make([]fsm.Transition, 0)
		for _, t := range ed.fsm.Transitions {
			if t.From == name {
				continue
			}
			newTo := make([]string, 0)
			for _, to := range t.To {
				if to != name {
					newTo = append(newTo, to)
				}
			}
			if len(newTo) > 0 {
				t.To = newTo
				newTrans = append(newTrans, t)
			}
		}
		ed.fsm.Transitions = newTrans

		// Remove from accepting
		newAcc := make([]string, 0)
		for _, a := range ed.fsm.Accepting {
			if a != name {
				newAcc = append(newAcc, a)
			}
		}
		ed.fsm.Accepting = newAcc

		// Clear initial if it was this state
		if ed.fsm.Initial == name {
			ed.fsm.Initial = ""
			if len(newStates) > 0 {
				ed.fsm.Initial = newStates[0]
			}
		}

		// Remove from state outputs
		delete(ed.fsm.StateOutputs, name)

		// Remove from positions
		ed.states = append(ed.states[:ed.selectedState], ed.states[ed.selectedState+1:]...)
		ed.selectedState = -1
		ed.modified = true
		ed.showMessage("Deleted state: "+name, MsgSuccess)
	}
}

func (ed *Editor) cycleSelection() {
	if len(ed.states) == 0 {
		return
	}
	ed.selectedState++
	if ed.selectedState >= len(ed.states) {
		ed.selectedState = 0
	}
}

func (ed *Editor) startMoveMode() {
	if ed.selectedState < 0 || ed.selectedState >= len(ed.states) {
		return
	}
	// Use the same dragging mechanism as mouse, but keyboard-driven
	ed.saveSnapshot()
	ed.dragging = true
	ed.dragStateIdx = ed.selectedState
	ed.dragOffsetX = 0
	ed.dragOffsetY = 0
	// Store original position in move fields for Esc cancel
	ed.moveStateIdx = ed.selectedState
	ed.moveOrigX = ed.states[ed.selectedState].X
	ed.moveOrigY = ed.states[ed.selectedState].Y
	ed.mode = ModeMove
	ed.showMessage("Move: arrows, Enter=confirm, Esc=cancel", MsgInfo)
}

func (ed *Editor) handleMoveKey(ev *tcell.EventKey) bool {
	if ed.dragStateIdx < 0 || ed.dragStateIdx >= len(ed.states) {
		ed.dragging = false
		ed.mode = ModeCanvas
		return false
	}

	switch ev.Key() {
	case tcell.KeyEscape:
		// Restore original position and undo the snapshot
		ed.states[ed.dragStateIdx].X = ed.moveOrigX
		ed.states[ed.dragStateIdx].Y = ed.moveOrigY
		ed.dragging = false
		ed.mode = ModeCanvas
		// Pop the snapshot we saved
		if len(ed.undoStack) > 0 {
			ed.undoStack = ed.undoStack[:len(ed.undoStack)-1]
		}
		ed.showMessage("Move cancelled", MsgInfo)
	case tcell.KeyEnter:
		// Confirm move - snapshot already saved
		ed.dragging = false
		ed.modified = true
		ed.mode = ModeCanvas
		ed.showMessage("State moved", MsgInfo)
	case tcell.KeyUp:
		if ed.states[ed.dragStateIdx].Y > 0 {
			ed.states[ed.dragStateIdx].Y--
		}
	case tcell.KeyDown:
		ed.states[ed.dragStateIdx].Y++
	case tcell.KeyLeft:
		if ed.states[ed.dragStateIdx].X > 0 {
			ed.states[ed.dragStateIdx].X--
		}
	case tcell.KeyRight:
		ed.states[ed.dragStateIdx].X++
	}
	return false
}

func (ed *Editor) handleHelpKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyEnter:
		ed.helpScrollOffset = 0
		ed.mode = ModeCanvas
	case tcell.KeyUp:
		if ed.helpScrollOffset > 0 {
			ed.helpScrollOffset--
		}
	case tcell.KeyDown:
		// Allow scrolling down if there's more content
		h := 24
		if ed.screen != nil {
			_, h = ed.screen.Size()
		}
		visibleLines := h - 10 // approximate visible area
		if ed.helpScrollOffset < ed.helpTotalLines-visibleLines {
			ed.helpScrollOffset++
		}
	case tcell.KeyPgUp:
		ed.helpScrollOffset -= 10
		if ed.helpScrollOffset < 0 {
			ed.helpScrollOffset = 0
		}
	case tcell.KeyPgDn:
		h := 24
		if ed.screen != nil {
			_, h = ed.screen.Size()
		}
		visibleLines := h - 10
		ed.helpScrollOffset += 10
		if ed.helpScrollOffset > ed.helpTotalLines-visibleLines {
			ed.helpScrollOffset = ed.helpTotalLines - visibleLines
		}
		if ed.helpScrollOffset < 0 {
			ed.helpScrollOffset = 0
		}
	case tcell.KeyHome:
		ed.helpScrollOffset = 0
	case tcell.KeyEnd:
		h := 24
		if ed.screen != nil {
			_, h = ed.screen.Size()
		}
		visibleLines := h - 10
		ed.helpScrollOffset = ed.helpTotalLines - visibleLines
		if ed.helpScrollOffset < 0 {
			ed.helpScrollOffset = 0
		}
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'q', 'Q', 'h', 'H':
			ed.helpScrollOffset = 0
			ed.mode = ModeCanvas
		case 'j':
			h := 24
		if ed.screen != nil {
			_, h = ed.screen.Size()
		}
			visibleLines := h - 10
			if ed.helpScrollOffset < ed.helpTotalLines-visibleLines {
				ed.helpScrollOffset++
			}
		case 'k':
			if ed.helpScrollOffset > 0 {
				ed.helpScrollOffset--
			}
		}
	}
	return false
}

// findStateAtCursor returns the index of the state under the cursor, or -1 if none.
func (ed *Editor) findStateAtCursor() int {
	for i, sp := range ed.states {
		// State box starts at sp.X, sp.Y and has width of name + prefix/suffix chars
		stateW := len(sp.Name) + 4 // "○[name]" or "→[name]*"
		if ed.canvasCursorX >= sp.X && ed.canvasCursorX < sp.X+stateW && ed.canvasCursorY == sp.Y {
			return i
		}
	}
	return -1
}

func (ed *Editor) startAddTransition() {
	if len(ed.fsm.States) < 1 {
		ed.showMessage("Add states first", MsgError)
		return
	}
	if ed.selectedState < 0 || ed.selectedState >= len(ed.states) {
		ed.showMessage("Select a source state first (Tab to cycle)", MsgError)
		return
	}
	// Check for inputs - without at least epsilon, we can't create a transition
	if len(ed.fsm.Alphabet) == 0 {
		ed.showMessage("Add input symbols first (press I)", MsgError)
		return
	}
	// For Mealy machines, also need outputs
	if ed.fsm.Type == fsm.TypeMealy && len(ed.fsm.OutputAlphabet) == 0 {
		ed.showMessage("Mealy machines need output symbols (press O)", MsgError)
		return
	}

	// Build list of valid targets, excluding source if it already has a self-loop
	sourceState := ed.states[ed.selectedState].Name
	hasSelfLoop := false
	for _, t := range ed.fsm.Transitions {
		if t.From == sourceState {
			for _, to := range t.To {
				if to == sourceState {
					hasSelfLoop = true
					break
				}
			}
		}
		if hasSelfLoop {
			break
		}
	}

	ed.validTargets = nil
	for _, s := range ed.fsm.States {
		if s == sourceState && hasSelfLoop {
			continue // Skip source state if it already has a self-loop
		}
		ed.validTargets = append(ed.validTargets, s)
	}

	if len(ed.validTargets) == 0 {
		ed.showMessage("No valid target states available", MsgError)
		return
	}

	ed.menuSelected = 0
	ed.mode = ModeAddTransition
}

// Temporary storage for transition being built
func (ed *Editor) completeAddTransition() {
	if ed.selectedState < 0 || ed.selectedState >= len(ed.states) {
		ed.mode = ModeCanvas
		return
	}
	if ed.menuSelected < 0 || ed.menuSelected >= len(ed.validTargets) {
		ed.mode = ModeCanvas
		return
	}
	ed.pendingTransFrom = ed.states[ed.selectedState].Name
	ed.pendingTransTo = ed.validTargets[ed.menuSelected]

	// Proceed to select input (already validated in startAddTransition)
	ed.menuSelected = 0
	ed.mode = ModeSelectInput
}

func (ed *Editor) completeSelectInput() {
	var inputPtr *string
	if ed.menuSelected == len(ed.fsm.Alphabet) {
		// Epsilon selected
		inputPtr = nil
	} else {
		inp := ed.fsm.Alphabet[ed.menuSelected]
		inputPtr = &inp
	}

	if ed.fsm.Type == fsm.TypeMealy {
		// Need to select output (already validated in startAddTransition)
		ed.pendingInput = inputPtr
		ed.menuSelected = 0
		ed.mode = ModeSelectOutput
	} else {
		// Add transition
		ed.saveSnapshot()
		ed.fsm.AddTransition(ed.pendingTransFrom, inputPtr, []string{ed.pendingTransTo}, nil)
		ed.modified = true
		ed.showMessage(fmt.Sprintf("Added transition: %s -> %s", ed.pendingTransFrom, ed.pendingTransTo), MsgSuccess)
		ed.mode = ModeCanvas
	}
}

func (ed *Editor) completeSelectOutput() {
	out := ed.fsm.OutputAlphabet[ed.menuSelected]
	
	ed.saveSnapshot()
	if ed.mooreOutputMode {
		// Setting Moore output for a state
		if ed.selectedState >= 0 && ed.selectedState < len(ed.states) {
			name := ed.states[ed.selectedState].Name
			ed.fsm.SetStateOutput(name, out)
			ed.modified = true
			ed.showMessage(fmt.Sprintf("Set %s output to %s", name, out), MsgSuccess)
		}
		ed.mooreOutputMode = false
	} else {
		// Adding Mealy transition output
		ed.fsm.AddTransition(ed.pendingTransFrom, ed.pendingInput, []string{ed.pendingTransTo}, &out)
		ed.modified = true
		ed.showMessage(fmt.Sprintf("Added transition: %s -> %s", ed.pendingTransFrom, ed.pendingTransTo), MsgSuccess)
	}
	ed.mode = ModeCanvas
}

func (ed *Editor) addInput() {
	ed.inputPrompt = "Input symbol: "
	ed.inputBuffer = ""
	ed.inputAction = func(name string) {
		if name == "" {
			ed.mode = ModeCanvas
			return
		}
		ed.saveSnapshot()
		ed.fsm.AddInput(name)
		ed.modified = true
		ed.showMessage("Added input: "+name, MsgSuccess)
		ed.mode = ModeCanvas
	}
	ed.mode = ModeInput
}

func (ed *Editor) addOutput() {
	// Warn if not in Mealy/Moore mode
	if ed.fsm.Type != fsm.TypeMealy && ed.fsm.Type != fsm.TypeMoore {
		ed.showMessage("Outputs only used in Mealy/Moore machines", MsgError)
		return
	}
	ed.inputPrompt = "Output symbol: "
	ed.inputBuffer = ""
	ed.inputAction = func(name string) {
		if name == "" {
			ed.mode = ModeCanvas
			return
		}
		ed.saveSnapshot()
		ed.fsm.AddOutput(name)
		ed.modified = true
		ed.showMessage("Added output: "+name, MsgSuccess)
		ed.mode = ModeCanvas
	}
	ed.mode = ModeInput
}

func (ed *Editor) setInitialState() {
	if ed.selectedState >= 0 && ed.selectedState < len(ed.states) {
		ed.saveSnapshot()
		name := ed.states[ed.selectedState].Name
		ed.fsm.SetInitial(name)
		ed.modified = true
		ed.showMessage("Initial state: "+name, MsgSuccess)
	}
}

func (ed *Editor) toggleAccepting() {
	if ed.selectedState >= 0 && ed.selectedState < len(ed.states) {
		ed.saveSnapshot()
		name := ed.states[ed.selectedState].Name
		isAcc := false
		for _, a := range ed.fsm.Accepting {
			if a == name {
				isAcc = true
				break
			}
		}
		if isAcc {
			// Remove from accepting
			newAcc := make([]string, 0)
			for _, a := range ed.fsm.Accepting {
				if a != name {
					newAcc = append(newAcc, a)
				}
			}
			ed.fsm.Accepting = newAcc
			ed.showMessage(name+" is no longer accepting", MsgInfo)
		} else {
			ed.fsm.Accepting = append(ed.fsm.Accepting, name)
			ed.showMessage(name+" is now accepting", MsgSuccess)
		}
		ed.modified = true
	}
}

func (ed *Editor) setMooreOutput() {
	if len(ed.fsm.OutputAlphabet) == 0 {
		ed.showMessage("Add output symbols first (press 'o')", MsgError)
		return
	}
	ed.mooreOutputMode = true
	ed.menuSelected = 0
	ed.mode = ModeSelectOutput
}

// setLinkedMachine sets or toggles the linked machine for a state
func (ed *Editor) setLinkedMachine() {
	if ed.selectedState < 0 || ed.selectedState >= len(ed.states) {
		ed.showMessage("No state selected", MsgError)
		return
	}
	
	name := ed.states[ed.selectedState].Name
	
	// If already linked, offer to unlink
	if ed.fsm.IsLinked(name) {
		targetMachine := ed.fsm.GetLinkedMachine(name)
		ed.mode = ModeInput
		ed.inputPrompt = fmt.Sprintf("Unlink %s from %s? (y/n): ", name, targetMachine)
		ed.inputBuffer = ""
		ed.inputAction = func(answer string) {
			if answer == "y" || answer == "Y" {
				ed.saveSnapshot()
				ed.fsm.SetLinkedMachine(name, "")
				ed.showMessage(name+" unlinked", MsgSuccess)
				ed.modified = true
			}
			ed.mode = ModeCanvas
		}
		return
	}
	
	// If in a bundle, show available machines to link to
	if ed.isBundle && len(ed.bundleMachines) > 1 {
		// Filter out current machine
		availableMachines := make([]string, 0)
		for _, m := range ed.bundleMachines {
			if m != ed.currentMachine {
				availableMachines = append(availableMachines, m)
			}
		}
		
		if len(availableMachines) == 0 {
			ed.offerCreateMachineForLink(name)
			return
		}
		
		// Start machine selection mode
		ed.linkTargetMachines = availableMachines
		ed.linkTargetSelected = 0
		ed.mode = ModeSelectLinkTarget
		ed.showMessage("Select target machine for "+name, MsgInfo)
		return
	}
	
	// Not in bundle — offer to create a bundle with a new machine.
	ed.offerCreateMachineForLink(name)
}


func (ed *Editor) showMessage(msg string, msgType MessageType) {
	ed.message = msg
	ed.messageType = msgType
	ed.messageFlashStart = time.Now().UnixMilli()
	// Trigger immediate refresh for flash animation
	if ed.screen != nil {
		ed.screen.PostEvent(tcell.NewEventInterrupt(nil))
	}
}

func (ed *Editor) fsmTypeIndex() int {
	switch ed.fsm.Type {
	case fsm.TypeDFA:
		return 0
	case fsm.TypeNFA:
		return 1
	case fsm.TypeMoore:
		return 2
	case fsm.TypeMealy:
		return 3
	}
	return 0
}



// Fix for Moore output selection mode
func (ed *Editor) completeSelectOutputMoore() {
	if ed.selectedState >= 0 && ed.selectedState < len(ed.states) {
		name := ed.states[ed.selectedState].Name
		out := ed.fsm.OutputAlphabet[ed.menuSelected]
		ed.fsm.SetStateOutput(name, out)
		ed.modified = true
		ed.showMessage(fmt.Sprintf("Set %s output to %s", name, out), MsgSuccess)
	}
	ed.mode = ModeCanvas
}
