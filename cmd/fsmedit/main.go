// Command fsmedit is a TUI editor for finite state machines.
package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
	"github.com/ha1tch/fsm-toolkit/pkg/fsmfile"
)

// Config holds persistent editor settings
type Config struct {
	Renderer    string // "native" or "graphviz"
	FileType    string // "png" or "svg"
	LastDir     string // last used directory
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	cwd, _ := os.Getwd()
	return Config{
		Renderer: "native",
		FileType: "png",
		LastDir:  cwd,
	}
}

// ConfigPath returns the path to the config file
func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".fsmedit"
	}
	return filepath.Join(home, ".fsmedit")
}

// LoadConfig loads configuration from TOML file
func LoadConfig() Config {
	cfg := DefaultConfig()
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		return cfg
	}
	
	// Simple TOML parser for our settings
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "renderer") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"")
				if val == "native" || val == "graphviz" {
					cfg.Renderer = val
				}
			}
		} else if strings.HasPrefix(line, "file_type") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"")
				if val == "png" || val == "svg" {
					cfg.FileType = val
				}
			}
		} else if strings.HasPrefix(line, "last_dir") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"")
				if val != "" {
					cfg.LastDir = val
				}
			}
		}
	}
	return cfg
}

// SaveConfig saves configuration to TOML file
func SaveConfig(cfg Config) error {
	content := fmt.Sprintf("# fsmedit configuration\nrenderer = \"%s\"\nfile_type = \"%s\"\nlast_dir = \"%s\"\n", 
		cfg.Renderer, cfg.FileType, cfg.LastDir)
	return os.WriteFile(ConfigPath(), []byte(content), 0644)
}

// Editor holds all editor state
type Editor struct {
	screen      tcell.Screen
	fsm         *fsm.FSM
	filename    string
	modified    bool
	mode        Mode
	message     string
	messageType MessageType
	config      Config

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

	// Left-button drag detection
	leftMouseDown    bool
	leftDownX        int
	leftDownY        int
	leftDownStateIdx int // state under cursor when left button pressed

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
	fileList        []string
	fileSelected    int
	dirList         []string
	dirSelected     int
	currentDir      string
	filePickerFocus int // 0 = directories, 1 = files
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
		config:        LoadConfig(),
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
	ed.showArcs = true // arcs visible by default
	ed.updateMenuItems()

	// If file was loaded from command line, go straight to canvas
	if ed.filename != "" && len(ed.states) > 0 {
		ed.mode = ModeCanvas
	} else {
		ed.mode = ModeMenu
	}

	// Main loop
	ed.run()

	screen.Fini()
}

