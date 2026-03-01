// Bundle management tests for fsmedit.
package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

// --- resolveImportNameAuto ---

func TestResolveImportNameAuto_NoCollision(t *testing.T) {
	ed := newTestBundle([]string{"main", "auth"})
	got := ed.resolveImportNameAuto("payment")
	if got != "payment" {
		t.Errorf("expected 'payment', got %q", got)
	}
}

func TestResolveImportNameAuto_SingleCollision(t *testing.T) {
	ed := newTestBundle([]string{"main", "auth"})
	got := ed.resolveImportNameAuto("auth")
	if got != "auth_2" {
		t.Errorf("expected 'auth_2', got %q", got)
	}
}

func TestResolveImportNameAuto_MultipleCollisions(t *testing.T) {
	ed := newTestBundle([]string{"main", "auth", "auth_2", "auth_3"})
	got := ed.resolveImportNameAuto("auth")
	if got != "auth_4" {
		t.Errorf("expected 'auth_4', got %q", got)
	}
}

// --- machineNameExists ---

func TestMachineNameExists(t *testing.T) {
	ed := newTestBundle([]string{"alpha", "beta"})

	if !ed.machineNameExists("alpha") {
		t.Error("expected alpha to exist")
	}
	if !ed.machineNameExists("beta") {
		t.Error("expected beta to exist")
	}
	if ed.machineNameExists("gamma") {
		t.Error("expected gamma to not exist")
	}
}

// --- promoteToBundle ---

func TestPromoteToBundle(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0", "s1", "s2"})
	ed.fsm.Alphabet = []string{"a", "b"}
	ed.fsm.AddTransition("s0", strPtr("a"), []string{"s1"}, nil)
	ed.modified = true

	ed.promoteToBundle("traffic_light")

	if !ed.isBundle {
		t.Error("expected isBundle to be true")
	}
	if !ed.promotedFromSingle {
		t.Error("expected promotedFromSingle to be true")
	}
	if ed.currentMachine != "traffic_light" {
		t.Errorf("expected currentMachine 'traffic_light', got %q", ed.currentMachine)
	}
	if len(ed.bundleMachines) != 1 || ed.bundleMachines[0] != "traffic_light" {
		t.Errorf("unexpected bundleMachines: %v", ed.bundleMachines)
	}
	// Verify FSM was cached
	if ed.bundleFSMs["traffic_light"] != ed.fsm {
		t.Error("FSM not stored in bundleFSMs")
	}
	// Verify states were cached
	if len(ed.bundleStates["traffic_light"]) != 3 {
		t.Errorf("expected 3 cached states, got %d", len(ed.bundleStates["traffic_light"]))
	}
	// Verify modified flag was cached
	if !ed.bundleModified["traffic_light"] {
		t.Error("expected modified flag to be cached as true")
	}
}

func TestPromoteToBundleInitialisesAllMaps(t *testing.T) {
	ed := newTestEditor()
	ed.promoteToBundle("test")

	if ed.bundleFSMs == nil {
		t.Error("bundleFSMs not initialised")
	}
	if ed.bundleStates == nil {
		t.Error("bundleStates not initialised")
	}
	if ed.bundleUndoStack == nil {
		t.Error("bundleUndoStack not initialised")
	}
	if ed.bundleRedoStack == nil {
		t.Error("bundleRedoStack not initialised")
	}
	if ed.bundleModified == nil {
		t.Error("bundleModified not initialised")
	}
	if ed.bundleOffsets == nil {
		t.Error("bundleOffsets not initialised")
	}
}

// --- resetBundleState ---

func TestResetBundleState(t *testing.T) {
	ed := newTestBundle([]string{"a", "b", "c"})
	ed.promotedFromSingle = true
	ed.originalFilename = "/tmp/test.fsm"
	ed.navStack = []NavFrame{{MachineName: "a"}}
	ed.importMode = true
	ed.importSourcePath = "/tmp/import.fsm"

	ed.resetBundleState()

	if ed.isBundle {
		t.Error("isBundle should be false")
	}
	if ed.currentMachine != "" {
		t.Error("currentMachine should be empty")
	}
	if ed.bundleMachines != nil {
		t.Error("bundleMachines should be nil")
	}
	if ed.promotedFromSingle {
		t.Error("promotedFromSingle should be false")
	}
	if ed.originalFilename != "" {
		t.Error("originalFilename should be empty")
	}
	if ed.navStack != nil {
		t.Error("navStack should be nil")
	}
	if ed.importMode {
		t.Error("importMode should be false")
	}
}

