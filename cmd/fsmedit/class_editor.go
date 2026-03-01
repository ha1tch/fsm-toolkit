package main

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// classAssignRow represents one row in the class assignment grid.
type classAssignRow struct {
	IsHeader bool   // true for machine-name separator rows
	Machine  string // machine name (always set)
	State    string // state name (empty for header rows)
	Class    string // current class assignment
}

// propEditorRow represents one row in the property value editor.
type propEditorRow struct {
	IsHeader bool            // true for class-name separator rows
	Label    string          // header text or property name
	PropDef  fsm.PropertyDef // property definition (zero for headers)
	Value    interface{}     // current value
}

// ====================================================================
// Class Editor (define/edit classes and their properties)
// ====================================================================

func (ed *Editor) openClassEditor() {
	ed.fsm.EnsureClassMaps()
	ed.classEditorSelected = 0
	ed.classEditorPropSel = 0
	ed.classEditorFocus = 0
	ed.classEditorScroll = 0
	ed.classEditorPropScroll = 0
	ed.mode = ModeClassEditor
}

func (ed *Editor) drawClassEditor(w, h int) {
	classNames := ed.fsm.ClassNames()

	// Fixed overlay size — content scrolls inside.
	boxW := 64
	boxH := h - 6
	if boxH < 16 {
		boxH = 16
	}
	if boxH > h-4 {
		boxH = h - 4
	}

	cx, cy, cw, ch := ed.drawOverlayBox("CLASS EDITOR", boxW, boxH, w, h)
	_ = cw

	// Split the overlay into two regions: class list (top half) and properties (bottom half).
	// Reserve 3 lines for headers/help per section plus 1 for footer.
	availH := ch - 3 // usable rows inside box (excluding top border, footer)
	classListH := availH / 2
	if classListH < 4 {
		classListH = 4
	}
	propListH := availH - classListH - 3 // gap + header
	if propListH < 3 {
		propListH = 3
	}

	// --- Class list section ---
	y := cy + 1
	classCount := len(classNames)
	countLabel := fmt.Sprintf("Classes (%d):", classCount)
	ed.drawString(cx, y, countLabel, styleOverlay)
	y++

	// Ensure scroll keeps selection visible.
	visibleClasses := classListH - 1 // rows for class names
	if visibleClasses < 1 {
		visibleClasses = 1
	}
	ed.classEditorScroll = ensureVisible(ed.classEditorSelected, ed.classEditorScroll, visibleClasses)

	// Draw visible class names.
	for i := 0; i < visibleClasses && ed.classEditorScroll+i < classCount; i++ {
		idx := ed.classEditorScroll + i
		name := classNames[idx]
		label := "  " + name
		if name == fsm.DefaultClassName {
			label += " (built-in)"
		}
		// Truncate to fit.
		maxLen := boxW - 4
		if len(label) > maxLen {
			label = label[:maxLen-2] + ".."
		}
		s := styleOverlay
		if idx == ed.classEditorSelected && ed.classEditorFocus == 0 {
			s = styleOverlayHl
		}
		ed.drawString(cx, y, label, s)
		y++
	}

	// Scroll indicators.
	if ed.classEditorScroll > 0 {
		ed.drawString(cx+boxW-6, cy+2, " ↑ ", styleOverlayDim)
	}
	if ed.classEditorScroll+visibleClasses < classCount {
		ed.drawString(cx+boxW-6, cy+1+classListH, " ↓ ", styleOverlayDim)
	}

	// Help for class list.
	y = cy + 1 + classListH
	if ed.classEditorFocus == 0 {
		ed.drawString(cx, y, "[A] Add  [D] Delete  [Tab] Properties", styleOverlayDim)
	}
	y++

	// --- Properties section ---
	y++ // gap
	if ed.classEditorSelected >= 0 && ed.classEditorSelected < classCount {
		selClass := classNames[ed.classEditorSelected]
		cls := ed.fsm.Classes[selClass]
		if cls != nil && y < cy+ch-2 {
			ed.drawString(cx, y, fmt.Sprintf("Properties of \"%s\":", selClass), styleOverlayHdr)
			y++
			propStartY := y

			if len(cls.Properties) == 0 {
				ed.drawString(cx+2, y, "(no properties)", styleOverlayDim)
			} else {
				visibleProps := cy + ch - 3 - y
				if visibleProps < 1 {
					visibleProps = 1
				}
				ed.classEditorPropScroll = ensureVisible(ed.classEditorPropSel, ed.classEditorPropScroll, visibleProps)

				for i := 0; i < visibleProps && ed.classEditorPropScroll+i < len(cls.Properties); i++ {
					idx := ed.classEditorPropScroll + i
					prop := cls.Properties[idx]
					label := fmt.Sprintf("  %-20s %s", prop.Name, prop.Type)
					maxLen := boxW - 6
					if len(label) > maxLen {
						label = label[:maxLen-2] + ".."
					}
					s := styleOverlay
					if idx == ed.classEditorPropSel && ed.classEditorFocus == 1 {
						s = styleOverlayHl
					}
					ed.drawString(cx+2, y, label, s)
					y++
				}

				// Scroll indicators for properties.
				if ed.classEditorPropScroll > 0 {
					ed.drawString(cx+boxW-6, propStartY, " ↑ ", styleOverlayDim)
				}
				if ed.classEditorPropScroll+visibleProps < len(cls.Properties) {
					ed.drawString(cx+boxW-6, cy+ch-3, " ↓ ", styleOverlayDim)
				}
			}

			if ed.classEditorFocus == 1 {
				helpY := cy + ch - 2
				ed.drawString(cx, helpY, "[P] Add prop  [D] Delete  [Tab] Classes", styleOverlayDim)
			}
		}
	}

	ed.drawString(cx, cy+ch-1, "[Esc] Back to Settings", styleOverlayDim)
}

