// Top-level draw dispatch for fsmedit.
package main

import (
	"fmt"
	"time"
)

func (ed *Editor) draw() {
	ed.screen.Clear()
	w, h := ed.screen.Size()

	// Complete zoom animation before drawing if it has elapsed.
	// This ensures the machine switch takes effect in the same frame,
	// so canvas and sidebar draw with the new machine data.
	if ed.animating {
		elapsed := time.Now().UnixMilli() - ed.animStartTime
		if elapsed >= ed.animDuration {
			ed.finishAnimation()
		}
	}

	// Draw breadcrumb bar if in navigation stack
	breadcrumbHeight := 0
	if len(ed.navStack) > 0 && ed.isBundle {
		ed.drawBreadcrumbBar(w)
		breadcrumbHeight = 1
	}

	// Draw canvas and sidebar in canvas-related modes, even if empty
	if ed.mode == ModeCanvas || ed.mode == ModeMove || 
	   (ed.fsm != nil && len(ed.states) > 0) {
		ed.drawCanvas(w, h)
		ed.drawSidebar(w, h)
	}

	switch ed.mode {
	case ModeMenu:
		ed.drawMenuOverlay(w, h)
	case ModeCanvas, ModeMove:
		// Canvas already drawn above
	case ModeInput:
		ed.drawInputBox(w, h)
	case ModeFilePicker:
		ed.drawFilePicker(w, h)
	case ModeSelectType:
		ed.drawMenuOverlay(w, h)
		ed.drawTypeSelector(w, h)
	case ModeAddTransition:
		ed.drawTransitionSelector(w, h)
	case ModeSelectInput:
		ed.drawInputSelector(w, h)
	case ModeSelectOutput:
		ed.drawOutputSelector(w, h)
	case ModeHelp:
		ed.drawCanvas(w, h)
		ed.drawSidebar(w, h)
		ed.drawHelpOverlay(w, h)
	case ModeCanvasDrag:
		ed.drawCanvas(w, h)
		ed.drawSidebar(w, h)
		ed.drawMinimap(w, h)
	case ModeSelectMachine:
		ed.drawMachineSelector(w, h)
	case ModeSelectLinkTarget:
		ed.drawCanvas(w, h)
		ed.drawSidebar(w, h)
		ed.drawLinkTargetSelector(w, h)
	case ModeImportMachineSelect:
		ed.drawCanvas(w, h)
		ed.drawSidebar(w, h)
		ed.drawImportMachineSelector(w, h)
	case ModeClassEditor:
		ed.drawCanvas(w, h)
		ed.drawSidebar(w, h)
		ed.drawClassEditor(w, h)
	case ModeClassAssign:
		ed.drawCanvas(w, h)
		ed.drawSidebar(w, h)
		ed.drawClassAssign(w, h)
	case ModePropertyEditor:
		ed.drawCanvas(w, h)
		ed.drawSidebar(w, h)
		ed.drawPropertyEditor(w, h)
	case ModeListEditor:
		ed.drawCanvas(w, h)
		ed.drawSidebar(w, h)
		ed.drawPropertyEditor(w, h)
		ed.drawListEditor(w, h)
	case ModeSettings:
		ed.drawCanvas(w, h)
		ed.drawSidebar(w, h)
		ed.drawSettings(w, h)
	case ModeDrawer:
		ed.drawCanvas(w, h)
		ed.drawSidebar(w, h)
	case ModeMachineManager:
		ed.drawCanvas(w, h)
		ed.drawSidebar(w, h)
		ed.drawMachineManager(w, h)
	}

	// Check drawer animation completion.
	ed.checkDrawerAnimation()

	// Draw component drawer (over status bar area, before status bar).
	ed.drawDrawer(w, h)
	ed.drawDragGhost(w, h)

	// Draw zoom animation overlay if animating
	if ed.animating {
		ed.drawZoomAnimation(w, h)
	}

	ed.drawStatusBar(w, h)

	// Draw [+] button after status bar (on top of it).
	ed.drawDrawerButton(w, h)

	_ = breadcrumbHeight // used for canvas offset calculation
}

func (ed *Editor) drawMenuOverlay(w, h int) {
	// Menu dimensions - much wider for comfortable display
	menuWidth := 40
	menuHeight := len(ed.menuItems) + 4
	
	// Centre on screen
	startX := (w - menuWidth) / 2
	startY := (h - menuHeight) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}

	// Draw box
	ed.drawTitledBox(startX, startY, menuWidth, menuHeight, "fsmedit")

	// Menu items
	for i, item := range ed.menuItems {
		style := styleMenu
		if i == ed.menuSelected {
			style = styleMenuSel
		}
		x := startX + 1
		y := startY + 2 + i
		// Pad item to fill full width inside box (menuWidth - 2 for borders)
		paddedItem := fmt.Sprintf(" %-*s", menuWidth-3, item)
		ed.drawString(x, y, paddedItem, style)
	}
}

func (ed *Editor) drawMenu(w, h int) {
	// Legacy - redirect to overlay
	ed.drawMenuOverlay(w, h)
}

