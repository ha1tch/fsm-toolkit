package fsm

import "fmt"

// NetEndpoint identifies a specific port on a specific component instance.
type NetEndpoint struct {
	Instance string `json:"instance"` // state name = component ref designator
	Port     string `json:"port"`     // port name on the class
}

// Net represents a named electrical connection between two or more ports.
// Nets are the structural (wiring) counterpart to transitions (behavioural).
type Net struct {
	Name      string        `json:"name"`      // "CLK_BUS", "N42", "VCC"
	Endpoints []NetEndpoint `json:"endpoints"`
}

// --- Net-level helpers ---

// HasEndpoint returns true if the net has an endpoint on the given
// instance and port.
func (n *Net) HasEndpoint(instance, port string) bool {
	for _, ep := range n.Endpoints {
		if ep.Instance == instance && ep.Port == port {
			return true
		}
	}
	return false
}

// HasInstance returns true if the net has any endpoint on the given instance.
func (n *Net) HasInstance(instance string) bool {
	for _, ep := range n.Endpoints {
		if ep.Instance == instance {
			return true
		}
	}
	return false
}

// EndpointsForInstance returns all endpoints on the given instance.
func (n *Net) EndpointsForInstance(instance string) []NetEndpoint {
	var result []NetEndpoint
	for _, ep := range n.Endpoints {
		if ep.Instance == instance {
			result = append(result, ep)
		}
	}
	return result
}

// RemoveEndpointsByInstance removes all endpoints referencing the
// given instance. Returns the number of endpoints removed.
func (n *Net) RemoveEndpointsByInstance(instance string) int {
	kept := n.Endpoints[:0]
	removed := 0
	for _, ep := range n.Endpoints {
		if ep.Instance == instance {
			removed++
		} else {
			kept = append(kept, ep)
		}
	}
	n.Endpoints = kept
	return removed
}

// RenameInstance updates all endpoints that reference oldName to use newName.
func (n *Net) RenameInstance(oldName, newName string) {
	for i := range n.Endpoints {
		if n.Endpoints[i].Instance == oldName {
			n.Endpoints[i].Instance = newName
		}
	}
}

// IsValid returns true if the net has at least two endpoints.
func (n *Net) IsValid() bool {
	return len(n.Endpoints) >= 2
}

// --- FSM-level net helpers ---

// HasNets returns true if the FSM has any nets defined.
func (f *FSM) HasNets() bool {
	return len(f.Nets) > 0
}

// GetNet returns the net with the given name, or nil.
func (f *FSM) GetNet(name string) *Net {
	for i := range f.Nets {
		if f.Nets[i].Name == name {
			return &f.Nets[i]
		}
	}
	return nil
}

// AddNet adds a net to the FSM. Returns an error if the name is
// already taken or if validation fails.
func (f *FSM) AddNet(net Net) error {
	if net.Name == "" {
		return fmt.Errorf("net name cannot be empty")
	}
	if f.GetNet(net.Name) != nil {
		return fmt.Errorf("net %q already exists", net.Name)
	}
	if len(net.Endpoints) < 2 {
		return fmt.Errorf("net %q must have at least 2 endpoints, got %d", net.Name, len(net.Endpoints))
	}
	// Validate endpoints reference existing states and ports.
	for _, ep := range net.Endpoints {
		if err := f.validateEndpoint(ep); err != nil {
			return fmt.Errorf("net %q: %w", net.Name, err)
		}
	}
	f.Nets = append(f.Nets, net)
	return nil
}

// RemoveNet removes a net by name. Returns false if not found.
func (f *FSM) RemoveNet(name string) bool {
	for i := range f.Nets {
		if f.Nets[i].Name == name {
			f.Nets = append(f.Nets[:i], f.Nets[i+1:]...)
			return true
		}
	}
	return false
}

// RenameNet renames a net. Returns an error if the old name is not
// found or the new name is already taken.
func (f *FSM) RenameNet(oldName, newName string) error {
	if newName == "" {
		return fmt.Errorf("net name cannot be empty")
	}
	if oldName == newName {
		return nil
	}
	if f.GetNet(newName) != nil {
		return fmt.Errorf("net %q already exists", newName)
	}
	net := f.GetNet(oldName)
	if net == nil {
		return fmt.Errorf("net %q not found", oldName)
	}
	net.Name = newName
	return nil
}

// NetsForState returns all nets that have at least one endpoint on
// the given state.
func (f *FSM) NetsForState(state string) []Net {
	var result []Net
	for _, n := range f.Nets {
		if n.HasInstance(state) {
			result = append(result, n)
		}
	}
	return result
}

// NetsBetween returns all nets that have at least one endpoint on
// stateA and at least one endpoint on stateB.
func (f *FSM) NetsBetween(stateA, stateB string) []Net {
	var result []Net
	for _, n := range f.Nets {
		if n.HasInstance(stateA) && n.HasInstance(stateB) {
			result = append(result, n)
		}
	}
	return result
}

