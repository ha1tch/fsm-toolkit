package fsmfile

import (
	"math"
	"sort"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// LayoutAlgorithm represents a layout strategy.
type LayoutAlgorithm int

const (
	LayoutGrid LayoutAlgorithm = iota
	LayoutCircular
	LayoutHierarchical
	LayoutForceDirected
)

// AutoLayout generates positions for FSM states.
// Returns map of state name to [x, y] coordinates.
func AutoLayout(f *fsm.FSM, algorithm LayoutAlgorithm, width, height int) map[string][2]int {
	var positions map[string][2]int
	
	switch algorithm {
	case LayoutCircular:
		positions = layoutCircular(f, width, height)
	case LayoutHierarchical:
		positions = layoutHierarchical(f, width, height)
	case LayoutForceDirected:
		positions = layoutForceDirected(f, width, height)
	default:
		positions = layoutGrid(f, width, height)
	}
	
	// Estimate label width
	maxLabelWidth := 8
	for _, name := range f.States {
		if len(name) > maxLabelWidth {
			maxLabelWidth = len(name)
		}
	}
	
	// Resolve collisions
	positions = resolveCollisions(positions, maxLabelWidth)
	
	// Clamp to bounds
	positions = clampToBounds(positions, width, height, maxLabelWidth)
	
	return positions
}

// SmartLayout chooses the best algorithm based on FSM structure.
func SmartLayout(f *fsm.FSM, width, height int) map[string][2]int {
	n := len(f.States)
	
	if n == 0 {
		return make(map[string][2]int)
	}
	
	// Analyse structure
	isLinear := isLinearChain(f)
	hasCyclic := hasCycles(f)
	density := float64(len(f.Transitions)) / float64(n*n)
	
	// Choose algorithm
	if isLinear || n <= 4 {
		// Linear chains and small FSMs: hierarchical works best
		return layoutHierarchical(f, width, height)
	}
	if n <= 8 && !hasCyclic {
		// Small acyclic graphs: hierarchical
		return layoutHierarchical(f, width, height)
	}
	if n <= 15 {
		// Medium FSMs: circular gives good visibility
		return layoutCircular(f, width, height)
	}
	if density > 0.25 {
		// Dense graphs: force directed spreads things out
		return layoutForceDirected(f, width, height)
	}
	
	// Large sparse graphs: hierarchical with layers
	return layoutHierarchical(f, width, height)
}

// layoutGrid arranges states in a simple grid pattern.
func layoutGrid(f *fsm.FSM, width, height int) map[string][2]int {
	positions := make(map[string][2]int)
	n := len(f.States)
	if n == 0 {
		return positions
	}
	
	// Calculate grid dimensions
	cols := int(math.Ceil(math.Sqrt(float64(n))))
	if cols < 1 {
		cols = 1
	}
	
	// Calculate spacing
	cellW := (width - 10) / cols
	if cellW < 15 {
		cellW = 15
	}
	cellH := 4
	
	for i, name := range f.States {
		col := i % cols
		row := i / cols
		positions[name] = [2]int{
			5 + col*cellW,
			2 + row*cellH,
		}
	}
	
	return positions
}

// layoutCircular arranges states in a circle with initial state at top.
func layoutCircular(f *fsm.FSM, width, height int) map[string][2]int {
	positions := make(map[string][2]int)
	n := len(f.States)
	if n == 0 {
		return positions
	}
	
	// Centre and radius
	centreX := width / 2
	centreY := height / 2
	
	// Radius based on available space
	radiusX := (width - 20) / 2
	radiusY := (height - 6) / 2
	if radiusX < 10 {
		radiusX = 10
	}
	if radiusY < 4 {
		radiusY = 4
	}
	
	// Order states: initial first, then by connectivity
	ordered := orderByConnectivity(f)
	
	for i, name := range ordered {
		// Angle: start from top (-π/2), go clockwise
		angle := -math.Pi/2 + 2*math.Pi*float64(i)/float64(n)
		
		x := centreX + int(float64(radiusX)*math.Cos(angle))
		y := centreY + int(float64(radiusY)*math.Sin(angle))
		
		positions[name] = [2]int{x, y}
	}
	
	return positions
}

// layoutHierarchical arranges states in layers based on distance from initial.
func layoutHierarchical(f *fsm.FSM, width, height int) map[string][2]int {
	positions := make(map[string][2]int)
	n := len(f.States)
	if n == 0 {
		return positions
	}
	
	// Build adjacency list
	adj := buildAdjacency(f)
	
	// BFS to find layers (distance from initial state)
	layers := make(map[string]int)
	maxLayer := 0
	
	if f.Initial != "" {
		queue := []string{f.Initial}
		layers[f.Initial] = 0
		
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			
			for _, next := range adj[current] {
				if _, visited := layers[next]; !visited {
					layers[next] = layers[current] + 1
					if layers[next] > maxLayer {
						maxLayer = layers[next]
					}
					queue = append(queue, next)
				}
			}
		}
	}
	
	// Add unreachable states to last layer
	for _, name := range f.States {
		if _, ok := layers[name]; !ok {
			maxLayer++
			layers[name] = maxLayer
		}
	}
	
	// Group states by layer
	layerGroups := make(map[int][]string)
	for name, layer := range layers {
		layerGroups[layer] = append(layerGroups[layer], name)
	}
	
	// Sort each layer for consistency
	for layer := range layerGroups {
		sort.Strings(layerGroups[layer])
	}
	
	// Find the widest layer (most states)
	maxStatesInLayer := 0
	for _, states := range layerGroups {
		if len(states) > maxStatesInLayer {
			maxStatesInLayer = len(states)
		}
	}
	
	// Calculate spacing
	numLayers := maxLayer + 1
	
	// Estimate max label width
	maxLabelWidth := 0
	for _, name := range f.States {
		if len(name)+4 > maxLabelWidth { // +4 for "[" "]" and padding
			maxLabelWidth = len(name) + 4
		}
	}
	if maxLabelWidth < 10 {
		maxLabelWidth = 10
	}
	
	// Horizontal spacing between layers
	layerSpacing := (width - 10) / numLayers
	if layerSpacing < maxLabelWidth+2 {
		layerSpacing = maxLabelWidth + 2
	}
	
	// Vertical spacing within layers
	rowSpacing := (height - 4) / maxStatesInLayer
	if rowSpacing < 3 {
		rowSpacing = 3
	}
	
	// Position states
	for layer := 0; layer <= maxLayer; layer++ {
		states := layerGroups[layer]
		numStates := len(states)
		
		// Centre the layer vertically
		startY := (height - numStates*rowSpacing) / 2
		if startY < 2 {
			startY = 2
		}
		
		for i, name := range states {
			x := 5 + layer*layerSpacing
			y := startY + i*rowSpacing
			positions[name] = [2]int{x, y}
		}
	}
	
	return positions
}

