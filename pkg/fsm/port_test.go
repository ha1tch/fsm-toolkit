package fsm

import (
	"encoding/json"
	"testing"
)

func TestPortDirConstants(t *testing.T) {
	dirs := ValidPortDirs()
	if len(dirs) != 4 {
		t.Fatalf("expected 4 port directions, got %d", len(dirs))
	}
	expected := map[PortDir]bool{PortInput: true, PortOutput: true, PortBidir: true, PortPower: true}
	for _, d := range dirs {
		if !expected[d] {
			t.Errorf("unexpected port direction: %q", d)
		}
	}
}

func TestIsValidPortDir(t *testing.T) {
	for _, d := range []string{"input", "output", "bidir", "power"} {
		if !IsValidPortDir(d) {
			t.Errorf("expected %q to be valid", d)
		}
	}
	for _, d := range []string{"", "in", "out", "INPUT", "Power"} {
		if IsValidPortDir(d) {
			t.Errorf("expected %q to be invalid", d)
		}
	}
}

func make7474Class() *Class {
	return &Class{
		Name:       "7474_dual_d_flipflop",
		Properties: []PropertyDef{{Name: "trigger", Type: PropShortString}},
		Ports: []Port{
			{Name: "1CLR_N", Direction: PortInput, PinNumber: 1, Group: "FF1"},
			{Name: "1D", Direction: PortInput, PinNumber: 2, Group: "FF1"},
			{Name: "1CLK", Direction: PortInput, PinNumber: 3, Group: "FF1"},
			{Name: "1PRE_N", Direction: PortInput, PinNumber: 4, Group: "FF1"},
			{Name: "1Q", Direction: PortOutput, PinNumber: 5, Group: "FF1"},
			{Name: "1Q_N", Direction: PortOutput, PinNumber: 6, Group: "FF1"},
			{Name: "GND", Direction: PortPower, PinNumber: 7},
			{Name: "2Q_N", Direction: PortOutput, PinNumber: 8, Group: "FF2"},
			{Name: "2Q", Direction: PortOutput, PinNumber: 9, Group: "FF2"},
			{Name: "2PRE_N", Direction: PortInput, PinNumber: 10, Group: "FF2"},
			{Name: "2CLK", Direction: PortInput, PinNumber: 11, Group: "FF2"},
			{Name: "2D", Direction: PortInput, PinNumber: 12, Group: "FF2"},
			{Name: "2CLR_N", Direction: PortInput, PinNumber: 13, Group: "FF2"},
			{Name: "VCC", Direction: PortPower, PinNumber: 14},
		},
	}
}

func TestClassHasPorts(t *testing.T) {
	c := &Class{Name: "empty", Properties: nil}
	if c.HasPorts() {
		t.Error("expected HasPorts() = false for class with no ports")
	}
	c = make7474Class()
	if !c.HasPorts() {
		t.Error("expected HasPorts() = true for 7474")
	}
}

func TestClassGetPort(t *testing.T) {
	c := make7474Class()
	p := c.GetPort("1CLK")
	if p == nil {
		t.Fatal("expected to find port 1CLK")
	}
	if p.Direction != PortInput {
		t.Errorf("expected input, got %s", p.Direction)
	}
	if p.PinNumber != 3 {
		t.Errorf("expected pin 3, got %d", p.PinNumber)
	}
	if p.Group != "FF1" {
		t.Errorf("expected group FF1, got %q", p.Group)
	}
	if c.GetPort("nonexistent") != nil {
		t.Error("expected nil for nonexistent port")
	}
}

func TestClassSignalPorts(t *testing.T) {
	c := make7474Class()
	sig := c.SignalPorts()
	// 14 total - 2 power (GND, VCC) = 12 signal
	if len(sig) != 12 {
		t.Errorf("expected 12 signal ports, got %d", len(sig))
	}
	for _, p := range sig {
		if p.Direction == PortPower {
			t.Errorf("signal ports should not include power port %q", p.Name)
		}
	}
}

func TestClassPowerPorts(t *testing.T) {
	c := make7474Class()
	pwr := c.PowerPorts()
	if len(pwr) != 2 {
		t.Errorf("expected 2 power ports, got %d", len(pwr))
	}
	names := map[string]bool{}
	for _, p := range pwr {
		names[p.Name] = true
	}
	if !names["VCC"] || !names["GND"] {
		t.Errorf("expected VCC and GND, got %v", names)
	}
}

func TestClassPortsByGroup(t *testing.T) {
	c := make7474Class()
	groups := c.PortsByGroup()
	if len(groups) != 3 {
		t.Errorf("expected 3 groups (FF1, FF2, ungrouped), got %d", len(groups))
	}
	if len(groups["FF1"]) != 6 {
		t.Errorf("expected 6 ports in FF1, got %d", len(groups["FF1"]))
	}
	if len(groups["FF2"]) != 6 {
		t.Errorf("expected 6 ports in FF2, got %d", len(groups["FF2"]))
	}
	if len(groups[""]) != 2 {
		t.Errorf("expected 2 ungrouped ports (VCC, GND), got %d", len(groups[""]))
	}
}

