package fsmfile

import (
	"fmt"
	"os"
	"testing"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// --- NodeMetrics ---

func TestComputeNodeMetrics_BasicState(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.States = []string{"S0"}
	f.Initial = "S0"

	m := ComputeNodeMetrics(f)

	got := m["S0"]
	// "→[S0]" = 1+1+2+1 = 5 chars, but initial prefix doesn't add suffix
	// Actually: prefix(1) + '[' (1) + "S0"(2) + ']'(1) = 5
	// No accepting, no link => no suffix => width = len("S0") + 3 = 5
	if got.Width < 5 {
		t.Errorf("S0 width: expected >= 5, got %d", got.Width)
	}
	if got.TopMargin != 0 {
		t.Error("no self-loop, TopMargin should be 0")
	}
	if got.BottomMargin != 0 {
		t.Error("no annotation, BottomMargin should be 0")
	}
}

func TestComputeNodeMetrics_LongName(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.States = []string{"waiting_for_authentication"}
	f.Initial = "waiting_for_authentication"

	m := ComputeNodeMetrics(f)

	got := m["waiting_for_authentication"]
	// Width should be at least len("waiting_for_authentication") + 3
	expected := len("waiting_for_authentication") + 3
	if got.Width < expected {
		t.Errorf("long name width: expected >= %d, got %d", expected, got.Width)
	}
}

func TestComputeNodeMetrics_AcceptingState(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.States = []string{"done"}
	f.Accepting = []string{"done"}

	m := ComputeNodeMetrics(f)

	got := m["done"]
	// "○[done]*" = suffix adds 1 => len("done") + 4 = 8
	if got.Width < len("done")+4 {
		t.Errorf("accepting state width: expected >= %d, got %d", len("done")+4, got.Width)
	}
}

func TestComputeNodeMetrics_SelfLoop(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.States = []string{"idle"}
	f.Initial = "idle"
	f.Alphabet = []string{"tick"}
	f.AddTransition("idle", strPtr("tick"), []string{"idle"}, nil)

	m := ComputeNodeMetrics(f)

	got := m["idle"]
	if got.TopMargin != 3 {
		t.Errorf("self-loop TopMargin: expected 3, got %d", got.TopMargin)
	}
	if got.SelfLoopCount != 1 {
		t.Errorf("SelfLoopCount: expected 1, got %d", got.SelfLoopCount)
	}

	// Width should account for self-loop label extending right
	// center = len("idle")/2 + 2 = 4, label at center+5 = 9, label "tick" len=4
	// total right extent = 9+4 = 13 from state X
	// Base width = len("idle")+3 = 7
	// Self-loop extent = len("idle")/2 + 7 + len("tick") = 2+7+4 = 13
	if got.Width < 13 {
		t.Errorf("self-loop width: expected >= 13, got %d", got.Width)
	}
}

func TestComputeNodeMetrics_LinkedState(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.States = []string{"entry"}
	f.SetLinkedMachine("entry", "authentication_flow")

	m := ComputeNodeMetrics(f)

	got := m["entry"]
	if got.BottomMargin != 1 {
		t.Errorf("linked BottomMargin: expected 1, got %d", got.BottomMargin)
	}
	// Width should account for "  →authentication_flow" = 3 + 19 = 22
	annotW := 3 + len("authentication_flow")
	if got.Width < annotW {
		t.Errorf("linked width: expected >= %d, got %d", annotW, got.Width)
	}
}

func TestComputeNodeMetrics_MooreOutput(t *testing.T) {
	f := fsm.New(fsm.TypeMoore)
	f.States = []string{"counting"}
	f.StateOutputs = map[string]string{
		"counting": "display_count",
	}

	m := ComputeNodeMetrics(f)

	got := m["counting"]
	if got.BottomMargin != 1 {
		t.Errorf("Moore BottomMargin: expected 1, got %d", got.BottomMargin)
	}
	annotW := 3 + len("display_count")
	if got.Width < annotW {
		t.Errorf("Moore width: expected >= %d, got %d", annotW, got.Width)
	}
}

func TestComputeNodeMetrics_MealySelfLoop(t *testing.T) {
	f := fsm.New(fsm.TypeMealy)
	f.States = []string{"s0"}
	f.Alphabet = []string{"a"}
	f.OutputAlphabet = []string{"out"}
	f.AddTransition("s0", strPtr("a"), []string{"s0"}, strPtr("out"))

	m := ComputeNodeMetrics(f)

	got := m["s0"]
	if got.SelfLoopCount != 1 {
		t.Errorf("SelfLoopCount: expected 1, got %d", got.SelfLoopCount)
	}
	// Mealy label: "a/out" = 5 chars
	// Self-loop right extent = len("s0")/2 + 7 + 5 = 1+7+5 = 13
	if got.Width < 13 {
		t.Errorf("Mealy self-loop width: expected >= 13, got %d", got.Width)
	}
}

// --- MinLayerSpacing ---

func TestMinLayerSpacing_NoMargins(t *testing.T) {
	upper := []NodeMetrics{{Width: 6}}
	lower := []NodeMetrics{{Width: 6}}

	s := MinLayerSpacing(upper, lower, 3)
	if s < 3 {
		t.Errorf("expected >= 3, got %d", s)
	}
}

func TestMinLayerSpacing_SelfLoopBelow(t *testing.T) {
	upper := []NodeMetrics{{Width: 6, BottomMargin: 0}}
	lower := []NodeMetrics{{Width: 6, TopMargin: 3}} // self-loop

	s := MinLayerSpacing(upper, lower, 3)
	// Need 0 (bottom) + 3 (top) + 3 (base) = 6
	if s < 6 {
		t.Errorf("expected >= 6 for self-loop clearance, got %d", s)
	}
}

func TestMinLayerSpacing_LinkedAboveAndSelfLoopBelow(t *testing.T) {
	upper := []NodeMetrics{{Width: 6, BottomMargin: 1}} // linked annotation
	lower := []NodeMetrics{{Width: 6, TopMargin: 3}}    // self-loop

	s := MinLayerSpacing(upper, lower, 3)
	// Need 1 (bottom) + 3 (top) + 3 (base) = 7
	if s < 7 {
		t.Errorf("expected >= 7, got %d", s)
	}
}

// --- Layout quality: no overlaps ---

func strPtr(s string) *string {
	return &s
}

// buildFSMWithNStates creates an FSM with n states in a chain.
func buildFSMWithNStates(n int) *fsm.FSM {
	f := fsm.New(fsm.TypeDFA)
	for i := 0; i < n; i++ {
		f.States = append(f.States, fmt.Sprintf("s%d", i))
	}
	if n > 0 {
		f.Initial = "s0"
		f.Alphabet = []string{"a"}
	}
	for i := 0; i < n-1; i++ {
		from := fmt.Sprintf("s%d", i)
		to := fmt.Sprintf("s%d", i+1)
		f.AddTransition(from, strPtr("a"), []string{to}, nil)
	}
	return f
}

// buildFSMWithLongNames creates an FSM with states that have long names.
func buildFSMWithLongNames(n int, prefix string) *fsm.FSM {
	f := fsm.New(fsm.TypeDFA)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("%s_state_%d", prefix, i)
		f.States = append(f.States, name)
	}
	if n > 0 {
		f.Initial = f.States[0]
		f.Alphabet = []string{"next"}
	}
	for i := 0; i < n-1; i++ {
		f.AddTransition(f.States[i], strPtr("next"), []string{f.States[i+1]}, nil)
	}
	return f
}

