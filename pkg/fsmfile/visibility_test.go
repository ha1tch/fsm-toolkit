package fsmfile

import (
	"math"
	"testing"
)

func TestComputeTangentPoints(t *testing.T) {
	// Ellipse centered at origin
	e := Ellipse{CX: 100, CY: 100, RX: 30, RY: 20}

	// Point to the right of ellipse
	p := Point{200, 100}

	tangents := computeTangentPoints(e, p)

	if len(tangents) != 2 {
		t.Fatalf("Expected 2 tangent points, got %d", len(tangents))
	}

	// Both tangent points should be on or near the ellipse
	for i, tp := range tangents {
		dx := (tp.X - e.CX) / e.RX
		dy := (tp.Y - e.CY) / e.RY
		// Check if point is near ellipse (allowing for padding)
		distFromCenter := math.Sqrt(dx*dx + dy*dy)
		if distFromCenter < 0.9 || distFromCenter > 1.2 {
			t.Errorf("Tangent point %d not on ellipse: distance ratio = %.2f", i, distFromCenter)
		}
	}
}

func TestComputeTangentPointsInsideEllipse(t *testing.T) {
	e := Ellipse{CX: 100, CY: 100, RX: 30, RY: 20}

	// Point inside ellipse
	p := Point{100, 100}

	tangents := computeTangentPoints(e, p)

	// Should return no tangents for point inside
	if tangents != nil && len(tangents) > 0 {
		t.Errorf("Expected no tangents for point inside ellipse, got %d", len(tangents))
	}
}

func TestLineIntersectsEllipse(t *testing.T) {
	e := Ellipse{CX: 100, CY: 100, RX: 30, RY: 20}

	tests := []struct {
		name     string
		p1, p2   Point
		expected bool
	}{
		{
			name:     "Line passes through center",
			p1:       Point{50, 100},
			p2:       Point{150, 100},
			expected: true,
		},
		{
			name:     "Line misses completely",
			p1:       Point{50, 50},
			p2:       Point{150, 50},
			expected: false,
		},
		{
			name:     "Line above ellipse (should not intersect)",
			p1:       Point{70, 70},
			p2:       Point{130, 70},
			expected: false,
		},
		{
			name:     "Diagonal through ellipse",
			p1:       Point{80, 80},
			p2:       Point{120, 120},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := lineIntersectsEllipse(tc.p1, tc.p2, e)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestCanSee(t *testing.T) {
	obstacles := []Ellipse{
		{CX: 100, CY: 100, RX: 30, RY: 20},
	}

	// Points that can see each other (no obstacle between them)
	a := Point{50, 50}
	b := Point{50, 150}
	if !canSee(a, b, obstacles) {
		t.Error("Points on same side should be able to see each other")
	}

	// Points that cannot see each other (obstacle between them)
	c := Point{50, 100}
	d := Point{150, 100}
	if canSee(c, d, obstacles) {
		t.Error("Points on opposite sides with obstacle between should not see each other")
	}
}

func TestRouteAroundObstacles(t *testing.T) {
	obstacles := []Ellipse{
		{CX: 100, CY: 100, RX: 30, RY: 20},
	}

	// Route that needs to go around obstacle
	start := Point{50, 100}
	end := Point{150, 100}

	path := RouteAroundObstacles(start, end, obstacles)

	// Should have more than 2 points (not direct)
	if len(path) < 2 {
		t.Errorf("Expected path with at least 2 points, got %d", len(path))
	}

	// First point should be start
	if path[0].X != start.X || path[0].Y != start.Y {
		t.Errorf("Path should start at start point")
	}

	// Last point should be end
	last := path[len(path)-1]
	if last.X != end.X || last.Y != end.Y {
		t.Errorf("Path should end at end point")
	}
}

func TestRouteAroundObstaclesDirect(t *testing.T) {
	obstacles := []Ellipse{
		{CX: 100, CY: 100, RX: 30, RY: 20},
	}

	// Route that doesn't need to go around (above obstacle)
	start := Point{50, 50}
	end := Point{150, 50}

	path := RouteAroundObstacles(start, end, obstacles)

	// Should be direct (2 points)
	if len(path) != 2 {
		t.Errorf("Expected direct path with 2 points, got %d", len(path))
	}
}

func TestRouteAroundObstaclesNoObstacles(t *testing.T) {
	start := Point{0, 0}
	end := Point{100, 100}

	path := RouteAroundObstacles(start, end, nil)

	// Should be direct (2 points)
	if len(path) != 2 {
		t.Errorf("Expected direct path with 2 points, got %d", len(path))
	}
}

func TestBuildVisibilityGraph(t *testing.T) {
	obstacles := []Ellipse{
		{CX: 100, CY: 100, RX: 30, RY: 20},
	}

	start := Point{50, 100}
	end := Point{150, 100}

	vg := BuildVisibilityGraph(obstacles, start, end)

	// Should have vertices (tangent points + start + end)
	if len(vg.Vertices) < 4 {
		t.Errorf("Expected at least 4 vertices, got %d", len(vg.Vertices))
	}

	// Should have adjacency list
	if len(vg.Adj) != len(vg.Vertices) {
		t.Errorf("Adjacency list size %d doesn't match vertex count %d",
			len(vg.Adj), len(vg.Vertices))
	}
}

func TestVisibilityGraphShortestPath(t *testing.T) {
	vg := &VisibilityGraph{
		Vertices: []Point{
			{0, 0},   // 0
			{50, 0},  // 1
			{100, 0}, // 2
		},
		Adj: [][]VisEdge{
			{{1, 50}},         // 0 -> 1
			{{0, 50}, {2, 50}}, // 1 -> 0, 1 -> 2
			{{1, 50}},         // 2 -> 1
		},
	}

	path := vg.ShortestPath(0, 2)

	if len(path) != 3 {
		t.Errorf("Expected path of length 3, got %d", len(path))
	}
}

func TestDistance(t *testing.T) {
	a := Point{0, 0}
	b := Point{3, 4}

	d := distance(a, b)

	if math.Abs(d-5) > 0.001 {
		t.Errorf("Expected distance 5, got %.3f", d)
	}
}
