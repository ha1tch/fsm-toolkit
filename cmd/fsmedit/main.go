// Command fsmedit is a TUI editor for finite state machines.
package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
	"github.com/ha1tch/fsm-toolkit/pkg/fsmfile"
)

// Editor holds all editor state
type Editor struct {
	screen      tcell.Screen
	fsm         *fsm.FSM
	filename    string
	modified    bool
	mode        Mode
	message     string
	messageType MessageType

	// Canvas state
	canvasCursorX int
	canvasCursorY int
	canvasOffsetX int
	canvasOffsetY int
	states        []StatePos // states with positions

	// Selection
	selectedState int // -1 = none
	selectedTrans int // -1 = none

	// Dragging state (mouse)
	dragging      bool
	dragStateIdx  int
	dragOffsetX   int // offset from mouse to state origin
	dragOffsetY   int

	// Move mode state (keyboard)
	moveStateIdx int // state being moved
	moveOrigX    int // original position for undo
	moveOrigY    int

	// Display options
	showArcs bool // toggle arc visibility with 'w'

	// Undo/Redo
	undoStack []Snapshot
	redoStack []Snapshot

	// UI regions
	canvasWidth  int
	canvasHeight int
	sidebarWidth int

	// Menu state
	menuItems    []string
	menuSelected int

	// Input state
	inputBuffer string
	inputPrompt string
	inputAction func(string)

	// File picker state
	fileList     []string
	fileSelected int
}

// Snapshot captures editor state for undo/redo
type Snapshot struct {
	FSM    *fsm.FSM
	States []StatePos
}

// StatePos tracks state position on canvas
type StatePos struct {
	Name string
	X, Y int
}

// Mode represents editor mode
type Mode int

const (
	ModeMenu Mode = iota
	ModeCanvas
	ModeInput
	ModeFilePicker
	ModeSelectType
	ModeAddTransition
	ModeSelectInput
	ModeSelectOutput
	ModeMove // keyboard-driven state movement
)

// MessageType for status messages
type MessageType int

const (
	MsgInfo MessageType = iota
	MsgError
	MsgSuccess
)

func main() {
	ed := &Editor{
		fsm:           fsm.New(fsm.TypeDFA),
		selectedState: -1,
		selectedTrans: -1,
		sidebarWidth:  30,
		states:        make([]StatePos, 0),
	}

	// Check command line
	if len(os.Args) > 1 {
		ed.filename = os.Args[1]
		if err := ed.loadFile(ed.filename); err != nil {
			fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", ed.filename, err)
			os.Exit(1)
		}
	}

	// Initialize screen
	screen, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating screen: %v\n", err)
		os.Exit(1)
	}
	if err := screen.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing screen: %v\n", err)
		os.Exit(1)
	}
	screen.EnableMouse()
	screen.Clear()

	ed.screen = screen
	ed.mode = ModeMenu
	ed.showArcs = true // arcs visible by default
	ed.menuItems = []string{
		"New FSM",
		"Open File",
		"Save",
		"Save As",
		"Edit Canvas",
		"Set FSM Type",
		"Quit",
	}

	// Main loop
	ed.run()

	screen.Fini()
}

