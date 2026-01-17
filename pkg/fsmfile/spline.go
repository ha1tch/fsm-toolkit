// Spline fitting for edge routing through corridors.
// Implements smooth Bézier curve fitting that respects routing box constraints.

package fsmfile

import "math"

// FitSplineThroughBoxes creates a smooth spline from start to end
// that stays within all routing boxes.
func FitSplineThroughBoxes(start, end Point, boxes []RoutingBox) []Point {
	if len(boxes) == 0 {
		// Direct edge: simple quadratic Bézier
		mid := Point{(start.X + end.X) / 2, (start.Y + end.Y) / 2}
		return []Point{start, mid, end}
	}

	// Build waypoints through box centers
	waypoints := []Point{start}
	for _, box := range boxes {
		waypoints = append(waypoints, box.Center())
	}
	waypoints = append(waypoints, end)

	// Try to fit a spline
	spline := fitBezierToWaypoints(waypoints)

	// Check if spline stays in boxes
	if splineInsideBoxes(spline, boxes, start, end) {
		return spline
	}

	// Tighten control points iteratively
	for iterations := 0; iterations < 10; iterations++ {
		spline = tightenSpline(spline)
		if splineInsideBoxes(spline, boxes, start, end) {
			return spline
		}
	}

	// Fall back to polyline through waypoints (always valid)
	return waypoints
}

// fitBezierToWaypoints creates a smooth curve through waypoints using
// Catmull-Rom to cubic Bézier conversion.
func fitBezierToWaypoints(waypoints []Point) []Point {
	if len(waypoints) <= 2 {
		return waypoints
	}

	result := []Point{waypoints[0]}

	for i := 0; i < len(waypoints)-1; i++ {
		// Get the four points needed for Catmull-Rom
		p0 := waypoints[maxInt(0, i-1)]
		p1 := waypoints[i]
		p2 := waypoints[minInt(len(waypoints)-1, i+1)]
		p3 := waypoints[minInt(len(waypoints)-1, i+2)]

		// Catmull-Rom to Bézier control points
		// The conversion uses 1/6 of the tangent vectors
		ctrl1 := Point{
			X: p1.X + (p2.X-p0.X)/6,
			Y: p1.Y + (p2.Y-p0.Y)/6,
		}
		ctrl2 := Point{
			X: p2.X - (p3.X-p1.X)/6,
			Y: p2.Y - (p3.Y-p1.Y)/6,
		}

		result = append(result, ctrl1, ctrl2, p2)
	}

	return result
}

// splineInsideBoxes checks if a spline stays within routing boxes.
// The start and end points are allowed to be outside boxes.
func splineInsideBoxes(spline []Point, boxes []RoutingBox, start, end Point) bool {
	if len(boxes) == 0 {
		return true
	}

	// Sample spline at many points and check each
	numSamples := 100
	for i := 0; i <= numSamples; i++ {
		t := float64(i) / float64(numSamples)
		pt := EvaluateSpline(spline, t)

		// Skip checking start and end regions (they're outside boxes)
		if t < 0.05 || t > 0.95 {
			continue
		}

		// Find which box this t value corresponds to
		// Map t ∈ [0.05, 0.95] to box index
		normalizedT := (t - 0.05) / 0.9
		boxIdx := int(normalizedT * float64(len(boxes)))
		if boxIdx >= len(boxes) {
			boxIdx = len(boxes) - 1
		}
		if boxIdx < 0 {
			boxIdx = 0
		}

		box := boxes[boxIdx]

		// Allow some margin for numerical imprecision
		margin := 2.0
		if pt.X < box.Left-margin || pt.X > box.Right+margin ||
			pt.Y < box.Top-margin || pt.Y > box.Bottom+margin {
			return false
		}
	}
	return true
}

// tightenSpline moves control points closer to their anchor waypoints.
func tightenSpline(spline []Point) []Point {
	if len(spline) < 4 {
		return spline
	}

	result := make([]Point, len(spline))
	copy(result, spline)

	// Move control points toward their anchor points
	// For cubic Bézier: indices are 0(start), 1(ctrl1), 2(ctrl2), 3(end), ...
	for i := 1; i < len(result)-1; i++ {
		// Skip waypoint positions (every 3rd point starting from 0)
		if i%3 == 0 {
			continue
		}

		// Find the nearest anchor points
		var anchor Point
		if i%3 == 1 {
			// First control point - move toward start of segment
			segmentStart := (i / 3) * 3
			anchor = result[segmentStart]
		} else {
			// Second control point - move toward end of segment
			segmentEnd := ((i / 3) + 1) * 3
			if segmentEnd >= len(result) {
				segmentEnd = len(result) - 1
			}
			anchor = result[segmentEnd]
		}

		// Move 20% toward anchor
		result[i] = Point{
			X: result[i].X*0.8 + anchor.X*0.2,
			Y: result[i].Y*0.8 + anchor.Y*0.2,
		}
	}

	return result
}

