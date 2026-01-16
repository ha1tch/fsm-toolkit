package fsmfile

import (
	"fmt"
	"html"
	"math"
	"strings"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// StateShape defines the shape used for state nodes.
type StateShape int

const (
	ShapeCircle    StateShape = iota // Default circle/ellipse
	ShapeRoundRect                   // Rounded rectangle
	ShapeRect                        // Rectangle
	ShapeEllipse                     // Ellipse (wider than circle)
	ShapeDiamond                     // Diamond shape
)

// SVGOptions controls native SVG rendering.
type SVGOptions struct {
	Width       int        // canvas width in pixels
	Height      int        // canvas height in pixels
	Title       string     // diagram title
	FontSize    int        // base font size for state labels
	LabelSize   int        // font size for transition labels (0 = FontSize - 2)
	TitleSize   int        // font size for title (0 = FontSize + 4)
	StateRadius int        // radius of state circles (or half-height for other shapes)
	StateShape  StateShape // shape of state nodes
	Padding     int        // padding around edges
	NodeSpacing float64    // multiplier for spacing between nodes (default 1.0)
}

// DefaultSVGOptions returns sensible defaults.
func DefaultSVGOptions() SVGOptions {
	return SVGOptions{
		Width:       800,
		Height:      600,
		FontSize:    14,
		LabelSize:   0,  // will default to FontSize - 2
		TitleSize:   0,  // will default to FontSize + 4
		StateRadius: 30,
		StateShape:  ShapeEllipse,
		Padding:     50,
		NodeSpacing: 1.5, // more generous default spacing
	}
}

// GenerateSVGNative renders FSM to SVG without external dependencies.
// Uses the built-in layout algorithms.
func GenerateSVGNative(f *fsm.FSM, opts SVGOptions) string {
	if opts.Width == 0 {
		opts.Width = 800
	}
	if opts.Height == 0 {
		opts.Height = 600
	}
	if opts.FontSize == 0 {
		opts.FontSize = 14
	}
	if opts.LabelSize == 0 {
		opts.LabelSize = opts.FontSize - 2
	}
	if opts.TitleSize == 0 {
		opts.TitleSize = opts.FontSize + 4
	}
	if opts.StateRadius == 0 {
		opts.StateRadius = 30
	}
	if opts.Padding == 0 {
		opts.Padding = 50
	}
	if opts.NodeSpacing == 0 {
		opts.NodeSpacing = 1.5
	}

	// Get layout in terminal coordinates
	layoutW := (opts.Width - 2*opts.Padding) / 10
	layoutH := (opts.Height - 2*opts.Padding) / 20
	positions := SmartLayout(f, layoutW, layoutH)

	// First pass: calculate positions and find bounding box
	rawPos := make(map[string][2]float64)
	var minX, minY, maxX, maxY float64
	first := true
	
	for name, pos := range positions {
		x := float64(pos[0]) * 10.0 * opts.NodeSpacing
		y := float64(pos[1]) * 20.0 * opts.NodeSpacing
		rawPos[name] = [2]float64{x, y}
		
		// Calculate node extent based on label length
		// Must match the drawing code's dimension calculation
		// State labels use FontSize-2
		labelLen := len(name)
		effectiveFontSize := opts.FontSize - 2
		if effectiveFontSize < 10 {
			effectiveFontSize = 10
		}
		textWidth := float64(labelLen*effectiveFontSize) * 0.6
		nodeWidth := math.Max(float64(opts.StateRadius)*2, textWidth+40)
		nodeHeight := math.Max(float64(opts.StateRadius)*1.6, float64(effectiveFontSize)+24)
		
		// Update bounding box
		nodeMinX := x - nodeWidth/2
		nodeMaxX := x + nodeWidth/2
		nodeMinY := y - nodeHeight/2
		nodeMaxY := y + nodeHeight/2
		
		// Account for Moore outputs below state
		if f.Type == fsm.TypeMoore {
			if _, ok := f.StateOutputs[name]; ok {
				nodeMaxY += 20
			}
		}
		
		if first {
			minX, maxX = nodeMinX, nodeMaxX
			minY, maxY = nodeMinY, nodeMaxY
			first = false
		} else {
			if nodeMinX < minX { minX = nodeMinX }
			if nodeMaxX > maxX { maxX = nodeMaxX }
			if nodeMinY < minY { minY = nodeMinY }
			if nodeMaxY > maxY { maxY = nodeMaxY }
		}
	}
	
	// Account for initial state arrow (extends left of initial state)
	if f.Initial != "" {
		if pos, ok := rawPos[f.Initial]; ok {
			arrowStart := pos[0] - float64(opts.StateRadius) - 40
			if arrowStart < minX {
				minX = arrowStart
			}
		}
	}
	
	// Calculate content dimensions
	contentWidth := maxX - minX
	contentHeight := maxY - minY
	
	// Ensure minimum dimensions to avoid degenerate scaling
	if contentWidth < 50 {
		// Expand content width for centering narrow layouts
		centerX := (minX + maxX) / 2
		minX = centerX - 50
		maxX = centerX + 50
		contentWidth = 100
	}
	if contentHeight < 50 {
		centerY := (minY + maxY) / 2
		minY = centerY - 50
		maxY = centerY + 50
		contentHeight = 100
	}
	
	// Available space (canvas minus padding, and title space)
	titleSpace := 0.0
	if opts.Title != "" {
		titleSpace = 35
	}
	availableWidth := float64(opts.Width - 2*opts.Padding)
	availableHeight := float64(opts.Height - 2*opts.Padding) - titleSpace
	
	// Calculate scale to fit content in available space
	scaleX := availableWidth / contentWidth
	scaleY := availableHeight / contentHeight
	scale := math.Min(scaleX, scaleY)
	
	// Don't scale up too much (max 1.5x), and don't scale down below readable
	if scale > 1.5 {
		scale = 1.5
	}
	if scale < 0.3 {
		scale = 0.3
	}
	
	// Calculate offset to center the content
	scaledWidth := contentWidth * scale
	scaledHeight := contentHeight * scale
	offsetX := float64(opts.Padding) + (availableWidth-scaledWidth)/2 - minX*scale
	offsetY := float64(opts.Padding) + titleSpace + (availableHeight-scaledHeight)/2 - minY*scale
	
	// Second pass: apply scale and offset to get final positions
	svgPos := make(map[string][2]float64)
	for name, pos := range rawPos {
		x := pos[0]*scale + offsetX
		y := pos[1]*scale + offsetY
		svgPos[name] = [2]float64{x, y}
	}
	
	// Scale the radius too
	scaledRadius := float64(opts.StateRadius) * scale
	if scaledRadius < 15 {
		scaledRadius = 15 // minimum readable size
	}
	if scaledRadius > float64(opts.StateRadius)*1.5 {
		scaledRadius = float64(opts.StateRadius) * 1.5 // don't grow too much
	}

	// State label font is slightly smaller than base for better fit
	stateLabelSize := opts.FontSize - 2
	if stateLabelSize < 10 {
		stateLabelSize = 10
	}

	var sb strings.Builder

	// SVG header
	sb.WriteString(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">
<defs>
  <marker id="arrowhead" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
    <polygon points="0 0, 10 3.5, 0 7" fill="#333"/>
  </marker>
  <marker id="arrowhead-self" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
    <polygon points="0 0, 10 3.5, 0 7" fill="#666"/>
  </marker>
</defs>
<style>
  .state { fill: white; stroke: #333; stroke-width: 2; }
  .state-initial { fill: #e8f5e9; stroke: #2e7d32; stroke-width: 2; }
  .state-accepting { fill: #fff3e0; stroke: #e65100; stroke-width: 2; }
  .state-both { fill: #e3f2fd; stroke: #1565c0; stroke-width: 2; }
  .state-label { font-family: sans-serif; font-size: %dpx; text-anchor: middle; dominant-baseline: middle; }
  .transition { fill: none; stroke: #333; stroke-width: 1.5; marker-end: url(#arrowhead); }
  .transition-self { fill: none; stroke: #666; stroke-width: 1.5; marker-end: url(#arrowhead-self); }
  .trans-label { font-family: sans-serif; font-size: %dpx; fill: #333; }
  .title { font-family: sans-serif; font-size: %dpx; font-weight: bold; text-anchor: middle; }
  .moore-output { font-family: sans-serif; font-size: %dpx; fill: #666; font-style: italic; text-anchor: middle; }
</style>
`, opts.Width, opts.Height, opts.Width, opts.Height, stateLabelSize, opts.LabelSize, opts.TitleSize, opts.LabelSize))

	// Title
	if opts.Title != "" {
		sb.WriteString(fmt.Sprintf(`<text x="%d" y="25" class="title">%s</text>
`, opts.Width/2, html.EscapeString(opts.Title)))
	}

	// Background
	sb.WriteString(fmt.Sprintf(`<rect width="%d" height="%d" fill="white"/>
`, opts.Width, opts.Height))

	// Group transitions by from->to for label aggregation
	type transKey struct{ from, to string }
	transLabels := make(map[transKey][]string)
	for _, t := range f.Transitions {
		for _, to := range t.To {
			key := transKey{t.From, to}
			label := ""
			if t.Input != nil {
				label = *t.Input
			} else {
				label = "ε"
			}
			if f.Type == fsm.TypeMealy && t.Output != nil {
				label += "/" + *t.Output
			}
			transLabels[key] = append(transLabels[key], label)
		}
	}

	// Calculate graph centre for edge routing
	var sumX, sumY float64
	for _, pos := range svgPos {
		sumX += pos[0]
		sumY += pos[1]
	}
	graphCentreX := sumX / float64(len(svgPos))
	graphCentreY := sumY / float64(len(svgPos))

	// Draw transitions first (under states)
	drawnPairs := make(map[transKey]bool)
	for key, labels := range transLabels {
		if drawnPairs[key] {
			continue
		}

		fromPos := svgPos[key.from]
		toPos := svgPos[key.to]
		label := strings.Join(labels, ", ")

		if key.from == key.to {
			// Self-loop
			drawSelfLoop(&sb, fromPos[0], fromPos[1], scaledRadius, label, opts.LabelSize)
		} else {
			// Check for bidirectional
			reverseKey := transKey{key.to, key.from}
			reverseLabels, hasBidi := transLabels[reverseKey]

			if hasBidi && !drawnPairs[reverseKey] {
				// Draw curved bidirectional arrows
				drawBidiTransition(&sb, fromPos[0], fromPos[1], toPos[0], toPos[1],
					scaledRadius, label, strings.Join(reverseLabels, ", "), opts.LabelSize)
				drawnPairs[reverseKey] = true
			} else if !hasBidi {
				// Draw single-direction arrow
				drawTransition(&sb, fromPos[0], fromPos[1], toPos[0], toPos[1],
					scaledRadius, label, opts.LabelSize, graphCentreX, graphCentreY)
			}
		}
		drawnPairs[key] = true
	}

	// Draw initial arrow
	if f.Initial != "" {
		if pos, ok := svgPos[f.Initial]; ok {
			startX := pos[0] - scaledRadius - 30
			startY := pos[1]
			endX := pos[0] - scaledRadius - 2
			sb.WriteString(fmt.Sprintf(`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" class="transition"/>
`, startX, startY, endX, startY))
		}
	}

	// Draw states
	for _, name := range f.States {
		pos := svgPos[name]
		x, y := pos[0], pos[1]

		isInitial := name == f.Initial
		isAccepting := f.IsAccepting(name)

		class := "state"
		if isInitial && isAccepting {
			class = "state-both"
		} else if isInitial {
			class = "state-initial"
		} else if isAccepting {
			class = "state-accepting"
		}

		// Calculate dimensions based on label length and scaled radius
		labelLen := len(name)
		r := scaledRadius
		
		// Width: text width + generous horizontal padding
		// Text width ≈ len * stateLabelSize * 0.6 (average char width)
		// Add 40px padding (20 each side) for comfortable fit
		textWidth := float64(labelLen*stateLabelSize) * 0.6
		stateWidth := math.Max(r*2, textWidth+40)
		
		// Height: enough for text + vertical padding
		// Text height ≈ stateLabelSize, add padding for comfortable fit
		stateHeight := math.Max(r*1.6, float64(stateLabelSize)+24)

		// Draw shape based on option
		switch opts.StateShape {
		case ShapeRoundRect:
			rx := 8.0 * scale // corner radius scales too
			if rx < 4 { rx = 4 }
			sb.WriteString(fmt.Sprintf(`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="%.1f" class="%s"/>
`, x-stateWidth/2, y-stateHeight/2, stateWidth, stateHeight, rx, class))
			if isAccepting {
				sb.WriteString(fmt.Sprintf(`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="%.1f" class="%s" fill="none"/>
`, x-stateWidth/2+4, y-stateHeight/2+4, stateWidth-8, stateHeight-8, math.Max(rx-2, 2), class))
			}

		case ShapeRect:
			sb.WriteString(fmt.Sprintf(`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" class="%s"/>
`, x-stateWidth/2, y-stateHeight/2, stateWidth, stateHeight, class))
			if isAccepting {
				sb.WriteString(fmt.Sprintf(`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" class="%s" fill="none"/>
`, x-stateWidth/2+4, y-stateHeight/2+4, stateWidth-8, stateHeight-8, class))
			}

		case ShapeEllipse:
			rx := stateWidth / 2
			ry := stateHeight / 2
			sb.WriteString(fmt.Sprintf(`<ellipse cx="%.1f" cy="%.1f" rx="%.1f" ry="%.1f" class="%s"/>
`, x, y, rx, ry, class))
			if isAccepting {
				sb.WriteString(fmt.Sprintf(`<ellipse cx="%.1f" cy="%.1f" rx="%.1f" ry="%.1f" class="%s" fill="none"/>
`, x, y, rx-4, ry-4, class))
			}

		case ShapeDiamond:
			// Diamond as rotated square
			points := fmt.Sprintf("%.1f,%.1f %.1f,%.1f %.1f,%.1f %.1f,%.1f",
				x, y-stateHeight/2,          // top
				x+stateWidth/2, y,            // right
				x, y+stateHeight/2,          // bottom
				x-stateWidth/2, y)           // left
			sb.WriteString(fmt.Sprintf(`<polygon points="%s" class="%s"/>
`, points, class))
			if isAccepting {
				innerW := stateWidth - 12
				innerH := stateHeight - 12
				points2 := fmt.Sprintf("%.1f,%.1f %.1f,%.1f %.1f,%.1f %.1f,%.1f",
					x, y-innerH/2, x+innerW/2, y, x, y+innerH/2, x-innerW/2, y)
				sb.WriteString(fmt.Sprintf(`<polygon points="%s" class="%s" fill="none"/>
`, points2, class))
			}

		default: // ShapeCircle
			sb.WriteString(fmt.Sprintf(`<circle cx="%.1f" cy="%.1f" r="%.1f" class="%s"/>
`, x, y, scaledRadius, class))
			if isAccepting {
				sb.WriteString(fmt.Sprintf(`<circle cx="%.1f" cy="%.1f" r="%.1f" class="%s" fill="none"/>
`, x, y, scaledRadius-4, class))
			}
		}

		// State label
		sb.WriteString(fmt.Sprintf(`<text x="%.1f" y="%.1f" class="state-label">%s</text>
`, x, y, html.EscapeString(name)))

		// Moore output below state
		if f.Type == fsm.TypeMoore {
			if output, ok := f.StateOutputs[name]; ok && output != "" {
				sb.WriteString(fmt.Sprintf(`<text x="%.1f" y="%.1f" class="moore-output">%s</text>
`, x, y+stateHeight/2+15, html.EscapeString(output)))
			}
		}
	}

	sb.WriteString("</svg>\n")
	return sb.String()
}

func drawTransition(sb *strings.Builder, x1, y1, x2, y2, r float64, label string, fontSize int, graphCentreX, graphCentreY float64) {
	// Calculate start and end points on circle edges
	dx := x2 - x1
	dy := y2 - y1
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist < 1 {
		return
	}

	// Normalize
	nx := dx / dist
	ny := dy / dist

	// Start from edge of source circle
	sx := x1 + nx*r
	sy := y1 + ny*r

	// End at edge of target circle
	ex := x2 - nx*(r+2)
	ey := y2 - ny*(r+2)

	// For long edges or edges going "backwards" (up), use a curve
	isLongEdge := dist > r*4
	isBackEdge := dy < -r*2 // going significantly upward

	if isLongEdge || isBackEdge {
		// Use quadratic bezier curve
		// For back edges and long edges, curve AWAY from graph centre
		// This routes edges around the perimeter instead of through the middle
		
		// Midpoint of the edge
		midX := (x1 + x2) / 2
		midY := (y1 + y2) / 2
		
		// Vector from graph centre to edge midpoint
		toCentreX := midX - graphCentreX
		toCentreY := midY - graphCentreY
		
		// Perpendicular to the edge direction
		perpX := -ny
		perpY := nx
		
		// Choose perpendicular direction that points AWAY from centre
		// Dot product: if perp points towards centre, flip it
		dotProduct := perpX*toCentreX + perpY*toCentreY
		if dotProduct < 0 {
			// Perpendicular points toward centre, flip it
			perpX = ny
			perpY = -nx
		}
		
		// For back edges, use stronger curve
		curveAmount := dist * 0.2
		if isBackEdge {
			curveAmount = dist * 0.35
		}

		cx := midX + perpX*curveAmount
		cy := midY + perpY*curveAmount

		sb.WriteString(fmt.Sprintf(`<path d="M%.1f,%.1f Q%.1f,%.1f %.1f,%.1f" class="transition"/>
`, sx, sy, cx, cy, ex, ey))

		// Label near control point - small fixed offset, not proportional to curve
		// The control point is already offset from the edge, so only need a small nudge
		labelX := cx + perpX*8
		labelY := cy + perpY*8
		sb.WriteString(fmt.Sprintf(`<text x="%.1f" y="%.1f" class="trans-label" text-anchor="middle">%s</text>
`, labelX, labelY, html.EscapeString(label)))
	} else {
		// Straight line for short edges
		sb.WriteString(fmt.Sprintf(`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" class="transition"/>
`, sx, sy, ex, ey))

		// Label at midpoint, offset perpendicular to line
		mx := (sx + ex) / 2
		my := (sy + ey) / 2
		ox := -ny * 12
		oy := nx * 12

		sb.WriteString(fmt.Sprintf(`<text x="%.1f" y="%.1f" class="trans-label" text-anchor="middle">%s</text>
`, mx+ox, my+oy, html.EscapeString(label)))
	}
}

func drawBidiTransition(sb *strings.Builder, x1, y1, x2, y2, r float64, label1, label2 string, fontSize int) {
	dx := x2 - x1
	dy := y2 - y1
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist < 1 {
		return
	}

	nx := dx / dist
	ny := dy / dist

	// Perpendicular offset for curve
	px := -ny * 20
	py := nx * 20

	// Forward arrow (curved up)
	sx1 := x1 + nx*r
	sy1 := y1 + ny*r
	ex1 := x2 - nx*(r+2)
	ey1 := y2 - ny*(r+2)
	cx1 := (x1+x2)/2 + px
	cy1 := (y1+y2)/2 + py

	sb.WriteString(fmt.Sprintf(`<path d="M%.1f,%.1f Q%.1f,%.1f %.1f,%.1f" class="transition"/>
`, sx1, sy1, cx1, cy1, ex1, ey1))

	// Label for forward
	sb.WriteString(fmt.Sprintf(`<text x="%.1f" y="%.1f" class="trans-label" text-anchor="middle">%s</text>
`, cx1, cy1-5, html.EscapeString(label1)))

	// Reverse arrow (curved down)
	sx2 := x2 - nx*r
	sy2 := y2 - ny*r
	ex2 := x1 + nx*(r+2)
	ey2 := y1 + ny*(r+2)
	cx2 := (x1+x2)/2 - px
	cy2 := (y1+y2)/2 - py

	sb.WriteString(fmt.Sprintf(`<path d="M%.1f,%.1f Q%.1f,%.1f %.1f,%.1f" class="transition"/>
`, sx2, sy2, cx2, cy2, ex2, ey2))

	// Label for reverse
	sb.WriteString(fmt.Sprintf(`<text x="%.1f" y="%.1f" class="trans-label" text-anchor="middle">%s</text>
`, cx2, cy2+12, html.EscapeString(label2)))
}

func drawSelfLoop(sb *strings.Builder, x, y, r float64, label string, fontSize int) {
	// Draw self-loop as a curve above the state
	loopR := r * 0.6

	// Arc path for self-loop
	startAngle := math.Pi * 0.75
	endAngle := math.Pi * 0.25

	sx := x + r*math.Cos(startAngle+math.Pi/2)
	sy := y + r*math.Sin(startAngle+math.Pi/2)*-1

	// Use a bezier curve for the loop
	cx1 := x - loopR*1.5
	cy1 := y - r - loopR*2
	cx2 := x + loopR*1.5
	cy2 := y - r - loopR*2
	ex := x + r*math.Cos(endAngle+math.Pi/2)
	ey := y + r*math.Sin(endAngle+math.Pi/2)*-1

	sb.WriteString(fmt.Sprintf(`<path d="M%.1f,%.1f C%.1f,%.1f %.1f,%.1f %.1f,%.1f" class="transition-self"/>
`, sx, sy, cx1, cy1, cx2, cy2, ex, ey))

	// Label above loop
	sb.WriteString(fmt.Sprintf(`<text x="%.1f" y="%.1f" class="trans-label" text-anchor="middle">%s</text>
`, x, y-r-loopR*2-8, html.EscapeString(label)))
}