// layoutForceDirected uses a simple force-directed algorithm.
func layoutForceDirected(f *fsm.FSM, width, height int) map[string][2]int {
	positions := make(map[string][2]int)
	n := len(f.States)
	if n == 0 {
		return positions
	}
	
	// Initial positions: random-ish but deterministic
	posX := make(map[string]float64)
	posY := make(map[string]float64)
	
	for i, name := range f.States {
		// Start with circular layout
		angle := 2 * math.Pi * float64(i) / float64(n)
		posX[name] = float64(width)/2 + float64(width)/3*math.Cos(angle)
		posY[name] = float64(height)/2 + float64(height)/3*math.Sin(angle)
	}
	
	// Build edge set
	edges := make(map[[2]string]bool)
	for _, t := range f.Transitions {
		for _, to := range t.To {
			if t.From != to {
				edges[[2]string{t.From, to}] = true
			}
		}
	}
	
	// Force-directed iterations
	const iterations = 50
	const repulsion = 500.0
	const attraction = 0.1
	const damping = 0.85
	
	for iter := 0; iter < iterations; iter++ {
		// Calculate forces
		forceX := make(map[string]float64)
		forceY := make(map[string]float64)
		
		for _, name := range f.States {
			forceX[name] = 0
			forceY[name] = 0
		}
		
		// Repulsion between all pairs
		for i, a := range f.States {
			for j, b := range f.States {
				if i >= j {
					continue
				}
				
				dx := posX[a] - posX[b]
				dy := posY[a] - posY[b]
				dist := math.Sqrt(dx*dx + dy*dy)
				if dist < 1 {
					dist = 1
				}
				
				force := repulsion / (dist * dist)
				fx := force * dx / dist
				fy := force * dy / dist
				
				forceX[a] += fx
				forceY[a] += fy
				forceX[b] -= fx
				forceY[b] -= fy
			}
		}
		
		// Attraction along edges
		for edge := range edges {
			a, b := edge[0], edge[1]
			
			dx := posX[b] - posX[a]
			dy := posY[b] - posY[a]
			dist := math.Sqrt(dx*dx + dy*dy)
			
			force := attraction * dist
			fx := force * dx / dist
			fy := force * dy / dist
			
			forceX[a] += fx
			forceY[a] += fy
			forceX[b] -= fx
			forceY[b] -= fy
		}
		
		// Apply forces with damping
		for _, name := range f.States {
			posX[name] += forceX[name] * damping
			posY[name] += forceY[name] * damping
			
			// Keep within bounds
			if posX[name] < 5 {
				posX[name] = 5
			}
			if posX[name] > float64(width-15) {
				posX[name] = float64(width - 15)
			}
			if posY[name] < 2 {
				posY[name] = 2
			}
			if posY[name] > float64(height-2) {
				posY[name] = float64(height - 2)
			}
		}
	}
	
	// Convert to integer positions
	for name := range posX {
		positions[name] = [2]int{
			int(math.Round(posX[name])),
			int(math.Round(posY[name])),
		}
	}
	
	// Snap to grid to avoid half-character positions
	positions = snapToGrid(positions, 2, 1)
	
	return positions
}

