// Modal overlay draw functions for fsmedit.
package main

import (
	"fmt"
)

func (ed *Editor) drawInputBox(w, h int) {
	boxW := 50
	boxH := 3
	boxX := (w - boxW) / 2
	boxY := (h - boxH) / 2

	// Draw box
	ed.drawBox(boxX, boxY, boxW, boxH, styleInput)

	// Draw prompt and input
	ed.drawString(boxX+2, boxY+1, ed.inputPrompt, styleInput)
	ed.drawString(boxX+2+len(ed.inputPrompt), boxY+1, ed.inputBuffer+"_", styleInput)
}

func (ed *Editor) drawFilePicker(w, h int) {
	// Two-column file picker: directories on left, files on right
	totalW := 80
	if totalW > w-4 {
		totalW = w - 4
	}
	dirW := totalW / 3
	fileW := totalW - dirW - 1
	
	// Calculate height based on content
	maxItems := len(ed.dirList)
	if len(ed.fileList) > maxItems {
		maxItems = len(ed.fileList)
	}
	boxH := maxItems + 6
	if boxH > h-4 {
		boxH = h - 4
	}
	if boxH < 10 {
		boxH = 10
	}
	
	boxX := (w - totalW) / 2
	boxY := 2
	
	// Draw main box
	ed.drawBox(boxX, boxY, totalW, boxH, styleDefault)
	
	// Draw current directory path at top
	pathDisplay := ed.currentDir
	if ed.importMode {
		pathDisplay = "Import from: " + pathDisplay
	} else if ed.dirPickerMode {
		pathDisplay = "Select directory: " + pathDisplay
	}
	if len(pathDisplay) > totalW-4 {
		pathDisplay = "..." + pathDisplay[len(pathDisplay)-(totalW-7):]
	}
	ed.drawString(boxX+2, boxY+1, pathDisplay, styleSidebarH)
	
	// Draw column headers
	dirHeader := "Directories"
	fileHeader := "Files"
	if ed.dirPickerMode {
		fileHeader = "Contents (preview)"
	}
	if ed.filePickerFocus == 0 {
		ed.drawString(boxX+2, boxY+3, dirHeader, styleMenuSel)
	} else {
		ed.drawString(boxX+2, boxY+3, dirHeader, styleSidebarH)
	}
	if ed.filePickerFocus == 1 {
		ed.drawString(boxX+dirW+2, boxY+3, fileHeader, styleMenuSel)
	} else {
		ed.drawString(boxX+dirW+2, boxY+3, fileHeader, styleSidebarH)
	}
	
	// Draw vertical separator
	for y := boxY + 3; y < boxY+boxH-1; y++ {
		ed.drawString(boxX+dirW, y, "│", styleDefault)
	}
	
	// Draw directories
	visibleItems := boxH - 6
	for i, d := range ed.dirList {
		if i >= visibleItems {
			break
		}
		style := styleMenu
		if ed.filePickerFocus == 0 && i == ed.dirSelected {
			style = styleMenuSel
		}
		// Use simple ASCII prefix for directories
		var display string
		if d == ".." {
			display = "[^] .."
		} else {
			display = "[/] " + d
		}
		// Truncate to fit column width (leaving space for padding)
		maxLen := dirW - 3
		if len(display) > maxLen {
			display = display[:maxLen-2] + ".."
		}
		line := fmt.Sprintf(" %-*s", maxLen, display)
		ed.drawString(boxX+1, boxY+5+i, line, style)
	}
	
	// Draw files
	if len(ed.fileList) == 0 {
		ed.drawString(boxX+dirW+2, boxY+5, "(no files)", styleDefault)
	} else {
		for i, f := range ed.fileList {
			if i >= visibleItems {
				break
			}
			style := styleMenu
			if ed.dirPickerMode {
				// Files are preview-only in directory picker mode.
				style = styleDefault
			} else if ed.filePickerFocus == 1 && i == ed.fileSelected {
				style = styleMenuSel
			}
			line := fmt.Sprintf(" %-*s", fileW-3, truncate(f, fileW-3))
			ed.drawString(boxX+dirW+1, boxY+5+i, line, style)
		}
	}
	
	// Draw help at bottom
	var help string
	if ed.dirPickerMode {
		help = "↑/↓: navigate | Enter: open dir | S: select this directory | Esc: cancel"
		if len(help) > totalW-4 {
			help = "↑↓:nav Enter:open S:select Esc:quit"
		}
	} else {
		help = "←/→ or Tab: switch | ↑/↓: navigate | Enter: select | Esc: cancel"
		if len(help) > totalW-4 {
			help = "Tab:switch ↑↓:nav Enter:sel Esc:quit"
		}
	}
	ed.drawString(boxX+2, boxY+boxH-1, help, styleDefault)
}