// --- saveMachineToCache / loadMachineFromCache ---

func TestCacheRoundTrip(t *testing.T) {
	ed := newTestBundle([]string{"main", "sub"})

	// Modify current machine (main)
	ed.fsm.States = append(ed.fsm.States, "main_s2")
	ed.states = append(ed.states, StatePos{Name: "main_s2", X: 40, Y: 10})
	ed.modified = true
	ed.canvasOffsetX = 15
	ed.canvasOffsetY = 20

	// Save to cache
	ed.saveMachineToCache()

	// Verify cache was updated
	cachedStates := ed.bundleStates["main"]
	if len(cachedStates) != 3 {
		t.Errorf("expected 3 cached states, got %d", len(cachedStates))
	}
	if !ed.bundleModified["main"] {
		t.Error("expected modified flag cached as true")
	}
	offsets := ed.bundleOffsets["main"]
	if offsets[0] != 15 || offsets[1] != 20 {
		t.Errorf("expected offsets [15,20], got %v", offsets)
	}

	// Switch to sub
	ed.loadMachineFromCache("sub")

	if ed.currentMachine != "sub" {
		t.Errorf("expected currentMachine 'sub', got %q", ed.currentMachine)
	}
	if ed.fsm != ed.bundleFSMs["sub"] {
		t.Error("FSM not loaded from cache")
	}
	if len(ed.states) != 2 {
		t.Errorf("expected 2 states for sub, got %d", len(ed.states))
	}

	// Switch back to main
	ed.loadMachineFromCache("main")

	if ed.currentMachine != "main" {
		t.Errorf("expected currentMachine 'main', got %q", ed.currentMachine)
	}
	if len(ed.states) != 3 {
		t.Error("states not restored from cache")
	}
	if ed.canvasOffsetX != 15 || ed.canvasOffsetY != 20 {
		t.Errorf("offsets not restored: got %d,%d", ed.canvasOffsetX, ed.canvasOffsetY)
	}
	if !ed.modified {
		t.Error("modified flag not restored from cache")
	}
}

// --- anyBundleModified / getModifiedMachines ---

func TestAnyBundleModified_NoneModified(t *testing.T) {
	ed := newTestBundle([]string{"a", "b", "c"})
	ed.modified = false
	if ed.anyBundleModified() {
		t.Error("expected no modifications")
	}
}

func TestAnyBundleModified_CurrentModified(t *testing.T) {
	ed := newTestBundle([]string{"a", "b"})
	ed.modified = true
	if !ed.anyBundleModified() {
		t.Error("expected modifications detected")
	}
}

func TestAnyBundleModified_CachedModified(t *testing.T) {
	ed := newTestBundle([]string{"a", "b", "c"})
	ed.modified = false
	ed.bundleModified["c"] = true
	if !ed.anyBundleModified() {
		t.Error("expected modifications detected for 'c'")
	}
}

func TestGetModifiedMachines(t *testing.T) {
	ed := newTestBundle([]string{"a", "b", "c"})
	ed.modified = false
	ed.bundleModified["a"] = true
	ed.bundleModified["c"] = true

	mods := ed.getModifiedMachines()
	if len(mods) != 2 {
		t.Errorf("expected 2 modified machines, got %d: %v", len(mods), mods)
	}
}

func TestGetModifiedMachines_IncludesCurrent(t *testing.T) {
	ed := newTestBundle([]string{"a", "b"})
	ed.modified = true // current machine (a) is modified
	ed.bundleModified["a"] = false // but cache says no

	mods := ed.getModifiedMachines()
	// Should include "a" because ed.modified is true for current
	found := false
	for _, m := range mods {
		if m == "a" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'a' in modified list, got %v", mods)
	}
}

// --- getBreadcrumbs ---

