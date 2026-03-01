package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// ====================================================================
// Class Assignment Grid (assign classes to states, grouped by machine)
// ====================================================================

// openClassAssign builds the flattened row list and enters ModeClassAssign.
func (ed *Editor) openClassAssign() {
	ed.classAssignRows = ed.buildClassAssignRows()
	ed.classAssignCursor = 0
	ed.classAssignClassPick = false
	// Skip to first non-header row.
	for ed.classAssignCursor < len(ed.classAssignRows) && ed.classAssignRows[ed.classAssignCursor].IsHeader {
		ed.classAssignCursor++
	}
	ed.mode = ModeClassAssign
}

// buildClassAssignRows constructs the flat row list for the assignment grid.
// States are grouped by machine (for bundles) or shown flat (single file).
func (ed *Editor) buildClassAssignRows() []classAssignRow {
	var rows []classAssignRow

	addMachineRows := func(machineName string, f *fsm.FSM) {
		if f == nil || len(f.States) == 0 {
			return
		}
		f.EnsureClassMaps()
		rows = append(rows, classAssignRow{
			IsHeader: true,
			Machine:  machineName,
		})
		for _, state := range f.States {
			cls := f.GetStateClass(state)
			rows = append(rows, classAssignRow{
				Machine: machineName,
				State:   state,
				Class:   cls,
			})
		}
	}

	if ed.isBundle && len(ed.bundleMachines) > 0 {
		for _, m := range ed.bundleMachines {
			f := ed.bundleFSMs[m]
			addMachineRows(m, f)
		}
	} else {
		name := ed.fsm.Name
		if name == "" {
			name = "(current)"
		}
		addMachineRows(name, ed.fsm)
	}
	return rows
}

// fsmForMachine returns the FSM for a given machine name.
func (ed *Editor) fsmForMachine(machineName string) *fsm.FSM {
	if ed.isBundle {
		if f, ok := ed.bundleFSMs[machineName]; ok {
			return f
		}
	}
	return ed.fsm
}

func (ed *Editor) drawClassAssign(w, h int) {
	if ed.classAssignClassPick {
		// Draw the class picker overlay (on top of assignment).
		ed.drawClassAssignBackground(w, h)
		ed.drawClassPicker(w, h)
		return
	}
	ed.drawClassAssignBackground(w, h)
}