func (ed *Editor) drawTypeSelector(w, h int) {
	types := []string{DisplayTypeDFA, DisplayTypeNFA, DisplayTypeMoore, DisplayTypeMealy}
	
	// Position next to the main menu, aligned with the FSM Type menu item
	// Main menu is 40 wide, centred
	menuWidth := 40
	menuHeight := len(ed.menuItems) + 4
	menuX := (w - menuWidth) / 2
	menuY := (h - menuHeight) / 2
	if menuX < 0 {
		menuX = 0
	}
	if menuY < 0 {
		menuY = 0
	}
	
	// Find which menu item is FSM Type (index 8, 0-based)
	fsmTypeItemIndex := 8
	itemY := menuY + 2 + fsmTypeItemIndex
	
	// Position type selector to the right of menu
	boxW := 20
	boxH := len(types) + 2
	boxX := menuX + menuWidth + 1
	boxY := itemY - 1
	
	// If it would go off screen, position it differently
	if boxX + boxW > w - 1 {
		boxX = menuX - boxW - 1
		if boxX < 0 {
			boxX = menuX + menuWidth/2 - boxW/2
			boxY = menuY + menuHeight + 1
		}
	}

	ed.drawBox(boxX, boxY, boxW, boxH, styleDefault)

	// Interior width is boxW - 2 (for left and right borders)
	interiorW := boxW - 2
	for i, t := range types {
		style := styleMenu
		if i == ed.typeMenuSelected {
			style = styleMenuSel
		}
		// Pad to fill exact interior width
		line := fmt.Sprintf(" %-*s", interiorW-1, t)
		ed.drawString(boxX+1, boxY+1+i, line, style)
	}
}

func (ed *Editor) drawTransitionSelector(w, h int) {
	boxW := 35
	boxH := len(ed.validTargets) + 4
	if boxH > h-4 {
		boxH = h - 4
	}
	boxX := (w - boxW) / 2
	boxY := 3

	ed.drawBox(boxX, boxY, boxW, boxH, styleDefault)
	ed.drawString(boxX+2, boxY+1, "Select Target State:", styleSidebarH)

	for i, s := range ed.validTargets {
		if i >= boxH-4 {
			break
		}
		style := styleMenu
		if i == ed.menuSelected {
			style = styleMenuSel
		}
		line := fmt.Sprintf(" %-31s", truncate(s, 31))
		ed.drawString(boxX+2, boxY+3+i, line, style)
	}
}

func (ed *Editor) drawInputSelector(w, h int) {
	items := append(ed.fsm.Alphabet, "ε (epsilon)")
	boxW := 35
	boxH := len(items) + 4
	if boxH > h-4 {
		boxH = h - 4
	}
	boxX := (w - boxW) / 2
	boxY := 3

	ed.drawBox(boxX, boxY, boxW, boxH, styleDefault)
	ed.drawString(boxX+2, boxY+1, "Select Input:", styleSidebarH)

	for i, inp := range items {
		if i >= boxH-4 {
			break
		}
		style := styleMenu
		if i == ed.menuSelected {
			style = styleMenuSel
		}
		line := fmt.Sprintf(" %-31s", truncate(inp, 31))
		ed.drawString(boxX+2, boxY+3+i, line, style)
	}
}

func (ed *Editor) drawOutputSelector(w, h int) {
	boxW := 35
	boxH := len(ed.fsm.OutputAlphabet) + 4
	if boxH > h-4 {
		boxH = h - 4
	}
	boxX := (w - boxW) / 2
	boxY := 3

	ed.drawBox(boxX, boxY, boxW, boxH, styleDefault)
	title := "Select Output:"
	if ed.inputAction == nil {
		title = "Select Moore Output:"
	}
	ed.drawString(boxX+2, boxY+1, title, styleSidebarH)

	for i, out := range ed.fsm.OutputAlphabet {
		if i >= boxH-4 {
			break
		}
		style := styleMenu
		if i == ed.menuSelected {
			style = styleMenuSel
		}
		line := fmt.Sprintf(" %-31s", truncate(out, 31))
		ed.drawString(boxX+2, boxY+3+i, line, style)
	}
}