func TestGetBreadcrumbs_Empty(t *testing.T) {
	ed := newTestBundle([]string{"main"})
	crumbs := ed.getBreadcrumbs()
	if len(crumbs) != 1 || crumbs[0] != "main" {
		t.Errorf("expected [main], got %v", crumbs)
	}
}

func TestGetBreadcrumbs_WithNavStack(t *testing.T) {
	ed := newTestBundle([]string{"main", "auth", "mfa"})
	ed.navStack = []NavFrame{
		{MachineName: "main"},
		{MachineName: "auth"},
	}
	ed.currentMachine = "mfa"

	crumbs := ed.getBreadcrumbs()
	if len(crumbs) != 3 {
		t.Errorf("expected 3 crumbs, got %d: %v", len(crumbs), crumbs)
	}
	expected := []string{"main", "auth", "mfa"}
	for i, e := range expected {
		if crumbs[i] != e {
			t.Errorf("crumb[%d]: expected %q, got %q", i, e, crumbs[i])
		}
	}
}

// --- addMachineToBundle ---

func TestAddMachineToBundle(t *testing.T) {
	ed := newTestBundle([]string{"main"})

	newFSM := fsm.New(fsm.TypeNFA)
	newFSM.Name = "auth"
	newFSM.States = []string{"idle", "checking", "done"}
	newFSM.Initial = "idle"
	newFSM.Alphabet = []string{"start", "pass", "fail"}

	ed.addMachineToBundle("auth", newFSM, nil)

	if len(ed.bundleMachines) != 2 {
		t.Errorf("expected 2 machines, got %d", len(ed.bundleMachines))
	}
	if ed.bundleFSMs["auth"] != newFSM {
		t.Error("FSM not stored in cache")
	}
	if len(ed.bundleStates["auth"]) != 3 {
		t.Errorf("expected 3 state positions, got %d", len(ed.bundleStates["auth"]))
	}
	if !ed.bundleModified["auth"] {
		t.Error("imported machine should be marked modified")
	}
}

func TestAddMachineToBundleAutoLayout(t *testing.T) {
	ed := newTestBundle([]string{"main"})

	newFSM := fsm.New(fsm.TypeDFA)
	newFSM.States = []string{"s0", "s1"}
	newFSM.Initial = "s0"

	ed.addMachineToBundle("small", newFSM, nil)

	// Verify positions were generated (not all zeros)
	states := ed.bundleStates["small"]
	if len(states) != 2 {
		t.Fatalf("expected 2 states, got %d", len(states))
	}
	// At minimum, names should be set
	if states[0].Name != "s0" || states[1].Name != "s1" {
		t.Errorf("state names wrong: %v", states)
	}
}

// --- promoteIfNeeded (synchronous path) ---

func TestPromoteIfNeeded_AlreadyBundle(t *testing.T) {
	ed := newTestBundle([]string{"main"})
	called := false
	ed.promoteIfNeeded(func() {
		called = true
	})
	if !called {
		t.Error("continuation should have been called immediately")
	}
}

func TestPromoteIfNeeded_NeedsPromotion(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0", "s1"})
	ed.filename = "/tmp/test_machine.fsm"
	ed.promoteIfNeeded(func() {})

	// Should have set up an input prompt
	if ed.mode != ModeInput {
		t.Errorf("expected ModeInput, got %d", ed.mode)
	}
	if ed.inputPrompt == "" {
		t.Error("expected input prompt to be set")
	}
	if ed.inputBuffer != "test_machine" {
		t.Errorf("expected default name 'test_machine', got %q", ed.inputBuffer)
	}
}

// --- Navigation stack ---

func TestNavigateBack_EmptyStack(t *testing.T) {
	ed := newTestBundle([]string{"main"})
	ed.navStack = nil

	// Should not panic
	ed.navigateBack()
}