func (ed *Editor) updateMenuItems() {
	rendererLabel := "Renderer: Native"
	if ed.config.Renderer == "graphviz" {
		rendererLabel = "Renderer: Graphviz"
	}
	
	fileTypeLabel := "File Type: PNG"
	if ed.config.FileType == "svg" {
		fileTypeLabel = "File Type: SVG"
	}
	
	ed.menuItems = []string{
		"New FSM",
		"Open File",
		"Save",
		"Save As",
		"Edit Canvas",
		"Render",
		rendererLabel,
		fileTypeLabel,
		"Set FSM Type",
		"Quit",
	}
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
	item := ed.menuItems[ed.menuSelected]
	
	switch {
	case item == "New FSM":
		ed.newFSM()
	case item == "Open File":
		ed.openFilePicker()
	case item == "Save":
		ed.save()
	case item == "Save As":
		ed.saveAs()
	case item == "Edit Canvas":
		ed.mode = ModeCanvas
	case item == "Render":
		ed.renderView()
	case strings.HasPrefix(item, "Renderer:"):
		ed.toggleRenderer()
	case strings.HasPrefix(item, "File Type:"):
		ed.toggleFileType()
	case item == "Set FSM Type":
		ed.mode = ModeSelectType
		ed.menuSelected = int(ed.fsmTypeIndex())
	case item == "Quit":
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
			// Check if cursor is on a state - if so, select it first
			stateUnderCursor := ed.findStateAtCursor()
			if stateUnderCursor >= 0 {
				ed.selectedState = stateUnderCursor
			}
			if ed.selectedState >= 0 {
				ed.startMoveMode()
			} else {
				ed.showMessage("Select a state first (Tab to cycle)", MsgInfo)
			}
		case 'l', 'L':
			ed.runAnalysis()
		case 'v', 'V':
			ed.runValidate()
		case 'r', 'R':
			ed.renderView()
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
	case tcell.KeyTab:
		// Switch focus between directories and files
		ed.filePickerFocus = 1 - ed.filePickerFocus
	case tcell.KeyLeft:
		ed.filePickerFocus = 0 // Focus directories
	case tcell.KeyRight:
		ed.filePickerFocus = 1 // Focus files
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
		if ed.filePickerFocus == 0 {
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
			// Open selected file
			if len(ed.fileList) > 0 {
				fullPath := filepath.Join(ed.currentDir, ed.fileList[ed.fileSelected])
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

	// Handle drag release (either button)
	if ed.dragging && buttons&tcell.Button3 == 0 && buttons&tcell.Button1 == 0 {
		ed.dragging = false
		ed.modified = true
		ed.showMessage("State moved", MsgSuccess)
		ed.leftMouseDown = false
		return
	}

	// Handle ongoing drag (either button)
	if ed.dragging {
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

	// Right button pressed - start drag if on a state (legacy support)
	if buttons&tcell.Button3 != 0 && !ed.dragging && ed.mode == ModeCanvas {
		if x < canvasW && y < h-2 {
			for i, sp := range ed.states {
				stateX := sp.X - ed.canvasOffsetX
				stateY := sp.Y - ed.canvasOffsetY
				stateW := len(sp.Name) + 4

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
		if ed.mode == ModeMenu || ed.mode == ModeSelectType {
			// Click on menu item
			menuW, menuH := 40, len(ed.menuItems)+4
			startX := (w - menuW) / 2
			startY := (h - menuH) / 2
			if x >= startX+1 && x < startX+menuW-1 && y >= startY+2 {
				idx := y - startY - 2
				switch ed.mode {
				case ModeMenu:
					if idx >= 0 && idx < len(ed.menuItems) {
						ed.menuSelected = idx
						ed.executeMenuItem()
					}
				case ModeSelectType:
					if idx >= 0 && idx < 4 {
						ed.menuSelected = idx
					}
				}
			}
		} else if ed.mode == ModeFilePicker {
			// Two-column file picker mouse handling
			totalW := 80
			if totalW > w-4 {
				totalW = w - 4
			}
			dirW := totalW / 3
			boxX := (w - totalW) / 2
			boxY := 2
			
			// Check if click is in directories column
			if x >= boxX+1 && x < boxX+dirW && y >= boxY+5 {
				idx := y - boxY - 5
				if idx >= 0 && idx < len(ed.dirList) {
					ed.filePickerFocus = 0
					ed.dirSelected = idx
				}
			}
			// Check if click is in files column
			if x >= boxX+dirW+1 && x < boxX+totalW-1 && y >= boxY+5 {
				idx := y - boxY - 5
				if idx >= 0 && idx < len(ed.fileList) {
					ed.filePickerFocus = 1
					ed.fileSelected = idx
				}
			}
		} else if ed.mode == ModeCanvas {
			if x < canvasW && y < h-2 {
				if !ed.leftMouseDown {
					// Mouse just pressed - record position
					ed.leftMouseDown = true
					ed.leftDownX = x
					ed.leftDownY = y
					ed.leftDownStateIdx = -1

					// Check if pressing on a state
					for i, sp := range ed.states {
						stateX := sp.X - ed.canvasOffsetX
						stateY := sp.Y - ed.canvasOffsetY
						stateW := len(sp.Name) + 4

						if x >= stateX && x < stateX+stateW && y == stateY {
							ed.leftDownStateIdx = i
							break
						}
					}
				} else {
					// Mouse still held - check for drag
					dx := x - ed.leftDownX
					dy := y - ed.leftDownY
					if (dx != 0 || dy != 0) && ed.leftDownStateIdx >= 0 {
						// Started dragging a state
						ed.saveSnapshot()
						ed.dragging = true
						ed.dragStateIdx = ed.leftDownStateIdx
						ed.selectedState = ed.leftDownStateIdx
						sp := ed.states[ed.leftDownStateIdx]
						stateX := sp.X - ed.canvasOffsetX
						stateY := sp.Y - ed.canvasOffsetY
						ed.dragOffsetX = ed.leftDownX - stateX
						ed.dragOffsetY = ed.leftDownY - stateY
					}
				}
			}
		}
	} else {
		// Left button released
		if ed.leftMouseDown && !ed.dragging {
			// It was a click, not a drag
			if ed.mode == ModeCanvas {
				x, y := ed.leftDownX, ed.leftDownY
				if x < canvasW && y < h-2 {
					ed.canvasCursorX = x + ed.canvasOffsetX
					ed.canvasCursorY = y + ed.canvasOffsetY

					// Select state if clicked on one
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
				}
			}
		}
		ed.leftMouseDown = false
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
	// Start in last used directory
	ed.currentDir = ed.config.LastDir
	if ed.currentDir == "" {
		ed.currentDir, _ = os.Getwd()
	}
	
	ed.refreshFilePicker()
	ed.filePickerFocus = 1 // Start with files focused
	ed.mode = ModeFilePicker
}

func (ed *Editor) refreshFilePicker() {
	// Get directories
	ed.dirList = []string{".."}
	entries, err := os.ReadDir(ed.currentDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				ed.dirList = append(ed.dirList, e.Name())
			}
		}
	}
	ed.dirSelected = 0
	
	// Get files
	ed.fileList = nil
	fsmPattern := filepath.Join(ed.currentDir, "*.fsm")
	jsonPattern := filepath.Join(ed.currentDir, "*.json")
	fsmFiles, _ := filepath.Glob(fsmPattern)
	jsonFiles, _ := filepath.Glob(jsonPattern)
	
	// Store just filenames, not full paths
	for _, f := range fsmFiles {
		ed.fileList = append(ed.fileList, filepath.Base(f))
	}
	for _, f := range jsonFiles {
		ed.fileList = append(ed.fileList, filepath.Base(f))
	}
	ed.fileSelected = 0
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
		ed.showMessage("State moved", MsgSuccess)
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

func (ed *Editor) runAnalysis() {
	warnings := ed.fsm.Analyse()

	if len(warnings) == 0 {
		ed.showMessage("✓ No issues found", MsgSuccess)
		return
	}

	// Build a summary message
	var issues []string
	for _, w := range warnings {
		switch w.Type {
		case "unreachable":
			issues = append(issues, fmt.Sprintf("%d unreachable", len(w.States)))
		case "dead":
			issues = append(issues, fmt.Sprintf("%d dead", len(w.States)))
		case "nondeterministic":
			issues = append(issues, fmt.Sprintf("%d nondet", len(w.States)))
		case "incomplete":
			issues = append(issues, fmt.Sprintf("%d incomplete", len(w.States)))
		case "unused_input":
			issues = append(issues, "unused inputs")
		case "unused_output":
			issues = append(issues, "unused outputs")
		}
	}

	msg := fmt.Sprintf("✗ %d issue(s): %s", len(warnings), strings.Join(issues, ", "))
	ed.showMessage(msg, MsgError)
}

func (ed *Editor) runValidate() {
	err := ed.fsm.Validate()
	if err == nil {
		ed.showMessage("✓ FSM is valid", MsgSuccess)
	} else {
		ed.showMessage("✗ "+err.Error(), MsgError)
	}
}

func (ed *Editor) renderView() {
	// Generate title
	title := ed.fsm.Name
	if title == "" {
		title = "FSM"
	}

	var tmpPath string
	useNative := ed.config.Renderer == "native"
	useSVG := ed.config.FileType == "svg"
	
	// Check for dot if graphviz is selected
	if !useNative {
		if _, err := exec.LookPath("dot"); err != nil {
			ed.showMessage("Graphviz not found, using native renderer", MsgInfo)
			useNative = true
		}
	}

	if useNative {
		if useSVG {
			// Native SVG
			tmpFile, err := os.CreateTemp("", "fsm-*.svg")
			if err != nil {
				ed.showMessage("Failed to create temp file", MsgError)
				return
			}
			tmpPath = tmpFile.Name()
			tmpFile.Close()

			opts := fsmfile.DefaultSVGOptions()
			opts.Title = title
			svg := fsmfile.GenerateSVGNative(ed.fsm, opts)

			if err := os.WriteFile(tmpPath, []byte(svg), 0644); err != nil {
				ed.showMessage("Failed to write SVG", MsgError)
				os.Remove(tmpPath)
				return
			}
		} else {
			// Native PNG
			tmpFile, err := os.CreateTemp("", "fsm-*.png")
			if err != nil {
				ed.showMessage("Failed to create temp file", MsgError)
				return
			}
			tmpPath = tmpFile.Name()
			
			opts := fsmfile.DefaultPNGOptions()
			opts.Title = title
			if err := fsmfile.RenderPNG(ed.fsm, tmpFile, opts); err != nil {
				tmpFile.Close()
				ed.showMessage("Failed to generate PNG: "+err.Error(), MsgError)
				os.Remove(tmpPath)
				return
			}
			tmpFile.Close()
		}
	} else {
		// Graphviz
		ext := ".png"
		format := "png"
		if useSVG {
			ext = ".svg"
			format = "svg"
		}
		
		tmpFile, err := os.CreateTemp("", "fsm-*"+ext)
		if err != nil {
			ed.showMessage("Failed to create temp file", MsgError)
			return
		}
		tmpPath = tmpFile.Name()
		tmpFile.Close()

		dot := fsmfile.GenerateDOT(ed.fsm, title)
		cmd := exec.Command("dot", "-T"+format, "-o", tmpPath)
		cmd.Stdin = strings.NewReader(dot)
		if err := cmd.Run(); err != nil {
			ed.showMessage("dot failed: "+err.Error(), MsgError)
			os.Remove(tmpPath)
			return
		}
	}

	// Open with system viewer
	var openCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		openCmd = exec.Command("open", tmpPath)
	case "windows":
		openCmd = exec.Command("cmd", "/c", "start", "", tmpPath)
	default: // linux, etc
		openCmd = exec.Command("xdg-open", tmpPath)
	}

	if err := openCmd.Start(); err != nil {
		ed.showMessage("Failed to open viewer: "+err.Error(), MsgError)
		os.Remove(tmpPath)
		return
	}

	ed.showMessage("Opened in viewer: "+tmpPath, MsgSuccess)
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

