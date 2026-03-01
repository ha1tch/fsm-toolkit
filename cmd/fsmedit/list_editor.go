package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// openListEditor initialises the list editor from a property row's value.
func (ed *Editor) openListEditor(row propEditorRow) {
	var items []string
	switch v := row.Value.(type) {
	case []string:
		items = make([]string, len(v))
		copy(items, v)
	case []interface{}:
		items = make([]string, 0, len(v))
		for _, elem := range v {
			items = append(items, fmt.Sprintf("%v", elem))
		}
	default:
		items = []string{}
	}

	ed.listEditorItems = items
	ed.listEditorCursor = 0
	ed.listEditorScroll = 0
	ed.listEditorAdding = false
	ed.listEditorEditIdx = -1
	ed.listEditorBuffer = ""
	ed.mode = ModeListEditor
}

func (ed *Editor) drawListEditor(w, h int) {
	title := "Edit List"
	if ed.propEditorCursor >= 0 && ed.propEditorCursor < len(ed.propEditorProps) {
		row := ed.propEditorProps[ed.propEditorCursor]
		if row.PropDef.Name != "" {
			title = fmt.Sprintf("Edit: %s", row.PropDef.Name)
		}
	}

	boxW := 52
	boxH := len(ed.listEditorItems) + 8
	if boxH > h-6 {
		boxH = h - 6
	}
	if boxH < 10 {
		boxH = 10
	}

	cx, cy, cw, ch := ed.drawOverlayBox(title, boxW, boxH, w, h)

	y := cy + 1

	// Item count.
	countLabel := fmt.Sprintf("%d items", len(ed.listEditorItems))
	ed.drawString(cx+cw-len(countLabel), y, countLabel, styleOverlayDim)

	if len(ed.listEditorItems) == 0 && !ed.listEditorAdding {
		ed.drawString(cx, y, "(empty list)", styleOverlayDim)
		y++
	} else {
		// Scroll to keep cursor visible.
		visibleRows := ch - 5
		if visibleRows < 1 {
			visibleRows = 1
		}
		if ed.listEditorCursor < ed.listEditorScroll {
			ed.listEditorScroll = ed.listEditorCursor
		}
		if ed.listEditorCursor >= ed.listEditorScroll+visibleRows {
			ed.listEditorScroll = ed.listEditorCursor - visibleRows + 1
		}
		if ed.listEditorScroll < 0 {
			ed.listEditorScroll = 0
		}

		drawn := 0
		for i := ed.listEditorScroll; i < len(ed.listEditorItems) && drawn < visibleRows; i++ {
			item := ed.listEditorItems[i]

			// If editing this item inline, show the edit buffer.
			if ed.listEditorAdding && ed.listEditorEditIdx == i {
				label := fmt.Sprintf(" %d. %s_", i+1, ed.listEditorBuffer)
				maxW := cw - 1
				if len(label) > maxW && maxW > 5 {
					label = label[len(label)-maxW:]
				}
				ed.drawString(cx, y, label, styleOverlayEdt)
			} else {
				label := fmt.Sprintf(" %d. %s", i+1, item)
				maxW := cw - 1
				if len(label) > maxW && maxW > 3 {
					label = label[:maxW-3] + "..."
				}
				s := styleOverlay
				if i == ed.listEditorCursor && !ed.listEditorAdding {
					s = styleOverlayHl
				}
				ed.drawString(cx, y, label, s)
			}
			y++
			drawn++
		}

		// Scroll indicator.
		if ed.listEditorScroll+drawn < len(ed.listEditorItems) {
			ed.drawString(cx+cw-5, y-1, " ... ", styleOverlayDim)
		}
	}

	// Add-item input line (only for new items, not inline edits).
	addLineY := cy + ch - 3
	if ed.listEditorAdding && ed.listEditorEditIdx < 0 {
		prompt := " > " + ed.listEditorBuffer + "_"
		maxW := cw - 1
		if len(prompt) > maxW && maxW > 5 {
			prompt = prompt[len(prompt)-maxW:]
		}
		ed.drawString(cx, addLineY, prompt, styleOverlayEdt)
	}

	// Help text.
	helpY := cy + ch - 1
	if ed.listEditorAdding {
		ed.drawString(cx, helpY, "[Enter] Confirm  [Esc] Cancel", styleOverlayDim)
	} else {
		ed.drawString(cx, helpY, "[A] Add  [D] Del  [E] Edit  [Esc] Done", styleOverlayDim)
	}
}

