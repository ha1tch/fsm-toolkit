// File operations (open, save, load) for fsmedit.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
	"github.com/ha1tch/fsm-toolkit/pkg/fsmfile"
)


// Actions

// confirmNew prompts for confirmation before clearing all work.
func (ed *Editor) confirmNew() {
	// Check if there's anything to lose.
	hasContent := len(ed.fsm.States) > 0 || ed.modified
	if ed.isBundle {
		hasContent = true
	}

	if !hasContent {
		// Nothing to lose — go straight to name prompt.
		ed.newFSM()
		return
	}

	ed.inputPrompt = "Clear all and start new? (y/n): "
	ed.inputBuffer = ""
	ed.inputAction = func(answer string) {
		if strings.ToLower(answer) == "y" {
			ed.newFSM()
		} else {
			ed.mode = ModeMenu
		}
	}
	ed.mode = ModeInput
}

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
		ed.resetBundleState()
		ed.updateMenuItems()
		ed.showMessage("New FSM created", MsgSuccess)
		ed.mode = ModeCanvas
	}
	ed.mode = ModeInput
}

// resetBundleState clears all bundle-related state for a fresh start.
func (ed *Editor) resetBundleState() {
	ed.isBundle = false
	ed.currentMachine = ""
	ed.bundleMachines = nil
	ed.bundleFSMs = nil
	ed.bundleStates = nil
	ed.bundleUndoStack = nil
	ed.bundleRedoStack = nil
	ed.bundleModified = nil
	ed.bundleOffsets = nil
	ed.navStack = nil
	ed.promotedFromSingle = false
	ed.originalFilename = ""
	ed.importMode = false
	ed.importSourcePath = ""
}

