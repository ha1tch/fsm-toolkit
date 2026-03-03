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

// buildTestFSMWithNets creates an FSM with classes, ports, and nets.
func buildTestFSMWithNets() *fsm.FSM {
	f := fsm.New(fsm.TypeDFA)
	f.Name = "net_test_circuit"
	f.AddState("U1")
	f.AddState("U2")
	f.AddInput("clk")
	f.AddTransition("U1", strp("clk"), []string{"U2"}, nil)

	nand := &fsm.Class{
		Name:       "7400_quad_nand",
		Properties: []fsm.PropertyDef{{Name: "package", Type: fsm.PropShortString}},
		Ports: []fsm.Port{
			{Name: "1A", Direction: fsm.PortInput, PinNumber: 1, Group: "GATE_A"},
			{Name: "1B", Direction: fsm.PortInput, PinNumber: 2, Group: "GATE_A"},
			{Name: "1Y", Direction: fsm.PortOutput, PinNumber: 3, Group: "GATE_A"},
			{Name: "GND", Direction: fsm.PortPower, PinNumber: 7},
			{Name: "VCC", Direction: fsm.PortPower, PinNumber: 14},
		},
	}
	flipflop := &fsm.Class{
		Name:       "7474_dual_d_flipflop",
		Properties: []fsm.PropertyDef{{Name: "trigger", Type: fsm.PropShortString}},
		Ports: []fsm.Port{
			{Name: "1D", Direction: fsm.PortInput, PinNumber: 2, Group: "FF1"},
			{Name: "1CLK", Direction: fsm.PortInput, PinNumber: 3, Group: "FF1"},
			{Name: "1Q", Direction: fsm.PortOutput, PinNumber: 5, Group: "FF1"},
			{Name: "GND", Direction: fsm.PortPower, PinNumber: 7},
			{Name: "VCC", Direction: fsm.PortPower, PinNumber: 14},
		},
	}

	f.Classes[nand.Name] = nand
	f.Classes[flipflop.Name] = flipflop
	f.StateClasses["U1"] = nand.Name
	f.StateClasses["U2"] = flipflop.Name

	f.Nets = []fsm.Net{
		{
			Name: "DATA_BUS",
			Endpoints: []fsm.NetEndpoint{
				{Instance: "U1", Port: "1Y"},
				{Instance: "U2", Port: "1D"},
			},
		},
		{
			Name: "VCC_RAIL",
			Endpoints: []fsm.NetEndpoint{
				{Instance: "U1", Port: "VCC"},
				{Instance: "U2", Port: "VCC"},
			},
		},
	}

	return f
}

func TestSingleFileNetPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nets.fsm")

	original := buildTestFSMWithNets()
	positions := map[string][2]int{"U1": {5, 3}, "U2": {25, 3}}

	if err := WriteFSMFileWithLayout(path, original, true, positions, 0, 0); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, _, err := ReadFSMFileWithLayout(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if len(loaded.Nets) != 2 {
		t.Fatalf("expected 2 nets, got %d", len(loaded.Nets))
	}

	// Verify DATA_BUS
	var dataBus *fsm.Net
	for i := range loaded.Nets {
		if loaded.Nets[i].Name == "DATA_BUS" {
			dataBus = &loaded.Nets[i]
			break
		}
	}
	if dataBus == nil {
		t.Fatal("DATA_BUS net not found after reload")
	}
	if len(dataBus.Endpoints) != 2 {
		t.Errorf("DATA_BUS: expected 2 endpoints, got %d", len(dataBus.Endpoints))
	}
	if !dataBus.HasEndpoint("U1", "1Y") {
		t.Error("DATA_BUS: missing endpoint U1.1Y")
	}
	if !dataBus.HasEndpoint("U2", "1D") {
		t.Error("DATA_BUS: missing endpoint U2.1D")
	}

	// Verify ports survived too
	cls, ok := loaded.Classes["7400_quad_nand"]
	if !ok {
		t.Fatal("class 7400_quad_nand not found after reload")
	}
	if len(cls.Ports) != 5 {
		t.Errorf("expected 5 ports on 7400_quad_nand, got %d", len(cls.Ports))
	}
}

func TestBundleNetPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bundle_nets.fsm")

	f := buildTestFSMWithNets()
	machines := map[string]BundleMachineData{
		"circuit1": {
			FSM:       f,
			Positions: map[string][2]int{"U1": {5, 3}, "U2": {25, 3}},
		},
	}

	if err := WriteBundleFromData(path, machines); err != nil {
		t.Fatalf("write bundle: %v", err)
	}

	loaded, _, err := ReadMachineFromBundle(path, "circuit1")
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}

	if len(loaded.Nets) != 2 {
		t.Errorf("expected 2 nets in circuit1, got %d", len(loaded.Nets))
	}
}

func TestJSONNetPersistence(t *testing.T) {
	original := buildTestFSMWithNets()

	data, err := ToJSON(original, true)
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}

	loaded, err := ParseJSON(data)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}

	if len(loaded.Nets) != 2 {
		t.Fatalf("expected 2 nets after JSON round-trip, got %d", len(loaded.Nets))
	}

	// Verify endpoint data fidelity
	for _, n := range loaded.Nets {
		if n.Name == "DATA_BUS" {
			if !n.HasEndpoint("U1", "1Y") || !n.HasEndpoint("U2", "1D") {
				t.Errorf("DATA_BUS endpoints corrupted: %+v", n.Endpoints)
			}
		}
	}
}

func TestNoNetsOmittedInJSON(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.Name = "bare"
	f.AddState("S1")

	data, err := ToJSON(f, true)
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}

	s := string(data)
	if indexOf(s, `"nets"`) >= 0 {
		t.Error("expected nets key to be omitted from JSON when empty")
	}
}

func TestLabelsTomlWithNets(t *testing.T) {
	f := buildTestFSMWithNets()
	_, states, inputs, outputs := FSMToRecords(f)
	toml := GenerateLabels(f, states, inputs, outputs)

	if indexOf(toml, "[nets]") < 0 {
		t.Error("expected [nets] section in labels.toml")
	}
	if indexOf(toml, "DATA_BUS") < 0 {
		t.Error("expected DATA_BUS in labels.toml")
	}
	if indexOf(toml, "U1.1Y") < 0 {
		t.Error("expected U1.1Y in labels.toml nets section")
	}

	// Verify it parses back
	labels, err := ParseLabels(toml)
	if err != nil {
		t.Fatalf("ParseLabels: %v", err)
	}
	if len(labels.Nets) != 2 {
		t.Errorf("expected 2 nets parsed from labels, got %d", len(labels.Nets))
	}
}

// indexOf is a simple substring search for test assertions.
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