// Regression: creating a new machine, switching away and back, should not
// bleed states from another machine into the empty one.
func TestLoadMachineFromBundle_EmptyNewMachine(t *testing.T) {
	ed := newTestBundle([]string{"alpha", "beta"})
	ed.filename = "/tmp/nonexistent_bundle.fsm" // simulate unsaved bundle

	// Create a new empty machine
	newFSM := fsm.New(fsm.TypeDFA)
	newFSM.Name = "empty"
	ed.addMachineToBundle("empty", newFSM, nil)

	// Switch to empty machine
	ed.saveMachineToCache()
	err := ed.loadMachineFromBundle("empty")
	if err != nil {
		t.Fatalf("loading empty machine failed: %v", err)
	}
	if len(ed.states) != 0 {
		t.Errorf("empty machine should have 0 states, got %d", len(ed.states))
	}

	// Switch to alpha (has states)
	ed.saveMachineToCache()
	err = ed.loadMachineFromBundle("alpha")
	if err != nil {
		t.Fatalf("loading alpha failed: %v", err)
	}
	if len(ed.states) == 0 {
		t.Error("alpha should have states")
	}
	alphaStateCount := len(ed.states)

	// Switch back to empty machine
	ed.saveMachineToCache()
	err = ed.loadMachineFromBundle("empty")
	if err != nil {
		t.Fatalf("loading empty machine second time failed: %v", err)
	}
	if len(ed.states) != 0 {
		t.Errorf("empty machine should still have 0 states, got %d (leaked %d from alpha)",
			len(ed.states), alphaStateCount)
	}
	if len(ed.fsm.Transitions) != 0 {
		t.Errorf("empty machine should have 0 transitions, got %d", len(ed.fsm.Transitions))
	}
}

// --- Multi-step switching: state and transition isolation ---

func TestSwitchCycle_StateIsolation(t *testing.T) {
	ed := newTestBundle([]string{"a", "b", "c"})

	// Each machine starts with 2 states from newTestBundle.
	// Add a third state to "a" only.
	ed.saveMachineToCache()
	ed.loadMachineFromCache("a")
	ed.fsm.States = append(ed.fsm.States, "a_s2")
	ed.states = append(ed.states, StatePos{Name: "a_s2", X: 40, Y: 5})
	ed.saveMachineToCache()

	// Cycle through all three machines and verify counts
	for _, tc := range []struct {
		name       string
		wantStates int
	}{
		{"b", 2},
		{"c", 2},
		{"a", 3},
		{"b", 2},
		{"a", 3},
	} {
		ed.saveMachineToCache()
		ed.loadMachineFromCache(tc.name)
		if len(ed.fsm.States) != tc.wantStates {
			t.Errorf("machine %s: expected %d states, got %d", tc.name, tc.wantStates, len(ed.fsm.States))
		}
		if len(ed.states) != tc.wantStates {
			t.Errorf("machine %s: expected %d state positions, got %d", tc.name, tc.wantStates, len(ed.states))
		}
	}
}

func TestSwitchCycle_TransitionIsolation(t *testing.T) {
	ed := newTestBundle([]string{"x", "y"})

	// x has 1 transition from newTestBundle, add another
	ed.saveMachineToCache()
	ed.loadMachineFromCache("x")
	ed.fsm.AddTransition("x_s1", strPtr("b"), []string{"x_s0"}, nil)
	ed.saveMachineToCache()

	// y should still have just its original transition
	ed.loadMachineFromCache("y")
	if len(ed.fsm.Transitions) != 1 {
		t.Errorf("machine y: expected 1 transition, got %d", len(ed.fsm.Transitions))
	}

	// x should have 2
	ed.saveMachineToCache()
	ed.loadMachineFromCache("x")
	if len(ed.fsm.Transitions) != 2 {
		t.Errorf("machine x: expected 2 transitions, got %d", len(ed.fsm.Transitions))
	}
}

// --- Modification tracking across switches ---

func TestModifiedFlag_SurvivesSwitchCycle(t *testing.T) {
	ed := newTestBundle([]string{"p", "q"})

	// Modify p
	ed.saveMachineToCache()
	ed.loadMachineFromCache("p")
	ed.modified = true
	ed.saveMachineToCache()

	// Switch to q (unmodified)
	ed.loadMachineFromCache("q")
	if ed.modified {
		t.Error("q should not be modified")
	}

	// Switch back to p
	ed.saveMachineToCache()
	ed.loadMachineFromCache("p")
	if !ed.modified {
		t.Error("p's modified flag should survive the round trip")
	}
}

// --- Undo stack isolation between machines ---