func (ed *Editor) drawClassAssignBackground(w, h int) {
	rows := ed.classAssignRows

	boxW := 70
	boxH := len(rows) + 8
	if boxH > h-4 {
		boxH = h - 4
	}
	if boxH < 10 {
		boxH = 10
	}

	cx, cy, cw, ch := ed.drawOverlayBox("ASSIGN CLASSES", boxW, boxH, w, h)

	y := cy + 1

	// Column layout relative to content area.
	col1 := cx
	col2 := cx + 32

	ed.drawString(col1, y, "State", styleOverlayDim)
	ed.drawString(col2, y, "Class", styleOverlayDim)
	y++
	ed.drawString(col1, y, strings.Repeat("-", 28), styleOverlayDim)
	ed.drawString(col2, y, strings.Repeat("-", cw-34), styleOverlayDim)
	y++

	contentStartY := y
	visibleRows := ch - (y - cy) - 3
	if visibleRows < 1 {
		visibleRows = 1
	}

	// Scroll to keep cursor visible (reuse classAssignCursor as scroll base).
	scrollOffset := 0
	if ed.classAssignCursor >= scrollOffset+visibleRows {
		scrollOffset = ed.classAssignCursor - visibleRows + 1
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	_ = contentStartY
	drawn := 0
	for i := scrollOffset; i < len(rows) && drawn < visibleRows; i++ {
		row := rows[i]

		if row.IsHeader {
			label := fmt.Sprintf("-- %s --", row.Machine)
			ed.drawString(col1-1, y, label, styleOverlayHdr)
		} else {
			s := styleOverlay
			if i == ed.classAssignCursor {
				s = styleOverlayHl
			}

			name := row.State
			if len(name) > 28 {
				name = name[:25] + "..."
			}
			ed.drawString(col1, y, name, s)

			classLabel := row.Class
			if classLabel == fsm.DefaultClassName {
				classLabel += " *"
			}
			ed.drawString(col2, y, classLabel, s)
		}
		y++
		drawn++
	}

	// Help text.
	ed.drawString(cx, cy+ch-2, "[Enter] Change class  [E] Edit props  [Esc] Back", styleOverlayDim)
}

func (ed *Editor) handleClassAssignKey(ev *tcell.EventKey) bool {
	if ed.classAssignClassPick {
		return ed.handleClassPickerKey(ev)
	}

	rows := ed.classAssignRows

	switch ev.Key() {
	case tcell.KeyEscape:
		ed.mode = ModeCanvas
		return false
	case tcell.KeyUp:
		ed.classAssignCursor--
		// Skip headers.
		for ed.classAssignCursor >= 0 && rows[ed.classAssignCursor].IsHeader {
			ed.classAssignCursor--
		}
		if ed.classAssignCursor < 0 {
			ed.classAssignCursor = 0
			for ed.classAssignCursor < len(rows) && rows[ed.classAssignCursor].IsHeader {
				ed.classAssignCursor++
			}
		}
	case tcell.KeyDown:
		ed.classAssignCursor++
		for ed.classAssignCursor < len(rows) && rows[ed.classAssignCursor].IsHeader {
			ed.classAssignCursor++
		}
		if ed.classAssignCursor >= len(rows) {
			ed.classAssignCursor = len(rows) - 1
			for ed.classAssignCursor >= 0 && rows[ed.classAssignCursor].IsHeader {
				ed.classAssignCursor--
			}
		}
	case tcell.KeyEnter:
		// Open class picker for current row.
		if ed.classAssignCursor >= 0 && ed.classAssignCursor < len(rows) && !rows[ed.classAssignCursor].IsHeader {
			row := rows[ed.classAssignCursor]
			f := ed.fsmForMachine(row.Machine)
			f.EnsureClassMaps()
			ed.classAssignClassList = f.ClassNames()
			ed.classAssignClassPick = true
			// Pre-select current class.
			for i, name := range ed.classAssignClassList {
				if name == row.Class {
					ed.classAssignCursor2 = i
					break
				}
			}
		}
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'e', 'E':
			// Open property editor for current state.
			if ed.classAssignCursor >= 0 && ed.classAssignCursor < len(rows) && !rows[ed.classAssignCursor].IsHeader {
				row := rows[ed.classAssignCursor]
				ed.openPropertyEditor(row.Machine, row.State)
			}
		}
	}
	return false
}

// --- Class picker (inline overlay when changing a state's class) ---

func (ed *Editor) drawClassPicker(w, h int) {
	row := ed.classAssignRows[ed.classAssignCursor]

	boxW := 44
	boxH := len(ed.classAssignClassList) + 5
	if boxH > h-6 {
		boxH = h - 6
	}
	if boxH < 6 {
		boxH = 6
	}

	title := fmt.Sprintf("Class for \"%s\"", row.State)
	if len(title) > boxW-8 {
		title = title[:boxW-11] + "..."
	}

	cx, cy, _, ch := ed.drawOverlayBox(title, boxW, boxH, w, h)

	y := cy + 1
	for i, name := range ed.classAssignClassList {
		if y >= cy+ch-2 {
			break
		}
		label := "  " + name
		if name == fsm.DefaultClassName {
			label += " *"
		}
		s := styleOverlay
		if i == ed.classAssignCursor2 {
			s = styleOverlayHl
		}
		ed.drawString(cx, y, label, s)
		y++
	}

	ed.drawString(cx, cy+ch-1, "[Enter] Select  [Esc] Cancel", styleOverlayDim)
}

func (ed *Editor) handleClassPickerKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		ed.classAssignClassPick = false
		return false
	case tcell.KeyUp:
		if ed.classAssignCursor2 > 0 {
			ed.classAssignCursor2--
		}
	case tcell.KeyDown:
		if ed.classAssignCursor2 < len(ed.classAssignClassList)-1 {
			ed.classAssignCursor2++
		}
	case tcell.KeyEnter:
		// Assign the selected class.
		row := ed.classAssignRows[ed.classAssignCursor]
		className := ed.classAssignClassList[ed.classAssignCursor2]
		f := ed.fsmForMachine(row.Machine)
		if err := f.SetStateClass(row.State, className); err != nil {
			ed.showMessage(err.Error(), MsgError)
		} else {
			ed.classAssignRows[ed.classAssignCursor].Class = className
			ed.showMessage(fmt.Sprintf("%s -> %s", row.State, className), MsgSuccess)
			ed.modified = true
			if ed.isBundle {
				ed.bundleModified[row.Machine] = true
			}
		}
		ed.classAssignClassPick = false
	}
	return false
}

