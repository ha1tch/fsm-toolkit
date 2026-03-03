// Connection detail window for fsmedit.
// Opened via 'c' on a selected state, shows pin-to-pin connections
// between two components with port grouping and multi-fan-out footnotes.
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// netDetailConn represents one connection row in the detail window.
type netDetailConn struct {
	PortA  string // port name on state A
	GroupA string // group on state A
	DirA   fsm.PortDir
	Net    string // net name
	PortB  string // port name on state B
	GroupB string // group on state B
	DirB   fsm.PortDir
	Power  bool // true if this is a power connection
}

// netDetailFootnote represents a multi-fan-out or self-connection footnote.
type netDetailFootnote struct {
	NetName string
	Text    string
}

// --- Entry point ---

// openNetDetail is called when the user presses 'c' on a selected state.
// It finds connected peers and either opens the detail window directly
// or presents a picker if multiple peers exist.
func (ed *Editor) openNetDetail() {
	if ed.selectedState < 0 || ed.selectedState >= len(ed.states) {
		ed.showMessage("Select a state first", MsgInfo)
		return
	}
	stateA := ed.states[ed.selectedState].Name

	if !ed.fsm.HasNets() {
		ed.showMessage("No nets defined", MsgInfo)
		return
	}

	// Find all unique peers connected via signal nets.
	peerSet := make(map[string]bool)
	for _, n := range ed.fsm.Nets {
		if ed.fsm.IsPowerNet(n) {
			continue
		}
		if !n.HasInstance(stateA) {
			continue
		}
		for _, ep := range n.Endpoints {
			if ep.Instance != stateA {
				peerSet[ep.Instance] = true
			}
		}
		// Self-connections: nets with 2+ endpoints on stateA.
		count := 0
		for _, ep := range n.Endpoints {
			if ep.Instance == stateA {
				count++
			}
		}
		if count >= 2 {
			peerSet[stateA] = true // self
		}
	}

	if len(peerSet) == 0 {
		ed.showMessage("No signal connections on "+stateA, MsgInfo)
		return
	}

	// Build sorted peer list.
	var peers []string
	for p := range peerSet {
		peers = append(peers, p)
	}
	sort.Strings(peers)

	if len(peers) == 1 {
		// Direct open.
		ed.netDetailOpen(stateA, peers[0])
		return
	}

	// Multiple peers — show a picker.
	ed.netDetailPeers = peers
	ed.netDetailPeerCursor = 0
	ed.netDetailPeerStateA = stateA
	ed.mode = ModeNetDetailPeer
}

// netDetailOpen populates the connection rows and switches to detail mode.
func (ed *Editor) netDetailOpen(stateA, stateB string) {
	ed.netDetailStateA = stateA
	ed.netDetailStateB = stateB
	ed.netDetailSelected = 0
	ed.netDetailScroll = 0

	ed.netDetailBuildRows()
	ed.mode = ModeNetDetail
}

