package fsmfile

import (
	"sort"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// SmartLayoutTUI generates positions for FSM states optimised for the TUI editor.
//
// The layout uses a cell-grid model:
//  1. Sugiyama assigns layers (rows) and left-to-right ordering within rows.
//  2. The canvas is divided into a grid whose cell size is derived from the
//     widest state, the longest transition label, and self-loop/annotation margins.
//  3. Each state occupies one or more columns depending on its connection degree:
//     states with many arcs get extra blank columns for routing space.
//  4. States are centred within their column span.
//
// Returned positions are left-edge character-cell coordinates matching the
// TUI draw origin (state box starts at pos.X).
func SmartLayoutTUI(f *fsm.FSM, width, height int) map[string][2]int {
	n := len(f.States)
	if n == 0 {
		return make(map[string][2]int)
	}

	metrics := ComputeNodeMetrics(f)

	// --- Recover layer structure from Sugiyama ---

	basePositions := SmartLayout(f, width, height)

	type stateInfo struct {
		name string
		x    int
	}
	rowMap := make(map[int][]stateInfo)
	for name, pos := range basePositions {
		rowMap[pos[1]] = append(rowMap[pos[1]], stateInfo{name, pos[0]})
	}

	rowYs := make([]int, 0, len(rowMap))
	for y := range rowMap {
		rowYs = append(rowYs, y)
	}
	sort.Ints(rowYs)

	layers := make([][]string, 0, len(rowYs))
	for _, y := range rowYs {
		row := rowMap[y]
		sort.Slice(row, func(i, j int) bool {
			return row[i].x < row[j].x
		})
		names := make([]string, len(row))
		for i, s := range row {
			names[i] = s.name
		}
		layers = append(layers, names)
	}

	// --- Compute cell dimensions ---

	cellW, cellH := computeCellSize(f, metrics)

	// --- Compute per-state column span ---

	degree := computeDegrees(f)
	colSpan := make(map[string]int, n)
	for _, name := range f.States {
		colSpan[name] = columnSpan(degree[name])
	}

	// --- Compute vertical positions ---

	layerY := computeCellLayerY(layers, metrics, cellH)

	// --- Assign column positions per layer ---

	positions := make(map[string][2]int, n)

	for layerIdx, layer := range layers {
		// Compute total columns consumed by this layer.
		totalCols := 0
		for _, name := range layer {
			totalCols += colSpan[name]
		}

		// Total pixel width of this layer.
		layerPixelW := totalCols * cellW

		// Centre the layer on the canvas.
		originX := (width - layerPixelW) / 2
		if originX < 2 {
			originX = 2
		}

		// Place each state centred within its column span.
		col := 0
		for _, name := range layer {
			span := colSpan[name]
			nw := nodeW(name, metrics)

			// Left edge of the span.
			spanLeft := originX + col*cellW
			// Centre the state box within the span.
			x := spanLeft + (span*cellW-nw)/2
			if x < 2 {
				x = 2
			}

			y := layerY[layerIdx]

			// Clamp for self-loop headroom.
			if m, ok := metrics[name]; ok && m.TopMargin > 0 {
				if y < m.TopMargin+1 {
					y = m.TopMargin + 1
				}
			}
			// Clamp for annotation foot room.
			if m, ok := metrics[name]; ok && m.BottomMargin > 0 {
				if y > height-2-m.BottomMargin {
					y = height - 2 - m.BottomMargin
				}
			}

			positions[name] = [2]int{x, y}
			col += span
		}
	}

	// --- Median alignment pass ---
	// Nudge states towards the centre of their connected neighbours,
	// staying within cell boundaries to preserve routing space.

	adj := buildAdjacency(f)
	revAdj := buildReverseAdjacency(f)

	for pass := 0; pass < 2; pass++ {
		// Forward: align to predecessors.
		for layerIdx := 1; layerIdx < len(layers); layerIdx++ {
			nudgeLayer(layers[layerIdx], positions, revAdj, metrics, colSpan, cellW, width)
		}
		// Backward: align to successors.
		for layerIdx := len(layers) - 2; layerIdx >= 0; layerIdx-- {
			nudgeLayer(layers[layerIdx], positions, adj, metrics, colSpan, cellW, width)
		}
	}

	return positions
}

// computeCellSize determines cell width and height from the FSM geometry.
//
// Cell width must accommodate:
//   - The widest state box
//   - Room for arc labels between states
//   - A minimum gap so arcs don't overlap state text
//
// Cell height must accommodate:
//   - Self-loop top margin (3 rows)
//   - Annotation bottom margin (1 row)
//   - A base vertical gap for horizontal arcs between layers
func computeCellSize(f *fsm.FSM, metrics map[string]NodeMetrics) (cellW, cellH int) {
	maxW := 0
	maxTop := 0
	maxBottom := 0

	for _, name := range f.States {
		if m, ok := metrics[name]; ok {
			if m.Width > maxW {
				maxW = m.Width
			}
			if m.TopMargin > maxTop {
				maxTop = m.TopMargin
			}
			if m.BottomMargin > maxBottom {
				maxBottom = m.BottomMargin
			}
		}
	}

	// Horizontal: state width + label room + routing gap.
	labelW := MaxTransitionLabelWidth(f)
	// The gap between states in adjacent cells must fit an arc label.
	// Cell width = node width + padding.  The padding between the right
	// edge of one state and the left edge of the next equals
	// cellW - maxW, which should be >= labelW + 4.
	padding := labelW + 4
	if padding < 8 {
		padding = 8
	}
	cellW = maxW + padding

	// Vertical.
	cellH = 1 + maxTop + maxBottom + 3 // state row + margins + base gap
	if cellH < 4 {
		cellH = 4
	}

	return cellW, cellH
}

// degreeInfo holds connection counts for a state.
type degreeInfo struct {
	In, Out, Self int
}

// computeDegrees returns per-state connection counts.
func computeDegrees(f *fsm.FSM) map[string]degreeInfo {
	deg := make(map[string]degreeInfo, len(f.States))
	for _, name := range f.States {
		deg[name] = degreeInfo{}
	}
	for _, t := range f.Transitions {
		for _, to := range t.To {
			if t.From == to {
				d := deg[t.From]
				d.Self++
				deg[t.From] = d
			} else {
				dFrom := deg[t.From]
				dFrom.Out++
				deg[t.From] = dFrom
				dTo := deg[to]
				dTo.In++
				deg[to] = dTo
			}
		}
	}
	return deg
}

// columnSpan determines how many grid columns a state should occupy.
// Most states occupy exactly one column. States with many arcs on their
// busiest side (in or out) get one additional blank routing column so
// the fan of L-shaped arcs has space to spread without overlapping.
//
// The threshold is conservative: only states with 4+ arcs on one side
// get the extra column. Below that, the cell padding (which already
// includes room for arc labels) provides sufficient routing space.
func columnSpan(d degreeInfo) int {
	busiest := d.In
	if d.Out > busiest {
		busiest = d.Out
	}
	if busiest >= 4 {
		return 2 // 1 state column + 1 blank routing column
	}
	return 1
}

// computeCellLayerY assigns Y coordinates to layers using cell height,
// with per-layer adjustments for margin needs.
func computeCellLayerY(layers [][]string, metrics map[string]NodeMetrics, cellH int) []int {
	if len(layers) == 0 {
		return nil
	}

	layerY := make([]int, len(layers))

	// First layer clears any top margins.
	firstTop := 2
	for _, name := range layers[0] {
		if m, ok := metrics[name]; ok && m.TopMargin+1 > firstTop {
			firstTop = m.TopMargin + 1
		}
	}
	layerY[0] = firstTop

	for i := 1; i < len(layers); i++ {
		// Use the larger of the fixed cell height and the metric-derived spacing.
		var upperM, lowerM []NodeMetrics
		for _, name := range layers[i-1] {
			if m, ok := metrics[name]; ok {
				upperM = append(upperM, m)
			}
		}
		for _, name := range layers[i] {
			if m, ok := metrics[name]; ok {
				lowerM = append(lowerM, m)
			}
		}
		metricSpacing := MinLayerSpacing(upperM, lowerM, 3)
		spacing := cellH
		if metricSpacing > spacing {
			spacing = metricSpacing
		}
		layerY[i] = layerY[i-1] + spacing
	}

	return layerY
}

// nodeW returns the character-cell width of a state.
func nodeW(name string, metrics map[string]NodeMetrics) int {
	if m, ok := metrics[name]; ok {
		return m.Width
	}
	return len(name) + 4
}

// buildReverseAdjacency builds an incoming-edge adjacency list.
func buildReverseAdjacency(f *fsm.FSM) map[string][]string {
	rev := make(map[string][]string, len(f.States))
	for _, name := range f.States {
		rev[name] = []string{}
	}
	for _, t := range f.Transitions {
		for _, to := range t.To {
			if t.From != to {
				rev[to] = append(rev[to], t.From)
			}
		}
	}
	return rev
}

// nudgeLayer adjusts X positions within a layer to align states with their
// connected neighbours in an adjacent layer, while respecting cell boundaries.
func nudgeLayer(layer []string, positions map[string][2]int, adj map[string][]string, metrics map[string]NodeMetrics, colSpan map[string]int, cellW, canvasW int) {
	if len(layer) <= 1 {
		return
	}

	// Compute cell boundaries for this layer so we know valid ranges.
	totalCols := 0
	for _, name := range layer {
		totalCols += colSpan[name]
	}
	layerPixelW := totalCols * cellW
	originX := (canvasW - layerPixelW) / 2
	if originX < 2 {
		originX = 2
	}

	// For each state, compute the ideal X (median of neighbours) and
	// the allowed range (within its cell span).
	col := 0
	for _, name := range layer {
		span := colSpan[name]
		nw := nodeW(name, metrics)

		// Allowed range: centred within span ± half the padding.
		spanLeft := originX + col*cellW
		spanRight := spanLeft + span*cellW
		minX := spanLeft + 1
		maxX := spanRight - nw - 1
		if maxX < minX {
			maxX = minX
		}

		// Find median X of neighbours.
		neighbours := adj[name]
		if len(neighbours) > 0 {
			centres := make([]int, 0, len(neighbours))
			for _, nb := range neighbours {
				if p, ok := positions[nb]; ok {
					centres = append(centres, p[0]+nodeW(nb, metrics)/2)
				}
			}
			if len(centres) > 0 {
				sort.Ints(centres)
				median := centres[len(centres)/2]
				// Target: place our centre at the median.
				idealX := median - nw/2

				// Clamp to cell bounds.
				if idealX < minX {
					idealX = minX
				}
				if idealX > maxX {
					idealX = maxX
				}

				// Blend: move 40% towards ideal to avoid oscillation.
				current := positions[name][0]
				newX := current + (idealX-current)*4/10
				if newX < minX {
					newX = minX
				}
				if newX > maxX {
					newX = maxX
				}

				positions[name] = [2]int{newX, positions[name][1]}
			}
		}

		col += span
	}
}
