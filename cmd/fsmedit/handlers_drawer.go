// Component drawer interaction handlers for fsmedit.
package main

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// openDrawer opens the component drawer with slide-up animation.
func (ed *Editor) openDrawer() {
	if len(ed.catalog) == 0 {
		ed.showMessage("No component libraries loaded (Settings > [L])", MsgInfo)
		return
	}
	ed.drawerOpen = true
	ed.drawerAnimating = true
	ed.drawerAnimStart = time.Now().UnixMilli()
	ed.drawerAnimDir = 1 // opening
	ed.drawerMaxHeight = drawerTargetHeight
	ed.mode = ModeDrawer
}

// closeDrawer closes the drawer with slide-down animation.
func (ed *Editor) closeDrawer() {
	ed.drawerAnimating = true
	ed.drawerAnimStart = time.Now().UnixMilli()
	ed.drawerAnimDir = -1 // closing
}

// finishDrawerAnimation completes the animation and sets final state.
func (ed *Editor) finishDrawerAnimation() {
	ed.drawerAnimating = false
	if ed.drawerAnimDir < 0 {
		ed.drawerOpen = false
		ed.drawerHeight = 0
		ed.mode = ModeCanvas
	} else {
		ed.drawerHeight = drawerTargetHeight
	}
}

// checkDrawerAnimation checks if the animation has completed.
func (ed *Editor) checkDrawerAnimation() {
	if !ed.drawerAnimating {
		return
	}
	elapsed := time.Now().UnixMilli() - ed.drawerAnimStart
	if elapsed >= drawerAnimDuration {
		ed.finishDrawerAnimation()
	}
}

// handleDrawerKey handles keyboard input when the drawer is open.
func (ed *Editor) handleDrawerKey(ev *tcell.EventKey) bool {
	// Check animation completion on any keypress.
	ed.checkDrawerAnimation()

	switch ev.Key() {
	case tcell.KeyEscape:
		ed.closeDrawer()
		return false
	case tcell.KeyTab:
		// Next category.
		if len(ed.catalog) > 0 {
			ed.drawerCatIdx = (ed.drawerCatIdx + 1) % len(ed.catalog)
			ed.drawerItemIdx = 0
			ed.drawerScroll = 0
		}
	case tcell.KeyBacktab:
		// Previous category.
		if len(ed.catalog) > 0 {
			ed.drawerCatIdx--
			if ed.drawerCatIdx < 0 {
				ed.drawerCatIdx = len(ed.catalog) - 1
			}
			ed.drawerItemIdx = 0
			ed.drawerScroll = 0
		}
	case tcell.KeyLeft:
		if ed.drawerItemIdx > 0 {
			ed.drawerItemIdx--
			ed.ensureDrawerItemVisible()
		}
	case tcell.KeyRight:
		if ed.drawerCatIdx >= 0 && ed.drawerCatIdx < len(ed.catalog) {
			cat := ed.catalog[ed.drawerCatIdx]
			if ed.drawerItemIdx < len(cat.Classes)-1 {
				ed.drawerItemIdx++
				ed.ensureDrawerItemVisible()
			}
		}
	case tcell.KeyEnter:
		// Place selected component at cursor position on canvas.
		ed.placeSelectedComponent()
	case tcell.KeyUp:
		// Up arrow: go back to canvas without closing drawer? No — close.
		ed.closeDrawer()
	}
	return false
}

// ensureDrawerItemVisible adjusts scroll so the selected item is visible.
func (ed *Editor) ensureDrawerItemVisible() {
	w, _ := ed.screen.Size()
	visibleCards := (w - 2) / drawerCardWidth
	if visibleCards < 1 {
		visibleCards = 1
	}

	if ed.drawerItemIdx < ed.drawerScroll {
		ed.drawerScroll = ed.drawerItemIdx
	}
	if ed.drawerItemIdx >= ed.drawerScroll+visibleCards {
		ed.drawerScroll = ed.drawerItemIdx - visibleCards + 1
	}
}