// netDetailBuildRows recomputes the connection rows and footnotes.
func (ed *Editor) netDetailBuildRows() {
	stateA := ed.netDetailStateA
	stateB := ed.netDetailStateB

	portsA := ed.fsm.EffectivePorts(stateA)
	portsB := ed.fsm.EffectivePorts(stateB)

	portMapA := make(map[string]fsm.Port)
	for _, p := range portsA {
		portMapA[p.Name] = p
	}
	portMapB := make(map[string]fsm.Port)
	for _, p := range portsB {
		portMapB[p.Name] = p
	}

	var rows []netDetailConn
	var footnotes []netDetailFootnote

	// Walk all nets, collecting connections between A and B.
	for _, n := range ed.fsm.Nets {
		isPower := ed.fsm.IsPowerNet(n)

		var epsA, epsB []fsm.NetEndpoint
		var epsOther []fsm.NetEndpoint

		for _, ep := range n.Endpoints {
			switch ep.Instance {
			case stateA:
				epsA = append(epsA, ep)
			case stateB:
				epsB = append(epsB, ep)
			default:
				epsOther = append(epsOther, ep)
			}
		}

		// Self-connection case: both endpoints on the same state.
		if stateA == stateB {
			if len(epsA) >= 2 {
				// Pair up endpoints for display.
				for i := 0; i < len(epsA)-1; i += 2 {
					epL := epsA[i]
					epR := epsA[i+1]
					pL := portMapA[epL.Port]
					pR := portMapA[epR.Port]
					rows = append(rows, netDetailConn{
						PortA: epL.Port, GroupA: pL.Group, DirA: pL.Direction,
						Net:   n.Name,
						PortB: epR.Port, GroupB: pR.Group, DirB: pR.Direction,
						Power: isPower,
					})
				}
				// Add self-connection footnote.
				footnotes = append(footnotes, netDetailFootnote{
					NetName: n.Name,
					Text:    n.Name + ": internal connection within same package",
				})
			}
			continue
		}

		// Normal case: net spans A and B.
		if len(epsA) == 0 || len(epsB) == 0 {
			continue
		}

		// Create a row for each A-B endpoint pair in this net.
		for _, epA := range epsA {
			for _, epB := range epsB {
				pA := portMapA[epA.Port]
				pB := portMapB[epB.Port]
				rows = append(rows, netDetailConn{
					PortA: epA.Port, GroupA: pA.Group, DirA: pA.Direction,
					Net:   n.Name,
					PortB: epB.Port, GroupB: pB.Group, DirB: pB.Direction,
					Power: isPower,
				})
			}
		}

		// Multi-fan-out footnote.
		if len(epsOther) > 0 {
			var parts []string
			for _, ep := range epsOther {
				parts = append(parts, ep.Instance+"."+ep.Port)
			}
			footnotes = append(footnotes, netDetailFootnote{
				NetName: n.Name,
				Text:    fmt.Sprintf("%s also connects to: %s", n.Name, strings.Join(parts, ", ")),
			})
		}
	}

	// Sort rows by group, then port name.
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].GroupA != rows[j].GroupA {
			return rows[i].GroupA < rows[j].GroupA
		}
		return rows[i].PortA < rows[j].PortA
	})

	ed.netDetailRows = rows
	ed.netDetailFootnotes = footnotes
}

// --- Draw ---

