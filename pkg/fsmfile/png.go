// Native PNG rendering for FSM diagrams.
// Mirrors the SVG renderer output using Go's image packages.

package fsmfile

import (
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"sort"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// PNGOptions configures PNG rendering.
type PNGOptions struct {
	Width       int
	Height      int
	Padding     int
	StateRadius int
	FontSize    int
	LabelSize   int
	NodeSpacing float64
	Title       string
}

// DefaultPNGOptions returns sensible defaults for PNG rendering.
func DefaultPNGOptions() PNGOptions {
	return PNGOptions{
		Width:       800,
		Height:      600,
		Padding:     50,
		StateRadius: 30,
		FontSize:    14,
		LabelSize:   12,
		NodeSpacing: 1.5,
		Title:       "",
	}
}

// Colors used in rendering
var (
	colorWhite      = color.RGBA{255, 255, 255, 255}
	colorBlack      = color.RGBA{51, 51, 51, 255}       // #333
	colorGray       = color.RGBA{102, 102, 102, 255}    // #666
	colorInitial    = color.RGBA{232, 245, 233, 255}    // #e8f5e9
	colorInitialBdr = color.RGBA{46, 125, 50, 255}      // #2e7d32
	colorAccepting  = color.RGBA{255, 243, 224, 255}    // #fff3e0
	colorAcceptBdr  = color.RGBA{230, 81, 0, 255}       // #e65100
	colorBoth       = color.RGBA{227, 242, 253, 255}    // #e3f2fd
	colorBothBdr    = color.RGBA{21, 101, 192, 255}     // #1565c0
)

// renderContext holds rendering parameters including scale
type renderContext struct {
	img       *image.RGBA
	scale     float64  // multiplier for line thickness, arrow size, etc.
	lineWidth float64  // base line width (scaled)
	fontSize  float64  // font size in points
	face      font.Face // font face for text rendering
}

func newRenderContext(img *image.RGBA, scale int) *renderContext {
	// Parse Go Regular font
	fnt, err := opentype.Parse(goregular.TTF)
	if err != nil {
		panic(err) // should never happen with embedded font
	}
	
	// Create face at scaled size (will be downsampled)
	// Base size 14pt, scaled by render scale
	fontSize := float64(14 * scale)
	face, err := opentype.NewFace(fnt, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     72,
		Hinting: font.HintingNone, // No hinting - we'll supersample instead
	})
	if err != nil {
		panic(err)
	}
	
	return &renderContext{
		img:       img,
		scale:     float64(scale),
		lineWidth: float64(scale) * 2,  // 2px base line width
		fontSize:  fontSize,
		face:      face,
	}
}

// RenderPNG renders an FSM to PNG format.
// Uses 4x supersampling for smoother output.
func RenderPNG(f *fsm.FSM, w io.Writer, opts PNGOptions) error {
	// Render at 4x size for supersampling
	scale := 4
	largeOpts := opts
	largeOpts.Width = opts.Width * scale
	largeOpts.Height = opts.Height * scale
	largeOpts.Padding = opts.Padding * scale
	largeOpts.StateRadius = opts.StateRadius * scale
	largeOpts.FontSize = opts.FontSize * scale
	largeOpts.LabelSize = opts.LabelSize * scale

	// Render large image with scale context
	largeImg := renderPNGInternal(f, largeOpts, scale)

	// Downsample to target size using high-quality interpolation
	finalImg := image.NewRGBA(image.Rect(0, 0, opts.Width, opts.Height))
	draw.CatmullRom.Scale(finalImg, finalImg.Bounds(), largeImg, largeImg.Bounds(), draw.Over, nil)

	return png.Encode(w, finalImg)
}