func TestUndoStack_PerMachineIsolation(t *testing.T) {
	ed := newTestBundle([]string{"m1", "m2"})

	// Build up undo history on m1
	ed.saveMachineToCache()
	ed.loadMachineFromCache("m1")
	ed.saveSnapshot() // undo entry 1
	ed.fsm.States = append(ed.fsm.States, "m1_extra")
	ed.saveSnapshot() // undo entry 2
	if len(ed.undoStack) != 2 {
		t.Fatalf("m1 should have 2 undo entries, got %d", len(ed.undoStack))
	}
	ed.saveMachineToCache()

	// Switch to m2 — should have empty undo stack
	ed.loadMachineFromCache("m2")
	if len(ed.undoStack) != 0 {
		t.Errorf("m2 should have 0 undo entries, got %d", len(ed.undoStack))
	}

	// Push one undo on m2
	ed.saveSnapshot()
	ed.saveMachineToCache()

	// Back to m1 — should still have 2
	ed.loadMachineFromCache("m1")
	if len(ed.undoStack) != 2 {
		t.Errorf("m1 should still have 2 undo entries, got %d", len(ed.undoStack))
	}
}

// --- Canvas offset isolation ---

func TestCanvasOffset_PerMachine(t *testing.T) {
	ed := newTestBundle([]string{"north", "south"})

	// Scroll north's viewport
	ed.saveMachineToCache()
	ed.loadMachineFromCache("north")
	ed.canvasOffsetX = 100
	ed.canvasOffsetY = 50
	ed.saveMachineToCache()

	// south should be at origin
	ed.loadMachineFromCache("south")
	if ed.canvasOffsetX != 0 || ed.canvasOffsetY != 0 {
		t.Errorf("south should be at (0,0), got (%d,%d)", ed.canvasOffsetX, ed.canvasOffsetY)
	}

	// Back to north
	ed.saveMachineToCache()
	ed.loadMachineFromCache("north")
	if ed.canvasOffsetX != 100 || ed.canvasOffsetY != 50 {
		t.Errorf("north offsets not restored: got (%d,%d)", ed.canvasOffsetX, ed.canvasOffsetY)
	}
}

// --- Linked state navigation ---

func TestDiveIntoLinkedState(t *testing.T) {
	ed := newTestBundle([]string{"root", "child"})

	// Set up root with a state linked to child
	ed.saveMachineToCache()
	ed.loadMachineFromCache("root")
	ed.fsm.SetLinkedMachine("root_s1", "child")

	// Dive into the linked state
	ed.diveIntoLinkedState(1) // index 1 = root_s1

	if len(ed.navStack) != 1 {
		t.Fatalf("expected 1 nav frame, got %d", len(ed.navStack))
	}
	if ed.navStack[0].MachineName != "root" {
		t.Errorf("nav frame should reference 'root', got %q", ed.navStack[0].MachineName)
	}
	if ed.navStack[0].LinkedState != "root_s1" {
		t.Errorf("nav frame should reference 'root_s1', got %q", ed.navStack[0].LinkedState)
	}
	if !ed.animating {
		t.Error("animation should be started")
	}
	if ed.animTargetMachine != "child" {
		t.Errorf("animation target should be 'child', got %q", ed.animTargetMachine)
	}
	if !ed.animZoomIn {
		t.Error("should be zoom-in animation")
	}
}

func TestDiveIntoLinkedState_NotLinked(t *testing.T) {
	ed := newTestBundle([]string{"root", "child"})
	ed.saveMachineToCache()
	ed.loadMachineFromCache("root")

	// s0 is not linked
	ed.diveIntoLinkedState(0)

	if len(ed.navStack) != 0 {
		t.Error("should not navigate into unlinked state")
	}
	if ed.animating {
		t.Error("should not start animation")
	}
}

func TestDiveIntoLinkedState_MissingTarget(t *testing.T) {
	ed := newTestBundle([]string{"root"})
	ed.saveMachineToCache()
	ed.loadMachineFromCache("root")
	ed.fsm.SetLinkedMachine("root_s0", "nonexistent")

	ed.diveIntoLinkedState(0)

	if len(ed.navStack) != 0 {
		t.Error("should not navigate to nonexistent machine")
	}
}