// IsPowerNet returns true if every endpoint in the net references a
// power-direction port. Returns false if any endpoint cannot be
// resolved (missing state or class) or if the net has no endpoints.
func (f *FSM) IsPowerNet(net Net) bool {
	if len(net.Endpoints) == 0 {
		return false
	}
	f.EnsureClassMaps()
	for _, ep := range net.Endpoints {
		ports := f.EffectivePorts(ep.Instance)
		if ports == nil {
			return false
		}
		found := false
		for _, p := range ports {
			if p.Name == ep.Port {
				if p.Direction != PortPower {
					return false
				}
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// SignalNets returns all nets that are not pure power nets.
func (f *FSM) SignalNets() []Net {
	var result []Net
	for _, n := range f.Nets {
		if !f.IsPowerNet(n) {
			result = append(result, n)
		}
	}
	return result
}

// SignalNetsBetween returns non-power nets between two states.
func (f *FSM) SignalNetsBetween(stateA, stateB string) []Net {
	var result []Net
	for _, n := range f.Nets {
		if n.HasInstance(stateA) && n.HasInstance(stateB) && !f.IsPowerNet(n) {
			result = append(result, n)
		}
	}
	return result
}

// NetDirection describes the signal direction of a summary line
// between two components.
type NetDirection int

const (
	NetDirNone    NetDirection = iota // no signal nets
	NetDirAtoB                       // all signals flow A→B
	NetDirBtoA                       // all signals flow B→A
	NetDirMixed                      // mixed or bidirectional
)

// SummaryDirection computes the aggregate signal direction between
// two states by examining all non-power nets connecting them.
// Returns NetDirNone if there are no signal nets between them.
func (f *FSM) SummaryDirection(stateA, stateB string) NetDirection {
	nets := f.SignalNetsBetween(stateA, stateB)
	if len(nets) == 0 {
		return NetDirNone
	}

	f.EnsureClassMaps()
	hasAtoB := false
	hasBtoA := false

	for _, n := range nets {
		for _, ep := range n.Endpoints {
			if ep.Instance != stateA && ep.Instance != stateB {
				continue // endpoint on a third component
			}
			ports := f.EffectivePorts(ep.Instance)
			for _, p := range ports {
				if p.Name != ep.Port {
					continue
				}
				switch p.Direction {
				case PortOutput:
					if ep.Instance == stateA {
						hasAtoB = true
					} else {
						hasBtoA = true
					}
				case PortInput:
					// Input on A means signal comes from B, etc.
					// But direction is determined by the output side.
				case PortBidir:
					hasAtoB = true
					hasBtoA = true
				case PortPower:
					// Filtered out by SignalNetsBetween.
				}
				break
			}
		}
		if hasAtoB && hasBtoA {
			return NetDirMixed
		}
	}

	switch {
	case hasAtoB && !hasBtoA:
		return NetDirAtoB
	case hasBtoA && !hasAtoB:
		return NetDirBtoA
	case hasAtoB && hasBtoA:
		return NetDirMixed
	default:
		// All endpoints are inputs with no outputs — unusual but not
		// impossible. Treat as mixed to flag for the designer.
		if len(nets) > 0 {
			return NetDirMixed
		}
		return NetDirNone
	}
}

// --- Cascade operations ---

// CascadeRenameState updates all net endpoints that reference oldName
// to use newName.
func (f *FSM) CascadeRenameState(oldName, newName string) {
	for i := range f.Nets {
		f.Nets[i].RenameInstance(oldName, newName)
	}
}

// CascadeDeleteState removes all net endpoints referencing the given
// state. Nets that fall below two endpoints are removed entirely.
func (f *FSM) CascadeDeleteState(state string) {
	kept := f.Nets[:0]
	for i := range f.Nets {
		f.Nets[i].RemoveEndpointsByInstance(state)
		if f.Nets[i].IsValid() {
			kept = append(kept, f.Nets[i])
		}
	}
	f.Nets = kept
}

// ValidateNets checks all nets for consistency: endpoint instances
// must exist as states, endpoint ports must exist on the assigned
// class. Returns a list of errors (empty if all valid).
func (f *FSM) ValidateNets() []error {
	var errs []error
	seen := make(map[string]bool)
	for _, n := range f.Nets {
		if seen[n.Name] {
			errs = append(errs, fmt.Errorf("duplicate net name %q", n.Name))
		}
		seen[n.Name] = true

		if !n.IsValid() {
			errs = append(errs, fmt.Errorf("net %q has fewer than 2 endpoints", n.Name))
		}
		for _, ep := range n.Endpoints {
			if err := f.validateEndpoint(ep); err != nil {
				errs = append(errs, fmt.Errorf("net %q: %w", n.Name, err))
			}
		}
	}
	return errs
}

// validateEndpoint checks that an endpoint references a valid state
// and a valid port on that state's class.
func (f *FSM) validateEndpoint(ep NetEndpoint) error {
	// Check state exists.
	found := false
	for _, s := range f.States {
		if s == ep.Instance {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("instance %q is not a state", ep.Instance)
	}

	// Check port exists on the state's class.
	ports := f.EffectivePorts(ep.Instance)
	if ports == nil {
		return fmt.Errorf("instance %q has no ports (class has no port definitions)", ep.Instance)
	}
	for _, p := range ports {
		if p.Name == ep.Port {
			return nil
		}
	}
	return fmt.Errorf("port %q not found on instance %q", ep.Port, ep.Instance)
}