// ====================================================================
// Property Editor (edit property values for a single state)
// ====================================================================

// openPropertyEditor builds the property row list and enters ModePropertyEditor.
func (ed *Editor) openPropertyEditor(machineName, stateName string) {
	f := ed.fsmForMachine(machineName)
	f.EnsureClassMaps()

	// Remember where to go back to.
	ed.propEditorReturnMode = ed.mode

	ed.propEditorMachine = machineName
	ed.propEditorState = stateName
	ed.propEditorEditing = false
	ed.propEditorBuffer = ""

	// Build rows: default_state properties first, then class-specific.
	var rows []propEditorRow

	className := f.GetStateClass(stateName)

	// Default properties.
	if defClass, ok := f.Classes[fsm.DefaultClassName]; ok && len(defClass.Properties) > 0 {
		rows = append(rows, propEditorRow{
			IsHeader: true,
			Label:    fmt.Sprintf("default_state properties"),
		})
		for _, prop := range defClass.Properties {
			val := f.GetStatePropertyValue(stateName, prop.Name)
			if val == nil {
				val = fsm.DefaultValue(prop.Type)
			}
			rows = append(rows, propEditorRow{
				PropDef: prop,
				Value:   val,
			})
		}
	}

	// Class-specific properties (if not default_state).
	if className != fsm.DefaultClassName {
		if cls, ok := f.Classes[className]; ok && len(cls.Properties) > 0 {
			rows = append(rows, propEditorRow{
				IsHeader: true,
				Label:    fmt.Sprintf("%s properties", className),
			})
			for _, prop := range cls.Properties {
				val := f.GetStatePropertyValue(stateName, prop.Name)
				if val == nil {
					val = fsm.DefaultValue(prop.Type)
				}
				rows = append(rows, propEditorRow{
					PropDef: prop,
					Value:   val,
				})
			}
		}
	}

	ed.propEditorProps = rows

	// Set cursor to first non-header row.
	ed.propEditorCursor = 0
	for ed.propEditorCursor < len(rows) && rows[ed.propEditorCursor].IsHeader {
		ed.propEditorCursor++
	}
	ed.propEditorScroll = 0

	ed.mode = ModePropertyEditor
}

