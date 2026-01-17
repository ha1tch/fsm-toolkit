// Visibility graph for routing edges around obstacles.
// Used for back-edges and flat edges that need to avoid crossing states.

package fsmfile

import (
	"container/heap"
	"math"
)

// VisibilityGraph represents the visibility graph for path planning.
type VisibilityGraph struct {
	Vertices []Point
	Adj      [][]VisEdge // Adjacency list
}

// VisEdge represents an edge in the visibility graph.
type VisEdge struct {
	To     int
	Weight float64
}

// BuildVisibilityGraph constructs a visibility graph around obstacles.
// The graph includes tangent points on each obstacle plus the start and end points.
func BuildVisibilityGraph(obstacles []Ellipse, start, end Point) *VisibilityGraph {
	vg := &VisibilityGraph{
		Vertices: make([]Point, 0),
	}

	// Add tangent points for each obstacle from both start and end
	for _, obs := range obstacles {
		// Add tangent points visible from start
		tangents := computeTangentPoints(obs, start)
		vg.Vertices = append(vg.Vertices, tangents...)

		// Add tangent points visible from end
		tangents = computeTangentPoints(obs, end)
		vg.Vertices = append(vg.Vertices, tangents...)
	}

	// Also add tangent points between all pairs of obstacles
	for i := 0; i < len(obstacles); i++ {
		for j := i + 1; j < len(obstacles); j++ {
			// Get points on obstacle i that are tangent to paths around obstacle j
			bitangents := computeBitangentPoints(obstacles[i], obstacles[j])
			vg.Vertices = append(vg.Vertices, bitangents...)
		}
	}

	// Remove duplicate vertices (within tolerance)
	vg.Vertices = removeDuplicatePoints(vg.Vertices, 1.0)

	// Add start and end
	startIdx := len(vg.Vertices)
	vg.Vertices = append(vg.Vertices, start)
	endIdx := len(vg.Vertices)
	vg.Vertices = append(vg.Vertices, end)

	// Initialize adjacency
	vg.Adj = make([][]VisEdge, len(vg.Vertices))
	for i := range vg.Adj {
		vg.Adj[i] = make([]VisEdge, 0)
	}

	// Connect vertices that can see each other
	for i := 0; i < len(vg.Vertices); i++ {
		for j := i + 1; j < len(vg.Vertices); j++ {
			if canSee(vg.Vertices[i], vg.Vertices[j], obstacles) {
				d := distance(vg.Vertices[i], vg.Vertices[j])
				vg.Adj[i] = append(vg.Adj[i], VisEdge{j, d})
				vg.Adj[j] = append(vg.Adj[j], VisEdge{i, d})
			}
		}
	}

	// Store indices for later use
	_ = startIdx
	_ = endIdx

	return vg
}

// computeTangentPoints returns the two tangent points on an ellipse from an external point.
func computeTangentPoints(e Ellipse, p Point) []Point {
	// Transform to unit circle space
	dx := (p.X - e.CX) / e.RX
	dy := (p.Y - e.CY) / e.RY
	d := math.Sqrt(dx*dx + dy*dy)

	if d <= 1.01 { // Point inside or on ellipse
		return nil
	}

	// Angle to point from ellipse center
	theta := math.Atan2(dy, dx)

	// Half-angle of tangent cone (for unit circle)
	alpha := math.Asin(1 / d)

	// Two tangent angles on the unit circle
	t1 := theta + alpha + math.Pi/2
	t2 := theta - alpha + math.Pi/2

	// Convert back to ellipse space with padding
	padding := 1.08 // Slightly outside ellipse for clearance
	return []Point{
		{e.CX + e.RX*padding*math.Cos(t1), e.CY + e.RY*padding*math.Sin(t1)},
		{e.CX + e.RX*padding*math.Cos(t2), e.CY + e.RY*padding*math.Sin(t2)},
	}
}

// computeBitangentPoints returns points on the hulls of two ellipses for routing between them.
func computeBitangentPoints(e1, e2 Ellipse) []Point {
	// Get tangent points on e1 from e2's center and vice versa
	points := make([]Point, 0, 8)

	t1 := computeTangentPoints(e1, Point{e2.CX, e2.CY})
	points = append(points, t1...)

	t2 := computeTangentPoints(e2, Point{e1.CX, e1.CY})
	points = append(points, t2...)

	return points
}

// removeDuplicatePoints removes points that are within tolerance of each other.
func removeDuplicatePoints(points []Point, tolerance float64) []Point {
	if len(points) == 0 {
		return points
	}

	result := make([]Point, 0, len(points))
	for _, p := range points {
		isDup := false
		for _, r := range result {
			if distance(p, r) < tolerance {
				isDup = true
				break
			}
		}
		if !isDup {
			result = append(result, p)
		}
	}
	return result
}

// canSee checks if two points can see each other without crossing any obstacle interior.
func canSee(a, b Point, obstacles []Ellipse) bool {
	for _, obs := range obstacles {
		if lineIntersectsEllipse(a, b, obs) {
			return false
		}
	}
	return true
}

