package fsm

import (
	"encoding/json"
	"testing"
)

// makeCircuit builds a small test circuit:
//   U1 (7400 quad NAND) ←→ U2 (7474 dual D FF)
// with ports on both classes.
func makeCircuit() *FSM {
	f := New(TypeDFA)
	f.AddState("U1")
	f.AddState("U2")

	nand := &Class{
		Name:       "7400_quad_nand",
		Properties: []PropertyDef{{Name: "package", Type: PropShortString}},
		Ports: []Port{
			{Name: "1A", Direction: PortInput, PinNumber: 1, Group: "GATE_A"},
			{Name: "1B", Direction: PortInput, PinNumber: 2, Group: "GATE_A"},
			{Name: "1Y", Direction: PortOutput, PinNumber: 3, Group: "GATE_A"},
			{Name: "GND", Direction: PortPower, PinNumber: 7},
			{Name: "VCC", Direction: PortPower, PinNumber: 14},
		},
	}
	ff := &Class{
		Name:       "7474_dual_d_flipflop",
		Properties: []PropertyDef{{Name: "trigger", Type: PropShortString}},
		Ports: []Port{
			{Name: "1D", Direction: PortInput, PinNumber: 2, Group: "FF1"},
			{Name: "1CLK", Direction: PortInput, PinNumber: 3, Group: "FF1"},
			{Name: "1Q", Direction: PortOutput, PinNumber: 5, Group: "FF1"},
			{Name: "GND", Direction: PortPower, PinNumber: 7},
			{Name: "VCC", Direction: PortPower, PinNumber: 14},
		},
	}

	f.Classes[nand.Name] = nand
	f.Classes[ff.Name] = ff
	f.StateClasses["U1"] = nand.Name
	f.StateClasses["U2"] = ff.Name

	return f
}

// --- Net-level tests ---

func TestNetHasEndpoint(t *testing.T) {
	n := Net{Name: "N1", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1Y"},
		{Instance: "U2", Port: "1D"},
	}}
	if !n.HasEndpoint("U1", "1Y") {
		t.Error("expected HasEndpoint to find U1.1Y")
	}
	if n.HasEndpoint("U1", "1D") {
		t.Error("expected HasEndpoint to miss U1.1D")
	}
	if n.HasEndpoint("U3", "1Y") {
		t.Error("expected HasEndpoint to miss U3.1Y")
	}
}

func TestNetHasInstance(t *testing.T) {
	n := Net{Name: "N1", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1Y"},
		{Instance: "U2", Port: "1D"},
	}}
	if !n.HasInstance("U1") {
		t.Error("expected HasInstance to find U1")
	}
	if n.HasInstance("U3") {
		t.Error("expected HasInstance to miss U3")
	}
}

func TestNetEndpointsForInstance(t *testing.T) {
	n := Net{Name: "BUS", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1Y"},
		{Instance: "U2", Port: "1D"},
		{Instance: "U2", Port: "1CLK"},
	}}
	eps := n.EndpointsForInstance("U2")
	if len(eps) != 2 {
		t.Errorf("expected 2 endpoints on U2, got %d", len(eps))
	}
	eps = n.EndpointsForInstance("U3")
	if len(eps) != 0 {
		t.Errorf("expected 0 endpoints on U3, got %d", len(eps))
	}
}

func TestNetRemoveEndpointsByInstance(t *testing.T) {
	n := Net{Name: "N1", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1Y"},
		{Instance: "U2", Port: "1D"},
		{Instance: "U3", Port: "1A"},
	}}
	removed := n.RemoveEndpointsByInstance("U2")
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
	if len(n.Endpoints) != 2 {
		t.Errorf("expected 2 endpoints remaining, got %d", len(n.Endpoints))
	}
	if n.HasInstance("U2") {
		t.Error("U2 should be removed")
	}
}

func TestNetRenameInstance(t *testing.T) {
	n := Net{Name: "N1", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1Y"},
		{Instance: "U2", Port: "1D"},
	}}
	n.RenameInstance("U1", "U99")
	if n.Endpoints[0].Instance != "U99" {
		t.Errorf("expected U99, got %s", n.Endpoints[0].Instance)
	}
	if n.Endpoints[1].Instance != "U2" {
		t.Errorf("U2 should be unchanged, got %s", n.Endpoints[1].Instance)
	}
}