func (ed *Editor) drawPropertyEditor(w, h int) {
	f := ed.fsmForMachine(ed.propEditorMachine)
	className := f.GetStateClass(ed.propEditorState)

	boxW := 78
	boxH := len(ed.propEditorProps) + 10
	if boxH > h-4 {
		boxH = h - 4
	}
	if boxH < 12 {
		boxH = 12
	}

	title := fmt.Sprintf("PROPERTIES: %s", ed.propEditorState)
	if ed.propEditorMachine != "" && ed.propEditorMachine != "(current)" {
		title = fmt.Sprintf("%s / %s", ed.propEditorMachine, ed.propEditorState)
	}

	cx, cy, cw, ch := ed.drawOverlayBox(title, boxW, boxH, w, h)

	y := cy + 1
	ed.drawString(cx, y, fmt.Sprintf("Class: %s", className), styleOverlayDim)

	// Property count.
	totalProps := 0
	for _, r := range ed.propEditorProps {
		if !r.IsHeader {
			totalProps++
		}
	}
	countLabel := fmt.Sprintf("%d properties", totalProps)
	ed.drawString(cx+cw-len(countLabel), y, countLabel, styleOverlayDim)
	y += 2

	// Column layout relative to content area.
	nameCol := cx
	typeCol := cx + 24
	valCol := cx + 38

	ed.drawString(nameCol, y, "Property", styleOverlayDim)
	ed.drawString(typeCol, y, "Type", styleOverlayDim)
	ed.drawString(valCol, y, "Value", styleOverlayDim)
	y++

	contentStartY := y
	visibleRows := ch - (y - cy) - 2
	if visibleRows < 1 {
		visibleRows = 1
	}

	// Scroll to keep cursor visible.
	if ed.propEditorCursor < ed.propEditorScroll {
		ed.propEditorScroll = ed.propEditorCursor
	}
	if ed.propEditorCursor >= ed.propEditorScroll+visibleRows {
		ed.propEditorScroll = ed.propEditorCursor - visibleRows + 1
	}
	if ed.propEditorScroll < 0 {
		ed.propEditorScroll = 0
	}

	// Scroll indicator (top).
	if ed.propEditorScroll > 0 {
		ed.drawString(cx+cw-5, contentStartY, " ... ", styleOverlayDim)
	}

	drawn := 0
	for i := ed.propEditorScroll; i < len(ed.propEditorProps) && drawn < visibleRows; i++ {
		row := ed.propEditorProps[i]

		if row.IsHeader {
			ed.drawString(nameCol, y, "-- "+row.Label+" --", styleOverlayHdr)
			y++
			drawn++
			continue
		}

		isCurrent := (i == ed.propEditorCursor)

		// Property name.
		s := styleOverlay
		if isCurrent {
			s = styleOverlayHl
		}
		ed.drawString(nameCol, y, row.PropDef.Name, s)

		// Type.
		ed.drawString(typeCol, y, string(row.PropDef.Type), styleOverlayDim)

		// Value.
		if isCurrent && ed.propEditorEditing {
			buf := ed.propEditorBuffer + "_"
			maxBufW := cw - (valCol - cx) - 1
			if len(buf) > maxBufW && maxBufW > 3 {
				buf = buf[len(buf)-maxBufW:]
			}
			ed.drawString(valCol, y, buf, styleOverlayEdt)
		} else {
			valStr := formatPropertyValue(row.PropDef.Type, row.Value)
			maxValW := cw - (valCol - cx) - 1
			if len(valStr) > maxValW && maxValW > 3 {
				valStr = valStr[:maxValW-3] + "..."
			}
			if isCurrent {
				ed.drawString(valCol, y, valStr, styleOverlayHl)
			} else {
				ed.drawString(valCol, y, valStr, styleOverlay)
			}
		}
		y++
		drawn++
	}

	// Scroll indicator (bottom).
	if ed.propEditorScroll+drawn < len(ed.propEditorProps) {
		ed.drawString(cx+cw-5, y-1, " ... ", styleOverlayDim)
	}

	// Help text.
	helpY := cy + ch - 1
	if ed.propEditorEditing {
		ed.drawString(cx, helpY, "[Enter] Confirm  [Esc] Cancel edit", styleOverlayDim)
	} else {
		ed.drawString(cx, helpY, "[Enter] Edit value  [Esc] Back", styleOverlayDim)
	}
}

func (ed *Editor) handlePropertyEditorKey(ev *tcell.EventKey) bool {
	rows := ed.propEditorProps

	if ed.propEditorEditing {
		return ed.handlePropertyEditInput(ev)
	}

	switch ev.Key() {
	case tcell.KeyEscape:
		// Return to wherever we came from.
		switch ed.propEditorReturnMode {
		case ModeClassAssign:
			ed.openClassAssign()
		default:
			ed.mode = ed.propEditorReturnMode
		}
		return false
	case tcell.KeyUp:
		ed.propEditorCursor--
		for ed.propEditorCursor >= 0 && rows[ed.propEditorCursor].IsHeader {
			ed.propEditorCursor--
		}
		if ed.propEditorCursor < 0 {
			ed.propEditorCursor = 0
			for ed.propEditorCursor < len(rows) && rows[ed.propEditorCursor].IsHeader {
				ed.propEditorCursor++
			}
		}
	case tcell.KeyDown:
		ed.propEditorCursor++
		for ed.propEditorCursor < len(rows) && rows[ed.propEditorCursor].IsHeader {
			ed.propEditorCursor++
		}
		if ed.propEditorCursor >= len(rows) {
			ed.propEditorCursor = len(rows) - 1
			for ed.propEditorCursor >= 0 && rows[ed.propEditorCursor].IsHeader {
				ed.propEditorCursor--
			}
		}
	case tcell.KeyEnter:
		if ed.propEditorCursor >= 0 && ed.propEditorCursor < len(rows) && !rows[ed.propEditorCursor].IsHeader {
			row := rows[ed.propEditorCursor]
			if row.PropDef.Type == fsm.PropBool {
				// Toggle directly.
				bVal, _ := row.Value.(bool)
				ed.commitPropertyValue(row.PropDef, !bVal)
			} else if row.PropDef.Type == fsm.PropList {
				// Open list editor popup.
				ed.openListEditor(row)
			} else {
				// Enter edit mode with current value as buffer.
				ed.propEditorEditing = true
				ed.propEditorBuffer = formatPropertyValue(row.PropDef.Type, row.Value)
			}
		}
	}
	return false
}

