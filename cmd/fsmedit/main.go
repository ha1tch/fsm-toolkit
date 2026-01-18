// Command fsmedit is a TUI editor for finite state machines.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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

	// Double-click detection
	lastClickTime  int64 // Unix milliseconds of last click
	lastClickX     int
	lastClickY     int
	lastClickState int // state index clicked, -1 if none

	// Pending state position (for right-click add state)
	pendingStateX int
	pendingStateY int

	// Right-button tracking
	rightMouseDown bool
	rightDownX     int
	rightDownY     int

	// Middle-button tracking (canvas drag)
	middleMouseDown bool
	middleDownX     int
	middleDownY     int

	// Canvas drag mode (Ctrl+D or middle-drag)
	canvasDragMode   bool
	dragStartOffsetX int // viewport offset when drag started
	dragStartOffsetY int

	// Move mode state (keyboard)
	moveStateIdx int // state being moved
	moveOrigX    int // original position for undo
	moveOrigY    int

	// Display options
	showArcs bool // toggle arc visibility with 'w'

	// Flash effects (when clicking items in sidebar)
	flashInput      string // input symbol being flashed, empty if none
	flashInputTime  int64  // Unix milliseconds when flash started
	flashOutput     string // output symbol being flashed
	flashOutputTime int64
	flashTransIdx   int   // transition index being flashed, -1 if none
	flashTransTime  int64

	// Undo/Redo
	undoStack []Snapshot
	redoStack []Snapshot

	// UI regions
	canvasWidth      int
	canvasHeight     int
	sidebarWidth     int
	sidebarCollapsed bool
	sidebarDragging  bool
	sidebarMinWidth  int
	sidebarMaxWidth  int
	sidebarSnapWidth int // snap to this width when near
	sidebarScrollY   int // scroll offset for sidebar content
	sidebarDraggingScroll bool // dragging the scrollbar thumb

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

	// Help scroll state
	helpScrollOffset int
	helpTotalLines   int

	// Type selector state (separate from main menu)
	typeMenuSelected int

	// Transition target selection (filtered list excluding existing self-loops)
	validTargets []string

	// Message flash state
	messageFlashStart int64 // Unix milliseconds when message was shown
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

// Virtual canvas dimensions (logical coordinate space)
const (
	CanvasMaxWidth  = 512
	CanvasMaxHeight = 512
)

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
	ModeMove       // keyboard-driven state movement
	ModeHelp       // help overlay
	ModeCanvasDrag // canvas panning with minimap
)

// MessageType for status messages
type MessageType int

const (
	MsgInfo    MessageType = iota // Informative, no flash
	MsgError                      // Errors, flash
	MsgSuccess                    // State changes, flash
	MsgWarning                    // Warnings, flash
)

func main() {
	ed := &Editor{
		fsm:              fsm.New(fsm.TypeDFA),
		selectedState:    -1,
		selectedTrans:    -1,
		lastClickState:   -1,
		sidebarWidth:     30,
		sidebarMinWidth:  1,  // Collapsed width (just the divider)
		sidebarMaxWidth:  60,
		sidebarSnapWidth: 30, // Default snap width
		flashTransIdx:    -1,
		states:           make([]StatePos, 0),
		config:           LoadConfig(),
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
	
	fsmTypeLabel := fmt.Sprintf("FSM Type: %s", fsmTypeDisplayName(ed.fsm.Type))
	
	ed.menuItems = []string{
		"New FSM",
		"Open File",
		"Save",
		"Save As",
		"Edit Canvas",
		"Render",
		rendererLabel,
		fileTypeLabel,
		fsmTypeLabel,
		"Quit",
	}
}

func (ed *Editor) run() {
	// Use a goroutine to send periodic refresh events during any flash animation
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond) // 20fps for smooth flash
		defer ticker.Stop()
		for range ticker.C {
			needsRefresh := false
			
			// Check message flash (still time-limited)
			if ed.message != "" && ed.messageFlashStart > 0 {
				elapsed := time.Now().UnixMilli() - ed.messageFlashStart
				if elapsed >= 0 && elapsed < 700 {
					needsRefresh = true
				}
			}
			
			// Check input/output/transition flash (persistent until cleared)
			if ed.flashInput != "" || ed.flashOutput != "" || ed.flashTransIdx >= 0 {
				needsRefresh = true
			}
			
			if needsRefresh {
				ed.screen.PostEvent(tcell.NewEventInterrupt(nil))
			}
		}
	}()

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
		case *tcell.EventInterrupt:
			// Refresh event for flash animation - just redraw
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

