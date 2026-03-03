package fsm

import "fmt"

// PortDir represents the signal direction of a port.
type PortDir string

const (
	PortInput  PortDir = "input"
	PortOutput PortDir = "output"
	PortBidir  PortDir = "bidir"
	PortPower  PortDir = "power"
)

// ValidPortDirs returns all valid port direction strings.
func ValidPortDirs() []PortDir {
	return []PortDir{PortInput, PortOutput, PortBidir, PortPower}
}

// IsValidPortDir checks whether a string is a valid port direction.
func IsValidPortDir(s string) bool {
	for _, d := range ValidPortDirs() {
		if string(d) == s {
			return true
		}
	}
	return false
}

// Port represents a connectable interface on a component.
// Ports belong to a Class and define the physical or logical
// pins through which instances of that class connect to other
// components.
type Port struct {
	Name      string  `json:"name"`                 // "1CLK", "1D", "1Q", "VCC"
	Direction PortDir `json:"direction"`             // input, output, bidir, power
	PinNumber int     `json:"pin_number,omitempty"`  // physical pin; 0 = unassigned
	Group     string  `json:"group,omitempty"`       // sub-unit: "FF1", "GATE_A"
}

// --- Class-level port helpers ---

// HasPorts returns true if the class defines any ports.
func (c *Class) HasPorts() bool {
	return len(c.Ports) > 0
}

// GetPort returns the port with the given name, or nil.
func (c *Class) GetPort(name string) *Port {
	for i := range c.Ports {
		if c.Ports[i].Name == name {
			return &c.Ports[i]
		}
	}
	return nil
}

// SignalPorts returns all ports whose direction is not power.
func (c *Class) SignalPorts() []Port {
	var result []Port
	for _, p := range c.Ports {
		if p.Direction != PortPower {
			result = append(result, p)
		}
	}
	return result
}

// PowerPorts returns all ports whose direction is power.
func (c *Class) PowerPorts() []Port {
	var result []Port
	for _, p := range c.Ports {
		if p.Direction == PortPower {
			result = append(result, p)
		}
	}
	return result
}

// PortsByGroup returns ports grouped by their Group field.
// Ports with an empty Group are collected under the key "".
func (c *Class) PortsByGroup() map[string][]Port {
	result := make(map[string][]Port)
	for _, p := range c.Ports {
		result[p.Group] = append(result[p.Group], p)
	}
	return result
}

// PortGroups returns the distinct group names in declaration order,
// with the empty group (ungrouped ports) last if present.
func (c *Class) PortGroups() []string {
	seen := make(map[string]bool)
	var groups []string
	hasEmpty := false
	for _, p := range c.Ports {
		if p.Group == "" {
			hasEmpty = true
			continue
		}
		if !seen[p.Group] {
			seen[p.Group] = true
			groups = append(groups, p.Group)
		}
	}
	if hasEmpty {
		groups = append(groups, "")
	}
	return groups
}

// ValidatePorts checks that port names are unique within the class.
// Returns nil if valid, or an error describing the first duplicate.
func (c *Class) ValidatePorts() error {
	seen := make(map[string]bool)
	for _, p := range c.Ports {
		if seen[p.Name] {
			return fmt.Errorf("duplicate port name %q in class %q", p.Name, c.Name)
		}
		seen[p.Name] = true
	}
	return nil
}

// --- FSM-level port helpers ---

// EffectivePorts returns the port list from the class assigned to the
// given state. Returns nil if the class has no ports or the state has
// no class assignment.
func (f *FSM) EffectivePorts(state string) []Port {
	f.EnsureClassMaps()
	className := f.GetStateClass(state)
	cls, ok := f.Classes[className]
	if !ok {
		return nil
	}
	return cls.Ports
}

// StateHasPorts returns true if the state's assigned class defines ports.
func (f *FSM) StateHasPorts(state string) bool {
	ports := f.EffectivePorts(state)
	return len(ports) > 0
}