func (ed *Editor) drawNetDetail(w, h int) {
	stateA := ed.netDetailStateA
	stateB := ed.netDetailStateB
	rows := ed.netDetailRows
	footnotes := ed.netDetailFootnotes

	// Determine class names for display.
	classA := ed.fsm.GetStateClass(stateA)
	classB := ed.fsm.GetStateClass(stateB)

	// Box sizing — fill most of the canvas.
	boxW := w - 8
	if boxW > 78 {
		boxW = 78
	}
	if boxW < 40 {
		boxW = 40
	}
	boxH := h - 4
	if boxH < 16 {
		boxH = 16
	}
	if boxH > h-2 {
		boxH = h - 2
	}

	cx, cy, cw, ch := ed.drawOverlayBox("CONNECTIONS", boxW, boxH, w, h)

	y := cy + 1

	// Title: component names and classes.
	titleA := stateA
	if classA != fsm.DefaultClassName {
		titleA += " (" + classA + ")"
	}
	titleB := stateB
	if classB != fsm.DefaultClassName {
		titleB += " (" + classB + ")"
	}
	if stateA == stateB {
		title := fmt.Sprintf("Self-connections: %s", titleA)
		ed.drawString(cx, y, truncate(title, cw), styleOverlay)
	} else {
		title := fmt.Sprintf("%s  <-->  %s", titleA, titleB)
		ed.drawString(cx, y, truncate(title, cw), styleOverlay)
	}
	y++

	// Column headers.
	y++
	colA := cx
	colNet := cx + cw/3
	colB := cx + 2*cw/3

	ed.drawString(colA, y, truncate(stateA, cw/3-1), styleOverlayHdr)
	ed.drawString(colNet, y, "Net", styleOverlayHdr)
	ed.drawString(colB, y, truncate(stateB, cw/3-1), styleOverlayHdr)
	y++

	// Separator.
	for x := cx; x < cx+cw; x++ {
		ed.screen.SetContent(x, y, '─', nil, styleOverlayBrd)
	}
	y++

	// Scrollable connection rows.
	footH := len(footnotes)
	if footH > 0 {
		footH += 2 // separator + gap
	}
	helpH := 1 // controls line
	listH := ch - (y - cy + 1) - footH - helpH - 1
	if listH < 3 {
		listH = 3
	}

	if len(rows) == 0 {
		ed.drawString(cx+2, y, "(no signal connections)", styleOverlayDim)
		y++
	} else {
		// Ensure selected row is visible.
		ed.netDetailScroll = ensureVisible(ed.netDetailSelected, ed.netDetailScroll, listH)

		// Track current group for group header insertion.
		lastGroupA := ""
		drawIdx := 0

		for i := 0; i < len(rows); i++ {
			row := rows[i]

			// Count effective draw lines for this row (group header + data).
			needsGroupHeader := row.GroupA != lastGroupA || (row.GroupB != "" && i > 0 && rows[i-1].GroupB != row.GroupB)

			if i < ed.netDetailScroll {
				if needsGroupHeader {
					lastGroupA = row.GroupA
				}
				continue
			}

			if drawIdx >= listH {
				break
			}

			// Group header.
			if needsGroupHeader && drawIdx < listH {
				if row.GroupA != "" || row.GroupB != "" {
					gA := row.GroupA
					if gA == "" {
						gA = "---"
					}
					gB := row.GroupB
					if gB == "" {
						gB = "---"
					}
					ed.drawString(colA, y, fmt.Sprintf("[%s]", gA), styleOverlayGroup)
					ed.drawString(colB, y, fmt.Sprintf("[%s]", gB), styleOverlayGroup)
					y++
					drawIdx++
					if drawIdx >= listH {
						break
					}
				}
				lastGroupA = row.GroupA
			}

			// Choose style based on selection state and net type.
			isSelected := i == ed.netDetailSelected
			var s tcell.Style
			if row.Power {
				if isSelected {
					s = styleOverlayPowerHl
				} else {
					s = styleOverlayPower
				}
			} else {
				if isSelected {
					s = styleOverlaySignalHl
				} else {
					s = styleOverlaySignal
				}
			}

			// Fill entire row width for continuous background.
			if isSelected {
				for x := cx; x < cx+cw; x++ {
					ed.screen.SetContent(x, y, ' ', nil, s)
				}
			}

			// Direction arrow.
			arrow := "────"
			switch {
			case row.DirA == fsm.PortOutput && row.DirB == fsm.PortInput:
				arrow = "───>"
			case row.DirA == fsm.PortInput && row.DirB == fsm.PortOutput:
				arrow = "<───"
			case row.DirA == fsm.PortBidir || row.DirB == fsm.PortBidir:
				arrow = "<──>"
			case row.Power:
				arrow = " ── "
			}

			// Render: portA ──── NetName ──── portB
			portAStr := truncate(row.PortA, cw/3-2)
			netStr := truncate(row.Net, cw/3-2)
			portBStr := truncate(row.PortB, cw/3-2)

			ed.drawString(colA, y, portAStr, s)

			// Draw connection line from end of portA to start of net label.
			lineStartX := colA + len(portAStr) + 1
			netLabelX := colNet
			for x := lineStartX; x < netLabelX; x++ {
				ed.screen.SetContent(x, y, '─', nil, s)
			}

			// Net name in white (selected) or appropriate colour.
			netStyle := s
			if isSelected {
				netStyle = styleOverlayHl
			} else {
				netStyle = styleOverlay
			}
			ed.drawString(colNet, y, netStr, netStyle)

			// Arrow after net name.
			arrowX := colNet + len(netStr) + 1
			for ai, ach := range arrow {
				px := arrowX + ai
				if px < colB-1 {
					ed.screen.SetContent(px, y, ach, nil, s)
				}
			}
			// Line from arrow to portB.
			lineEnd := colB - 1
			for x := arrowX + len(arrow); x < lineEnd; x++ {
				ed.screen.SetContent(x, y, '─', nil, s)
			}

			ed.drawString(colB, y, portBStr, s)

			y++
			drawIdx++
		}

		// Scroll indicators.
		if ed.netDetailScroll > 0 {
			ed.drawString(cx+cw-3, cy+5, " ^ ", styleOverlayHdr)
		}
		if ed.netDetailScroll+listH < len(rows) {
			ed.drawString(cx+cw-3, cy+5+listH-1, " v ", styleOverlayHdr)
		}
	}

	// Footnotes.
	if len(footnotes) > 0 {
		// Find space after connection rows.
		footY := cy + ch - helpH - footH - 1
		if footY <= y {
			footY = y + 1
		}

		// Separator.
		for x := cx; x < cx+cw; x++ {
			ed.screen.SetContent(x, footY, '─', nil, styleOverlayBrd)
		}
		footY++

		for _, fn := range footnotes {
			if footY >= cy+ch-helpH-1 {
				break
			}
			ed.drawString(cx, footY, truncate(fn.Text, cw), styleOverlayDim)
			footY++
		}
	}

	// Help bar.
	helpY := cy + ch - 1
	help := "[A]dd  [D]elete  [R]ename net  [Esc] Close"
	if len(rows) == 0 {
		help = "[A]dd connection  [Esc] Close"
	}
	ed.drawString(cx, helpY, help, styleOverlayDim)
}

