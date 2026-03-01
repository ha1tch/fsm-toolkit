// Canvas and mouse event handlers for fsmedit.
package main

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

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
	// Check for Shift+Arrow for viewport panning or bundle navigation
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
			// Navigate back in bundle if possible, otherwise pan
			if ed.isBundle && len(ed.navStack) > 0 {
				ed.navigateBack()
			} else {
				ed.panViewport(-1, 0)
			}
			return false
		case tcell.KeyRight:
			// Dive into linked state if possible, otherwise pan
			if ed.selectedState >= 0 && ed.selectedState < len(ed.states) &&
				ed.fsm.IsLinked(ed.states[ed.selectedState].Name) {
				if ed.isBundle {
					ed.diveIntoLinkedState(ed.selectedState)
				} else {
					ed.showMessage("Link navigation requires bundle mode", MsgInfo)
				}
			} else {
				ed.panViewport(1, 0)
			}
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
		// If selected state is linked, dive into it; otherwise add state
		if ed.selectedState >= 0 && ed.selectedState < len(ed.states) && ed.fsm.IsLinked(ed.states[ed.selectedState].Name) {
			if ed.isBundle {
				ed.diveIntoLinkedState(ed.selectedState)
			} else {
				ed.showMessage("Link navigation requires bundle mode", MsgInfo)
			}
		} else {
			ed.addStateAtCursor()
		}
	case tcell.KeyDelete, tcell.KeyBackspace, tcell.KeyBackspace2:
		ed.deleteSelected()
	case tcell.KeyTab:
		ed.cycleSelection()
	case tcell.KeyCtrlB:
		// Navigate back in linked state hierarchy
		if len(ed.navStack) > 0 {
			ed.navigateBack()
		}
	case tcell.KeyRune:
		switch ev.Rune() {
		case ' ':
			// Space: dive into linked state in bundle mode
			if ed.selectedState >= 0 && ed.selectedState < len(ed.states) &&
				ed.fsm.IsLinked(ed.states[ed.selectedState].Name) {
				if ed.isBundle {
					ed.diveIntoLinkedState(ed.selectedState)
				} else {
					ed.showMessage("Link navigation requires bundle mode", MsgInfo)
				}
			}
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
		case 'k', 'K':
			// Set/toggle linked state (only in bundle mode or for creating new links)
			if ed.selectedState >= 0 {
				ed.setLinkedMachine()
			} else {
				ed.showMessage("Select a state first", MsgInfo)
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
		case 'c', 'C':
			ed.openDrawer()
		case 'p', 'P':
			if ed.selectedState >= 0 && ed.selectedState < len(ed.states) {
				stateName := ed.states[ed.selectedState].Name
				machineName := "(current)"
				if ed.isBundle && ed.currentMachine != "" {
					machineName = ed.currentMachine
				}
				ed.openPropertyEditor(machineName, stateName)
			} else {
				ed.showMessage("Select a state first", MsgInfo)
			}
		case 'x', 'X':
			ed.openClassAssign()
		case 'b', 'B':
			ed.openMachineManager()
		case '\\':
			// Toggle sidebar collapse
			ed.toggleSidebarCollapse()
		}
	}
	return false
}


func (ed *Editor) handleMouse(ev *tcell.EventMouse) {
	x, y := ev.Position()
	buttons := ev.Buttons()

	w, h := ed.screen.Size()
	dividerX := w - ed.sidebarWidth
	canvasW := dividerX

	// Component drawer mouse handling (drag, button, card clicks).
	if ed.drawerDragging || ed.drawerOpen || len(ed.catalog) > 0 {
		if ed.handleDrawerMouse(ev, w, h) {
			return
		}
	}

	// Block mouse events from reaching the canvas during modal overlays.
	switch ed.mode {
	case ModeClassEditor:
		ed.handleClassEditorMouse(ev, w, h)
		return
	case ModeMachineManager:
		ed.handleMachineManagerMouse(ev, w, h)
		return
	case ModeMenu, ModeInput, ModeFilePicker, ModeSelectType,
		ModeAddTransition, ModeSelectInput, ModeSelectOutput,
		ModeHelp, ModeSelectMachine, ModeSelectLinkTarget,
		ModeImportMachineSelect, ModeClassAssign,
		ModePropertyEditor, ModeListEditor, ModeSettings:
		return // Consume mouse events — don't let them reach canvas.
	}

	// Handle breadcrumb bar clicks (if visible)
	if len(ed.navStack) > 0 && ed.isBundle && y == 0 {
		if buttons&tcell.Button1 != 0 && !ed.leftMouseDown {
			// Check if clicking back button (first 3 chars)
			if x < 3 {
				ed.navigateBack()
				ed.leftMouseDown = true
				return
			}
			
			// Check if clicking a breadcrumb segment
			crumbs := ed.getBreadcrumbs()
			crumbX := 4 // Start after back button
			for i, crumb := range crumbs {
				if i > 0 {
					crumbX += 3 // " › " separator
				}
				crumbEnd := crumbX + len(crumb)
				if x >= crumbX && x < crumbEnd {
					if i < len(crumbs)-1 {
						// Click on ancestor - navigate to it
						ed.navigateToBreadcrumb(i)
						ed.leftMouseDown = true
						return
					}
					// Click on current machine - do nothing
					break
				}
				crumbX = crumbEnd
			}
		}
		return
	}

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
			// In bundle mode, check if clicking on a machine name in the header
			if ed.isBundle && y >= 2 && y < 2+len(ed.bundleMachines) {
				machineIdx := y - 2
				if machineIdx >= 0 && machineIdx < len(ed.bundleMachines) {
					targetMachine := ed.bundleMachines[machineIdx]
					if targetMachine != ed.currentMachine {
						ed.saveMachineToCache()
						ed.loadMachineFromCache(targetMachine)
						ed.showMessage("Switched to: "+targetMachine, MsgSuccess)
					}
				}
				return
			}
			
			// Calculate fixed header height for content offset
			fixedHeaderLines := 2
			if ed.isBundle {
				fixedHeaderLines = 2 + len(ed.bundleMachines) + 1
			}
			
			// Only process content clicks below the fixed header
			if y < fixedHeaderLines {
				return
			}
			
			// Convert screen Y to content line index (accounting for scroll and header)
			contentY := (y - fixedHeaderLines) + ed.sidebarScrollY
			
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
						// Double-click detected
						// If state is linked and we're in a bundle, dive into it
						if clickedState >= 0 && clickedState < len(ed.states) {
							stateName := ed.states[clickedState].Name
							if ed.fsm.IsLinked(stateName) {
								if ed.isBundle {
									ed.diveIntoLinkedState(clickedState)
									ed.lastClickTime = 0
									ed.lastClickState = -1
									return
								}
								ed.showMessage("Link navigation requires bundle mode", MsgInfo)
								ed.lastClickTime = 0
								ed.lastClickState = -1
								return
							}
						}
						// Otherwise edit state name
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