// renderPNGInternal renders the FSM to an image at the specified size.
func renderPNGInternal(f *fsm.FSM, opts PNGOptions, scale int) *image.RGBA {
	// Create image
	img := image.NewRGBA(image.Rect(0, 0, opts.Width, opts.Height))
	
	// Create render context with scale for line thickness etc.
	ctx := newRenderContext(img, scale)

	// Fill background white
	for y := 0; y < opts.Height; y++ {
		for x := 0; x < opts.Width; x++ {
			img.Set(x, y, colorWhite)
		}
	}

	// Get layout positions
	// Sugiyama produces hierarchical (typically tall) layouts
	// Adjust layout dimensions based on canvas aspect ratio
	canvasAspect := float64(opts.Width) / float64(opts.Height)
	
	var layoutWidth, layoutHeight int
	if canvasAspect > 1.3 {
		// Wide canvas: give layout more width to spread horizontally
		layoutWidth = opts.Width / 8
		layoutHeight = opts.Height / 15
	} else if canvasAspect < 0.7 {
		// Tall canvas: standard vertical layout
		layoutWidth = opts.Width / 12
		layoutHeight = opts.Height / 18
	} else {
		// Square-ish canvas
		layoutWidth = opts.Width / 10
		layoutHeight = opts.Height / 18
	}
	
	positions := SmartLayout(f, layoutWidth, layoutHeight)

	// Convert to pixel coordinates (same logic as SVG)
	rawPos := make(map[string][2]float64)
	var minX, minY, maxX, maxY float64
	first := true

	stateLabelSize := opts.FontSize - 2
	if stateLabelSize < 10 {
		stateLabelSize = 10
	}

	for name, pos := range positions {
		x := float64(pos[0]) * 10.0 * opts.NodeSpacing
		y := float64(pos[1]) * 20.0 * opts.NodeSpacing
		rawPos[name] = [2]float64{x, y}

		labelLen := len(name)
		textWidth := float64(labelLen*stateLabelSize) * 0.6
		nodeWidth := math.Max(float64(opts.StateRadius)*2, textWidth+40)
		nodeHeight := math.Max(float64(opts.StateRadius)*1.0, float64(stateLabelSize)+16)

		nodeMinX := x - nodeWidth/2
		nodeMaxX := x + nodeWidth/2
		nodeMinY := y - nodeHeight/2
		nodeMaxY := y + nodeHeight/2

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
			if nodeMinX < minX {
				minX = nodeMinX
			}
			if nodeMaxX > maxX {
				maxX = nodeMaxX
			}
			if nodeMinY < minY {
				minY = nodeMinY
			}
			if nodeMaxY > maxY {
				maxY = nodeMaxY
			}
		}
	}

	// Initial arrow extent
	if f.Initial != "" {
		if pos, ok := rawPos[f.Initial]; ok {
			arrowStart := pos[0] - float64(opts.StateRadius) - 40
			if arrowStart < minX {
				minX = arrowStart
			}
		}
	}

	contentWidth := maxX - minX
	contentHeight := maxY - minY

	if contentWidth < 50 {
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

	titleSpace := 0.0
	if opts.Title != "" {
		titleSpace = 35
	}
	availableWidth := float64(opts.Width - 2*opts.Padding)
	availableHeight := float64(opts.Height-2*opts.Padding) - titleSpace

	scaleX := availableWidth / contentWidth
	scaleY := availableHeight / contentHeight
	fitScale := math.Min(scaleX, scaleY)

	// Allow more generous scaling for small graphs
	if fitScale > 2.0 {
		fitScale = 2.0
	}
	if fitScale < 0.2 {
		fitScale = 0.2
	}

	scaledWidth := contentWidth * fitScale
	scaledHeight := contentHeight * fitScale
	
	// Perfect centering: compute offset to place scaled content in center of available area
	offsetX := float64(opts.Padding) + (availableWidth-scaledWidth)/2 - minX*fitScale
	offsetY := float64(opts.Padding) + titleSpace + (availableHeight-scaledHeight)/2 - minY*fitScale

	// Final positions
	pngPos := make(map[string][2]float64)
	for name, pos := range rawPos {
		x := pos[0]*fitScale + offsetX
		y := pos[1]*fitScale + offsetY
		pngPos[name] = [2]float64{x, y}
	}

	scaledRadius := float64(opts.StateRadius) * fitScale
	if scaledRadius < 15 {
		scaledRadius = 15
	}
	if scaledRadius > float64(opts.StateRadius)*1.5 {
		scaledRadius = float64(opts.StateRadius) * 1.5
	}

	// Calculate ellipse dimensions for each state (for arc endpoint calculation)
	ellipseDims := make(map[string][2]float64) // [rx, ry]
	for name := range pngPos {
		labelLen := len(name)
		textWidth := float64(labelLen) * ctx.fontSize * 0.6
		stateWidth := math.Max(scaledRadius*2, textWidth+40*ctx.scale)
		stateHeight := math.Max(scaledRadius*1.0, ctx.fontSize+16*ctx.scale)
		ellipseDims[name] = [2]float64{stateWidth / 2, stateHeight / 2}
	}

	// Resolve horizontal overlaps
	// Group states by Y position (same row within tolerance)
	yTolerance := 20.0 * ctx.scale
	rows := make(map[int][]string)
	for name, pos := range pngPos {
		rowKey := int(pos[1] / yTolerance)
		rows[rowKey] = append(rows[rowKey], name)
	}
	
	// For each row, sort by X and push apart overlapping ellipses
	for _, row := range rows {
		if len(row) <= 1 {
			continue
		}
		// Sort by X position
		sort.Slice(row, func(i, j int) bool {
			return pngPos[row[i]][0] < pngPos[row[j]][0]
		})
		// Push apart if overlapping
		for i := 1; i < len(row); i++ {
			prev := row[i-1]
			curr := row[i]
			prevRx := ellipseDims[prev][0]
			currRx := ellipseDims[curr][0]
			minGap := prevRx + currRx + 40*ctx.scale // 40px padding between ellipses
			actualGap := pngPos[curr][0] - pngPos[prev][0]
			if actualGap < minGap {
				// Push current node right
				newX := pngPos[prev][0] + minGap
				pngPos[curr] = [2]float64{newX, pngPos[curr][1]}
			}
		}
	}

	// Re-center after overlap resolution
	// Recalculate actual bounds
	var finalMinX, finalMaxX, finalMinY, finalMaxY float64
	firstFinal := true
	for name, pos := range pngPos {
		dims := ellipseDims[name]
		nodeMinX := pos[0] - dims[0]
		nodeMaxX := pos[0] + dims[0]
		nodeMinY := pos[1] - dims[1]
		nodeMaxY := pos[1] + dims[1]
		
		if firstFinal {
			finalMinX, finalMaxX = nodeMinX, nodeMaxX
			finalMinY, finalMaxY = nodeMinY, nodeMaxY
			firstFinal = false
		} else {
			if nodeMinX < finalMinX { finalMinX = nodeMinX }
			if nodeMaxX > finalMaxX { finalMaxX = nodeMaxX }
			if nodeMinY < finalMinY { finalMinY = nodeMinY }
			if nodeMaxY > finalMaxY { finalMaxY = nodeMaxY }
		}
	}
	
	// Target center of available area
	targetCenterX := float64(opts.Padding) + availableWidth/2
	targetCenterY := float64(opts.Padding) + titleSpace + availableHeight/2
	
	// Current center of content
	currentCenterX := (finalMinX + finalMaxX) / 2
	currentCenterY := (finalMinY + finalMaxY) / 2
	
	// Shift all positions to perfectly center
	shiftX := targetCenterX - currentCenterX
	shiftY := targetCenterY - currentCenterY
	
	for name := range pngPos {
		pngPos[name] = [2]float64{pngPos[name][0] + shiftX, pngPos[name][1] + shiftY}
	}

	// Calculate graph centre for edge routing
	var sumX, sumY float64
	for _, pos := range pngPos {
		sumX += pos[0]
		sumY += pos[1]
	}
	graphCentreX := sumX / float64(len(pngPos))
	graphCentreY := sumY / float64(len(pngPos))

	// Draw title
	if opts.Title != "" {
		drawTextCentered(ctx, opts.Width/2, 25*scale, opts.Title, colorBlack)
	}

	// Collect transitions
	type transKey struct {
		from, to string
	}
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

	// Draw transitions
	// First, build state obstacles for LabelPlacer
	var stateRects []Rect
	var stateEllipses []Ellipse // For obstacle-aware routing
	for name, pos := range pngPos {
		dims := ellipseDims[name]
		stateRects = append(stateRects, Rect{
			X: pos[0], Y: pos[1],
			W: dims[0] * 2, H: dims[1] * 2,
		})
		stateEllipses = append(stateEllipses, Ellipse{
			CX: pos[0], CY: pos[1],
			RX: dims[0] + 5*ctx.scale, // Add padding for routing clearance
			RY: dims[1] + 5*ctx.scale,
		})
	}
	labelPlacer := NewLabelPlacer(stateRects)

	// First pass: draw non-self-loop transitions
	var labelBoxes []labelBox
	drawnPairs := make(map[transKey]bool)
	var selfLoops []struct {
		x, y, rx, ry float64
		label        string
	}

	for key, labels := range transLabels {
		if drawnPairs[key] {
			continue
		}

		fromPos := pngPos[key.from]
		toPos := pngPos[key.to]
		label := strings.Join(labels, ", ")
		fromDims := ellipseDims[key.from]
		toDims := ellipseDims[key.to]

		if key.from == key.to {
			// Defer self-loops to second pass
			selfLoops = append(selfLoops, struct {
				x, y, rx, ry float64
				label        string
			}{fromPos[0], fromPos[1], fromDims[0], fromDims[1], label})
		} else {
			reverseKey := transKey{key.to, key.from}
			reverseLabels, hasBidi := transLabels[reverseKey]

			// Check if this is a back-edge that should use routed path
			dy := toPos[1] - fromPos[1]
			avgR := (fromDims[0] + fromDims[1] + toDims[0] + toDims[1]) / 4
			isBackEdge := dy < -avgR*2

			if hasBidi && !drawnPairs[reverseKey] {
				lx, ly := drawBidiTransitionPNGWithPlacer(ctx, fromPos[0], fromPos[1], toPos[0], toPos[1],
					fromDims, toDims, label, strings.Join(reverseLabels, ", "), labelPlacer)
				labelBoxes = append(labelBoxes, labelBox{lx, ly, 50 * ctx.scale, 15 * ctx.scale})
				drawnPairs[reverseKey] = true
			} else if !hasBidi {
				if isBackEdge {
					// Use obstacle-aware routing for back-edges
					// Build list of obstacles excluding source and target
					var routingObstacles []Ellipse
					for name, pos := range pngPos {
						if name == key.from || name == key.to {
							continue
						}
						dims := ellipseDims[name]
						routingObstacles = append(routingObstacles, Ellipse{
							CX: pos[0], CY: pos[1],
							RX: dims[0] + 8*ctx.scale,
							RY: dims[1] + 8*ctx.scale,
						})
					}
					lx, ly := drawTransitionWithRouting(ctx, fromPos[0], fromPos[1], toPos[0], toPos[1],
						fromDims, toDims, label, routingObstacles, labelPlacer)
					labelBoxes = append(labelBoxes, labelBox{lx, ly, 50 * ctx.scale, 15 * ctx.scale})
				} else {
					lx, ly := drawTransitionPNGWithPlacer(ctx, fromPos[0], fromPos[1], toPos[0], toPos[1],
						fromDims, toDims, label, graphCentreX, graphCentreY, labelPlacer)
					labelBoxes = append(labelBoxes, labelBox{lx, ly, 50 * ctx.scale, 15 * ctx.scale})
				}
			}
		}
		drawnPairs[key] = true
	}

	// Second pass: draw self-loops with smart label placement
	canvasW := float64(opts.Width)
	canvasH := float64(opts.Height)
	for _, loop := range selfLoops {
		drawSelfLoopPNG(ctx, loop.x, loop.y, loop.rx, loop.ry, loop.label, labelBoxes, graphCentreY, canvasW, canvasH)
	}

	// Draw initial arrow
	if f.Initial != "" {
		if pos, ok := pngPos[f.Initial]; ok {
			dims := ellipseDims[f.Initial]
			rx := dims[0]
			startX := pos[0] - rx - 30*ctx.scale
			endX := pos[0] - rx - 2*ctx.scale
			drawArrowLine(ctx, startX, pos[1], endX, pos[1], colorBlack)
		}
	}

	// Draw states
	for _, name := range f.States {
		pos := pngPos[name]
		x, y := pos[0], pos[1]

		isInitial := name == f.Initial
		isAccepting := f.IsAccepting(name)

		// Determine colors
		fillColor := colorWhite
		borderColor := colorBlack
		if isInitial && isAccepting {
			fillColor = colorBoth
			borderColor = colorBothBdr
		} else if isInitial {
			fillColor = colorInitial
			borderColor = colorInitialBdr
		} else if isAccepting {
			fillColor = colorAccepting
			borderColor = colorAcceptBdr
		}

		// Calculate dimensions
		labelLen := len(name)
		textWidth := float64(labelLen*stateLabelSize) * 0.6
		stateWidth := math.Max(scaledRadius*2, textWidth+40*ctx.scale)
		stateHeight := math.Max(scaledRadius*1.0, float64(stateLabelSize)+16*ctx.scale)

		// Draw ellipse
		drawEllipse(ctx, x, y, stateWidth/2, stateHeight/2, fillColor, borderColor)

		// Draw inner ellipse for accepting states
		if isAccepting {
			drawEllipse(ctx, x, y, stateWidth/2-4*ctx.scale, stateHeight/2-4*ctx.scale, color.Transparent, borderColor)
		}

		// Draw label
		drawTextCentered(ctx, int(x), int(y)+int(4*ctx.scale), name, colorBlack)

		// Draw Moore output
		if f.Type == fsm.TypeMoore {
			if output, ok := f.StateOutputs[name]; ok {
				drawTextCentered(ctx, int(x), int(y+stateHeight/2+12*ctx.scale), "/"+output, colorGray)
			}
		}
	}

	return img
}