func TestClassPortGroups(t *testing.T) {
	c := make7474Class()
	groups := c.PortGroups()
	// FF1, FF2 in declaration order, then "" last
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d: %v", len(groups), groups)
	}
	if groups[0] != "FF1" {
		t.Errorf("expected first group FF1, got %q", groups[0])
	}
	if groups[1] != "FF2" {
		t.Errorf("expected second group FF2, got %q", groups[1])
	}
	if groups[2] != "" {
		t.Errorf("expected last group empty, got %q", groups[2])
	}
}

func TestClassPortGroupsNoPower(t *testing.T) {
	c := &Class{
		Name: "simple",
		Ports: []Port{
			{Name: "A", Direction: PortInput, Group: "G1"},
			{Name: "Y", Direction: PortOutput, Group: "G1"},
		},
	}
	groups := c.PortGroups()
	if len(groups) != 1 || groups[0] != "G1" {
		t.Errorf("expected [G1], got %v", groups)
	}
}

func TestClassValidatePorts(t *testing.T) {
	c := make7474Class()
	if err := c.ValidatePorts(); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}

	dup := &Class{
		Name: "bad",
		Ports: []Port{
			{Name: "A", Direction: PortInput},
			{Name: "A", Direction: PortOutput},
		},
	}
	if err := dup.ValidatePorts(); err == nil {
		t.Error("expected error for duplicate port names")
	}
}

func TestClassNoPorts(t *testing.T) {
	c := &Class{Name: "behavioural", Properties: []PropertyDef{
		{Name: "description", Type: PropString},
	}}
	if c.HasPorts() {
		t.Error("behavioural class should not have ports")
	}
	if len(c.SignalPorts()) != 0 {
		t.Error("expected empty signal ports")
	}
	if len(c.PowerPorts()) != 0 {
		t.Error("expected empty power ports")
	}
	if len(c.PortsByGroup()) != 0 {
		t.Error("expected empty port groups")
	}
	if c.GetPort("anything") != nil {
		t.Error("expected nil from GetPort")
	}
	if err := c.ValidatePorts(); err != nil {
		t.Errorf("expected no error for empty ports, got: %v", err)
	}
}

func TestFSMEffectivePorts(t *testing.T) {
	f := New(TypeDFA)
	f.AddState("U3")
	cls := make7474Class()
	f.Classes[cls.Name] = cls
	f.StateClasses["U3"] = cls.Name

	ports := f.EffectivePorts("U3")
	if len(ports) != 14 {
		t.Errorf("expected 14 ports for U3, got %d", len(ports))
	}

	// State with default class has no ports
	f.AddState("S1")
	ports = f.EffectivePorts("S1")
	if ports != nil {
		t.Errorf("expected nil ports for default_state class, got %d", len(ports))
	}

	// Nonexistent state
	ports = f.EffectivePorts("nonexistent")
	if ports != nil {
		t.Errorf("expected nil ports for nonexistent state, got %v", ports)
	}
}

func TestFSMStateHasPorts(t *testing.T) {
	f := New(TypeDFA)
	f.AddState("U3")
	cls := make7474Class()
	f.Classes[cls.Name] = cls
	f.StateClasses["U3"] = cls.Name

	if !f.StateHasPorts("U3") {
		t.Error("expected StateHasPorts = true for U3")
	}

	f.AddState("S1")
	if f.StateHasPorts("S1") {
		t.Error("expected StateHasPorts = false for S1")
	}
}

func TestPortJSONRoundTrip(t *testing.T) {
	f := New(TypeDFA)
	f.AddState("U3")
	cls := make7474Class()
	f.Classes[cls.Name] = cls
	f.StateClasses["U3"] = cls.Name

	// Marshal
	data, err := jsonMarshal(f)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// Unmarshal
	f2 := &FSM{}
	if err := jsonUnmarshal(data, f2); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	// Verify ports survived
	cls2, ok := f2.Classes[cls.Name]
	if !ok {
		t.Fatal("class not found after round-trip")
	}
	if len(cls2.Ports) != 14 {
		t.Errorf("expected 14 ports after round-trip, got %d", len(cls2.Ports))
	}
	p := cls2.GetPort("1CLK")
	if p == nil {
		t.Fatal("port 1CLK not found after round-trip")
	}
	if p.Direction != PortInput || p.PinNumber != 3 || p.Group != "FF1" {
		t.Errorf("port 1CLK data mismatch: dir=%s pin=%d group=%q", p.Direction, p.PinNumber, p.Group)
	}
}

func TestPortsOmittedWhenEmpty(t *testing.T) {
	f := New(TypeDFA)
	f.AddState("S1")

	data, err := jsonMarshal(f)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// Default class should not emit "ports" in JSON
	s := string(data)
	// The default_state class has no ports, so "ports" should not appear
	// in the default_state class definition. Check by looking for the
	// class name near a ports field.
	if containsPortsField(s, DefaultClassName) {
		t.Error("expected ports to be omitted for default_state class")
	}
}

// containsPortsField is a rough check: does the JSON contain a "ports"
// key within the context of the given class name? This is an
// approximation — a full parse would be more robust but this suffices
// for the omitempty check.
func containsPortsField(json, className string) bool {
	// Find class position
	idx := indexOf(json, `"`+className+`"`)
	if idx < 0 {
		return false
	}
	// Look for "ports" within the next 200 chars (generous for a small class)
	end := idx + 200
	if end > len(json) {
		end = len(json)
	}
	return indexOf(json[idx:end], `"ports"`) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