// handlePropertyEditInput handles keystrokes while editing a property value.
func (ed *Editor) handlePropertyEditInput(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		ed.propEditorEditing = false
		ed.propEditorBuffer = ""
		return false
	case tcell.KeyEnter:
		// Parse and commit the value.
		row := ed.propEditorProps[ed.propEditorCursor]
		val, err := parsePropertyValue(row.PropDef.Type, ed.propEditorBuffer)
		if err != nil {
			ed.showMessage(err.Error(), MsgError)
		} else {
			ed.commitPropertyValue(row.PropDef, val)
		}
		ed.propEditorEditing = false
		ed.propEditorBuffer = ""
		return false
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(ed.propEditorBuffer) > 0 {
			ed.propEditorBuffer = ed.propEditorBuffer[:len(ed.propEditorBuffer)-1]
		}
	case tcell.KeyRune:
		r := ev.Rune()
		row := ed.propEditorProps[ed.propEditorCursor]
		// Enforce [40]string length limit.
		if row.PropDef.Type == fsm.PropShortString && len(ed.propEditorBuffer) >= 40 {
			return false
		}
		ed.propEditorBuffer += string(r)
	}
	return false
}

// commitPropertyValue writes a property value to the FSM and updates the row.
func (ed *Editor) commitPropertyValue(prop fsm.PropertyDef, val interface{}) {
	f := ed.fsmForMachine(ed.propEditorMachine)
	f.SetStatePropertyValue(ed.propEditorState, prop.Name, val)
	ed.propEditorProps[ed.propEditorCursor].Value = val
	ed.modified = true
	if ed.isBundle {
		ed.bundleModified[ed.propEditorMachine] = true
	}
}

// ====================================================================
// Value formatting and parsing
// ====================================================================

// formatPropertyValue renders a property value as a display string.
func formatPropertyValue(typ fsm.PropertyType, val interface{}) string {
	if val == nil {
		return ""
	}
	switch typ {
	case fsm.PropBool:
		if b, ok := val.(bool); ok {
			if b {
				return "true"
			}
			return "false"
		}
	case fsm.PropFloat64:
		switch v := val.(type) {
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		case int64:
			return strconv.FormatFloat(float64(v), 'f', -1, 64)
		}
	case fsm.PropInt64:
		switch v := val.(type) {
		case int64:
			return strconv.FormatInt(v, 10)
		case float64:
			return strconv.FormatInt(int64(v), 10)
		}
	case fsm.PropUint64:
		switch v := val.(type) {
		case uint64:
			return strconv.FormatUint(v, 10)
		case float64:
			return strconv.FormatUint(uint64(v), 10)
		}
	case fsm.PropShortString, fsm.PropString:
		if s, ok := val.(string); ok {
			return s
		}
	case fsm.PropList:
		switch items := val.(type) {
		case []string:
			if len(items) == 0 {
				return "(empty)"
			}
			return fmt.Sprintf("[%d items]", len(items))
		case []interface{}:
			if len(items) == 0 {
				return "(empty)"
			}
			return fmt.Sprintf("[%d items]", len(items))
		}
		return "(empty)"
	}
	return fmt.Sprintf("%v", val)
}

// parsePropertyValue converts a string to the appropriate Go type.
func parsePropertyValue(typ fsm.PropertyType, s string) (interface{}, error) {
	s = strings.TrimSpace(s)
	switch typ {
	case fsm.PropFloat64:
		return strconv.ParseFloat(s, 64)
	case fsm.PropInt64:
		return strconv.ParseInt(s, 10, 64)
	case fsm.PropUint64:
		return strconv.ParseUint(s, 10, 64)
	case fsm.PropShortString:
		if len(s) > 40 {
			return nil, fmt.Errorf("value exceeds 40 characters (got %d)", len(s))
		}
		return s, nil
	case fsm.PropString:
		return s, nil
	case fsm.PropBool:
		return strconv.ParseBool(s)
	default:
		return s, nil
	}
}
