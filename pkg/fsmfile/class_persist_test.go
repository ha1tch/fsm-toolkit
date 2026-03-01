package fsmfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

func strp(s string) *string { return &s }

// buildTestFSMWithClasses creates an FSM with class data for testing.
func buildTestFSMWithClasses() *fsm.FSM {
	f := fsm.New(fsm.TypeDFA)
	f.Name = "test_circuit"
	f.AddState("idle")
	f.AddState("running")
	f.AddInput("start")
	f.AddTransition("idle", strp("start"), []string{"running"}, nil)

	f.Classes["7400_nand"] = &fsm.Class{
		Name: "7400_nand",
		Properties: []fsm.PropertyDef{
			{Name: "gate_count", Type: fsm.PropInt64},
			{Name: "pin_names", Type: fsm.PropList},
			{Name: "output_type", Type: fsm.PropShortString},
			{Name: "enabled", Type: fsm.PropBool},
		},
	}
	f.StateClasses["idle"] = "7400_nand"
	f.StateProperties["idle"] = map[string]interface{}{
		"gate_count":  int64(4),
		"pin_names":   []interface{}{"1A", "1B", "1Y"},
		"output_type": "totem-pole",
		"enabled":     true,
	}
	return f
}

func TestSingleFileClassPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.fsm")

	original := buildTestFSMWithClasses()
	positions := map[string][2]int{"idle": {5, 3}, "running": {20, 3}}

	if err := WriteFSMFileWithLayout(path, original, true, positions, 0, 0); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, _, err := ReadFSMFileWithLayout(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// default_state is always present, plus our 7400_nand
	if len(loaded.Classes) != 2 {
		t.Errorf("Classes: got %d, want 2", len(loaded.Classes))
	}
	cls, ok := loaded.Classes["7400_nand"]
	if !ok {
		t.Fatal("class 7400_nand not found after reload")
	}
	if len(cls.Properties) != 4 {
		t.Errorf("7400_nand properties: got %d, want 4", len(cls.Properties))
	}

	if sc := loaded.StateClasses["idle"]; sc != "7400_nand" {
		t.Errorf("StateClasses[idle]: got %q, want %q", sc, "7400_nand")
	}

	props := loaded.StateProperties["idle"]
	if props == nil {
		t.Fatal("StateProperties[idle] is nil")
	}
	// JSON round-trip coerces int64 to float64
	if v, ok := props["gate_count"].(float64); !ok || v != 4 {
		t.Errorf("gate_count: got %v (%T), want 4", props["gate_count"], props["gate_count"])
	}
	if v, ok := props["enabled"].(bool); !ok || !v {
		t.Errorf("enabled: got %v, want true", props["enabled"])
	}
}

func TestBundleClassPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bundle.fsm")

	machines := map[string]BundleMachineData{
		"controller": {
			FSM:       buildTestFSMWithClasses(),
			Positions: map[string][2]int{"idle": {5, 3}, "running": {20, 3}},
		},
		"datapath": {
			FSM: func() *fsm.FSM {
				f := fsm.New(fsm.TypeDFA)
				f.Name = "datapath"
				f.AddState("off")
				f.AddInput("on")
				f.AddTransition("off", strp("on"), []string{"off"}, nil)
				return f
			}(),
			Positions: map[string][2]int{"off": {5, 3}},
		},
	}

	if err := WriteBundleFromData(path, machines); err != nil {
		t.Fatalf("write bundle: %v", err)
	}

	// Verify controller has class data
	ctrl, _, err := ReadMachineFromBundle(path, "controller")
	if err != nil {
		t.Fatalf("read controller: %v", err)
	}
	if _, ok := ctrl.Classes["7400_nand"]; !ok {
		t.Error("controller: class 7400_nand not found after reload")
	}
	if ctrl.StateClasses["idle"] != "7400_nand" {
		t.Errorf("controller: StateClasses[idle] = %q, want 7400_nand", ctrl.StateClasses["idle"])
	}
	if ctrl.StateProperties["idle"] == nil {
		t.Error("controller: StateProperties[idle] is nil")
	}

	// Verify datapath has no user classes (only default_state)
	dp, _, err := ReadMachineFromBundle(path, "datapath")
	if err != nil {
		t.Fatalf("read datapath: %v", err)
	}
	if len(dp.Classes) != 1 {
		t.Errorf("datapath: got %d classes, want 1 (default_state only)", len(dp.Classes))
	}
}

func TestUpdateBundlePreservesClasses(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bundle.fsm")

	// Create initial bundle with classes on controller only
	machines := map[string]BundleMachineData{
		"controller": {
			FSM:       buildTestFSMWithClasses(),
			Positions: map[string][2]int{"idle": {5, 3}, "running": {20, 3}},
		},
		"datapath": {
			FSM: func() *fsm.FSM {
				f := fsm.New(fsm.TypeDFA)
				f.Name = "datapath"
				f.AddState("off")
				f.AddInput("on")
				f.AddTransition("off", strp("on"), []string{"off"}, nil)
				return f
			}(),
			Positions: map[string][2]int{"off": {5, 3}},
		},
	}
	if err := WriteBundleFromData(path, machines); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Update only datapath (add a class)
	dp, _, _ := ReadMachineFromBundle(path, "datapath")
	dp.Classes["7408_and"] = &fsm.Class{
		Name:       "7408_and",
		Properties: []fsm.PropertyDef{{Name: "inputs", Type: fsm.PropInt64}},
	}
	dp.StateClasses["off"] = "7408_and"
	dp.StateProperties["off"] = map[string]interface{}{"inputs": int64(2)}

	if err := UpdateBundleMachines(path, map[string]BundleMachineData{
		"datapath": {FSM: dp, Positions: map[string][2]int{"off": {5, 3}}},
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	// Controller's classes should be untouched
	ctrl, _, _ := ReadMachineFromBundle(path, "controller")
	if _, ok := ctrl.Classes["7400_nand"]; !ok {
		t.Error("controller lost 7400_nand class after unrelated update")
	}

	// Datapath should now have its class
	dp2, _, _ := ReadMachineFromBundle(path, "datapath")
	if _, ok := dp2.Classes["7408_and"]; !ok {
		t.Error("datapath: class 7408_and not found after update")
	}
	if dp2.StateProperties["off"] == nil {
		t.Error("datapath: StateProperties[off] lost after update")
	}
}

func TestNoClassDataProducesNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bare.fsm")

	f := fsm.New(fsm.TypeDFA)
	f.Name = "bare"
	f.AddState("s0")
	f.AddInput("a")
	f.AddTransition("s0", strp("a"), []string{"s0"}, nil)
	// Don't add any classes beyond the default_state

	positions := map[string][2]int{"s0": {5, 3}}
	if err := WriteFSMFileWithLayout(path, f, true, positions, 0, 0); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Verify file still loads correctly
	loaded, _, err := ReadFSMFileWithLayout(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if loaded.Name != "bare" {
		t.Errorf("name: got %q, want %q", loaded.Name, "bare")
	}

	// Verify file size is small (no classes.json bloat)
	info, _ := os.Stat(path)
	if info.Size() > 2000 {
		t.Errorf("bare FSM file unexpectedly large: %d bytes", info.Size())
	}
}