// drawHelpOverlay displays a comprehensive help window with all keyboard shortcuts
func (ed *Editor) drawHelpOverlay(w, h int) {
	// Help content organised by functional groups with full descriptions
	helpGroups := []struct {
		title string
		items [][2]string // key, description pairs
	}{
		{
			title: "Navigation",
			items: [][2]string{
				{"↑ ↓ ← →", "Move the cursor around the canvas"},
				{"Shift+↑↓←→", "Pan viewport (quick scroll without minimap)"},
				{"Tab", "Cycle selection through states"},
				{"Esc", "Return to main menu"},
				{"H / ?", "Show this help screen"},
			},
		},
		{
			title: "Canvas Navigation",
			items: [][2]string{
				{"Ctrl+D", "Enter canvas drag mode (shows minimap)"},
				{"Middle-drag", "Pan canvas with minimap overlay"},
				{"", "  Arrow keys pan viewport in drag mode"},
				{"", "  Esc or Ctrl+D to exit drag mode"},
				{"", "  Canvas is 512×512 logical units"},
			},
		},
		{
			title: "Creating States",
			items: [][2]string{
				{"Enter", "Add state at cursor (or dive into linked state)"},
				{"Right-click", "Add a new state at mouse position"},
			},
		},
		{
			title: "Editing States",
			items: [][2]string{
				{"S", "Set the selected state as the initial state"},
				{"A", "Toggle the selected state as an accepting state"},
				{"M", "Set Moore output for selected state (Moore only)"},
				{"K", "Link state to a machine (offers to create if none)"},
				{"P", "Edit properties for the selected state"},
				{"X", "Assign classes to states, edit property values"},
				{"B", "Open machine manager (bundle management)"},
				{"Del", "Delete the selected state and its transitions"},
				{"Double-click", "Edit state name (or dive into linked state)"},
			},
		},
		{
			title: "Linked State Navigation",
			items: [][2]string{
				{"Space", "Dive into linked state (when selected)"},
				{"Shift+→", "Dive into linked state (when selected)"},
				{"Shift+←", "Go back to parent machine"},
				{"Ctrl+B", "Go back to parent machine"},
				{"Enter", "Dive into linked state (or add state if not linked)"},
				{"Double-click", "Dive into linked state"},
				{"◀ button", "Click breadcrumb bar to navigate back"},
				{"", "  Breadcrumbs show: main › child › grandchild"},
			},
		},
		{
			title: "Moving States",
			items: [][2]string{
				{"G", "Grab selected state for keyboard movement"},
				{"", "  Then use ↑↓←→ to move, Enter to confirm, Esc to cancel"},
				{"Left-drag", "Drag a state to a new position with the mouse"},
			},
		},
		{
			title: "Transitions",
			items: [][2]string{
				{"T", "Add a transition from the selected state"},
				{"", "  Select target state, then choose input symbol"},
				{"I", "Add a new input symbol to the alphabet"},
				{"O", "Add a new output symbol (Mealy/Moore)"},
			},
		},
		{
			title: "Display Options",
			items: [][2]string{
				{"W", "Toggle visibility of transition arcs on the canvas"},
				{"R", "Render the FSM to an image file and open viewer"},
				{"\\", "Toggle sidebar collapse/expand"},
				{"", "  Drag divider to resize, snaps at default width"},
			},
		},
		{
			title: "Component Drawer",
			items: [][2]string{
				{"C", "Open the component drawer (bottom panel)"},
				{"[+] button", "Click the [+] button (bottom-right) to open"},
				{"", "  Tab / Shift+Tab: switch category"},
				{"", "  ← →: browse components within category"},
				{"", "  Enter: place selected component at canvas centre"},
				{"", "  Drag a card onto the canvas to drop it"},
				{"", "  Esc: close the drawer"},
				{"", "  Requires class libraries to be loaded in Settings"},
			},
		},
		{
			title: "Validation & Analysis",
			items: [][2]string{
				{"V", "Validate the FSM structure (check for errors)"},
				{"L", "Run analysis (reachability, dead states, etc.)"},
			},
		},
		{
			title: "Global Shortcuts",
			items: [][2]string{
				{"Ctrl+C", "Copy FSM to clipboard"},
				{"Ctrl+V", "Paste FSM from clipboard"},
				{"Ctrl+S", "Save the current file"},
				{"Ctrl+Z", "Undo the last action"},
				{"Ctrl+Y", "Redo a previously undone action"},
			},
		},
		{
			title: "Mouse Actions",
			items: [][2]string{
				{"Left-click", "Select a state / move cursor"},
				{"Left-drag", "Move a state by dragging"},
				{"Right-click", "Add a new state at mouse position"},
				{"Double-click", "Rename a state"},
			},
		},
		{
			title: "Menu Operations",
			items: [][2]string{
				{"New", "Start fresh (confirms if unsaved work exists)"},
				{"Open File", "Load an FSM from .fsm or .json file"},
				{"Import", "Add machine(s) from a file into the project"},
				{"", "  Promotes to bundle mode if currently single-FSM"},
				{"Machines", "Open machine manager (add, rename, delete, switch)"},
				{"Save / Save As", "Save the current FSM or bundle to a file"},
				{"Render", "Render to image and open in system viewer"},
				{"Settings", "Renderer, file type, FSM type, vocabulary, classes"},
			},
		},
		{
			title: "Settings Screen",
			items: [][2]string{
				{"← →", "Cycle setting values (renderer, type, vocabulary)"},
				{"Enter", "Browse for class library directory"},
				{"L", "Load class libraries from configured directory"},
				{"C", "Open the Class Editor (define/edit class schemas)"},
				{"Esc", "Save settings and return"},
			},
		},
		{
			title: "Bundle Mode",
			items: [][2]string{
				{"", "  The sidebar shows mode: Single FSM or Bundle [N machines]"},
				{"", "  In bundle mode, machines are listed in the sidebar header"},
				{"", "  Click a machine name in the sidebar to switch to it"},
				{"B / Machines", "Open machine manager to add, rename, delete, switch"},
				{"Import", "Imports from single files or multi-selects from bundles"},
				{"", "  Space toggles selection, A toggles all, Enter imports"},
				{"K", "Link a state to another machine (creates one if needed)"},
			},
		},
	}

	// Build a flat list of lines for scrolling
	type helpLine struct {
		isTitle bool
		isBlank bool
		key     string
		desc    string
		title   string
	}
	var lines []helpLine

	for i, g := range helpGroups {
		// Group title
		lines = append(lines, helpLine{isTitle: true, title: g.title})

		// Items
		for _, item := range g.items {
			lines = append(lines, helpLine{key: item[0], desc: item[1]})
		}

		// Blank line between groups (except after last)
		if i < len(helpGroups)-1 {
			lines = append(lines, helpLine{isBlank: true})
		}
	}

	ed.helpTotalLines = len(lines)

	// Calculate dimensions - use most of the screen
	boxWidth := w - 8
	if boxWidth > 90 {
		boxWidth = 90
	}
	if boxWidth < 50 {
		boxWidth = w - 4
	}

	boxHeight := h - 4
	if boxHeight < 10 {
		boxHeight = 10
	}

	startX := (w - boxWidth) / 2
	startY := (h - boxHeight) / 2
	if startX < 1 {
		startX = 1
	}
	if startY < 1 {
		startY = 1
	}

	// Content area dimensions (inside the box, minus title and footer)
	contentStartY := startY + 2
	contentHeight := boxHeight - 5 // title bar, footer, borders
	contentWidth := boxWidth - 4
	keyColWidth := 18

	// Adjust scroll offset bounds
	maxScroll := len(lines) - contentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if ed.helpScrollOffset > maxScroll {
		ed.helpScrollOffset = maxScroll
	}
	if ed.helpScrollOffset < 0 {
		ed.helpScrollOffset = 0
	}

	// Draw box with title
	ed.drawTitledBox(startX, startY, boxWidth, boxHeight, "Help - Keyboard Shortcuts & Commands")

	// Draw visible lines
	for i := 0; i < contentHeight; i++ {
		lineIdx := i + ed.helpScrollOffset
		if lineIdx >= len(lines) {
			break
		}

		y := contentStartY + i
		line := lines[lineIdx]

		if line.isBlank {
			continue
		}

		if line.isTitle {
			ed.drawString(startX+2, y, line.title, styleSidebarH)
		} else if line.key == "" {
			// Continuation line (indented description)
			desc := line.desc
			if len(desc) > contentWidth {
				desc = desc[:contentWidth]
			}
			ed.drawString(startX+2, y, desc, styleHelp)
		} else {
			// Normal key + description line
			keyStr := fmt.Sprintf("%-*s", keyColWidth, line.key)
			if len(keyStr) > keyColWidth {
				keyStr = keyStr[:keyColWidth]
			}
			ed.drawString(startX+2, y, keyStr, styleTrans)

			descStart := startX + 2 + keyColWidth
			maxDescLen := contentWidth - keyColWidth
			desc := line.desc
			if len(desc) > maxDescLen {
				desc = desc[:maxDescLen]
			}
			ed.drawString(descStart, y, desc, styleSidebar)
		}
	}

	// Draw scrollbar if content overflows
	needsScroll := len(lines) > contentHeight
	if needsScroll {
		scrollX := startX + boxWidth - 2

		// Calculate scrollbar thumb position and size
		thumbHeight := contentHeight * contentHeight / len(lines)
		if thumbHeight < 1 {
			thumbHeight = 1
		}

		thumbPos := 0
		if maxScroll > 0 {
			thumbPos = ed.helpScrollOffset * (contentHeight - thumbHeight) / maxScroll
		}

		// Draw scroll track and thumb
		for i := 0; i < contentHeight; i++ {
			y := contentStartY + i
			if i >= thumbPos && i < thumbPos+thumbHeight {
				// Thumb
				ed.screen.SetContent(scrollX, y, '█', nil, styleBorder)
			} else {
				// Track
				ed.screen.SetContent(scrollX, y, '░', nil, styleBorder)
			}
		}
	}

	// Footer with scroll hint if needed
	var footer string
	if needsScroll {
		footer = "↑↓/PgUp/PgDn: Scroll   Esc/Enter/Q: Close"
	} else {
		footer = "Press Esc, Enter, or Q to close"
	}
	footerX := startX + (boxWidth-len(footer))/2
	ed.drawString(footerX, startY+boxHeight-2, footer, styleHelp)
}