// toggleCanvasDragMode enters or exits canvas drag mode (with minimap)
func (ed *Editor) toggleCanvasDragMode() {
	if ed.mode == ModeCanvasDrag {
		ed.exitCanvasDragMode()
	} else if ed.mode == ModeCanvas {
		ed.enterCanvasDragMode()
	}
}

// enterCanvasDragMode activates canvas panning with minimap display
func (ed *Editor) enterCanvasDragMode() {
	ed.mode = ModeCanvasDrag
	ed.canvasDragMode = true
	ed.dragStartOffsetX = ed.canvasOffsetX
	ed.dragStartOffsetY = ed.canvasOffsetY
	ed.showMessage("Canvas drag mode - Arrow keys to pan, Esc to exit", MsgInfo)
}

// exitCanvasDragMode returns to normal canvas mode
func (ed *Editor) exitCanvasDragMode() {
	ed.mode = ModeCanvas
	ed.canvasDragMode = false
	ed.middleMouseDown = false
}

// panViewport moves the viewport by the given delta, clamping to canvas bounds
func (ed *Editor) panViewport(dx, dy int) {
	ed.canvasOffsetX += dx
	ed.canvasOffsetY += dy

	// Clamp to valid range (0 to CanvasMax - visible area)
	if ed.canvasOffsetX < 0 {
		ed.canvasOffsetX = 0
	}
	if ed.canvasOffsetY < 0 {
		ed.canvasOffsetY = 0
	}

	// Get visible canvas dimensions
	w, h := ed.screen.Size()
	visibleW := w - ed.sidebarWidth - 1
	visibleH := h - 2 // status bar

	maxOffsetX := CanvasMaxWidth - visibleW
	maxOffsetY := CanvasMaxHeight - visibleH
	if maxOffsetX < 0 {
		maxOffsetX = 0
	}
	if maxOffsetY < 0 {
		maxOffsetY = 0
	}

	if ed.canvasOffsetX > maxOffsetX {
		ed.canvasOffsetX = maxOffsetX
	}
	if ed.canvasOffsetY > maxOffsetY {
		ed.canvasOffsetY = maxOffsetY
	}
}

// handleCanvasDragKey handles keys while in canvas drag mode
func (ed *Editor) handleCanvasDragKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		ed.exitCanvasDragMode()
	case tcell.KeyUp:
		ed.panViewport(0, -3)
	case tcell.KeyDown:
		ed.panViewport(0, 3)
	case tcell.KeyLeft:
		ed.panViewport(-3, 0)
	case tcell.KeyRight:
		ed.panViewport(3, 0)
	}
	return false
}

func (ed *Editor) handleSidebarScrollDrag(mouseY, screenH int) {
	visibleHeight := screenH - 4
	scrollTrackStart := 2
	scrollTrackHeight := visibleHeight
	
	// Calculate total content height
	totalHeight := 0
	totalHeight += 1 + len(ed.fsm.States) + 1 // States section
	totalHeight += 1 + len(ed.fsm.Alphabet) + 1 // Inputs section
	if len(ed.fsm.OutputAlphabet) > 0 {
		totalHeight += 1 + len(ed.fsm.OutputAlphabet) + 1 // Outputs section
	}
	totalHeight += 1 // Transitions header
	for _, t := range ed.fsm.Transitions {
		totalHeight += len(t.To)
	}
	
	maxScroll := totalHeight - visibleHeight
	if maxScroll <= 0 {
		ed.sidebarScrollY = 0
		return
	}
	
	// Calculate thumb size
	thumbHeight := (visibleHeight * visibleHeight) / totalHeight
	if thumbHeight < 1 {
		thumbHeight = 1
	}
	
	// Convert mouse Y to scroll position
	// Mouse position relative to track
	relY := mouseY - scrollTrackStart
	if relY < 0 {
		relY = 0
	}
	if relY > scrollTrackHeight-thumbHeight {
		relY = scrollTrackHeight - thumbHeight
	}
	
	// Calculate scroll offset
	ed.sidebarScrollY = (relY * maxScroll) / (scrollTrackHeight - thumbHeight)
	if ed.sidebarScrollY < 0 {
		ed.sidebarScrollY = 0
	}
	if ed.sidebarScrollY > maxScroll {
		ed.sidebarScrollY = maxScroll
	}
}

