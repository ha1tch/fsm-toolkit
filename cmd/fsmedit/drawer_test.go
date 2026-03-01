package main

import (
	"testing"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

func TestSplitClassName(t *testing.T) {
	tests := []struct {
		input     string
		wantShort string
		wantDesc  string
	}{
		{"7407_hex_buffer_oc", "7407", "Hex Buffer OC"},
		{"74181_4bit_alu", "74181", "4bit ALU"},
		{"7400_quad_nand", "7400", "Quad Nand"},
		{"custom_class", "custom_class", ""},
		{"", "", ""},
		{"7404_hex_inverter", "7404", "Hex Inverter"},
	}

	for _, tt := range tests {
		short, desc := splitClassName(tt.input)
		if short != tt.wantShort || desc != tt.wantDesc {
			t.Errorf("splitClassName(%q) = (%q, %q), want (%q, %q)",
				tt.input, short, desc, tt.wantShort, tt.wantDesc)
		}
	}
}

func TestCatalogNameFromFile(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"74xx_gates.classes.json", "74xx Gates"},
		{"74xx_registers_memory.classes.json", "74xx Registers Memory"},
		{"custom.classes.json", "Custom"},
	}

	for _, tt := range tests {
		got := catalogNameFromFile(tt.filename)
		if got != tt.want {
			t.Errorf("catalogNameFromFile(%q) = %q, want %q", tt.filename, got, tt.want)
		}
	}
}

func TestSortCatalogClasses(t *testing.T) {
	classes := []*fsm.Class{
		{Name: "7432_quad_or"},
		{Name: "7400_quad_nand"},
		{Name: "7408_quad_and"},
	}
	sortCatalogClasses(classes)
	if classes[0].Name != "7400_quad_nand" || classes[2].Name != "7432_quad_or" {
		t.Errorf("sort order wrong: %s, %s, %s", classes[0].Name, classes[1].Name, classes[2].Name)
	}
}

func TestInstantiateComponent(t *testing.T) {
	ed := newTestEditor()
	ed.fsm = fsm.New(fsm.TypeDFA)
	ed.fsm.EnsureClassMaps()

	cls := &fsm.Class{
		Name: "7407_hex_buffer_oc",
		Properties: []fsm.PropertyDef{
			{Name: "buffer_count", Type: fsm.PropInt64},
			{Name: "pin_names", Type: fsm.PropList},
		},
	}

	ed.instantiateComponent(cls, 10, 5)

	// State should be created with short name.
	if len(ed.fsm.States) != 1 || ed.fsm.States[0] != "7407" {
		t.Errorf("expected state '7407', got %v", ed.fsm.States)
	}

	// Class should be assigned.
	if ed.fsm.StateClasses["7407"] != "7407_hex_buffer_oc" {
		t.Errorf("expected class '7407_hex_buffer_oc', got %q", ed.fsm.StateClasses["7407"])
	}

	// Properties should be initialised.
	props := ed.fsm.StateProperties["7407"]
	if props == nil {
		t.Fatal("expected properties to be initialised")
	}
	if props["buffer_count"] != int64(0) {
		t.Errorf("expected buffer_count default 0, got %v", props["buffer_count"])
	}

	// Position should be set.
	if len(ed.states) != 1 || ed.states[0].X != 10 || ed.states[0].Y != 5 {
		t.Errorf("expected position (10,5), got %v", ed.states)
	}
}

func TestInstantiateComponentDuplicateNames(t *testing.T) {
	ed := newTestEditor()
	ed.fsm = fsm.New(fsm.TypeDFA)
	ed.fsm.EnsureClassMaps()

	cls := &fsm.Class{
		Name: "7407_hex_buffer_oc",
		Properties: []fsm.PropertyDef{
			{Name: "buffer_count", Type: fsm.PropInt64},
		},
	}

	ed.instantiateComponent(cls, 10, 5)
	ed.instantiateComponent(cls, 20, 5)
	ed.instantiateComponent(cls, 30, 5)

	// Should have 7407, 7407_1, 7407_2.
	if len(ed.fsm.States) != 3 {
		t.Fatalf("expected 3 states, got %d", len(ed.fsm.States))
	}
	expected := []string{"7407", "7407_1", "7407_2"}
	for i, want := range expected {
		if ed.fsm.States[i] != want {
			t.Errorf("state %d: expected %q, got %q", i, want, ed.fsm.States[i])
		}
	}
}

func TestDrawerEffectiveHeight(t *testing.T) {
	ed := newTestEditor()

	// Closed: height = 0.
	ed.drawerOpen = false
	ed.drawerAnimating = false
	if h := ed.drawerEffectiveHeight(); h != 0 {
		t.Errorf("closed drawer height = %d, want 0", h)
	}

	// Open, no animation: full height.
	ed.drawerOpen = true
	ed.drawerAnimating = false
	if h := ed.drawerEffectiveHeight(); h != drawerTargetHeight {
		t.Errorf("open drawer height = %d, want %d", h, drawerTargetHeight)
	}
}

