package fsmfile

import (
	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// NodeMetrics holds the visual footprint of a state node in character cells.
// All measurements are in terminal character units.
type NodeMetrics struct {
	// Width is the total horizontal span of the state and its decorations.
	Width int

	// TopMargin is the number of rows needed above the anchor point.
	// Self-loops draw 3 rows above the state.
	TopMargin int

	// BottomMargin is the number of rows needed below the anchor point.
	// Linked targets and Moore outputs draw 1 row below.
	BottomMargin int

	// SelfLoopCount is the number of self-loop transitions on this state.
	SelfLoopCount int
}

// ComputeNodeMetrics calculates the visual footprint of each state in an FSM.
// This accounts for:
//   - State label length (prefix[name]suffix)
//   - Self-loops (3 rows above + label extending right)
//   - Linked machine annotations (1 row below)
//   - Moore outputs (1 row below)
//   - Accepting/initial markers
func ComputeNodeMetrics(f *fsm.FSM) map[string]NodeMetrics {
	metrics := make(map[string]NodeMetrics, len(f.States))

	// Count self-loops per state and find longest self-loop label
	selfLoopCount := make(map[string]int)
	selfLoopMaxLabel := make(map[string]int)
	for _, t := range f.Transitions {
		for _, to := range t.To {
			if t.From == to {
				selfLoopCount[t.From]++
				lbl := selfLoopLabelLen(t, f.Type)
				if lbl > selfLoopMaxLabel[t.From] {
					selfLoopMaxLabel[t.From] = lbl
				}
			}
		}
	}

	for _, name := range f.States {
		m := NodeMetrics{}

		// Base width: "○[name]" or "→[name]*" or "○[name]↗"
		// prefix(1) + '[' (1) + name + ']'(1) + suffix(0 or 1)
		baseW := len(name) + 3
		hasAccepting := f.IsAccepting(name)
		isLinked := f.IsLinked(name)

		if hasAccepting || isLinked {
			baseW = len(name) + 4 // suffix adds 1
		}

		m.Width = baseW

		// Linked machine annotation: "  →targetMachine" drawn at x+2, y+1
		if isLinked {
			target := f.GetLinkedMachine(name)
			if target != "" {
				annotW := 2 + 1 + len(target) // indent + arrow + name
				if annotW > m.Width {
					m.Width = annotW
				}
				m.BottomMargin = 1
			}
		}

		// Moore output annotation: "  /output" drawn at x+2, y+1
		if f.Type == fsm.TypeMoore {
			if out, ok := f.StateOutputs[name]; ok && out != "" {
				annotW := 2 + 1 + len(out) // indent + slash + output
				if annotW > m.Width {
					m.Width = annotW
				}
				m.BottomMargin = 1
			}
		}

		// Self-loop: draws 3 rows above, label extends to the right.
		// Loop body is 4 chars wide at center of state.
		// Label starts at center + 5.
		if count := selfLoopCount[name]; count > 0 {
			m.SelfLoopCount = count
			m.TopMargin = 3

			// Self-loop label extends right from state center:
			// center = len(name)/2 + 2, label at center + 5
			// Total right extent from state X = len(name)/2 + 2 + 5 + labelLen
			maxLbl := selfLoopMaxLabel[name]
			loopRightExtent := len(name)/2 + 7 + maxLbl
			if loopRightExtent > m.Width {
				m.Width = loopRightExtent
			}
		}

		metrics[name] = m
	}

	return metrics
}

// selfLoopLabelLen returns the display length of a transition label.
func selfLoopLabelLen(t fsm.Transition, fsmType fsm.Type) int {
	n := 0
	if t.Input != nil {
		n = len(*t.Input)
	} else {
		n = 1 // "ε"
	}
	if fsmType == fsm.TypeMealy && t.Output != nil {
		n += 1 + len(*t.Output) // "/output"
	}
	return n
}

// MinLayerSpacing calculates the minimum vertical spacing between two
// adjacent layers given the metrics of nodes in each layer.
// It ensures the bottom margin of the upper layer plus the top margin
// of the lower layer plus a base gap all fit.
func MinLayerSpacing(upperMetrics, lowerMetrics []NodeMetrics, baseGap int) int {
	maxBottom := 0
	for _, m := range upperMetrics {
		if m.BottomMargin > maxBottom {
			maxBottom = m.BottomMargin
		}
	}

	maxTop := 0
	for _, m := range lowerMetrics {
		if m.TopMargin > maxTop {
			maxTop = m.TopMargin
		}
	}

	spacing := maxBottom + maxTop + baseGap
	if spacing < baseGap {
		spacing = baseGap
	}
	return spacing
}

// MaxTransitionLabelWidth returns the length of the longest transition
// label in the FSM. For Mealy machines this includes "input/output".
func MaxTransitionLabelWidth(f *fsm.FSM) int {
	maxLen := 0
	for _, t := range f.Transitions {
		n := 0
		if t.Input != nil {
			n = len(*t.Input)
		} else {
			n = 1 // "ε"
		}
		if f.Type == fsm.TypeMealy && t.Output != nil {
			n += 1 + len(*t.Output) // "/output"
		}
		if n > maxLen {
			maxLen = n
		}
	}
	return maxLen
}