func TestNetIsValid(t *testing.T) {
	n := Net{Name: "N1", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1Y"},
	}}
	if n.IsValid() {
		t.Error("net with 1 endpoint should be invalid")
	}
	n.Endpoints = append(n.Endpoints, NetEndpoint{Instance: "U2", Port: "1D"})
	if !n.IsValid() {
		t.Error("net with 2 endpoints should be valid")
	}
}

// --- FSM-level net tests ---

func TestFSMHasNets(t *testing.T) {
	f := makeCircuit()
	if f.HasNets() {
		t.Error("expected HasNets = false initially")
	}
	f.Nets = []Net{{Name: "N1", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1Y"},
		{Instance: "U2", Port: "1D"},
	}}}
	if !f.HasNets() {
		t.Error("expected HasNets = true after adding net")
	}
}

func TestFSMAddNet(t *testing.T) {
	f := makeCircuit()
	err := f.AddNet(Net{
		Name: "DATA",
		Endpoints: []NetEndpoint{
			{Instance: "U1", Port: "1Y"},
			{Instance: "U2", Port: "1D"},
		},
	})
	if err != nil {
		t.Fatalf("AddNet: %v", err)
	}
	if len(f.Nets) != 1 {
		t.Fatalf("expected 1 net, got %d", len(f.Nets))
	}
}

func TestFSMAddNetDuplicateName(t *testing.T) {
	f := makeCircuit()
	net := Net{
		Name: "DATA",
		Endpoints: []NetEndpoint{
			{Instance: "U1", Port: "1Y"},
			{Instance: "U2", Port: "1D"},
		},
	}
	if err := f.AddNet(net); err != nil {
		t.Fatalf("first AddNet: %v", err)
	}
	if err := f.AddNet(net); err == nil {
		t.Error("expected error for duplicate net name")
	}
}

func TestFSMAddNetEmptyName(t *testing.T) {
	f := makeCircuit()
	err := f.AddNet(Net{
		Name: "",
		Endpoints: []NetEndpoint{
			{Instance: "U1", Port: "1Y"},
			{Instance: "U2", Port: "1D"},
		},
	})
	if err == nil {
		t.Error("expected error for empty net name")
	}
}

func TestFSMAddNetTooFewEndpoints(t *testing.T) {
	f := makeCircuit()
	err := f.AddNet(Net{
		Name:      "LONELY",
		Endpoints: []NetEndpoint{{Instance: "U1", Port: "1Y"}},
	})
	if err == nil {
		t.Error("expected error for net with 1 endpoint")
	}
}

func TestFSMAddNetBadInstance(t *testing.T) {
	f := makeCircuit()
	err := f.AddNet(Net{
		Name: "BAD",
		Endpoints: []NetEndpoint{
			{Instance: "U99", Port: "1Y"},
			{Instance: "U2", Port: "1D"},
		},
	})
	if err == nil {
		t.Error("expected error for nonexistent instance")
	}
}

func TestFSMAddNetBadPort(t *testing.T) {
	f := makeCircuit()
	err := f.AddNet(Net{
		Name: "BAD",
		Endpoints: []NetEndpoint{
			{Instance: "U1", Port: "NONEXISTENT"},
			{Instance: "U2", Port: "1D"},
		},
	})
	if err == nil {
		t.Error("expected error for nonexistent port")
	}
}

func TestFSMAddNetNoPorts(t *testing.T) {
	f := New(TypeDFA)
	f.AddState("S1")
	f.AddState("S2")
	// default_state has no ports
	err := f.AddNet(Net{
		Name: "BAD",
		Endpoints: []NetEndpoint{
			{Instance: "S1", Port: "foo"},
			{Instance: "S2", Port: "bar"},
		},
	})
	if err == nil {
		t.Error("expected error for instance without ports")
	}
}

func TestFSMRemoveNet(t *testing.T) {
	f := makeCircuit()
	f.AddNet(Net{
		Name: "DATA",
		Endpoints: []NetEndpoint{
			{Instance: "U1", Port: "1Y"},
			{Instance: "U2", Port: "1D"},
		},
	})
	if !f.RemoveNet("DATA") {
		t.Error("expected RemoveNet to return true")
	}
	if f.HasNets() {
		t.Error("expected no nets after removal")
	}
	if f.RemoveNet("NONEXISTENT") {
		t.Error("expected RemoveNet to return false for missing net")
	}
}