// placeSelectedComponent places the currently selected drawer component
// at the cursor position on the canvas.
func (ed *Editor) placeSelectedComponent() {
	if ed.drawerCatIdx < 0 || ed.drawerCatIdx >= len(ed.catalog) {
		return
	}
	cat := ed.catalog[ed.drawerCatIdx]
	if ed.drawerItemIdx < 0 || ed.drawerItemIdx >= len(cat.Classes) {
		return
	}
	cls := cat.Classes[ed.drawerItemIdx]

	// Find a free position near the current cursor/viewport centre.
	w, h := ed.screen.Size()
	dividerX := w - ed.sidebarWidth
	canvasW := dividerX
	canvasH := h - 2

	posX := ed.canvasOffsetX + canvasW/2
	posY := ed.canvasOffsetY + canvasH/2

	ed.instantiateComponent(cls, posX, posY)
	ed.closeDrawer()
}

// instantiateComponent creates a new state from a class template.
func (ed *Editor) instantiateComponent(cls *fsm.Class, canvasX, canvasY int) {
	// Generate unique state name using short part number.
	shortName, _ := splitClassName(cls.Name)
	baseName := shortName
	if baseName == cls.Name {
		// No numeric prefix found, use the full name.
		baseName = cls.Name
	}

	name := baseName
	suffix := 1
	for ed.fsm.HasState(name) {
		name = fmt.Sprintf("%s_%d", baseName, suffix)
		suffix++
	}

	// Ensure the class exists in the FSM.
	ed.fsm.EnsureClassMaps()
	if _, exists := ed.fsm.Classes[cls.Name]; !exists {
		// Deep copy the class to avoid shared mutation.
		clsCopy := &fsm.Class{
			Name:       cls.Name,
			Parent:     cls.Parent,
			Properties: make([]fsm.PropertyDef, len(cls.Properties)),
		}
		copy(clsCopy.Properties, cls.Properties)
		if len(cls.Ports) > 0 {
			clsCopy.Ports = make([]fsm.Port, len(cls.Ports))
			copy(clsCopy.Ports, cls.Ports)
		}
		ed.fsm.Classes[cls.Name] = clsCopy
	}

	// Snapshot for undo.
	ed.saveSnapshot()

	// Add state.
	ed.fsm.AddState(name)

	// Assign class.
	ed.fsm.SetStateClass(name, cls.Name)

	// Initialise all properties to defaults.
	effectiveProps := ed.fsm.EffectiveProperties(name)
	for _, prop := range effectiveProps {
		ed.fsm.SetStatePropertyValue(name, prop.Name, fsm.DefaultValue(prop.Type))
	}

	// Place on canvas.
	ed.states = append(ed.states, StatePos{Name: name, X: canvasX, Y: canvasY})
	ed.selectedState = len(ed.states) - 1
	ed.modified = true

	ed.showMessage("Added "+name+" ("+cls.Name+")", MsgSuccess)
}

// --- Mouse interactions for the drawer ---

// drawerHitTest checks if a mouse click is on the [+] button.
// Returns true if the click was consumed.
func (ed *Editor) drawerButtonHitTest(mx, my, w, h int) bool {
	if ed.drawerOpen || ed.drawerAnimating {
		return false
	}
	if len(ed.catalog) == 0 {
		return false
	}

	btnX := w - len(drawerButtonLabel)
	btnY := h - 2
	if my == btnY && mx >= btnX && mx < btnX+len(drawerButtonLabel) {
		ed.openDrawer()
		return true
	}
	return false
}