// Helper functions

func buildAdjacency(f *fsm.FSM) map[string][]string {
	adj := make(map[string][]string)
	for _, name := range f.States {
		adj[name] = []string{}
	}
	for _, t := range f.Transitions {
		for _, to := range t.To {
			adj[t.From] = append(adj[t.From], to)
		}
	}
	return adj
}

func orderByConnectivity(f *fsm.FSM) []string {
	if len(f.States) == 0 {
		return nil
	}
	
	// Start with initial state
	result := make([]string, 0, len(f.States))
	visited := make(map[string]bool)
	
	adj := buildAdjacency(f)
	
	// BFS from initial
	if f.Initial != "" {
		queue := []string{f.Initial}
		visited[f.Initial] = true
		
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			result = append(result, current)
			
			for _, next := range adj[current] {
				if !visited[next] {
					visited[next] = true
					queue = append(queue, next)
				}
			}
		}
	}
	
	// Add remaining states
	for _, name := range f.States {
		if !visited[name] {
			result = append(result, name)
		}
	}
	
	return result
}

func isLinearChain(f *fsm.FSM) bool {
	if len(f.States) <= 2 {
		return true
	}
	
	// Count in-degree and out-degree
	inDegree := make(map[string]int)
	outDegree := make(map[string]int)
	
	for _, name := range f.States {
		inDegree[name] = 0
		outDegree[name] = 0
	}
	
	for _, t := range f.Transitions {
		for _, to := range t.To {
			if t.From != to { // ignore self-loops
				outDegree[t.From]++
				inDegree[to]++
			}
		}
	}
	
	// Linear chain: most nodes have in=1, out=1
	// Start has in=0, end has out=0
	startCount := 0
	endCount := 0
	middleCount := 0
	
	for _, name := range f.States {
		in := inDegree[name]
		out := outDegree[name]
		
		if in == 0 && out <= 1 {
			startCount++
		} else if out == 0 && in <= 1 {
			endCount++
		} else if in <= 1 && out <= 1 {
			middleCount++
		}
	}
	
	return startCount == 1 && endCount >= 1 && middleCount == len(f.States)-startCount-endCount
}