func TestFSMGetNet(t *testing.T) {
	f := makeCircuit()
	f.AddNet(Net{
		Name: "DATA",
		Endpoints: []NetEndpoint{
			{Instance: "U1", Port: "1Y"},
			{Instance: "U2", Port: "1D"},
		},
	})
	n := f.GetNet("DATA")
	if n == nil {
		t.Fatal("expected to find net DATA")
	}
	if n.Name != "DATA" {
		t.Errorf("expected name DATA, got %s", n.Name)
	}
	if f.GetNet("MISSING") != nil {
		t.Error("expected nil for missing net")
	}
}

func TestFSMRenameNet(t *testing.T) {
	f := makeCircuit()
	f.AddNet(Net{
		Name: "OLD",
		Endpoints: []NetEndpoint{
			{Instance: "U1", Port: "1Y"},
			{Instance: "U2", Port: "1D"},
		},
	})
	if err := f.RenameNet("OLD", "NEW"); err != nil {
		t.Fatalf("RenameNet: %v", err)
	}
	if f.GetNet("OLD") != nil {
		t.Error("old name should not exist")
	}
	if f.GetNet("NEW") == nil {
		t.Error("new name should exist")
	}
}

func TestFSMRenameNetConflict(t *testing.T) {
	f := makeCircuit()
	f.AddNet(Net{Name: "A", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1Y"}, {Instance: "U2", Port: "1D"},
	}})
	f.AddNet(Net{Name: "B", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1A"}, {Instance: "U2", Port: "1CLK"},
	}})
	if err := f.RenameNet("A", "B"); err == nil {
		t.Error("expected error for conflicting rename")
	}
}

func TestFSMRenameNetEmpty(t *testing.T) {
	f := makeCircuit()
	f.AddNet(Net{Name: "A", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1Y"}, {Instance: "U2", Port: "1D"},
	}})
	if err := f.RenameNet("A", ""); err == nil {
		t.Error("expected error for empty rename")
	}
}

func TestFSMNetsForState(t *testing.T) {
	f := makeCircuit()
	f.AddNet(Net{Name: "DATA", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1Y"}, {Instance: "U2", Port: "1D"},
	}})
	f.AddNet(Net{Name: "CLK", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1A"}, {Instance: "U2", Port: "1CLK"},
	}})

	nets := f.NetsForState("U1")
	if len(nets) != 2 {
		t.Errorf("expected 2 nets for U1, got %d", len(nets))
	}
	nets = f.NetsForState("U2")
	if len(nets) != 2 {
		t.Errorf("expected 2 nets for U2, got %d", len(nets))
	}
	nets = f.NetsForState("U99")
	if len(nets) != 0 {
		t.Errorf("expected 0 nets for U99, got %d", len(nets))
	}
}

func TestFSMNetsBetween(t *testing.T) {
	f := makeCircuit()
	f.AddState("U3")
	f.Classes["7400_quad_nand"].Ports = append(f.Classes["7400_quad_nand"].Ports,
		Port{Name: "2A", Direction: PortInput, PinNumber: 4, Group: "GATE_B"})
	f.StateClasses["U3"] = "7400_quad_nand"

	f.AddNet(Net{Name: "DATA", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1Y"}, {Instance: "U2", Port: "1D"},
	}})
	f.AddNet(Net{Name: "OTHER", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1A"}, {Instance: "U3", Port: "2A"},
	}})

	nets := f.NetsBetween("U1", "U2")
	if len(nets) != 1 {
		t.Errorf("expected 1 net between U1-U2, got %d", len(nets))
	}
	nets = f.NetsBetween("U1", "U3")
	if len(nets) != 1 {
		t.Errorf("expected 1 net between U1-U3, got %d", len(nets))
	}
	nets = f.NetsBetween("U2", "U3")
	if len(nets) != 0 {
		t.Errorf("expected 0 nets between U2-U3, got %d", len(nets))
	}
}

