// Canvas, sidebar, status bar, minimap, and transition rendering for fsmedit.
package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

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

	// Draw nets (structural connections) between transitions and states
	if ed.showNets {
		ed.drawNets(canvasW, canvasH)
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

		isLinked := ed.fsm.IsLinked(sp.Name)
		
		if isLinked {
			style = styleStateLinked
			suffix = "↗" // Arrow indicating link to another machine
		}
		if ed.fsm.Initial == sp.Name {
			style = styleStateInit
			prefix = "→"
		}
		if ed.fsm.IsAccepting(sp.Name) {
			if !isLinked {
				style = styleStateAcc
			}
			suffix = "*"
		}
		if i == ed.selectedState {
			style = styleStateSel
		}
		// Highlight state being dragged (mouse or keyboard)
		if ed.dragging && i == ed.dragStateIdx {
			style = styleDragging
		}

		label := fmt.Sprintf("%s[%s]%s", prefix, sp.Name, suffix)
		ed.drawString(x, y, label, style)

		// Draw linked machine name below state if linked
		if isLinked {
			targetMachine := ed.fsm.GetLinkedMachine(sp.Name)
			if targetMachine != "" && y+1 < canvasH {
				ed.drawString(x+2, y+1, "→"+targetMachine, styleStateLinked)
			}
		} else if ed.fsm.Type == fsm.TypeMoore {
			// Draw Moore output if applicable
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

	// Draw scroll indicators if content exists beyond viewport
	ed.drawScrollIndicators(canvasW, canvasH)
}

// drawScrollIndicators shows arrows at edges when content exists off-screen
func (ed *Editor) drawScrollIndicators(canvasW, canvasH int) {
	styleIndicator := tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
	
	// Check for content beyond each edge
	hasLeft := false
	hasRight := false
	hasTop := false
	hasBottom := false
	
	for _, sp := range ed.states {
		if sp.X < ed.canvasOffsetX {
			hasLeft = true
		}
		if sp.X > ed.canvasOffsetX+canvasW {
			hasRight = true
		}
		if sp.Y < ed.canvasOffsetY {
			hasTop = true
		}
		if sp.Y > ed.canvasOffsetY+canvasH {
			hasBottom = true
		}
	}
	
	// Also check if viewport is scrolled (content might be there even without states)
	if ed.canvasOffsetX > 0 {
		hasLeft = true
	}
	if ed.canvasOffsetY > 0 {
		hasTop = true
	}
	if ed.canvasOffsetX+canvasW < CanvasMaxWidth {
		hasRight = true
	}
	if ed.canvasOffsetY+canvasH < CanvasMaxHeight {
		hasBottom = true
	}
	
	// Draw indicators at edges (subtle, near corners)
	if hasLeft {
		ed.screen.SetContent(0, canvasH/2, '◀', nil, styleIndicator)
	}
	if hasRight {
		ed.screen.SetContent(canvasW-1, canvasH/2, '▶', nil, styleIndicator)
	}
	if hasTop {
		ed.screen.SetContent(canvasW/2, 0, '▲', nil, styleIndicator)
	}
	if hasBottom {
		ed.screen.SetContent(canvasW/2, canvasH-1, '▼', nil, styleIndicator)
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

	// Check if we're flashing an input (no time limit - cleared by other actions)
	flashingInput := ed.flashInput

	// Check if we're flashing an output
	flashingOutput := ed.flashOutput

	// Check if we're flashing a specific transition
	flashingTransIdx := ed.flashTransIdx

	// Flash styles - alternate between bright white and bright blue
	flashStyleWhite := tcell.StyleDefault.Foreground(tcell.ColorWhite).Bold(true)
	flashStyleBlue := tcell.StyleDefault.Foreground(tcell.ColorBlue).Bold(true)

	// Determine which flash style to use based on current blink phase
	now := time.Now().UnixMilli()
	getFlashStyle := func(startTime int64) tcell.Style {
		elapsed := now - startTime
		// Alternate every 200ms between white and blue
		if (elapsed/200)%2 == 0 {
			return flashStyleWhite
		}
		return flashStyleBlue
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
	for tIdx, t := range ed.fsm.Transitions {
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

			// Determine style - flash if this transition matches any flash criteria
			arcStyle := lineStyle
			if flashingInput != "" && t.Input != nil && *t.Input == flashingInput {
				arcStyle = getFlashStyle(ed.flashInputTime)
			} else if flashingOutput != "" && t.Output != nil && *t.Output == flashingOutput {
				arcStyle = getFlashStyle(ed.flashOutputTime)
			} else if flashingTransIdx == tIdx {
				arcStyle = getFlashStyle(ed.flashTransTime)
			}

			// Self-loop
			if t.From == to {
				ed.drawSelfLoop(fromX, fromY-1, label, canvasW, canvasH, arcStyle)
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
			ed.drawArcWithOffset(fromX, fromY, toX, toY, label, offset, canvasW, canvasH, arcStyle)
		}
	}
}

// drawNets renders structural net connections between component instances.
// Nets are drawn below the transition layer using orange dashed lines.
// Power nets use a dimmer style; signal nets use bright orange.
func (ed *Editor) drawNets(canvasW, canvasH int) {
	if !ed.fsm.HasNets() {
		return
	}

	// Build state position map
	statePos := make(map[string]StatePos)
	for _, sp := range ed.states {
		statePos[sp.Name] = sp
	}

	// Assign vertical offsets to each net to avoid overlap.
	// Start at +2 below the state row (below state label + Moore output/link).
	netOffset := 2

	for _, net := range ed.fsm.Nets {
		style := styleNet
		labelStyle := styleNetLabel
		if ed.fsm.IsPowerNet(net) {
			style = styleNetPower
			labelStyle = styleNetPower
		}

		// Collect screen positions for all endpoints that are on-screen
		type screenEP struct {
			x, y int
		}
		var visible []screenEP
		for _, ep := range net.Endpoints {
			sp, ok := statePos[ep.Instance]
			if !ok {
				continue
			}
			sx := sp.X - ed.canvasOffsetX + len(sp.Name)/2 + 2
			sy := sp.Y - ed.canvasOffsetY
			visible = append(visible, screenEP{x: sx, y: sy})
		}

		if len(visible) < 2 {
			continue
		}

		// Determine drawing approach based on endpoint count.
		if len(visible) == 2 {
			// Two-endpoint net: draw a direct routed line offset below.
			ed.drawNetDirect(visible[0].x, visible[0].y, visible[1].x, visible[1].y,
				net.Name, netOffset, canvasW, canvasH, style, labelStyle)
		} else {
			// Multi-endpoint net: draw a horizontal bus with vertical stubs.
			xs := make([]int, len(visible))
			ys := make([]int, len(visible))
			for i, v := range visible {
				xs[i] = v.x
				ys[i] = v.y
			}
			ed.drawNetBus(xs, ys, net.Name, netOffset, canvasW, canvasH, style, labelStyle)
		}

		netOffset++
	}
}

// drawNetDirect draws a two-endpoint net as a line routed below the components.
//
//   [U1]          [U2]
//     │             │
//     ╰─── NET_A ───╯
//
func (ed *Editor) drawNetDirect(x1, y1, x2, y2 int, label string, yOff int, canvasW, canvasH int, style, labelStyle tcell.Style) {
	// Bus row: below the lower of the two endpoints
	busY := y1 + yOff
	if y2+yOff > busY {
		busY = y2 + yOff
	}
	if busY >= canvasH {
		return // off-screen
	}

	// Vertical stubs from each endpoint down to the bus row
	ed.drawNetVStub(x1, y1+1, busY, canvasW, canvasH, style)
	ed.drawNetVStub(x2, y2+1, busY, canvasW, canvasH, style)

	// Horizontal segment between the two stubs
	minX, maxX := x1, x2
	if x1 > x2 {
		minX, maxX = x2, x1
	}

	// Corner characters
	if minX >= 0 && minX < canvasW && busY >= 0 && busY < canvasH {
		ch := '╰'
		if minX == x2 {
			ch = '╰'
		}
		ed.screen.SetContent(minX, busY, ch, nil, style)
	}
	if maxX >= 0 && maxX < canvasW && busY >= 0 && busY < canvasH {
		ed.screen.SetContent(maxX, busY, '╯', nil, style)
	}

	// Horizontal line
	for x := minX + 1; x < maxX; x++ {
		if x >= 0 && x < canvasW && busY >= 0 && busY < canvasH {
			ed.screen.SetContent(x, busY, '╌', nil, style)
		}
	}

	// Label at midpoint
	midX := (minX+maxX)/2 - len(label)/2
	for i, ch := range label {
		px := midX + i
		if px >= 0 && px < canvasW && busY >= 0 && busY < canvasH {
			ed.screen.SetContent(px, busY, ch, nil, labelStyle)
		}
	}
}

// drawNetBus draws a multi-endpoint net as a horizontal bus with vertical stubs.
//
//   [U1]     [U2]     [U3]
//     │        │        │
//     ╰────────┴────────╯  VCC
//
func (ed *Editor) drawNetBus(xs, ys []int, label string, yOff int, canvasW, canvasH int, style, labelStyle tcell.Style) {
	if len(xs) < 2 {
		return
	}

	// Find the bus row: max Y + offset among all endpoints
	busY := 0
	for _, y := range ys {
		if y+yOff > busY {
			busY = y + yOff
		}
	}
	if busY >= canvasH {
		return
	}

	// Find X extent
	minX, maxX := xs[0], xs[0]
	for _, x := range xs {
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
	}

	// Draw the horizontal bus line
	if busY >= 0 && busY < canvasH {
		for x := minX; x <= maxX; x++ {
			if x >= 0 && x < canvasW {
				ed.screen.SetContent(x, busY, '╌', nil, style)
			}
		}
	}

	// Draw vertical stubs and junction characters
	for i := range xs {
		ed.drawNetVStub(xs[i], ys[i]+1, busY, canvasW, canvasH, style)

		// Junction character on the bus line
		if xs[i] >= 0 && xs[i] < canvasW && busY >= 0 && busY < canvasH {
			ch := '┴'
			if xs[i] == minX {
				ch = '╰'
			}
			if xs[i] == maxX {
				ch = '╯'
			}
			if minX == maxX {
				ch = '│'
			}
			ed.screen.SetContent(xs[i], busY, ch, nil, style)
		}
	}

	// Label after the bus
	labelX := maxX + 2
	if busY >= 0 && busY < canvasH {
		for i, ch := range label {
			px := labelX + i
			if px >= 0 && px < canvasW {
				ed.screen.SetContent(px, busY, ch, nil, labelStyle)
			}
		}
	}
}

// drawNetVStub draws a vertical stub from startY to endY at column x.
func (ed *Editor) drawNetVStub(x, startY, endY int, canvasW, canvasH int, style tcell.Style) {
	if x < 0 || x >= canvasW {
		return
	}
	minY, maxY := startY, endY
	if startY > endY {
		minY, maxY = endY, startY
	}
	for y := minY; y < maxY; y++ {
		if y >= 0 && y < canvasH {
			ed.screen.SetContent(x, y, '│', nil, style)
		}
	}
}

func (ed *Editor) drawSelfLoop(x, y int, label string, canvasW, canvasH int, style tcell.Style) {
	// Draw a loop above the state with label to the right
	//  ╭──╮
	//  ╰─→╯ label
	if y < 2 || x < 1 || x >= canvasW-6 {
		return
	}
	
	loopY := y - 2
	
	// Top of loop
	if loopY >= 0 {
		ed.screen.SetContent(x, loopY, '╭', nil, style)
		ed.screen.SetContent(x+1, loopY, '─', nil, style)
		ed.screen.SetContent(x+2, loopY, '─', nil, style)
		ed.screen.SetContent(x+3, loopY, '╮', nil, style)
	}
	
	// Sides
	if loopY+1 >= 0 && loopY+1 < canvasH {
		ed.screen.SetContent(x, loopY+1, '│', nil, style)
		ed.screen.SetContent(x+3, loopY+1, '│', nil, style)
		
		// Draw label to the right of the loop
		labelX := x + 5
		for i, r := range label {
			if labelX+i < canvasW {
				ed.screen.SetContent(labelX+i, loopY+1, r, nil, style)
			}
		}
	}
	
	// Bottom connects back with arrow
	if loopY+2 >= 0 && loopY+2 < canvasH {
		ed.screen.SetContent(x, loopY+2, '╰', nil, style)
		ed.screen.SetContent(x+1, loopY+2, '─', nil, style)
		ed.screen.SetContent(x+2, loopY+2, '→', nil, style)
		ed.screen.SetContent(x+3, loopY+2, '╯', nil, style)
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

func (ed *Editor) drawSidebar(w, h int) {
	dividerX := w - ed.sidebarWidth
	
	// Draw the divider line
	dividerStyle := tcell.StyleDefault.Foreground(tcell.ColorGray)
	if ed.sidebarDragging {
		dividerStyle = tcell.StyleDefault.Foreground(tcell.ColorYellow)
	}
	for y := 0; y < h-2; y++ {
		ed.screen.SetContent(dividerX, y, '│', nil, dividerStyle)
	}
	
	// Draw collapse indicator at top of divider
	if ed.sidebarCollapsed {
		ed.screen.SetContent(dividerX, 0, '◀', nil, dividerStyle)
	} else {
		ed.screen.SetContent(dividerX, 0, '▶', nil, dividerStyle)
	}
	
	// If collapsed, don't draw sidebar content
	if ed.sidebarCollapsed || ed.sidebarWidth < 10 {
		return
	}
	
	contentX := dividerX + 2
	scrollbarX := w - 1 // Rightmost column for scrollbar
	
	// Calculate fixed header height (title + mode + optional machine list)
	fixedHeaderLines := 2 // title + mode indicator
	if ed.isBundle {
		fixedHeaderLines = 2 + len(ed.bundleMachines) + 1 // +1 blank separator
	}
	visibleHeight := h - fixedHeaderLines - 2 // subtract status bar lines
	
	// Calculate total content height
	totalHeight := 0
	// States section: header + states + blank
	totalHeight += 1 + len(ed.fsm.States) + 1
	// Inputs section: header + inputs + blank  
	totalHeight += 1 + len(ed.fsm.Alphabet) + 1
	// Outputs section (if any): header + outputs + blank
	if len(ed.fsm.OutputAlphabet) > 0 {
		totalHeight += 1 + len(ed.fsm.OutputAlphabet) + 1
	}
	// Transitions section: header + transition lines
	totalHeight += 1
	for _, t := range ed.fsm.Transitions {
		totalHeight += len(t.To)
	}
	// Nets section (if any): header + net lines + blank
	if ed.fsm.HasNets() {
		totalHeight += 1 + len(ed.fsm.Nets) + 1
	}
	
	// Clamp scroll offset
	maxScroll := totalHeight - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if ed.sidebarScrollY > maxScroll {
		ed.sidebarScrollY = maxScroll
	}
	if ed.sidebarScrollY < 0 {
		ed.sidebarScrollY = 0
	}
	
	// Draw title (fixed, not scrolled)
	typeName := fsmTypeDisplayName(ed.fsm.Type)
	title := fmt.Sprintf("FSM: %s", typeName)
	if ed.fsm.Name != "" {
		title = ed.fsm.Name + " (" + typeName + ")"
	}
	ed.drawString(contentX, 0, truncate(title, ed.sidebarWidth-4), styleSidebarH)
	
	// Mode indicator on line 1
	if ed.isBundle {
		modeStr := fmt.Sprintf("Bundle [%d machines]", len(ed.bundleMachines))
		styleBundleIndicator := tcell.StyleDefault.Foreground(tcell.ColorOrange).Bold(true)
		ed.drawString(contentX, 1, truncate(modeStr, ed.sidebarWidth-4), styleBundleIndicator)
		
		// Machine selector: draw on lines 2..2+N
		styleMachineItem := tcell.StyleDefault.Foreground(tcell.ColorSilver)
		styleMachineCurrent := tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
		for i, mName := range ed.bundleMachines {
			y := 2 + i
			style := styleMachineItem
			prefix := "  "
			if mName == ed.currentMachine {
				style = styleMachineCurrent
				prefix = "▸ "
			}
			ed.drawString(contentX, y, truncate(prefix+mName, ed.sidebarWidth-4), style)
		}
	} else {
		styleModeSingle := tcell.StyleDefault.Foreground(tcell.ColorGray)
		ed.drawString(contentX, 1, "Single FSM", styleModeSingle)
	}
	
	// Style for flashing items in sidebar - light cyan for visibility
	styleFlashHighlight := tcell.StyleDefault.Foreground(tcell.ColorAqua).Bold(true)
	
	// Build content lines with their styles
	type contentLine struct {
		text  string
		style tcell.Style
	}
	var lines []contentLine
	
	// States section
	vocab := ed.Vocab()
	lines = append(lines, contentLine{vocab.States + ":", styleSidebarH})
	for i, s := range ed.fsm.States {
		prefix := "  "
		suffix := ""
		if s == ed.fsm.Initial {
			prefix = "→ "
		}
		if ed.fsm.IsAccepting(s) {
			suffix = " *"
		}
		if ed.fsm.IsLinked(s) {
			suffix += " ↗"
		}
		style := styleSidebar
		if ed.fsm.IsLinked(s) {
			style = styleStateLinked
		}
		if i == ed.selectedState {
			style = styleMenuSel
		}
		lines = append(lines, contentLine{truncate(prefix+s+suffix, ed.sidebarWidth-4), style})
	}
	lines = append(lines, contentLine{"", styleSidebar}) // blank line
	
	// Inputs section
	lines = append(lines, contentLine{vocab.Alphabet + ":", styleSidebarH})
	for _, inp := range ed.fsm.Alphabet {
		style := styleSidebar
		if ed.flashInput == inp {
			style = styleFlashHighlight
		}
		lines = append(lines, contentLine{"  " + truncate(inp, ed.sidebarWidth-6), style})
	}
	lines = append(lines, contentLine{"", styleSidebar}) // blank line
	
	// Outputs section
	if len(ed.fsm.OutputAlphabet) > 0 {
		lines = append(lines, contentLine{"Outputs:", styleSidebarH})
		for _, out := range ed.fsm.OutputAlphabet {
			style := styleSidebar
			if ed.flashOutput == out {
				style = styleFlashHighlight
			}
			lines = append(lines, contentLine{"  " + truncate(out, ed.sidebarWidth-6), style})
		}
		lines = append(lines, contentLine{"", styleSidebar}) // blank line
	}
	
	// Transitions section
	lines = append(lines, contentLine{vocab.Transition + "s:", styleSidebarH})
	for tIdx, t := range ed.fsm.Transitions {
		inp := "ε"
		if t.Input != nil {
			inp = *t.Input
		}
		for _, to := range t.To {
			line := fmt.Sprintf("  %s --%s--> %s", t.From, inp, to)
			if ed.fsm.Type == fsm.TypeMealy && t.Output != nil {
				line += " [" + *t.Output + "]"
			}
			style := styleSidebar
			if ed.flashTransIdx == tIdx {
				style = styleFlashHighlight
			}
			lines = append(lines, contentLine{truncate(line, ed.sidebarWidth-4), style})
		}
	}

	// Nets section
	if ed.fsm.HasNets() {
		lines = append(lines, contentLine{"", styleSidebar}) // blank separator
		signalCount := len(ed.fsm.SignalNets())
		powerCount := len(ed.fsm.Nets) - signalCount
		netHeader := fmt.Sprintf("Nets: %d", len(ed.fsm.Nets))
		if powerCount > 0 {
			netHeader = fmt.Sprintf("Nets: %d (%d sig, %d pwr)", len(ed.fsm.Nets), signalCount, powerCount)
		}
		lines = append(lines, contentLine{netHeader, styleSidebarH})
		for _, n := range ed.fsm.Nets {
			var eps []string
			for _, ep := range n.Endpoints {
				eps = append(eps, ep.Instance+"."+ep.Port)
			}
			tag := ""
			if ed.fsm.IsPowerNet(n) {
				tag = " [pwr]"
			}
			netLine := fmt.Sprintf("  %s: %s%s", n.Name, strings.Join(eps, ", "), tag)
			lines = append(lines, contentLine{truncate(netLine, ed.sidebarWidth-4), styleNet})
		}
	}
	
	// Draw visible content (starting after fixed header)
	startY := fixedHeaderLines
	for i := 0; i < visibleHeight && i+ed.sidebarScrollY < len(lines); i++ {
		lineIdx := i + ed.sidebarScrollY
		ed.drawString(contentX, startY+i, lines[lineIdx].text, lines[lineIdx].style)
	}
	
	// Draw scrollbar if content exceeds visible area
	if totalHeight > visibleHeight {
		scrollTrackStart := startY
		scrollTrackHeight := visibleHeight
		
		// Calculate thumb size and position
		thumbHeight := (visibleHeight * visibleHeight) / totalHeight
		if thumbHeight < 1 {
			thumbHeight = 1
		}
		if thumbHeight > scrollTrackHeight {
			thumbHeight = scrollTrackHeight
		}
		
		thumbPos := scrollTrackStart
		if maxScroll > 0 {
			thumbPos = scrollTrackStart + (ed.sidebarScrollY * (scrollTrackHeight - thumbHeight)) / maxScroll
		}
		
		// Draw track
		trackStyle := tcell.StyleDefault.Foreground(tcell.ColorGray)
		thumbStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorGray)
		
		for y := scrollTrackStart; y < scrollTrackStart+scrollTrackHeight; y++ {
			if y >= thumbPos && y < thumbPos+thumbHeight {
				ed.screen.SetContent(scrollbarX, y, '█', nil, thumbStyle)
			} else {
				ed.screen.SetContent(scrollbarX, y, '░', nil, trackStyle)
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
		if len(ed.filename) > 30 {
			fileInfo = filepath.Base(ed.filename)
		} else {
			fileInfo = ed.filename
		}
		// Show machine name if editing a bundle
		if ed.isBundle && ed.currentMachine != "" {
			fileInfo += " [" + ed.currentMachine + "]"
		}
	}
	if ed.modified {
		fileInfo += " *"
	}
	// Show count of other modified machines in bundle
	if ed.isBundle {
		modCount := 0
		for name, mod := range ed.bundleModified {
			if mod && name != ed.currentMachine {
				modCount++
			}
		}
		if modCount > 0 {
			fileInfo += fmt.Sprintf(" (+%d)", modCount)
		}
	}
	ed.drawString(1, y, fileInfo, styleStatus)

	// Mode
	modeStr := ed.modeString()
	ed.drawString(w/2-len(modeStr)/2, y, modeStr, styleStatus)

	// Message
	if ed.message != "" {
		// Determine base style for message type
		baseStyle := styleMsgInfo
		shouldFlash := false
		switch ed.messageType {
		case MsgError:
			baseStyle = styleMsgError
			shouldFlash = true
		case MsgSuccess:
			baseStyle = styleMsgSuccess
			shouldFlash = true
		case MsgWarning:
			baseStyle = styleMsgError // Use error style for warnings too
			shouldFlash = true
		case MsgInfo:
			baseStyle = styleMsgInfo
			shouldFlash = false
		}
		
		// Start with base style (defensive: ensures normal display after flash)
		style := baseStyle
		
		// Flash effect for first 500ms: alternate colours every 125ms (4 flashes)
		// Pattern: normal(0-125) -> inverted(125-250) -> normal(250-375) -> inverted(375-500) -> normal(500+)
		if shouldFlash && ed.messageFlashStart > 0 {
			elapsed := time.Now().UnixMilli() - ed.messageFlashStart
			if elapsed >= 0 && elapsed < 500 {
				// Determine which phase of the flash we're in
				// Phases at 125ms intervals: 0=normal, 1=inverted, 2=normal, 3=inverted
				phaseNum := elapsed / 125
				if phaseNum == 1 || phaseNum == 3 {
					// Inverted colours for flash
					fg, bg, _ := baseStyle.Decompose()
					style = tcell.StyleDefault.Foreground(bg).Background(fg)
				}
				// phaseNum 0, 2, or >=4: style remains baseStyle (normal)
			}
			// elapsed < 0 or >= 500: style remains baseStyle (normal)
		}
		
		ed.drawString(w-len(ed.message)-2, y, ed.message, style)
	}

	// Help bar
	y = h - 2
	for x := 0; x < w; x++ {
		ed.screen.SetContent(x, y, ' ', nil, styleDefault)
	}
	help := ed.helpString()
	ed.drawString(1, y, help, styleHelp)
}


func (ed *Editor) drawMinimap(screenW, screenH int) {
	// Minimap dimensions: scale 512x512 down to fit nicely on screen
	// Use 1:8 ratio, so minimap is 64x64 max, but cap to reasonable size
	minimapW := 48
	minimapH := 24
	
	// Adjust for screen size
	if minimapW > screenW-10 {
		minimapW = screenW - 10
	}
	if minimapH > screenH-8 {
		minimapH = screenH - 8
	}
	if minimapW < 16 {
		minimapW = 16
	}
	if minimapH < 8 {
		minimapH = 8
	}
	
	// Position: centered on screen
	startX := (screenW - minimapW - 2) / 2
	startY := (screenH - minimapH - 2) / 2
	
	// Calculate scale factors
	scaleX := float64(CanvasMaxWidth) / float64(minimapW)
	scaleY := float64(CanvasMaxHeight) / float64(minimapH)
	
	// Draw box
	styleMinimap := tcell.StyleDefault.Background(tcell.NewRGBColor(32, 32, 48)).Foreground(tcell.ColorWhite)
	styleMinimapBorder := tcell.StyleDefault.Foreground(tcell.ColorTeal)
	styleMinimapState := tcell.StyleDefault.Foreground(tcell.ColorGreen).Background(tcell.NewRGBColor(32, 32, 48))
	styleMinimapViewport := tcell.StyleDefault.Foreground(tcell.ColorYellow)
	
	// Draw border
	for x := startX; x < startX+minimapW+2; x++ {
		ed.screen.SetContent(x, startY, '─', nil, styleMinimapBorder)
		ed.screen.SetContent(x, startY+minimapH+1, '─', nil, styleMinimapBorder)
	}
	for y := startY; y < startY+minimapH+2; y++ {
		ed.screen.SetContent(startX, y, '│', nil, styleMinimapBorder)
		ed.screen.SetContent(startX+minimapW+1, y, '│', nil, styleMinimapBorder)
	}
	ed.screen.SetContent(startX, startY, '┌', nil, styleMinimapBorder)
	ed.screen.SetContent(startX+minimapW+1, startY, '┐', nil, styleMinimapBorder)
	ed.screen.SetContent(startX, startY+minimapH+1, '└', nil, styleMinimapBorder)
	ed.screen.SetContent(startX+minimapW+1, startY+minimapH+1, '┘', nil, styleMinimapBorder)
	
	// Title
	title := " Canvas Navigator "
	titleX := startX + (minimapW+2-len(title))/2
	ed.drawString(titleX, startY, title, styleMinimapBorder.Bold(true))
	
	// Fill background
	for y := startY + 1; y < startY+minimapH+1; y++ {
		for x := startX + 1; x < startX+minimapW+1; x++ {
			ed.screen.SetContent(x, y, ' ', nil, styleMinimap)
		}
	}
	
	// Draw states as dots
	for _, sp := range ed.states {
		// Convert state position to minimap coordinates
		mx := int(float64(sp.X) / scaleX)
		my := int(float64(sp.Y) / scaleY)
		
		// Clamp to minimap bounds
		if mx >= 0 && mx < minimapW && my >= 0 && my < minimapH {
			screenPosX := startX + 1 + mx
			screenPosY := startY + 1 + my
			ed.screen.SetContent(screenPosX, screenPosY, '●', nil, styleMinimapState)
		}
	}
	
	// Draw viewport rectangle
	// Calculate visible area in minimap coordinates
	visibleW := screenW - ed.sidebarWidth - 1
	visibleH := screenH - 2
	
	vpLeft := int(float64(ed.canvasOffsetX) / scaleX)
	vpTop := int(float64(ed.canvasOffsetY) / scaleY)
	vpRight := int(float64(ed.canvasOffsetX+visibleW) / scaleX)
	vpBottom := int(float64(ed.canvasOffsetY+visibleH) / scaleY)
	
	// Clamp viewport rect to minimap bounds
	if vpLeft < 0 {
		vpLeft = 0
	}
	if vpTop < 0 {
		vpTop = 0
	}
	if vpRight >= minimapW {
		vpRight = minimapW - 1
	}
	if vpBottom >= minimapH {
		vpBottom = minimapH - 1
	}
	
	// Draw viewport rectangle edges
	for x := vpLeft; x <= vpRight; x++ {
		screenPosX := startX + 1 + x
		// Top edge
		if vpTop >= 0 && vpTop < minimapH {
			screenPosY := startY + 1 + vpTop
			ed.screen.SetContent(screenPosX, screenPosY, '─', nil, styleMinimapViewport)
		}
		// Bottom edge
		if vpBottom >= 0 && vpBottom < minimapH {
			screenPosY := startY + 1 + vpBottom
			ed.screen.SetContent(screenPosX, screenPosY, '─', nil, styleMinimapViewport)
		}
	}
	for y := vpTop; y <= vpBottom; y++ {
		screenPosY := startY + 1 + y
		// Left edge
		if vpLeft >= 0 && vpLeft < minimapW {
			screenPosX := startX + 1 + vpLeft
			ed.screen.SetContent(screenPosX, screenPosY, '│', nil, styleMinimapViewport)
		}
		// Right edge
		if vpRight >= 0 && vpRight < minimapW {
			screenPosX := startX + 1 + vpRight
			ed.screen.SetContent(screenPosX, screenPosY, '│', nil, styleMinimapViewport)
		}
	}
	
	// Corners of viewport rectangle
	if vpLeft >= 0 && vpLeft < minimapW && vpTop >= 0 && vpTop < minimapH {
		ed.screen.SetContent(startX+1+vpLeft, startY+1+vpTop, '┌', nil, styleMinimapViewport)
	}
	if vpRight >= 0 && vpRight < minimapW && vpTop >= 0 && vpTop < minimapH {
		ed.screen.SetContent(startX+1+vpRight, startY+1+vpTop, '┐', nil, styleMinimapViewport)
	}
	if vpLeft >= 0 && vpLeft < minimapW && vpBottom >= 0 && vpBottom < minimapH {
		ed.screen.SetContent(startX+1+vpLeft, startY+1+vpBottom, '└', nil, styleMinimapViewport)
	}
	if vpRight >= 0 && vpRight < minimapW && vpBottom >= 0 && vpBottom < minimapH {
		ed.screen.SetContent(startX+1+vpRight, startY+1+vpBottom, '┘', nil, styleMinimapViewport)
	}
	
	// Footer with instructions
	footer := "Arrow keys: Pan   Esc/Ctrl+D: Exit"
	footerX := startX + (minimapW+2-len(footer))/2
	ed.drawString(footerX, startY+minimapH+1, footer, styleMinimapBorder)
}

// drawMachineSelector draws a selector for choosing a machine from a bundle
func (ed *Editor) drawBreadcrumbBar(w int) {
	y := 0
	
	// Background
	styleBreadcrumb := tcell.StyleDefault.Background(tcell.NewRGBColor(40, 40, 60)).Foreground(tcell.ColorWhite)
	styleBackBtn := tcell.StyleDefault.Background(tcell.NewRGBColor(60, 60, 90)).Foreground(tcell.ColorWhite)
	styleSeparator := tcell.StyleDefault.Background(tcell.NewRGBColor(40, 40, 60)).Foreground(tcell.ColorGray)
	styleCurrentMachine := tcell.StyleDefault.Background(tcell.NewRGBColor(40, 40, 60)).Foreground(tcell.ColorYellow).Bold(true)
	
	for x := 0; x < w; x++ {
		ed.screen.SetContent(x, y, ' ', nil, styleBreadcrumb)
	}
	
	// Back button
	backBtn := " ◀ "
	for i, r := range backBtn {
		ed.screen.SetContent(i, y, r, nil, styleBackBtn)
	}
	
	// Breadcrumbs
	x := len(backBtn) + 1
	crumbs := ed.getBreadcrumbs()
	
	for i, crumb := range crumbs {
		// Separator
		if i > 0 {
			sep := " › "
			for _, r := range sep {
				if x < w-1 {
					ed.screen.SetContent(x, y, r, nil, styleSeparator)
					x++
				}
			}
		}
		
		// Crumb name
		style := styleBreadcrumb
		if i == len(crumbs)-1 {
			// Current machine - highlighted
			style = styleCurrentMachine
		}
		
		for _, r := range crumb {
			if x < w-1 {
				ed.screen.SetContent(x, y, r, nil, style)
				x++
			}
		}
	}
}

// drawZoomAnimation draws the zoom in/out animation overlay
func (ed *Editor) drawZoomAnimation(w, h int) {
	if !ed.animating {
		return
	}
	
	elapsed := time.Now().UnixMilli() - ed.animStartTime
	if elapsed >= ed.animDuration {
		// Already handled by draw() before canvas rendering
		return
	}
	
	// Calculate animation progress (0.0 to 1.0)
	progress := float64(elapsed) / float64(ed.animDuration)
	
	// Ease-out cubic for smoother animation
	progress = 1 - (1-progress)*(1-progress)*(1-progress)
	
	// Calculate box dimensions
	// Center point in screen coordinates
	centerX := ed.animCenterX - ed.canvasOffsetX
	centerY := ed.animCenterY - ed.canvasOffsetY + 1 // +1 for breadcrumb bar offset
	
	var boxLeft, boxRight, boxTop, boxBottom int
	
	if ed.animZoomIn {
		// Zooming in: box expands from center to fill screen
		halfW := int(float64(w/2) * progress)
		halfH := int(float64(h/2) * progress)
		boxLeft = centerX - halfW
		boxRight = centerX + halfW
		boxTop = centerY - halfH
		boxBottom = centerY + halfH
	} else {
		// Zooming out: box contracts from full screen to center
		invProgress := 1 - progress
		halfW := int(float64(w/2) * invProgress)
		halfH := int(float64(h/2) * invProgress)
		boxLeft = centerX - halfW
		boxRight = centerX + halfW
		boxTop = centerY - halfH
		boxBottom = centerY + halfH
	}
	
	// Clamp to screen bounds
	if boxLeft < 0 { boxLeft = 0 }
	if boxTop < 0 { boxTop = 0 }
	if boxRight >= w { boxRight = w - 1 }
	if boxBottom >= h { boxBottom = h - 1 }
	
	// Draw the animated box
	styleBox := tcell.StyleDefault.Background(tcell.NewRGBColor(80, 60, 120)).Foreground(tcell.ColorWhite)
	styleBorder := tcell.StyleDefault.Background(tcell.NewRGBColor(120, 80, 160)).Foreground(tcell.ColorWhite)
	
	// Fill box interior
	for y := boxTop; y <= boxBottom; y++ {
		for x := boxLeft; x <= boxRight; x++ {
			// Border or interior
			if y == boxTop || y == boxBottom || x == boxLeft || x == boxRight {
				ed.screen.SetContent(x, y, ' ', nil, styleBorder)
			} else {
				ed.screen.SetContent(x, y, ' ', nil, styleBox)
			}
		}
	}
	
	// Show target machine name in center
	if boxRight-boxLeft > 10 && boxBottom-boxTop > 2 {
		label := ed.animTargetMachine
		labelX := (boxLeft + boxRight - len(label)) / 2
		labelY := (boxTop + boxBottom) / 2
		if labelX >= boxLeft && labelX+len(label) <= boxRight {
			ed.drawString(labelX, labelY, label, styleBorder.Bold(true))
		}
	}
}
