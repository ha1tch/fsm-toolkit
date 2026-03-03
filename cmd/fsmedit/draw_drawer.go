// Component drawer rendering for fsmedit.
// Bottom panel showing class library components for drag-and-drop instantiation.
package main

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

const (
	drawerTargetHeight = 8  // rows when fully open
	drawerAnimDuration = 180 // ms
	drawerCardWidth    = 24
	drawerCardHeight   = 3
	drawerButtonLabel  = " [+] "
)

// drawerEffectiveHeight returns the current drawer height accounting for animation.
func (ed *Editor) drawerEffectiveHeight() int {
	if !ed.drawerOpen && !ed.drawerAnimating {
		return 0
	}
	if ed.drawerAnimating {
		elapsed := time.Now().UnixMilli() - ed.drawerAnimStart
		t := float64(elapsed) / float64(drawerAnimDuration)
		if t > 1.0 {
			t = 1.0
		}

		// Ease-out cubic for opening (fast start, gentle landing),
		// ease-in cubic for closing (gentle start, fast drop).
		var eased float64
		if ed.drawerAnimDir > 0 {
			// Opening: ease-out — 1 - (1-t)^3
			inv := 1.0 - t
			eased = 1.0 - inv*inv*inv
		} else {
			// Closing: ease-in — (1-t) with cubic acceleration = 1 - t^3
			eased = 1.0 - t*t*t
		}

		h := int(float64(drawerTargetHeight)*eased + 0.5)
		if h < 0 {
			h = 0
		}
		if h > drawerTargetHeight {
			h = drawerTargetHeight
		}
		return h
	}
	if ed.drawerOpen {
		return drawerTargetHeight
	}
	return 0
}

// drawDrawerButton draws the small [+] button when the drawer is closed.
func (ed *Editor) drawDrawerButton(w, h int) {
	if ed.drawerOpen || ed.drawerAnimating {
		return
	}
	if len(ed.catalog) == 0 {
		return
	}

	// Place button at bottom-right of status bar area.
	btnX := w - len(drawerButtonLabel)
	btnY := h - 2
	style := tcell.StyleDefault.Background(tcell.PaletteColor(235)).Foreground(tcell.ColorOrange)
	ed.drawString(btnX, btnY, drawerButtonLabel, style)
}

// drawDrawer draws the component drawer panel at the bottom of the screen.
func (ed *Editor) drawDrawer(w, h int) {
	dh := ed.drawerEffectiveHeight()
	if dh <= 0 {
		return
	}

	// Drawer occupies the bottom dh rows.
	drawerY := h - dh

	// Background fill.
	bgStyle := tcell.StyleDefault.Background(tcell.PaletteColor(236)).Foreground(tcell.ColorWhite)
	for y := drawerY; y < h; y++ {
		for x := 0; x < w; x++ {
			ed.screen.SetContent(x, y, ' ', nil, bgStyle)
		}
	}

	// Top border.
	borderStyle := tcell.StyleDefault.Background(tcell.PaletteColor(236)).Foreground(tcell.PaletteColor(242))
	for x := 0; x < w; x++ {
		ed.screen.SetContent(x, drawerY, '─', nil, borderStyle)
	}

	if dh < 3 {
		return // Too small to show content during animation.
	}

	// Category tabs on the border line.
	tabX := 1
	tabStyle := tcell.StyleDefault.Background(tcell.PaletteColor(236)).Foreground(tcell.ColorGray)
	tabSelStyle := tcell.StyleDefault.Background(tcell.PaletteColor(238)).Foreground(tcell.ColorOrange)

	for i, cat := range ed.catalog {
		label := " " + cat.Name + " "
		s := tabStyle
		if i == ed.drawerCatIdx {
			s = tabSelStyle
		}
		if tabX+len(label) >= w-1 {
			break
		}
		ed.drawString(tabX, drawerY, label, s)
		tabX += len(label) + 1
	}

	// Close hint on the right of the tab bar.
	closeHint := " [Esc] Close "
	ed.drawString(w-len(closeHint)-1, drawerY, closeHint, tabStyle)

	if dh < 5 {
		return // Not enough room for cards during animation.
	}

	// Component cards.
	if ed.drawerCatIdx >= 0 && ed.drawerCatIdx < len(ed.catalog) {
		cat := ed.catalog[ed.drawerCatIdx]
		cardY := drawerY + 1
		cardX := 1 - ed.drawerScroll*drawerCardWidth

		for i, cls := range cat.Classes {
			x0 := cardX + i*drawerCardWidth
			if x0+drawerCardWidth < 0 {
				continue // Scrolled off left.
			}
			if x0 >= w {
				break // Off right edge.
			}
			ed.drawComponentCard(x0, cardY, cls, i == ed.drawerItemIdx, w, dh-2)
		}
	}

	// Help line at bottom (if room).
	if dh >= drawerTargetHeight {
		helpY := h - 1
		helpStyle := tcell.StyleDefault.Background(tcell.PaletteColor(236)).Foreground(tcell.ColorGray)
		help := "[Tab] Category  [</>] Browse  [Enter] Place  [Drag] Drop on canvas"
		ed.drawString(1, helpY, help, helpStyle)
	}
}