// drawEllipse draws an ellipse outline and optional fill.
func drawEllipse(ctx *renderContext, cx, cy, rx, ry float64, fill, stroke color.Color) {
	img := ctx.img
	thickness := ctx.lineWidth
	
	// Fill interior first
	if fill != color.Transparent {
		for dy := -ry; dy <= ry; dy++ {
			yNorm := dy / ry
			if yNorm*yNorm <= 1 {
				xExtent := rx * math.Sqrt(1-yNorm*yNorm)
				for dx := -xExtent; dx <= xExtent; dx++ {
					img.Set(int(cx+dx), int(cy+dy), fill)
				}
			}
		}
	}
	
	// Draw thick outline
	for angle := 0.0; angle < 2*math.Pi; angle += 0.005 {
		x := cx + rx*math.Cos(angle)
		y := cy + ry*math.Sin(angle)
		
		// Draw with thickness
		for t := -thickness / 2; t <= thickness/2; t += 0.5 {
			// Offset along the normal (radial direction)
			nx := math.Cos(angle)
			ny := math.Sin(angle)
			img.Set(int(x+nx*t), int(y+ny*t), stroke)
		}
	}
}

// drawLine draws a line between two points with thickness from context.
func drawLine(ctx *renderContext, x1, y1, x2, y2 float64, c color.Color) {
	img := ctx.img
	thickness := ctx.lineWidth
	
	dx := x2 - x1
	dy := y2 - y1
	steps := math.Max(math.Abs(dx), math.Abs(dy))
	if steps < 1 {
		steps = 1
	}

	halfThick := thickness / 2
	
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist < 1 {
		for ty := -halfThick; ty <= halfThick; ty++ {
			for tx := -halfThick; tx <= halfThick; tx++ {
				img.Set(int(x1+tx), int(y1+ty), c)
			}
		}
		return
	}
	
	perpX := -dy / dist
	perpY := dx / dist

	for i := 0.0; i <= steps; i++ {
		t := i / steps
		cx := x1 + dx*t
		cy := y1 + dy*t
		
		for offset := -halfThick; offset <= halfThick; offset += 0.5 {
			img.Set(int(cx+perpX*offset), int(cy+perpY*offset), c)
		}
	}
}

// drawArrowLine draws a line with an arrowhead at the end.
func drawArrowLine(ctx *renderContext, x1, y1, x2, y2 float64, c color.Color) {
	drawLine(ctx, x1, y1, x2, y2, c)

	// Arrowhead - scale with context
	dx := x2 - x1
	dy := y2 - y1
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist < 1 {
		return
	}

	nx := dx / dist
	ny := dy / dist

	// Arrow wings - scale with context
	arrowLen := 8.0 * ctx.scale
	arrowWidth := 4.0 * ctx.scale

	ax1 := x2 - nx*arrowLen + ny*arrowWidth
	ay1 := y2 - ny*arrowLen - nx*arrowWidth
	ax2 := x2 - nx*arrowLen - ny*arrowWidth
	ay2 := y2 - ny*arrowLen + nx*arrowWidth

	drawLine(ctx, x2, y2, ax1, ay1, c)
	drawLine(ctx, x2, y2, ax2, ay2, c)

	// Fill arrowhead
	for t := 0.0; t <= 1.0; t += 0.05 {
		mx := ax1 + (ax2-ax1)*t
		my := ay1 + (ay2-ay1)*t
		drawLine(ctx, x2, y2, mx, my, c)
	}
}

// drawQuadBezier draws a quadratic Bezier curve.
func drawQuadBezier(ctx *renderContext, x1, y1, cx, cy, x2, y2 float64, c color.Color) {
	steps := 100.0
	var prevX, prevY float64

	for i := 0.0; i <= steps; i++ {
		t := i / steps
		x := (1-t)*(1-t)*x1 + 2*(1-t)*t*cx + t*t*x2
		y := (1-t)*(1-t)*y1 + 2*(1-t)*t*cy + t*t*y2

		if i > 0 {
			drawLine(ctx, prevX, prevY, x, y, c)
		}
		prevX, prevY = x, y
	}
}