// --- Peer picker draw ---

func (ed *Editor) drawNetDetailPeerPicker(w, h int) {
	peers := ed.netDetailPeers
	stateA := ed.netDetailPeerStateA

	boxW := 40
	if boxW > w-4 {
		boxW = w - 4
	}
	boxH := len(peers) + 6
	if boxH > h-4 {
		boxH = h - 4
	}

	v := ed.Vocab()
	cx, cy, cw, ch := ed.drawOverlayBox(stateA+" — "+v.Transition+"s", boxW, boxH, w, h)
	_ = ch

	y := cy + 1
	ed.drawString(cx, y, "Select "+strings.ToLower(v.State)+":", styleOverlay)
	y += 2

	for i, peer := range peers {
		if i >= boxH-5 {
			break
		}
		s := styleOverlay
		if i == ed.netDetailPeerCursor {
			s = styleOverlayHl
		}

		label := peer
		// Count signal nets between A and this peer.
		netCount := 0
		if stateA == peer {
			// Self-connections.
			for _, n := range ed.fsm.Nets {
				if ed.fsm.IsPowerNet(n) {
					continue
				}
				count := 0
				for _, ep := range n.Endpoints {
					if ep.Instance == peer {
						count++
					}
				}
				if count >= 2 {
					netCount++
				}
			}
			label += " (self)"
		} else {
			nets := ed.fsm.SignalNetsBetween(stateA, peer)
			netCount = len(nets)
		}
		label += fmt.Sprintf("  [%d nets]", netCount)

		ed.drawString(cx, y, truncate(label, cw), s)
		y++
	}

	helpY := cy + boxH - 3
	ed.drawString(cx, helpY, "Enter=Open  Esc=Cancel", styleOverlayDim)
}

// --- Handler: connection detail ---

func (ed *Editor) handleNetDetailKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		ed.mode = ModeCanvas
		return false
	case tcell.KeyUp:
		if ed.netDetailSelected > 0 {
			ed.netDetailSelected--
		}
		return false
	case tcell.KeyDown:
		if ed.netDetailSelected < len(ed.netDetailRows)-1 {
			ed.netDetailSelected++
		}
		return false
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'a', 'A':
			ed.netDetailAddConnection()
			return false
		case 'd', 'D':
			ed.netDetailDeleteConnection()
			return false
		case 'r', 'R':
			ed.netDetailRenameNet()
			return false
		case 'q', 'Q':
			ed.mode = ModeCanvas
			return false
		}
	}
	return false
}