func TestDiveIntoLinkedState_InvalidIndex(t *testing.T) {
	ed := newTestBundle([]string{"root"})
	ed.saveMachineToCache()
	ed.loadMachineFromCache("root")

	// Should not panic
	ed.diveIntoLinkedState(-1)
	ed.diveIntoLinkedState(999)
}

// --- Navigate back ---

func TestNavigateBack_RestoresFrame(t *testing.T) {
	ed := newTestBundle([]string{"root", "child"})

	ed.saveMachineToCache()
	ed.loadMachineFromCache("root")

	// Push a nav frame manually (simulating dive without animation)
	ed.navStack = append(ed.navStack, NavFrame{
		MachineName:   "root",
		LinkedState:   "root_s1",
		LinkedStateX:  20,
		LinkedStateY:  5,
		CanvasOffsetX: 10,
		CanvasOffsetY: 15,
		SelectedState: 1,
	})

	// Load child machine
	ed.saveMachineToCache()
	ed.loadMachineFromCache("child")
	ed.currentMachine = "child"

	// Navigate back
	ed.navigateBack()

	if len(ed.navStack) != 0 {
		t.Errorf("nav stack should be empty, got %d", len(ed.navStack))
	}
	if !ed.animating {
		t.Error("should start zoom-out animation")
	}
	if ed.animZoomIn {
		t.Error("should be zoom-out, not zoom-in")
	}
	if ed.animTargetMachine != "root" {
		t.Errorf("animation target should be 'root', got %q", ed.animTargetMachine)
	}
	if ed.animCenterX != 20 || ed.animCenterY != 5 {
		t.Errorf("animation center should be (20,5), got (%d,%d)", ed.animCenterX, ed.animCenterY)
	}
}

// --- Breadcrumb navigation ---

func TestNavigateToBreadcrumb_MultiLevel(t *testing.T) {
	ed := newTestBundle([]string{"root", "mid", "leaf"})

	// Simulate a 3-level deep stack: root -> mid -> leaf (current)
	ed.navStack = []NavFrame{
		{MachineName: "root", LinkedState: "root_s0", LinkedStateX: 5, LinkedStateY: 5},
		{MachineName: "mid", LinkedState: "mid_s0", LinkedStateX: 10, LinkedStateY: 10},
	}
	ed.currentMachine = "leaf"

	crumbs := ed.getBreadcrumbs()
	if len(crumbs) != 3 {
		t.Fatalf("expected 3 breadcrumbs, got %v", crumbs)
	}

	// Jump back to root (level 0)
	ed.navigateToBreadcrumb(0)

	if len(ed.navStack) != 0 {
		t.Errorf("nav stack should be empty after jump to root, got %d", len(ed.navStack))
	}
	if ed.animTargetMachine != "root" {
		t.Errorf("should target 'root', got %q", ed.animTargetMachine)
	}
}

func TestNavigateToBreadcrumb_MidLevel(t *testing.T) {
	ed := newTestBundle([]string{"root", "mid", "leaf"})

	ed.navStack = []NavFrame{
		{MachineName: "root", LinkedState: "root_s0", LinkedStateX: 5, LinkedStateY: 5},
		{MachineName: "mid", LinkedState: "mid_s0", LinkedStateX: 10, LinkedStateY: 10},
	}
	ed.currentMachine = "leaf"

	// Jump to mid (level 1)
	ed.navigateToBreadcrumb(1)

	if len(ed.navStack) != 1 {
		t.Errorf("nav stack should have 1 frame, got %d", len(ed.navStack))
	}
	if ed.navStack[0].MachineName != "root" {
		t.Errorf("remaining frame should be root, got %q", ed.navStack[0].MachineName)
	}
	if ed.animTargetMachine != "mid" {
		t.Errorf("should target 'mid', got %q", ed.animTargetMachine)
	}
}

func TestNavigateToBreadcrumb_InvalidLevel(t *testing.T) {
	ed := newTestBundle([]string{"root", "child"})
	ed.navStack = []NavFrame{
		{MachineName: "root"},
	}
	ed.currentMachine = "child"

	stackBefore := len(ed.navStack)

	// Out of range -- should be no-op
	ed.navigateToBreadcrumb(-1)
	ed.navigateToBreadcrumb(5)

	if len(ed.navStack) != stackBefore {
		t.Error("nav stack should not change for invalid levels")
	}
}

