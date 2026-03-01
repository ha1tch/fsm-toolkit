// UI primitive helpers for fsmedit: styles, drawing primitives, display strings.
package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)


// FSM type display names (always uppercase for consistency)
const (
	DisplayTypeDFA   = "DFA"
	DisplayTypeNFA   = "NFA"
	DisplayTypeMoore = "Moore"
	DisplayTypeMealy = "Mealy"
)

// fsmTypeDisplayName returns the uppercase display name for an FSM type
func fsmTypeDisplayName(t fsm.Type) string {
	switch t {
	case fsm.TypeDFA:
		return DisplayTypeDFA
	case fsm.TypeNFA:
		return DisplayTypeNFA
	case fsm.TypeMoore:
		return DisplayTypeMoore
	case fsm.TypeMealy:
		return DisplayTypeMealy
	default:
		return "UNKNOWN"
	}
}

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
	styleStateLinked = tcell.StyleDefault.Foreground(tcell.ColorFuchsia).Bold(true)
	styleTrans      = tcell.StyleDefault.Foreground(tcell.ColorTeal)
	styleTransDrag  = tcell.StyleDefault.Foreground(tcell.NewRGBColor(200, 162, 200)) // Lilac
	styleSidebar    = tcell.StyleDefault.Foreground(tcell.ColorWhite)
	styleSidebarH   = tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
	styleStatus     = tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorNavy)
	styleMsgInfo    = tcell.StyleDefault.Foreground(tcell.ColorSilver).Background(tcell.ColorNavy)
	styleMsgError   = tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorNavy).Bold(true)
	styleMsgSuccess = tcell.StyleDefault.Foreground(tcell.ColorSilver).Background(tcell.ColorNavy)
	styleHelp       = tcell.StyleDefault.Foreground(tcell.ColorGray) // Help bar on default background
	styleCursor     = tcell.StyleDefault.Background(tcell.ColorDarkGray)
	styleInput      = tcell.StyleDefault.Background(tcell.ColorNavy).Foreground(tcell.ColorWhite)
	styleBorder     = tcell.StyleDefault.Foreground(tcell.ColorGray)
	styleDragging   = tcell.StyleDefault.Background(tcell.ColorPurple).Foreground(tcell.ColorWhite)

	// Overlay panel styles (very dark grey background: #262626)
	styleOverlay    = tcell.StyleDefault.Background(tcell.PaletteColor(235)).Foreground(tcell.ColorWhite)
	styleOverlayHl  = tcell.StyleDefault.Background(tcell.ColorBlue).Foreground(tcell.ColorWhite)
	styleOverlayDim = tcell.StyleDefault.Background(tcell.PaletteColor(235)).Foreground(tcell.ColorGray)
	styleOverlayHdr = tcell.StyleDefault.Background(tcell.PaletteColor(235)).Foreground(tcell.ColorYellow)
	styleOverlayEdt = tcell.StyleDefault.Background(tcell.ColorDarkGreen).Foreground(tcell.ColorWhite)
	styleOverlayBrd = tcell.StyleDefault.Background(tcell.PaletteColor(235)).Foreground(tcell.PaletteColor(240))
)

// drawTitledBox draws a bordered box with optional title
func (ed *Editor) drawTitledBox(x, y, w, h int, title string) {
	// Top border
	ed.screen.SetContent(x, y, '┌', nil, styleBorder)
	for i := 1; i < w-1; i++ {
		ed.screen.SetContent(x+i, y, '─', nil, styleBorder)
	}
	ed.screen.SetContent(x+w-1, y, '┐', nil, styleBorder)

	// Title if provided
	if title != "" {
		titleX := x + (w-len(title)-2)/2
		ed.screen.SetContent(titleX, y, ' ', nil, styleBorder)
		ed.drawString(titleX+1, y, title, styleSidebarH)
		ed.screen.SetContent(titleX+1+len(title), y, ' ', nil, styleBorder)
	}

	// Sides and fill
	for row := 1; row < h-1; row++ {
		ed.screen.SetContent(x, y+row, '│', nil, styleBorder)
		for col := 1; col < w-1; col++ {
			ed.screen.SetContent(x+col, y+row, ' ', nil, styleDefault)
		}
		ed.screen.SetContent(x+w-1, y+row, '│', nil, styleBorder)
	}

	// Bottom border
	ed.screen.SetContent(x, y+h-1, '└', nil, styleBorder)
	for i := 1; i < w-1; i++ {
		ed.screen.SetContent(x+i, y+h-1, '─', nil, styleBorder)
	}
	ed.screen.SetContent(x+w-1, y+h-1, '┘', nil, styleBorder)
}