// drawCubicBezier draws a cubic Bézier curve.
func drawCubicBezier(ctx *renderContext, p0, p1, p2, p3 Point, c color.Color) {
	steps := 100.0
	var prevX, prevY float64

	for i := 0.0; i <= steps; i++ {
		t := i / steps
		t2 := t * t
		t3 := t2 * t
		mt := 1 - t
		mt2 := mt * mt
		mt3 := mt2 * mt

		x := mt3*p0.X + 3*mt2*t*p1.X + 3*mt*t2*p2.X + t3*p3.X
		y := mt3*p0.Y + 3*mt2*t*p1.Y + 3*mt*t2*p2.Y + t3*p3.Y

		if i > 0 {
			drawLine(ctx, prevX, prevY, x, y, c)
		}
		prevX, prevY = x, y
	}
}

// drawQuadBezierArrow draws a quadratic Bezier curve with arrowhead.
func drawQuadBezierArrow(ctx *renderContext, x1, y1, cx, cy, x2, y2 float64, c color.Color) {
	drawQuadBezier(ctx, x1, y1, cx, cy, x2, y2, c)

	// Calculate tangent at end for arrowhead direction
	tx := x2 - cx
	ty := y2 - cy
	dist := math.Sqrt(tx*tx + ty*ty)
	if dist < 1 {
		return
	}

	nx := tx / dist
	ny := ty / dist

	arrowLen := 8.0 * ctx.scale
	arrowWidth := 4.0 * ctx.scale

	ax1 := x2 - nx*arrowLen + ny*arrowWidth
	ay1 := y2 - ny*arrowLen - nx*arrowWidth
	ax2 := x2 - nx*arrowLen - ny*arrowWidth
	ay2 := y2 - ny*arrowLen + nx*arrowWidth

	drawLine(ctx, x2, y2, ax1, ay1, c)
	drawLine(ctx, x2, y2, ax2, ay2, c)
	for t := 0.0; t <= 1.0; t += 0.05 {
		mx := ax1 + (ax2-ax1)*t
		my := ay1 + (ay2-ay1)*t
		drawLine(ctx, x2, y2, mx, my, c)
	}
}

// drawTextCentered draws text centered at the given position using Go Regular font.
func drawTextCentered(ctx *renderContext, x, y int, text string, c color.Color) {
	// Measure text width
	width := font.MeasureString(ctx.face, text).Ceil()
	
	// Calculate baseline position
	// y is the vertical centre point of the ellipse (Y increases downward)
	// Text was appearing too LOW - we need to move it UP (smaller Y)
	// The baseline is where letter bottoms sit; text extends UP by ascent
	// For visual centering of caps: baseline = y + (capHeight / 2)
	// Cap height ≈ 0.7 * ascent
	// So baseline ≈ y + 0.35 * ascent for true centering
	// Text was too low, so reduce this offset further
	metrics := ctx.face.Metrics()
	ascent := metrics.Ascent.Ceil()
	
	baselineY := y + int(float64(ascent)*0.15)
	
	point := fixed.Point26_6{
		X: fixed.I(x - width/2),
		Y: fixed.I(baselineY),
	}

	d := &font.Drawer{
		Dst:  ctx.img,
		Src:  image.NewUniform(c),
		Face: ctx.face,
		Dot:  point,
	}
	d.DrawString(text)
}

// ellipseEdgePoint calculates the point on an ellipse edge in a given direction.
// cx, cy: centre; rx, ry: semi-axes; nx, ny: normalised direction
func ellipseEdgePoint(cx, cy, rx, ry, nx, ny float64) (float64, float64) {
	// For ellipse (x/rx)² + (y/ry)² = 1, point in direction (nx, ny) is at:
	// t = 1 / sqrt((nx/rx)² + (ny/ry)²)
	t := 1.0 / math.Sqrt((nx*nx)/(rx*rx)+(ny*ny)/(ry*ry))
	return cx + nx*t, cy + ny*t
}

// drawTransitionPNG draws a single transition arrow and returns the label position.
func drawTransitionPNG(ctx *renderContext, x1, y1, x2, y2 float64, fromDims, toDims [2]float64, label string, graphCentreX, graphCentreY float64) (float64, float64) {
	dx := x2 - x1
	dy := y2 - y1
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist < 1 {
		return x1, y1
	}

	nx := dx / dist
	ny := dy / dist

	// Calculate start point on source ellipse edge
	sx, sy := ellipseEdgePoint(x1, y1, fromDims[0], fromDims[1], nx, ny)
	
	// Calculate end point on target ellipse edge (with small gap for arrow)
	ex, ey := ellipseEdgePoint(x2, y2, toDims[0]+2*ctx.scale, toDims[1]+2*ctx.scale, -nx, -ny)

	// Use average radius for distance comparisons
	avgR := (fromDims[0] + fromDims[1] + toDims[0] + toDims[1]) / 4
	isLongEdge := dist > avgR*4
	isBackEdge := dy < -avgR*2

	var labelX, labelY float64

	if isLongEdge || isBackEdge {
		midX := (x1 + x2) / 2
		midY := (y1 + y2) / 2

		toCentreX := midX - graphCentreX
		toCentreY := midY - graphCentreY

		perpX := -ny
		perpY := nx

		dotProduct := perpX*toCentreX + perpY*toCentreY
		if dotProduct < 0 {
			perpX = ny
			perpY = -nx
		}

		curveAmount := dist * 0.2
		if isBackEdge {
			curveAmount = dist * 0.35
		}

		cx := midX + perpX*curveAmount
		cy := midY + perpY*curveAmount

		drawQuadBezierArrow(ctx, sx, sy, cx, cy, ex, ey, colorBlack)

		// Place label on the curve at t=0.5, not at the control point
		// Quadratic Bezier at t=0.5: B(0.5) = 0.25*P0 + 0.5*P1 + 0.25*P2
		curveMidX := 0.25*sx + 0.5*cx + 0.25*ex
		curveMidY := 0.25*sy + 0.5*cy + 0.25*ey
		// Small offset perpendicular to curve for label readability
		labelOffset := 10.0 * ctx.scale
		labelX = curveMidX + perpX*labelOffset
		labelY = curveMidY + perpY*labelOffset
		drawTextCentered(ctx, int(labelX), int(labelY), label, colorBlack)
	} else {
		drawArrowLine(ctx, sx, sy, ex, ey, colorBlack)

		mx := (sx + ex) / 2
		my := (sy + ey) / 2
		ox := -ny * 12 * ctx.scale
		oy := nx * 12 * ctx.scale

		labelX = mx + ox
		labelY = my + oy
		drawTextCentered(ctx, int(labelX), int(labelY), label, colorBlack)
	}
	return labelX, labelY
}