func TestFSMIsPowerNet(t *testing.T) {
	f := makeCircuit()

	powerNet := Net{Name: "VCC", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "VCC"}, {Instance: "U2", Port: "VCC"},
	}}
	if !f.IsPowerNet(powerNet) {
		t.Error("expected VCC net to be a power net")
	}

	signalNet := Net{Name: "DATA", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1Y"}, {Instance: "U2", Port: "1D"},
	}}
	if f.IsPowerNet(signalNet) {
		t.Error("expected DATA net not to be a power net")
	}

	mixedNet := Net{Name: "MIX", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "VCC"}, {Instance: "U2", Port: "1D"},
	}}
	if f.IsPowerNet(mixedNet) {
		t.Error("expected mixed net not to be a power net")
	}
}

func TestFSMSignalNets(t *testing.T) {
	f := makeCircuit()
	f.Nets = []Net{
		{Name: "VCC", Endpoints: []NetEndpoint{
			{Instance: "U1", Port: "VCC"}, {Instance: "U2", Port: "VCC"},
		}},
		{Name: "GND", Endpoints: []NetEndpoint{
			{Instance: "U1", Port: "GND"}, {Instance: "U2", Port: "GND"},
		}},
		{Name: "DATA", Endpoints: []NetEndpoint{
			{Instance: "U1", Port: "1Y"}, {Instance: "U2", Port: "1D"},
		}},
	}

	sig := f.SignalNets()
	if len(sig) != 1 {
		t.Errorf("expected 1 signal net, got %d", len(sig))
	}
	if sig[0].Name != "DATA" {
		t.Errorf("expected DATA, got %s", sig[0].Name)
	}
}

// --- Summary direction tests ---

func TestSummaryDirectionAtoB(t *testing.T) {
	f := makeCircuit()
	// U1.1Y (output) → U2.1D (input): all A→B
	f.Nets = []Net{{Name: "DATA", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1Y"}, {Instance: "U2", Port: "1D"},
	}}}
	dir := f.SummaryDirection("U1", "U2")
	if dir != NetDirAtoB {
		t.Errorf("expected AtoB, got %d", dir)
	}
}

func TestSummaryDirectionBtoA(t *testing.T) {
	f := makeCircuit()
	// U2.1Q (output) → U1.1A (input): B→A from U1's perspective
	f.Nets = []Net{{Name: "FEEDBACK", Endpoints: []NetEndpoint{
		{Instance: "U2", Port: "1Q"}, {Instance: "U1", Port: "1A"},
	}}}
	dir := f.SummaryDirection("U1", "U2")
	if dir != NetDirBtoA {
		t.Errorf("expected BtoA, got %d", dir)
	}
}

func TestSummaryDirectionMixed(t *testing.T) {
	f := makeCircuit()
	// U1.1Y (output) → U2.1D (input) AND U2.1Q (output) → U1.1A (input)
	f.Nets = []Net{
		{Name: "DATA", Endpoints: []NetEndpoint{
			{Instance: "U1", Port: "1Y"}, {Instance: "U2", Port: "1D"},
		}},
		{Name: "FEEDBACK", Endpoints: []NetEndpoint{
			{Instance: "U2", Port: "1Q"}, {Instance: "U1", Port: "1A"},
		}},
	}
	dir := f.SummaryDirection("U1", "U2")
	if dir != NetDirMixed {
		t.Errorf("expected Mixed, got %d", dir)
	}
}

func TestSummaryDirectionNone(t *testing.T) {
	f := makeCircuit()
	dir := f.SummaryDirection("U1", "U2")
	if dir != NetDirNone {
		t.Errorf("expected None, got %d", dir)
	}
}

func TestSummaryDirectionPowerFiltered(t *testing.T) {
	f := makeCircuit()
	// Only power nets between U1 and U2 — should be None
	f.Nets = []Net{
		{Name: "VCC", Endpoints: []NetEndpoint{
			{Instance: "U1", Port: "VCC"}, {Instance: "U2", Port: "VCC"},
		}},
	}
	dir := f.SummaryDirection("U1", "U2")
	if dir != NetDirNone {
		t.Errorf("expected None (power-only), got %d", dir)
	}
}

// --- Cascade tests ---

func TestCascadeRenameState(t *testing.T) {
	f := makeCircuit()
	f.Nets = []Net{{Name: "DATA", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1Y"}, {Instance: "U2", Port: "1D"},
	}}}
	f.CascadeRenameState("U1", "U99")
	if f.Nets[0].Endpoints[0].Instance != "U99" {
		t.Errorf("expected U99, got %s", f.Nets[0].Endpoints[0].Instance)
	}
	if f.Nets[0].Endpoints[1].Instance != "U2" {
		t.Errorf("U2 should be unchanged")
	}
}

