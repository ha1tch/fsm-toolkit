// Command fsmedit is a TUI editor for finite state machines.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
	"github.com/ha1tch/fsm-toolkit/pkg/fsmfile"
	"github.com/ha1tch/fsm-toolkit/pkg/version"
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
	config      Config

	// Bundle state
	isBundle        bool     // true if editing a machine from a bundle
	currentMachine  string   // name of current machine in bundle
	bundleMachines  []string // list of machine names in bundle
	bundleFSMs      map[string]*fsm.FSM       // all loaded FSMs in bundle
	bundleStates    map[string][]StatePos     // state positions per machine
	bundleUndoStack map[string][]Snapshot     // undo stack per machine
	bundleRedoStack map[string][]Snapshot     // redo stack per machine
	bundleModified  map[string]bool           // modified flag per machine
	bundleOffsets   map[string][2]int         // canvas offset per machine
	promotedFromSingle bool  // true if session was promoted from single to bundle
	originalFilename   string // pre-promotion filename (for save logic)
	
	// Import state
	importMode       bool   // true when file picker is for import (not open)
	dirPickerMode    bool   // true when file picker is for directory selection
	dirPickerAction  func(string) // callback when directory is selected
	importMachines   []string // machines available to import from a bundle
	importSelected   []bool   // multi-select state for import picker
	importCursor     int      // cursor position in import picker
	importSourcePath string   // source file path for bundle import
	
	// New machine state (pending type selection)
	pendingNewMachineName string
	newMachineTypeSelect  bool
	
	// Navigation stack for linked state traversal
	navStack        []NavFrame  // stack of parent contexts when diving into linked states
	
	// Link target selection
	linkTargetMachines []string // available machines to link to
	linkTargetSelected int      // selected index in linkTargetMachines
	
	// Zoom animation state
	animating       bool    // true during zoom animation
	animStartTime   int64   // Unix milliseconds when animation started
	animDuration    int64   // animation duration in milliseconds
	animZoomIn      bool    // true = zooming in, false = zooming out
	animCenterX     int     // center point of zoom (state position)
	animCenterY     int
	animTargetMachine string // machine we're transitioning to

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

	// Machine selector state (for bundles)
	machineList     []fsmfile.MachineInfo
	machineSelected int

	// Help scroll state
	helpScrollOffset int
	helpTotalLines   int

	// Type selector state (separate from main menu)
	typeMenuSelected int

	// Transition target selection (filtered list excluding existing self-loops)
	validTargets []string
	
	// Pending transition state (used during multi-step transition creation)
	pendingTransFrom string
	pendingTransTo   string
	pendingInput     *string
	mooreOutputMode  bool

	// Message flash state
	messageFlashStart int64 // Unix milliseconds when message was shown

	// Class editor state
	classEditorSelected   int       // selected class index (in ClassNames list)
	classEditorPropSel    int       // selected property index within class
	classEditorFocus      int       // 0=class list, 1=property list
	classEditorScroll     int       // scroll offset for class list
	classEditorPropScroll int       // scroll offset for property list

	// Machine manager state
	machMgrSelected int  // selected machine index
	machMgrScroll   int  // scroll offset
	machMgrShowInfo bool // show details panel

	// Class assignment state
	classAssignRows       []classAssignRow // flattened list of rows
	classAssignCursor     int              // selected row
	classAssignClassPick  bool             // true when picking a class for a state
	classAssignClassList  []string         // classes available to pick
	classAssignCursor2    int              // selected class in picker

	// Property editor state
	propEditorState       string           // state being edited
	propEditorMachine     string           // machine name (for bundles)
	propEditorProps       []propEditorRow  // flattened property rows
	propEditorCursor      int              // selected row
	propEditorScroll      int              // scroll offset (first visible row index)
	propEditorEditing     bool             // true when editing a value
	propEditorBuffer      string           // edit buffer for the current field
	propEditorReturnMode  Mode             // mode to return to on Esc

	// List editor (popup for editing list property values).
	listEditorItems       []string         // current list items
	listEditorCursor      int              // selected item
	listEditorScroll      int              // scroll offset
	listEditorAdding      bool             // true when typing a new item
	listEditorEditIdx     int              // >=0 when editing an existing item, -1 when adding new
	listEditorBuffer      string           // input buffer for new item

	// Settings screen.
	settingsCursor        int              // selected setting row

	// Component catalog (populated from class libraries).
	catalog               []CatalogCategory

	// Component drawer (bottom panel for drag-and-drop).
	drawerOpen            bool
	drawerAnimating       bool
	drawerAnimStart       int64            // for slide animation
	drawerAnimDir         int              // +1 opening, -1 closing
	drawerHeight          int              // current rendered height (animated)
	drawerMaxHeight       int              // target height when fully open
	drawerCatIdx          int              // selected category tab
	drawerItemIdx         int              // selected item within category
	drawerScroll          int              // horizontal scroll offset

	// Drag from drawer to canvas.
	drawerDragging        bool
	drawerDragClass       *fsm.Class       // class being dragged
	drawerDragX           int              // current mouse X
	drawerDragY           int              // current mouse Y
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

// CatalogCategory groups class definitions from a single library file.
type CatalogCategory struct {
	Name    string       // display name derived from filename
	Classes []*fsm.Class // class definitions in this category
	Source  string       // originating .classes.json filename
}

// NavFrame captures context when diving into a linked state
type NavFrame struct {
	MachineName    string     // machine we came from
	LinkedState    string     // state we clicked to dive in
	LinkedStateX   int        // position of that state (for zoom animation)
	LinkedStateY   int
	CanvasOffsetX  int        // viewport offset to restore
	CanvasOffsetY  int
	SelectedState  int        // selection to restore
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
	ModeHelp         // help overlay
	ModeCanvasDrag   // canvas panning with minimap
	ModeSelectMachine // bundle machine selector
	ModeSelectLinkTarget // linked state target machine selector
	ModeImportMachineSelect // multi-select picker for importing machines from bundle
	ModeClassEditor         // class definition editor
	ModeClassAssign         // state-to-class assignment grid
	ModePropertyEditor      // property value editor for a single state
	ModeListEditor          // list property value editor (popup)
	ModeSettings            // settings overlay
	ModeDrawer              // component drawer open (bottom panel)
	ModeMachineManager      // bundle machine management overlay
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
		switch os.Args[1] {
		case "-v", "--version", "version":
			fmt.Printf("fsmedit %s\n", version.Version)
			return
		default:
			ed.filename = os.Args[1]
			if err := ed.loadFile(ed.filename); err != nil {
				fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", ed.filename, err)
				os.Exit(1)
			}
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
	ed.menuItems = []string{
		"New",
		"Open File",
		"Import",
		"Machines",
		"Save",
		"Save As",
		"Edit Canvas",
		"Render",
		"Settings",
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
			
			// Check zoom animation in progress
			if ed.animating {
				needsRefresh = true
			}

			// Check drawer animation in progress
			if ed.drawerAnimating {
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
