package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// Styles
var (
	styleDefault    = tcell.StyleDefault
	styleTitle      = tcell.StyleDefault.Bold(true).Foreground(tcell.ColorWhite)
	styleMenu       = tcell.StyleDefault.Foreground(tcell.ColorWhite)
	styleMenuSel    = tcell.StyleDefault.Background(tcell.ColorBlue).Foreground(tcell.ColorWhite)
	styleState      = tcell.StyleDefault.Foreground(tcell.ColorGreen)
	styleStateSel   = tcell.StyleDefault.Background(tcell.ColorGreen).Foreground(tcell.ColorBlack)
	styleStateInit  = tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
	styleStateAcc   = tcell.StyleDefault.Foreground(tcell.ColorPurple)
	styleTrans      = tcell.StyleDefault.Foreground(tcell.ColorTeal)
	styleTransDrag  = tcell.StyleDefault.Foreground(tcell.NewRGBColor(200, 162, 200)) // Lilac
	styleSidebar    = tcell.StyleDefault.Foreground(tcell.ColorWhite)
	styleSidebarH   = tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
	styleStatus     = tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorNavy)
	styleMsgInfo    = tcell.StyleDefault.Foreground(tcell.ColorWhite)
	styleMsgError   = tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true)
	styleMsgSuccess = tcell.StyleDefault.Foreground(tcell.ColorGreen)
	styleCursor     = tcell.StyleDefault.Background(tcell.ColorDarkGray)
	styleInput      = tcell.StyleDefault.Background(tcell.ColorNavy).Foreground(tcell.ColorWhite)
	styleBorder     = tcell.StyleDefault.Foreground(tcell.ColorGray)
	styleDragging   = tcell.StyleDefault.Background(tcell.ColorPurple).Foreground(tcell.ColorWhite)
)

func (ed *Editor) draw() {
	ed.screen.Clear()
	w, h := ed.screen.Size()

	switch ed.mode {
	case ModeMenu:
		ed.drawMenu(w, h)
	case ModeCanvas:
		ed.drawCanvas(w, h)
		ed.drawSidebar(w, h)
	case ModeInput:
		ed.drawCanvas(w, h)
		ed.drawSidebar(w, h)
		ed.drawInputBox(w, h)
	case ModeFilePicker:
		ed.drawFilePicker(w, h)
	case ModeSelectType:
		ed.drawTypeSelector(w, h)
	case ModeAddTransition:
		ed.drawCanvas(w, h)
		ed.drawSidebar(w, h)
		ed.drawTransitionSelector(w, h)
	case ModeSelectInput:
		ed.drawCanvas(w, h)
		ed.drawSidebar(w, h)
		ed.drawInputSelector(w, h)
	case ModeSelectOutput:
		ed.drawCanvas(w, h)
		ed.drawSidebar(w, h)
		ed.drawOutputSelector(w, h)
	}

	ed.drawStatusBar(w, h)
}

func (ed *Editor) drawMenu(w, h int) {
	// Title
	title := "╔══════════════════════════════╗"
	ed.drawString(10, 2, title, styleBorder)
	ed.drawString(10, 3, "║       FSM Editor             ║", styleBorder)
	ed.drawString(10, 4, "╚══════════════════════════════╝", styleBorder)

	// Menu items
	for i, item := range ed.menuItems {
		style := styleMenu
		if i == ed.menuSelected {
			style = styleMenuSel
		}
		line := fmt.Sprintf("  %-26s", item)
		ed.drawString(10, 6+i, line, style)
	}

	// Help
	help := "↑/↓: Select  Enter: Confirm  Esc: Cancel"
	ed.drawString(10, 6+len(ed.menuItems)+2, help, styleMsgInfo)
}