func TestIsAbbreviation(t *testing.T) {
	if !isAbbreviation("oc") {
		t.Error("oc should be abbreviation")
	}
	if !isAbbreviation("alu") {
		t.Error("alu should be abbreviation")
	}
	if isAbbreviation("hex") {
		t.Error("hex should not be abbreviation")
	}
}

func TestEnsureVisible(t *testing.T) {
	tests := []struct {
		selected int
		scroll   int
		visible  int
		want     int
	}{
		{0, 0, 10, 0},    // at top, no change
		{5, 0, 10, 0},    // within view, no change
		{9, 0, 10, 0},    // last visible, no change
		{10, 0, 10, 1},   // just below view, scroll down
		{15, 0, 10, 6},   // well below view, scroll to show
		{3, 5, 10, 3},    // above view, scroll up
		{0, 5, 10, 0},    // at top but scrolled, scroll to top
		{49, 0, 10, 40},  // far below (like 49 classes)
		{48, 40, 10, 40}, // within view near bottom, no change
	}
	for _, tt := range tests {
		got := ensureVisible(tt.selected, tt.scroll, tt.visible)
		if got != tt.want {
			t.Errorf("ensureVisible(%d, %d, %d) = %d, want %d",
				tt.selected, tt.scroll, tt.visible, got, tt.want)
		}
	}
}

func TestMachMgrMachineNames_SingleMode(t *testing.T) {
	ed := &Editor{}
	ed.fsm = fsm.New(fsm.TypeDFA)
	ed.fsm.Name = "test_machine"
	ed.isBundle = false

	names := ed.machMgrMachineNames()
	if len(names) != 1 {
		t.Fatalf("expected 1 name, got %d", len(names))
	}
	if names[0] != "test_machine" {
		t.Errorf("expected 'test_machine', got %q", names[0])
	}
}

func TestMachMgrMachineNames_BundleMode(t *testing.T) {
	ed := &Editor{}
	ed.fsm = fsm.New(fsm.TypeDFA)
	ed.isBundle = true
	ed.bundleMachines = []string{"alpha", "beta", "gamma"}

	names := ed.machMgrMachineNames()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	if names[1] != "beta" {
		t.Errorf("expected 'beta' at [1], got %q", names[1])
	}
}

func TestRenamePropagation(t *testing.T) {
	// Build a bundle with two machines where main links to child.
	ed := &Editor{}

	mainFSM := fsm.New(fsm.TypeDFA)
	mainFSM.Name = "main"
	mainFSM.AddState("s0")
	mainFSM.SetLinkedMachine("s0", "child")

	childFSM := fsm.New(fsm.TypeDFA)
	childFSM.Name = "child"
	childFSM.AddState("c0")

	ed.isBundle = true
	ed.currentMachine = "main"
	ed.fsm = mainFSM
	ed.bundleMachines = []string{"main", "child"}
	ed.bundleFSMs = map[string]*fsm.FSM{"main": mainFSM, "child": childFSM}
	ed.bundleStates = map[string][]StatePos{"main": {}, "child": {}}
	ed.bundleUndoStack = map[string][]Snapshot{"main": nil, "child": nil}
	ed.bundleRedoStack = map[string][]Snapshot{"main": nil, "child": nil}
	ed.bundleModified = map[string]bool{"main": false, "child": false}
	ed.bundleOffsets = map[string][2]int{"main": {0, 0}, "child": {0, 0}}

	// Simulate rename: child -> controller
	// (Inline the rename logic since machMgrRenameMachine uses input prompts.)
	oldName := "child"
	newName := "controller"

	for i, name := range ed.bundleMachines {
		if name == oldName {
			ed.bundleMachines[i] = newName
			break
		}
	}
	if f, ok := ed.bundleFSMs[oldName]; ok {
		f.Name = newName
		ed.bundleFSMs[newName] = f
		delete(ed.bundleFSMs, oldName)
	}
	if s, ok := ed.bundleStates[oldName]; ok {
		ed.bundleStates[newName] = s
		delete(ed.bundleStates, oldName)
	}
	delete(ed.bundleUndoStack, oldName)
	delete(ed.bundleRedoStack, oldName)
	delete(ed.bundleModified, oldName)
	delete(ed.bundleOffsets, oldName)

	// Propagate link references.
	for _, machName := range ed.bundleMachines {
		if f, ok := ed.bundleFSMs[machName]; ok {
			for state, target := range f.LinkedMachines {
				if target == oldName {
					f.LinkedMachines[state] = newName
				}
			}
		}
	}

	// Verify: main's s0 should now link to "controller", not "child".
	target := mainFSM.GetLinkedMachine("s0")
	if target != "controller" {
		t.Errorf("expected link target 'controller', got %q", target)
	}

	// Verify: "child" no longer exists in the bundle.
	if _, ok := ed.bundleFSMs["child"]; ok {
		t.Error("old name 'child' still in bundleFSMs")
	}
	if _, ok := ed.bundleFSMs["controller"]; !ok {
		t.Error("new name 'controller' not in bundleFSMs")
	}
}
