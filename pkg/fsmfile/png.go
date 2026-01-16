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
	layoutWidth := opts.Width / 10
	layoutHeight := opts.Height / 20
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

	if fitScale > 1.5 {
		fitScale = 1.5
	}
	if fitScale < 0.3 {
		fitScale = 0.3
	}

	scaledWidth := contentWidth * fitScale
	scaledHeight := contentHeight * fitScale
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
	// First pass: draw non-self-loop transitions and collect label positions
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

			if hasBidi && !drawnPairs[reverseKey] {
				lx, ly := drawBidiTransitionPNG(ctx, fromPos[0], fromPos[1], toPos[0], toPos[1],
					fromDims, toDims, label, strings.Join(reverseLabels, ", "))
				// Add both label positions
				labelBoxes = append(labelBoxes, labelBox{lx, ly, 50 * ctx.scale, 15 * ctx.scale})
				drawnPairs[reverseKey] = true
			} else if !hasBidi {
				lx, ly := drawTransitionPNG(ctx, fromPos[0], fromPos[1], toPos[0], toPos[1],
					fromDims, toDims, label, graphCentreX, graphCentreY)
				labelBoxes = append(labelBoxes, labelBox{lx, ly, 50 * ctx.scale, 15 * ctx.scale})
			}
		}
		drawnPairs[key] = true
	}
	
	// Also add state ellipses as occupied regions
	for _, pos := range pngPos {
		for name := range pngPos {
			dims := ellipseDims[name]
			labelBoxes = append(labelBoxes, labelBox{pos[0], pos[1], dims[0] * 2, dims[1] * 2})
			break
		}
	}
	for name, pos := range pngPos {
		dims := ellipseDims[name]
		labelBoxes = append(labelBoxes, labelBox{pos[0], pos[1], dims[0] * 2, dims[1] * 2})
	}
	
	// Second pass: draw self-loops with smart label placement
	for _, loop := range selfLoops {
		drawSelfLoopPNG(ctx, loop.x, loop.y, loop.rx, loop.ry, loop.label, labelBoxes, graphCentreY)
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

// labelBox represents a rectangular occupied region
type labelBox struct {
	x, y, w, h float64
}

// boxesOverlap checks if two boxes overlap
func boxesOverlap(x1, y1, w1, h1, x2, y2, w2, h2 float64) bool {
	// Check if boxes overlap (using centre + half-dimensions)
	return math.Abs(x1-x2) < (w1+w2)/2 && math.Abs(y1-y2) < (h1+h2)/2
}

// drawSelfLoopPNG draws a self-loop on a state (on the right side).
func drawSelfLoopPNG(ctx *renderContext, x, y, rx, ry float64, label string, occupiedBoxes []labelBox, graphCentreY float64) {
	// Draw a ~300-degree arc that touches the right edge of the ellipse
	
	// Arc size - 20% larger than previous v18
	arcRx := rx * 0.48  // Horizontal radius of arc
	arcRy := ry * 0.42  // Vertical radius of arc (flatter)
	
	// Position arc so its left edge touches the state's right edge
	arcCx := x + rx + arcRx
	arcCy := y
	
	// Arc sweep: ~306°
	startAngle := -0.85 * math.Pi  // -153° (top-left of arc)
	endAngle := 0.85 * math.Pi     // +153° (bottom-left of arc)
	
	// Draw the arc
	steps := 50
	var prevPx, prevPy float64
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		angle := startAngle + t*(endAngle-startAngle)
		px := arcCx + arcRx*math.Cos(angle)
		py := arcCy + arcRy*math.Sin(angle)
		
		if i > 0 {
			drawLine(ctx, prevPx, prevPy, px, py, colorBlack)
		}
		prevPx, prevPy = px, py
	}
	
	// Arrowhead at end of arc
	endPx := arcCx + arcRx*math.Cos(endAngle)
	endPy := arcCy + arcRy*math.Sin(endAngle)
	
	// Tangent direction at end
	tx := -arcRx * math.Sin(endAngle)
	ty := arcRy * math.Cos(endAngle)
	dist := math.Sqrt(tx*tx + ty*ty)
	if dist > 0 {
		tx /= dist
		ty /= dist
	}
	
	arrowLen := 8.0 * ctx.scale
	arrowWidth := 4.0 * ctx.scale
	
	ax1 := endPx - tx*arrowLen + ty*arrowWidth
	ay1 := endPy - ty*arrowLen - tx*arrowWidth
	ax2 := endPx - tx*arrowLen - ty*arrowWidth
	ay2 := endPy - ty*arrowLen + tx*arrowWidth
	
	drawLine(ctx, endPx, endPy, ax1, ay1, colorBlack)
	drawLine(ctx, endPx, endPy, ax2, ay2, colorBlack)
	for t := 0.0; t <= 1.0; t += 0.05 {
		mx := ax1 + (ax2-ax1)*t
		my := ay1 + (ay2-ay1)*t
		drawLine(ctx, endPx, endPy, mx, my, colorBlack)
	}
	
	// Smart label placement: try multiple positions and pick the first unoccupied one
	// Estimate label size
	labelW := float64(len(label)) * ctx.fontSize * 0.6
	labelH := ctx.fontSize
	
	// Candidate positions: right, above, below (based on graph position)
	type candidate struct {
		x, y float64
	}
	var candidates []candidate
	
	// Right of loop (default, usually safe)
	candidates = append(candidates, candidate{arcCx + arcRx + 6*ctx.scale + labelW/2, arcCy})
	
	// Above or below based on position in graph
	if y > graphCentreY {
		// State in lower half - prefer above
		candidates = append(candidates, candidate{arcCx, arcCy - arcRy - 8*ctx.scale})
		candidates = append(candidates, candidate{arcCx, arcCy + arcRy + 8*ctx.scale})
	} else {
		// State in upper half - prefer below
		candidates = append(candidates, candidate{arcCx, arcCy + arcRy + 8*ctx.scale})
		candidates = append(candidates, candidate{arcCx, arcCy - arcRy - 8*ctx.scale})
	}
	
	// Find first non-overlapping position
	bestX, bestY := candidates[0].x, candidates[0].y
	for _, c := range candidates {
		overlaps := false
		for _, box := range occupiedBoxes {
			if boxesOverlap(c.x, c.y, labelW, labelH, box.x, box.y, box.w, box.h) {
				overlaps = true
				break
			}
		}
		if !overlaps {
			bestX, bestY = c.x, c.y
			break
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