func (ed *Editor) drawCanvas(w, h int) {
	canvasW := w - ed.sidebarWidth
	canvasH := h - 2 // Leave room for status bar

	// Draw border
	for y := 0; y < canvasH; y++ {
		ed.screen.SetContent(canvasW, y, '│', nil, styleBorder)
	}

	// Draw transitions FIRST (so states render on top)
	if ed.showArcs {
		ed.drawTransitions(canvasW, canvasH)
	}

	// Draw states LAST (on top of arcs)
	for i, sp := range ed.states {
		x := sp.X - ed.canvasOffsetX
		y := sp.Y - ed.canvasOffsetY

		if x < 0 || x >= canvasW-4 || y < 0 || y >= canvasH {
			continue
		}

		// Determine style
		style := styleState
		prefix := "○"
		suffix := ""

		if ed.fsm.Initial == sp.Name {
			style = styleStateInit
			prefix = "→"
		}
		if ed.fsm.IsAccepting(sp.Name) {
			style = styleStateAcc
			suffix = "*"
		}
		if i == ed.selectedState {
			style = styleStateSel
		}

		label := fmt.Sprintf("%s[%s]%s", prefix, sp.Name, suffix)
		ed.drawString(x, y, label, style)

		// Draw Moore output if applicable
		if ed.fsm.Type == fsm.TypeMoore {
			if out, ok := ed.fsm.StateOutputs[sp.Name]; ok {
				ed.drawString(x+2, y+1, "/"+out, styleTrans)
			}
		}
	}

	// Draw cursor
	cx := ed.canvasCursorX - ed.canvasOffsetX
	cy := ed.canvasCursorY - ed.canvasOffsetY
	if cx >= 0 && cx < canvasW && cy >= 0 && cy < canvasH {
		ed.screen.SetContent(cx, cy, '+', nil, styleCursor)
	}
}

func (ed *Editor) drawTransitions(canvasW, canvasH int) {
	// Find state positions by name
	statePos := make(map[string]StatePos)
	for _, sp := range ed.states {
		statePos[sp.Name] = sp
	}

	// Choose style based on drag state
	lineStyle := styleTrans
	if ed.dragging {
		lineStyle = styleTransDrag
	}

	// Count transitions between each pair of states for offset calculation
	// Key: "from->to" or "to->from" (normalized), Value: count seen so far
	pairCount := make(map[string]int)
	pairIndex := make(map[string]int) // current index for this pair

	// First pass: count transitions between each pair
	for _, t := range ed.fsm.Transitions {
		for _, to := range t.To {
			if t.From == to {
				continue // self-loops handled separately
			}
			// Normalize key so A->B and B->A use same counter
			key := normalizePairKey(t.From, to)
			pairCount[key]++
		}
	}

	// Draw each transition
	for _, t := range ed.fsm.Transitions {
		fromSP, ok1 := statePos[t.From]
		if !ok1 {
			continue
		}

		for _, to := range t.To {
			toSP, ok2 := statePos[to]
			if !ok2 {
				continue
			}

			// Calculate positions (center of state boxes)
			fromX := fromSP.X - ed.canvasOffsetX + len(fromSP.Name)/2 + 2
			fromY := fromSP.Y - ed.canvasOffsetY
			toX := toSP.X - ed.canvasOffsetX + len(toSP.Name)/2 + 2
			toY := toSP.Y - ed.canvasOffsetY

			// Build label
			label := ""
			if t.Input != nil {
				label = *t.Input
			} else {
				label = "ε"
			}
			if ed.fsm.Type == fsm.TypeMealy && t.Output != nil {
				label += "/" + *t.Output
			}

			// Self-loop
			if t.From == to {
				ed.drawSelfLoop(fromX, fromY-1, label, canvasW, canvasH, lineStyle)
				continue
			}

			// Calculate offset for parallel arcs
			key := normalizePairKey(t.From, to)
			total := pairCount[key]
			idx := pairIndex[key]
			pairIndex[key]++

			// Offset: center around 0, spread by 2 units
			offset := 0
			if total > 1 {
				offset = (idx - (total-1)/2) * 2
				if total%2 == 0 {
					offset = (idx - total/2) * 2 + 1
				}
			}

			// Draw the arc with offset
			ed.drawArcWithOffset(fromX, fromY, toX, toY, label, offset, canvasW, canvasH, lineStyle)
		}
	}
}

// normalizePairKey returns a consistent key for a pair of states
func normalizePairKey(a, b string) string {
	if a < b {
		return a + "->" + b
	}
	return b + "->" + a
}