func (ed *Editor) openFilePicker() {
	// Start in the current working directory. Fall back to last used
	// directory from config only if CWD is unavailable.
	ed.currentDir, _ = os.Getwd()
	if ed.currentDir == "" {
		ed.currentDir = ed.config.LastDir
	}
	if ed.currentDir == "" {
		ed.currentDir = "/"
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
	
	// If promoted from single to bundle, saving to the original file needs confirmation
	if ed.promotedFromSingle && ed.filename == ed.originalFilename {
		ed.inputPrompt = "Save will convert to bundle. (b)ackup first, (o)verwrite, (c)ancel: "
		ed.inputBuffer = ""
		ed.inputAction = func(answer string) {
			switch strings.ToLower(answer) {
			case "b":
				// Backup original, then save
				backupPath := ed.originalFilename + ".bak"
				data, err := os.ReadFile(ed.originalFilename)
				if err != nil {
					ed.showMessage("Backup failed: "+err.Error(), MsgError)
					ed.mode = ModeMenu
					return
				}
				if err := os.WriteFile(backupPath, data, 0644); err != nil {
					ed.showMessage("Backup failed: "+err.Error(), MsgError)
					ed.mode = ModeMenu
					return
				}
				ed.showMessage("Backup: "+filepath.Base(backupPath), MsgSuccess)
				ed.promotedFromSingle = false // don't ask again
				ed.doSave()
			case "o":
				ed.promotedFromSingle = false
				ed.doSave()
			default:
				ed.showMessage("Save cancelled", MsgInfo)
			}
			ed.mode = ModeMenu
		}
		ed.mode = ModeInput
		return
	}
	
	ed.doSave()
}

func (ed *Editor) doSave() {
	if err := ed.saveFile(ed.filename); err != nil {
		ed.showMessage("Error: "+err.Error(), MsgError)
	} else {
		ed.modified = false
		if ed.isBundle {
			ed.showMessage("Saved bundle: "+filepath.Base(ed.filename), MsgSuccess)
		} else {
			ed.showMessage("Saved: "+ed.filename, MsgSuccess)
		}
	}
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
		ed.promotedFromSingle = false // new filename, no more promotion concern
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

// File operations

func (ed *Editor) loadFile(path string) error {
	// Reset bundle state before loading a new file
	ed.resetBundleState()
	
	ext := filepath.Ext(path)

	var f *fsm.FSM
	var layout *fsmfile.Layout
	var err error

	switch ext {
	case ".fsm":
		// Check if this is a bundle with multiple machines
		machines, listErr := fsmfile.ListMachines(path)
		if listErr == nil && len(machines) > 1 {
			// It's a bundle - show machine selector
			ed.filename = path
			ed.machineList = machines
			ed.machineSelected = 0
			ed.isBundle = true
			ed.bundleMachines = make([]string, len(machines))
			for i, m := range machines {
				ed.bundleMachines[i] = m.Name
			}
			
			// Initialize bundle caches
			ed.bundleFSMs = make(map[string]*fsm.FSM)
			ed.bundleStates = make(map[string][]StatePos)
			ed.bundleUndoStack = make(map[string][]Snapshot)
			ed.bundleRedoStack = make(map[string][]Snapshot)
			ed.bundleModified = make(map[string]bool)
			ed.bundleOffsets = make(map[string][2]int)
			ed.navStack = nil
			
			// Pre-load all machines into cache
			for _, m := range machines {
				f, _, err := fsmfile.ReadMachineFromBundle(path, m.Name)
				if err == nil {
					ed.bundleFSMs[m.Name] = f
				}
			}
			
			ed.mode = ModeSelectMachine
			return nil
		}
		// Single machine - load normally
		f, layout, err = fsmfile.ReadFSMFileWithLayout(path)
		ed.isBundle = false
		ed.currentMachine = ""
	case ".json":
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		f, err = fsmfile.ParseJSON(data)
		ed.isBundle = false
		ed.currentMachine = ""
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
		ed.isBundle = false
		ed.currentMachine = ""
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
		
		autoPositions := fsmfile.SmartLayoutTUI(f, w, h)
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
	
	// Bundle save - save all modified machines
	if ed.isBundle && ext == ".fsm" {
		return ed.saveBundleFile(path)
	}
	
	// Single file save
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

// saveBundleFile saves the bundle. If the target file exists, only modified
// machines are updated (preserving unmodified data). If the file doesn't exist
// (Save As), a complete new bundle is created from all cached machines.
func (ed *Editor) saveBundleFile(path string) error {
	// Save current machine to cache first
	ed.saveMachineToCache()
	
	// Check if target file already exists
	_, statErr := os.Stat(path)
	fileExists := statErr == nil
	
	if fileExists {
		// Existing file: only update modified machines
		updates := make(map[string]fsmfile.BundleMachineData)
		
		for name, isModified := range ed.bundleModified {
			if !isModified {
				continue
			}
			f, ok := ed.bundleFSMs[name]
			if !ok {
				continue
			}
			updates[name] = ed.buildMachineData(name, f)
		}
		
		if len(updates) == 0 {
			return nil
		}
		
		if err := fsmfile.UpdateBundleMachines(path, updates); err != nil {
			return err
		}
	} else {
		// New file (Save As): write ALL machines
		allMachines := make(map[string]fsmfile.BundleMachineData)
		
		for _, name := range ed.bundleMachines {
			f, ok := ed.bundleFSMs[name]
			if !ok {
				continue
			}
			allMachines[name] = ed.buildMachineData(name, f)
		}
		
		if len(allMachines) == 0 {
			return fmt.Errorf("no machines to save")
		}
		
		if err := fsmfile.WriteBundleFromData(path, allMachines); err != nil {
			return err
		}
	}
	
	// Clear modified flags
	for name := range ed.bundleModified {
		ed.bundleModified[name] = false
	}
	ed.modified = false
	
	return nil
}

// buildMachineData assembles BundleMachineData for a machine from the cache.
func (ed *Editor) buildMachineData(name string, f *fsm.FSM) fsmfile.BundleMachineData {
	positions := make(map[string][2]int)
	if states, ok := ed.bundleStates[name]; ok {
		for _, sp := range states {
			positions[sp.Name] = [2]int{sp.X, sp.Y}
		}
	}
	
	offsetX, offsetY := 0, 0
	if offsets, ok := ed.bundleOffsets[name]; ok {
		offsetX = offsets[0]
		offsetY = offsets[1]
	}
	
	return fsmfile.BundleMachineData{
		FSM:       f,
		Positions: positions,
		OffsetX:   offsetX,
		OffsetY:   offsetY,
	}
}