func (ed *Editor) handleListEditorKey(ev *tcell.EventKey) bool {
	if ed.listEditorAdding {
		return ed.handleListEditorInput(ev)
	}

	switch ev.Key() {
	case tcell.KeyEscape:
		ed.commitListValue()
		ed.mode = ModePropertyEditor
		return false
	case tcell.KeyUp:
		if ed.listEditorCursor > 0 {
			ed.listEditorCursor--
		}
	case tcell.KeyDown:
		if ed.listEditorCursor < len(ed.listEditorItems)-1 {
			ed.listEditorCursor++
		}
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'a', 'A':
			ed.listEditorAdding = true
			ed.listEditorEditIdx = -1
			ed.listEditorBuffer = ""
		case 'd', 'D':
			if len(ed.listEditorItems) > 0 && ed.listEditorCursor < len(ed.listEditorItems) {
				ed.listEditorItems = append(
					ed.listEditorItems[:ed.listEditorCursor],
					ed.listEditorItems[ed.listEditorCursor+1:]...,
				)
				if ed.listEditorCursor >= len(ed.listEditorItems) && ed.listEditorCursor > 0 {
					ed.listEditorCursor--
				}
			}
		case 'e', 'E':
			if len(ed.listEditorItems) > 0 && ed.listEditorCursor < len(ed.listEditorItems) {
				ed.listEditorAdding = true
				ed.listEditorEditIdx = ed.listEditorCursor
				ed.listEditorBuffer = ed.listEditorItems[ed.listEditorCursor]
			}
		}
	}
	return false
}

// handleListEditorInput handles text input when adding or editing a list item.
func (ed *Editor) handleListEditorInput(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		ed.listEditorAdding = false
		ed.listEditorEditIdx = -1
		ed.listEditorBuffer = ""
		return false
	case tcell.KeyEnter:
		item := ed.listEditorBuffer
		if item == "" {
			ed.listEditorAdding = false
			ed.listEditorEditIdx = -1
			return false
		}
		if ed.listEditorEditIdx >= 0 && ed.listEditorEditIdx < len(ed.listEditorItems) {
			// Replace existing item.
			ed.listEditorItems[ed.listEditorEditIdx] = item
			ed.listEditorCursor = ed.listEditorEditIdx
		} else {
			// Append new item.
			ed.listEditorItems = append(ed.listEditorItems, item)
			ed.listEditorCursor = len(ed.listEditorItems) - 1
		}
		ed.listEditorAdding = false
		ed.listEditorEditIdx = -1
		ed.listEditorBuffer = ""
		return false
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(ed.listEditorBuffer) > 0 {
			ed.listEditorBuffer = ed.listEditorBuffer[:len(ed.listEditorBuffer)-1]
		}
	case tcell.KeyRune:
		ed.listEditorBuffer += string(ev.Rune())
	}
	return false
}

// commitListValue writes the list items back to the property.
func (ed *Editor) commitListValue() {
	if ed.propEditorCursor < 0 || ed.propEditorCursor >= len(ed.propEditorProps) {
		return
	}
	row := ed.propEditorProps[ed.propEditorCursor]
	items := make([]string, len(ed.listEditorItems))
	copy(items, ed.listEditorItems)
	ed.commitPropertyValue(row.PropDef, items)
}

// isListType checks if a PropertyDef is a list type.
func isListType(prop fsm.PropertyDef) bool {
	return prop.Type == fsm.PropList
}
