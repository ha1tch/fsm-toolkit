// Layout result structures for edge routing support.
// Extends the basic layout with virtual nodes and routing corridors.

package fsmfile

// LayoutResult contains complete layout information including edge routing.
type LayoutResult struct {
	// Node positions (center coordinates)
	Nodes map[string]NodeLayout

	// Edge routing information
	Edges map[string]EdgeLayout // Key: "from->to"

	// Rank (layer) information
	Ranks []RankInfo

	// Canvas dimensions used during layout
	Width, Height int
}

// NodeLayout contains position and size for a node.
type NodeLayout struct {
	X, Y    float64 // Center position
	Width   float64 // Full width
	Height  float64 // Full height
	Rank    int     // Which layer (for hierarchical layouts)
	Order   int     // Position within rank
	Virtual bool    // True for virtual nodes (not rendered)
	EdgeID  string  // For virtual nodes: which edge this belongs to
}

// EdgeLayout contains routing information for an edge.
type EdgeLayout struct {
	From, To   string
	Waypoints  []Point      // Path through routing corridor
	Boxes      []RoutingBox // Constraint boxes (for spline fitting)
	IsSelfLoop bool
	IsBackEdge bool // Goes against hierarchy direction
	IsFlatEdge bool // Same rank
}

// RoutingBox defines the available space for an edge segment.
type RoutingBox struct {
	Left, Right float64 // Horizontal bounds
	Top, Bottom float64 // Vertical bounds
}

// RankInfo contains information about a rank (layer).
type RankInfo struct {
	Y      float64  // Vertical position
	Height float64  // Height of rank
	Nodes  []string // Nodes in this rank (in order)
}

// NewLayoutResult creates an empty LayoutResult.
func NewLayoutResult(width, height int) *LayoutResult {
	return &LayoutResult{
		Nodes:  make(map[string]NodeLayout),
		Edges:  make(map[string]EdgeLayout),
		Ranks:  nil,
		Width:  width,
		Height: height,
	}
}

// GetNodePosition returns the position of a node, or (0,0) if not found.
func (lr *LayoutResult) GetNodePosition(name string) (float64, float64) {
	if node, ok := lr.Nodes[name]; ok {
		return node.X, node.Y
	}
	return 0, 0
}

// GetEdgeWaypoints returns the waypoints for an edge, or nil if not found.
func (lr *LayoutResult) GetEdgeWaypoints(from, to string) []Point {
	key := from + "->" + to
	if edge, ok := lr.Edges[key]; ok {
		return edge.Waypoints
	}
	return nil
}

// ToSimplePositions converts LayoutResult to the simple map format for backward compatibility.
func (lr *LayoutResult) ToSimplePositions() map[string][2]int {
	result := make(map[string][2]int)
	for name, node := range lr.Nodes {
		if !node.Virtual {
			result[name] = [2]int{int(node.X), int(node.Y)}
		}
	}
	return result
}

// BoxContains checks if a point is inside a routing box.
func (box RoutingBox) Contains(p Point) bool {
	return p.X >= box.Left && p.X <= box.Right &&
		p.Y >= box.Top && p.Y <= box.Bottom
}

// BoxCenter returns the center point of a routing box.
func (box RoutingBox) Center() Point {
	return Point{
		X: (box.Left + box.Right) / 2,
		Y: (box.Top + box.Bottom) / 2,
	}
}

// BoxWidth returns the width of a routing box.
func (box RoutingBox) Width() float64 {
	return box.Right - box.Left
}

// BoxHeight returns the height of a routing box.
func (box RoutingBox) Height() float64 {
	return box.Bottom - box.Top
}