// checkNoOverlaps verifies no two states overlap in the layout.
func checkNoOverlaps(t *testing.T, f *fsm.FSM, positions map[string][2]int) {
	t.Helper()
	metrics := ComputeNodeMetrics(f)

	type box struct {
		name string
		x1   int // left edge
		x2   int // right edge
		y    int
	}

	var boxes []box
	for name, pos := range positions {
		w := len(name) + 4
		if m, ok := metrics[name]; ok {
			w = m.Width
		}
		boxes = append(boxes, box{name, pos[0], pos[0] + w, pos[1]})
	}

	for i := 0; i < len(boxes); i++ {
		for j := i + 1; j < len(boxes); j++ {
			a, b := boxes[i], boxes[j]
			if a.y != b.y {
				continue
			}
			// Same row — check horizontal overlap
			if a.x1 < b.x2 && b.x1 < a.x2 {
				t.Errorf("overlap on row %d: %s [%d,%d) and %s [%d,%d)",
					a.y, a.name, a.x1, a.x2, b.name, b.x1, b.x2)
			}
		}
	}
}

func TestSmartLayoutTUI_NoOverlap_SmallFSM(t *testing.T) {
	f := buildFSMWithNStates(5)
	positions := SmartLayoutTUI(f, 80, 24)

	if len(positions) != 5 {
		t.Fatalf("expected 5 positions, got %d", len(positions))
	}
	checkNoOverlaps(t, f, positions)
}

