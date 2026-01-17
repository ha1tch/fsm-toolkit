// Geometric utilities for FSM diagram rendering.
// Provides unified self-loop rendering, label placement, and routing support.

package fsmfile

import "math"

// LoopSide indicates which side of a state the self-loop extends to.
type LoopSide int

const (
	LoopRight  LoopSide = iota // Default: loop extends right
	LoopLeft                   // Loop extends left
	LoopTop                    // Loop extends above
	LoopBottom                 // Loop extends below
)

// SelfLoopParams configures self-loop rendering.
type SelfLoopParams struct {
	Side       LoopSide
	Index      int     // 0-based index for multiple self-loops on same state
	BaseOffset float64 // Base distance from state edge to loop apex
	Spacing    float64 // Additional offset per loop index
	PortOffset float64 // Vertical offset of ports from center (as fraction of ry)
}

// DefaultSelfLoopParams returns standard parameters.
func DefaultSelfLoopParams() SelfLoopParams {
	return SelfLoopParams{
		Side:       LoopRight,
		Index:      0,
		BaseOffset: 25.0,
		Spacing:    18.0,
		PortOffset: 0.35,
	}
}

// Point represents a 2D coordinate.
type Point struct {
	X, Y float64
}

// Ellipse represents a state's bounding ellipse.
type Ellipse struct {
	CX, CY float64 // Center
	RX, RY float64 // Radii
}

// SelfLoopControlPoints computes the 7 control points for a self-loop.
// Returns points P0-P6 forming two cubic Bézier segments:
//
//	Segment 1: P0, P1, P2, P3 (tail to apex)
//	Segment 2: P3, P4, P5, P6 (apex to head)
func SelfLoopControlPoints(state Ellipse, params SelfLoopParams, scale float64) []Point {
	cx, cy := state.CX, state.CY
	rx, ry := state.RX, state.RY

	// Total offset from state center to apex
	offset := (params.BaseOffset + float64(params.Index)*params.Spacing) * scale

	// Port positions (where loop connects to state)
	portY := ry * params.PortOffset

	// Control point spread
	ctrlSpread := ry * 0.5

	switch params.Side {
	case LoopRight:
		dx := rx + offset
		return []Point{
			{cx + rx, cy - portY},                       // P0: tail port
			{cx + rx + dx*0.4, cy - portY - ctrlSpread}, // P1
			{cx + dx, cy - ctrlSpread},                  // P2
			{cx + dx, cy},                               // P3: apex
			{cx + dx, cy + ctrlSpread},                  // P4
			{cx + rx + dx*0.4, cy + portY + ctrlSpread}, // P5
			{cx + rx, cy + portY},                       // P6: head port
		}

	case LoopLeft:
		dx := rx + offset
		return []Point{
			{cx - rx, cy - portY},                       // P0
			{cx - rx - dx*0.4, cy - portY - ctrlSpread}, // P1
			{cx - dx, cy - ctrlSpread},                  // P2
			{cx - dx, cy},                               // P3: apex
			{cx - dx, cy + ctrlSpread},                  // P4
			{cx - rx - dx*0.4, cy + portY + ctrlSpread}, // P5
			{cx - rx, cy + portY},                       // P6
		}

	case LoopTop:
		dy := ry + offset
		portX := rx * params.PortOffset
		return []Point{
			{cx - portX, cy - ry},                       // P0
			{cx - portX - ctrlSpread, cy - ry - dy*0.4}, // P1
			{cx - ctrlSpread, cy - dy},                  // P2
			{cx, cy - dy},                               // P3: apex
			{cx + ctrlSpread, cy - dy},                  // P4
			{cx + portX + ctrlSpread, cy - ry - dy*0.4}, // P5
			{cx + portX, cy - ry},                       // P6
		}

	case LoopBottom:
		dy := ry + offset
		portX := rx * params.PortOffset
		return []Point{
			{cx - portX, cy + ry},                       // P0
			{cx - portX - ctrlSpread, cy + ry + dy*0.4}, // P1
			{cx - ctrlSpread, cy + dy},                  // P2
			{cx, cy + dy},                               // P3: apex
			{cx + ctrlSpread, cy + dy},                  // P4
			{cx + portX + ctrlSpread, cy + ry + dy*0.4}, // P5
			{cx + portX, cy + ry},                       // P6
		}
	}

	return nil // unreachable
}

