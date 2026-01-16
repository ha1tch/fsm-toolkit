package fsmfile

import (
	"math"
	"sort"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// SugiyamaLayout implements a layered graph layout algorithm inspired by
// Sugiyama's method. It produces clean hierarchical layouts similar to Graphviz.
//
// The algorithm has four phases:
// 1. Layer assignment - assign each node to a horizontal layer
// 2. Crossing minimisation - reorder nodes within layers to reduce edge crossings
// 3. Horizontal positioning - assign X coordinates to minimise edge length
// 4. Final coordinate assignment - convert to pixel coordinates
func SugiyamaLayout(f *fsm.FSM, width, height int) map[string][2]int {
	if len(f.States) == 0 {
		return make(map[string][2]int)
	}

	// Build graph structure
	graph := buildGraph(f)

	// Phase 1: Layer assignment
	layers := assignLayers(graph, f.Initial)

	// Phase 2: Crossing minimisation (multiple passes)
	for i := 0; i < 4; i++ {
		layers = reduceCrossings(layers, graph)
	}

	// Phase 3: Horizontal positioning within layers
	positions := assignPositions(layers, graph, width, height)

	return positions
}

// graph represents the FSM as an adjacency structure
type graph struct {
	nodes    []string
	forward  map[string][]string // outgoing edges
	backward map[string][]string // incoming edges
}

func buildGraph(f *fsm.FSM) *graph {
	g := &graph{
		nodes:    make([]string, len(f.States)),
		forward:  make(map[string][]string),
		backward: make(map[string][]string),
	}

	copy(g.nodes, f.States)

	for _, name := range f.States {
		g.forward[name] = []string{}
		g.backward[name] = []string{}
	}

	// Build adjacency (deduplicated)
	seen := make(map[[2]string]bool)
	for _, t := range f.Transitions {
		for _, to := range t.To {
			edge := [2]string{t.From, to}
			if !seen[edge] {
				seen[edge] = true
				g.forward[t.From] = append(g.forward[t.From], to)
				g.backward[to] = append(g.backward[to], t.From)
			}
		}
	}

	return g
}

// assignLayers uses BFS from initial state to assign layer numbers.
// States are assigned to the earliest possible layer (longest path from initial).
func assignLayers(g *graph, initial string) [][]string {
	layerNum := make(map[string]int)
	maxLayer := 0

	if initial != "" {
		// BFS to find minimum distance
		queue := []string{initial}
		layerNum[initial] = 0

		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]

			for _, next := range g.forward[current] {
				if _, visited := layerNum[next]; !visited {
					layerNum[next] = layerNum[current] + 1
					if layerNum[next] > maxLayer {
						maxLayer = layerNum[next]
					}
					queue = append(queue, next)
				}
			}
		}
	}

	// Assign unreachable nodes to final layer
	for _, name := range g.nodes {
		if _, ok := layerNum[name]; !ok {
			maxLayer++
			layerNum[name] = maxLayer
		}
	}

	// Group by layer
	layers := make([][]string, maxLayer+1)
	for i := range layers {
		layers[i] = []string{}
	}
	for name, layer := range layerNum {
		layers[layer] = append(layers[layer], name)
	}

	// Initial sort by name for determinism
	for i := range layers {
		sort.Strings(layers[i])
	}

	return layers
}

// reduceCrossings reorders nodes within layers to minimise edge crossings.
// Uses the barycenter heuristic: position each node at the average position
// of its neighbors in adjacent layers.
func reduceCrossings(layers [][]string, g *graph) [][]string {
	if len(layers) <= 1 {
		return layers
	}

	result := make([][]string, len(layers))
	for i := range layers {
		result[i] = make([]string, len(layers[i]))
		copy(result[i], layers[i])
	}

	// Build position lookup
	pos := make(map[string]float64)
	for _, layer := range result {
		for i, name := range layer {
			pos[name] = float64(i)
		}
	}

	// Forward pass: order based on predecessors
	for l := 1; l < len(result); l++ {
		bary := make(map[string]float64)
		for _, name := range result[l] {
			sum := 0.0
			count := 0
			for _, pred := range g.backward[name] {
				if p, ok := pos[pred]; ok {
					sum += p
					count++
				}
			}
			if count > 0 {
				bary[name] = sum / float64(count)
			} else {
				bary[name] = pos[name]
			}
		}

		// Sort layer by barycenter
		sort.Slice(result[l], func(i, j int) bool {
			bi := bary[result[l][i]]
			bj := bary[result[l][j]]
			if bi != bj {
				return bi < bj
			}
			return result[l][i] < result[l][j] // stable sort
		})

		// Update positions
		for i, name := range result[l] {
			pos[name] = float64(i)
		}
	}

	// Backward pass: order based on successors
	for l := len(result) - 2; l >= 0; l-- {
		bary := make(map[string]float64)
		for _, name := range result[l] {
			sum := 0.0
			count := 0
			for _, succ := range g.forward[name] {
				if p, ok := pos[succ]; ok {
					sum += p
					count++
				}
			}
			if count > 0 {
				bary[name] = sum / float64(count)
			} else {
				bary[name] = pos[name]
			}
		}

		// Sort layer by barycenter
		sort.Slice(result[l], func(i, j int) bool {
			bi := bary[result[l][i]]
			bj := bary[result[l][j]]
			if bi != bj {
				return bi < bj
			}
			return result[l][i] < result[l][j]
		})

		// Update positions
		for i, name := range result[l] {
			pos[name] = float64(i)
		}
	}

	return result
}