func (ed *Editor) drawSelfLoop(x, y int, label string, canvasW, canvasH int, style tcell.Style) {
	// Draw a small loop above the state
	//  ╭─╮
	//  │a│
	//  ╰─╯
	if y < 1 || x < 1 || x >= canvasW-3 {
		return
	}
	
	// Top
	if y-1 >= 0 {
		ed.screen.SetContent(x-1, y-1, '╭', nil, style)
		ed.screen.SetContent(x, y-1, '─', nil, style)
		ed.screen.SetContent(x+1, y-1, '╮', nil, style)
	}
	// Sides with label
	if y >= 0 && y < canvasH {
		ed.screen.SetContent(x-1, y, '│', nil, style)
		// Draw label in middle
		for i, r := range label {
			if x+i < canvasW {
				ed.screen.SetContent(x+i, y, r, nil, style)
			}
		}
		if x+len(label) < canvasW {
			ed.screen.SetContent(x+len(label), y, '│', nil, style)
		}
	}
	// Bottom connects back
	if y+1 < canvasH {
		ed.screen.SetContent(x-1, y+1, '╰', nil, style)
		ed.screen.SetContent(x, y+1, '→', nil, style)
	}
}

func (ed *Editor) drawArc(fromX, fromY, toX, toY int, label string, canvasW, canvasH int, style tcell.Style) {
	ed.drawArcWithOffset(fromX, fromY, toX, toY, label, 0, canvasW, canvasH, style)
}

func (ed *Editor) drawArcWithOffset(fromX, fromY, toX, toY int, label string, offset int, canvasW, canvasH int, style tcell.Style) {
	// Determine direction
	dx := toX - fromX
	dy := toY - fromY

	if dy == 0 {
		// Horizontal line - offset vertically
		ed.drawHorizontalArc(fromX, fromY+offset, toX, label, canvasW, canvasH, style)
	} else if dx == 0 {
		// Vertical line - offset horizontally
		ed.drawVerticalArc(fromX+offset, fromY, toY, label, canvasW, canvasH, style)
	} else {
		// Diagonal - use L-shaped path with offset
		ed.drawLShapedArcWithOffset(fromX, fromY, toX, toY, label, offset, canvasW, canvasH, style)
	}
}

func (ed *Editor) drawHorizontalArc(fromX, y, toX int, label string, canvasW, canvasH int, style tcell.Style) {
	if y < 0 || y >= canvasH {
		return
	}

	minX, maxX := fromX, toX
	goingRight := true
	if fromX > toX {
		minX, maxX = toX, fromX
		goingRight = false
	}

	// Draw line
	for x := minX + 1; x < maxX; x++ {
		if x >= 0 && x < canvasW {
			ed.screen.SetContent(x, y, '─', nil, style)
		}
	}

	// Draw arrow at destination
	if goingRight {
		if maxX-1 >= 0 && maxX-1 < canvasW {
			ed.screen.SetContent(maxX-1, y, '→', nil, style)
		}
	} else {
		if minX+1 >= 0 && minX+1 < canvasW {
			ed.screen.SetContent(minX+1, y, '←', nil, style)
		}
	}

	// Draw label at midpoint
	midX := (minX + maxX) / 2 - len(label)/2
	if y > 0 {
		ed.drawLabel(midX, y-1, label, canvasW, canvasH, style)
	}
}

func (ed *Editor) drawVerticalArc(x, fromY, toY int, label string, canvasW, canvasH int, style tcell.Style) {
	if x < 0 || x >= canvasW {
		return
	}

	minY, maxY := fromY, toY
	goingDown := true
	if fromY > toY {
		minY, maxY = toY, fromY
		goingDown = false
	}

	// Draw line
	for y := minY + 1; y < maxY; y++ {
		if y >= 0 && y < canvasH {
			ed.screen.SetContent(x, y, '│', nil, style)
		}
	}

	// Draw arrow at destination
	if goingDown {
		if maxY-1 >= 0 && maxY-1 < canvasH {
			ed.screen.SetContent(x, maxY-1, '↓', nil, style)
		}
	} else {
		if minY+1 >= 0 && minY+1 < canvasH {
			ed.screen.SetContent(x, minY+1, '↑', nil, style)
		}
	}

	// Draw label beside midpoint
	midY := (minY + maxY) / 2
	ed.drawLabel(x+1, midY, label, canvasW, canvasH, style)
}

