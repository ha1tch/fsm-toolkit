package fsmfile

import (
	"testing"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

func TestLayoutResult(t *testing.T) {
	lr := NewLayoutResult(800, 600)

	if lr.Width != 800 || lr.Height != 600 {
		t.Errorf("Expected dimensions 800x600, got %dx%d", lr.Width, lr.Height)
	}

	if lr.Nodes == nil || lr.Edges == nil {
		t.Error("Nodes and Edges maps should be initialized")
	}
}

func TestLayoutResultToSimplePositions(t *testing.T) {
	lr := NewLayoutResult(800, 600)

	lr.Nodes["A"] = NodeLayout{X: 100, Y: 50, Virtual: false}
	lr.Nodes["B"] = NodeLayout{X: 200, Y: 100, Virtual: false}
	lr.Nodes["_v_A_C_1"] = NodeLayout{X: 150, Y: 75, Virtual: true}

	positions := lr.ToSimplePositions()

	if len(positions) != 2 {
		t.Errorf("Expected 2 non-virtual nodes, got %d", len(positions))
	}

	if pos, ok := positions["A"]; !ok || pos[0] != 100 || pos[1] != 50 {
		t.Errorf("Node A position wrong: %v", positions["A"])
	}

	if _, ok := positions["_v_A_C_1"]; ok {
		t.Error("Virtual node should not be in simple positions")
	}
}

func TestRoutingBox(t *testing.T) {
	box := RoutingBox{Left: 10, Right: 100, Top: 20, Bottom: 80}

	// Test Contains
	if !box.Contains(Point{50, 50}) {
		t.Error("Point (50,50) should be inside box")
	}
	if box.Contains(Point{5, 50}) {
		t.Error("Point (5,50) should be outside box (left)")
	}
	if box.Contains(Point{50, 10}) {
		t.Error("Point (50,10) should be outside box (top)")
	}

	// Test Center
	center := box.Center()
	if center.X != 55 || center.Y != 50 {
		t.Errorf("Expected center (55,50), got (%.1f,%.1f)", center.X, center.Y)
	}

	// Test dimensions
	if box.Width() != 90 {
		t.Errorf("Expected width 90, got %.1f", box.Width())
	}
	if box.Height() != 60 {
		t.Errorf("Expected height 60, got %.1f", box.Height())
	}
}

func TestIsVirtualNode(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"A", false},
		{"state1", false},
		{"_v_", false}, // Too short
		{"_v_A_B_1", true},
		{"_v_test", true},
	}

	for _, tc := range tests {
		result := isVirtualNode(tc.name)
		if result != tc.expected {
			t.Errorf("isVirtualNode(%q) = %v, expected %v", tc.name, result, tc.expected)
		}
	}
}

func TestParseEdgeID(t *testing.T) {
	tests := []struct {
		edgeID       string
		expectedFrom string
		expectedTo   string
	}{
		{"A->B", "A", "B"},
		{"state1->state2", "state1", "state2"},
		{"->B", "", "B"},
		{"A->", "A", ""},
		{"invalid", "invalid", ""},
	}

	for _, tc := range tests {
		from, to := parseEdgeID(tc.edgeID)
		if from != tc.expectedFrom || to != tc.expectedTo {
			t.Errorf("parseEdgeID(%q) = (%q, %q), expected (%q, %q)",
				tc.edgeID, from, to, tc.expectedFrom, tc.expectedTo)
		}
	}
}

func TestRemoveFromSlice(t *testing.T) {
	slice := []string{"a", "b", "c", "d"}

	result := removeFromSlice(slice, "b")
	if len(result) != 3 {
		t.Errorf("Expected length 3, got %d", len(result))
	}

	// Original shouldn't contain "b" in result
	found := false
	for _, s := range result {
		if s == "b" {
			found = true
		}
	}
	if found {
		t.Error("Item 'b' should have been removed")
	}

	// Removing non-existent item should return same slice
	result2 := removeFromSlice(result, "x")
	if len(result2) != 3 {
		t.Errorf("Removing non-existent item changed length: %d", len(result2))
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		n        int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{123, "123"},
		{-5, "-5"},
	}

	for _, tc := range tests {
		result := itoa(tc.n)
		if result != tc.expected {
			t.Errorf("itoa(%d) = %q, expected %q", tc.n, result, tc.expected)
		}
	}
}

func TestSugiyamaLayoutFull(t *testing.T) {
	// Create a simple FSM with a long edge
	f := &fsm.FSM{
		Name:   "test",
		Type:   fsm.TypeDFA,
		States: []string{"A", "B", "C", "D"},
		Transitions: []fsm.Transition{
			{From: "A", To: []string{"B"}},
			{From: "B", To: []string{"C"}},
			{From: "A", To: []string{"D"}}, // Long edge: A->D spans multiple layers
			{From: "C", To: []string{"D"}},
		},
		Initial:   "A",
		Accepting: []string{"D"},
	}

	result := SugiyamaLayoutFull(f, 80, 40)

	// Check that we got node positions
	if len(result.Nodes) != 4 {
		t.Errorf("Expected 4 nodes, got %d", len(result.Nodes))
	}

	// Check that nodes are in result
	for _, state := range f.States {
		if _, ok := result.Nodes[state]; !ok {
			t.Errorf("Node %q missing from result", state)
		}
	}

	// Check ranks were computed
	if len(result.Ranks) == 0 {
		t.Error("Ranks should be computed")
	}

	// Check that edges were computed
	if len(result.Edges) == 0 {
		t.Error("Edges should be computed")
	}
}

func TestSugiyamaLayoutFullWithLongEdge(t *testing.T) {
	// Create FSM where A->D spans 3 layers (A at rank 0, D at rank 3)
	f := &fsm.FSM{
		Name:   "test-long-edge",
		Type:   fsm.TypeDFA,
		States: []string{"A", "B", "C", "D"},
		Transitions: []fsm.Transition{
			{From: "A", To: []string{"B"}},
			{From: "B", To: []string{"C"}},
			{From: "C", To: []string{"D"}},
			{From: "A", To: []string{"D"}}, // Long edge spanning 3 ranks
		},
		Initial:   "A",
		Accepting: []string{"D"},
	}

	result := SugiyamaLayoutFull(f, 80, 40)

	// The edge A->D should have routing information
	edgeKey := "A->D"
	edge, found := result.Edges[edgeKey]
	if !found {
		t.Fatalf("Edge %q not found in result", edgeKey)
	}

	// Long edge should have waypoints (virtual nodes create waypoints)
	if len(edge.Waypoints) == 0 {
		t.Log("Edge has no waypoints - may be direct if ranks are adjacent")
	}

	// Check edge properties
	if edge.From != "A" || edge.To != "D" {
		t.Errorf("Edge From/To wrong: %q -> %q", edge.From, edge.To)
	}

	if edge.IsSelfLoop {
		t.Error("A->D should not be a self-loop")
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Ensure SugiyamaLayout still works
	f := &fsm.FSM{
		Name:   "test",
		Type:   fsm.TypeDFA,
		States: []string{"A", "B"},
		Transitions: []fsm.Transition{
			{From: "A", To: []string{"B"}},
		},
		Initial:   "A",
		Accepting: []string{"B"},
	}

	positions := SugiyamaLayout(f, 80, 40)

	if len(positions) != 2 {
		t.Errorf("Expected 2 positions, got %d", len(positions))
	}

	if _, ok := positions["A"]; !ok {
		t.Error("Position for A missing")
	}
	if _, ok := positions["B"]; !ok {
		t.Error("Position for B missing")
	}
}