// drawMinimap draws a miniature overview of the 512x512 canvas
// showing state positions and the current viewport rectangle

func (ed *Editor) drawMachineSelector(w, h int) {
	if len(ed.machineList) == 0 {
		return
	}

	// Calculate box dimensions
	boxWidth := 60
	boxHeight := len(ed.machineList) + 6
	if boxHeight > h-4 {
		boxHeight = h - 4
	}
	if boxWidth > w-4 {
		boxWidth = w - 4
	}

	startX := (w - boxWidth) / 2
	startY := (h - boxHeight) / 2

	// Draw box
	ed.drawTitledBox(startX, startY, boxWidth, boxHeight, " Select Machine ")

	// Draw header
	headerY := startY + 2
	header := fmt.Sprintf("%-20s %-8s %6s %6s", "NAME", "TYPE", "STATES", "TRANS")
	ed.drawString(startX+2, headerY, header, styleSidebarH)

	// Draw machines
	visibleHeight := boxHeight - 5
	scrollOffset := 0
	if ed.machineSelected >= visibleHeight {
		scrollOffset = ed.machineSelected - visibleHeight + 1
	}

	for i := 0; i < visibleHeight && i+scrollOffset < len(ed.machineList); i++ {
		m := ed.machineList[i+scrollOffset]
		y := startY + 3 + i

		style := styleMenu
		if i+scrollOffset == ed.machineSelected {
			style = styleMenuSel
		}

		// Format line
		name := m.Name
		if len(name) > 20 {
			name = name[:17] + "..."
		}
		line := fmt.Sprintf("%-20s %-8s %6d %6d", name, m.Type, m.StateCount, m.TransCount)
		
		// Clear line and draw
		for x := startX + 1; x < startX+boxWidth-1; x++ {
			ed.screen.SetContent(x, y, ' ', nil, style)
		}
		ed.drawString(startX+2, y, line, style)
	}

	// Draw instructions
	footer := "↑↓: Select   Enter: Open   Esc: Cancel"
	footerX := startX + (boxWidth-len(footer))/2
	ed.drawString(footerX, startY+boxHeight-2, footer, styleHelp)
}