// --- Handler: peer picker ---

func (ed *Editor) handleNetDetailPeerKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		ed.mode = ModeCanvas
		return false
	case tcell.KeyUp:
		if ed.netDetailPeerCursor > 0 {
			ed.netDetailPeerCursor--
		}
		return false
	case tcell.KeyDown:
		if ed.netDetailPeerCursor < len(ed.netDetailPeers)-1 {
			ed.netDetailPeerCursor++
		}
		return false
	case tcell.KeyEnter:
		if ed.netDetailPeerCursor >= 0 && ed.netDetailPeerCursor < len(ed.netDetailPeers) {
			ed.netDetailOpen(ed.netDetailPeerStateA, ed.netDetailPeers[ed.netDetailPeerCursor])
		}
		return false
	}
	return false
}

// --- Controls: add, delete, rename ---

func (ed *Editor) netDetailAddConnection() {
	stateA := ed.netDetailStateA
	stateB := ed.netDetailStateB

	portsA := ed.fsm.EffectivePorts(stateA)
	portsB := ed.fsm.EffectivePorts(stateB)

	if len(portsA) == 0 || len(portsB) == 0 {
		ed.showMessage("Both components need ports to add connections", MsgError)
		return
	}

	// Collect available signal ports on A.
	var sigPortsA []string
	for _, p := range portsA {
		if p.Direction != fsm.PortPower {
			sigPortsA = append(sigPortsA, p.Name)
		}
	}
	var sigPortsB []string
	for _, p := range portsB {
		if p.Direction != fsm.PortPower {
			sigPortsB = append(sigPortsB, p.Name)
		}
	}

	if len(sigPortsA) == 0 || len(sigPortsB) == 0 {
		ed.showMessage("No signal ports available", MsgError)
		return
	}

	// Use chained input prompts: port on A, port on B, net name.
	ed.inputPrompt = fmt.Sprintf("Port on %s (%s): ", stateA, strings.Join(sigPortsA, "/"))
	ed.inputBuffer = sigPortsA[0]
	ed.inputAction = func(portA string) {
		// Validate port A.
		found := false
		for _, p := range sigPortsA {
			if p == portA {
				found = true
				break
			}
		}
		if !found {
			ed.showMessage("Unknown port: "+portA, MsgError)
			ed.mode = ModeNetDetail
			return
		}

		ed.inputPrompt = fmt.Sprintf("Port on %s (%s): ", stateB, strings.Join(sigPortsB, "/"))
		ed.inputBuffer = sigPortsB[0]
		ed.inputAction = func(portB string) {
			// Validate port B.
			found := false
			for _, p := range sigPortsB {
				if p == portB {
					found = true
					break
				}
			}
			if !found {
				ed.showMessage("Unknown port: "+portB, MsgError)
				ed.mode = ModeNetDetail
				return
			}

			// Prompt for net name.
			defaultNet := fmt.Sprintf("N%d", len(ed.fsm.Nets)+1)
			ed.inputPrompt = "Net name: "
			ed.inputBuffer = defaultNet
			ed.inputAction = func(netName string) {
				if netName == "" {
					ed.mode = ModeNetDetail
					return
				}

				ed.saveSnapshot()

				// Check if net already exists — if so, add endpoint(s) to it.
				existing := ed.fsm.GetNet(netName)
				if existing != nil {
					// Add new endpoints to existing net.
					if !existing.HasEndpoint(stateA, portA) {
						existing.Endpoints = append(existing.Endpoints, fsm.NetEndpoint{
							Instance: stateA, Port: portA,
						})
					}
					if !existing.HasEndpoint(stateB, portB) {
						existing.Endpoints = append(existing.Endpoints, fsm.NetEndpoint{
							Instance: stateB, Port: portB,
						})
					}
				} else {
					// Create new net.
					ed.fsm.Nets = append(ed.fsm.Nets, fsm.Net{
						Name: netName,
						Endpoints: []fsm.NetEndpoint{
							{Instance: stateA, Port: portA},
							{Instance: stateB, Port: portB},
						},
					})
				}

				ed.modified = true
				ed.netDetailBuildRows()
				ed.showMessage(fmt.Sprintf("Added: %s.%s -- %s -- %s.%s", stateA, portA, netName, stateB, portB), MsgSuccess)
				ed.mode = ModeNetDetail
			}
			ed.mode = ModeInput
		}
		ed.mode = ModeInput
	}
	ed.mode = ModeInput
}