// normalizePairKey returns a consistent key for a pair of states
func normalizePairKey(a, b string) string {
	if a < b {
		return a + "->" + b
	}
	return b + "->" + a
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

// drawOverlayBox draws a bordered overlay panel with the dark grey theme.
// Returns the interior region (x, y, w, h) for content placement.
func (ed *Editor) drawOverlayBox(title string, boxW, boxH, screenW, screenH int) (int, int, int, int) {
	if boxW > screenW-4 {
		boxW = screenW - 4
	}
	if boxH > screenH-4 {
		boxH = screenH - 4
	}

	boxX := (screenW - boxW) / 2
	boxY := (screenH - boxH) / 2
	if boxY < 1 {
		boxY = 1
	}

	// Corners
	ed.screen.SetContent(boxX, boxY, '┌', nil, styleOverlayBrd)
	ed.screen.SetContent(boxX+boxW-1, boxY, '┐', nil, styleOverlayBrd)
	ed.screen.SetContent(boxX, boxY+boxH-1, '└', nil, styleOverlayBrd)
	ed.screen.SetContent(boxX+boxW-1, boxY+boxH-1, '┘', nil, styleOverlayBrd)

	// Horizontal borders
	for i := boxX + 1; i < boxX+boxW-1; i++ {
		ed.screen.SetContent(i, boxY, '─', nil, styleOverlayBrd)
		ed.screen.SetContent(i, boxY+boxH-1, '─', nil, styleOverlayBrd)
	}

	// Vertical borders
	for i := boxY + 1; i < boxY+boxH-1; i++ {
		ed.screen.SetContent(boxX, i, '│', nil, styleOverlayBrd)
		ed.screen.SetContent(boxX+boxW-1, i, '│', nil, styleOverlayBrd)
	}

	// Fill interior
	for row := boxY + 1; row < boxY+boxH-1; row++ {
		for col := boxX + 1; col < boxX+boxW-1; col++ {
			ed.screen.SetContent(col, row, ' ', nil, styleOverlay)
		}
	}

	// Title
	if title != "" {
		if len(title) > boxW-6 {
			title = title[:boxW-9] + "..."
		}
		tx := boxX + 2
		ed.screen.SetContent(tx-1, boxY, '┤', nil, styleOverlayBrd)
		ed.drawString(tx, boxY, " "+title+" ", styleOverlayHdr)
		ed.screen.SetContent(tx+len(title)+2, boxY, '├', nil, styleOverlayBrd)
	}

	// Return interior region
	return boxX + 2, boxY + 1, boxW - 4, boxH - 2
}

func (ed *Editor) drawString(x, y int, s string, style tcell.Style) {
	for i, r := range s {
		ed.screen.SetContent(x+i, y, r, nil, style)
	}
}

func (ed *Editor) modeString() string {
	if ed.dragging {
		return "MOVE"
	}
	switch ed.mode {
	case ModeMenu:
		return "MENU"
	case ModeCanvas:
		return "" // No label for normal canvas mode
	case ModeMove:
		return "MOVE"
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
	case ModeHelp:
		return "HELP"
	default:
		return ""
	}
}

func (ed *Editor) helpString() string {
	switch ed.mode {
	case ModeMenu:
		return "↑↓:Select  Enter:Confirm  Esc:Canvas"
	case ModeCanvas:
		if len(ed.navStack) > 0 {
			return "H:Help  Shift+←:Back  Space:Dive  Tab:Cycle  T:Trans  K:Link  G:Move  Del:Del  Esc:Menu"
		}
		if ed.isBundle {
			return "H:Help  Space:Dive  Tab:Cycle  T:Trans  S:Initial  A:Accept  K:Link  G:Move  Del:Del  Esc:Menu"
		}
		return "H:Help  Enter:Add  Tab:Cycle  T:Trans  S:Initial  A:Accept  K:Link  G:Move  Del:Del  Esc:Menu"
	case ModeInput:
		return "Type text  Enter:Confirm  Esc:Cancel"
	case ModeFilePicker:
		if ed.importMode {
			return "↑↓:Select  Tab:Switch  Enter:Import  Esc:Cancel"
		}
		return "↑↓:Select  Tab:Switch  Enter:Open  Esc:Cancel"
	case ModeSelectType:
		return "↑↓:Select  Enter:Confirm  Esc:Cancel"
	case ModeAddTransition, ModeSelectInput, ModeSelectOutput:
		return "↑↓:Select  Enter:Confirm  Esc:Cancel"
	case ModeMove:
		return "Arrows:Move  Enter:Confirm  Esc:Cancel"
	case ModeHelp:
		return "↑↓/PgUp/PgDn: Scroll   Esc/Q: Close"
	case ModeSelectLinkTarget:
		return "↑↓:Select  Enter:Link  Esc:Cancel"
	case ModeImportMachineSelect:
		return "↑↓:Navigate  Space:Toggle  A:All  Enter:Import  Esc:Cancel"
	default:
		return "Ctrl+Z:Undo  Ctrl+Y:Redo"
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