// EvaluateSpline computes the point on a spline at parameter t ∈ [0,1].
func EvaluateSpline(spline []Point, t float64) Point {
	if len(spline) == 0 {
		return Point{0, 0}
	}
	if len(spline) == 1 {
		return spline[0]
	}
	if len(spline) < 4 {
		// Linear interpolation for simple paths
		idx := int(t * float64(len(spline)-1))
		if idx >= len(spline)-1 {
			return spline[len(spline)-1]
		}
		localT := t*float64(len(spline)-1) - float64(idx)
		return Point{
			X: spline[idx].X*(1-localT) + spline[idx+1].X*localT,
			Y: spline[idx].Y*(1-localT) + spline[idx+1].Y*localT,
		}
	}

	// Cubic Bézier segments
	// Spline format: [P0, C1, C2, P1, C3, C4, P2, ...]
	// Each segment has 4 points: start, ctrl1, ctrl2, end
	numSegments := (len(spline) - 1) / 3
	if numSegments < 1 {
		numSegments = 1
	}

	segment := int(t * float64(numSegments))
	if segment >= numSegments {
		segment = numSegments - 1
	}

	// Local t within this segment
	localT := t*float64(numSegments) - float64(segment)
	if localT < 0 {
		localT = 0
	}
	if localT > 1 {
		localT = 1
	}

	// Get the four control points for this segment
	i := segment * 3
	if i+3 >= len(spline) {
		// Handle edge case
		return spline[len(spline)-1]
	}

	p0, p1, p2, p3 := spline[i], spline[i+1], spline[i+2], spline[i+3]

	// Cubic Bézier evaluation
	mt := 1 - localT
	mt2 := mt * mt
	mt3 := mt2 * mt
	t2 := localT * localT
	t3 := t2 * localT

	return Point{
		X: mt3*p0.X + 3*mt2*localT*p1.X + 3*mt*t2*p2.X + t3*p3.X,
		Y: mt3*p0.Y + 3*mt2*localT*p1.Y + 3*mt*t2*p2.Y + t3*p3.Y,
	}
}

// EvaluateSplineTangent computes the tangent vector at parameter t.
func EvaluateSplineTangent(spline []Point, t float64) Point {
	if len(spline) < 4 {
		if len(spline) >= 2 {
			// Linear: tangent is just the direction
			return Point{
				X: spline[len(spline)-1].X - spline[0].X,
				Y: spline[len(spline)-1].Y - spline[0].Y,
			}
		}
		return Point{1, 0}
	}

	numSegments := (len(spline) - 1) / 3
	segment := int(t * float64(numSegments))
	if segment >= numSegments {
		segment = numSegments - 1
	}

	localT := t*float64(numSegments) - float64(segment)

	i := segment * 3
	if i+3 >= len(spline) {
		return Point{1, 0}
	}

	p0, p1, p2, p3 := spline[i], spline[i+1], spline[i+2], spline[i+3]

	// Derivative of cubic Bézier
	mt := 1 - localT
	mt2 := mt * mt
	t2 := localT * localT

	return Point{
		X: 3*mt2*(p1.X-p0.X) + 6*mt*localT*(p2.X-p1.X) + 3*t2*(p3.X-p2.X),
		Y: 3*mt2*(p1.Y-p0.Y) + 6*mt*localT*(p2.Y-p1.Y) + 3*t2*(p3.Y-p2.Y),
	}
}

// SplineLength approximates the length of a spline by sampling.
func SplineLength(spline []Point) float64 {
	if len(spline) < 2 {
		return 0
	}

	length := 0.0
	numSamples := 100
	prev := EvaluateSpline(spline, 0)

	for i := 1; i <= numSamples; i++ {
		t := float64(i) / float64(numSamples)
		curr := EvaluateSpline(spline, t)

		dx := curr.X - prev.X
		dy := curr.Y - prev.Y
		length += math.Sqrt(dx*dx + dy*dy)

		prev = curr
	}

	return length
}

// SplineMidpoint returns the point at the middle of the spline (t=0.5).
func SplineMidpoint(spline []Point) Point {
	return EvaluateSpline(spline, 0.5)
}

// Helper functions
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
