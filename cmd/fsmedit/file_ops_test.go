// File operations tests for fsmedit.
package main

import (
	"os"
	"path/filepath"
	"testing"
)

// --- loadFile smoke tests ---

func TestLoadFile_SingleJSON(t *testing.T) {
	ed := newTestEditor()
	// Use a known example file
	path := filepath.Join("../../examples", "turnstile.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("example file not found: " + path)
	}

	err := ed.loadFile(path)
	if err != nil {
		t.Fatalf("loadFile failed: %v", err)
	}
	if len(ed.fsm.States) == 0 {
		t.Error("no states loaded")
	}
	if ed.fsm.Initial == "" {
		t.Error("no initial state")
	}
	if ed.isBundle {
		t.Error("single JSON should not set bundle mode")
	}
}

func TestLoadFile_SingleFSM(t *testing.T) {
	ed := newTestEditor()
	path := filepath.Join("../../examples", "traffic_light.fsm")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("example file not found: " + path)
	}

	err := ed.loadFile(path)
	if err != nil {
		t.Fatalf("loadFile failed: %v", err)
	}
	if len(ed.fsm.States) == 0 {
		t.Error("no states loaded")
	}
}

func TestLoadFile_Bundle(t *testing.T) {
	ed := newTestEditor()
	path := filepath.Join("../../examples/bundles", "http_validator.fsm")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("example bundle not found: " + path)
	}

	err := ed.loadFile(path)
	if err != nil {
		t.Fatalf("loadFile failed: %v", err)
	}
	if !ed.isBundle {
		t.Error("bundle file should set isBundle=true")
	}
	if len(ed.bundleMachines) < 2 {
		t.Errorf("expected multiple machines, got %d", len(ed.bundleMachines))
	}
}

func TestLoadFile_ResetsState(t *testing.T) {
	ed := newTestBundle([]string{"old_a", "old_b"})
	ed.promotedFromSingle = true
	ed.navStack = []NavFrame{{MachineName: "old_a"}}

	path := filepath.Join("../../examples", "turnstile.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("example file not found: " + path)
	}

	err := ed.loadFile(path)
	if err != nil {
		t.Fatalf("loadFile failed: %v", err)
	}
	if ed.isBundle {
		t.Error("bundle state should have been reset")
	}
	if ed.promotedFromSingle {
		t.Error("promotedFromSingle should have been reset")
	}
	if ed.navStack != nil {
		t.Error("navStack should have been reset")
	}
}

// --- Smoke test all example files ---

func TestLoadFile_AllExamples(t *testing.T) {
	patterns := []string{
		"../../examples/*.json",
		"../../examples/*.fsm",
		"../../examples/bundles/*.fsm",
	}

	for _, pattern := range patterns {
		files, _ := filepath.Glob(pattern)
		for _, f := range files {
			t.Run(filepath.Base(f), func(t *testing.T) {
				ed := newTestEditor()
				err := ed.loadFile(f)
				if err != nil {
					t.Errorf("loadFile(%s) failed: %v", f, err)
				}
			})
		}
	}
}

// --- Save/load round-trip ---

func TestSaveLoadRoundTrip_SingleFSM(t *testing.T) {
	ed := newTestEditorWithStates([]string{"s0", "s1", "s2"})
	ed.fsm.Name = "roundtrip_test"
	ed.fsm.Alphabet = []string{"a", "b"}
	ed.fsm.Accepting = []string{"s2"}
	ed.fsm.AddTransition("s0", strPtr("a"), []string{"s1"}, nil)
	ed.fsm.AddTransition("s1", strPtr("b"), []string{"s2"}, nil)

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.fsm")

	// Save
	ed.filename = path
	err := ed.saveFile(path)
	if err != nil {
		t.Fatalf("saveFile failed: %v", err)
	}

	// Load into fresh editor
	ed2 := newTestEditor()
	err = ed2.loadFile(path)
	if err != nil {
		t.Fatalf("loadFile failed: %v", err)
	}

	// Verify
	if ed2.fsm.Name != "roundtrip_test" {
		t.Errorf("name: expected 'roundtrip_test', got %q", ed2.fsm.Name)
	}
	if len(ed2.fsm.States) != 3 {
		t.Errorf("states: expected 3, got %d", len(ed2.fsm.States))
	}
	if len(ed2.fsm.Transitions) != 2 {
		t.Errorf("transitions: expected 2, got %d", len(ed2.fsm.Transitions))
	}
	if ed2.fsm.Initial != "s0" {
		t.Errorf("initial: expected 's0', got %q", ed2.fsm.Initial)
	}
	if len(ed2.fsm.Accepting) != 1 || ed2.fsm.Accepting[0] != "s2" {
		t.Errorf("accepting: expected [s2], got %v", ed2.fsm.Accepting)
	}
}

func TestSaveLoadRoundTrip_Bundle(t *testing.T) {
	ed := newTestBundle([]string{"main", "auth"})
	ed.bundleModified["main"] = true
	ed.bundleModified["auth"] = true

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_bundle.fsm")
	ed.filename = path

	// Save bundle
	err := ed.saveBundleFile(path)
	if err != nil {
		t.Fatalf("saveBundleFile failed: %v", err)
	}

	// Load into fresh editor
	ed2 := newTestEditor()
	err = ed2.loadFile(path)
	if err != nil {
		t.Fatalf("loadFile failed: %v", err)
	}

	if !ed2.isBundle {
		t.Error("expected bundle mode after loading bundle")
	}
	if len(ed2.bundleMachines) != 2 {
		t.Errorf("expected 2 machines, got %d", len(ed2.bundleMachines))
	}
}

// --- resetBundleState via newFSM path ---

func TestNewFSMResetsBundle(t *testing.T) {
	ed := newTestBundle([]string{"a", "b", "c"})
	ed.promotedFromSingle = true

	// Simulate the newFSM action callback
	ed.fsm.Name = ""
	ed.filename = ""
	ed.modified = true
	ed.states = make([]StatePos, 0)
	ed.selectedState = -1
	ed.resetBundleState()

	if ed.isBundle {
		t.Error("isBundle should be false after reset")
	}
	if ed.bundleMachines != nil {
		t.Error("bundleMachines should be nil")
	}
}

// --- buildMachineData ---

func TestBuildMachineData(t *testing.T) {
	ed := newTestBundle([]string{"main"})
	ed.bundleStates["main"] = []StatePos{
		{Name: "s0", X: 10, Y: 20},
		{Name: "s1", X: 30, Y: 40},
	}
	ed.bundleOffsets["main"] = [2]int{5, 8}

	data := ed.buildMachineData("main", ed.bundleFSMs["main"])

	if data.FSM != ed.bundleFSMs["main"] {
		t.Error("FSM not set in machine data")
	}
	if data.Positions["s0"] != [2]int{10, 20} {
		t.Errorf("s0 position wrong: %v", data.Positions["s0"])
	}
	if data.Positions["s1"] != [2]int{30, 40} {
		t.Errorf("s1 position wrong: %v", data.Positions["s1"])
	}
}