// drawLinkTargetSelector draws a selector for choosing a link target machine
func (ed *Editor) drawLinkTargetSelector(w, h int) {
	if len(ed.linkTargetMachines) == 0 {
		return
	}

	stateName := ""
	if ed.selectedState >= 0 && ed.selectedState < len(ed.fsm.States) {
		stateName = ed.fsm.States[ed.selectedState]
	}

	// Calculate box dimensions
	boxWidth := 40
	boxHeight := len(ed.linkTargetMachines) + 6
	if boxHeight > h-4 {
		boxHeight = h - 4
	}
	if boxWidth > w-4 {
		boxWidth = w - 4
	}

	startX := (w - boxWidth) / 2
	startY := (h - boxHeight) / 2

	// Draw box
	title := fmt.Sprintf(" Link %s to: ", stateName)
	ed.drawTitledBox(startX, startY, boxWidth, boxHeight, title)

	// Draw machines
	visibleHeight := boxHeight - 4
	scrollOffset := 0
	if ed.linkTargetSelected >= visibleHeight {
		scrollOffset = ed.linkTargetSelected - visibleHeight + 1
	}

	for i := 0; i < visibleHeight && i+scrollOffset < len(ed.linkTargetMachines); i++ {
		machineName := ed.linkTargetMachines[i+scrollOffset]
		y := startY + 2 + i

		style := styleMenu
		if i+scrollOffset == ed.linkTargetSelected {
			style = styleMenuSel
		}

		// Clear line and draw
		for x := startX + 1; x < startX+boxWidth-1; x++ {
			ed.screen.SetContent(x, y, ' ', nil, style)
		}
		ed.drawString(startX+3, y, machineName, style)
	}

	// Draw instructions
	footer := "↑↓: Select   Enter: Link   Esc: Cancel"
	footerX := startX + (boxWidth-len(footer))/2
	ed.drawString(footerX, startY+boxHeight-2, footer, styleHelp)
}