// lineIntersectsEllipse checks if a line segment passes through an ellipse interior.
func lineIntersectsEllipse(p1, p2 Point, e Ellipse) bool {
	// Transform to unit circle space
	x1 := (p1.X - e.CX) / e.RX
	y1 := (p1.Y - e.CY) / e.RY
	x2 := (p2.X - e.CX) / e.RX
	y2 := (p2.Y - e.CY) / e.RY

	// Check if both endpoints are outside
	d1 := x1*x1 + y1*y1
	d2 := x2*x2 + y2*y2

	// If either endpoint is inside, the line definitely intersects
	if d1 < 0.95 || d2 < 0.95 {
		return true
	}

	// Line: P = P1 + t(P2-P1), t ∈ [0,1]
	// Circle: x² + y² = 1
	// Substitute and solve quadratic: at² + bt + c = 0

	dx := x2 - x1
	dy := y2 - y1

	a := dx*dx + dy*dy
	if a < 1e-10 {
		return false // Degenerate line segment
	}

	b := 2 * (x1*dx + y1*dy)
	c := x1*x1 + y1*y1 - 1

	discriminant := b*b - 4*a*c

	if discriminant < 0 {
		return false // No intersection with circle
	}

	sqrtD := math.Sqrt(discriminant)
	t1 := (-b - sqrtD) / (2 * a)
	t2 := (-b + sqrtD) / (2 * a)

	// Check if intersection is within segment [0,1] with margin for tangent touches
	margin := 0.02
	return (t1 > margin && t1 < 1-margin) || (t2 > margin && t2 < 1-margin)
}

// ShortestPath finds the shortest path through the visibility graph using Dijkstra's algorithm.
func (vg *VisibilityGraph) ShortestPath(startIdx, endIdx int) []Point {
	if startIdx < 0 || startIdx >= len(vg.Vertices) ||
		endIdx < 0 || endIdx >= len(vg.Vertices) {
		return nil
	}

	n := len(vg.Vertices)
	dist := make([]float64, n)
	prev := make([]int, n)

	for i := range dist {
		dist[i] = math.MaxFloat64
		prev[i] = -1
	}
	dist[startIdx] = 0

	// Priority queue (min-heap)
	pq := &priorityQueue{}
	heap.Init(pq)
	heap.Push(pq, &pqItem{startIdx, 0})

	for pq.Len() > 0 {
		item := heap.Pop(pq).(*pqItem)
		u := item.vertex

		if u == endIdx {
			break
		}

		if item.dist > dist[u] {
			continue
		}

		for _, edge := range vg.Adj[u] {
			v := edge.To
			newDist := dist[u] + edge.Weight
			if newDist < dist[v] {
				dist[v] = newDist
				prev[v] = u
				heap.Push(pq, &pqItem{v, newDist})
			}
		}
	}

	// Reconstruct path
	if dist[endIdx] == math.MaxFloat64 {
		// No path found, return direct line
		return []Point{vg.Vertices[startIdx], vg.Vertices[endIdx]}
	}

	var path []Point
	for v := endIdx; v != -1; v = prev[v] {
		path = append([]Point{vg.Vertices[v]}, path...)
	}

	return path
}

// FindStartEndIndices returns the indices of start and end points in the visibility graph.
func (vg *VisibilityGraph) FindStartEndIndices(start, end Point) (int, int) {
	startIdx := -1
	endIdx := -1
	tolerance := 0.1

	for i, v := range vg.Vertices {
		if distance(v, start) < tolerance {
			startIdx = i
		}
		if distance(v, end) < tolerance {
			endIdx = i
		}
	}

	return startIdx, endIdx
}

// RouteAroundObstacles finds a path from start to end that avoids all obstacles.
func RouteAroundObstacles(start, end Point, obstacles []Ellipse) []Point {
	if len(obstacles) == 0 {
		return []Point{start, end}
	}

	// Check if direct path is clear
	if canSee(start, end, obstacles) {
		return []Point{start, end}
	}

	// Build visibility graph and find shortest path
	vg := BuildVisibilityGraph(obstacles, start, end)

	startIdx, endIdx := vg.FindStartEndIndices(start, end)
	if startIdx == -1 || endIdx == -1 {
		// Fallback: return direct path
		return []Point{start, end}
	}

	path := vg.ShortestPath(startIdx, endIdx)
	if len(path) == 0 {
		return []Point{start, end}
	}

	return path
}

// Priority queue implementation for Dijkstra's algorithm
type pqItem struct {
	vertex int
	dist   float64
}

type priorityQueue []*pqItem

func (pq priorityQueue) Len() int           { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool { return pq[i].dist < pq[j].dist }
func (pq priorityQueue) Swap(i, j int)      { pq[i], pq[j] = pq[j], pq[i] }

func (pq *priorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*pqItem))
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

func distance(a, b Point) float64 {
	dx := b.X - a.X
	dy := b.Y - a.Y
	return math.Sqrt(dx*dx + dy*dy)
}