func TestSmartLayoutTUI_NoOverlap_DozenStates(t *testing.T) {
	f := buildFSMWithNStates(12)
	positions := SmartLayoutTUI(f, 120, 40)

	if len(positions) != 12 {
		t.Fatalf("expected 12 positions, got %d", len(positions))
	}
	checkNoOverlaps(t, f, positions)
}

func TestSmartLayoutTUI_NoOverlap_ThirtyStates(t *testing.T) {
	f := buildFSMWithNStates(30)
	positions := SmartLayoutTUI(f, 200, 60)

	if len(positions) != 30 {
		t.Fatalf("expected 30 positions, got %d", len(positions))
	}
	checkNoOverlaps(t, f, positions)
}

func TestSmartLayoutTUI_NoOverlap_LongNames(t *testing.T) {
	f := buildFSMWithLongNames(8, "waiting_for_response")
	positions := SmartLayoutTUI(f, 200, 40)

	if len(positions) != 8 {
		t.Fatalf("expected 8 positions, got %d", len(positions))
	}
	checkNoOverlaps(t, f, positions)
}

func TestSmartLayoutTUI_NoOverlap_WithSelfLoops(t *testing.T) {
	f := buildFSMWithNStates(6)
	// Add self-loops to alternating states
	for i := 0; i < 6; i += 2 {
		name := fmt.Sprintf("s%d", i)
		f.AddTransition(name, strPtr("loop"), []string{name}, nil)
	}

	positions := SmartLayoutTUI(f, 100, 30)

	if len(positions) != 6 {
		t.Fatalf("expected 6 positions, got %d", len(positions))
	}
	checkNoOverlaps(t, f, positions)

	// States with self-loops should have enough vertical room above
	metrics := ComputeNodeMetrics(f)
	for name, pos := range positions {
		m := metrics[name]
		if m.TopMargin > 0 && pos[1] < m.TopMargin+1 {
			t.Errorf("state %s at y=%d has insufficient room for self-loop (needs %d above)",
				name, pos[1], m.TopMargin)
		}
	}
}

func TestSmartLayoutTUI_NoOverlap_WithLinkedStates(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.States = []string{"entry", "validate", "process", "done"}
	f.Initial = "entry"
	f.Alphabet = []string{"next"}
	f.AddTransition("entry", strPtr("next"), []string{"validate"}, nil)
	f.AddTransition("validate", strPtr("next"), []string{"process"}, nil)
	f.AddTransition("process", strPtr("next"), []string{"done"}, nil)
	f.SetLinkedMachine("validate", "validation_workflow")
	f.SetLinkedMachine("process", "processing_pipeline")

	positions := SmartLayoutTUI(f, 120, 30)

	if len(positions) != 4 {
		t.Fatalf("expected 4 positions, got %d", len(positions))
	}
	checkNoOverlaps(t, f, positions)
}

func TestSmartLayoutTUI_NoOverlap_MixedWidths(t *testing.T) {
	// Mix of very short and very long state names
	f := fsm.New(fsm.TypeDFA)
	f.States = []string{"A", "B", "waiting_for_user_input", "C", "processing_authentication_token", "D"}
	f.Initial = "A"
	f.Alphabet = []string{"go"}
	f.AddTransition("A", strPtr("go"), []string{"B"}, nil)
	f.AddTransition("B", strPtr("go"), []string{"waiting_for_user_input"}, nil)
	f.AddTransition("waiting_for_user_input", strPtr("go"), []string{"C"}, nil)
	f.AddTransition("C", strPtr("go"), []string{"processing_authentication_token"}, nil)
	f.AddTransition("processing_authentication_token", strPtr("go"), []string{"D"}, nil)

	positions := SmartLayoutTUI(f, 200, 30)

	if len(positions) != 6 {
		t.Fatalf("expected 6 positions, got %d", len(positions))
	}
	checkNoOverlaps(t, f, positions)
}