func (ed *Editor) drawLShapedArc(fromX, fromY, toX, toY int, label string, canvasW, canvasH int, style tcell.Style) {
	ed.drawLShapedArcWithOffset(fromX, fromY, toX, toY, label, 0, canvasW, canvasH, style)
}

func (ed *Editor) drawLShapedArcWithOffset(fromX, fromY, toX, toY int, label string, offset int, canvasW, canvasH int, style tcell.Style) {
	// Decide corner position - go horizontal first, then vertical
	// Apply offset to the corner to separate parallel arcs
	cornerX := toX + offset
	cornerY := fromY

	// Horizontal segment
	if fromX != cornerX {
		minX, maxX := fromX, cornerX
		if fromX > cornerX {
			minX, maxX = cornerX, fromX
		}
		for x := minX + 1; x < maxX; x++ {
			if x >= 0 && x < canvasW && cornerY >= 0 && cornerY < canvasH {
				ed.screen.SetContent(x, cornerY, '─', nil, style)
			}
		}
	}

	// Corner
	if cornerX >= 0 && cornerX < canvasW && cornerY >= 0 && cornerY < canvasH {
		var cornerChar rune
		if toX > fromX && toY > fromY {
			cornerChar = '╮' // going right then down
		} else if toX > fromX && toY < fromY {
			cornerChar = '╯' // going right then up
		} else if toX < fromX && toY > fromY {
			cornerChar = '╭' // going left then down
		} else {
			cornerChar = '╰' // going left then up
		}
		ed.screen.SetContent(cornerX, cornerY, cornerChar, nil, style)
	}

	// Vertical segment from corner to target
	targetX := toX
	if cornerY != toY {
		minY, maxY := cornerY, toY
		goingDown := true
		if cornerY > toY {
			minY, maxY = toY, cornerY
			goingDown = false
		}
		for y := minY + 1; y < maxY; y++ {
			if cornerX >= 0 && cornerX < canvasW && y >= 0 && y < canvasH {
				ed.screen.SetContent(cornerX, y, '│', nil, style)
			}
		}

		// If offset, need a horizontal segment to reach target
		if offset != 0 && cornerX != targetX {
			// Draw connector from corner column to target column at target row
			connY := toY
			if goingDown {
				connY = maxY - 1
			} else {
				connY = minY + 1
			}
			
			minCX, maxCX := cornerX, targetX
			if cornerX > targetX {
				minCX, maxCX = targetX, cornerX
			}
			for cx := minCX; cx <= maxCX; cx++ {
				if cx >= 0 && cx < canvasW && connY >= 0 && connY < canvasH {
					if cx == minCX || cx == maxCX {
						// corners or endpoints
						continue
					}
					ed.screen.SetContent(cx, connY, '─', nil, style)
				}
			}
		}

		// Arrow at end
		if goingDown {
			if cornerX >= 0 && cornerX < canvasW && maxY-1 >= 0 && maxY-1 < canvasH {
				ed.screen.SetContent(cornerX, maxY-1, '↓', nil, style)
			}
		} else {
			if cornerX >= 0 && cornerX < canvasW && minY+1 >= 0 && minY+1 < canvasH {
				ed.screen.SetContent(cornerX, minY+1, '↑', nil, style)
			}
		}
	}

	// Label near the corner
	labelX := (fromX + cornerX) / 2 - len(label)/2
	labelY := cornerY - 1
	if labelY < 0 {
		labelY = cornerY + 1
	}
	ed.drawLabel(labelX, labelY, label, canvasW, canvasH, style)
}

func (ed *Editor) drawLabel(x, y int, label string, canvasW, canvasH int, style tcell.Style) {
	if y < 0 || y >= canvasH {
		return
	}
	for i, r := range label {
		if x+i >= 0 && x+i < canvasW {
			ed.screen.SetContent(x+i, y, r, nil, style)
		}
	}
}