// drawTransitionPNGWithPlacer draws a transition with collision-aware label placement.
func drawTransitionPNGWithPlacer(ctx *renderContext, x1, y1, x2, y2 float64, fromDims, toDims [2]float64, label string, graphCentreX, graphCentreY float64, placer *LabelPlacer) (float64, float64) {
	dx := x2 - x1
	dy := y2 - y1
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist < 1 {
		return x1, y1
	}

	nx := dx / dist
	ny := dy / dist

	// Calculate start point on source ellipse edge
	sx, sy := ellipseEdgePoint(x1, y1, fromDims[0], fromDims[1], nx, ny)

	// Calculate end point on target ellipse edge (with small gap for arrow)
	ex, ey := ellipseEdgePoint(x2, y2, toDims[0]+2*ctx.scale, toDims[1]+2*ctx.scale, -nx, -ny)

	// Use average radius for distance comparisons
	avgR := (fromDims[0] + fromDims[1] + toDims[0] + toDims[1]) / 4
	isLongEdge := dist > avgR*4
	isBackEdge := dy < -avgR*2

	labelW := float64(len(label)) * ctx.fontSize * 0.6
	labelH := ctx.fontSize
	gap := 8.0 * ctx.scale

	var labelX, labelY float64

	if isLongEdge || isBackEdge {
		midX := (x1 + x2) / 2
		midY := (y1 + y2) / 2

		toCentreX := midX - graphCentreX
		toCentreY := midY - graphCentreY

		perpX := -ny
		perpY := nx

		dotProduct := perpX*toCentreX + perpY*toCentreY
		if dotProduct < 0 {
			perpX = ny
			perpY = -nx
		}

		curveAmount := dist * 0.2
		if isBackEdge {
			curveAmount = dist * 0.35
		}

		cx := midX + perpX*curveAmount
		cy := midY + perpY*curveAmount

		drawQuadBezierArrow(ctx, sx, sy, cx, cy, ex, ey, colorBlack)

		// Place label on the curve at t=0.5 using collision avoidance
		curveMidX := 0.25*sx + 0.5*cx + 0.25*ex
		curveMidY := 0.25*sy + 0.5*cy + 0.25*ey

		// Use LabelPlacer to find best position
		labelPos := placer.PlaceLabelOnCurve(
			Point{curveMidX, curveMidY},
			Point{perpX, perpY},
			labelW, labelH, gap,
		)
		labelX, labelY = labelPos.X, labelPos.Y
		drawTextCentered(ctx, int(labelX), int(labelY), label, colorBlack)
	} else {
		drawArrowLine(ctx, sx, sy, ex, ey, colorBlack)

		// Use LabelPlacer for label position
		labelPos := placer.PlaceLabelOnEdge(
			Point{sx, sy}, Point{ex, ey},
			labelW, labelH, gap,
		)
		labelX, labelY = labelPos.X, labelPos.Y
		drawTextCentered(ctx, int(labelX), int(labelY), label, colorBlack)
	}
	return labelX, labelY
}

// drawTransitionWithRouting draws a transition using obstacle-aware routing.
// Uses visibility graph to find waypoints, then fits a smooth quadratic curve
// guided by those waypoints (rather than passing exactly through each one).
func drawTransitionWithRouting(ctx *renderContext, x1, y1, x2, y2 float64, fromDims, toDims [2]float64, label string, obstacles []Ellipse, placer *LabelPlacer) (float64, float64) {
	dx := x2 - x1
	dy := y2 - y1
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist < 1 {
		return x1, y1
	}

	nx := dx / dist
	ny := dy / dist

	// Calculate start point on source ellipse edge
	sx, sy := ellipseEdgePoint(x1, y1, fromDims[0], fromDims[1], nx, ny)

	// Calculate end point on target ellipse edge (with small gap for arrow)
	ex, ey := ellipseEdgePoint(x2, y2, toDims[0]+2*ctx.scale, toDims[1]+2*ctx.scale, -nx, -ny)

	labelW := float64(len(label)) * ctx.fontSize * 0.6
	labelH := ctx.fontSize
	gap := 8.0 * ctx.scale

	// Try to route around obstacles
	start := Point{sx, sy}
	end := Point{ex, ey}
	path := RouteAroundObstacles(start, end, obstacles)

	var labelX, labelY float64
	var cx, cy float64 // Control point for the curve

	// Perpendicular direction (for curving away from direct line)
	perpX := -ny
	perpY := nx

	// Default curve parameters
	midX := (x1 + x2) / 2
	midY := (y1 + y2) / 2
	curveAmount := dist * 0.35

	if len(path) > 2 {
		// Path has waypoints - find the one that deviates most from direct line
		// This guides where our smooth curve should arc
		maxDev := 0.0
		var guideX, guideY float64

		for i := 1; i < len(path)-1; i++ {
			wp := path[i]
			// Calculate perpendicular distance from waypoint to direct line
			// Project waypoint onto line from start to end
			t := ((wp.X-sx)*(ex-sx) + (wp.Y-sy)*(ey-sy)) / ((ex-sx)*(ex-sx) + (ey-sy)*(ey-sy))
			// Closest point on line
			closestX := sx + t*(ex-sx)
			closestY := sy + t*(ey-sy)
			// Distance from waypoint to line
			dev := math.Sqrt(math.Pow(wp.X-closestX, 2) + math.Pow(wp.Y-closestY, 2))

			if dev > maxDev {
				maxDev = dev
				guideX = wp.X
				guideY = wp.Y
			}
		}

		// If we found a significant deviation, use it to guide the curve
		if maxDev > dist*0.08 {
			// Vector from midpoint of line to guide point
			toGuideX := guideX - midX
			toGuideY := guideY - midY
			toGuideDist := math.Sqrt(toGuideX*toGuideX + toGuideY*toGuideY)

			if toGuideDist > 1 {
				// The control point should be in the direction of the guide,
				// but pushed out enough for a smooth curve
				// Use the larger of: standard curve amount or guide distance * 1.3
				effectiveCurve := math.Max(curveAmount, toGuideDist*1.3)

				cx = midX + (toGuideX/toGuideDist)*effectiveCurve
				cy = midY + (toGuideY/toGuideDist)*effectiveCurve
			} else {
				// Guide is at midpoint, use standard perpendicular curve
				cx = midX + perpX*curveAmount
				cy = midY + perpY*curveAmount
			}
		} else {
			// Waypoints don't deviate much, use standard curve
			cx = midX + perpX*curveAmount
			cy = midY + perpY*curveAmount
		}
	} else {
		// Direct path or minimal waypoints - use standard curve
		cx = midX + perpX*curveAmount
		cy = midY + perpY*curveAmount
	}

	// Draw the smooth quadratic Bézier
	drawQuadBezierArrow(ctx, sx, sy, cx, cy, ex, ey, colorBlack)

	// Place label on the curve at t=0.5
	curveMidX := 0.25*sx + 0.5*cx + 0.25*ex
	curveMidY := 0.25*sy + 0.5*cy + 0.25*ey

	// Calculate perpendicular at curve midpoint for label placement
	tangentX := (ex - sx)
	tangentY := (ey - sy)
	tangentDist := math.Sqrt(tangentX*tangentX + tangentY*tangentY)

	var labelPerpX, labelPerpY float64
	if tangentDist > 0 {
		labelPerpX = -tangentY / tangentDist
		labelPerpY = tangentX / tangentDist
	} else {
		labelPerpX = perpX
		labelPerpY = perpY
	}

	labelPos := placer.PlaceLabelOnCurve(
		Point{curveMidX, curveMidY},
		Point{labelPerpX, labelPerpY},
		labelW, labelH, gap,
	)
	labelX, labelY = labelPos.X, labelPos.Y

	drawTextCentered(ctx, int(labelX), int(labelY), label, colorBlack)
	return labelX, labelY
}