// drawComponentCard renders a single component card in the drawer.
func (ed *Editor) drawComponentCard(x, y int, cls *fsm.Class, selected bool, screenW, maxH int) {
	cardBg := tcell.PaletteColor(238)
	cardFg := tcell.ColorWhite
	if selected {
		cardBg = tcell.PaletteColor(240)
		cardFg = tcell.ColorOrange
	}
	cardStyle := tcell.StyleDefault.Background(cardBg).Foreground(cardFg)
	dimStyle := tcell.StyleDefault.Background(cardBg).Foreground(tcell.ColorGray)

	cw := drawerCardWidth - 2 // inner width

	// Extract short name (part number) and description.
	shortName, desc := splitClassName(cls.Name)

	// Row 1: part number (bold-ish via colour).
	row1 := shortName
	if len(row1) > cw {
		row1 = row1[:cw]
	}

	// Row 2: description.
	row2 := desc
	if len(row2) > cw {
		row2 = row2[:cw-3] + "..."
	}

	// Row 3: property and port counts.
	row3 := fmt.Sprintf("%d props", len(cls.Properties))
	if cls.HasPorts() {
		row3 = fmt.Sprintf("%d props, %d pins", len(cls.Properties), len(cls.Ports))
	}

	// Draw background fill.
	for dy := 0; dy < drawerCardHeight && dy < maxH; dy++ {
		for dx := 0; dx < drawerCardWidth-1; dx++ {
			cx := x + dx
			if cx >= 0 && cx < screenW {
				ed.screen.SetContent(cx, y+dy, ' ', nil, cardStyle)
			}
		}
	}

	// Draw text.
	if maxH >= 1 {
		ed.drawStringClipped(x+1, y, row1, cardStyle, 0, screenW)
	}
	if maxH >= 2 {
		ed.drawStringClipped(x+1, y+1, row2, dimStyle, 0, screenW)
	}
	if maxH >= 3 {
		ed.drawStringClipped(x+1, y+2, row3, dimStyle, 0, screenW)
	}

	// Selection indicator.
	if selected && x >= 0 && x < screenW {
		selStyle := tcell.StyleDefault.Background(cardBg).Foreground(tcell.ColorOrange)
		ed.screen.SetContent(x, y, '▸', nil, selStyle)
	}
}

// drawStringClipped draws a string, clipping to screen bounds.
func (ed *Editor) drawStringClipped(x, y int, s string, style tcell.Style, minX, maxX int) {
	for i, r := range s {
		cx := x + i
		if cx < minX || cx >= maxX {
			continue
		}
		ed.screen.SetContent(cx, y, r, nil, style)
	}
}

// drawDragGhost draws the component name following the cursor during drag.
func (ed *Editor) drawDragGhost(w, h int) {
	if !ed.drawerDragging || ed.drawerDragClass == nil {
		return
	}

	shortName, _ := splitClassName(ed.drawerDragClass.Name)
	label := " " + shortName + " "

	ghostStyle := tcell.StyleDefault.Background(tcell.ColorOrange).Foreground(tcell.ColorBlack)

	gx := ed.drawerDragX
	gy := ed.drawerDragY - 1 // above cursor
	if gy < 0 {
		gy = 0
	}

	for i, r := range label {
		cx := gx + i
		if cx >= 0 && cx < w && gy >= 0 && gy < h {
			ed.screen.SetContent(cx, gy, r, nil, ghostStyle)
		}
	}
}

// splitClassName splits "7407_hex_buffer_oc" into ("7407", "Hex Buffer OC").
func splitClassName(name string) (string, string) {
	// Find the first underscore after leading digits.
	i := 0
	for i < len(name) && name[i] >= '0' && name[i] <= '9' {
		i++
	}
	if i == 0 || i >= len(name) || name[i] != '_' {
		// No numeric prefix — use full name.
		return name, ""
	}

	partNum := name[:i]
	rest := name[i+1:]

	// Convert rest to display form.
	desc := ""
	words := splitOnUnderscore(rest)
	for j, w := range words {
		if j > 0 {
			desc += " "
		}
		// Title case, but keep common abbreviations upper.
		if isAbbreviation(w) {
			desc += upper(w)
		} else if len(w) > 0 {
			desc += upper(w[:1]) + w[1:]
		}
	}

	return partNum, desc
}

func splitOnUnderscore(s string) []string {
	var parts []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '_' {
			if i > start {
				parts = append(parts, s[start:i])
			}
			start = i + 1
		}
	}
	return parts
}

func isAbbreviation(s string) bool {
	switch s {
	case "oc", "hv", "alu", "bcd", "ram", "rbi", "mux":
		return true
	}
	return false
}

func upper(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= 'a' && s[i] <= 'z' {
			result[i] = s[i] - 32
		} else {
			result[i] = s[i]
		}
	}
	return string(result)
}