func (ed *Editor) drawSidebar(w, h int) {
	x := w - ed.sidebarWidth + 2
	y := 0

	// Title
	title := fmt.Sprintf("FSM: %s", ed.fsm.Type)
	if ed.fsm.Name != "" {
		title = ed.fsm.Name + " (" + string(ed.fsm.Type) + ")"
	}
	ed.drawString(x, y, truncate(title, ed.sidebarWidth-4), styleSidebarH)
	y += 2

	// States
	ed.drawString(x, y, "States:", styleSidebarH)
	y++
	for i, s := range ed.fsm.States {
		prefix := "  "
		suffix := ""
		if s == ed.fsm.Initial {
			prefix = "→ "
		}
		if ed.fsm.IsAccepting(s) {
			suffix = " *"
		}
		style := styleSidebar
		if i == ed.selectedState {
			style = styleMenuSel
		}
		line := truncate(prefix+s+suffix, ed.sidebarWidth-4)
		ed.drawString(x, y, line, style)
		y++
	}
	y++

	// Inputs
	ed.drawString(x, y, "Inputs:", styleSidebarH)
	y++
	for _, inp := range ed.fsm.Alphabet {
		ed.drawString(x, y, "  "+truncate(inp, ed.sidebarWidth-6), styleSidebar)
		y++
	}
	y++

	// Outputs (if applicable)
	if len(ed.fsm.OutputAlphabet) > 0 {
		ed.drawString(x, y, "Outputs:", styleSidebarH)
		y++
		for _, out := range ed.fsm.OutputAlphabet {
			ed.drawString(x, y, "  "+truncate(out, ed.sidebarWidth-6), styleSidebar)
			y++
		}
		y++
	}

	// Transitions
	ed.drawString(x, y, "Transitions:", styleSidebarH)
	y++
	for _, t := range ed.fsm.Transitions {
		inp := "ε"
		if t.Input != nil {
			inp = *t.Input
		}
		for _, to := range t.To {
			line := fmt.Sprintf("  %s --%s--> %s", t.From, inp, to)
			if ed.fsm.Type == fsm.TypeMealy && t.Output != nil {
				line += " [" + *t.Output + "]"
			}
			ed.drawString(x, y, truncate(line, ed.sidebarWidth-4), styleSidebar)
			y++
			if y >= h-3 {
				ed.drawString(x, y, "  ...", styleSidebar)
				return
			}
		}
	}
}

func (ed *Editor) drawStatusBar(w, h int) {
	y := h - 1

	// Background
	for x := 0; x < w; x++ {
		ed.screen.SetContent(x, y, ' ', nil, styleStatus)
	}

	// File info
	fileInfo := "[New]"
	if ed.filename != "" {
		fileInfo = ed.filename
	}
	if ed.modified {
		fileInfo += " *"
	}
	ed.drawString(1, y, fileInfo, styleStatus)

	// Mode
	modeStr := ed.modeString()
	ed.drawString(w/2-len(modeStr)/2, y, modeStr, styleStatus)

	// Message
	if ed.message != "" {
		style := styleMsgInfo
		switch ed.messageType {
		case MsgError:
			style = styleMsgError
		case MsgSuccess:
			style = styleMsgSuccess
		}
		ed.drawString(w-len(ed.message)-2, y, ed.message, style)
	}

	// Help bar
	y = h - 2
	for x := 0; x < w; x++ {
		ed.screen.SetContent(x, y, ' ', nil, styleDefault)
	}
	help := ed.helpString()
	ed.drawString(1, y, help, styleMsgInfo)
}

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
	boxW := 40
	boxH := len(ed.fileList) + 4
	if boxH > h-4 {
		boxH = h - 4
	}
	boxX := (w - boxW) / 2
	boxY := 3

	ed.drawBox(boxX, boxY, boxW, boxH, styleDefault)
	ed.drawString(boxX+2, boxY+1, "Select File:", styleSidebarH)

	for i, f := range ed.fileList {
		if i >= boxH-4 {
			break
		}
		style := styleMenu
		if i == ed.fileSelected {
			style = styleMenuSel
		}
		line := fmt.Sprintf(" %-36s", truncate(f, 36))
		ed.drawString(boxX+2, boxY+3+i, line, style)
	}
}