// drawPathWithArrow draws a path as a smooth curve with an arrowhead at the end.
// Converts waypoints to a smooth cubic Bézier spline using Catmull-Rom interpolation.
func drawPathWithArrow(ctx *renderContext, path []Point, c color.Color) {
	if len(path) < 2 {
		return
	}

	if len(path) == 2 {
		// Just two points - draw a straight line with arrow
		drawArrowLine(ctx, path[0].X, path[0].Y, path[1].X, path[1].Y, c)
		return
	}

	// For paths with few waypoints (3-4), add intermediate points to ensure smooth curves
	// This prevents the "kinked" appearance near endpoints
	smoothedPath := path
	if len(path) <= 4 {
		smoothedPath = addIntermediatePoints(path)
	}

	// Convert waypoints to smooth cubic Bézier using Catmull-Rom
	// For each segment, we need 4 control points
	smoothPath := waypointsToSmoothCurve(smoothedPath)
	
	// Draw the smooth curve
	if len(smoothPath) >= 4 {
		// Draw cubic Bézier segments
		// smoothPath has format: [P0, C1, C2, P1, C3, C4, P2, ...]
		// So for n waypoints, we have (n-1) segments, each using 4 points
		// Total points = 1 + 3*(n-1) = 3n - 2
		numSegments := (len(smoothPath) - 1) / 3
		for seg := 0; seg < numSegments; seg++ {
			i := seg * 3
			// Need indices i, i+1, i+2, i+3
			if i+3 <= len(smoothPath)-1 {
				drawCubicBezier(ctx, smoothPath[i], smoothPath[i+1], smoothPath[i+2], smoothPath[i+3], c)
			}
		}
	} else {
		// Fallback: draw as quadratic through midpoint
		mid := Point{
			X: (path[0].X + path[len(path)-1].X) / 2,
			Y: (path[0].Y + path[len(path)-1].Y) / 2,
		}
		// Offset the control point based on the path's middle waypoint
		if len(path) > 2 {
			midWaypoint := path[len(path)/2]
			mid.X = midWaypoint.X
			mid.Y = midWaypoint.Y
		}
		drawQuadBezier(ctx, path[0].X, path[0].Y, mid.X, mid.Y, path[len(path)-1].X, path[len(path)-1].Y, c)
	}

	// Draw arrowhead using tangent at end of curve
	last := path[len(path)-1]
	var tx, ty float64
	
	if len(smoothPath) >= 4 {
		// Get tangent from last cubic segment
		// Last segment starts at index (numSegments-1)*3
		numSegments := (len(smoothPath) - 1) / 3
		lastSegStart := (numSegments - 1) * 3
		if lastSegStart >= 0 && lastSegStart+3 <= len(smoothPath)-1 {
			// Tangent at t=1 of cubic Bézier: 3*(P3 - P2)
			tx = smoothPath[lastSegStart+3].X - smoothPath[lastSegStart+2].X
			ty = smoothPath[lastSegStart+3].Y - smoothPath[lastSegStart+2].Y
		} else {
			tx = last.X - path[len(path)-2].X
			ty = last.Y - path[len(path)-2].Y
		}
	} else {
		// Use direction from second-to-last to last point
		tx = last.X - path[len(path)-2].X
		ty = last.Y - path[len(path)-2].Y
	}

	dist := math.Sqrt(tx*tx + ty*ty)
	if dist < 1 {
		return
	}

	nx := tx / dist
	ny := ty / dist

	arrowLen := 8.0 * ctx.scale
	arrowWidth := 4.0 * ctx.scale

	ax1 := last.X - nx*arrowLen + ny*arrowWidth
	ay1 := last.Y - ny*arrowLen - nx*arrowWidth
	ax2 := last.X - nx*arrowLen - ny*arrowWidth
	ay2 := last.Y - ny*arrowLen + nx*arrowWidth

	drawLine(ctx, last.X, last.Y, ax1, ay1, c)
	drawLine(ctx, last.X, last.Y, ax2, ay2, c)

	// Fill arrowhead
	for t := 0.0; t <= 1.0; t += 0.05 {
		mx := ax1 + (ax2-ax1)*t
		my := ay1 + (ay2-ay1)*t
		drawLine(ctx, last.X, last.Y, mx, my, c)
	}
}

// addIntermediatePoints adds points between waypoints to ensure smoother curves.
// This is especially important for paths with only 3-4 waypoints.
func addIntermediatePoints(path []Point) []Point {
	if len(path) < 3 {
		return path
	}

	result := make([]Point, 0, len(path)*2)
	result = append(result, path[0])

	for i := 0; i < len(path)-1; i++ {
		p1 := path[i]
		p2 := path[i+1]
		
		// Calculate distance between points
		dx := p2.X - p1.X
		dy := p2.Y - p1.Y
		dist := math.Sqrt(dx*dx + dy*dy)
		
		// Add intermediate point(s) for long segments
		if dist > 100 {
			// Add point at 1/3 and 2/3 along the segment
			result = append(result, Point{
				X: p1.X + dx/3,
				Y: p1.Y + dy/3,
			})
			result = append(result, Point{
				X: p1.X + 2*dx/3,
				Y: p1.Y + 2*dy/3,
			})
		} else if dist > 50 {
			// Add point at midpoint
			result = append(result, Point{
				X: (p1.X + p2.X) / 2,
				Y: (p1.Y + p2.Y) / 2,
			})
		}
		
		if i < len(path)-2 {
			result = append(result, p2)
		}
	}
	
	result = append(result, path[len(path)-1])
	return result
}

