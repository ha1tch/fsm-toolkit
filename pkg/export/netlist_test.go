package export

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ha1tch/fsm-toolkit/pkg/fsm"
)

func strp(s string) *string { return &s }

// buildTestCircuit creates a two-component circuit for testing.
func buildTestCircuit() *fsm.FSM {
	f := fsm.New(fsm.TypeDFA)
	f.Name = "Test Circuit"
	f.AddState("U1")
	f.AddState("U2")
	f.AddInput("clk")
	f.AddTransition("U1", strp("clk"), []string{"U2"}, nil)

	nand := &fsm.Class{
		Name:       "7400_quad_nand",
		Properties: []fsm.PropertyDef{{Name: "pkg", Type: fsm.PropShortString}},
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
		{
			Name: "GND_RAIL",
			Endpoints: []fsm.NetEndpoint{
				{Instance: "U1", Port: "GND"},
				{Instance: "U2", Port: "GND"},
			},
		},
	}

	return f
}

// --- Build tests ---

func TestBuild(t *testing.T) {
	f := buildTestCircuit()
	nl := Build(f)

	if nl.Name != "Test Circuit" {
		t.Errorf("name: got %q, want %q", nl.Name, "Test Circuit")
	}
	if len(nl.Components) != 2 {
		t.Fatalf("components: got %d, want 2", len(nl.Components))
	}
	if len(nl.Nets) != 3 {
		t.Fatalf("nets: got %d, want 3", len(nl.Nets))
	}

	// Components sorted alphabetically.
	if nl.Components[0].Ref != "U1" || nl.Components[1].Ref != "U2" {
		t.Errorf("components not sorted: got %s, %s", nl.Components[0].Ref, nl.Components[1].Ref)
	}

	// KiCad fields derived for 74xx.
	if nl.Components[0].Footprint != "Package_DIP:DIP-5_W7.62mm" {
		t.Errorf("U1 footprint: got %q", nl.Components[0].Footprint)
	}
	if len(nl.Unresolved) != 0 {
		t.Errorf("expected 0 unresolved, got %d: %v", len(nl.Unresolved), nl.Unresolved)
	}
}

func TestBuildSkipsBehaviouralStates(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.Name = "Mixed"
	f.AddState("idle")   // behavioural — default class, no ports
	f.AddState("U1")
	f.AddInput("go")
	f.AddTransition("idle", strp("go"), []string{"U1"}, nil)

	nand := &fsm.Class{
		Name:       "7400_quad_nand",
		Properties: []fsm.PropertyDef{},
		Ports: []fsm.Port{
			{Name: "1A", Direction: fsm.PortInput, PinNumber: 1},
			{Name: "GND", Direction: fsm.PortPower, PinNumber: 7},
		},
	}
	f.Classes[nand.Name] = nand
	f.StateClasses["U1"] = nand.Name

	nl := Build(f)
	if len(nl.Components) != 1 {
		t.Errorf("expected 1 component (U1 only), got %d", len(nl.Components))
	}
	if len(nl.Components) > 0 && nl.Components[0].Ref != "U1" {
		t.Errorf("expected U1, got %s", nl.Components[0].Ref)
	}
}

func TestBuildUnresolved(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.Name = "Custom"
	f.AddState("X1")

	custom := &fsm.Class{
		Name:       "my_custom_chip",
		Properties: []fsm.PropertyDef{},
		Ports: []fsm.Port{
			{Name: "IN", Direction: fsm.PortInput, PinNumber: 1},
			{Name: "OUT", Direction: fsm.PortOutput, PinNumber: 2},
		},
	}
	f.Classes[custom.Name] = custom
	f.StateClasses["X1"] = custom.Name

	nl := Build(f)
	if len(nl.Unresolved) != 1 {
		t.Errorf("expected 1 unresolved, got %d", len(nl.Unresolved))
	}
}

func TestBuildDoesNotMutateSource(t *testing.T) {
	f := buildTestCircuit()
	cls := f.Classes["7400_quad_nand"]

	// Ensure the source has no KiCad fields.
	if cls.KiCadPart != "" || cls.KiCadFootprint != "" {
		t.Fatal("precondition: source class already has KiCad fields")
	}

	Build(f)

	// Source must remain unchanged.
	if cls.KiCadPart != "" || cls.KiCadFootprint != "" {
		t.Error("Build() mutated the source FSM class")
	}
}

// --- Derivation tests ---