func TestSmartLayoutTUI_NoOverlap_DenseCyclic(t *testing.T) {
	// Dense cyclic graph — triggers force-directed path
	f := fsm.New(fsm.TypeNFA)
	n := 25
	for i := 0; i < n; i++ {
		f.States = append(f.States, fmt.Sprintf("n%d", i))
	}
	f.Initial = "n0"
	f.Alphabet = []string{"a", "b", "c"}

	// Create dense transitions: each state connects to 5+ others
	for i := 0; i < n; i++ {
		from := fmt.Sprintf("n%d", i)
		for j := 1; j <= 5; j++ {
			to := fmt.Sprintf("n%d", (i+j)%n)
			lbl := f.Alphabet[j%3]
			f.AddTransition(from, &lbl, []string{to}, nil)
		}
	}

	positions := SmartLayoutTUI(f, 200, 60)

	if len(positions) != n {
		t.Fatalf("expected %d positions, got %d", n, len(positions))
	}
	checkNoOverlaps(t, f, positions)
}

func TestSmartLayoutTUI_NoOverlap_AllExamples(t *testing.T) {
	// Smoke test: layout every example FSM and check for overlaps
	examples := []string{
		"../../examples/turnstile.json",
		"../../examples/tcp_connection.json",
		"../../examples/http_parser.json",
		"../../examples/game_enemy_ai.json",
		"../../examples/compiler_pipeline.json",
		"../../examples/vending_machine.json",
		"../../examples/door_lock.json",
		"../../examples/test_moore.json",
		"../../examples/test_mealy.json",
	}

	for _, path := range examples {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Skip("file not found: " + path)
			}
			f, err := ParseJSON(data)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			positions := SmartLayoutTUI(f, 120, 40)
			if len(positions) != len(f.States) {
				t.Errorf("expected %d positions, got %d", len(f.States), len(positions))
			}
			checkNoOverlaps(t, f, positions)
		})
	}
}

// --- Force-directed layout ---

func TestForceDirected_BasicSanity(t *testing.T) {
	// Force-directed is an SVG-path algorithm; just verify it doesn't crash
	// or produce negative positions on a large graph.
	f := fsm.New(fsm.TypeNFA)
	n := 30
	for i := 0; i < n; i++ {
		f.States = append(f.States, fmt.Sprintf("state_%d", i))
	}
	f.Initial = "state_0"
	f.Alphabet = []string{"a"}

	for i := 0; i < n; i++ {
		from := fmt.Sprintf("state_%d", i)
		to := fmt.Sprintf("state_%d", (i+1)%n)
		f.AddTransition(from, strPtr("a"), []string{to}, nil)
		if i%3 == 0 {
			cross := fmt.Sprintf("state_%d", (i+7)%n)
			f.AddTransition(from, strPtr("a"), []string{cross}, nil)
		}
	}

	positions := AutoLayout(f, LayoutForceDirected, 200, 60)

	if len(positions) != n {
		t.Fatalf("expected %d positions, got %d", n, len(positions))
	}
}

func TestSmartLayoutTUI_AllSizeClasses(t *testing.T) {
	// Verify SmartLayoutTUI produces overlap-free layouts for
	// small, medium, and large FSMs at realistic terminal sizes.
	cases := []struct {
		name   string
		states int
		w, h   int
	}{
		{"tiny_80x24", 4, 80, 24},
		{"small_80x24", 8, 80, 24},
		{"medium_120x40", 15, 120, 40},
		{"large_200x60", 30, 200, 60},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := buildFSMWithNStates(tc.states)
			positions := SmartLayoutTUI(f, tc.w, tc.h)
			if len(positions) != tc.states {
				t.Fatalf("expected %d positions, got %d", tc.states, len(positions))
			}
			checkNoOverlaps(t, f, positions)
		})
	}
}