// ensureVisible adjusts a scroll offset so that the selected index is visible
// within a window of visibleCount rows.
func ensureVisible(selected, scroll, visibleCount int) int {
	if selected < scroll {
		return selected
	}
	if selected >= scroll+visibleCount {
		return selected - visibleCount + 1
	}
	return scroll
}

func (ed *Editor) handleClassEditorKey(ev *tcell.EventKey) bool {
	classNames := ed.fsm.ClassNames()

	switch ev.Key() {
	case tcell.KeyEscape:
		ed.mode = ModeSettings
		return false
	case tcell.KeyTab:
		ed.classEditorFocus = 1 - ed.classEditorFocus
		ed.classEditorPropSel = 0
		ed.classEditorPropScroll = 0
		return false
	case tcell.KeyUp:
		if ed.classEditorFocus == 0 {
			if ed.classEditorSelected > 0 {
				ed.classEditorSelected--
				ed.classEditorPropSel = 0
				ed.classEditorPropScroll = 0
			}
		} else {
			if ed.classEditorPropSel > 0 {
				ed.classEditorPropSel--
			}
		}
	case tcell.KeyDown:
		if ed.classEditorFocus == 0 {
			if ed.classEditorSelected < len(classNames)-1 {
				ed.classEditorSelected++
				ed.classEditorPropSel = 0
				ed.classEditorPropScroll = 0
			}
		} else {
			selClass := classNames[ed.classEditorSelected]
			cls := ed.fsm.Classes[selClass]
			if cls != nil && ed.classEditorPropSel < len(cls.Properties)-1 {
				ed.classEditorPropSel++
			}
		}
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'a', 'A':
			if ed.classEditorFocus == 0 {
				ed.promptAddClass()
			}
		case 'p', 'P':
			if ed.classEditorFocus == 1 {
				ed.promptAddProperty()
			}
		case 'd', 'D':
			if ed.classEditorFocus == 0 {
				ed.deleteSelectedClass()
			} else {
				ed.deleteSelectedProperty()
			}
		}
	}
	return false
}

func (ed *Editor) promptAddClass() {
	ed.inputPrompt = "New class name: "
	ed.inputBuffer = ""
	ed.inputAction = func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			ed.showMessage("Class name cannot be empty", MsgError)
			ed.mode = ModeClassEditor
			return
		}
		cls := &fsm.Class{Name: name, Properties: []fsm.PropertyDef{}}
		if err := ed.fsm.AddClass(cls); err != nil {
			ed.showMessage(err.Error(), MsgError)
		} else {
			ed.showMessage("Class \""+name+"\" created", MsgSuccess)
			ed.modified = true
		}
		ed.mode = ModeClassEditor
	}
	ed.mode = ModeInput
}

func (ed *Editor) promptAddProperty() {
	classNames := ed.fsm.ClassNames()
	if ed.classEditorSelected >= len(classNames) {
		return
	}
	selClass := classNames[ed.classEditorSelected]

	ed.inputPrompt = "Property name: "
	ed.inputBuffer = ""
	ed.inputAction = func(propName string) {
		propName = strings.TrimSpace(propName)
		if propName == "" {
			ed.showMessage("Property name cannot be empty", MsgError)
			ed.mode = ModeClassEditor
			return
		}
		ed.promptPropertyType(selClass, propName)
	}
	ed.mode = ModeInput
}

func (ed *Editor) promptPropertyType(className, propName string) {
	types := fsm.ValidPropertyTypes()
	typeNames := make([]string, len(types))
	for i, t := range types {
		typeNames[i] = string(t)
	}
	ed.inputPrompt = fmt.Sprintf("Type for \"%s\" (%s): ", propName, strings.Join(typeNames, ", "))
	ed.inputBuffer = ""
	ed.inputAction = func(typeName string) {
		typeName = strings.TrimSpace(typeName)
		if !fsm.IsValidPropertyType(typeName) {
			ed.showMessage("Invalid type: "+typeName, MsgError)
			ed.mode = ModeClassEditor
			return
		}
		cls := ed.fsm.Classes[className]
		if cls == nil {
			ed.showMessage("Class not found", MsgError)
			ed.mode = ModeClassEditor
			return
		}
		if err := cls.AddProperty(propName, fsm.PropertyType(typeName)); err != nil {
			ed.showMessage(err.Error(), MsgError)
		} else {
			ed.showMessage(fmt.Sprintf("Added %s.%s (%s)", className, propName, typeName), MsgSuccess)
			ed.modified = true
		}
		ed.mode = ModeClassEditor
	}
	ed.mode = ModeInput
}