func TestCascadeDeleteStateRemovesEndpoints(t *testing.T) {
	f := makeCircuit()
	f.AddState("U3")
	f.StateClasses["U3"] = "7400_quad_nand"
	f.Nets = []Net{{Name: "BUS", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1Y"},
		{Instance: "U2", Port: "1D"},
		{Instance: "U3", Port: "1A"},
	}}}
	f.CascadeDeleteState("U3")
	if len(f.Nets) != 1 {
		t.Fatalf("expected net to survive (still 2 endpoints), got %d nets", len(f.Nets))
	}
	if len(f.Nets[0].Endpoints) != 2 {
		t.Errorf("expected 2 endpoints after removing U3, got %d", len(f.Nets[0].Endpoints))
	}
}

func TestCascadeDeleteStateRemovesNet(t *testing.T) {
	f := makeCircuit()
	f.Nets = []Net{{Name: "DATA", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1Y"}, {Instance: "U2", Port: "1D"},
	}}}
	// Deleting U1 leaves only 1 endpoint — net should be removed
	f.CascadeDeleteState("U1")
	if len(f.Nets) != 0 {
		t.Errorf("expected net to be removed (fell below 2 endpoints), got %d nets", len(f.Nets))
	}
}

// --- Validation tests ---

func TestValidateNetsClean(t *testing.T) {
	f := makeCircuit()
	f.Nets = []Net{{Name: "DATA", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1Y"}, {Instance: "U2", Port: "1D"},
	}}}
	errs := f.ValidateNets()
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateNetsDuplicateName(t *testing.T) {
	f := makeCircuit()
	f.Nets = []Net{
		{Name: "DATA", Endpoints: []NetEndpoint{
			{Instance: "U1", Port: "1Y"}, {Instance: "U2", Port: "1D"},
		}},
		{Name: "DATA", Endpoints: []NetEndpoint{
			{Instance: "U1", Port: "1A"}, {Instance: "U2", Port: "1CLK"},
		}},
	}
	errs := f.ValidateNets()
	if len(errs) == 0 {
		t.Error("expected error for duplicate net name")
	}
}

func TestValidateNetsBadEndpoint(t *testing.T) {
	f := makeCircuit()
	f.Nets = []Net{{Name: "BAD", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1Y"}, {Instance: "U99", Port: "1D"},
	}}}
	errs := f.ValidateNets()
	if len(errs) == 0 {
		t.Error("expected error for nonexistent instance")
	}
}

// --- JSON round-trip ---

func TestNetJSONRoundTrip(t *testing.T) {
	f := makeCircuit()
	f.AddNet(Net{Name: "DATA", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1Y"}, {Instance: "U2", Port: "1D"},
	}})

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	f2 := &FSM{}
	if err := json.Unmarshal(data, f2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(f2.Nets) != 1 {
		t.Fatalf("expected 1 net after round-trip, got %d", len(f2.Nets))
	}
	n := f2.Nets[0]
	if n.Name != "DATA" {
		t.Errorf("expected net name DATA, got %s", n.Name)
	}
	if len(n.Endpoints) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(n.Endpoints))
	}
}

func TestNetJSONOmittedWhenEmpty(t *testing.T) {
	f := New(TypeDFA)
	f.AddState("S1")

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	s := string(data)
	if indexOf(s, `"nets"`) >= 0 {
		t.Error("expected nets to be omitted from JSON when empty")
	}
}

// --- Self-connection test ---

func TestSelfConnectionNet(t *testing.T) {
	f := makeCircuit()
	// Self-connection: U1 gate A output to gate A input (unusual but valid)
	err := f.AddNet(Net{Name: "SELF", Endpoints: []NetEndpoint{
		{Instance: "U1", Port: "1Y"},
		{Instance: "U1", Port: "1A"},
	}})
	if err != nil {
		t.Fatalf("AddNet self-connection: %v", err)
	}
	nets := f.NetsBetween("U1", "U1")
	if len(nets) != 1 {
		t.Errorf("expected 1 self-net, got %d", len(nets))
	}
}