func TestDeriveKiCadFields(t *testing.T) {
	cases := []struct {
		name     string
		pins     int
		wantPart string
		wantFP   string
	}{
		{"7400_quad_nand", 14, "74xx:7400", "Package_DIP:DIP-14_W7.62mm"},
		{"7474_dual_d_flipflop", 14, "74xx:7474", "Package_DIP:DIP-14_W7.62mm"},
		{"74181_alu_4bit", 24, "74xx:74181", "Package_DIP:DIP-24_W7.62mm"},
		{"7432_quad_or", 14, "74xx:7432", "Package_DIP:DIP-14_W7.62mm"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cls := &fsm.Class{Name: tc.name}
			for i := 0; i < tc.pins; i++ {
				cls.Ports = append(cls.Ports, fsm.Port{
					Name:      "P" + string(rune('A'+i)),
					Direction: fsm.PortInput,
					PinNumber: i + 1,
				})
			}

			cls.DeriveKiCadFields()

			if cls.KiCadPart != tc.wantPart {
				t.Errorf("KiCadPart: got %q, want %q", cls.KiCadPart, tc.wantPart)
			}
			if cls.KiCadFootprint != tc.wantFP {
				t.Errorf("KiCadFootprint: got %q, want %q", cls.KiCadFootprint, tc.wantFP)
			}
		})
	}
}

func TestDeriveNon74xx(t *testing.T) {
	cls := &fsm.Class{
		Name: "custom_decoder",
		Ports: []fsm.Port{
			{Name: "A", Direction: fsm.PortInput, PinNumber: 1},
			{Name: "B", Direction: fsm.PortOutput, PinNumber: 2},
		},
	}

	cls.DeriveKiCadFields()

	if cls.KiCadPart != "" {
		t.Errorf("expected empty KiCadPart for non-74xx, got %q", cls.KiCadPart)
	}
	// Footprint should still be derived from pin count.
	if cls.KiCadFootprint != "Package_DIP:DIP-2_W7.62mm" {
		t.Errorf("KiCadFootprint: got %q", cls.KiCadFootprint)
	}
}

func TestDeriveDoesNotOverrideExisting(t *testing.T) {
	cls := &fsm.Class{
		Name:           "7400_quad_nand",
		KiCadPart:      "custom_lib:CUSTOM_7400",
		KiCadFootprint: "MyFP:Custom_DIP",
		Ports: []fsm.Port{
			{Name: "A", Direction: fsm.PortInput, PinNumber: 1},
		},
	}

	changed := cls.DeriveKiCadFields()
	if changed {
		t.Error("DeriveKiCadFields should not change when fields are already set")
	}
	if cls.KiCadPart != "custom_lib:CUSTOM_7400" {
		t.Error("existing KiCadPart was overwritten")
	}
	if cls.KiCadFootprint != "MyFP:Custom_DIP" {
		t.Error("existing KiCadFootprint was overwritten")
	}
}

func TestDeriveWideDIP(t *testing.T) {
	cls := &fsm.Class{Name: "74999_big_chip"}
	for i := 0; i < 40; i++ {
		cls.Ports = append(cls.Ports, fsm.Port{
			Name: "P" + string(rune('A'+i%26)), Direction: fsm.PortInput, PinNumber: i + 1,
		})
	}

	cls.DeriveKiCadFields()

	if cls.KiCadFootprint != "Package_DIP:DIP-40_W15.24mm" {
		t.Errorf("expected wide DIP, got %q", cls.KiCadFootprint)
	}
}

// --- Text export tests ---

func TestWriteText(t *testing.T) {
	f := buildTestCircuit()
	nl := Build(f)

	var buf bytes.Buffer
	if err := WriteText(&buf, nl); err != nil {
		t.Fatalf("WriteText: %v", err)
	}

	out := buf.String()

	// Check header.
	if !strings.Contains(out, "Netlist: Test Circuit") {
		t.Error("missing netlist name in header")
	}
	if !strings.Contains(out, "Components: 2") {
		t.Error("missing component count")
	}

	// Check components.
	if !strings.Contains(out, "U1") || !strings.Contains(out, "7400_quad_nand") {
		t.Error("missing U1 component")
	}
	if !strings.Contains(out, "U2") || !strings.Contains(out, "7474_dual_d_flipflop") {
		t.Error("missing U2 component")
	}

	// Check nets with pin numbers.
	if !strings.Contains(out, "DATA_BUS:") {
		t.Error("missing DATA_BUS net")
	}
	if !strings.Contains(out, "pin3") {
		t.Error("missing pin3 reference for U1.1Y")
	}
	if !strings.Contains(out, "pin2") {
		t.Error("missing pin2 reference for U2.1D")
	}
}

// --- JSON export tests ---

func TestWriteJSON(t *testing.T) {
	f := buildTestCircuit()
	nl := Build(f)

	var buf bytes.Buffer
	if err := WriteJSON(&buf, nl); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// Parse it back.
	var parsed Netlist
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}

	if parsed.Name != "Test Circuit" {
		t.Errorf("name: got %q", parsed.Name)
	}
	if len(parsed.Components) != 2 {
		t.Errorf("components: got %d, want 2", len(parsed.Components))
	}
	if len(parsed.Nets) != 3 {
		t.Errorf("nets: got %d, want 3", len(parsed.Nets))
	}

	// Verify pin mapping survived round-trip.
	for _, n := range parsed.Nets {
		if n.Name == "DATA_BUS" {
			for _, p := range n.Pins {
				if p.Ref == "U1" && p.Pin != "3" {
					t.Errorf("DATA_BUS U1 pin: got %q, want 3", p.Pin)
				}
				if p.Ref == "U2" && p.Pin != "2" {
					t.Errorf("DATA_BUS U2 pin: got %q, want 2", p.Pin)
				}
			}
		}
	}
}

