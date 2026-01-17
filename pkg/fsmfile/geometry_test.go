package fsmfile

import (
	"math"
	"testing"
)

func TestSelfLoopControlPoints(t *testing.T) {
	state := Ellipse{CX: 100, CY: 100, RX: 40, RY: 30}
	params := DefaultSelfLoopParams()
	scale := 1.0

	points := SelfLoopControlPoints(state, params, scale)

	// Should have 7 points
	if len(points) != 7 {
		t.Errorf("Expected 7 points, got %d", len(points))
	}

	// P0 and P6 should be on the right edge of the ellipse (for LoopRight)
	if math.Abs(points[0].X-140) > 1 { // cx + rx = 100 + 40 = 140
		t.Errorf("P0.X expected ~140, got %.2f", points[0].X)
	}
	if math.Abs(points[6].X-140) > 1 {
		t.Errorf("P6.X expected ~140, got %.2f", points[6].X)
	}

	// P3 (apex) should be to the right of the ellipse
	if points[3].X <= state.CX+state.RX {
		t.Errorf("Apex should be right of ellipse edge, got X=%.2f", points[3].X)
	}

	// P0 and P6 should be vertically offset from center
	if math.Abs(points[0].Y-points[6].Y) < 5 {
		t.Errorf("P0 and P6 should have different Y values for arrowhead direction")
	}
}

func TestSelfLoopControlPointsTop(t *testing.T) {
	state := Ellipse{CX: 100, CY: 100, RX: 40, RY: 30}
	params := DefaultSelfLoopParams()
	params.Side = LoopTop
	scale := 1.0

	points := SelfLoopControlPoints(state, params, scale)

	// For LoopTop, apex should be above the ellipse
	if points[3].Y >= state.CY-state.RY {
		t.Errorf("Apex should be above ellipse for LoopTop, got Y=%.2f", points[3].Y)
	}

	// P0 and P6 should be on the top edge
	if math.Abs(points[0].Y-70) > 1 { // cy - ry = 100 - 30 = 70
		t.Errorf("P0.Y expected ~70, got %.2f", points[0].Y)
	}
}

func TestSelfLoopBounds(t *testing.T) {
	state := Ellipse{CX: 100, CY: 100, RX: 40, RY: 30}
	params := DefaultSelfLoopParams()
	points := SelfLoopControlPoints(state, params, 1.0)

	minX, minY, maxX, maxY := SelfLoopBounds(points)

	// Bounds should contain all control points (with margin)
	for i, p := range points {
		// Allow for the 10% margin added by SelfLoopBounds
		if p.X < minX-5 || p.X > maxX+5 || p.Y < minY-5 || p.Y > maxY+5 {
			t.Errorf("Point %d (%.2f, %.2f) outside bounds [%.2f-%.2f, %.2f-%.2f]",
				i, p.X, p.Y, minX, maxX, minY, maxY)
		}
	}
}

func TestChooseSelfLoopSide(t *testing.T) {
	// State near right edge of canvas - should choose another side
	state := Ellipse{CX: 180, CY: 100, RX: 30, RY: 20}
	side := ChooseSelfLoopSide(state, 200, 200, nil)

	// Should not choose Right since there's minimal space (only 200-180-30 = -10)
	if side == LoopRight {
		t.Logf("Side chosen: %v (LoopRight=%v)", side, LoopRight)
		// This is acceptable if there's actually space, let's check
		spaceRight := 200 - (state.CX + state.RX)
		required := state.RX * 1.5
		if spaceRight < required {
			t.Errorf("Should not choose LoopRight when only %.1f space available (need %.1f)", spaceRight, required)
		}
	}

	// State with plenty of space on all sides
	state2 := Ellipse{CX: 100, CY: 100, RX: 30, RY: 20}
	side2 := ChooseSelfLoopSide(state2, 300, 300, nil)

	// Should choose Right as the default preference
	if side2 != LoopRight {
		t.Errorf("Expected LoopRight for centered state, got %v", side2)
	}
}