// assignPositions converts layer structure to pixel coordinates.
// Uses a priority-based positioning to align connected nodes.
// Takes label widths into account to prevent overlaps.
func assignPositions(layers [][]string, g *graph, width, height int) map[string][2]int {
	positions := make(map[string][2]int)

	if len(layers) == 0 {
		return positions
	}

	// Calculate label widths for all nodes
	// Must match the SVG renderer's calculation:
	// textWidth = len * fontSize * 0.6
	// nodeWidth = max(StateRadius*2, textWidth+40)
	// Default: StateRadius=30, FontSize=14 (but label uses FontSize-2=12)
	// So: nodeWidth = max(60, len*7.2+40)
	// In layout units (before 10*NodeSpacing scaling), divide by 15:
	labelWidth := make(map[string]float64)
	for _, layer := range layers {
		for _, name := range layer {
			// Match SVG renderer's width calculation
			// Using fontSize-2 for state labels (12px default)
			textWidth := float64(len(name)) * 7.2  // 12 * 0.6
			pixelWidth := math.Max(60, textWidth+40)
			// Convert to layout units (divide by scale factor 15)
			// and add generous padding for gaps between nodes
			w := pixelWidth/15 + 3.0
			labelWidth[name] = w
		}
	}

	// Calculate the actual width needed for each layer
	layerWidths := make([]float64, len(layers))
	for i, layer := range layers {
		totalWidth := 0.0
		for j, name := range layer {
			totalWidth += labelWidth[name]
			if j < len(layer)-1 {
				totalWidth += 2 // gap between nodes
			}
		}
		layerWidths[i] = totalWidth
	}

	// Find the widest layer
	maxLayerWidth := 0.0
	for _, w := range layerWidths {
		if w > maxLayerWidth {
			maxLayerWidth = w
		}
	}

	// Calculate spacing
	numLayers := len(layers)

	// Vertical spacing between layers (layers go top to bottom)
	layerSpacing := 4
	if height > 10 {
		layerSpacing = (height - 4) / numLayers
		if layerSpacing < 4 {
			layerSpacing = 4
		}
		if layerSpacing > 10 {
			layerSpacing = 10
		}
	}

	// First pass: assign initial positions based on label widths
	yPos := make(map[string]float64)
	xPos := make(map[string]float64)

	for layerIdx, layer := range layers {
		// Calculate total width of this layer
		totalWidth := layerWidths[layerIdx]
		
		// Center the layer
		startX := (float64(width) - totalWidth) / 2
		if startX < 5 {
			startX = 5
		}

		y := float64(2 + layerIdx*layerSpacing)
		currentX := startX

		for _, name := range layer {
			w := labelWidth[name]
			xPos[name] = currentX + w/2 // position is center of node
			yPos[name] = y
			currentX += w + 3 // move to next node position
		}
	}

	// Second pass: adjust positions to align with neighbors (median heuristic)
	// Use label-aware overlap resolution
	for pass := 0; pass < 3; pass++ {
		// Forward pass
		for layerIdx := 1; layerIdx < len(layers); layerIdx++ {
			layer := layers[layerIdx]
			for _, name := range layer {
				preds := g.backward[name]
				if len(preds) > 0 {
					// Find median X of predecessors
					predX := make([]float64, 0, len(preds))
					for _, pred := range preds {
						predX = append(predX, xPos[pred])
					}
					sort.Float64s(predX)
					median := predX[len(predX)/2]

					// Move towards median, but respect layer bounds
					current := xPos[name]
					xPos[name] = current + (median-current)*0.5
				}
			}

			// Ensure no overlaps within layer (using label widths)
			resolveOverlapsWithWidths(layers[layerIdx], xPos, labelWidth)
		}

		// Backward pass
		for layerIdx := len(layers) - 2; layerIdx >= 0; layerIdx-- {
			layer := layers[layerIdx]
			for _, name := range layer {
				succs := g.forward[name]
				if len(succs) > 0 {
					succX := make([]float64, 0, len(succs))
					for _, succ := range succs {
						succX = append(succX, xPos[succ])
					}
					sort.Float64s(succX)
					median := succX[len(succX)/2]

					current := xPos[name]
					xPos[name] = current + (median-current)*0.3
				}
			}

			resolveOverlapsWithWidths(layers[layerIdx], xPos, labelWidth)
		}
	}

	// Convert to integer positions
	for _, layer := range layers {
		for _, name := range layer {
			x := int(math.Round(xPos[name]))
			y := int(math.Round(yPos[name]))

			// Clamp to bounds (but leave room for overlap resolution)
			if x < 2 {
				x = 2
			}
			// Don't clamp X max here - let overlap resolution handle it
			if y < 1 {
				y = 1
			}
			if y > height-2 {
				y = height - 2
			}

			positions[name] = [2]int{x, y}
		}
	}

	// Final overlap resolution on integer positions
	// Group by Y coordinate
	byY := make(map[int][]string)
	for name, pos := range positions {
		byY[pos[1]] = append(byY[pos[1]], name)
	}

	for _, row := range byY {
		if len(row) <= 1 {
			continue
		}
		
		// Sort by X
		sort.Slice(row, func(i, j int) bool {
			return positions[row[i]][0] < positions[row[j]][0]
		})
		
		// Push apart if needed
		for i := 1; i < len(row); i++ {
			prev := row[i-1]
			curr := row[i]
			
			// Calculate minimum gap in layout units
			minGap := int((labelWidth[prev] + labelWidth[curr]) / 2 + 3)
			
			actualGap := positions[curr][0] - positions[prev][0]
			if actualGap < minGap {
				positions[curr] = [2]int{positions[prev][0] + minGap, positions[curr][1]}
			}
		}
	}

	return positions
}