func hasCycles(f *fsm.FSM) bool {
	adj := buildAdjacency(f)
	
	// DFS-based cycle detection
	white := make(map[string]bool) // not visited
	gray := make(map[string]bool)  // in current path
	
	for _, name := range f.States {
		white[name] = true
	}
	
	var dfs func(string) bool
	dfs = func(node string) bool {
		white[node] = false
		gray[node] = true
		
		for _, next := range adj[node] {
			if gray[next] {
				return true // back edge = cycle
			}
			if white[next] && dfs(next) {
				return true
			}
		}
		
		gray[node] = false
		return false
	}
	
	for _, name := range f.States {
		if white[name] {
			if dfs(name) {
				return true
			}
		}
	}
	
	return false
}

func snapToGrid(positions map[string][2]int, gridX, gridY int) map[string][2]int {
	result := make(map[string][2]int)
	for name, pos := range positions {
		x := ((pos[0] + gridX/2) / gridX) * gridX
		y := ((pos[1] + gridY/2) / gridY) * gridY
		result[name] = [2]int{x, y}
	}
	return result
}

// resolveCollisions adjusts positions to prevent overlapping states.
func resolveCollisions(positions map[string][2]int, minLabelWidth int) map[string][2]int {
	if len(positions) <= 1 {
		return positions
	}
	
	// Build list for sorting
	type statePos struct {
		name string
		x, y int
	}
	states := make([]statePos, 0, len(positions))
	for name, pos := range positions {
		states = append(states, statePos{name, pos[0], pos[1]})
	}
	
	// Sort by Y then X
	sort.Slice(states, func(i, j int) bool {
		if states[i].y != states[j].y {
			return states[i].y < states[j].y
		}
		return states[i].x < states[j].x
	})
	
	// Check and resolve collisions
	result := make(map[string][2]int)
	occupied := make(map[[2]int]bool)
	
	for _, s := range states {
		x, y := s.x, s.y
		
		// Check if position (or nearby) is occupied
		attempts := 0
		for occupied[[2]int{x, y}] && attempts < 20 {
			// Try moving right first, then down
			if attempts%2 == 0 {
				x += minLabelWidth + 2
			} else {
				y += 2
				x = s.x // reset x
			}
			attempts++
		}
		
		result[s.name] = [2]int{x, y}
		
		// Mark this position and neighbours as occupied
		for dx := -minLabelWidth; dx <= minLabelWidth; dx++ {
			occupied[[2]int{x + dx, y}] = true
		}
	}
	
	return result
}

// clampToBounds ensures all positions are within the canvas.
func clampToBounds(positions map[string][2]int, width, height, labelWidth int) map[string][2]int {
	result := make(map[string][2]int)
	
	minX := 2
	maxX := width - labelWidth - 4
	minY := 1
	maxY := height - 2
	
	if maxX < minX {
		maxX = minX + 10
	}
	if maxY < minY {
		maxY = minY + 5
	}
	
	for name, pos := range positions {
		x, y := pos[0], pos[1]
		
		if x < minX {
			x = minX
		}
		if x > maxX {
			x = maxX
		}
		if y < minY {
			y = minY
		}
		if y > maxY {
			y = maxY
		}
		
		result[name] = [2]int{x, y}
	}
	
	return result
}