func TestRectOverlap(t *testing.T) {
	tests := []struct {
		name     string
		a, b     Rect
		expected float64
	}{
		{
			name:     "No overlap - horizontally separated",
			a:        Rect{0, 0, 10, 10},
			b:        Rect{20, 0, 10, 10},
			expected: 0,
		},
		{
			name:     "No overlap - vertically separated",
			a:        Rect{0, 0, 10, 10},
			b:        Rect{0, 20, 10, 10},
			expected: 0,
		},
		{
			name:     "Full overlap (same rect)",
			a:        Rect{0, 0, 10, 10},
			b:        Rect{0, 0, 10, 10},
			expected: 100,
		},
		{
			name:     "Partial overlap",
			a:        Rect{0, 0, 10, 10},
			b:        Rect{5, 5, 10, 10},
			expected: 25,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := RectOverlap(tc.a, tc.b)
			if math.Abs(result-tc.expected) > 0.01 {
				t.Errorf("Expected overlap %.2f, got %.2f", tc.expected, result)
			}
		})
	}
}

func TestLabelPlacer(t *testing.T) {
	// Create placer with one obstacle (a state)
	obstacles := []Rect{
		{X: 100, Y: 100, W: 80, H: 60}, // State at (100,100)
	}
	placer := NewLabelPlacer(obstacles)

	// Try to place a label near the state
	anchor := Point{150, 100} // Right edge of state
	labelW := 40.0
	labelH := 14.0
	gap := 8.0

	pos := placer.PlaceLabel(anchor, labelW, labelH, gap)

	// The placed label should not overlap with the state
	labelRect := Rect{pos.X, pos.Y, labelW, labelH}
	overlap := RectOverlap(labelRect, obstacles[0])
	if overlap > 0 {
		t.Errorf("Label at (%.2f, %.2f) overlaps with state (overlap=%.2f)", pos.X, pos.Y, overlap)
	}
}

func TestLabelPlacerOnEdge(t *testing.T) {
	// Create placer with two states
	obstacles := []Rect{
		{X: 100, Y: 100, W: 60, H: 40}, // State 1
		{X: 250, Y: 100, W: 60, H: 40}, // State 2
	}
	placer := NewLabelPlacer(obstacles)

	// Place label on edge between states
	p1 := Point{130, 100} // Right edge of state 1
	p2 := Point{220, 100} // Left edge of state 2
	labelW := 40.0
	labelH := 14.0
	gap := 8.0

	pos := placer.PlaceLabelOnEdge(p1, p2, labelW, labelH, gap)

	// Label should be between the two states (approximately)
	if pos.X < 130 || pos.X > 220 {
		t.Errorf("Label X=%.2f should be between states (130-220)", pos.X)
	}

	// Label should not overlap either state
	labelRect := Rect{pos.X, pos.Y, labelW, labelH}
	for i, obs := range obstacles {
		overlap := RectOverlap(labelRect, obs)
		if overlap > 0 {
			t.Errorf("Label overlaps state %d (overlap=%.2f)", i, overlap)
		}
	}
}

func TestSelfLoopLabelPosition(t *testing.T) {
	state := Ellipse{CX: 100, CY: 100, RX: 40, RY: 30}
	params := DefaultSelfLoopParams()
	points := SelfLoopControlPoints(state, params, 1.0)

	labelW := 50.0
	labelH := 14.0
	scale := 1.0

	pos := SelfLoopLabelPosition(points, LoopRight, labelW, labelH, scale)

	// For LoopRight, label should be to the right of apex
	apex := points[3]
	if pos.X <= apex.X {
		t.Errorf("Label X=%.2f should be right of apex X=%.2f for LoopRight", pos.X, apex.X)
	}
	// Label Y should be near apex Y
	if math.Abs(pos.Y-apex.Y) > labelH {
		t.Errorf("Label Y=%.2f should be near apex Y=%.2f", pos.Y, apex.Y)
	}
}