// drawerCardHitTest checks if a mouse position is over a component card.
// Returns the class if hit, or nil.
func (ed *Editor) drawerCardHitTest(mx, my, w, h int) *fsm.Class {
	if !ed.drawerOpen || ed.drawerAnimating {
		return nil
	}
	if ed.drawerCatIdx < 0 || ed.drawerCatIdx >= len(ed.catalog) {
		return nil
	}

	dh := ed.drawerEffectiveHeight()
	drawerY := h - dh
	cardY := drawerY + 1

	// Check if in card row area.
	if my < cardY || my >= cardY+drawerCardHeight {
		return nil
	}

	cat := ed.catalog[ed.drawerCatIdx]
	cardX := 1 - ed.drawerScroll*drawerCardWidth

	for i, cls := range cat.Classes {
		x0 := cardX + i*drawerCardWidth
		if mx >= x0 && mx < x0+drawerCardWidth-1 {
			ed.drawerItemIdx = i
			return cls
		}
	}
	return nil
}

// drawerTabHitTest checks if a mouse click is on a category tab.
// Returns true if a tab was clicked.
func (ed *Editor) drawerTabHitTest(mx, my, w, h int) bool {
	if !ed.drawerOpen {
		return false
	}

	dh := ed.drawerEffectiveHeight()
	drawerY := h - dh

	if my != drawerY {
		return false
	}

	tabX := 1
	for i, cat := range ed.catalog {
		label := " " + cat.Name + " "
		if mx >= tabX && mx < tabX+len(label) {
			ed.drawerCatIdx = i
			ed.drawerItemIdx = 0
			ed.drawerScroll = 0
			return true
		}
		tabX += len(label) + 1
	}
	return false
}

// handleDrawerMouse processes mouse events when the drawer is open or
// when a drag is in progress. Returns true if the event was consumed.
func (ed *Editor) handleDrawerMouse(ev *tcell.EventMouse, w, h int) bool {
	mx, my := ev.Position()
	buttons := ev.Buttons()

	ed.checkDrawerAnimation()

	// Handle ongoing drag.
	if ed.drawerDragging {
		if buttons&tcell.Button1 != 0 {
			// Update drag position.
			ed.drawerDragX = mx
			ed.drawerDragY = my
			return true
		}
		// Button released — drop.
		ed.handleDrawerDrop(mx, my, w, h)
		return true
	}

	// Check for button click when drawer is closed.
	if !ed.drawerOpen {
		if buttons&tcell.Button1 != 0 {
			return ed.drawerButtonHitTest(mx, my, w, h)
		}
		return false
	}

	// Drawer is open. Check for interactions.
	dh := ed.drawerEffectiveHeight()
	drawerY := h - dh

	// Click outside drawer area (above it) — close.
	if my < drawerY && buttons&tcell.Button1 != 0 {
		// Don't close — might be a canvas click. Let it pass through.
		return false
	}

	if buttons&tcell.Button1 != 0 {
		// Tab click.
		if ed.drawerTabHitTest(mx, my, w, h) {
			return true
		}

		// Card click — start drag.
		cls := ed.drawerCardHitTest(mx, my, w, h)
		if cls != nil {
			ed.drawerDragging = true
			ed.drawerDragClass = cls
			ed.drawerDragX = mx
			ed.drawerDragY = my
			return true
		}
	}

	return my >= drawerY // Consume clicks within drawer area.
}

// handleDrawerDrop completes a drag-and-drop from the drawer.
func (ed *Editor) handleDrawerDrop(mx, my, w, h int) {
	ed.drawerDragging = false
	cls := ed.drawerDragClass
	ed.drawerDragClass = nil

	if cls == nil {
		return
	}

	// Check if dropped on canvas (above drawer, left of sidebar).
	dh := ed.drawerEffectiveHeight()
	drawerY := h - dh
	dividerX := w - ed.sidebarWidth

	if my >= drawerY || mx >= dividerX {
		// Dropped back on drawer or sidebar — cancel.
		return
	}

	// Convert screen position to canvas position.
	canvasX := mx + ed.canvasOffsetX
	canvasY := my + ed.canvasOffsetY

	// Account for breadcrumb bar offset.
	if len(ed.navStack) > 0 && ed.isBundle {
		canvasY -= 1
	}

	ed.instantiateComponent(cls, canvasX, canvasY)
	ed.closeDrawer()
}