func (ed *Editor) run() {
	for {
		ed.draw()
		ed.screen.Show()

		ev := ed.screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			ed.screen.Sync()
		case *tcell.EventKey:
			if ed.handleKey(ev) {
				return
			}
		case *tcell.EventMouse:
			ed.handleMouse(ev)
		}
	}
}

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

	if isCtrlOrCmd(tcell.KeyCtrlC, 'c') {
		ed.copyToClipboard()
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
	switch ed.menuItems[ed.menuSelected] {
	case "New FSM":
		ed.newFSM()
	case "Open File":
		ed.openFilePicker()
	case "Save":
		ed.save()
	case "Save As":
		ed.saveAs()
	case "Edit Canvas":
		ed.mode = ModeCanvas
	case "Set FSM Type":
		ed.mode = ModeSelectType
		ed.menuSelected = int(ed.fsmTypeIndex())
	case "Quit":
		if ed.modified {
			ed.inputPrompt = "Unsaved changes. Quit anyway? (y/n): "
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

func (ed *Editor) handleCanvasKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		ed.mode = ModeMenu
		ed.selectedState = -1
	case tcell.KeyUp:
		if ed.canvasCursorY > 0 {
			ed.canvasCursorY--
		}
	case tcell.KeyDown:
		ed.canvasCursorY++
	case tcell.KeyLeft:
		if ed.canvasCursorX > 0 {
			ed.canvasCursorX--
		}
	case tcell.KeyRight:
		ed.canvasCursorX++
	case tcell.KeyEnter:
		ed.addStateAtCursor()
	case tcell.KeyDelete, tcell.KeyBackspace, tcell.KeyBackspace2:
		ed.deleteSelected()
	case tcell.KeyTab:
		ed.cycleSelection()
	case tcell.KeyRune:
		switch ev.Rune() {
		case 't', 'T':
			if ed.selectedState >= 0 {
				ed.startAddTransition()
			}
		case 'i', 'I':
			ed.addInput()
		case 'o', 'O':
			ed.addOutput()
		case 's', 'S':
			ed.setInitialState()
		case 'a', 'A':
			ed.toggleAccepting()
		case 'm', 'M':
			if ed.fsm.Type == fsm.TypeMoore && ed.selectedState >= 0 {
				ed.setMooreOutput()
			}
		case 'w', 'W':
			ed.showArcs = !ed.showArcs
			if ed.showArcs {
				ed.showMessage("Arcs visible", MsgInfo)
			} else {
				ed.showMessage("Arcs hidden", MsgInfo)
			}
		case 'g', 'G':
			if ed.selectedState >= 0 {
				ed.startMoveMode()
			} else {
				ed.showMessage("Select a state first (Tab to cycle)", MsgInfo)
			}
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
		ed.mode = ModeMenu
	case tcell.KeyUp:
		if ed.fileSelected > 0 {
			ed.fileSelected--
		}
	case tcell.KeyDown:
		if ed.fileSelected < len(ed.fileList)-1 {
			ed.fileSelected++
		}
	case tcell.KeyEnter:
		if len(ed.fileList) > 0 {
			ed.filename = ed.fileList[ed.fileSelected]
			if err := ed.loadFile(ed.filename); err != nil {
				ed.showMessage("Error: "+err.Error(), MsgError)
			} else {
				ed.showMessage("Loaded: "+ed.filename, MsgSuccess)
				ed.mode = ModeCanvas
			}
		}
	}
	return false
}

func (ed *Editor) handleSelectTypeKey(ev *tcell.EventKey) bool {
	types := []fsm.Type{fsm.TypeDFA, fsm.TypeNFA, fsm.TypeMoore, fsm.TypeMealy}
	switch ev.Key() {
	case tcell.KeyEscape:
		ed.mode = ModeMenu
	case tcell.KeyUp:
		if ed.menuSelected > 0 {
			ed.menuSelected--
		}
	case tcell.KeyDown:
		if ed.menuSelected < len(types)-1 {
			ed.menuSelected++
		}
	case tcell.KeyEnter:
		ed.fsm.Type = types[ed.menuSelected]
		ed.modified = true
		ed.showMessage("FSM type set to "+string(ed.fsm.Type), MsgSuccess)
		ed.mode = ModeMenu
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
		if ed.menuSelected < len(ed.fsm.States)-1 {
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

func (ed *Editor) handleMouse(ev *tcell.EventMouse) {
	x, y := ev.Position()
	buttons := ev.Buttons()

	w, h := ed.screen.Size()
	canvasW := w - ed.sidebarWidth

	// Handle drag release
	if ed.dragging && buttons&tcell.Button3 == 0 {
		// Right button released - drop the state
		ed.dragging = false
		ed.modified = true
		ed.showMessage("State moved", MsgSuccess)
		return
	}

	// Handle ongoing drag
	if ed.dragging && buttons&tcell.Button3 != 0 {
		// Update state position while dragging
		if ed.dragStateIdx >= 0 && ed.dragStateIdx < len(ed.states) {
			newX := x - ed.dragOffsetX + ed.canvasOffsetX
			newY := y - ed.dragOffsetY + ed.canvasOffsetY
			if newX < 0 {
				newX = 0
			}
			if newY < 0 {
				newY = 0
			}
			ed.states[ed.dragStateIdx].X = newX
			ed.states[ed.dragStateIdx].Y = newY
		}
		return
	}

	// Right button pressed - start drag if on a state
	if buttons&tcell.Button3 != 0 && !ed.dragging && ed.mode == ModeCanvas {
		if x < canvasW && y < h-2 {
			// Check if clicking on a state
			for i, sp := range ed.states {
				stateX := sp.X - ed.canvasOffsetX
				stateY := sp.Y - ed.canvasOffsetY
				stateW := len(sp.Name) + 4 // account for prefix/suffix chars
				
				if x >= stateX && x < stateX+stateW && y == stateY {
					ed.saveSnapshot()
					ed.dragging = true
					ed.dragStateIdx = i
					ed.dragOffsetX = x - stateX
					ed.dragOffsetY = y - stateY
					ed.selectedState = i
					return
				}
			}
		}
	}

	// Left button handling
	if buttons&tcell.Button1 != 0 {
		if ed.mode == ModeMenu || ed.mode == ModeFilePicker || ed.mode == ModeSelectType {
			// Click on menu item
			if x >= 10 && x < 40 && y >= 5 {
				idx := y - 5
				switch ed.mode {
				case ModeMenu:
					if idx >= 0 && idx < len(ed.menuItems) {
						ed.menuSelected = idx
						ed.executeMenuItem()
					}
				case ModeFilePicker:
					if idx >= 0 && idx < len(ed.fileList) {
						ed.fileSelected = idx
					}
				case ModeSelectType:
					if idx >= 0 && idx < 4 {
						ed.menuSelected = idx
					}
				}
			}
		} else if ed.mode == ModeCanvas {
			if x < canvasW && y < h-2 {
				// Click on canvas
				ed.canvasCursorX = x + ed.canvasOffsetX
				ed.canvasCursorY = y + ed.canvasOffsetY

				// Check if clicking on a state
				ed.selectedState = -1
				for i, sp := range ed.states {
					stateX := sp.X - ed.canvasOffsetX
					stateY := sp.Y - ed.canvasOffsetY
					stateW := len(sp.Name) + 4
					
					if x >= stateX && x < stateX+stateW && y == stateY {
						ed.selectedState = i
						break
					}
				}
			} else if x >= canvasW {
				// Click on sidebar - could select items there
			}
		}
	} else if buttons&tcell.Button1 != 0 {
		// Single click on canvas - could add more logic here
		if ed.mode == ModeCanvas && x < canvasW && y < h-2 {
			ed.canvasCursorX = x
			ed.canvasCursorY = y
		}
	}
}

// Actions

func (ed *Editor) newFSM() {
	ed.inputPrompt = "FSM Name (optional): "
	ed.inputBuffer = ""
	ed.inputAction = func(name string) {
		ed.fsm = fsm.New(fsm.TypeDFA)
		ed.fsm.Name = name
		ed.filename = ""
		ed.modified = true
		ed.states = make([]StatePos, 0)
		ed.selectedState = -1
		ed.showMessage("New FSM created", MsgSuccess)
		ed.mode = ModeCanvas
	}
	ed.mode = ModeInput
}

func (ed *Editor) openFilePicker() {
	files, _ := filepath.Glob("*.fsm")
	jsonFiles, _ := filepath.Glob("*.json")
	ed.fileList = append(files, jsonFiles...)
	if len(ed.fileList) == 0 {
		ed.showMessage("No .fsm or .json files in current directory", MsgError)
		return
	}
	ed.fileSelected = 0
	ed.mode = ModeFilePicker
}

func (ed *Editor) save() {
	if ed.filename == "" {
		ed.saveAs()
		return
	}
	if err := ed.saveFile(ed.filename); err != nil {
		ed.showMessage("Error: "+err.Error(), MsgError)
	} else {
		ed.modified = false
		ed.showMessage("Saved: "+ed.filename, MsgSuccess)
	}
}

func (ed *Editor) copyToClipboard() {
	// Generate hex representation of the FSM
	records, _, _, _ := fsmfile.FSMToRecords(ed.fsm)
	hex := fsmfile.FormatHex(records, 1) // width=1 means one record per line
	
	// Use OSC 52 escape sequence to copy to system clipboard
	// This works in most modern terminals (iTerm2, kitty, alacritty, Windows Terminal, etc.)
	// Format: ESC ] 52 ; c ; <base64-encoded-text> BEL
	encoded := base64.StdEncoding.EncodeToString([]byte(hex))
	osc52 := fmt.Sprintf("\x1b]52;c;%s\x07", encoded)
	
	// Write directly to terminal (bypassing tcell temporarily)
	os.Stdout.WriteString(osc52)
	
	ed.showMessage(fmt.Sprintf("Copied %d hex records to clipboard", len(records)), MsgSuccess)
}

func (ed *Editor) saveAs() {
	ed.inputPrompt = "Save as: "
	ed.inputBuffer = ed.filename
	ed.inputAction = func(name string) {
		if name == "" {
			ed.showMessage("Cancelled", MsgInfo)
			ed.mode = ModeMenu
			return
		}
		// Add .fsm extension if none
		if filepath.Ext(name) == "" {
			name += ".fsm"
		}
		ed.filename = name
		if err := ed.saveFile(ed.filename); err != nil {
			ed.showMessage("Error: "+err.Error(), MsgError)
		} else {
			ed.modified = false
			ed.showMessage("Saved: "+ed.filename, MsgSuccess)
		}
		ed.mode = ModeMenu
	}
	ed.mode = ModeInput
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
	// Save original position for undo
	ed.moveStateIdx = ed.selectedState
	ed.moveOrigX = ed.states[ed.selectedState].X
	ed.moveOrigY = ed.states[ed.selectedState].Y
	ed.mode = ModeMove
	ed.showMessage("Move: arrows to move, Enter to confirm, Esc to cancel", MsgInfo)
}

func (ed *Editor) handleMoveKey(ev *tcell.EventKey) bool {
	if ed.moveStateIdx < 0 || ed.moveStateIdx >= len(ed.states) {
		ed.mode = ModeCanvas
		return false
	}

	switch ev.Key() {
	case tcell.KeyEscape:
		// Restore original position
		ed.states[ed.moveStateIdx].X = ed.moveOrigX
		ed.states[ed.moveStateIdx].Y = ed.moveOrigY
		ed.mode = ModeCanvas
		ed.showMessage("Move cancelled", MsgInfo)
	case tcell.KeyEnter:
		// Confirm move - save to undo stack
		ed.saveSnapshot()
		ed.modified = true
		ed.mode = ModeCanvas
		ed.showMessage("State moved", MsgSuccess)
	case tcell.KeyUp:
		if ed.states[ed.moveStateIdx].Y > 0 {
			ed.states[ed.moveStateIdx].Y--
		}
	case tcell.KeyDown:
		ed.states[ed.moveStateIdx].Y++
	case tcell.KeyLeft:
		if ed.states[ed.moveStateIdx].X > 0 {
			ed.states[ed.moveStateIdx].X--
		}
	case tcell.KeyRight:
		ed.states[ed.moveStateIdx].X++
	}
	return false
}

func (ed *Editor) startAddTransition() {
	if len(ed.fsm.States) < 2 && ed.selectedState >= len(ed.fsm.States) {
		ed.showMessage("Need at least one target state", MsgError)
		return
	}
	ed.menuSelected = 0
	ed.mode = ModeAddTransition
}

// Temporary storage for transition being built
var pendingTransFrom string
var pendingTransTo string

func (ed *Editor) completeAddTransition() {
	if ed.selectedState < 0 || ed.selectedState >= len(ed.states) {
		ed.mode = ModeCanvas
		return
	}
	pendingTransFrom = ed.states[ed.selectedState].Name
	pendingTransTo = ed.fsm.States[ed.menuSelected]

	// Now select input
	if len(ed.fsm.Alphabet) == 0 {
		ed.showMessage("Add input symbols first (press 'i')", MsgError)
		ed.mode = ModeCanvas
		return
	}
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
		// Need to select output
		if len(ed.fsm.OutputAlphabet) == 0 {
			ed.showMessage("Add output symbols first (press 'o')", MsgError)
			ed.mode = ModeCanvas
			return
		}
		// Store input and go to output selection
		pendingInput = inputPtr
		ed.menuSelected = 0
		ed.mode = ModeSelectOutput
	} else {
		// Add transition
		ed.saveSnapshot()
		ed.fsm.AddTransition(pendingTransFrom, inputPtr, []string{pendingTransTo}, nil)
		ed.modified = true
		ed.showMessage(fmt.Sprintf("Added transition: %s -> %s", pendingTransFrom, pendingTransTo), MsgSuccess)
		ed.mode = ModeCanvas
	}
}

var pendingInput *string
var mooreOutputMode bool

func (ed *Editor) completeSelectOutput() {
	out := ed.fsm.OutputAlphabet[ed.menuSelected]
	
	ed.saveSnapshot()
	if mooreOutputMode {
		// Setting Moore output for a state
		if ed.selectedState >= 0 && ed.selectedState < len(ed.states) {
			name := ed.states[ed.selectedState].Name
			ed.fsm.SetStateOutput(name, out)
			ed.modified = true
			ed.showMessage(fmt.Sprintf("Set %s output to %s", name, out), MsgSuccess)
		}
		mooreOutputMode = false
	} else {
		// Adding Mealy transition output
		ed.fsm.AddTransition(pendingTransFrom, pendingInput, []string{pendingTransTo}, &out)
		ed.modified = true
		ed.showMessage(fmt.Sprintf("Added transition: %s -> %s", pendingTransFrom, pendingTransTo), MsgSuccess)
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
	mooreOutputMode = true
	ed.menuSelected = 0
	ed.mode = ModeSelectOutput
}

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

func (ed *Editor) showMessage(msg string, msgType MessageType) {
	ed.message = msg
	ed.messageType = msgType
}

// File operations

func (ed *Editor) loadFile(path string) error {
	ext := filepath.Ext(path)

	var f *fsm.FSM
	var layout *fsmfile.Layout
	var err error

	switch ext {
	case ".fsm":
		f, layout, err = fsmfile.ReadFSMFileWithLayout(path)
	case ".json":
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		f, err = fsmfile.ParseJSON(data)
	case ".hex":
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		records, perr := fsmfile.ParseHex(string(data))
		if perr != nil {
			return perr
		}
		f, err = fsmfile.RecordsToFSM(records, nil)
	default:
		return fmt.Errorf("unknown format: %s", ext)
	}

	if err != nil {
		return err
	}

	ed.fsm = f
	ed.modified = false

	// Apply layout if present, otherwise generate default positions
	ed.states = make([]StatePos, len(f.States))
	
	if layout != nil && len(layout.States) > 0 {
		// Use saved positions
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
				// Fallback for states not in layout
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
		// Generate smart layout based on FSM structure
		// Use canvas dimensions for layout calculation
		w, h := 80, 24 // default terminal size estimate
		if ed.screen != nil {
			w, h = ed.screen.Size()
			w = w - ed.sidebarWidth - 5 // account for sidebar
			h = h - 4                    // account for status bars
		}
		
		autoPositions := fsmfile.SmartLayout(f, w, h)
		for i, name := range f.States {
			if pos, ok := autoPositions[name]; ok {
				ed.states[i] = StatePos{
					Name: name,
					X:    pos[0],
					Y:    pos[1],
				}
			} else {
				// Fallback
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
	
	ed.selectedState = -1
	return nil
}

func (ed *Editor) saveFile(path string) error {
	ext := filepath.Ext(path)
	
	// Build positions map
	positions := make(map[string][2]int)
	for _, sp := range ed.states {
		positions[sp.Name] = [2]int{sp.X, sp.Y}
	}
	
	switch ext {
	case ".fsm":
		return fsmfile.WriteFSMFileWithLayout(path, ed.fsm, true, positions, ed.canvasOffsetX, ed.canvasOffsetY)
	case ".json":
		data, err := fsmfile.ToJSON(ed.fsm, true)
		if err != nil {
			return err
		}
		return os.WriteFile(path, data, 0644)
	default:
		return fsmfile.WriteFSMFileWithLayout(path, ed.fsm, true, positions, ed.canvasOffsetX, ed.canvasOffsetY)
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