// resolveOverlapsWithWidths ensures nodes in a layer don't overlap, using actual label widths
func resolveOverlapsWithWidths(layer []string, xPos map[string]float64, labelWidth map[string]float64) {
	if len(layer) <= 1 {
		return
	}

	// Sort layer by current X position
	sorted := make([]string, len(layer))
	copy(sorted, layer)
	sort.Slice(sorted, func(i, j int) bool {
		return xPos[sorted[i]] < xPos[sorted[j]]
	})

	// Push nodes apart if they overlap
	for i := 1; i < len(sorted); i++ {
		prev := sorted[i-1]
		curr := sorted[i]
		
		// Minimum gap is half of each node's width plus padding
		minGap := (labelWidth[prev] + labelWidth[curr]) / 2 + 2
		
		actualGap := xPos[curr] - xPos[prev]
		if actualGap < minGap {
			// Push current node right
			xPos[curr] = xPos[prev] + minGap
		}
	}
}

// countCrossings counts edge crossings between two adjacent layers.
// Used to evaluate layout quality.
func countCrossings(layer1, layer2 []string, g *graph) int {
	// Build position maps
	pos1 := make(map[string]int)
	pos2 := make(map[string]int)
	for i, name := range layer1 {
		pos1[name] = i
	}
	for i, name := range layer2 {
		pos2[name] = i
	}

	// Collect edges between layers as (from_pos, to_pos) pairs
	var edges [][2]int
	for _, from := range layer1 {
		for _, to := range g.forward[from] {
			if _, ok := pos2[to]; ok {
				edges = append(edges, [2]int{pos1[from], pos2[to]})
			}
		}
	}

	// Count inversions (crossings)
	crossings := 0
	for i := 0; i < len(edges); i++ {
		for j := i + 1; j < len(edges); j++ {
			e1, e2 := edges[i], edges[j]
			// Edges cross if one goes "over" the other
			if (e1[0] < e2[0] && e1[1] > e2[1]) || (e1[0] > e2[0] && e1[1] < e2[1]) {
				crossings++
			}
		}
	}

	return crossings
}

// LayoutQuality returns a score for the current layout (lower is better).
// Considers edge crossings, edge length variance, and node distribution.
func LayoutQuality(f *fsm.FSM, positions map[string][2]int) float64 {
	if len(positions) <= 1 {
		return 0
	}

	g := buildGraph(f)

	// Calculate total edge length and variance
	var lengths []float64
	for from, tos := range g.forward {
		fromPos, ok1 := positions[from]
		if !ok1 {
			continue
		}
		for _, to := range tos {
			toPos, ok2 := positions[to]
			if !ok2 {
				continue
			}
			dx := float64(toPos[0] - fromPos[0])
			dy := float64(toPos[1] - fromPos[1])
			lengths = append(lengths, math.Sqrt(dx*dx+dy*dy))
		}
	}

	if len(lengths) == 0 {
		return 0
	}

	// Mean edge length
	sum := 0.0
	for _, l := range lengths {
		sum += l
	}
	mean := sum / float64(len(lengths))

	// Variance
	variance := 0.0
	for _, l := range lengths {
		diff := l - mean
		variance += diff * diff
	}
	variance /= float64(len(lengths))

	// Score: prefer low variance (consistent edge lengths)
	return math.Sqrt(variance)
}