// --- KiCad export tests ---

func TestWriteKiCad(t *testing.T) {
	f := buildTestCircuit()
	nl := Build(f)

	var buf bytes.Buffer
	if err := WriteKiCad(&buf, nl, f); err != nil {
		t.Fatalf("WriteKiCad: %v", err)
	}

	out := buf.String()

	// Check overall structure.
	if !strings.HasPrefix(out, "(export (version D)") {
		t.Error("missing export header")
	}
	if !strings.Contains(out, "(components") {
		t.Error("missing components section")
	}
	if !strings.Contains(out, "(nets") {
		t.Error("missing nets section")
	}

	// Check component entries.
	if !strings.Contains(out, "(comp (ref U1)") {
		t.Error("missing U1 component")
	}
	if !strings.Contains(out, "(comp (ref U2)") {
		t.Error("missing U2 component")
	}

	// Check KiCad part derivation.
	if !strings.Contains(out, "(value 7400)") {
		t.Error("missing derived value 7400")
	}
	if !strings.Contains(out, "(value 7474)") {
		t.Error("missing derived value 7474")
	}

	// Check library source.
	if !strings.Contains(out, "(lib 74xx)") {
		t.Error("missing 74xx library reference")
	}

	// Check footprints.
	if !strings.Contains(out, "Package_DIP:DIP-5_W7.62mm") {
		t.Logf("Output:\n%s", out)
		t.Error("missing DIP footprint")
	}

	// Check nets with pin numbers.
	if !strings.Contains(out, `(name "DATA_BUS")`) {
		t.Error("missing DATA_BUS net")
	}
	if !strings.Contains(out, "(pin 3)") {
		t.Error("missing pin 3 in nets")
	}
	if !strings.Contains(out, "(pin 2)") {
		t.Error("missing pin 2 in nets")
	}
}

func TestWriteKiCadUnresolved(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.Name = "Custom"
	f.AddState("X1")

	custom := &fsm.Class{
		Name:       "my_decoder",
		Properties: []fsm.PropertyDef{},
		Ports: []fsm.Port{
			{Name: "A", Direction: fsm.PortInput, PinNumber: 1},
			{Name: "B", Direction: fsm.PortOutput, PinNumber: 2},
		},
	}
	f.Classes[custom.Name] = custom
	f.StateClasses["X1"] = custom.Name

	nl := Build(f)

	var buf bytes.Buffer
	if err := WriteKiCad(&buf, nl, f); err != nil {
		t.Fatalf("WriteKiCad: %v", err)
	}

	out := buf.String()

	// Should use fsm-toolkit as library for unresolved parts.
	if !strings.Contains(out, "(lib fsm-toolkit)") {
		t.Error("expected fsm-toolkit library for unresolved component")
	}
	if !strings.Contains(out, "(part my_decoder)") {
		t.Error("expected my_decoder as part name")
	}
}

func TestKiCadEscaping(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.Name = "Test (special)"
	f.AddState("U 1") // space in ref

	cls := &fsm.Class{
		Name: "7400_quad_nand",
		Ports: []fsm.Port{
			{Name: "A", Direction: fsm.PortInput, PinNumber: 1},
		},
	}
	f.Classes[cls.Name] = cls
	f.StateClasses["U 1"] = cls.Name

	nl := Build(f)

	var buf bytes.Buffer
	if err := WriteKiCad(&buf, nl, f); err != nil {
		t.Fatalf("WriteKiCad: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"U 1"`) {
		t.Error("expected quoted ref for name with space")
	}
}

// --- Empty FSM tests ---

func TestEmptyFSM(t *testing.T) {
	f := fsm.New(fsm.TypeDFA)
	f.Name = "Empty"
	f.AddState("idle")

	nl := Build(f)

	if len(nl.Components) != 0 {
		t.Errorf("expected 0 components, got %d", len(nl.Components))
	}
	if len(nl.Nets) != 0 {
		t.Errorf("expected 0 nets, got %d", len(nl.Nets))
	}

	// All exporters should handle empty gracefully.
	var buf bytes.Buffer
	if err := WriteText(&buf, nl); err != nil {
		t.Errorf("WriteText on empty: %v", err)
	}

	buf.Reset()
	if err := WriteJSON(&buf, nl); err != nil {
		t.Errorf("WriteJSON on empty: %v", err)
	}

	buf.Reset()
	if err := WriteKiCad(&buf, nl, f); err != nil {
		t.Errorf("WriteKiCad on empty: %v", err)
	}
}