// waypointsToSmoothCurve converts a sequence of waypoints to cubic Bézier control points
// using Catmull-Rom spline interpolation for smooth curves through all points.
func waypointsToSmoothCurve(waypoints []Point) []Point {
	if len(waypoints) < 2 {
		return waypoints
	}
	if len(waypoints) == 2 {
		// Two points: create a simple cubic with control points on the line
		p0, p1 := waypoints[0], waypoints[1]
		ctrl1 := Point{p0.X + (p1.X-p0.X)/3, p0.Y + (p1.Y-p0.Y)/3}
		ctrl2 := Point{p0.X + 2*(p1.X-p0.X)/3, p0.Y + 2*(p1.Y-p0.Y)/3}
		return []Point{p0, ctrl1, ctrl2, p1}
	}

	// Build extended waypoints with phantom points at start and end
	// Use smarter phantom points that maintain curve continuity
	extended := make([]Point, len(waypoints)+2)
	
	// For the phantom start point, we want the curve to depart smoothly.
	// If we have at least 3 waypoints, use the direction from waypoints[0] to waypoints[2]
	// to inform the phantom, creating a more natural curve start.
	if len(waypoints) >= 3 {
		// Direction from first to third point (overall trend)
		dx := waypoints[2].X - waypoints[0].X
		dy := waypoints[2].Y - waypoints[0].Y
		// Phantom is first point minus this direction scaled
		extended[0] = Point{
			X: waypoints[0].X - dx*0.5,
			Y: waypoints[0].Y - dy*0.5,
		}
	} else {
		// Fallback: reflect first segment
		extended[0] = Point{
			X: 2*waypoints[0].X - waypoints[1].X,
			Y: 2*waypoints[0].Y - waypoints[1].Y,
		}
	}
	
	copy(extended[1:], waypoints)
	
	// For the phantom end point, similar logic
	n := len(waypoints)
	if n >= 3 {
		// Direction from third-to-last to last point (overall trend)
		dx := waypoints[n-1].X - waypoints[n-3].X
		dy := waypoints[n-1].Y - waypoints[n-3].Y
		extended[len(extended)-1] = Point{
			X: waypoints[n-1].X + dx*0.5,
			Y: waypoints[n-1].Y + dy*0.5,
		}
	} else {
		// Fallback: reflect last segment
		extended[len(extended)-1] = Point{
			X: 2*waypoints[n-1].X - waypoints[n-2].X,
			Y: 2*waypoints[n-1].Y - waypoints[n-2].Y,
		}
	}

	// Generate cubic Bézier control points using Catmull-Rom
	// For n waypoints, we get n-1 cubic segments = (n-1)*3 + 1 = 3n-2 control points
	result := make([]Point, 0, (len(waypoints)-1)*3+1)
	result = append(result, waypoints[0])

	// Tension parameter - higher values (up to 1.0) make tighter curves
	// Lower values make smoother, wider curves
	tension := 0.4 // Slightly less than standard Catmull-Rom for smoother curves

	for i := 0; i < len(waypoints)-1; i++ {
		p0 := extended[i]
		p1 := extended[i+1] // = waypoints[i]
		p2 := extended[i+2] // = waypoints[i+1]
		p3 := extended[i+3]

		// Control point 1: p1 + (p2 - p0) * tension / 3
		ctrl1 := Point{
			X: p1.X + (p2.X-p0.X)*tension/3,
			Y: p1.Y + (p2.Y-p0.Y)*tension/3,
		}

		// Control point 2: p2 - (p3 - p1) * tension / 3
		ctrl2 := Point{
			X: p2.X - (p3.X-p1.X)*tension/3,
			Y: p2.Y - (p3.Y-p1.Y)*tension/3,
		}

		result = append(result, ctrl1, ctrl2, p2)
	}

	return result
}

// drawSplineWithArrow draws a spline (cubic Bézier sequence) with arrowhead.
func drawSplineWithArrow(ctx *renderContext, spline []Point, c color.Color) {
	if len(spline) < 2 {
		return
	}

	if len(spline) < 4 {
		// Not enough for cubic, draw as path
		drawPathWithArrow(ctx, spline, c)
		return
	}

	// Draw cubic Bézier segments
	numSegments := (len(spline) - 1) / 3
	for seg := 0; seg < numSegments; seg++ {
		i := seg * 3
		if i+3 >= len(spline) {
			break
		}
		drawCubicBezier(ctx, spline[i], spline[i+1], spline[i+2], spline[i+3], c)
	}

	// Draw arrowhead using tangent at end
	tangent := EvaluateSplineTangent(spline, 1.0)
	endPt := EvaluateSpline(spline, 1.0)

	dist := math.Sqrt(tangent.X*tangent.X + tangent.Y*tangent.Y)
	if dist < 0.01 {
		return
	}

	nx := tangent.X / dist
	ny := tangent.Y / dist

	arrowLen := 8.0 * ctx.scale
	arrowWidth := 4.0 * ctx.scale

	ax1 := endPt.X - nx*arrowLen + ny*arrowWidth
	ay1 := endPt.Y - ny*arrowLen - nx*arrowWidth
	ax2 := endPt.X - nx*arrowLen - ny*arrowWidth
	ay2 := endPt.Y - ny*arrowLen + nx*arrowWidth

	drawLine(ctx, endPt.X, endPt.Y, ax1, ay1, c)
	drawLine(ctx, endPt.X, endPt.Y, ax2, ay2, c)

	for t := 0.0; t <= 1.0; t += 0.05 {
		mx := ax1 + (ax2-ax1)*t
		my := ay1 + (ay2-ay1)*t
		drawLine(ctx, endPt.X, endPt.Y, mx, my, c)
	}
}

// drawBidiTransitionPNG draws bidirectional transition arrows and returns one label position.
func drawBidiTransitionPNG(ctx *renderContext, x1, y1, x2, y2 float64, fromDims, toDims [2]float64, label1, label2 string) (float64, float64) {
	dx := x2 - x1
	dy := y2 - y1
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist < 1 {
		return x1, y1
	}

	nx := dx / dist
	ny := dy / dist

	// Perpendicular offset for the two curves
	perpX := -ny
	perpY := nx

	offset := dist * 0.15
	perpOffset := 5 * ctx.scale

	// First arrow (from -> to), curves one way
	// Start point offset perpendicular from ellipse edge
	sx1, sy1 := ellipseEdgePoint(x1, y1, fromDims[0], fromDims[1], nx, ny)
	sx1 += perpX * perpOffset
	sy1 += perpY * perpOffset
	
	ex1, ey1 := ellipseEdgePoint(x2, y2, toDims[0]+2*ctx.scale, toDims[1]+2*ctx.scale, -nx, -ny)
	ex1 += perpX * perpOffset
	ey1 += perpY * perpOffset
	
	cx1 := (x1+x2)/2 + perpX*offset
	cy1 := (y1+y2)/2 + perpY*offset

	drawQuadBezierArrow(ctx, sx1, sy1, cx1, cy1, ex1, ey1, colorBlack)
	lx1 := cx1 + perpX*10*ctx.scale
	ly1 := cy1 + perpY*10*ctx.scale
	drawTextCentered(ctx, int(lx1), int(ly1), label1, colorBlack)

	// Second arrow (to -> from), curves the other way
	sx2, sy2 := ellipseEdgePoint(x2, y2, toDims[0], toDims[1], -nx, -ny)
	sx2 -= perpX * perpOffset
	sy2 -= perpY * perpOffset
	
	ex2, ey2 := ellipseEdgePoint(x1, y1, fromDims[0]+2*ctx.scale, fromDims[1]+2*ctx.scale, nx, ny)
	ex2 -= perpX * perpOffset
	ey2 -= perpY * perpOffset
	
	cx2 := (x1+x2)/2 - perpX*offset
	cy2 := (y1+y2)/2 - perpY*offset

	drawQuadBezierArrow(ctx, sx2, sy2, cx2, cy2, ex2, ey2, colorBlack)
	drawTextCentered(ctx, int(cx2-perpX*10*ctx.scale), int(cy2-perpY*10*ctx.scale), label2, colorBlack)
	
	return lx1, ly1
}