// SelfLoopLabelPosition returns the label anchor point for a self-loop.
func SelfLoopLabelPosition(points []Point, side LoopSide, labelWidth, labelHeight, scale float64) Point {
	apex := points[3]
	gap := 6.0 * scale

	switch side {
	case LoopRight:
		return Point{apex.X + gap + labelWidth/2, apex.Y}
	case LoopLeft:
		return Point{apex.X - gap - labelWidth/2, apex.Y}
	case LoopTop:
		return Point{apex.X, apex.Y - gap - labelHeight/2}
	case LoopBottom:
		return Point{apex.X, apex.Y + gap + labelHeight/2}
	}
	return apex
}

// SelfLoopBounds returns the bounding box of the self-loop (for canvas clipping check).
func SelfLoopBounds(points []Point) (minX, minY, maxX, maxY float64) {
	if len(points) == 0 {
		return 0, 0, 0, 0
	}

	minX, minY = points[0].X, points[0].Y
	maxX, maxY = points[0].X, points[0].Y

	for _, p := range points {
		if p.X < minX {
			minX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}

	// Bézier curves can extend beyond control points
	// Add 10% margin
	dx := (maxX - minX) * 0.1
	dy := (maxY - minY) * 0.1
	return minX - dx, minY - dy, maxX + dx, maxY + dy
}

// ChooseSelfLoopSide determines the best side for a self-loop based on
// available space and canvas boundaries.
func ChooseSelfLoopSide(state Ellipse, canvasWidth, canvasHeight float64,
	occupiedSides map[LoopSide]bool) LoopSide {
	// Preference order: Right, Top, Left, Bottom
	preference := []LoopSide{LoopRight, LoopTop, LoopLeft, LoopBottom}

	// Calculate available space on each side
	spaceRight := canvasWidth - (state.CX + state.RX)
	spaceLeft := state.CX - state.RX
	spaceTop := state.CY - state.RY
	spaceBottom := canvasHeight - (state.CY + state.RY)

	requiredSpace := state.RX * 1.5 // Minimum space needed for loop

	for _, side := range preference {
		if occupiedSides != nil && occupiedSides[side] {
			continue
		}

		switch side {
		case LoopRight:
			if spaceRight >= requiredSpace {
				return LoopRight
			}
		case LoopLeft:
			if spaceLeft >= requiredSpace {
				return LoopLeft
			}
		case LoopTop:
			if spaceTop >= requiredSpace {
				return LoopTop
			}
		case LoopBottom:
			if spaceBottom >= requiredSpace {
				return LoopBottom
			}
		}
	}

	// Fall back to side with most space
	maxSpace := spaceRight
	bestSide := LoopRight
	if spaceLeft > maxSpace && (occupiedSides == nil || !occupiedSides[LoopLeft]) {
		maxSpace = spaceLeft
		bestSide = LoopLeft
	}
	if spaceTop > maxSpace && (occupiedSides == nil || !occupiedSides[LoopTop]) {
		maxSpace = spaceTop
		bestSide = LoopTop
	}
	if spaceBottom > maxSpace && (occupiedSides == nil || !occupiedSides[LoopBottom]) {
		bestSide = LoopBottom
	}

	return bestSide
}

// Rect represents an axis-aligned rectangle.
type Rect struct {
	X, Y float64 // Center
	W, H float64 // Full width and height
}

// RectOverlap returns the overlap area between two rectangles.
// Returns 0 if they don't overlap.
func RectOverlap(a, b Rect) float64 {
	// Half dimensions
	aHalfW, aHalfH := a.W/2, a.H/2
	bHalfW, bHalfH := b.W/2, b.H/2

	// Check for separation
	dx := math.Abs(a.X - b.X)
	dy := math.Abs(a.Y - b.Y)

	overlapX := (aHalfW + bHalfW) - dx
	overlapY := (aHalfH + bHalfH) - dy

	if overlapX <= 0 || overlapY <= 0 {
		return 0
	}

	return overlapX * overlapY
}

// LabelPlacer manages label placement with collision avoidance.
type LabelPlacer struct {
	obstacles []Rect
}

// NewLabelPlacer creates a LabelPlacer with initial obstacles (states).
func NewLabelPlacer(states []Rect) *LabelPlacer {
	obstacles := make([]Rect, len(states))
	copy(obstacles, states)
	return &LabelPlacer{obstacles: obstacles}
}

// PlaceLabel finds the best position for a label near an anchor point.
// Returns the center position for the label.
func (lp *LabelPlacer) PlaceLabel(anchor Point, labelW, labelH, gap float64) Point {
	// Candidate positions (center of label at each position)
	candidates := []Point{
		// Priority 1: perpendicular to likely edge direction
		{anchor.X, anchor.Y - labelH/2 - gap},         // above
		{anchor.X, anchor.Y + labelH/2 + gap},         // below
		{anchor.X + labelW/2 + gap, anchor.Y},         // right
		{anchor.X - labelW/2 - gap, anchor.Y},         // left
		// Priority 2: diagonals
		{anchor.X + labelW/2 + gap, anchor.Y - labelH/2 - gap}, // top-right
		{anchor.X - labelW/2 - gap, anchor.Y - labelH/2 - gap}, // top-left
		{anchor.X + labelW/2 + gap, anchor.Y + labelH/2 + gap}, // bottom-right
		{anchor.X - labelW/2 - gap, anchor.Y + labelH/2 + gap}, // bottom-left
	}

	bestPos := candidates[0]
	bestOverlap := math.MaxFloat64

	for _, pos := range candidates {
		labelRect := Rect{pos.X, pos.Y, labelW, labelH}

		totalOverlap := 0.0
		for _, obs := range lp.obstacles {
			totalOverlap += RectOverlap(labelRect, obs)
		}

		if totalOverlap == 0 {
			// Found non-overlapping position
			lp.obstacles = append(lp.obstacles, labelRect)
			return pos
		}

		if totalOverlap < bestOverlap {
			bestOverlap = totalOverlap
			bestPos = pos
		}
	}

	// Use best available position (may have overlap)
	labelRect := Rect{bestPos.X, bestPos.Y, labelW, labelH}
	lp.obstacles = append(lp.obstacles, labelRect)
	return bestPos
}

// PlaceLabelOnEdge places a label along an edge, trying the midpoint first,
// then sliding along the edge to find a clear spot.
func (lp *LabelPlacer) PlaceLabelOnEdge(p1, p2 Point, labelW, labelH, gap float64) Point {
	// Start at midpoint
	mid := Point{(p1.X + p2.X) / 2, (p1.Y + p2.Y) / 2}

	// Perpendicular direction
	dx := p2.X - p1.X
	dy := p2.Y - p1.Y
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist < 1 {
		return lp.PlaceLabel(mid, labelW, labelH, gap)
	}

	perpX := -dy / dist
	perpY := dx / dist

	// Try positions offset perpendicular to edge
	offsets := []float64{gap, -gap, gap * 2, -gap * 2}

	for _, offset := range offsets {
		pos := Point{mid.X + perpX*offset, mid.Y + perpY*offset}
		labelRect := Rect{pos.X, pos.Y, labelW, labelH}

		hasOverlap := false
		for _, obs := range lp.obstacles {
			if RectOverlap(labelRect, obs) > 0 {
				hasOverlap = true
				break
			}
		}

		if !hasOverlap {
			lp.obstacles = append(lp.obstacles, labelRect)
			return pos
		}
	}

	// Fall back to standard placement
	return lp.PlaceLabel(mid, labelW, labelH, gap)
}

// PlaceLabelOnCurve places a label at a point on a Bézier curve,
// offset perpendicular to the curve tangent.
func (lp *LabelPlacer) PlaceLabelOnCurve(curvePoint, tangent Point, labelW, labelH, offset float64) Point {
	// Normalize tangent
	dist := math.Sqrt(tangent.X*tangent.X + tangent.Y*tangent.Y)
	if dist < 0.001 {
		return curvePoint
	}

	// Perpendicular to tangent
	perpX := -tangent.Y / dist
	perpY := tangent.X / dist

	// Try both sides
	for _, sign := range []float64{1, -1} {
		pos := Point{
			X: curvePoint.X + perpX*offset*sign,
			Y: curvePoint.Y + perpY*offset*sign,
		}
		labelRect := Rect{pos.X, pos.Y, labelW, labelH}

		hasOverlap := false
		for _, obs := range lp.obstacles {
			if RectOverlap(labelRect, obs) > 0 {
				hasOverlap = true
				break
			}
		}

		if !hasOverlap {
			lp.obstacles = append(lp.obstacles, labelRect)
			return pos
		}
	}

	// Fall back to first position
	pos := Point{
		X: curvePoint.X + perpX*offset,
		Y: curvePoint.Y + perpY*offset,
	}
	labelRect := Rect{pos.X, pos.Y, labelW, labelH}
	lp.obstacles = append(lp.obstacles, labelRect)
	return pos
}