// --- finishAnimation ---

func TestFinishAnimation_LoadsTargetMachine(t *testing.T) {
	ed := newTestBundle([]string{"src", "dst"})

	ed.saveMachineToCache()
	ed.loadMachineFromCache("src")
	ed.animating = true
	ed.animTargetMachine = "dst"
	ed.animZoomIn = true

	ed.finishAnimation()

	if ed.animating {
		t.Error("animation should be cleared")
	}
	if ed.currentMachine != "dst" {
		t.Errorf("should have loaded 'dst', got %q", ed.currentMachine)
	}
	if ed.mode != ModeCanvas {
		t.Errorf("should be in ModeCanvas, got %d", ed.mode)
	}
}

// --- generateStatesForMachine ---

func TestGenerateStatesForMachine_AutoLayout(t *testing.T) {
	ed := newTestBundle([]string{"gen"})
	ed.filename = "" // no file, force auto-layout

	ed.saveMachineToCache()
	ed.loadMachineFromCache("gen")

	states := ed.generateStatesForMachine("gen")

	if len(states) != len(ed.fsm.States) {
		t.Errorf("expected %d state positions, got %d", len(ed.fsm.States), len(states))
	}
	for i, sp := range states {
		if sp.Name != ed.fsm.States[i] {
			t.Errorf("state %d: expected name %q, got %q", i, ed.fsm.States[i], sp.Name)
		}
	}
}

// --- loadMachineFromBundle with real file ---

func TestLoadMachineFromBundle_RealFile(t *testing.T) {
	// Load a real bundle, switch between its machines via the full path
	ed := newTestEditor()
	path := "../../examples/bundles/http_validator.fsm"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("example bundle not found: " + path)
	}

	err := ed.loadFile(path)
	if err != nil {
		t.Fatalf("loadFile: %v", err)
	}
	if !ed.isBundle {
		t.Fatal("expected bundle mode")
	}
	if len(ed.bundleMachines) < 2 {
		t.Fatalf("expected 2+ machines, got %d", len(ed.bundleMachines))
	}

	first := ed.currentMachine
	firstStates := len(ed.states)
	firstFSM := ed.fsm

	// Pick a different machine
	var other string
	for _, m := range ed.bundleMachines {
		if m != first {
			other = m
			break
		}
	}

	// Switch to other
	ed.saveMachineToCache()
	err = ed.loadMachineFromBundle(other)
	if err != nil {
		t.Fatalf("switch to %s: %v", other, err)
	}
	if ed.currentMachine != other {
		t.Errorf("expected current=%q, got %q", other, ed.currentMachine)
	}
	if ed.fsm == firstFSM {
		t.Error("FSM pointer should differ after switch")
	}

	// Switch back
	ed.saveMachineToCache()
	err = ed.loadMachineFromBundle(first)
	if err != nil {
		t.Fatalf("switch back to %s: %v", first, err)
	}
	if ed.currentMachine != first {
		t.Errorf("expected current=%q, got %q", first, ed.currentMachine)
	}
	if len(ed.states) != firstStates {
		t.Errorf("state count changed: was %d, now %d", firstStates, len(ed.states))
	}
}

// --- Add machine then edit it ---

func TestAddMachine_ThenEditAndSwitch(t *testing.T) {
	ed := newTestBundle([]string{"existing"})

	// Create new machine
	newFSM := fsm.New(fsm.TypeNFA)
	newFSM.Name = "fresh"
	ed.addMachineToBundle("fresh", newFSM, nil)

	// Switch to fresh
	ed.saveMachineToCache()
	ed.loadMachineFromCache("fresh")

	// Add states to it
	ed.addStateAtPosition(10, 5)
	ed.addStateAtPosition(30, 5)
	ed.addStateAtPosition(50, 5)

	if len(ed.fsm.States) != 3 {
		t.Fatalf("expected 3 states, got %d", len(ed.fsm.States))
	}

	// Switch away
	ed.saveMachineToCache()
	ed.loadMachineFromCache("existing")

	if len(ed.fsm.States) != 2 {
		t.Errorf("existing should have 2 states, got %d", len(ed.fsm.States))
	}

	// Switch back to fresh
	ed.saveMachineToCache()
	ed.loadMachineFromCache("fresh")

	if len(ed.fsm.States) != 3 {
		t.Errorf("fresh should still have 3 states, got %d", len(ed.fsm.States))
	}
	if len(ed.states) != 3 {
		t.Errorf("fresh should have 3 positions, got %d", len(ed.states))
	}
}