func (ed *Editor) netDetailDeleteConnection() {
	if len(ed.netDetailRows) == 0 {
		ed.showMessage("No connections to delete", MsgInfo)
		return
	}
	if ed.netDetailSelected < 0 || ed.netDetailSelected >= len(ed.netDetailRows) {
		return
	}

	row := ed.netDetailRows[ed.netDetailSelected]
	stateA := ed.netDetailStateA
	stateB := ed.netDetailStateB

	ed.inputPrompt = fmt.Sprintf("Delete %s.%s-%s-%s.%s? (y/n): ", stateA, row.PortA, row.Net, stateB, row.PortB)
	ed.inputBuffer = ""
	ed.inputAction = func(answer string) {
		if strings.ToLower(answer) != "y" {
			ed.mode = ModeNetDetail
			return
		}

		ed.saveSnapshot()

		net := ed.fsm.GetNet(row.Net)
		if net == nil {
			ed.showMessage("Net not found: "+row.Net, MsgError)
			ed.mode = ModeNetDetail
			return
		}

		// Remove the specific endpoint pair.
		// Remove endpoint on A side for this specific port.
		newEps := make([]fsm.NetEndpoint, 0, len(net.Endpoints))
		removedA := false
		removedB := false
		for _, ep := range net.Endpoints {
			skip := false
			if !removedA && ep.Instance == stateA && ep.Port == row.PortA {
				skip = true
				removedA = true
			}
			if !removedB && ep.Instance == stateB && ep.Port == row.PortB {
				skip = true
				removedB = true
			}
			if !skip {
				newEps = append(newEps, ep)
			}
		}
		net.Endpoints = newEps

		// Remove net if it fell below 2 endpoints.
		if !net.IsValid() {
			ed.fsm.RemoveNet(row.Net)
			ed.showMessage("Deleted net: "+row.Net, MsgSuccess)
		} else {
			ed.showMessage("Removed endpoints from "+row.Net, MsgSuccess)
		}

		ed.modified = true
		ed.netDetailBuildRows()

		// Adjust selection.
		if ed.netDetailSelected >= len(ed.netDetailRows) {
			ed.netDetailSelected = len(ed.netDetailRows) - 1
		}
		if ed.netDetailSelected < 0 {
			ed.netDetailSelected = 0
		}

		ed.mode = ModeNetDetail
	}
	ed.mode = ModeInput
}

func (ed *Editor) netDetailRenameNet() {
	if len(ed.netDetailRows) == 0 {
		ed.showMessage("No connections to rename", MsgInfo)
		return
	}
	if ed.netDetailSelected < 0 || ed.netDetailSelected >= len(ed.netDetailRows) {
		return
	}

	row := ed.netDetailRows[ed.netDetailSelected]
	oldName := row.Net

	ed.inputPrompt = fmt.Sprintf("Rename net %q to: ", oldName)
	ed.inputBuffer = oldName
	ed.inputAction = func(newName string) {
		if newName == "" || newName == oldName {
			ed.mode = ModeNetDetail
			return
		}

		ed.saveSnapshot()

		if err := ed.fsm.RenameNet(oldName, newName); err != nil {
			ed.showMessage(err.Error(), MsgError)
			ed.mode = ModeNetDetail
			return
		}

		ed.modified = true
		ed.netDetailBuildRows()
		ed.showMessage(fmt.Sprintf("Renamed: %s -> %s", oldName, newName), MsgSuccess)
		ed.mode = ModeNetDetail
	}
	ed.mode = ModeInput
}