// drawImportMachineSelector draws a multi-select picker for importing machines from a bundle
func (ed *Editor) drawImportMachineSelector(w, h int) {
	if len(ed.importMachines) == 0 {
		return
	}

	boxWidth := 60
	boxHeight := len(ed.importMachines) + 6
	if boxHeight > h-4 {
		boxHeight = h - 4
	}
	if boxWidth > w-4 {
		boxWidth = w - 4
	}

	startX := (w - boxWidth) / 2
	startY := (h - boxHeight) / 2

	ed.drawTitledBox(startX, startY, boxWidth, boxHeight, " Import Machines ")

	// Count selected
	selectedCount := 0
	for _, s := range ed.importSelected {
		if s {
			selectedCount++
		}
	}
	header := fmt.Sprintf("Select machines to import (%d/%d):", selectedCount, len(ed.importMachines))
	ed.drawString(startX+2, startY+2, header, styleSidebarH)

	// Draw machines with checkboxes
	visibleHeight := boxHeight - 5
	scrollOffset := 0
	if ed.importCursor >= visibleHeight {
		scrollOffset = ed.importCursor - visibleHeight + 1
	}

	for i := 0; i < visibleHeight && i+scrollOffset < len(ed.importMachines); i++ {
		idx := i + scrollOffset
		name := ed.importMachines[idx]
		y := startY + 3 + i

		style := styleMenu
		if idx == ed.importCursor {
			style = styleMenuSel
		}

		checkbox := "☐ "
		if ed.importSelected[idx] {
			checkbox = "☑ "
		}

		line := checkbox + name
		if len(line) > boxWidth-4 {
			line = line[:boxWidth-7] + "..."
		}

		// Clear line
		for x := startX + 1; x < startX+boxWidth-1; x++ {
			ed.screen.SetContent(x, y, ' ', nil, style)
		}
		ed.drawString(startX+2, y, line, style)
	}

	footer := "Space: Toggle   A: All   Enter: Import   Esc: Cancel"
	footerX := startX + (boxWidth-len(footer))/2
	if footerX < startX+1 {
		footerX = startX + 1
	}
	ed.drawString(footerX, startY+boxHeight-2, footer, styleHelp)
}

// drawBreadcrumbBar draws the navigation breadcrumb bar at top of screen