// --- Rapid switching doesn't corrupt ---

func TestRapidSwitching_NCorruption(t *testing.T) {
	machines := []string{"m0", "m1", "m2", "m3", "m4"}
	ed := newTestBundle(machines)

	// Add distinct state counts to each machine
	for i, name := range machines {
		ed.saveMachineToCache()
		ed.loadMachineFromCache(name)
		for j := 0; j < i; j++ {
			ed.fsm.States = append(ed.fsm.States, fmt.Sprintf("%s_extra_%d", name, j))
			ed.states = append(ed.states, StatePos{
				Name: fmt.Sprintf("%s_extra_%d", name, j),
				X:    5 + j*15, Y: 15,
			})
		}
		// m0: 2 states, m1: 3, m2: 4, m3: 5, m4: 6
	}
	ed.saveMachineToCache()

	// Rapid-switch 50 times, verify counts stay correct
	for iter := 0; iter < 50; iter++ {
		idx := iter % len(machines)
		name := machines[idx]
		expected := 2 + idx // base 2 + idx extra

		ed.saveMachineToCache()
		ed.loadMachineFromCache(name)

		if len(ed.fsm.States) != expected {
			t.Fatalf("iter %d, machine %s: expected %d states, got %d",
				iter, name, expected, len(ed.fsm.States))
		}
		if len(ed.states) != expected {
			t.Fatalf("iter %d, machine %s: expected %d positions, got %d",
				iter, name, expected, len(ed.states))
		}
	}
}

// --- Delete state in one machine doesn't affect another ---

func TestDeleteState_CrossMachineIsolation(t *testing.T) {
	ed := newTestBundle([]string{"keep", "trim"})

	// Add a third state to trim, then delete it
	ed.saveMachineToCache()
	ed.loadMachineFromCache("trim")
	ed.addStateAtPosition(40, 10)
	if len(ed.fsm.States) != 3 {
		t.Fatalf("trim should have 3 states, got %d", len(ed.fsm.States))
	}
	ed.selectedState = 2
	ed.deleteSelected()
	if len(ed.fsm.States) != 2 {
		t.Fatalf("trim should have 2 states after delete, got %d", len(ed.fsm.States))
	}
	ed.saveMachineToCache()

	// keep should be untouched
	ed.loadMachineFromCache("keep")
	if len(ed.fsm.States) != 2 {
		t.Errorf("keep should still have 2 states, got %d", len(ed.fsm.States))
	}
}

// --- Promote single then add machine ---

func TestPromoteThenAddMachine(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0", "s1"})
	ed.fsm.Name = "original"
	ed.fsm.Alphabet = []string{"a"}
	ed.fsm.AddTransition("s0", strPtr("a"), []string{"s1"}, nil)
	ed.modified = true

	// Promote
	ed.promoteToBundle("original")
	if len(ed.bundleMachines) != 1 {
		t.Fatalf("expected 1 machine after promote, got %d", len(ed.bundleMachines))
	}

	// Add a second machine
	newFSM := fsm.New(fsm.TypeDFA)
	newFSM.Name = "second"
	newFSM.States = []string{"x0", "x1", "x2"}
	newFSM.Initial = "x0"
	ed.addMachineToBundle("second", newFSM, nil)

	if len(ed.bundleMachines) != 2 {
		t.Fatalf("expected 2 machines, got %d", len(ed.bundleMachines))
	}

	// Switch to second
	ed.saveMachineToCache()
	ed.loadMachineFromCache("second")
	if len(ed.fsm.States) != 3 {
		t.Errorf("second should have 3 states, got %d", len(ed.fsm.States))
	}

	// Back to original — transitions should be intact
	ed.saveMachineToCache()
	ed.loadMachineFromCache("original")
	if len(ed.fsm.Transitions) != 1 {
		t.Errorf("original should have 1 transition, got %d", len(ed.fsm.Transitions))
	}
}