func (ed *Editor) handleCanvasKey(ev *tcell.EventKey) bool {
	// Check for Shift+Arrow for viewport panning
	mod := ev.Modifiers()
	if mod&tcell.ModShift != 0 {
		switch ev.Key() {
		case tcell.KeyUp:
			ed.panViewport(0, -1)
			return false
		case tcell.KeyDown:
			ed.panViewport(0, 1)
			return false
		case tcell.KeyLeft:
			ed.panViewport(-1, 0)
			return false
		case tcell.KeyRight:
			ed.panViewport(1, 0)
			return false
		}
	}

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
			if len(ed.fsm.States) == 0 {
				ed.showMessage("Canvas is empty - nothing to render", MsgError)
			} else {
				ed.renderView()
			}
		case 'h', 'H', '?':
			ed.mode = ModeHelp
		case '\\':
			// Toggle sidebar collapse
			ed.toggleSidebarCollapse()
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
		if ed.typeMenuSelected > 0 {
			ed.typeMenuSelected--
		}
	case tcell.KeyDown:
		if ed.typeMenuSelected < len(types)-1 {
			ed.typeMenuSelected++
		}
	case tcell.KeyEnter:
		ed.fsm.Type = types[ed.typeMenuSelected]
		ed.modified = true
		ed.updateMenuItems()
		ed.showMessage("FSM type set to "+fsmTypeDisplayName(ed.fsm.Type), MsgSuccess)
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

func (ed *Editor) handleMouse(ev *tcell.EventMouse) {
	x, y := ev.Position()
	buttons := ev.Buttons()

	w, h := ed.screen.Size()
	dividerX := w - ed.sidebarWidth
	canvasW := dividerX

	// Handle sidebar divider dragging
	allReleased := buttons&tcell.Button1 == 0 && buttons&tcell.Button2 == 0 && buttons&tcell.Button3 == 0
	
	if ed.sidebarDragging {
		if allReleased {
			ed.sidebarDragging = false
		} else {
			// Calculate new sidebar width (divider is at w - sidebarWidth)
			newWidth := w - x
			
			// Snap behaviour: if within 5 pixels of snap width, snap to it
			if newWidth >= ed.sidebarSnapWidth-5 && newWidth <= ed.sidebarSnapWidth+5 {
				newWidth = ed.sidebarSnapWidth
			}
			
			// Snap to max width when near the right edge
			if newWidth >= ed.sidebarMaxWidth-5 {
				newWidth = ed.sidebarMaxWidth
			}
			
			// Clamp to min/max
			if newWidth < ed.sidebarMinWidth {
				newWidth = ed.sidebarMinWidth
				ed.sidebarCollapsed = true
			} else {
				ed.sidebarCollapsed = false
			}
			if newWidth > ed.sidebarMaxWidth {
				newWidth = ed.sidebarMaxWidth
			}
			
			ed.sidebarWidth = newWidth
		}
		return
	}
	
	// Handle mouse wheel scrolling in sidebar
	if x > dividerX && !ed.sidebarCollapsed {
		if buttons&tcell.WheelUp != 0 {
			ed.sidebarScrollY -= 3
			if ed.sidebarScrollY < 0 {
				ed.sidebarScrollY = 0
			}
			return
		}
		if buttons&tcell.WheelDown != 0 {
			ed.sidebarScrollY += 3
			// Max scroll will be clamped in drawSidebar
			return
		}
	}
	
	// Check for click on divider to start drag or double-click to toggle
	if buttons&tcell.Button1 != 0 && !ed.leftMouseDown {
		// Check if clicking on or near the divider (within 1 char)
		if x >= dividerX-1 && x <= dividerX+1 && y < h-2 {
			ed.sidebarDragging = true
			return
		}
	}
	
	// Double-click on divider to toggle collapse
	if buttons&tcell.Button1 == 0 && ed.leftMouseDown {
		// This is a release - check for double-click on divider
		// (simplified: just use single click near divider edge to toggle)
	}
	
	// Handle scrollbar drag release
	if ed.sidebarDraggingScroll && allReleased {
		ed.sidebarDraggingScroll = false
	}
	
	// Handle ongoing scrollbar drag
	if ed.sidebarDraggingScroll && buttons&tcell.Button1 != 0 {
		ed.handleSidebarScrollDrag(y, h)
		return
	}

	// Handle clicks in sidebar to select states, flash inputs, or interact with scrollbar
	if buttons&tcell.Button1 != 0 && !ed.leftMouseDown && !ed.sidebarCollapsed {
		scrollbarX := w - 1
		
		// Check if clicking on scrollbar
		if x == scrollbarX && y >= 2 && y < h-2 {
			ed.sidebarDraggingScroll = true
			ed.handleSidebarScrollDrag(y, h)
			return
		}
		
		// Check if clicking in sidebar content area (past the divider, before scrollbar)
		if x > dividerX && x < scrollbarX && y >= 2 && y < h-2 {
			// Convert screen Y to content line index (accounting for scroll)
			visibleHeight := h - 4
			contentY := (y - 2) + ed.sidebarScrollY
			
			// Calculate content line ranges
			// States section: line 0 = "States:", lines 1..len(states) = states, then blank
			statesHeaderLine := 0
			statesStartLine := 1
			statesEndLine := statesStartLine + len(ed.fsm.States)
			blankAfterStates := statesEndLine
			
			// Inputs section
			inputsHeaderLine := blankAfterStates + 1
			inputsStartLine := inputsHeaderLine + 1
			inputsEndLine := inputsStartLine + len(ed.fsm.Alphabet)
			blankAfterInputs := inputsEndLine
			
			// Outputs section (if any)
			var outputsHeaderLine, outputsStartLine, outputsEndLine, blankAfterOutputs int
			if len(ed.fsm.OutputAlphabet) > 0 {
				outputsHeaderLine = blankAfterInputs + 1
				outputsStartLine = outputsHeaderLine + 1
				outputsEndLine = outputsStartLine + len(ed.fsm.OutputAlphabet)
				blankAfterOutputs = outputsEndLine
			} else {
				blankAfterOutputs = blankAfterInputs
			}
			
			// Transitions section
			transHeaderLine := blankAfterOutputs + 1
			transStartLine := transHeaderLine + 1
			transLineCount := 0
			for _, t := range ed.fsm.Transitions {
				transLineCount += len(t.To)
			}
			transEndLine := transStartLine + transLineCount
			
			_ = statesHeaderLine
			_ = inputsHeaderLine
			_ = outputsHeaderLine
			_ = transHeaderLine
			_ = visibleHeight
			
			if contentY >= statesStartLine && contentY < statesEndLine {
				// Clicked on a state
				ed.clearFlash()
				clickedStateIdx := contentY - statesStartLine
				if clickedStateIdx >= 0 && clickedStateIdx < len(ed.fsm.States) {
					ed.selectedState = clickedStateIdx
					ed.selectedTrans = -1
					if ed.mode == ModeMenu {
						ed.mode = ModeCanvas
					}
				}
			} else if contentY >= inputsStartLine && contentY < inputsEndLine {
				// Clicked on an input
				ed.clearFlash()
				clickedInputIdx := contentY - inputsStartLine
				if clickedInputIdx >= 0 && clickedInputIdx < len(ed.fsm.Alphabet) {
					ed.flashInput = ed.fsm.Alphabet[clickedInputIdx]
					ed.flashInputTime = time.Now().UnixMilli()
				}
			} else if len(ed.fsm.OutputAlphabet) > 0 && contentY >= outputsStartLine && contentY < outputsEndLine {
				// Clicked on an output
				ed.clearFlash()
				clickedOutputIdx := contentY - outputsStartLine
				if clickedOutputIdx >= 0 && clickedOutputIdx < len(ed.fsm.OutputAlphabet) {
					ed.flashOutput = ed.fsm.OutputAlphabet[clickedOutputIdx]
					ed.flashOutputTime = time.Now().UnixMilli()
				}
			} else if contentY >= transStartLine && contentY < transEndLine {
				// Clicked on a transition
				ed.clearFlash()
				clickedLine := contentY - transStartLine
				lineIdx := 0
				for tIdx, t := range ed.fsm.Transitions {
					for range t.To {
						if lineIdx == clickedLine {
							ed.flashTransIdx = tIdx
							ed.flashTransTime = time.Now().UnixMilli()
							break
						}
						lineIdx++
					}
					if ed.flashTransIdx == tIdx {
						break
					}
				}
			}
		}
	}

	// Handle drag release (all buttons released)
	if ed.dragging && allReleased {
		ed.dragging = false
		ed.modified = true
		ed.showMessage("State moved", MsgInfo)
		ed.leftMouseDown = false
		ed.rightMouseDown = false
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
			// Clamp to canvas bounds
			if newX > CanvasMaxWidth-10 {
				newX = CanvasMaxWidth - 10
			}
			if newY > CanvasMaxHeight-2 {
				newY = CanvasMaxHeight - 2
			}
			ed.states[ed.dragStateIdx].X = newX
			ed.states[ed.dragStateIdx].Y = newY

			// Auto-scroll viewport when dragging near edge
			edgeMargin := 3
			scrollSpeed := 2
			if x < edgeMargin {
				ed.panViewport(-scrollSpeed, 0)
			}
			if x > canvasW-edgeMargin {
				ed.panViewport(scrollSpeed, 0)
			}
			if y < edgeMargin {
				ed.panViewport(0, -scrollSpeed)
			}
			if y > h-2-edgeMargin {
				ed.panViewport(0, scrollSpeed)
			}
		}
		return
	}

	// Right button handling (tcell: Button2 = right/secondary, Button3 = middle)
	rightPressed := buttons&tcell.Button2 != 0
	if rightPressed {
		if !ed.rightMouseDown && ed.mode == ModeCanvas {
			// Clear any active flash when interacting with canvas
			ed.clearFlash()
			ed.rightMouseDown = true
			ed.rightDownX = x
			ed.rightDownY = y
		}
	} else {
		// Right button released
		if ed.rightMouseDown && ed.mode == ModeCanvas {
			// Check if it was a click (not moved much)
			dx := x - ed.rightDownX
			dy := y - ed.rightDownY
			if dx >= -1 && dx <= 1 && dy >= -1 && dy <= 1 {
				// Right-click detected
				clickX, clickY := ed.rightDownX, ed.rightDownY
				if clickX < canvasW && clickY < h-2 {
					// Check if clicked on a state (select, not add)
					clickedOnState := false
					for i, sp := range ed.states {
						stateX := sp.X - ed.canvasOffsetX
						stateY := sp.Y - ed.canvasOffsetY
						stateW := len(sp.Name) + 4

						if clickX >= stateX && clickX < stateX+stateW && clickY == stateY {
							clickedOnState = true
							ed.selectedState = i
							break
						}
					}

					if !clickedOnState {
						// Right-click on empty canvas - add state at position
						ed.addStateAtPosition(clickX+ed.canvasOffsetX, clickY+ed.canvasOffsetY)
					}
				}
			}
		}
		ed.rightMouseDown = false
	}

	// Middle button handling (Button3) - canvas drag mode
	middlePressed := buttons&tcell.Button3 != 0
	if middlePressed {
		if !ed.middleMouseDown {
			// Middle button just pressed - enter canvas drag mode
			ed.middleMouseDown = true
			ed.middleDownX = x
			ed.middleDownY = y
			ed.dragStartOffsetX = ed.canvasOffsetX
			ed.dragStartOffsetY = ed.canvasOffsetY
			if ed.mode == ModeCanvas {
				ed.mode = ModeCanvasDrag
				ed.canvasDragMode = true
			}
		} else if ed.mode == ModeCanvasDrag {
			// Middle button held - drag to pan viewport
			dx := ed.middleDownX - x
			dy := ed.middleDownY - y
			ed.canvasOffsetX = ed.dragStartOffsetX + dx
			ed.canvasOffsetY = ed.dragStartOffsetY + dy

			// Clamp viewport
			if ed.canvasOffsetX < 0 {
				ed.canvasOffsetX = 0
			}
			if ed.canvasOffsetY < 0 {
				ed.canvasOffsetY = 0
			}
			visibleW := w - ed.sidebarWidth - 1
			visibleH := h - 2
			maxOffsetX := CanvasMaxWidth - visibleW
			maxOffsetY := CanvasMaxHeight - visibleH
			if maxOffsetX < 0 {
				maxOffsetX = 0
			}
			if maxOffsetY < 0 {
				maxOffsetY = 0
			}
			if ed.canvasOffsetX > maxOffsetX {
				ed.canvasOffsetX = maxOffsetX
			}
			if ed.canvasOffsetY > maxOffsetY {
				ed.canvasOffsetY = maxOffsetY
			}
		}
	} else {
		// Middle button released
		if ed.middleMouseDown {
			ed.middleMouseDown = false
			if ed.mode == ModeCanvasDrag {
				ed.exitCanvasDragMode()
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
				// Clear any active flash when interacting with canvas
				ed.clearFlash()
				
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
				clickX, clickY := ed.leftDownX, ed.leftDownY
				if clickX < canvasW && clickY < h-2 {
					ed.canvasCursorX = clickX + ed.canvasOffsetX
					ed.canvasCursorY = clickY + ed.canvasOffsetY

					// Find which state was clicked (if any)
					clickedState := -1
					for i, sp := range ed.states {
						stateX := sp.X - ed.canvasOffsetX
						stateY := sp.Y - ed.canvasOffsetY
						stateW := len(sp.Name) + 4

						if clickX >= stateX && clickX < stateX+stateW && clickY == stateY {
							clickedState = i
							break
						}
					}

					// Check for double-click (within 400ms and same location)
					now := time.Now().UnixMilli()
					isDoubleClick := false
					if clickedState >= 0 && clickedState == ed.lastClickState {
						if now-ed.lastClickTime < 400 {
							// Double-click on same state
							isDoubleClick = true
						}
					}

					if isDoubleClick {
						// Double-click detected - edit state name
						ed.editStateName(clickedState)
						ed.lastClickTime = 0 // Reset to prevent triple-click
						ed.lastClickState = -1
					} else {
						// Single click - select state
						ed.selectedState = clickedState
						ed.lastClickTime = now
						ed.lastClickX = clickX
						ed.lastClickY = clickY
						ed.lastClickState = clickedState
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
		_, h := ed.screen.Size()
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
		_, h := ed.screen.Size()
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
		_, h := ed.screen.Size()
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
			_, h := ed.screen.Size()
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
var pendingTransFrom string
var pendingTransTo string

func (ed *Editor) completeAddTransition() {
	if ed.selectedState < 0 || ed.selectedState >= len(ed.states) {
		ed.mode = ModeCanvas
		return
	}
	if ed.menuSelected < 0 || ed.menuSelected >= len(ed.validTargets) {
		ed.mode = ModeCanvas
		return
	}
	pendingTransFrom = ed.states[ed.selectedState].Name
	pendingTransTo = ed.validTargets[ed.menuSelected]

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
		ed.showMessage("✓ No issues found", MsgInfo)
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
	ed.showMessage(msg, MsgWarning)
}

func (ed *Editor) runValidate() {
	err := ed.fsm.Validate()
	if err == nil {
		ed.showMessage("✓ FSM is valid", MsgInfo)
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

	ed.showMessage("Opened in viewer: "+tmpPath, MsgInfo)
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