func (ed *Editor) deleteSelectedClass() {
	classNames := ed.fsm.ClassNames()
	if ed.classEditorSelected >= len(classNames) {
		return
	}
	name := classNames[ed.classEditorSelected]
	if name == fsm.DefaultClassName {
		ed.showMessage("Cannot delete built-in default_state", MsgError)
		return
	}
	if ed.fsm.RemoveClass(name) {
		ed.showMessage("Deleted class \""+name+"\"", MsgSuccess)
		ed.modified = true
		if ed.classEditorSelected >= len(ed.fsm.ClassNames()) {
			ed.classEditorSelected = len(ed.fsm.ClassNames()) - 1
		}
	}
}

func (ed *Editor) deleteSelectedProperty() {
	classNames := ed.fsm.ClassNames()
	if ed.classEditorSelected >= len(classNames) {
		return
	}
	selClass := classNames[ed.classEditorSelected]
	cls := ed.fsm.Classes[selClass]
	if cls == nil || ed.classEditorPropSel >= len(cls.Properties) {
		return
	}
	propName := cls.Properties[ed.classEditorPropSel].Name
	cls.RemoveProperty(propName)
	ed.showMessage(fmt.Sprintf("Removed %s.%s", selClass, propName), MsgSuccess)
	ed.modified = true
	if ed.classEditorPropSel >= len(cls.Properties) && ed.classEditorPropSel > 0 {
		ed.classEditorPropSel--
	}
}

// handleClassEditorMouse handles mouse events in the class editor overlay.
func (ed *Editor) handleClassEditorMouse(ev *tcell.EventMouse, w, h int) {
	mx, my := ev.Position()
	buttons := ev.Buttons()

	classNames := ed.fsm.ClassNames()
	classCount := len(classNames)

	// Recompute the same layout as drawClassEditor.
	boxW := 64
	boxH := h - 6
	if boxH < 16 {
		boxH = 16
	}
	if boxH > h-4 {
		boxH = h - 4
	}
	boxX := (w - boxW) / 2
	boxY := (h - boxH) / 2
	if boxY < 1 {
		boxY = 1
	}

	cx := boxX + 2
	cy := boxY + 1
	ch := boxH - 2

	availH := ch - 3
	classListH := availH / 2
	if classListH < 4 {
		classListH = 4
	}

	visibleClasses := classListH - 1
	if visibleClasses < 1 {
		visibleClasses = 1
	}

	// Class list region: rows [cy+1, cy+1+visibleClasses).
	classStartY := cy + 1
	classEndY := classStartY + visibleClasses

	// Property region starts after class list + help + gap.
	propHeaderY := cy + 1 + classListH + 2
	propStartY := propHeaderY + 1
	propEndY := cy + ch - 3

	visibleProps := propEndY - propStartY
	if visibleProps < 1 {
		visibleProps = 1
	}

	// Scroll wheel.
	if buttons&tcell.WheelUp != 0 {
		if my < propHeaderY {
			// Scroll class list.
			if ed.classEditorSelected > 0 {
				ed.classEditorSelected--
				ed.classEditorPropSel = 0
				ed.classEditorPropScroll = 0
			}
		} else {
			// Scroll property list.
			if ed.classEditorPropSel > 0 {
				ed.classEditorPropSel--
			}
		}
		return
	}
	if buttons&tcell.WheelDown != 0 {
		if my < propHeaderY {
			if ed.classEditorSelected < classCount-1 {
				ed.classEditorSelected++
				ed.classEditorPropSel = 0
				ed.classEditorPropScroll = 0
			}
		} else {
			if ed.classEditorSelected >= 0 && ed.classEditorSelected < classCount {
				cls := ed.fsm.Classes[classNames[ed.classEditorSelected]]
				if cls != nil && ed.classEditorPropSel < len(cls.Properties)-1 {
					ed.classEditorPropSel++
				}
			}
		}
		return
	}

	// Left click.
	if buttons&tcell.Button1 == 0 {
		return
	}

	// Click inside class list?
	if mx >= cx && mx < cx+boxW-4 && my >= classStartY && my < classEndY {
		idx := ed.classEditorScroll + (my - classStartY)
		if idx >= 0 && idx < classCount {
			ed.classEditorSelected = idx
			ed.classEditorFocus = 0
			ed.classEditorPropSel = 0
			ed.classEditorPropScroll = 0
		}
		return
	}

	// Click inside property list?
	if mx >= cx && mx < cx+boxW-4 && my >= propStartY && my < propEndY {
		if ed.classEditorSelected >= 0 && ed.classEditorSelected < classCount {
			cls := ed.fsm.Classes[classNames[ed.classEditorSelected]]
			if cls != nil && len(cls.Properties) > 0 {
				idx := ed.classEditorPropScroll + (my - propStartY)
				if idx >= 0 && idx < len(cls.Properties) {
					ed.classEditorPropSel = idx
					ed.classEditorFocus = 1
				}
			}
		}
		return
	}
}