// drawBidiTransitionPNGWithPlacer draws bidirectional arrows with collision-aware labels.
func drawBidiTransitionPNGWithPlacer(ctx *renderContext, x1, y1, x2, y2 float64, fromDims, toDims [2]float64, label1, label2 string, placer *LabelPlacer) (float64, float64) {
	dx := x2 - x1
	dy := y2 - y1
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist < 1 {
		return x1, y1
	}

	nx := dx / dist
	ny := dy / dist

	// Perpendicular offset for the two curves
	perpX := -ny
	perpY := nx

	offset := dist * 0.15
	perpOffset := 5 * ctx.scale

	// First arrow (from -> to), curves one way
	sx1, sy1 := ellipseEdgePoint(x1, y1, fromDims[0], fromDims[1], nx, ny)
	sx1 += perpX * perpOffset
	sy1 += perpY * perpOffset

	ex1, ey1 := ellipseEdgePoint(x2, y2, toDims[0]+2*ctx.scale, toDims[1]+2*ctx.scale, -nx, -ny)
	ex1 += perpX * perpOffset
	ey1 += perpY * perpOffset

	cx1 := (x1+x2)/2 + perpX*offset
	cy1 := (y1+y2)/2 + perpY*offset

	drawQuadBezierArrow(ctx, sx1, sy1, cx1, cy1, ex1, ey1, colorBlack)

	// Place first label with collision avoidance
	labelW1 := float64(len(label1)) * ctx.fontSize * 0.6
	labelH := ctx.fontSize
	gap := 8.0 * ctx.scale
	labelPos1 := placer.PlaceLabelOnCurve(
		Point{cx1, cy1},
		Point{perpX, perpY},
		labelW1, labelH, gap,
	)
	drawTextCentered(ctx, int(labelPos1.X), int(labelPos1.Y), label1, colorBlack)

	// Second arrow (to -> from), curves the other way
	sx2, sy2 := ellipseEdgePoint(x2, y2, toDims[0], toDims[1], -nx, -ny)
	sx2 -= perpX * perpOffset
	sy2 -= perpY * perpOffset

	ex2, ey2 := ellipseEdgePoint(x1, y1, fromDims[0]+2*ctx.scale, fromDims[1]+2*ctx.scale, nx, ny)
	ex2 -= perpX * perpOffset
	ey2 -= perpY * perpOffset

	cx2 := (x1+x2)/2 - perpX*offset
	cy2 := (y1+y2)/2 - perpY*offset

	drawQuadBezierArrow(ctx, sx2, sy2, cx2, cy2, ex2, ey2, colorBlack)

	// Place second label with collision avoidance
	labelW2 := float64(len(label2)) * ctx.fontSize * 0.6
	labelPos2 := placer.PlaceLabelOnCurve(
		Point{cx2, cy2},
		Point{-perpX, -perpY},
		labelW2, labelH, gap,
	)
	drawTextCentered(ctx, int(labelPos2.X), int(labelPos2.Y), label2, colorBlack)

	return labelPos1.X, labelPos1.Y
}

// labelBox represents a rectangular occupied region
type labelBox struct {
	x, y, w, h float64
}

// boxesOverlap checks if two boxes overlap
func boxesOverlap(x1, y1, w1, h1, x2, y2, w2, h2 float64) bool {
	// Check if boxes overlap (using centre + half-dimensions)
	return math.Abs(x1-x2) < (w1+w2)/2 && math.Abs(y1-y2) < (h1+h2)/2
}

// drawSelfLoopPNG draws a self-loop using the unified 7-point Bézier approach.
func drawSelfLoopPNG(ctx *renderContext, x, y, rx, ry float64, label string, occupiedBoxes []labelBox, graphCentreY, canvasW, canvasH float64) {
	state := Ellipse{CX: x, CY: y, RX: rx, RY: ry}

	// Choose the best side for the loop
	side := ChooseSelfLoopSide(state, canvasW, canvasH, nil)

	params := DefaultSelfLoopParams()
	params.Side = side

	points := SelfLoopControlPoints(state, params, ctx.scale)

	// Check bounds and adjust side if needed
	minX, minY, maxX, maxY := SelfLoopBounds(points)
	margin := 10.0 * ctx.scale
	if minX < margin || minY < margin || maxX > canvasW-margin || maxY > canvasH-margin {
		// Try alternative sides
		for _, altSide := range []LoopSide{LoopTop, LoopLeft, LoopBottom, LoopRight} {
			if altSide == side {
				continue
			}
			altParams := params
			altParams.Side = altSide
			altPoints := SelfLoopControlPoints(state, altParams, ctx.scale)
			aMinX, aMinY, aMaxX, aMaxY := SelfLoopBounds(altPoints)
			if aMinX >= margin && aMinY >= margin && aMaxX <= canvasW-margin && aMaxY <= canvasH-margin {
				points = altPoints
				params.Side = altSide
				break
			}
		}
	}

	// Draw the two cubic Bézier segments
	drawCubicBezier(ctx, points[0], points[1], points[2], points[3], colorBlack)
	drawCubicBezier(ctx, points[3], points[4], points[5], points[6], colorBlack)

	// Draw arrowhead at P6
	// Tangent direction at end: derivative of cubic Bézier at t=1
	// B'(1) = 3(P6 - P5)
	tx := 3 * (points[6].X - points[5].X)
	ty := 3 * (points[6].Y - points[5].Y)
	dist := math.Sqrt(tx*tx + ty*ty)
	if dist > 0 {
		tx /= dist
		ty /= dist
	}

	arrowLen := 8.0 * ctx.scale
	arrowWidth := 4.0 * ctx.scale

	ax1 := points[6].X - tx*arrowLen + ty*arrowWidth
	ay1 := points[6].Y - ty*arrowLen - tx*arrowWidth
	ax2 := points[6].X - tx*arrowLen - ty*arrowWidth
	ay2 := points[6].Y - ty*arrowLen + tx*arrowWidth

	drawLine(ctx, points[6].X, points[6].Y, ax1, ay1, colorBlack)
	drawLine(ctx, points[6].X, points[6].Y, ax2, ay2, colorBlack)
	for t := 0.0; t <= 1.0; t += 0.05 {
		mx := ax1 + (ax2-ax1)*t
		my := ay1 + (ay2-ay1)*t
		drawLine(ctx, points[6].X, points[6].Y, mx, my, colorBlack)
	}

	// Label placement with collision avoidance
	labelW := float64(len(label)) * ctx.fontSize * 0.6
	labelH := ctx.fontSize
	labelPos := SelfLoopLabelPosition(points, params.Side, labelW, labelH, ctx.scale)

	// Check for collisions with occupied boxes
	bestX, bestY := labelPos.X, labelPos.Y
	foundClear := true

	for _, box := range occupiedBoxes {
		if boxesOverlap(labelPos.X, labelPos.Y, labelW, labelH, box.x, box.y, box.w, box.h) {
			foundClear = false
			break
		}
	}

	if !foundClear {
		// Try alternative positions
		apex := points[3]
		gap := 8.0 * ctx.scale
		candidates := []Point{
			labelPos,
			{apex.X + gap + labelW/2, apex.Y - labelH},
			{apex.X + gap + labelW/2, apex.Y + labelH},
			{apex.X - gap - labelW/2, apex.Y},
		}

		for _, c := range candidates {
			overlaps := false
			for _, box := range occupiedBoxes {
				if boxesOverlap(c.X, c.Y, labelW, labelH, box.x, box.y, box.w, box.h) {
					overlaps = true
					break
				}
			}
			if !overlaps {
				bestX, bestY = c.X, c.Y
				break
			}
		}
	}

	drawTextCentered(ctx, int(bestX), int(bestY), label, colorBlack)
}

// SortedStates returns states in a deterministic order.
func SortedStates(states []string) []string {
	result := make([]string, len(states))
	copy(result, states)
	sort.Strings(result)
	return result
}