func (ed *Editor) drawTypeSelector(w, h int) {
	types := []string{"DFA", "NFA", "Moore", "Mealy"}
	boxW := 30
	boxH := len(types) + 4
	boxX := (w - boxW) / 2
	boxY := 5

	ed.drawBox(boxX, boxY, boxW, boxH, styleDefault)
	ed.drawString(boxX+2, boxY+1, "Select FSM Type:", styleSidebarH)

	for i, t := range types {
		style := styleMenu
		if i == ed.menuSelected {
			style = styleMenuSel
		}
		line := fmt.Sprintf(" %-26s", t)
		ed.drawString(boxX+2, boxY+3+i, line, style)
	}
}

func (ed *Editor) drawTransitionSelector(w, h int) {
	boxW := 35
	boxH := len(ed.fsm.States) + 4
	if boxH > h-4 {
		boxH = h - 4
	}
	boxX := (w - boxW) / 2
	boxY := 3

	ed.drawBox(boxX, boxY, boxW, boxH, styleDefault)
	ed.drawString(boxX+2, boxY+1, "Select Target State:", styleSidebarH)

	for i, s := range ed.fsm.States {
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

func (ed *Editor) drawBox(x, y, w, h int, style tcell.Style) {
	// Corners
	ed.screen.SetContent(x, y, '┌', nil, styleBorder)
	ed.screen.SetContent(x+w-1, y, '┐', nil, styleBorder)
	ed.screen.SetContent(x, y+h-1, '└', nil, styleBorder)
	ed.screen.SetContent(x+w-1, y+h-1, '┘', nil, styleBorder)

	// Horizontal borders
	for i := x + 1; i < x+w-1; i++ {
		ed.screen.SetContent(i, y, '─', nil, styleBorder)
		ed.screen.SetContent(i, y+h-1, '─', nil, styleBorder)
	}

	// Vertical borders
	for i := y + 1; i < y+h-1; i++ {
		ed.screen.SetContent(x, i, '│', nil, styleBorder)
		ed.screen.SetContent(x+w-1, i, '│', nil, styleBorder)
	}

	// Fill
	for row := y + 1; row < y+h-1; row++ {
		for col := x + 1; col < x+w-1; col++ {
			ed.screen.SetContent(col, row, ' ', nil, style)
		}
	}
}

func (ed *Editor) drawString(x, y int, s string, style tcell.Style) {
	for i, r := range s {
		ed.screen.SetContent(x+i, y, r, nil, style)
	}
}

func (ed *Editor) modeString() string {
	if ed.dragging {
		return "DRAGGING"
	}
	switch ed.mode {
	case ModeMenu:
		return "MENU"
	case ModeCanvas:
		return "CANVAS"
	case ModeInput:
		return "INPUT"
	case ModeFilePicker:
		return "FILE SELECT"
	case ModeSelectType:
		return "SELECT TYPE"
	case ModeAddTransition:
		return "ADD TRANSITION"
	case ModeSelectInput:
		return "SELECT INPUT"
	case ModeSelectOutput:
		return "SELECT OUTPUT"
	default:
		return ""
	}
}

func (ed *Editor) helpString() string {
	switch ed.mode {
	case ModeMenu:
		return "↑↓:Select  Enter:Confirm  Esc:Canvas"
	case ModeCanvas:
		return "Arrows:Move  Enter:Add State  Tab:Cycle  T:Transition  I:Input  O:Output  S:Initial  A:Accept  Del:Delete  Esc:Menu"
	case ModeInput:
		return "Type text  Enter:Confirm  Esc:Cancel"
	case ModeFilePicker:
		return "↑↓:Select  Enter:Open  Esc:Cancel"
	case ModeSelectType:
		return "↑↓:Select  Enter:Confirm  Esc:Cancel"
	case ModeAddTransition, ModeSelectInput, ModeSelectOutput:
		return "↑↓:Select  Enter:Confirm  Esc:Cancel"
	default:
		return "Ctrl+S:Save  Ctrl+C:Quit"
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
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

// Override completeSelectOutput to handle Moore case
func init() {
	// This is handled in the main file's completeSelectOutput
}
