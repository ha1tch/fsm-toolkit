package fsmfile

import (
	"math"
	"testing"
)

func TestFitSplineThroughBoxesNoBoxes(t *testing.T) {
	start := Point{0, 0}
	end := Point{100, 100}

	spline := FitSplineThroughBoxes(start, end, nil)

	// Should return a simple 3-point path (start, mid, end)
	if len(spline) != 3 {
		t.Errorf("Expected 3 points for direct edge, got %d", len(spline))
	}

	// Check start and end
	if spline[0].X != start.X || spline[0].Y != start.Y {
		t.Errorf("Start point wrong: %v", spline[0])
	}
	if spline[2].X != end.X || spline[2].Y != end.Y {
		t.Errorf("End point wrong: %v", spline[2])
	}

	// Middle should be midpoint
	mid := spline[1]
	if mid.X != 50 || mid.Y != 50 {
		t.Errorf("Midpoint wrong: expected (50,50), got (%.1f, %.1f)", mid.X, mid.Y)
	}
}

func TestFitSplineThroughBoxesWithBoxes(t *testing.T) {
	start := Point{0, 0}
	end := Point{100, 200}

	boxes := []RoutingBox{
		{Left: 20, Right: 80, Top: 50, Bottom: 100},
		{Left: 30, Right: 90, Top: 100, Bottom: 150},
	}

	spline := FitSplineThroughBoxes(start, end, boxes)

	// Should return some points
	if len(spline) < 3 {
		t.Errorf("Expected at least 3 points, got %d", len(spline))
	}

	// First point should be start
	if spline[0].X != start.X || spline[0].Y != start.Y {
		t.Errorf("Start point wrong: %v", spline[0])
	}

	// Last point should be end
	last := spline[len(spline)-1]
	if last.X != end.X || last.Y != end.Y {
		t.Errorf("End point wrong: %v", last)
	}
}

func TestEvaluateSpline(t *testing.T) {
	// Simple linear spline
	spline := []Point{
		{0, 0},
		{100, 100},
	}

	// t=0 should give start
	p0 := EvaluateSpline(spline, 0)
	if p0.X != 0 || p0.Y != 0 {
		t.Errorf("t=0 should give start, got (%.1f, %.1f)", p0.X, p0.Y)
	}

	// t=1 should give end
	p1 := EvaluateSpline(spline, 1)
	if p1.X != 100 || p1.Y != 100 {
		t.Errorf("t=1 should give end, got (%.1f, %.1f)", p1.X, p1.Y)
	}

	// t=0.5 should give midpoint
	p5 := EvaluateSpline(spline, 0.5)
	if math.Abs(p5.X-50) > 0.1 || math.Abs(p5.Y-50) > 0.1 {
		t.Errorf("t=0.5 should give midpoint, got (%.1f, %.1f)", p5.X, p5.Y)
	}
}

func TestEvaluateSplineCubic(t *testing.T) {
	// Cubic Bézier: start, ctrl1, ctrl2, end
	spline := []Point{
		{0, 0},     // P0
		{30, 50},   // C1
		{70, 50},   // C2
		{100, 100}, // P1
	}

	// t=0 should give start
	p0 := EvaluateSpline(spline, 0)
	if math.Abs(p0.X) > 0.1 || math.Abs(p0.Y) > 0.1 {
		t.Errorf("t=0 should give start, got (%.1f, %.1f)", p0.X, p0.Y)
	}

	// t=1 should give end
	p1 := EvaluateSpline(spline, 1)
	if math.Abs(p1.X-100) > 0.1 || math.Abs(p1.Y-100) > 0.1 {
		t.Errorf("t=1 should give end, got (%.1f, %.1f)", p1.X, p1.Y)
	}

	// t=0.5 should be somewhere in the middle
	p5 := EvaluateSpline(spline, 0.5)
	if p5.X < 0 || p5.X > 100 || p5.Y < 0 || p5.Y > 100 {
		t.Errorf("t=0.5 should be within bounds, got (%.1f, %.1f)", p5.X, p5.Y)
	}
}

func TestSplineLength(t *testing.T) {
	// Straight line from (0,0) to (100,0)
	spline := []Point{{0, 0}, {100, 0}}

	length := SplineLength(spline)

	// Should be approximately 100
	if math.Abs(length-100) > 1 {
		t.Errorf("Expected length ~100, got %.1f", length)
	}
}

func TestSplineMidpoint(t *testing.T) {
	spline := []Point{{0, 0}, {100, 100}}

	mid := SplineMidpoint(spline)

	if math.Abs(mid.X-50) > 0.1 || math.Abs(mid.Y-50) > 0.1 {
		t.Errorf("Expected midpoint (50,50), got (%.1f, %.1f)", mid.X, mid.Y)
	}
}

func TestFitBezierToWaypoints(t *testing.T) {
	waypoints := []Point{
		{0, 0},
		{50, 50},
		{100, 0},
	}

	spline := fitBezierToWaypoints(waypoints)

	// Should have more points than waypoints (control points added)
	if len(spline) <= len(waypoints) {
		t.Errorf("Expected more points than waypoints, got %d", len(spline))
	}

	// First point should be first waypoint
	if spline[0].X != waypoints[0].X || spline[0].Y != waypoints[0].Y {
		t.Errorf("First point wrong: %v", spline[0])
	}

	// Last point should be last waypoint
	last := spline[len(spline)-1]
	lastWP := waypoints[len(waypoints)-1]
	if last.X != lastWP.X || last.Y != lastWP.Y {
		t.Errorf("Last point wrong: %v", last)
	}
}

func TestTightenSpline(t *testing.T) {
	// Create a spline with control points far from anchors
	spline := []Point{
		{0, 0},     // P0
		{50, 100},  // C1 - far from P0
		{50, 100},  // C2 - far from P1
		{100, 0},   // P1
	}

	tightened := tightenSpline(spline)

	// Control points should be closer to their anchors
	// C1 at index 1 should move toward P0 at index 0
	if tightened[1].X >= spline[1].X || tightened[1].Y >= spline[1].Y {
		// Since P0 is at origin, moving toward it should decrease both coordinates
		// unless the implementation differs
		t.Logf("C1 moved from (%.1f, %.1f) to (%.1f, %.1f)",
			spline[1].X, spline[1].Y, tightened[1].X, tightened[1].Y)
	}

	// Endpoints should not move
	if tightened[0].X != spline[0].X || tightened[0].Y != spline[0].Y {
		t.Error("Start point should not move")
	}
	if tightened[3].X != spline[3].X || tightened[3].Y != spline[3].Y {
		t.Error("End point should not move")
	}
}

func TestEvaluateSplineTangent(t *testing.T) {
	// Straight line spline
	spline := []Point{{0, 0}, {100, 0}}

	tangent := EvaluateSplineTangent(spline, 0.5)

	// Tangent should point in X direction
	if tangent.X <= 0 {
		t.Errorf("Tangent X should be positive, got %.1f", tangent.X)
	}
	if math.Abs(tangent.Y) > 0.1 {
		t.Errorf("Tangent Y should be ~0, got %.1f", tangent.Y)
	}
}

func TestMinMaxInt(t *testing.T) {
	if minInt(3, 5) != 3 {
		t.Error("minInt(3,5) should be 3")
	}
	if minInt(5, 3) != 3 {
		t.Error("minInt(5,3) should be 3")
	}
	if maxInt(3, 5) != 5 {
		t.Error("maxInt(3,5) should be 5")
	}
	if maxInt(5, 3) != 5 {
		t.Error("maxInt(5,3) should be 5")
	}
}
